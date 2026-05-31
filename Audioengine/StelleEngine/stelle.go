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
	closed     bool
}

/*
NewStelleEngine creates a new Stelle engine instance and initializes the SDL
audio subsystem.

	params:
	      volume: default volume of the audio engine (0.0 - 1.0)
	returns:
	      *StelleEngine
	      error: if the SDL audio subsystem fails to initialize
*/
func NewStelleEngine(volume float32) (*StelleEngine, error) {
	if !sdl.Init(sdl.InitAudio) {
		return nil, fmt.Errorf("failed to init SDL audio: %s", sdl.GetError())
	}

	return &StelleEngine{
		state:  AudioEngine.StateStopped,
		volume: volume,
		stopCh: make(chan struct{}),
	}, nil
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
	case ".flac":
		return openFlacChunkDecoder(filePath), nil
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
		// Ring data is normalized to DefaultChannels, so output frames are
		// n / DefaultChannels regardless of the file's native channel count.
		src.AdvanceFrames(int64(n / DefaultChannels))

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

	params:
	      streamSrc: the source this monitor owns (snapshot, not e.streamSrc)
	      stream:    the SDL stream this monitor owns (snapshot)
	      stopCh:    cancellation channel for this specific playback session
	Note: All session state is passed in by value so the goroutine never reads
	      mutable engine fields concurrently with Play/Resume/stopInternal.
*/
func (e *StelleEngine) monitorCompletion(streamSrc *StreamingAudioSource, stream *sdl.AudioStream, stopCh chan struct{}) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			if streamSrc.done.Load() && streamSrc.ring.Available() == 0 {
				if sdl.GetAudioStreamQueued(stream) <= 0 {
					e.mu.Lock()
					// Only fire if this monitor still owns the active stream;
					// a newer Play() may have replaced streamSrc since.
					if e.streamSrc != streamSrc {
						e.mu.Unlock()
						return
					}
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
	go e.monitorCompletion(streamSrc, stream, e.stopCh)

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
Close tears down any active playback and shuts down the SDL audio subsystem.
After Close the engine must not be reused.

	returns:
	      error
	Note: Pairs with the sdl.Init in NewStelleEngine. Safe to call more than
	      once; subsequent calls are no-ops.
*/
func (e *StelleEngine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil
	}
	e.closed = true

	e.stopInternal()
	e.state = AudioEngine.StateStopped
	sdl.QuitSubSystem(sdl.InitAudio)
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

	// Destroy the SDL stream BEFORE dropping the Go reference to streamSrc.
	// The audio callback holds streamSrc as a raw unsafe.Pointer userdata,
	// which the GC can't see; destroying the stream guarantees SDL stops
	// calling back, so the object can't be collected mid-callback.
	if e.stream != nil {
		sdl.PauseAudioStreamDevice(e.stream)
		sdl.DestroyAudioStream(e.stream)
		e.stream = nil
	}

	if e.streamSrc != nil {
		// Close ring buffer to unblock any goroutine stuck in ring.Write().
		e.streamSrc.ring.Close()

		select {
		case <-e.streamSrc.stopCh:
		default:
			close(e.streamSrc.stopCh)
		}
		e.streamSrc = nil
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
		// Do NOT start a new monitorCompletion here: Pause never cancels the
		// session's stopCh, so the monitor goroutine spawned by Play is still
		// alive and owns this stream. Spawning another would leak a goroutine
		// and risk firing onComplete twice.
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
SetVolume updates the playback volume dynamically.

	params:
	      volume: playback volume (0-100)
*/
func (e *StelleEngine) SetVolume(volume int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.volume = float32(volume) / 100.0
	if e.streamSrc != nil {
		e.streamSrc.SetVolume(e.volume)
	}
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

/*
GetSampleRate returns the actual sample rate of the current audio track.

	returns:
	      int
*/
func (e *StelleEngine) GetSampleRate() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.streamSrc != nil {
		return e.streamSrc.SampleRate()
	}
	return DefaultSampleRate
}

/*
GetChannels returns the actual channel count of the current audio track.

	returns:
	      int
*/
func (e *StelleEngine) GetChannels() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.streamSrc != nil {
		return e.streamSrc.Channels()
	}
	return DefaultChannels
}
