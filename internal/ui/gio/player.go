// internal/ui/gio/player.go
// Player state management. Bridges the StelleEngine with the Gio UI layer.
// Tracks the queue, current index, metadata, cover art, and playback position.
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
	"sync"

	"github.com/dhowden/tag"
	"github.com/dlcuy22/OngoPlayer/internal/shared"
	stelleengine "github.com/dlcuy22/OngoPlayer/Audioengine/StelleEngine"
	"golang.org/x/image/draw"
)

type TrackMeta struct {
	Path   string
	Title  string
	Artist string
	Album  string
	Cover  image.Image
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
NewPlayer creates a new player instance with the given engine and volume.

	params:
	      engine: StelleEngine instance
	      volume: initial volume (0-100)
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
LoadFolder scans a folder for audio files and extracts metadata for each.

	params:
	      folder: path to the music folder
	returns:
	      error
*/
func (p *Player) LoadFolder(folder string) error {
	exts := map[string]bool{".opus": true, ".mp3": true, ".ogg": true, ".oga": true}

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
				if t := m.Title(); t != "" {
					meta.Title = t
				}
				meta.Artist = m.Artist()
				meta.Album = m.Album()

				if pic := m.Picture(); pic != nil {
					if img, _, err := image.Decode(bytes.NewReader(pic.Data)); err == nil {
						meta.Cover = thumbnailCover(img, 96)
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
PlayTrack starts playing the track at the given index.

	params:
	      index: queue index
*/
func (p *Player) PlayTrack(index int) {
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
Next advances to the next track in the queue.
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
Prev goes back to the previous track in the queue.
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
TogglePause pauses or resumes playback.
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
CurrentTrack returns the metadata of the currently playing track.

	returns:
	      *TrackMeta: nil if nothing is playing
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
thumbnailCover resizes a cover image to a square thumbnail.
Uses nearest-neighbor for speed since these are small display thumbnails.

	params:
	      src:  original image
	      size: target width and height in pixels
	returns:
	      *image.RGBA
*/
func thumbnailCover(src image.Image, size int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}
