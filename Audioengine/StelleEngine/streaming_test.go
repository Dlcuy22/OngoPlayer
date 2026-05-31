// Audioengine/StelleEngine/streaming_test.go
// Tests the decoder-agnostic streaming layer: centralized channel/sample-rate
// normalization and the native-vs-output seek frame domains. No SDL involved.

package stelleengine

import (
	"io"
	"sync/atomic"
	"testing"
	"time"
)

// fakeDecoder emits a fixed pool of native-rate, native-channel PCM, then EOF.
// seekFrame records the last frame passed to SeekToFrame for domain assertions.
type fakeDecoder struct {
	data       []float32
	pos        int
	channels   int
	sampleRate int
	total      int64
	seekable   bool
	seekFrame  atomic.Int64
	infinite   bool // emit silence forever instead of EOF (for seek tests)
}

func (f *fakeDecoder) ReadSamples(buf []float32) (int, error) {
	if f.infinite {
		for i := range buf {
			buf[i] = 0
		}
		return len(buf), nil
	}
	if f.pos >= len(f.data) {
		return 0, io.EOF
	}
	n := copy(buf, f.data[f.pos:])
	f.pos += n
	return n, nil
}

func (f *fakeDecoder) Channels() int      { return f.channels }
func (f *fakeDecoder) SampleRate() int    { return f.sampleRate }
func (f *fakeDecoder) TotalFrames() int64 { return f.total }
func (f *fakeDecoder) Close() error       { return nil }

// seekableFakeDecoder adds native seek so the streaming layer uses SeekToFrame
// rather than the close+reopen fallback.
type seekableFakeDecoder struct{ *fakeDecoder }

func (s seekableFakeDecoder) SeekToFrame(frame int64) error {
	s.fakeDecoder.seekFrame.Store(frame)
	return nil
}

func TestStreamingNormalizesToStereo48k(t *testing.T) {
	// 1 second of mono audio at 44100. Expect stereo output at 48000.
	const nativeRate = 44100
	mono := make([]float32, nativeRate) // 1s mono
	fd := &fakeDecoder{
		data:       mono,
		channels:   1,
		sampleRate: nativeRate,
		total:      int64(nativeRate),
	}

	open := func(seekTo float64) (ChunkDecoder, error) { return fd, nil }
	src, err := NewStreamingSource(open, 0)
	if err != nil {
		t.Fatalf("NewStreamingSource: %v", err)
	}
	defer func() {
		select {
		case <-src.stopCh:
		default:
			close(src.stopCh)
		}
		src.ring.Close()
	}()

	// Native rate/channels must be preserved for display purposes.
	if src.SampleRate() != nativeRate {
		t.Errorf("SampleRate() = %d, want %d", src.SampleRate(), nativeRate)
	}
	if src.Channels() != 1 {
		t.Errorf("Channels() = %d, want 1", src.Channels())
	}

	// Wait for the decode goroutine to finish producing.
	deadline := time.After(2 * time.Second)
	for !src.done.Load() {
		select {
		case <-deadline:
			t.Fatal("decode goroutine did not finish")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	// Drain everything the goroutine buffered.
	total := 0
	dst := make([]float32, 4096)
	for {
		n := src.ring.Read(dst)
		total += n
		if n == 0 && src.ring.Available() == 0 {
			break
		}
	}

	if total%DefaultChannels != 0 {
		t.Errorf("output sample count %d is not a whole number of stereo frames", total)
	}

	// ~1s mono at 44100 -> ~1s stereo at 48000 -> ~96000 samples. Per-chunk
	// resampling introduces minor rounding, so allow a tolerance.
	outFrames := total / DefaultChannels
	if outFrames < 47000 || outFrames > 49000 {
		t.Errorf("output frames = %d, want ~48000", outFrames)
	}
}

func TestStreamingInitialSeekUsesOutputDomain(t *testing.T) {
	const nativeRate = 44100
	fd := &fakeDecoder{channels: 2, sampleRate: nativeRate, total: nativeRate * 10, infinite: true}
	open := func(seekTo float64) (ChunkDecoder, error) { return seekableFakeDecoder{fd}, nil }

	src, err := NewStreamingSource(open, 5.0) // seek to 5s on open
	if err != nil {
		t.Fatalf("NewStreamingSource: %v", err)
	}
	defer func() {
		select {
		case <-src.stopCh:
		default:
			close(src.stopCh)
		}
		src.ring.Close()
	}()

	// posFrame is in OUTPUT frames: 5s * 48000.
	if got := src.posFrame.Load(); got != int64(5*DefaultSampleRate) {
		t.Errorf("posFrame = %d, want %d", got, 5*DefaultSampleRate)
	}
	if pos := src.Position(); pos < 4.99 || pos > 5.01 {
		t.Errorf("Position() = %f, want ~5.0", pos)
	}
}

func TestStreamingSeekSplitsDomains(t *testing.T) {
	const nativeRate = 44100
	fd := &fakeDecoder{channels: 2, sampleRate: nativeRate, total: nativeRate * 100, infinite: true}
	open := func(seekTo float64) (ChunkDecoder, error) { return seekableFakeDecoder{fd}, nil }

	src, err := NewStreamingSource(open, 0)
	if err != nil {
		t.Fatalf("NewStreamingSource: %v", err)
	}
	defer func() {
		select {
		case <-src.stopCh:
		default:
			close(src.stopCh)
		}
		src.ring.Close()
	}()

	// Drain the ring continuously so the decode goroutine never blocks in
	// ring.Write and can service the seek request promptly.
	stopDrain := make(chan struct{})
	go func() {
		dst := make([]float32, 4096)
		for {
			select {
			case <-stopDrain:
				return
			default:
				src.ring.Read(dst)
			}
		}
	}()
	defer close(stopDrain)

	src.Seek(10.0)

	// Native frame domain: 10s * 44100 goes to the decoder.
	wantNative := int64(10 * nativeRate)
	deadline := time.After(2 * time.Second)
	for fd.seekFrame.Load() != wantNative {
		select {
		case <-deadline:
			t.Fatalf("SeekToFrame got %d, want %d", fd.seekFrame.Load(), wantNative)
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	// Output frame domain: posFrame = 10s * 48000.
	if got := src.posFrame.Load(); got != int64(10*DefaultSampleRate) {
		t.Errorf("posFrame = %d, want %d", got, 10*DefaultSampleRate)
	}
}
