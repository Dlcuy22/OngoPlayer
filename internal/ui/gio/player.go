// internal/ui/gio/player.go
// Player state management. Bridges the StelleEngine with the Gio UI layer.
// Tracks the queue, current index, metadata, cover art, and playback position.
//
// Cover images are stored at two resolutions:
//   - Thumb (48px): compact sidebar thumbnails
//   - Cover (256px): main panel album art display
//
// Dependencies:
//   - StelleEngine: audio playback
//   - dhowden/tag: metadata extraction

package gio

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dhowden/tag"
	stelleengine "github.com/dlcuy22/OngoPlayer/Audioengine/StelleEngine"
	"github.com/dlcuy22/OngoPlayer/internal/shared"
	"golang.org/x/image/draw"
)

type TrackMeta struct {
	Path   string
	Title  string
	Artist string
	Album  string
	Thumb  image.Image // 48px sidebar thumbnail
	Cover  image.Image // 256px main panel art
	Format string
}

type Player struct {
	mu sync.Mutex

	Engine        *stelleengine.StelleEngine
	Queue         []TrackMeta
	Current       int
	Volume        int
	MusicDir      string
	OnUpdate      func()
	OnTrackChange func(track TrackMeta)
}

/*
NewPlayer creates an instance of Player and initializes its fields.

	params:
	      engine: Audio engine reference
	      volume: Initial volume level (0-100)
	returns:
	      *Player
*/
func NewPlayer(engine *stelleengine.StelleEngine, volume int) *Player {
	return &Player{
		Engine:  engine,
		Volume:  volume,
		Current: -1,
	}
}

/*
LoadFolder scans a local directory for audio files and populates the Queue.

	params:
	      folder: Path to directory
	returns:
	      error: Returns on decoding errors or missing paths
*/
func (p *Player) LoadFolder(folder string) error {
	exts := map[string]bool{".opus": true, ".mp3": true, ".ogg": true, ".oga": true, ".flac": true}

	entries, err := os.ReadDir(folder)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.Queue = nil
	p.MusicDir = folder
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if !exts[ext] {
			continue
		}

		fullPath := filepath.Join(folder, e.Name())
		meta := TrackMeta{
			Path:  fullPath,
			Title: e.Name(),
		}

		if f, err := os.Open(fullPath); err == nil {
			if m, err := tag.ReadFrom(f); err == nil {
				meta.Format = string(m.Format())
				if strings.ToLower(ext) == ".opus" {
					meta.Format = "OPUS"
				} else if strings.ToLower(ext) == ".mp3" {
					meta.Format = "MP3"
				} else if strings.ToLower(ext) == ".flac" {
					meta.Format = "FLAC"
				} else if strings.ToLower(ext) == ".ogg" || strings.ToLower(ext) == ".oga" {
					meta.Format = "OGG"
				} else {
					meta.Format = "unknown"
				}
				if t := m.Title(); t != "" {
					meta.Title = t
				}
				meta.Artist = m.Artist()
				meta.Album = m.Album()

				if pic := m.Picture(); pic != nil {
					if img, _, err := image.Decode(bytes.NewReader(pic.Data)); err == nil {
						meta.Thumb = resizeCover(img, 48)
						// The 256px cover art is lazy-loaded in PlayTrack to save RAM
						// and is intentionally excluded here.
					}
				}
			}
			f.Close()
		}

		p.Queue = append(p.Queue, meta)
	}

	return nil
}

/*
PlayTrack seeks to the specified track index and begins playback.
It also lazy-loads the track's cover art and clears previous covers to save RAM.

	params:
	      index: The queue index to play
*/
func (p *Player) PlayTrack(index int) {
	p.populateCover(index)

	p.mu.Lock()
	if index < 0 || index >= len(p.Queue) {
		p.mu.Unlock()
		return
	}
	p.Current = index
	track := p.Queue[index]
	p.mu.Unlock()

	p.Engine.SetOnComplete(func() {
		p.Next()
	})

	_ = p.Engine.Play(track.Path, 0, p.Volume)

	if shared.Debug {
		fmt.Printf("[DEBUG][player] PlayTrack(%d): %q by %q\n", index, track.Title, track.Artist)
	}

	if p.OnTrackChange != nil {
		p.OnTrackChange(track)
	}

	if p.OnUpdate != nil {
		p.OnUpdate()
	}
}

/*
SetVolume changes the playback volume.

	params:
	      volume: Target volume level (0-100)
*/
func (p *Player) SetVolume(volume int) {
	p.mu.Lock()
	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}
	p.Volume = volume
	p.mu.Unlock()

	p.Engine.SetVolume(volume)

	if p.OnUpdate != nil {
		p.OnUpdate()
	}

}

/*
populateCover lazy-loads the 256x256 cover art for the given index,
and aggressively sets all other loaded covers to nil to free memory.
*/
func (p *Player) populateCover(targetIdx int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Clear all other covers to free up RAM footprint immediately
	for i := range p.Queue {
		if i != targetIdx {
			p.Queue[i].Cover = nil
		}
	}

	if targetIdx < 0 || targetIdx >= len(p.Queue) {
		return
	}

	// Already loaded? Return early
	if p.Queue[targetIdx].Cover != nil {
		return
	}

	// Lazy load tag.Picture
	meta := &p.Queue[targetIdx]
	if f, err := os.Open(meta.Path); err == nil {
		if m, err := tag.ReadFrom(f); err == nil {
			if pic := m.Picture(); pic != nil {
				if img, _, err := image.Decode(bytes.NewReader(pic.Data)); err == nil {
					meta.Cover = resizeCover(img, 256)
				}
			}
		}
		f.Close()
	}
}

/*
Next advances playback to the next track in the queue, wrapping around.
*/
func (p *Player) Next() {
	p.mu.Lock()
	next := p.Current + 1
	if next >= len(p.Queue) {
		next = 0
	}
	p.mu.Unlock()
	p.PlayTrack(next)
}

/*
Prev reverses playback to the previous track in the queue, wrapping around.
*/
func (p *Player) Prev() {
	p.mu.Lock()
	prev := p.Current - 1
	if prev < 0 {
		prev = len(p.Queue) - 1
	}
	p.mu.Unlock()
	p.PlayTrack(prev)
}

/*
TogglePause switches between playing and paused state.
*/
func (p *Player) TogglePause() {
	state := p.Engine.GetState()
	if state == 1 { // Playing
		_ = p.Engine.Pause()
	} else if state == 2 { // Paused
		_ = p.Engine.Resume(p.Engine.GetPosition(), p.Volume)
	}
}

/*
CurrentTrack returns the metadata for the currently active track.

	returns:
	      *TrackMeta: pointer to the track data, or nil if invalid index
*/
func (p *Player) CurrentTrack() *TrackMeta {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Current < 0 || p.Current >= len(p.Queue) {
		return nil
	}
	t := p.Queue[p.Current]
	return &t
}

/*
resizeCover scales a cover image to a square thumbnail using bilinear interpolation.

	params:
	      src: original image
	      size: target width and height in pixels
	returns:
	      *image.NRGBA: GPU-efficient representation for GioUI
*/
func resizeCover(src image.Image, size int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	draw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}
