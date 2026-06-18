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
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	AudioEngine "github.com/dlcuy22/OngoPlayer/Audioengine"
	"github.com/dlcuy22/OngoPlayer/Audioengine/MetaResolver"
	stelleengine "github.com/dlcuy22/OngoPlayer/Audioengine/StelleEngine"
	"github.com/dlcuy22/OngoPlayer/internal/service/discordrpc"
	"github.com/dlcuy22/OngoPlayer/internal/service/lyrics"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// debugEnabled gates verbose logging; set ONGO_DEBUG=1 (any non-empty value).
var debugEnabled = os.Getenv("ONGO_DEBUG") != ""

// logInfo writes an always-on log line tagged for the webui subsystem.
func logInfo(format string, args ...any) {
	log.Printf("[webui] "+format, args...)
}

// logDebug writes a log line only when ONGO_DEBUG is set.
func logDebug(format string, args ...any) {
	if debugEnabled {
		log.Printf("[webui][debug] "+format, args...)
	}
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
	Path     string `json:"path"`
	Name     string `json:"name"`
	Index    int    `json:"index"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	Format   string `json:"format"`
	HasCover bool   `json:"hasCover"`
}

type App struct {
	ctx context.Context

	mu       sync.Mutex
	engine   *stelleengine.StelleEngine
	queue    []TrackInfo
	order    []int // playback order into queue; identity unless shuffled
	current  int   // index into queue (-1 when nothing selected)
	volume   int
	shuffle  bool
	loopMode LoopMode

	rpc        *discordrpc.Manager // nil when Rich Presence is disabled
	rpcEnabled bool
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
	return &App{
		engine:  engine,
		current: -1,
		volume:  100,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.engine == nil {
		return
	}
	logInfo("startup (debug logging: %v)", debugEnabled)
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

// rebuildOrderLocked recomputes the playback order. Identity unless shuffle is
// on, in which case it is a random permutation with the current track moved to
// the front so playback continues from where it is. Caller must hold a.mu.
func (a *App) rebuildOrderLocked() {
	n := len(a.queue)
	a.order = make([]int, n)
	for i := range a.order {
		a.order[i] = i
	}
	if a.shuffle && n > 1 {
		rand.Shuffle(n, func(i, j int) {
			a.order[i], a.order[j] = a.order[j], a.order[i]
		})
		if a.current >= 0 {
			for i, qi := range a.order {
				if qi == a.current {
					a.order[0], a.order[i] = a.order[i], a.order[0]
					break
				}
			}
		}
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

// playIndex starts playback of a queue index and emits track_changed.
func (a *App) playIndex(index int) error {
	a.mu.Lock()
	if index < 0 || index >= len(a.queue) {
		a.mu.Unlock()
		return nil
	}
	a.current = index
	track := a.queue[index]
	vol := a.volume
	a.mu.Unlock()

	logInfo("play index=%d title=%q", index, track.Title)
	err := a.engine.Play(track.Path, 0, vol)
	if err != nil {
		logInfo("play failed for %q: %v", track.Path, err)
		return err
	}
	runtime.EventsEmit(a.ctx, "track_changed", a.GetCurrentTrack())
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
	musicDir := filepath.Dir(track.Path)

	emit := func(lines []lyrics.Line, source string) {
		out := make([]LyricLine, 0, len(lines))
		for _, l := range lines {
			out = append(out, LyricLine{Time: l.Time, Text: l.Text})
		}
		runtime.EventsEmit(a.ctx, "lyrics_loaded", map[string]any{
			"index":  track.Index,
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

	if track.Artist == "" || track.Title == "" {
		logDebug("lyrics: skipping API fetch, missing artist or title")
		emit(nil, "none")
		return
	}

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
	a.current = -1
	a.rebuildOrderLocked()
	q := a.queue
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
	a.rebuildOrderLocked()
	q := a.queue
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
	a.queue = []TrackInfo{ti}
	a.current = 0
	a.rebuildOrderLocked()
	vol := a.volume
	a.mu.Unlock()

	err := a.engine.Play(filePath, 0, vol)
	if err == nil {
		runtime.EventsEmit(a.ctx, "track_changed", a.GetCurrentTrack())
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
	a.mu.Unlock()
	a.engine.SetVolume(vol)
}

// GetVolume returns the current volume.
func (a *App) GetVolume() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.volume
}

// SetShuffle toggles shuffle and regenerates the playback order.
func (a *App) SetShuffle(on bool) {
	a.mu.Lock()
	a.shuffle = on
	a.rebuildOrderLocked()
	a.mu.Unlock()
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
