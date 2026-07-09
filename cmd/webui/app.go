// cmd/webui/app.go
// Wails application bridge between StelleEngine and the Svelte frontend.
// Exposes playback controls, queue management, folder loading (replace and
// append), lazy cover-art extraction, and shuffle/loop modes as callable
// JavaScript functions via Wails bindings.
//
// Key Components:
//   - App: bound struct; every exported method becomes a JS binding
//   - TrackInfo: per-track metadata pushed to the frontend (no raw cover bytes)
//
// Dependencies:
//   - Audioengine: Engine interface and PlaybackState enum
//   - Audioengine/StelleEngine: SDL3-based audio backend
//   - Audioengine/MetaResolver: UI-agnostic metadata + cover extraction
//   - wails/v2/pkg/runtime: Wails event system and dialog API
//
// Concurrency:
//   - Wails invokes each bound method on its own goroutine and broadcastProgress
//     runs on a ticker, so all access to queue/current/order/mode state is
//     guarded by mu.

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math/rand"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	AudioEngine "github.com/dlcuy22/OngoPlayer/Audioengine"
	"github.com/dlcuy22/OngoPlayer/Audioengine/MetaResolver"
	stelleengine "github.com/dlcuy22/OngoPlayer/Audioengine/StelleEngine"
	"github.com/dlcuy22/OngoPlayer/internal/logging"
	"github.com/dlcuy22/OngoPlayer/internal/service/discordrpc"
	"github.com/dlcuy22/OngoPlayer/internal/service/lyrics"
	"github.com/dlcuy22/OngoPlayer/internal/service/ytdl"
	"github.com/dlcuy22/ytm-go"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var log = logging.NewLogger("webui")

func logInfo(msg string, args ...any) {
	log.Info(fmt.Sprintf(msg, args...))
}

func logDebug(msg string, args ...any) {
	log.Debug(fmt.Sprintf(msg, args...))
}

func logError(msg string, args ...any) {
	log.Error(fmt.Sprintf(msg, args...))
}

// LoopMode enumerates the repeat behavior at end-of-track.
type LoopMode int

const (
	LoopOff LoopMode = iota // stop after the last track
	LoopAll                 // wrap to the first track
	LoopOne                 // replay the current track
)

// LyricLine is a single timestamped lyric line pushed to the frontend.
type LyricLine struct {
	Time float64 `json:"time"`
	Text string  `json:"text"`
}

type TrackInfo struct {
	Path           string `json:"path"`
	Name           string `json:"name"`
	Index          int    `json:"index"`
	Title          string `json:"title"`
	Artist         string `json:"artist"`
	Album          string `json:"album"`
	Format         string `json:"format"`
	HasCover       bool   `json:"hasCover"`
	LyricsBrowseID string `json:"lyricsBrowseID,omitempty"`
	YTMSongID      string `json:"ytmSongID,omitempty"`
	CoverURL       string `json:"coverURL,omitempty"`
}

type TrackChangedPayload struct {
	Track   *TrackInfo `json:"track"`
	Playing bool       `json:"playing"`
}

type activeDownload struct {
	done     chan struct{}
	cancel   context.CancelFunc
	filePath string
}

type AppConfig struct {
	StreamQuality string `json:"streamQuality"`
	StreamCodec   string `json:"streamCodec"`
	RPCEnabled    bool   `json:"rpcEnabled"`
	Volume        int    `json:"volume"`
}

type App struct {
	ctx context.Context

	mu              sync.Mutex
	engine          *stelleengine.StelleEngine
	queue           []TrackInfo
	unshuffledQueue []TrackInfo // Backup of queue before shuffle is turned on
	order           []int       // playback order into queue; identity unless shuffled
	current         int         // index into queue (-1 when nothing selected)
	volume          int
	shuffle         bool
	loopMode        LoopMode

	rpc        *discordrpc.Manager // nil when Rich Presence is disabled
	rpcEnabled bool

	ytmClient  *ytm.Client
	ytdlClient *ytdl.Downloader
	ytdlpReady bool
	config     AppConfig

	playSessionID     uint64
	activeDownloads   map[string]*activeDownload
	activeDownloadsMu sync.Mutex
}

func NewApp() *App {
	// SDL audio init can fail (no audio device, missing libSDL3). A media
	// player is useless without it, so fail fast and consistent with the
	// gui/tui entrypoints rather than running with a dead engine.
	engine, err := stelleengine.NewStelleEngine(1.0)
	if err != nil {
		println("audio engine init failed:", err.Error())
		os.Exit(1)
	}
	app := &App{
		engine:          engine,
		current:         -1,
		volume:          100,
		ytmClient:       ytm.NewClient(),
		activeDownloads: make(map[string]*activeDownload),
	}
	app.loadConfig()
	app.ytdlClient = ytdl.NewDownloader(getExeDir())
	return app
}

func (a *App) startup(ctx context.Context) {
	logInfo("ytm-go version %s", ytm.Version)
	a.ctx = ctx
	if a.engine == nil {
		return
	}
	logInfo("startup (debug logging: %v)", logging.DebugEnabled())
	
	// Create cache folder for streaming
	_ = os.MkdirAll("./cache", 0755)

	go func() {
		logInfo("checking/installing dependencies...")
		err := a.ytdlClient.CheckAndInstall(context.Background(), func(name string, pct float64, done bool) {
			runtime.EventsEmit(ctx, "dependency_progress", map[string]any{
				"name":     name,
				"progress": pct,
				"done":     done,
			})
		})
		if err != nil {
			logError("dependency installation failed: %v", err)
		} else {
			logInfo("dependencies are ready.")
			a.mu.Lock()
			a.ytdlpReady = true
			a.mu.Unlock()
		}
	}()

	a.engine.SetOnComplete(a.onTrackComplete)
	go a.broadcastProgress()
}

func (a *App) shutdown(ctx context.Context) {
	logInfo("shutdown")
	a.mu.Lock()
	rpc := a.rpc
	a.rpc = nil
	a.mu.Unlock()
	if rpc != nil {
		rpc.Stop()
	}
	if a.engine != nil {
		a.engine.Close()
	}
}

func (a *App) broadcastProgress() {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	var lastUnderruns int64
	for range ticker.C {
		if a.ctx == nil || a.engine == nil {
			continue
		}
		if a.engine.GetState() == AudioEngine.StatePlaying {
			runtime.EventsEmit(a.ctx, "playback_progress", map[string]float64{
				"position": a.engine.GetPosition(),
				"duration": a.engine.GetDuration(),
			})

			currentUnderruns := a.engine.GetUnderruns()
			if currentUnderruns > lastUnderruns {
				diff := currentUnderruns - lastUnderruns
				logInfo("audio underrun detected: engine buffer ran dry %d time(s) (total: %d)", diff, currentUnderruns)
				lastUnderruns = currentUnderruns
			}
		} else {
			lastUnderruns = 0
		}
	}
}

/*
scanToTrackInfos scans a folder via MetaResolver and maps the results to
TrackInfo. Cover bytes are intentionally dropped (only HasCover is kept); the
frontend pulls covers lazily through GetCover to keep memory flat.

	params:
	      folder:     directory to scan
	      startIndex: base index assigned to the first track (for appends)
	returns:
	      []TrackInfo
*/
func scanToTrackInfos(folder string, startIndex int) []TrackInfo {
	metas, err := MetaResolver.ScanFolder(folder)
	if err != nil {
		return nil
	}
	out := make([]TrackInfo, 0, len(metas))
	for i, m := range metas {
		out = append(out, TrackInfo{
			Path:     m.Path,
			Name:     filepath.Base(m.Path),
			Index:    startIndex + i,
			Title:    m.Title,
			Artist:   m.Artist,
			Album:    m.Album,
			Format:   m.Format,
			HasCover: len(m.CoverData) > 0,
		})
	}
	return out
}

// rebuildOrderLocked recomputes the playback order as a simple identity order
// because shuffle mode physically shuffles the queue itself. Caller must hold a.mu.
func (a *App) rebuildOrderLocked() {
	n := len(a.queue)
	a.order = make([]int, n)
	for i := range a.order {
		a.order[i] = i
	}
}

// orderPosLocked returns the position of a queue index within order, or -1.
// Caller must hold a.mu.
func (a *App) orderPosLocked(qIndex int) int {
	for i, qi := range a.order {
		if qi == qIndex {
			return i
		}
	}
	return -1
}

// nextIndexLocked resolves the next queue index to play. When auto is true the
// call comes from natural track completion and respects loop mode (LoopOne
// replays, LoopOff stops at the end by returning -1). Manual Next always wraps
// and never replays. Caller must hold a.mu.
func (a *App) nextIndexLocked(auto bool) int {
	n := len(a.order)
	if n == 0 {
		return -1
	}
	if auto && a.loopMode == LoopOne {
		return a.current
	}
	pos := a.orderPosLocked(a.current)
	if pos < 0 {
		return a.order[0]
	}
	if pos >= n-1 {
		if auto && a.loopMode == LoopOff {
			return -1
		}
		return a.order[0]
	}
	return a.order[pos+1]
}

// prevIndexLocked resolves the previous queue index, wrapping at the start.
// Caller must hold a.mu.
func (a *App) prevIndexLocked() int {
	n := len(a.order)
	if n == 0 {
		return -1
	}
	pos := a.orderPosLocked(a.current)
	if pos <= 0 {
		return a.order[n-1]
	}
	return a.order[pos-1]
}

// onTrackComplete is the engine completion callback (fires on the engine's
// monitor goroutine). It advances per loop/shuffle state or signals the
// frontend that playback has ended.
func (a *App) onTrackComplete() {
	a.mu.Lock()
	next := a.nextIndexLocked(true)
	a.mu.Unlock()

	if next < 0 {
		runtime.EventsEmit(a.ctx, "track_completed")
		return
	}
	a.playIndex(next)
}

// ensureYTMSong handles cache lookup and background progressive download for YouTube Music tracks.
func (a *App) ensureYTMSong(songID string) (string, error) {
	cachePath := filepath.Join("cache", songID+".opus")
	readyPath := cachePath + ".ready"

	// Cache hit
	if _, err := os.Stat(readyPath); err == nil {
		if _, errStat := os.Stat(cachePath); errStat == nil {
			logInfo("YTM Playback: Cache hit for %s", songID)
			return cachePath, nil
		}
	}

	// Coordinate concurrent downloads of the same song
	a.activeDownloadsMu.Lock()
	dlState, downloading := a.activeDownloads[songID]
	if downloading {
		a.activeDownloadsMu.Unlock()
		logInfo("YTM Playback: Waiting for existing download of %s...", songID)
		<-dlState.done
		if _, err := os.Stat(readyPath); err == nil {
			return cachePath, nil
		}
		return "", fmt.Errorf("download failed in another thread")
	}

	// We are the one downloading
	ctx, cancel := context.WithCancel(context.Background())
	dlState = &activeDownload{
		done:     make(chan struct{}),
		cancel:   cancel,
		filePath: cachePath,
	}
	a.activeDownloads[songID] = dlState
	a.activeDownloadsMu.Unlock()

	defer func() {
		a.activeDownloadsMu.Lock()
		delete(a.activeDownloads, songID)
		a.activeDownloadsMu.Unlock()
		cancel()
		close(dlState.done)
	}()

	// Cache miss / partial download cleanup
	logInfo("YTM Playback: Cache miss/partial cache for %s. Downloading...", songID)
	_ = os.Remove(readyPath)
	_ = os.Remove(cachePath)

	a.mu.Lock()
	ready := a.ytdlpReady
	a.mu.Unlock()

	if !ready {
		for i := 0; i < 15; i++ {
			time.Sleep(500 * time.Millisecond)
			a.mu.Lock()
			ready = a.ytdlpReady
			a.mu.Unlock()
			if ready {
				break
			}
		}
		if !ready {
			return "", fmt.Errorf("yt-dlp is not ready/installed yet")
		}
	}

	a.mu.Lock()
	quality := a.config.StreamQuality
	codec := a.config.StreamCodec
	a.mu.Unlock()

	errCh := make(chan error, 1)
	go func() {
		err := a.ytdlClient.DownloadSong(ctx, songID, cachePath, quality, codec)
		if err != nil {
			logError("ytdlp download error for %s: %v", songID, err)
			errCh <- err
			return
		}
		
		// Write the ready file to mark caching complete
		fReady, errReady := os.Create(readyPath)
		if errReady == nil {
			fReady.Close()
		}
		logInfo("YTM Playback: Finished caching %s", songID)
		errCh <- nil
	}()

	err := <-errCh
	if err != nil {
		_ = os.Remove(cachePath)
		return "", err
	}
	return cachePath, nil
}

// resolvePlayPath resolves dynamic network/YTM paths to local paths.
func (a *App) resolvePlayPath(path string) (string, error) {
	if strings.HasPrefix(path, "ytm:") {
		songID := strings.TrimPrefix(path, "ytm:")
		return a.ensureYTMSong(songID)
	}
	return path, nil
}

// playIndex starts playback of a queue index and emits track_changed.
func (a *App) playIndex(index int) error {
	a.mu.Lock()
	if index < 0 || index >= len(a.queue) {
		a.mu.Unlock()
		return nil
	}

	a.playSessionID++
	mySessionID := a.playSessionID

	a.current = index
	track := a.queue[index]
	vol := a.volume
	a.mu.Unlock()

	logInfo("play index=%d title=%q session=%d", index, track.Title, mySessionID)

	a.mu.Lock()
	if mySessionID != a.playSessionID {
		a.mu.Unlock()
		logInfo("discarding play request for index=%d (stale before resolve)", index)
		return nil
	}
	a.mu.Unlock()

	// Emit track_loading event for this index so UI shows loader
	runtime.EventsEmit(a.ctx, "track_loading", map[string]any{
		"index": index,
	})

	playPath, err := a.resolvePlayPath(track.Path)
	if err != nil {
		logError("resolve play path failed for %q: %v", track.Path, err)
		runtime.EventsEmit(a.ctx, "track_loading_finished", map[string]any{
			"index": index,
			"error": err.Error(),
		})
		return err
	}

	a.mu.Lock()
	if mySessionID != a.playSessionID {
		a.mu.Unlock()
		logInfo("discarding play request for index=%d (stale after resolve)", index)
		runtime.EventsEmit(a.ctx, "track_loading_finished", map[string]any{
			"index": index,
		})
		return nil
	}
	a.mu.Unlock()

	runtime.EventsEmit(a.ctx, "track_loading_finished", map[string]any{
		"index": index,
	})

	err = a.engine.Play(playPath, 0, vol)
	if err != nil {
		logError("play failed for %q (resolved: %q): %v", track.Path, playPath, err)
		return err
	}
	runtime.EventsEmit(a.ctx, "track_changed", TrackChangedPayload{
		Track:   a.GetCurrentTrack(),
		Playing: true,
	})
	go a.resolveLyrics(track)
	go a.updateRPC(track)
	return nil
}

/*
resolveLyrics loads lyrics for a track on a background goroutine: local .lrc
first (from a "lyrics" folder beside the track), then the lrclib.net API with a
save-back on success. Result is pushed to the frontend via the lyrics_loaded
event, tagged with the track index so the UI can ignore stale results.

	params:
	      track: the track to resolve lyrics for (value copy, goroutine-safe)
*/
func (a *App) resolveLyrics(track TrackInfo) {
	// Debounce to avoid API spam during skipping, and prevent races with track_changed event
	time.Sleep(350 * time.Millisecond)

	a.mu.Lock()
	if a.current < 0 || a.current >= len(a.queue) || a.queue[a.current].Path != track.Path {
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()

	logInfo("lyrics: resolving lyric for %+v", track)
	musicDir := filepath.Dir(track.Path)

	emit := func(lines []lyrics.Line, source string) {
		out := make([]LyricLine, 0, len(lines))
		for _, l := range lines {
			out = append(out, LyricLine{Time: l.Time, Text: l.Text})
		}
		runtime.EventsEmit(a.ctx, "lyrics_loaded", map[string]any{
			"index":  track.Index,
			"path":   track.Path,
			"source": source,
			"lines":  out,
		})
	}

	logDebug("lyrics: resolving for title=%q artist=%q album=%q", track.Title, track.Artist, track.Album)

	if lr, ok := lyrics.LoadFromFile(track.Path, musicDir); ok {
		logDebug("lyrics: loaded %d lines from local file", len(lr.Lines))
		emit(lr.Lines, "local")
		return
	}

	// 1. Prioritize YouTube Music lyrics if LyricsBrowseID is available
	if track.LyricsBrowseID != "" {
		logDebug("lyrics: fetching from YTM using LyricsBrowseID: %s", track.LyricsBrowseID)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		content, err := a.ytmClient.GetSongLyrics(ctx, track.LyricsBrowseID)
		cancel()
		if err == nil && content != "" {
			lines := lyrics.Parse(content)
			logInfo("lyrics: fetched %d lines from YTM for %q", len(lines), track.Title)
			if path, saveErr := lyrics.SaveToFile(track.Path, musicDir, content); saveErr != nil {
				logDebug("lyrics: save failed: %v", saveErr)
			} else {
				logDebug("lyrics: saved to %s", path)
			}
			emit(lines, "ytm")
			return
		}
		logDebug("lyrics: YTM fetch failed or empty, falling back to lrclib")
	}

	if track.Artist == "" || track.Title == "" {
		logDebug("lyrics: skipping API fetch, missing artist or title")
		emit(nil, "none")
		return
	}

	// 2. Fallback to lrclib API
	content, err := lyrics.FetchFromAPI(track.Artist, track.Title, track.Album, a.engine.GetDuration())
	if err != nil || content == "" {
		logDebug("lyrics: API miss for %q: %v", track.Title, err)
		emit(nil, "none")
		return
	}

	lines := lyrics.Parse(content)
	logInfo("lyrics: fetched %d lines from lrclib for %q", len(lines), track.Title)
	if path, saveErr := lyrics.SaveToFile(track.Path, musicDir, content); saveErr != nil {
		logDebug("lyrics: save failed: %v", saveErr)
	} else {
		logDebug("lyrics: saved to %s", path)
	}
	emit(lines, "api")
}

/*
SetRPCEnabled toggles Discord Rich Presence. Starting it spins up the IPC
manager and wires position/pause polling to the engine; stopping it tears the
connection down. Safe to call repeatedly.

	params:
	      on: desired enabled state
*/
func (a *App) SetRPCEnabled(on bool) {
	a.mu.Lock()
	if on == a.rpcEnabled {
		a.mu.Unlock()
		return
	}
	a.rpcEnabled = on
	a.config.RPCEnabled = on
	a.mu.Unlock()
	a.saveConfig()

	a.mu.Lock()
	if !on {
		rpc := a.rpc
		a.rpc = nil
		a.mu.Unlock()
		if rpc != nil {
			rpc.Stop()
		}
		logInfo("discord rpc disabled")
		return
	}

	mgr := discordrpc.New()
	mgr.GetPosition = func() float64 { return a.engine.GetPosition() }
	mgr.IsPaused = func() bool { return a.engine.GetState() == AudioEngine.StatePaused }
	a.rpc = mgr
	track, hasTrack := a.currentTrackLocked()
	a.mu.Unlock()

	mgr.Start()
	logInfo("discord rpc enabled")
	if hasTrack {
		go a.updateRPC(track)
	}
}

// GetRPCEnabled reports whether Discord Rich Presence is active.
func (a *App) GetRPCEnabled() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.rpcEnabled
}

// currentTrackLocked returns a copy of the active track. Caller must hold a.mu.
func (a *App) currentTrackLocked() (TrackInfo, bool) {
	if a.current < 0 || a.current >= len(a.queue) {
		return TrackInfo{}, false
	}
	return a.queue[a.current], true
}

/*
updateRPC pushes the current track to the Discord RPC manager. The embedded
cover is decoded lazily from disk (MetaResolver keeps only raw bytes) so the
presence shows album art. Skipped silently when RPC is disabled.

	params:
	      track: the track to present
*/
func (a *App) updateRPC(track TrackInfo) {
	a.mu.Lock()
	rpc := a.rpc
	a.mu.Unlock()
	if rpc == nil {
		return
	}

	var cover image.Image
	if track.HasCover {
		if meta := MetaResolver.ResolveTrack(track.Path); len(meta.CoverData) > 0 {
			if img, _, err := image.Decode(bytes.NewReader(meta.CoverData)); err == nil {
				cover = img
			}
		}
	}

	rpc.Update(discordrpc.TrackInfo{
		Title:       track.Title,
		Artist:      track.Artist,
		Album:       track.Album,
		Cover:       cover,
		CoverURL:    track.CoverURL,
		DurationSec: a.engine.GetDuration(),
		ElapsedSec:  a.engine.GetPosition(),
		IsPaused:    a.engine.GetState() == AudioEngine.StatePaused,
	})
}

// PickFolder opens a native directory picker and REPLACES the queue with it.
func (a *App) PickFolder() ([]TrackInfo, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Music Folder",
	})
	if err != nil {
		return nil, err
	}
	if dir == "" {
		return a.GetQueue(), nil
	}
	return a.LoadFolder(dir)
}

// PickFolderAppend opens a native directory picker and APPENDS it to the queue.
func (a *App) PickFolderAppend() ([]TrackInfo, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Add Music Folder",
	})
	if err != nil {
		return nil, err
	}
	if dir == "" {
		return a.GetQueue(), nil
	}
	return a.AppendFolder(dir)
}

// LoadFolder scans a directory and replaces the queue with its tracks.
func (a *App) LoadFolder(folder string) ([]TrackInfo, error) {
	tracks := scanToTrackInfos(folder, 0)
	logInfo("load folder %q: %d tracks", folder, len(tracks))
	a.mu.Lock()
	a.queue = tracks
	a.unshuffledQueue = nil
	a.current = -1
	a.rebuildOrderLocked()
	q := a.queue
	runtime.EventsEmit(a.ctx, "queue_changed", a.queue)
	a.mu.Unlock()
	return q, nil
}

// AppendFolder scans a directory and appends its tracks to the current queue.
func (a *App) AppendFolder(folder string) ([]TrackInfo, error) {
	a.mu.Lock()
	start := len(a.queue)
	a.mu.Unlock()

	tracks := scanToTrackInfos(folder, start)
	logInfo("append folder %q: +%d tracks (total %d)", folder, len(tracks), start+len(tracks))

	a.mu.Lock()
	a.queue = append(a.queue, tracks...)
	a.unshuffledQueue = nil
	a.rebuildOrderLocked()
	q := a.queue
	runtime.EventsEmit(a.ctx, "queue_changed", a.queue)
	a.mu.Unlock()
	return q, nil
}

/*
GetCover lazily extracts the embedded cover for a queue index and returns it as
a base64 data URL, or "" if the track has no art.

	params:
	      index: queue index
	returns:
	      string: "data:<mime>;base64,..." or empty
*/
func (a *App) GetCover(index int) string {
	a.mu.Lock()
	if index < 0 || index >= len(a.queue) {
		a.mu.Unlock()
		return ""
	}
	path := a.queue[index].Path
	a.mu.Unlock()

	meta := MetaResolver.ResolveTrack(path)
	if len(meta.CoverData) == 0 {
		return ""
	}
	mime := http.DetectContentType(meta.CoverData)
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(meta.CoverData)
}

// PlayTrack plays a track at the given queue index.
func (a *App) PlayTrack(index int) error {
	return a.playIndex(index)
}

// PlayFile plays a single file by path, replacing the queue with just it.
func (a *App) PlayFile(filePath string) error {
	meta := MetaResolver.ResolveTrack(filePath)
	ti := TrackInfo{
		Path:     filePath,
		Name:     filepath.Base(filePath),
		Index:    0,
		Title:    meta.Title,
		Artist:   meta.Artist,
		Album:    meta.Album,
		Format:   meta.Format,
		HasCover: len(meta.CoverData) > 0,
	}

	a.mu.Lock()
	a.playSessionID++
	a.queue = []TrackInfo{ti}
	a.current = 0
	a.rebuildOrderLocked()
	vol := a.volume
	a.mu.Unlock()

	err := a.engine.Play(filePath, 0, vol)
	if err == nil {
		playPath, errResolve := a.resolvePlayPath(filePath)
		if errResolve == nil && playPath != filePath {
			// If it's a resolved YTM path, play the resolved local cache path instead
			_ = a.engine.Stop()
			err = a.engine.Play(playPath, 0, vol)
		}
	}
	if err == nil {
		runtime.EventsEmit(a.ctx, "track_changed", TrackChangedPayload{
			Track:   a.GetCurrentTrack(),
			Playing: true,
		})
		go a.resolveLyrics(ti)
		go a.updateRPC(ti)
	}
	return err
}

// Pause pauses the audio playback.
func (a *App) Pause() error {
	return a.engine.Pause()
}

// Resume resumes from the current position.
func (a *App) Resume() error {
	a.mu.Lock()
	vol := a.volume
	a.mu.Unlock()
	return a.engine.Resume(a.engine.GetPosition(), vol)
}

// Seek seeks to a specific position in seconds.
func (a *App) Seek(positionSeconds float64) error {
	a.mu.Lock()
	vol := a.volume
	a.mu.Unlock()
	return a.engine.Seek(positionSeconds, vol)
}

// SetVolume sets the playback volume (0-100).
func (a *App) SetVolume(vol int) {
	if vol < 0 {
		vol = 0
	}
	if vol > 100 {
		vol = 100
	}
	a.mu.Lock()
	a.volume = vol
	a.config.Volume = vol
	a.mu.Unlock()
	a.saveConfig()
	a.engine.SetVolume(vol)
}

// GetVolume returns the current volume.
func (a *App) GetVolume() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.volume
}

// SetShuffle toggles shuffle and regenerates the playback order physically.
func (a *App) SetShuffle(on bool) {
	a.mu.Lock()
	if a.shuffle == on {
		a.mu.Unlock()
		return
	}
	a.shuffle = on

	if on {
		// Toggle shuffle ON: Shuffle physically
		if len(a.queue) > 1 {
			// Save current queue as unshuffled backup
			a.unshuffledQueue = make([]TrackInfo, len(a.queue))
			copy(a.unshuffledQueue, a.queue)

			// Extract current track
			var currentTrack TrackInfo
			hasCurrent := false
			if a.current >= 0 && a.current < len(a.queue) {
				currentTrack = a.queue[a.current]
				hasCurrent = true
			}

			// Shuffle the queue
			// Shuffle the entire queue, but keep current track at index 0
			// so that playback continues seamlessly from it.
			var otherTracks []TrackInfo
			for i, t := range a.queue {
				if hasCurrent && i == a.current {
					continue
				}
				otherTracks = append(otherTracks, t)
			}

			// Shuffle other tracks
			rand.Shuffle(len(otherTracks), func(i, j int) {
				otherTracks[i], otherTracks[j] = otherTracks[j], otherTracks[i]
			})

			// Reassemble queue: current track at 0, followed by shuffled tracks
			if hasCurrent {
				a.queue = append([]TrackInfo{currentTrack}, otherTracks...)
				a.current = 0
			} else {
				a.queue = otherTracks
			}

			// Update index field
			for i := range a.queue {
				a.queue[i].Index = i
			}
		}
	} else {
		// Toggle shuffle OFF: Restore original queue
		if len(a.unshuffledQueue) > 0 {
			var currentPath string
			if a.current >= 0 && a.current < len(a.queue) {
				currentPath = a.queue[a.current].Path
			}

			a.queue = make([]TrackInfo, len(a.unshuffledQueue))
			copy(a.queue, a.unshuffledQueue)
			a.unshuffledQueue = nil

			// Find current track in restored queue
			a.current = -1
			if currentPath != "" {
				for i, t := range a.queue {
					if t.Path == currentPath {
						a.current = i
						break
					}
				}
			}

			// Update index field
			for i := range a.queue {
				a.queue[i].Index = i
			}
		}
	}

	a.rebuildOrderLocked()
	runtime.EventsEmit(a.ctx, "queue_changed", a.queue)
	if a.current >= 0 && a.current < len(a.queue) {
		playing := a.engine.GetState() == AudioEngine.StatePlaying
		runtime.EventsEmit(a.ctx, "track_changed", TrackChangedPayload{
			Track:   &a.queue[a.current],
			Playing: playing,
		})
	}
	a.mu.Unlock()
}

// ReorderQueue moves a track from fromIndex to toIndex in the queue.
func (a *App) ReorderQueue(fromIndex, toIndex int) []TrackInfo {
	a.mu.Lock()
	defer a.mu.Unlock()

	if fromIndex < 0 || fromIndex >= len(a.queue) || toIndex < 0 || toIndex >= len(a.queue) || fromIndex == toIndex {
		return a.queue
	}

	// Track current playing track
	var currentTrackPath string
	if a.current >= 0 && a.current < len(a.queue) {
		currentTrackPath = a.queue[a.current].Path
	}

	track := a.queue[fromIndex]
	// Remove from queue
	a.queue = append(a.queue[:fromIndex], a.queue[fromIndex+1:]...)
	// Insert at new position
	a.queue = append(a.queue[:toIndex], append([]TrackInfo{track}, a.queue[toIndex:]...)...)

	// Update index field
	for i := range a.queue {
		a.queue[i].Index = i
	}

	// Update current index
	if currentTrackPath != "" {
		a.current = -1
		for i, t := range a.queue {
			if t.Path == currentTrackPath {
				a.current = i
				break
			}
		}
	}

	a.rebuildOrderLocked()
	// Clear backup queue since structure changed
	a.unshuffledQueue = nil

	runtime.EventsEmit(a.ctx, "queue_changed", a.queue)
	if a.current >= 0 && a.current < len(a.queue) {
		playing := a.engine.GetState() == AudioEngine.StatePlaying
		runtime.EventsEmit(a.ctx, "track_changed", TrackChangedPayload{
			Track:   &a.queue[a.current],
			Playing: playing,
		})
	}
	return a.queue
}

// RemoveFromQueue deletes a track from the queue by index.
func (a *App) RemoveFromQueue(index int) []TrackInfo {
	a.mu.Lock()
	defer a.mu.Unlock()

	if index < 0 || index >= len(a.queue) {
		return a.queue
	}

	wasPlaying := false
	if a.current == index {
		wasPlaying = (a.engine.GetState() == AudioEngine.StatePlaying)
		_ = a.engine.Stop()
	}

	a.queue = append(a.queue[:index], a.queue[index+1:]...)

	// Update index field
	for i := range a.queue {
		a.queue[i].Index = i
	}

	playStarted := false
	if a.current == index {
		if len(a.queue) == 0 {
			a.current = -1
		} else {
			// Stay at same index, which is now the next song
			if a.current >= len(a.queue) {
				a.current = 0
			}
			if wasPlaying {
				// We unlock to call playIndex, because playIndex locks a.mu
				a.mu.Unlock()
				_ = a.playIndex(a.current)
				a.mu.Lock()
				playStarted = true
			}
		}
	} else if a.current > index {
		a.current--
	}

	a.rebuildOrderLocked()
	// Clear backup queue since structure changed
	a.unshuffledQueue = nil

	runtime.EventsEmit(a.ctx, "queue_changed", a.queue)
	if !playStarted {
		if a.current >= 0 && a.current < len(a.queue) {
			playing := a.engine.GetState() == AudioEngine.StatePlaying
			runtime.EventsEmit(a.ctx, "track_changed", TrackChangedPayload{
				Track:   &a.queue[a.current],
				Playing: playing,
			})
		} else {
			runtime.EventsEmit(a.ctx, "track_changed", TrackChangedPayload{
				Track:   nil,
				Playing: false,
			})
		}
	}
	return a.queue
}

// ClearQueue clears all tracks from the queue.
func (a *App) ClearQueue() []TrackInfo {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.playSessionID++
	_ = a.engine.Stop()
	a.queue = nil
	a.unshuffledQueue = nil
	a.current = -1
	a.rebuildOrderLocked()

	runtime.EventsEmit(a.ctx, "queue_changed", a.queue)
	runtime.EventsEmit(a.ctx, "track_changed", TrackChangedPayload{
		Track:   nil,
		Playing: false,
	})
	return a.queue
}

// PlayYTMSong appends a YTM song to the queue and starts playing it.
func (a *App) PlayYTMSong(songID, title, artist, album, lyricsBrowseID, coverURL string) error {
	a.mu.Lock()

	// Create TrackInfo
	track := TrackInfo{
		Path:           "ytm:" + songID,
		Name:           title,
		Index:          len(a.queue),
		Title:          title,
		Artist:         artist,
		Album:          album,
		Format:         "YTM",
		HasCover:       true,
		LyricsBrowseID: lyricsBrowseID,
		YTMSongID:      songID,
		CoverURL:       coverURL,
	}

	insertIndex := len(a.queue)
	a.queue = append(a.queue, track)
	a.rebuildOrderLocked()

	// Clear backup queue since queue structure changed
	a.unshuffledQueue = nil

	runtime.EventsEmit(a.ctx, "queue_changed", a.queue)
	a.mu.Unlock()

	return a.playIndex(insertIndex)
}

// InsertYTMSongAt inserts a YTM song at the specified queue index without playing it.
func (a *App) InsertYTMSongAt(index int, songID, title, artist, album, lyricsBrowseID, coverURL string) ([]TrackInfo, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if index < 0 {
		index = 0
	}
	if index > len(a.queue) {
		index = len(a.queue)
	}

	track := TrackInfo{
		Path:           "ytm:" + songID,
		Name:           title,
		Index:          index,
		Title:          title,
		Artist:         artist,
		Album:          album,
		Format:         "YTM",
		HasCover:       true,
		LyricsBrowseID: lyricsBrowseID,
		YTMSongID:      songID,
		CoverURL:       coverURL,
	}

	// Insert track into a.queue at index
	if index == len(a.queue) {
		a.queue = append(a.queue, track)
	} else {
		a.queue = append(a.queue[:index], append([]TrackInfo{track}, a.queue[index:]...)...)
	}

	// Update index field and current track index
	for i := range a.queue {
		a.queue[i].Index = i
	}
	if a.current >= index {
		a.current++
	}

	a.rebuildOrderLocked()
	a.unshuffledQueue = nil

	runtime.EventsEmit(a.ctx, "queue_changed", a.queue)
	return a.queue, nil
}

// GetShuffle returns whether shuffle is enabled.
func (a *App) GetShuffle() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.shuffle
}

// SetLoopMode sets the loop mode (0 off, 1 all, 2 one).
func (a *App) SetLoopMode(mode int) {
	a.mu.Lock()
	a.loopMode = LoopMode(mode)
	a.mu.Unlock()
}

// GetLoopMode returns the current loop mode (0 off, 1 all, 2 one).
func (a *App) GetLoopMode() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return int(a.loopMode)
}

// Next plays the next track. Manual advance always wraps and never replays.
func (a *App) Next() error {
	a.mu.Lock()
	next := a.nextIndexLocked(false)
	a.mu.Unlock()
	if next < 0 {
		return nil
	}
	return a.playIndex(next)
}

// Prev plays the previous track, wrapping at the start.
func (a *App) Prev() error {
	a.mu.Lock()
	prev := a.prevIndexLocked()
	a.mu.Unlock()
	if prev < 0 {
		return nil
	}
	return a.playIndex(prev)
}

// GetCurrentTrack returns a copy of the currently playing track, or nil.
func (a *App) GetCurrentTrack() *TrackInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.current < 0 || a.current >= len(a.queue) {
		return nil
	}
	t := a.queue[a.current]
	return &t
}

// GetQueue returns the full track queue.
func (a *App) GetQueue() []TrackInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.queue
}

// SearchYTM searches for music on YouTube Music.
func (a *App) SearchYTM(query string, params string) (*ytm.SearchResults, error) {
	logInfo("YTM Search: %q (params=%q)", query, params)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	results, err := a.ytmClient.Search(ctx, query, params, false)
	if results != nil {
		var chipNames []string
		for _, ch := range results.Chips {
			chipNames = append(chipNames, ch.Name)
		}
		catCount := len(results.Categories)
		totalItems := 0
		for _, cat := range results.Categories {
			totalItems += len(cat.Layout.Items)
		}
		logInfo("search: %d categories, %d items total, chips=%v", catCount, totalItems, chipNames)
	}
	return results, err
}

// GetYTMSuggestions fetches real-time autocomplete suggestions from YouTube Music.
func (a *App) GetYTMSuggestions(query string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	suggestions, err := a.ytmClient.GetSearchSuggestions(ctx, query)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(suggestions))
	for i, s := range suggestions {
		out[i] = s.Query
	}
	return out, nil
}

// GetYTMArtist retrieves detail layout for an artist.
func (a *App) GetYTMArtist(artistID string) (*ytm.Artist, error) {
	logInfo("YTM LoadArtist: %s", artistID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return a.ytmClient.LoadArtist(ctx, artistID)
}

// GetYTMPlaylist loads a playlist or album.
func (a *App) GetYTMPlaylist(playlistID string) (*ytm.Playlist, error) {
	logInfo("YTM LoadPlaylist: %s", playlistID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	p, err := a.ytmClient.LoadPlaylist(ctx, playlistID, nil, nil, nil, false)
	if err != nil && strings.HasPrefix(playlistID, "VL") {
		p, err = a.ytmClient.LoadPlaylist(ctx, playlistID, nil, nil, nil, true)
	}
	return p, err
}

// GetYTMRadio retrieves auto-mix recommended tracks for a song and formats them as TrackInfo.
func (a *App) GetYTMRadio(songID string) ([]TrackInfo, error) {
	logInfo("YTM GetSongRadio for songID: %s", songID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	radio, err := a.ytmClient.GetSongRadio(ctx, songID, nil, nil)
	if err != nil {
		return nil, err
	}
	
	out := make([]TrackInfo, 0, len(radio.Items))
	for i, s := range radio.Items {
		var artists []string
		for _, art := range s.Artists {
			artists = append(artists, art.Name)
		}
		artistStr := strings.Join(artists, ", ")
		
		albumName := ""
		if s.Album != nil {
			albumName = s.Album.Name
		}
		
		var coverURL string
		if s.Thumbnail != nil {
			coverURL = s.Thumbnail.GetThumbnailURL(ytm.ThumbnailQualityHigh)
		}

		out = append(out, TrackInfo{
			Path:           "ytm:" + s.ID,
			Name:           s.Name,
			Index:          i,
			Title:          s.Name,
			Artist:         artistStr,
			Album:          albumName,
			Format:         "YTM",
			HasCover:       true,
			LyricsBrowseID: s.LyricsBrowseID,
			YTMSongID:      s.ID,
			CoverURL:       coverURL,
		})
	}
	return out, nil
}

// SearchContinuationResult holds the next page of search items and the next continuation token.
type SearchContinuationResult struct {
	Items        []ytm.MediaItem `json:"items"`
	NextToken    string          `json:"nextToken,omitempty"`
}

// SearchYTMMore loads the next page of search results for a continuation token.
func (a *App) SearchYTMMore(continuation string) (*SearchContinuationResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	items, nextToken, err := a.ytmClient.GetSearchContinuation(ctx, continuation)
	if err != nil {
		return nil, err
	}
	return &SearchContinuationResult{Items: items, NextToken: nextToken}, nil
}

// SearchYTMViewMore loads all items for a browse ID (typically from a carousel's viewMore).
func (a *App) SearchYTMViewMore(browseID string) ([]ytm.MediaItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return a.ytmClient.GetGenericFeedViewMore(ctx, browseID)
}

// GetYTMSongLyrics retrieves song lyrics in plaintext.
func (a *App) GetYTMSongLyrics(lyricsBrowseID string) (string, error) {
	logInfo("YTM GetSongLyrics for ID: %s", lyricsBrowseID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return a.ytmClient.GetSongLyrics(ctx, lyricsBrowseID)
}

func (a *App) CancelAllDownloads() {
	a.activeDownloadsMu.Lock()
	defer a.activeDownloadsMu.Unlock()

	logInfo("gracefully cancelling %d active download(s)...", len(a.activeDownloads))
	for _, dlState := range a.activeDownloads {
		dlState.cancel()
		_ = os.Remove(dlState.filePath)
	}
}

func (a *App) GetConfig() AppConfig {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.config
}

func (a *App) UpdateConfig(cfg AppConfig) {
	a.mu.Lock()
	a.config = cfg
	a.volume = cfg.Volume
	a.rpcEnabled = cfg.RPCEnabled
	a.mu.Unlock()
	
	a.saveConfig()
	
	if a.engine != nil {
		a.engine.SetVolume(cfg.Volume)
	}
	a.SetRPCEnabled(cfg.RPCEnabled)
}

func getExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func (a *App) loadConfig() {
	exeDir := getExeDir()
	configPath := filepath.Join(exeDir, "config.json")
	
	a.config = AppConfig{
		StreamQuality: "0",
		StreamCodec:   "opus",
		RPCEnabled:    false,
		Volume:        100,
	}
	
	data, err := os.ReadFile(configPath)
	if err == nil {
		var loaded AppConfig
		if errUnmarshal := json.Unmarshal(data, &loaded); errUnmarshal == nil {
			if loaded.StreamQuality != "" {
				a.config.StreamQuality = loaded.StreamQuality
			}
			if loaded.StreamCodec != "" {
				a.config.StreamCodec = loaded.StreamCodec
			}
			a.config.RPCEnabled = loaded.RPCEnabled
			if loaded.Volume > 0 && loaded.Volume <= 100 {
				a.config.Volume = loaded.Volume
			}
		}
	}
	
	a.volume = a.config.Volume
	a.rpcEnabled = a.config.RPCEnabled
}

func (a *App) saveConfig() {
	exeDir := getExeDir()
	configPath := filepath.Join(exeDir, "config.json")
	
	data, err := json.MarshalIndent(a.config, "", "  ")
	if err == nil {
		_ = os.WriteFile(configPath, data, 0644)
	}
}

