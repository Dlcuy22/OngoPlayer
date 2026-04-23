// AudioEngine/StelleEngine/opus.go
// Opus decoder implementation using the real Xiph libopusfile via purego.
// Requires libopusfile-0.dll, libopus-0.dll, and libogg-0.dll at runtime.
//
// Types:
//   - OpusDecoder: implements the Decoder interface for Opus files
//
// Functions:
//   - NewOpusDecoder: creates a new Opus decoder instance
//   - Name: returns the decoder name
//   - CanHandle: returns true for .opus and .ogg extensions
//   - Decode: loads and decodes an Opus file into PCM samples via libopusfile

package stelleengine

import (
	"fmt"
	"strings"
	"sync"
	"unsafe"

	"io"
	"runtime"

	shared "github.com/dlcuy22/OngoPlayer/internal/shared"
	"github.com/ebitengine/purego"
)

var (
	opusfileOnce    sync.Once
	opusfileInitErr error

	opOpenFile        func(path *byte, errPtr *int32) uintptr
	opReadFloatStereo func(of uintptr, pcm *float32, bufSize int32) int32
	opPcmTotal        func(of uintptr, li int32) int64
	opChannelCount    func(of uintptr, li int32) int32
	opFree            func(of uintptr)
	opPcmSeek         func(of uintptr, pcmOffset int64) int32
	opPcmTell         func(of uintptr) int64
)

func initOpusFile() error {
	opusfileOnce.Do(func() {
		var filename string
		switch runtime.GOOS {
		case "linux", "freebsd":
			filename = "libopusfile.so.0"
		case "windows":
			filename = "libopusfile-0.dll"
		case "darwin":
			filename = "libopusfile.0.dylib"
		}

		lib, err := shared.Load(filename)
		if err != nil {
			panic(fmt.Errorf("failed to load opusfile library (%s): %w", filename, err))
		}

		purego.RegisterLibFunc(&opOpenFile, lib, "op_open_file")
		purego.RegisterLibFunc(&opReadFloatStereo, lib, "op_read_float_stereo")
		purego.RegisterLibFunc(&opPcmTotal, lib, "op_pcm_total")
		purego.RegisterLibFunc(&opChannelCount, lib, "op_channel_count")
		purego.RegisterLibFunc(&opFree, lib, "op_free")
		purego.RegisterLibFunc(&opPcmSeek, lib, "op_pcm_seek")
		purego.RegisterLibFunc(&opPcmTell, lib, "op_pcm_tell")
	})
	return opusfileInitErr
}

type OpusDecoder struct{}

func NewOpusDecoder() *OpusDecoder {
	return &OpusDecoder{}
}

func (d *OpusDecoder) Name() string {
	return "opus"
}

func (d *OpusDecoder) CanHandle(ext string) bool {
	lower := strings.ToLower(ext)
	return lower == ".opus" || lower == ".ogg"
}

func (d *OpusDecoder) Decode(path string) (*AudioSource, error) {
	if err := initOpusFile(); err != nil {
		return nil, err
	}

	// op_open_file expects a null-terminated C string
	cPath := append([]byte(path), 0)
	var opErr int32

	of := opOpenFile(&cPath[0], &opErr)
	if of == 0 {
		return nil, fmt.Errorf("op_open_file failed with error code %d", opErr)
	}
	defer opFree(of)

	// Get total PCM samples for pre-allocation (-1 = entire stream)
	totalSamples := opPcmTotal(of, -1)

	// op_read_float_stereo always outputs stereo (2 channels)
	channels := 2
	var allSamples []float32

	if totalSamples > 0 {
		// Pre-allocate: totalSamples is per-channel
		allSamples = make([]float32, 0, int(totalSamples)*channels)
	}

	// Read in chunks op_read_float_stereo returns samples per channel per call
	// Buffer: 120ms at 48kHz stereo = 5760 * 2 = 11520 floats
	buf := make([]float32, 11520)

	for {
		// n = number of samples per channel read, 0 = EOF, <0 = error
		n := opReadFloatStereo(of, &buf[0], int32(len(buf)))
		if n == 0 {
			break
		}
		if n < 0 {
			// OP_HOLE (-3) means a gap in the data, skip it
			if n == -3 {
				continue
			}
			return nil, fmt.Errorf("op_read_float_stereo error: %d", n)
		}

		// n samples per channel * 2 channels = total float32s
		totalFloats := int(n) * channels
		allSamples = append(allSamples, buf[:totalFloats]...)
	}

	if len(allSamples) == 0 {
		return nil, fmt.Errorf("no audio data decoded from opus file")
	}

	return &AudioSource{
		samples:    allSamples,
		posFrame:   0,
		channels:   DefaultChannels,
		sampleRate: DefaultSampleRate,
	}, nil
}

type OpusChunkDecoder struct {
	of          uintptr
	totalFrames int64
}

// openOpusChunkDecoder is the openFn for opus, passed to NewStreamingSource.
func openOpusChunkDecoder(path string) openFn {
	return func(seekTo float64) (ChunkDecoder, error) {
		if err := initOpusFile(); err != nil {
			return nil, err
		}
		cPath := append([]byte(path), 0)
		var opErr int32
		of := opOpenFile(&cPath[0], &opErr)
		if of == 0 {
			return nil, fmt.Errorf("op_open_file failed: code %d", opErr)
		}

		cd := &OpusChunkDecoder{
			of:          of,
			totalFrames: opPcmTotal(of, -1),
		}

		if seekTo > 0 {
			targetFrame := int64(seekTo * float64(DefaultSampleRate))
			if opPcmSeek(of, targetFrame) != 0 {
				opFree(of)
				return nil, fmt.Errorf("op_pcm_seek failed")
			}
		}
		return cd, nil
	}
}

func (c *OpusChunkDecoder) ReadSamples(buf []float32) (int, error) {
	n := opReadFloatStereo(c.of, &buf[0], int32(len(buf)))
	if n == 0 {
		return 0, io.EOF
	}
	if n == -3 {
		return 0, nil
	}
	if n < 0 {
		return 0, fmt.Errorf("op_read_float_stereo: %d", n)
	}
	return int(n) * DefaultChannels, nil
}

func (c *OpusChunkDecoder) SeekToFrame(frame int64) error {
	if opPcmSeek(c.of, frame) != 0 {
		return fmt.Errorf("op_pcm_seek failed")
	}
	return nil
}

func (c *OpusChunkDecoder) Channels() int      { return DefaultChannels }
func (c *OpusChunkDecoder) SampleRate() int    { return DefaultSampleRate }
func (c *OpusChunkDecoder) TotalFrames() int64 { return c.totalFrames }
func (c *OpusChunkDecoder) Close() error       { opFree(c.of); return nil }

// cString converts a Go string to a null-terminated byte pointer.
// The caller must keep a reference to the returned slice to prevent GC.
func cString(s string) *byte {
	b := append([]byte(s), 0)
	return (*byte)(unsafe.Pointer(&b[0]))
}
