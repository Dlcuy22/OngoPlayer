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
	"strings"
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
	streamSrc  *StreamingAudioSource
	onComplete func()
	stopCh     chan struct{}
}

/*
NewStelleEngine creates a new Stelle engine instance.

	params:
	      volume: default volume of the audio engine (0.0 - 1.0)
	returns:
		  *stelleengine.StelleEngine
*/
func NewStelleEngine(volume float32) *StelleEngine {
	if !sdl.Init(sdl.InitAudio) {
		panic(sdl.GetError())
	}

	return &StelleEngine{
		state:  AudioEngine.StateStopped,
		volume: volume,
		stopCh: make(chan struct{}),
	}
}

// openFnForFile picks the possible openFns based on file extension.
func openFnForFile(filePath string) ([]openFn, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".opus":
		return []openFn{openOpusChunkDecoder(filePath)}, nil
	case ".mp3":
		return []openFn{openMp3ChunkDecoder(filePath)}, nil
	case ".ogg", ".oga":
		// .ogg can be Opus or Vorbis. Try Opus first, fallback to Vorbis.
		return []openFn{openOpusChunkDecoder(filePath), openVorbisChunkDecoder(filePath)}, nil
	default:
		return nil, fmt.Errorf("no streaming decoder for extension: %s", ext)
	}
}

// audioCallback is the SDL audio callback that feeds samples from the ring buffer to the audio device
func audioCallback(userdata unsafe.Pointer, stream *sdl.AudioStream, additionalAmount, _ int32) {
	if additionalAmount <= 0 {
		return
	}

	src := (*StreamingAudioSource)(userdata)
	if src == nil {
		nSamples := int(additionalAmount) / 4
		silence := make([]float32, nSamples)
		ptr := (*uint8)(unsafe.Pointer(&silence[0]))
		sdl.PutAudioStreamData(stream, ptr, additionalAmount)
		return
	}

	nSamples := int(additionalAmount) / 4
	outBuf := make([]float32, nSamples)

	n := src.ring.Read(outBuf)

	// Track playback position: samples read / channels = frames consumed
	if n > 0 {
		src.AdvanceFrames(int64(n / src.channels))

		vol := src.Volume()
		for i := range outBuf[:n] {
			outBuf[i] *= vol
		}
	}

	// Zero-fill the tail if the ring was starved (buffering gap or near EOF)
	for i := n; i < nSamples; i++ {
		outBuf[i] = 0
	}

	ptr := (*uint8)(unsafe.Pointer(&outBuf[0]))
	sdl.PutAudioStreamData(stream, ptr, int32(nSamples*4))
}

// monitorCompletion runs in a goroutine to detect when the decoder goroutine
// has finished AND SDL has drained its internal queue
func (e *StelleEngine) monitorCompletion() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			if e.streamSrc == nil {
				continue
			}
			// Two conditions must both be true before firing onComplete:
			// 1. Decoder goroutine hit EOF and set done
			// 2. Ring buffer is fully drained (SDL has consumed everything)
			if e.streamSrc.done.Load() && e.streamSrc.ring.Available() == 0 {
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
// seekTo is in seconds, volume is 0–100.
func (e *StelleEngine) Play(filePath string, seekTo float64, volume int) error {
	// Stop any current playback before starting new one
	e.mu.Lock()
	e.stopInternal()
	e.mu.Unlock()

	// Resolve the possible openFns for this file type
	openFns, err := openFnForFile(filePath)
	if err != nil {
		return err
	}

	var streamSrc *StreamingAudioSource
	var errs []string
	for _, open := range openFns {
		streamSrc, err = NewStreamingSource(open, seekTo)
		if err == nil {
			break
		}
		errs = append(errs, err.Error())
	}
	if streamSrc == nil {
		return fmt.Errorf("all decoders failed: %s", strings.Join(errs, " | "))
	}
	streamSrc.SetVolume(float32(volume) / 100.0)

	e.mu.Lock()
	defer e.mu.Unlock()

	e.volume = float32(volume) / 100.0
	e.streamSrc = streamSrc

	cb := sdl.NewAudioStreamCallback(audioCallback)
	spec := sdl.AudioSpec{Format: sdl.AudioF32, Freq: DefaultSampleRate, Channels: DefaultChannels}
	stream := sdl.OpenAudioDeviceStream(sdl.AudioDeviceDefaultPlayback, &spec, cb, unsafe.Pointer(streamSrc))
	if stream == nil {
		return fmt.Errorf("failed to open audio device stream: %s", sdl.GetError())
	}
	e.stream = stream

	if !sdl.ResumeAudioStreamDevice(e.stream) {
		return fmt.Errorf("failed to resume audio stream device: %s", sdl.GetError())
	}

	e.state = AudioEngine.StatePlaying

	e.stopCh = make(chan struct{})
	go e.monitorCompletion()

	return nil
}

// Stop stops playback and resets state
func (e *StelleEngine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopInternal()
	e.state = AudioEngine.StateStopped
	return nil
}

func (e *StelleEngine) stopInternal() {
	select {
	case <-e.stopCh:

	default:
		close(e.stopCh)
	}

	// Shut down the decoder goroutine via the ring's stopCh
	if e.streamSrc != nil {
		select {
		case <-e.streamSrc.stopCh:
		default:
			close(e.streamSrc.stopCh)
		}
		e.streamSrc = nil
	}

	if e.stream != nil {
		sdl.PauseAudioStreamDevice(e.stream)
		sdl.DestroyAudioStream(e.stream)
		e.stream = nil
	}
}

// Pause pauses playback, preserving the current ring buffer state.
func (e *StelleEngine) Pause() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stream != nil && e.state == AudioEngine.StatePlaying {
		sdl.PauseAudioStreamDevice(e.stream)
		e.state = AudioEngine.StatePaused
	}
	return nil
}

// Resume resumes a paused stream. seekTo repositions if non-zero.
func (e *StelleEngine) Resume(seekTo float64, volume int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.streamSrc == nil {
		return nil
	}

	e.volume = float32(volume) / 100.0
	e.streamSrc.SetVolume(e.volume)

	if seekTo > 0 {
		e.streamSrc.Seek(seekTo)
		if e.stream != nil {
			sdl.ClearAudioStream(e.stream)
		}
	}

	if e.stream != nil && e.state == AudioEngine.StatePaused {
		if !sdl.ResumeAudioStreamDevice(e.stream) {
			return fmt.Errorf("failed to resume audio stream: %s", sdl.GetError())
		}
		e.state = AudioEngine.StatePlaying

		e.stopCh = make(chan struct{})
		go e.monitorCompletion()
	}

	return nil
}

// Seek jumps to the specified position while maintaining current playback state.
func (e *StelleEngine) Seek(position float64, volume int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.streamSrc == nil {
		return nil
	}

	e.volume = float32(volume) / 100.0
	e.streamSrc.SetVolume(e.volume)

	// Send seek to the decoder goroutine (non-blocking, latest wins)
	e.streamSrc.Seek(position)

	// Clear SDL's internal buffer so stale pre-seek audio isn't heard
	if e.stream != nil {
		sdl.ClearAudioStream(e.stream)
	}

	return nil
}

// GetState returns the current playback state.
func (e *StelleEngine) GetState() AudioEngine.PlaybackState {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state
}

// SetOnComplete sets the callback invoked when playback finishes naturally.
func (e *StelleEngine) SetOnComplete(callback func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onComplete = callback
}

// GetPosition returns the current playback position in seconds.
func (e *StelleEngine) GetPosition() float64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.streamSrc == nil {
		return 0
	}
	// fmt.Println("Position:", e.streamSrc.Position)
	return e.streamSrc.Position()
}

// GetDuration returns the total duration of the current track in seconds.
func (e *StelleEngine) GetDuration() float64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.streamSrc == nil {
		return 0
	}
	return e.streamSrc.Duration()
}
