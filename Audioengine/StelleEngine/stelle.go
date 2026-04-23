// Audioengine/StelleEngine/stelle.go
// Native Go audio engine implementation using SDL3.
// Manages playback lifecycle, state machine, decoder selection, and SDL stream I/O.
//
// Dependencies:
//   - Audioengine: Engine interface and PlaybackState enum
//   - purego-sdl3/sdl: SDL3 audio device, stream, and callback API
//   - streaming.go: StreamingAudioSource, RingBuffer, openFn

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
	      *StelleEngine
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

/*
openFnForFile picks the right openFn based on file extension.

	params:
	      filePath: path to the audio file
	returns:
	      openFn, error
	Note: To add a new codec, add a case here and implement ChunkDecoder for it.
*/
func openFnForFile(filePath string) (openFn, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".opus":
		return openOpusChunkDecoder(filePath), nil
	case ".mp3":
		return openMp3ChunkDecoder(filePath), nil
	case ".ogg", ".oga":
		return openVorbisChunkDecoder(filePath), nil
	default:
		return nil, fmt.Errorf("no streaming decoder for extension: %s", ext)
	}
}

/*
audioCallback is the SDL audio callback that feeds samples from the
ring buffer to the audio device. Called by SDL on its audio thread.

	params:
	      userdata:         pointer to the StreamingAudioSource
	      stream:           the SDL audio stream requesting data
	      additionalAmount: number of bytes SDL wants
*/
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

	if n > 0 {
		src.AdvanceFrames(int64(n / src.channels))

		vol := src.Volume()
		for i := range outBuf[:n] {
			outBuf[i] *= vol
		}
	}

	for i := n; i < nSamples; i++ {
		outBuf[i] = 0
	}

	ptr := (*uint8)(unsafe.Pointer(&outBuf[0]))
	sdl.PutAudioStreamData(stream, ptr, int32(nSamples*4))
}

/*
monitorCompletion runs in a goroutine to detect when the decoder goroutine
has finished AND SDL has drained its internal queue, then fires onComplete.
*/
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

/*
Play starts playing the audio file from the given position.

	params:
	      filePath: path to the audio file
	      seekTo:   start position in seconds
	      volume:   playback volume (0-100)
	returns:
	      error
*/
func (e *StelleEngine) Play(filePath string, seekTo float64, volume int) error {
	e.mu.Lock()
	e.stopInternal()
	e.mu.Unlock()

	open, err := openFnForFile(filePath)
	if err != nil {
		return err
	}

	streamSrc, err := NewStreamingSource(open, seekTo)
	if err != nil {
		return fmt.Errorf("streaming source: %w", err)
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

/*
Stop stops playback and resets state.

	returns:
	      error
*/
func (e *StelleEngine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopInternal()
	e.state = AudioEngine.StateStopped
	return nil
}

/*
stopInternal tears down the current playback session.
Closes the decoder goroutine, destroys the SDL stream, and nils references.

	Note: Must be called with e.mu held.
*/
func (e *StelleEngine) stopInternal() {
	select {
	case <-e.stopCh:
	default:
		close(e.stopCh)
	}

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

/*
Pause pauses playback, preserving the current ring buffer state.

	returns:
	      error
*/
func (e *StelleEngine) Pause() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stream != nil && e.state == AudioEngine.StatePlaying {
		sdl.PauseAudioStreamDevice(e.stream)
		e.state = AudioEngine.StatePaused
	}
	return nil
}

/*
Resume resumes a paused stream. Optionally repositions if seekTo is non-zero.

	params:
	      seekTo: position in seconds (0 = resume from current)
	      volume: playback volume (0-100)
	returns:
	      error
*/
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

/*
Seek jumps to the specified position while maintaining current playback state.

	params:
	      position: target position in seconds
	      volume:   playback volume (0-100)
	returns:
	      error
*/
func (e *StelleEngine) Seek(position float64, volume int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.streamSrc == nil {
		return nil
	}

	e.volume = float32(volume) / 100.0
	e.streamSrc.SetVolume(e.volume)

	e.streamSrc.Seek(position)

	if e.stream != nil {
		sdl.ClearAudioStream(e.stream)
	}

	return nil
}

/*
GetState returns the current playback state.

	returns:
	      AudioEngine.PlaybackState
*/
func (e *StelleEngine) GetState() AudioEngine.PlaybackState {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state
}

/*
SetOnComplete sets the callback invoked when playback finishes naturally.

	params:
	      callback: function to call on completion
*/
func (e *StelleEngine) SetOnComplete(callback func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onComplete = callback
}

/*
GetPosition returns the current playback position in seconds.

	returns:
	      float64
*/
func (e *StelleEngine) GetPosition() float64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.streamSrc == nil {
		return 0
	}
	return e.streamSrc.Position()
}

/*
GetDuration returns the total duration of the current track in seconds.

	returns:
	      float64
*/
func (e *StelleEngine) GetDuration() float64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.streamSrc == nil {
		return 0
	}
	return e.streamSrc.Duration()
}
