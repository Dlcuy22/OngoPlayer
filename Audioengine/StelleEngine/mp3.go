// Audioengine/StelleEngine/mp3.go
// MP3 decoder implementation using libmpg123 via purego.
// Provides Mp3ChunkDecoder which implements ChunkDecoder and SeekableChunkDecoder.
//
// Dependencies:
//   - internal/shared: cross-platform dynamic library loading
//   - purego: register C function symbols without cgo
//
// Runtime Libraries:
//   - Linux:   libmpg123.so.0
//   - Windows: libmpg123-0.dll
//   - macOS:   libmpg123.dylib

package stelleengine

import (
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/dlcuy22/OngoPlayer/internal/shared"
	"github.com/ebitengine/purego"
)

var (
	mpg123Once    sync.Once
	mpg123InitErr error

	mpg123_init      func() int32
	mpg123_new       func(decoder *byte, err *int32) uintptr
	mpg123_open      func(mh uintptr, path *byte) int32
	mpg123_getformat func(mh uintptr, rate *int64, channels *int32, encoding *int32) int32
	mpg123_read      func(mh uintptr, outmemory *byte, outmemsize int64, done *int64) int32
	mpg123_length    func(mh uintptr) int64
	mpg123_seek      func(mh uintptr, sampleoff int64, whence int32) int64
	mpg123_delete    func(mh uintptr)
	mpg123_exit      func()
)

/*
initMpg123File loads libmpg123 and registers all required C function symbols.
Called once via sync.Once on first use. Also calls mpg123_init().
*/
func initMpg123File() error {
	mpg123Once.Do(func() {
		var filename string
		switch runtime.GOOS {
		case "linux", "freebsd":
			filename = "libmpg123.so.0"
		case "windows":
			filename = "libmpg123-0.dll"
		case "darwin":
			filename = "libmpg123.dylib"
		}

		lib, err := shared.Load(filename)
		if err != nil {
			mpg123InitErr = fmt.Errorf("failed to load mpg123 library (%s): %w", filename, err)
			return
		}

		purego.RegisterLibFunc(&mpg123_init, lib, "mpg123_init")
		purego.RegisterLibFunc(&mpg123_new, lib, "mpg123_new")
		purego.RegisterLibFunc(&mpg123_open, lib, "mpg123_open")
		purego.RegisterLibFunc(&mpg123_getformat, lib, "mpg123_getformat")
		purego.RegisterLibFunc(&mpg123_read, lib, "mpg123_read")
		purego.RegisterLibFunc(&mpg123_length, lib, "mpg123_length")
		purego.RegisterLibFunc(&mpg123_seek, lib, "mpg123_seek")
		purego.RegisterLibFunc(&mpg123_delete, lib, "mpg123_delete")
		purego.RegisterLibFunc(&mpg123_exit, lib, "mpg123_exit")

		if res := mpg123_init(); res != 0 {
			mpg123InitErr = fmt.Errorf("mpg123_init failed with code %d", res)
		}
	})
	return mpg123InitErr
}

type Mp3Decoder struct{}

/*
NewMp3Decoder creates a new MP3 decoder instance.

	returns:
	      *Mp3Decoder
*/
func NewMp3Decoder() *Mp3Decoder {
	return &Mp3Decoder{}
}

func (d *Mp3Decoder) Name() string { return "mp3" }

func (d *Mp3Decoder) CanHandle(ext string) bool {
	return strings.ToLower(ext) == ".mp3"
}

/*
isMp3File checks if the file extension is handled by Mp3Decoder.

	params:
	      path: filesystem path
	returns:
	      bool
*/
func isMp3File(path string) bool {
	ext := filepath.Ext(path)
	decoder := NewMp3Decoder()
	return decoder.CanHandle(ext)
}

type Mp3ChunkDecoder struct {
	mh          uintptr
	channels    int
	sampleRate  int
	totalFrames int64
	buf         []byte
}

/*
openMp3ChunkDecoder returns an openFn factory for MP3 files.

	params:
	      path: filesystem path to the .mp3 file
	returns:
	      openFn
*/
func openMp3ChunkDecoder(path string) openFn {
	return func(seekTo float64) (ChunkDecoder, error) {
		if err := initMpg123File(); err != nil {
			return nil, err
		}

		var errCode int32
		mh := mpg123_new(nil, &errCode)
		if mh == 0 {
			return nil, fmt.Errorf("mpg123_new failed with code %d", errCode)
		}

		cPath := append([]byte(path), 0)
		if res := mpg123_open(mh, &cPath[0]); res != 0 {
			mpg123_delete(mh)
			return nil, fmt.Errorf("mpg123_open failed: code %d", res)
		}

		var rate int64
		var channels, encoding int32

		if res := mpg123_getformat(mh, &rate, &channels, &encoding); res != 0 {
			mpg123_delete(mh)
			return nil, fmt.Errorf("mpg123_getformat failed: code %d", res)
		}

		lenRes := mpg123_length(mh)
		total := int64(-1)
		if lenRes >= 0 {
			total = lenRes
		}

		cd := &Mp3ChunkDecoder{
			mh:          mh,
			channels:    int(channels),
			sampleRate:  int(rate),
			totalFrames: total,
			buf:         make([]byte, DecodeChunkSize*2*int(channels)),
		}

		if seekTo > 0 {
			targetFrame := int64(seekTo * float64(cd.sampleRate))
			if err := cd.SeekToFrame(targetFrame); err != nil {
				mpg123_delete(mh)
				return nil, err
			}
		}

		return cd, nil
	}
}

/*
ReadSamples decodes 16-bit PCM from the MP3 stream, converts to float32,
and resamples if the file's native rate differs from DefaultSampleRate.

	params:
	      buf: destination buffer for interleaved float32 samples
	returns:
	      int:   number of float32 values written
	      error: io.EOF on end of stream
*/
func (c *Mp3ChunkDecoder) ReadSamples(buf []float32) (int, error) {
	bytesNeeded := int64(len(buf) * 2)
	if int64(len(c.buf)) < bytesNeeded {
		c.buf = make([]byte, bytesNeeded)
	}

	var done int64
	res := mpg123_read(c.mh, &c.buf[0], bytesNeeded, &done)
	if done == 0 && res != 0 {
		if res == -12 {
			return 0, io.EOF
		}
		return 0, fmt.Errorf("mpg123_read error: %d", res)
	}

	if done == 0 {
		return 0, io.EOF
	}

	converted := ConvertInt16BytesToFloat32(c.buf[:done])

	if c.sampleRate != DefaultSampleRate {
		converted = Resample(converted, c.sampleRate, DefaultSampleRate, c.channels)
	}

	copy(buf, converted)
	return len(converted), nil
}

/*
SeekToFrame seeks to the specified PCM frame position using mpg123_seek.

	params:
	      frame: target frame offset
	returns:
	      error
*/
func (c *Mp3ChunkDecoder) SeekToFrame(frame int64) error {
	res := mpg123_seek(c.mh, frame, 0)
	if res < 0 {
		return fmt.Errorf("mpg123_seek failed returning %d", res)
	}
	return nil
}

func (c *Mp3ChunkDecoder) Channels() int      { return c.channels }
func (c *Mp3ChunkDecoder) SampleRate() int    { return c.sampleRate }
func (c *Mp3ChunkDecoder) TotalFrames() int64 { return c.totalFrames }
func (c *Mp3ChunkDecoder) Close() error {
	mpg123_delete(c.mh)
	return nil
}
