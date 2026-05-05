// cmd/webui/app.go
// Wails application bridge between StelleEngine and the Svelte frontend.
// Exposes playback controls, queue management, and folder loading as
// callable JavaScript functions via Wails bindings.
//
// Dependencies:
//   - Audioengine: Engine interface and PlaybackState enum
//   - StelleEngine: SDL3-based audio backend
//   - wails/v2/pkg/runtime: Wails event system and dialog API

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	AudioEngine "github.com/dlcuy22/OngoPlayer/Audioengine"
	stelleengine "github.com/dlcuy22/OngoPlayer/Audioengine/StelleEngine"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type TrackInfo struct {
	Path   string `json:"path"`
	Name   string `json:"name"`
	Index  int    `json:"index"`
}

type App struct {
	ctx     context.Context
	engine  *stelleengine.StelleEngine
	queue   []TrackInfo
	current int
	volume  int
}

func NewApp() *App {
	return &App{
		engine:  stelleengine.NewStelleEngine(1.0),
		current: -1,
		volume:  100,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	a.engine.SetOnComplete(func() {
		runtime.EventsEmit(a.ctx, "track_completed")
	})

	go a.broadcastProgress()
}

func (a *App) broadcastProgress() {
	ticker := time.NewTicker(200 * time.Millisecond)
	for range ticker.C {
		if a.ctx == nil {
			continue
		}

		if a.engine.GetState() == AudioEngine.StatePlaying {
			progress := map[string]float64{
				"position": a.engine.GetPosition(),
				"duration": a.engine.GetDuration(),
			}
			runtime.EventsEmit(a.ctx, "playback_progress", progress)
		}
	}
}

// PickFolder opens a native OS directory picker and loads audio files from it
func (a *App) PickFolder() ([]TrackInfo, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Music Folder",
	})
	if err != nil {
		return nil, err
	}
	if dir == "" {
		return a.queue, nil
	}

	return a.LoadFolder(dir)
}

// LoadFolder scans a directory for audio files and populates the queue
func (a *App) LoadFolder(folder string) ([]TrackInfo, error) {
	exts := map[string]bool{".opus": true, ".mp3": true, ".ogg": true, ".oga": true, ".flac": true}

	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, err
	}

	a.queue = nil
	idx := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !exts[ext] {
			continue
		}

		a.queue = append(a.queue, TrackInfo{
			Path:  filepath.Join(folder, e.Name()),
			Name:  e.Name(),
			Index: idx,
		})
		idx++
	}

	return a.queue, nil
}

// PlayTrack plays a track at the given queue index
func (a *App) PlayTrack(index int) error {
	if index < 0 || index >= len(a.queue) {
		return nil
	}
	a.current = index
	track := a.queue[index]

	a.engine.SetOnComplete(func() {
		a.Next()
		runtime.EventsEmit(a.ctx, "track_changed", a.GetCurrentTrack())
	})

	err := a.engine.Play(track.Path, 0, a.volume)
	if err == nil {
		runtime.EventsEmit(a.ctx, "track_changed", a.GetCurrentTrack())
	}
	return err
}

// PlayFile plays a single file by path (for the text input test)
func (a *App) PlayFile(filePath string) error {
	a.queue = []TrackInfo{{Path: filePath, Name: filepath.Base(filePath), Index: 0}}
	a.current = 0

	a.engine.SetOnComplete(func() {
		runtime.EventsEmit(a.ctx, "track_completed")
	})

	return a.engine.Play(filePath, 0, a.volume)
}

// Pause pauses the audio playback
func (a *App) Pause() error {
	return a.engine.Pause()
}

// Resume resumes from the current position (matching the Gio TogglePause pattern)
func (a *App) Resume() error {
	return a.engine.Resume(a.engine.GetPosition(), a.volume)
}

// Seek seeks to a specific position in seconds
func (a *App) Seek(positionSeconds float64) error {
	return a.engine.Seek(positionSeconds, a.volume)
}

// SetVolume sets the playback volume (0-100)
func (a *App) SetVolume(vol int) {
	if vol < 0 {
		vol = 0
	}
	if vol > 100 {
		vol = 100
	}
	a.volume = vol
	a.engine.SetVolume(vol)
}

// GetVolume returns the current volume
func (a *App) GetVolume() int {
	return a.volume
}

// Next plays the next track in the queue
func (a *App) Next() error {
	if len(a.queue) == 0 {
		return nil
	}
	next := a.current + 1
	if next >= len(a.queue) {
		next = 0
	}
	return a.PlayTrack(next)
}

// Prev plays the previous track in the queue
func (a *App) Prev() error {
	if len(a.queue) == 0 {
		return nil
	}
	prev := a.current - 1
	if prev < 0 {
		prev = len(a.queue) - 1
	}
	return a.PlayTrack(prev)
}

// GetCurrentTrack returns info about the currently playing track
func (a *App) GetCurrentTrack() *TrackInfo {
	if a.current < 0 || a.current >= len(a.queue) {
		return nil
	}
	t := a.queue[a.current]
	return &t
}

// GetQueue returns the full track queue
func (a *App) GetQueue() []TrackInfo {
	return a.queue
}
