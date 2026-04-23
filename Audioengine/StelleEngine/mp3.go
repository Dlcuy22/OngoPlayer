// AudioEngine/StelleEngine/mp3.go
// MP3 decoder implementation using native libmpg123 via purego.
//
// Functions:
//   - NewMp3Decoder: creates a new MP3 decoder instance.
//   - Name: returns the decoder name.
//   - CanHandle: returns true if the file extension is supported by this decoder.
//   - isMp3File: Helper to check if a file can be decoded by Mp3Decoder based on extension.

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

// Mp3Decoder implements the Decoder interface for MP3 files.
type Mp3Decoder struct{}

func NewMp3Decoder() *Mp3Decoder {
	return &Mp3Decoder{}
}

func (d *Mp3Decoder) Name() string {
	return "mp3"
}

func (d *Mp3Decoder) CanHandle(ext string) bool {
	return strings.ToLower(ext) == ".mp3"
}

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
	buf         []byte // internal buffer for 16-bit PCM bytes
}

// openMp3ChunkDecoder is the openFn for mp3 using libmpg123.
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

		// Calculate total valid samples across the whole file. 
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
			buf:         make([]byte, DecodeChunkSize*2*int(channels)), // allocate byte buffer
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

func (c *Mp3ChunkDecoder) ReadSamples(buf []float32) (int, error) {
	// buf size represents requested float32 samples.
	// mpg123 yields natively 16-bit ints, meaning 2 bytes per float sample.
	bytesNeeded := int64(len(buf) * 2)
	if int64(len(c.buf)) < bytesNeeded {
		c.buf = make([]byte, bytesNeeded)
	}

	var done int64
	res := mpg123_read(c.mh, &c.buf[0], bytesNeeded, &done)
	if done == 0 && res != 0 {
		// MPG123_DONE is -12
		if res == -12 {
			return 0, io.EOF
		}
		return 0, fmt.Errorf("mpg123_read error: %d", res)
	}

	if done == 0 {
		return 0, io.EOF
	}

	// Read returns bytes. Convert -> Float32 natively with common util vectoring
	converted := ConvertInt16BytesToFloat32(c.buf[:done])

	// Resample if the native sample rate requires interpolation to match SDL audio specs
	if c.sampleRate != DefaultSampleRate {
		converted = Resample(converted, c.sampleRate, DefaultSampleRate, c.channels)
	}

	copy(buf, converted)
	return len(converted), nil
}

func (c *Mp3ChunkDecoder) SeekToFrame(frame int64) error {
	// Let mpg123 seek accurately across all frames via internal jump calculations.
	res := mpg123_seek(c.mh, frame, 0) // SEEK_SET
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
