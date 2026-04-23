// AudioEngine/StelleEngine/stelle.go
// Native Go audio engine implementation using SDL3.
// This file manages playback, state, and decoder selection.
//
// Types:
//   - StelleEngine: implements AudioEngine.Engine interface
//
// Functions:
//   - NewStelleEngine: creates a new Stelle engine instance

package stelleengine

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	AudioEngine "github.com/dlcuy22/OngoPlayer/Audioengine"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

// StelleEngine implements the AudioEngine.Engine interface using native Go + SDL3.
type StelleEngine struct {
	mu         sync.Mutex
	state      AudioEngine.PlaybackState
	stream     *sdl.AudioStream
	volume     float32
	src        *AudioSource
	onComplete func()
	decoders   []Decoder
	stopCh     chan struct{}
}

// NewStelleEngine creates a new Stelle-based audio engine.
func NewStelleEngine() *StelleEngine {
	if !sdl.Init(sdl.InitAudio) {
		panic(sdl.GetError())
	}

	return &StelleEngine{
		state:    AudioEngine.StateStopped,
		volume:   0.3,
		decoders: []Decoder{NewVorbisDecoder(), NewMp3Decoder(), NewOpusDecoder()},
		stopCh:   make(chan struct{}),
	}
}

// findDecoders returns all decoders that can handle the given file extension.
// Multiple decoders may match (e.g., both Vorbis and Opus handle .ogg).
func (e *StelleEngine) findDecoders(filePath string) []Decoder {
	ext := filepath.Ext(filePath)
	var matches []Decoder
	for _, d := range e.decoders {
		if d.CanHandle(ext) {
			matches = append(matches, d)
		}
	}
	return matches
}

// audioCallback is the SDL audio callback that feeds samples to the audio device.
//
//export audioCallback
func audioCallback(userdata unsafe.Pointer, stream *sdl.AudioStream, additionalAmount, _ int32) {
	if additionalAmount <= 0 {
		return
	}

	src := (*AudioSource)(userdata)
	if src == nil {
		// Output silence if no source
		nFrames := int(additionalAmount) / (4 * DefaultChannels)
		silence := make([]float32, nFrames*DefaultChannels)
		if len(silence) > 0 {
			ptr := (*uint8)(unsafe.Pointer(&silence[0]))
			sdl.PutAudioStreamData(stream, ptr, int32(len(silence)*4))
		}
		return
	}

	nFrames := int(additionalAmount) / (4 * DefaultChannels)
	if nFrames <= 0 {
		return
	}

	outBuf := make([]float32, nFrames*DefaultChannels)

	src.mu.Lock()
	pos := src.posFrame
	totalFrames := int64(len(src.samples) / src.channels)
	framesLeft := totalFrames - pos
	vol := src.volume // Get volume from source
	src.mu.Unlock()

	if framesLeft <= 0 {
		src.done.Store(true)
		return
	}

	framesToCopy := nFrames
	if int64(framesToCopy) > framesLeft {
		framesToCopy = int(framesLeft)
	}

	// Copy samples with volume adjustment
	switch {
	case src.channels == DefaultChannels:
		start := pos * int64(src.channels)
		end := start + int64(framesToCopy*DefaultChannels)
		for i, s := range src.samples[start:end] {
			outBuf[i] = s * vol
		}

	case src.channels == 1 && DefaultChannels == 2:
		// Mono to stereo
		start := pos
		for i := 0; i < framesToCopy; i++ {
			s := src.samples[start+int64(i)] * vol
			outBuf[2*i] = s
			outBuf[2*i+1] = s
		}

	case src.channels == 2 && DefaultChannels == 1:
		// Stereo to mono
		start := pos * 2
		for i := 0; i < framesToCopy; i++ {
			l := src.samples[start+int64(2*i)]
			r := src.samples[start+int64(2*i+1)]
			outBuf[i] = (l + r) * 0.5 * vol
		}
	}

	src.mu.Lock()
	src.posFrame += int64(framesToCopy)
	src.mu.Unlock()

	ptr := (*uint8)(unsafe.Pointer(&outBuf[0]))
	byteLen := int32(len(outBuf) * 4)
	sdl.PutAudioStreamData(stream, ptr, byteLen)
}

// monitorCompletion runs in a goroutine to detect playback completion.
func (e *StelleEngine) monitorCompletion() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			if e.src == nil {
				continue
			}
			if e.src.done.Load() {
				// Wait for SDL to drain queued audio
				queued := sdl.GetAudioStreamQueued(e.stream)
				if queued <= 0 {
					e.mu.Lock()
					e.state = AudioEngine.StateStopped
					callback := e.onComplete
					e.mu.Unlock()

					if callback != nil {
						callback()
					}
					return
				}
			}
		}
	}
}

// Play starts playing the audio file from the given position.
func (e *StelleEngine) Play(filePath string, seekTo float64, volume int) error {
	// Stop any current playback first (needs lock)
	e.mu.Lock()
	e.stopInternal()
	e.mu.Unlock()

	// Find all decoders that can handle this file
	decoders := e.findDecoders(filePath)
	if len(decoders) == 0 {
		return fmt.Errorf("no decoder found for file: %s", filePath)
	}

	// Try each matching decoder in order (fallback for shared extensions like .ogg)
	var src *AudioSource
	var lastErr error
	for _, decoder := range decoders {
		src, lastErr = decoder.Decode(filePath)
		if lastErr == nil {
			break
		}
	}
	if src == nil {
		return fmt.Errorf("decode failed: %w", lastErr)
	}

	// Now lock only for the quick SDL setup
	e.mu.Lock()
	defer e.mu.Unlock()

	// Set volume (convert 0-100 to 0.0-1.0)
	e.volume = float32(volume) / 100.0
	src.volume = e.volume

	// Set initial position from seekTo
	src.SetPosition(seekTo)

	e.src = src

	// Create SDL audio stream with callback
	cb := sdl.NewAudioStreamCallback(audioCallback)
	spec := sdl.AudioSpec{Format: sdl.AudioF32, Freq: DefaultSampleRate, Channels: DefaultChannels}
	stream := sdl.OpenAudioDeviceStream(sdl.AudioDeviceDefaultPlayback, &spec, cb, unsafe.Pointer(src))
	if stream == nil {
		return fmt.Errorf("failed to open audio device stream: %s", sdl.GetError())
	}
	e.stream = stream

	// Start playback
	if !sdl.ResumeAudioStreamDevice(e.stream) {
		return fmt.Errorf("failed to resume audio stream device: %s", sdl.GetError())
	}

	e.state = AudioEngine.StatePlaying

	// Start completion monitor
	e.stopCh = make(chan struct{})
	go e.monitorCompletion()

	return nil
}

// Stop stops playback and resets state.
func (e *StelleEngine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopInternal()
	e.state = AudioEngine.StateStopped
}

// stopInternal stops playback without locking (called internally).
func (e *StelleEngine) stopInternal() {
	// Signal completion monitor to stop
	select {
	case <-e.stopCh:
		// Already closed
	default:
		close(e.stopCh)
	}

	if e.stream != nil {
		sdl.PauseAudioStreamDevice(e.stream)
		sdl.DestroyAudioStream(e.stream)
		e.stream = nil
	}
	e.src = nil
}

// Pause pauses playback, preserving current position.
func (e *StelleEngine) Pause() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stream != nil && e.state == AudioEngine.StatePlaying {
		sdl.PauseAudioStreamDevice(e.stream)
		e.state = AudioEngine.StatePaused
	}
}

// Resume resumes playback from the given position.
func (e *StelleEngine) Resume(seekTo float64, volume int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.src == nil {
		return nil
	}

	// Update position if needed
	e.src.SetPosition(seekTo)

	// Update volume
	e.volume = float32(volume) / 100.0
	e.src.volume = e.volume

	// Resume SDL stream
	if e.stream != nil && e.state == AudioEngine.StatePaused {
		if !sdl.ResumeAudioStreamDevice(e.stream) {
			return fmt.Errorf("failed to resume audio stream: %s", sdl.GetError())
		}
		e.state = AudioEngine.StatePlaying

		// Restart completion monitor
		e.stopCh = make(chan struct{})
		go e.monitorCompletion()
	}

	return nil
}

// Seek jumps to the specified position while maintaining playback.
func (e *StelleEngine) Seek(position float64, volume int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.src == nil {
		return nil
	}

	// Update position
	e.src.SetPosition(position)

	// Update volume
	e.volume = float32(volume) / 100.0
	e.src.volume = e.volume

	// For accurate seeking, clear queued audio and restart stream
	if e.stream != nil && e.state == AudioEngine.StatePlaying {
		sdl.PauseAudioStreamDevice(e.stream)
		sdl.ClearAudioStream(e.stream)
		sdl.ResumeAudioStreamDevice(e.stream)
	}

	return nil
}

// GetState returns the current playback state.
func (e *StelleEngine) GetState() AudioEngine.PlaybackState {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state
}

// SetOnComplete sets the callback for when playback finishes.
func (e *StelleEngine) SetOnComplete(callback func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onComplete = callback
}
