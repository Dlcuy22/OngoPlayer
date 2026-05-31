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
	_ "os"
	"sync"

	_ "github.com/dhowden/tag"
	"github.com/dlcuy22/OngoPlayer/Audioengine/MetaResolver"
	stelleengine "github.com/dlcuy22/OngoPlayer/Audioengine/StelleEngine"
	"github.com/dlcuy22/OngoPlayer/internal/shared"
	"golang.org/x/image/draw"
)

type TrackMeta struct {
	MetaResolver.TrackMeta
	Thumb image.Image // 48px sidebar thumbnail
	Cover image.Image // 256px main panel art
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
Delegates metadata resolution to the shared MetaResolver package, then
generates Gio-specific image thumbnails from the raw cover bytes.

	params:
	      folder: Path to directory
	returns:
	      error: Returns on decoding errors or missing paths
*/
func (p *Player) LoadFolder(folder string) error {
	metas, err := MetaResolver.ScanFolder(folder)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.Queue = nil
	p.MusicDir = folder
	for _, m := range metas {
		track := TrackMeta{TrackMeta: m}

		// Generate 48px thumbnail from raw cover bytes
		if len(m.CoverData) > 0 {
			if img, _, err := image.Decode(bytes.NewReader(m.CoverData)); err == nil {
				track.Thumb = resizeCover(img, 48)
			}
		}

		p.Queue = append(p.Queue, track)
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

	// Decode 256px cover from raw CoverData bytes
	meta := &p.Queue[targetIdx]
	if len(meta.CoverData) > 0 {
		if img, _, err := image.Decode(bytes.NewReader(meta.CoverData)); err == nil {
			meta.Cover = resizeCover(img, 256)
		}
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
