// Audioengine/StelleEngine/streaming.go
// Generic streaming layer that works with any codec implementing ChunkDecoder.
// Handles the decode goroutine, ring buffer, seek dispatch, and position tracking.
//
// Dependencies:
//   - io: EOF detection
//   - sync, sync/atomic: thread-safe ring buffer and position counter

package stelleengine

import (
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

const (
	RingBufferFrames = DefaultSampleRate * 3
	DecodeChunkSize  = 11520
)

/*
RingBuffer is a lock-free circular buffer used to decouple the decoder
goroutine from the SDL audio callback. The decoder writes decoded PCM
float32 samples in, and the SDL callback reads them out.
*/
type RingBuffer struct {
	buf         []float32
	cap         int
	writeCursor atomic.Uint64
	readCursor  atomic.Uint64
	closed      atomic.Bool
}

/*
NewRingBuffer allocates a new ring buffer.

	params:
	      frames:   number of audio frames to buffer
	      channels: number of audio channels (e.g. 2 for stereo)
	returns:
	      *RingBuffer
*/
func NewRingBuffer(frames, channels int) *RingBuffer {
	c := frames * channels
	return &RingBuffer{
		buf: make([]float32, c),
		cap: c,
	}
}

/*
Write pushes interleaved float32 samples into the ring buffer.
Blocks if the buffer is full until space is available or the buffer is closed.

	params:
	      samples: interleaved float32 PCM data to write
	returns:
	      bool: false if the buffer was closed during the write
*/
func (rb *RingBuffer) Write(samples []float32) bool {
	written := 0
	totalToWrite := len(samples)
	for written < totalToWrite {
		if rb.closed.Load() {
			return false
		}

		w := rb.writeCursor.Load()
		r := rb.readCursor.Load()

		occupied := w - r
		space := uint64(rb.cap) - occupied

		if space == 0 {
			runtime.Gosched()
			time.Sleep(1 * time.Millisecond)
			continue
		}

		toWrite := int(space)
		if totalToWrite-written < toWrite {
			toWrite = totalToWrite - written
		}

		for i := 0; i < toWrite; i++ {
			idx := (w + uint64(i)) % uint64(rb.cap)
			rb.buf[idx] = samples[written+i]
		}

		rb.writeCursor.Add(uint64(toWrite))
		written += toWrite
	}
	return true
}

/*
Read copies available samples from the ring buffer into dst.

	params:
	      dst: destination slice to fill with samples
	returns:
	      int: number of float32 values actually copied
*/
func (rb *RingBuffer) Read(dst []float32) int {
	w := rb.writeCursor.Load()
	r := rb.readCursor.Load()

	avail := w - r
	toRead := len(dst)
	if avail < uint64(toRead) {
		toRead = int(avail)
	}

	for i := 0; i < toRead; i++ {
		idx := (r + uint64(i)) % uint64(rb.cap)
		dst[i] = rb.buf[idx]
	}

	rb.readCursor.Add(uint64(toRead))
	return toRead
}

/*
Clear resets the ring buffer read cursor to the write cursor to empty it.
*/
func (rb *RingBuffer) Clear() {
	w := rb.writeCursor.Load()
	rb.readCursor.Store(w)
}

/*
Available returns how many float32 samples are currently buffered.
*/
func (rb *RingBuffer) Available() int {
	w := rb.writeCursor.Load()
	r := rb.readCursor.Load()
	if w < r {
		return 0
	}
	return int(w - r)
}

/*
Close marks the buffer as closed.
*/
func (rb *RingBuffer) Close() {
	rb.closed.Store(true)
}

/*
Reopen resets the buffer to an empty, open state for reuse.
*/
func (rb *RingBuffer) Reopen() {
	rb.writeCursor.Store(0)
	rb.readCursor.Store(0)
	rb.closed.Store(false)
}

type seekRequest struct {
	positionSecs float64
}

/*
StreamingAudioSource is the decoder-agnostic bridge between a ChunkDecoder
and the SDL audio callback. A background goroutine reads decoded PCM into
the ring buffer, and the SDL callback drains it.
*/
type StreamingAudioSource struct {
	ring        *RingBuffer
	channels    int
	sampleRate  int
	totalFrames int64

	posFrame atomic.Int64
	done     atomic.Bool
	underruns atomic.Int64

	volume float32
	volMu  sync.Mutex

	seekCh chan seekRequest
	stopCh chan struct{}
}

/*
Position returns the current playback position in seconds.

	returns:
	      float64
*/
func (s *StreamingAudioSource) Position() float64 {
	// posFrame is counted in OUTPUT frames (DefaultSampleRate) because the SDL
	// callback advances it after normalization, not in the file's native rate.
	return float64(s.posFrame.Load()) / float64(DefaultSampleRate)
}

/*
SampleRate returns the native sample rate of the loaded audio file.
*/
func (s *StreamingAudioSource) SampleRate() int {
	return s.sampleRate
}

/*
Channels returns the native channel count of the loaded audio file.
*/
func (s *StreamingAudioSource) Channels() int {
	return s.channels
}

/*
Duration returns the total duration of the audio source in seconds.

	returns:
	      float64: 0 if unknown
*/
func (s *StreamingAudioSource) Duration() float64 {
	if s.totalFrames <= 0 || s.sampleRate == 0 {
		return 0
	}
	return float64(s.totalFrames) / float64(s.sampleRate)
}

/*
AdvanceFrames increments the internal position counter by n frames.

	params:
	      n: number of frames consumed by the SDL callback
*/
func (s *StreamingAudioSource) AdvanceFrames(n int64) {
	s.posFrame.Add(n)
}

/*
Volume returns the current playback volume.

	returns:
	      float32: 0.0 - 1.0
*/
func (s *StreamingAudioSource) Volume() float32 {
	s.volMu.Lock()
	defer s.volMu.Unlock()
	return s.volume
}

/*
SetVolume updates the playback volume.

	params:
	      v: volume level (0.0 - 1.0)
*/
func (s *StreamingAudioSource) SetVolume(v float32) {
	s.volMu.Lock()
	defer s.volMu.Unlock()
	s.volume = v
}

/*
Seek sends a seek request to the decoder goroutine.
Drains any pending request before sending the new one (latest-wins).

	params:
	      positionSecs: target position in seconds
*/
func (s *StreamingAudioSource) Seek(positionSecs float64) {
	select {
	case <-s.seekCh:
	default:
	}
	s.seekCh <- seekRequest{positionSecs}
}

/*
openFn is a factory that opens (or reopens) a ChunkDecoder at a given seek offset.
For decoders with native seek this is only ever called once (seekTo=0).
For decoders without native seek it is called again on each Seek() request.
*/
type openFn func(seekTo float64) (ChunkDecoder, error)

/*
NewStreamingSource opens a streaming source using the provided factory
and spawns the background decoder goroutine.

	params:
	      open:   factory function that creates a ChunkDecoder
	      seekTo: initial playback position in seconds
	returns:
	      *StreamingAudioSource, error
*/
func NewStreamingSource(open openFn, seekTo float64) (*StreamingAudioSource, error) {
	cd, err := open(seekTo)
	if err != nil {
		return nil, err
	}

	_ = initDspBindings() // Try to load C DSP library, ignore error to fallback

	src := &StreamingAudioSource{
		ring:        NewRingBuffer(RingBufferFrames, DefaultChannels),
		channels:    cd.Channels(),
		sampleRate:  cd.SampleRate(),
		totalFrames: cd.TotalFrames(),
		seekCh:      make(chan seekRequest, 1),
		stopCh:      make(chan struct{}),
	}
	if seekTo > 0 {
		// posFrame is in OUTPUT frames (DefaultSampleRate), not native rate.
		src.posFrame.Store(int64(seekTo * float64(DefaultSampleRate)))
	}

	go func() {
		defer cd.Close()

		buf := make([]float32, DecodeChunkSize)
		var chanBuf []float32
		var resampleBuf []float32

		for {
			select {
			case <-src.stopCh:
				src.ring.Close()
				return

			case req := <-src.seekCh:
				// Native frame domain (file rate) for the decoder's own seek.
				nativeFrame := int64(req.positionSecs * float64(src.sampleRate))
				// Output frame domain (DefaultSampleRate) for the position counter,
				// since posFrame is advanced post-normalization by the SDL callback.
				outputFrame := int64(req.positionSecs * float64(DefaultSampleRate))

				if sd, ok := cd.(SeekableChunkDecoder); ok {
					_ = sd.SeekToFrame(nativeFrame)
				} else {
					cd.Close()
					newCD, err := open(req.positionSecs)
					if err == nil {
						cd = newCD
					}
				}

				src.posFrame.Store(outputFrame)
				src.done.Store(false)
				src.ring.Clear()
				continue

			default:
			}

			n, err := cd.ReadSamples(buf)
			if n > 0 {
				// Centralized normalization: decoders emit native-rate,
				// native-channel PCM; we convert to DefaultChannels then
				// DefaultSampleRate exactly once before buffering.
				out := buf[:n]

				// 1. Channel conversion
				if src.channels != DefaultChannels {
					if convert_channels_c != nil {
						neededChanSize := (n / src.channels) * DefaultChannels
						if len(chanBuf) < neededChanSize {
							chanBuf = make([]float32, neededChanSize)
						}
						convert_channels_c(
							uintptr(unsafe.Pointer(&out[0])),
							uintptr(unsafe.Pointer(&chanBuf[0])),
							int32(n/src.channels),
							int32(src.channels),
							int32(DefaultChannels),
						)
						out = chanBuf[:neededChanSize]
					} else {
						out = ConvertChannels(out, src.channels, DefaultChannels)
					}
				}

				// 2. Resampling
				if src.sampleRate != DefaultSampleRate {
					if resample_linear_c != nil {
						inFrames := len(out) / DefaultChannels
						ratio := float64(src.sampleRate) / float64(DefaultSampleRate)
						outFrames := int(float64(inFrames) / ratio)
						neededResampleSize := outFrames * DefaultChannels

						if len(resampleBuf) < neededResampleSize {
							resampleBuf = make([]float32, neededResampleSize)
						}
						resample_linear_c(
							uintptr(unsafe.Pointer(&out[0])),
							uintptr(unsafe.Pointer(&resampleBuf[0])),
							int32(src.sampleRate),
							int32(DefaultSampleRate),
							int32(DefaultChannels),
							int32(inFrames),
							int32(outFrames),
						)
						out = resampleBuf[:neededResampleSize]
					} else {
						out = Resample(out, src.sampleRate, DefaultSampleRate, DefaultChannels)
					}
				}

				if ok := src.ring.Write(out); !ok {
					return
				}
			}
			if err == io.EOF || (err == nil && n == 0) {
				src.done.Store(true)
				return
			}
			if err != nil {
				src.done.Store(true)
				src.ring.Close()
				return
			}
		}
	}()

	return src, nil
}
