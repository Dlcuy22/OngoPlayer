// AudioEngine/StelleEngine/streaming.go
// Generic streaming layer. Works with any decoder that implements ChunkDecoder

package stelleengine

import (
	"io"
	"sync"
	"sync/atomic"
)

const (
	RingBufferFrames = DefaultSampleRate * 3 // 3s headroom
	DecodeChunkSize  = 11520                 // 120ms @ 48kHz stereo
)

/*
ChunkDecoder what every decoder must expose
ChunkDecoder is a thin read-cursor over a decoded audio stream.
Each decoder wraps its own library into this interface.
*/
type ChunkDecoder interface {
	// ReadSamples fills buf with interleaved float32 PCM.
	// Returns number of float32 values written.
	// Returns 0, io.EOF on end of file.
	ReadSamples(buf []float32) (int, error)

	Channels() int
	SampleRate() int
	TotalFrames() int64 // -1 if unknown
	Close() error
}

// SeekableChunkDecoder is the optional extension for decoders with native seek.
// Decoders that don't implement this get automatic re-open+skip seeking for free.
type SeekableChunkDecoder interface {
	ChunkDecoder
	SeekToFrame(frame int64) error
}

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

func NewRingBuffer(frames, channels int) *RingBuffer {
	c := frames * channels
	rb := &RingBuffer{buf: make([]float32, c), cap: c}
	rb.notEmpty = sync.NewCond(&rb.mu)
	rb.notFull = sync.NewCond(&rb.mu)
	return rb
}

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

func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.head, rb.tail, rb.count = 0, 0, 0
	rb.notFull.Broadcast()
}

func (rb *RingBuffer) Available() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.count
}

func (rb *RingBuffer) Close() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.closed = true
	rb.notFull.Broadcast()
	rb.notEmpty.Broadcast()
}

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

// StreamingAudioSource is decoder-agnostic. The goroutine inside calls
// ChunkDecoder.ReadSamples in a loop and handles seek for any decoder.
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

func (s *StreamingAudioSource) Position() float64 {
	return float64(s.posFrame.Load()) / float64(s.sampleRate)
}

func (s *StreamingAudioSource) Duration() float64 {
	if s.totalFrames <= 0 || s.sampleRate == 0 {
		return 0
	}
	return float64(s.totalFrames) / float64(s.sampleRate)
}

func (s *StreamingAudioSource) AdvanceFrames(n int64) {
	s.posFrame.Add(n)
}

func (s *StreamingAudioSource) Volume() float32 {
	s.volMu.Lock()
	defer s.volMu.Unlock()
	return s.volume
}

func (s *StreamingAudioSource) SetVolume(v float32) {
	s.volMu.Lock()
	defer s.volMu.Unlock()
	s.volume = v
}

// Seek sends a seek request to the decoder goroutine.
func (s *StreamingAudioSource) Seek(positionSecs float64) {
	// Drain old request if not yet consumed, then send new one.
	select {
	case <-s.seekCh:
	default:
	}
	s.seekCh <- seekRequest{positionSecs}
}

/*
openFn is a factory that opens (or reopens) a ChunkDecoder at a given seek offset
For decoders with native seek this is only ever called once (seekTo=0)
For decoders without native seek it is called again on each Seek() request
*/
type openFn func(seekTo float64) (ChunkDecoder, error)

// NewStreamingSource opens a streaming source using the provided factory.
// seekTo is the initial playback position in seconds.
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
