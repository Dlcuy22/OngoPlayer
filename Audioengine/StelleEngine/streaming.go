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
	"sync"
	"sync/atomic"
)

const (
	RingBufferFrames = DefaultSampleRate * 3
	DecodeChunkSize  = 11520
)

/*
RingBuffer is a lock-based circular buffer used to decouple the decoder
goroutine from the SDL audio callback. The decoder writes decoded PCM
float32 samples in, and the SDL callback reads them out.
*/
type RingBuffer struct {
	buf      []float32
	cap      int
	head     int
	tail     int
	count    int
	closed   bool
	mu       sync.Mutex
	notEmpty *sync.Cond
	notFull  *sync.Cond
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
	rb := &RingBuffer{buf: make([]float32, c), cap: c}
	rb.notEmpty = sync.NewCond(&rb.mu)
	rb.notFull = sync.NewCond(&rb.mu)
	return rb
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
	rb.mu.Lock()
	defer rb.mu.Unlock()
	for _, s := range samples {
		for rb.count == rb.cap && !rb.closed {
			rb.notFull.Wait()
		}
		if rb.closed {
			return false
		}
		rb.buf[rb.tail] = s
		rb.tail = (rb.tail + 1) % rb.cap
		rb.count++
	}
	rb.notEmpty.Broadcast()
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
	rb.mu.Lock()
	defer rb.mu.Unlock()
	n := 0
	for n < len(dst) && rb.count > 0 {
		dst[n] = rb.buf[rb.head]
		rb.head = (rb.head + 1) % rb.cap
		rb.count--
		n++
	}
	rb.notFull.Broadcast()
	return n
}

/*
Clear resets the ring buffer head/tail/count to zero without deallocating.
*/
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.head, rb.tail, rb.count = 0, 0, 0
	rb.notFull.Broadcast()
}

/*
Available returns how many float32 samples are currently buffered.
*/
func (rb *RingBuffer) Available() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.count
}

/*
Close marks the buffer as closed, waking any blocked writers/readers.
*/
func (rb *RingBuffer) Close() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.closed = true
	rb.notFull.Broadcast()
	rb.notEmpty.Broadcast()
}

/*
Reopen resets the buffer to an empty, open state for reuse.
*/
func (rb *RingBuffer) Reopen() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.head, rb.tail, rb.count = 0, 0, 0
	rb.closed = false
	rb.notFull.Broadcast()
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
	return float64(s.posFrame.Load()) / float64(s.sampleRate)
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

	src := &StreamingAudioSource{
		ring:        NewRingBuffer(RingBufferFrames, DefaultChannels),
		channels:    cd.Channels(),
		sampleRate:  cd.SampleRate(),
		totalFrames: cd.TotalFrames(),
		seekCh:      make(chan seekRequest, 1),
		stopCh:      make(chan struct{}),
	}
	if seekTo > 0 {
		src.posFrame.Store(int64(seekTo * float64(cd.SampleRate())))
	}

	go func() {
		defer cd.Close()

		buf := make([]float32, DecodeChunkSize)

		for {
			select {
			case <-src.stopCh:
				src.ring.Close()
				return

			case req := <-src.seekCh:
				targetFrame := int64(req.positionSecs * float64(src.sampleRate))

				if sd, ok := cd.(SeekableChunkDecoder); ok {
					_ = sd.SeekToFrame(targetFrame)
				} else {
					cd.Close()
					newCD, err := open(req.positionSecs)
					if err == nil {
						cd = newCD
					}
				}

				src.posFrame.Store(targetFrame)
				src.done.Store(false)
				src.ring.Clear()
				continue

			default:
			}

			n, err := cd.ReadSamples(buf)
			if n > 0 {
				if ok := src.ring.Write(buf[:n]); !ok {
					return
				}
			}
			if err == io.EOF || (err == nil && n == 0) {
				src.done.Store(true)
				src.ring.notEmpty.Broadcast()
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
