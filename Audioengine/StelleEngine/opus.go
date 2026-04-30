// Audioengine/StelleEngine/opus.go
// Opus decoder implementation using Xiph libopusfile via purego.
// Provides OpusChunkDecoder which implements ChunkDecoder and SeekableChunkDecoder.
//
// Dependencies:
//   - internal/shared: cross-platform dynamic library loading
//   - purego: register C function symbols without cgo
//
// Runtime Libraries:
//   - Linux:   libopusfile.so.0
//   - Windows: libopusfile-0.dll
//   - macOS:   libopusfile.0.dylib

package stelleengine

import (
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"unsafe"

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

/*
initOpusFile loads libopusfile and registers all required C function symbols
Called once via sync.Once on first use
*/
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

/*
NewOpusDecoder creates a new Opus decoder instance.

	returns:
	      *OpusDecoder
*/
func NewOpusDecoder() *OpusDecoder {
	return &OpusDecoder{}
}

func (d *OpusDecoder) Name() string { return "opus" }

func (d *OpusDecoder) CanHandle(ext string) bool {
	lower := strings.ToLower(ext)
	return lower == ".opus" || lower == ".ogg"
}

type OpusChunkDecoder struct {
	of          uintptr
	totalFrames int64
}

/*
openOpusChunkDecoder returns an openFn factory for Opus files.

	params:
	      path: filesystem path to the .opus file
	returns:
	      openFn
*/
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

/*
ReadSamples decodes interleaved stereo float32 PCM from the Opus stream.

	params:
	      buf: destination buffer for interleaved samples
	returns:
	      int:   number of float32 values written
	      error: io.EOF on end of stream
*/
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

/*
SeekToFrame seeks to the specified PCM frame position.

	params:
	      frame: target frame offset
	returns:
	      error
*/
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

/*
cString converts a Go string to a null-terminated byte pointer for C interop.

	params:
	      s: Go string
	returns:
	      *byte
	Note: Caller must keep a reference to the returned slice to prevent GC.
*/
func cString(s string) *byte {
	b := append([]byte(s), 0)
	return (*byte)(unsafe.Pointer(&b[0]))
}
