// Audioengine/StelleEngine/vorbis.go
// Vorbis decoder implementation using Xiph libvorbisfile via purego.
// Provides VorbisChunkDecoder which implements ChunkDecoder and SeekableChunkDecoder.
//
// Dependencies:
//   - internal/shared: cross-platform dynamic library loading
//   - purego: register C function symbols without cgo
//
// Runtime Libraries:
//   - Linux:   libvorbisfile.so.3
//   - Windows: libvorbisfile.dll
//   - macOS:   libvorbisfile.dylib

package stelleengine

import (
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/dlcuy22/OngoPlayer/internal/shared"
	"github.com/ebitengine/purego"
)

var (
	vorbisfileOnce    sync.Once
	vorbisfileInitErr error

	ov_fopen      func(path *byte, vf *OggVorbis_File) int32
	ov_clear      func(vf *OggVorbis_File) int32
	ov_info       func(vf *OggVorbis_File, link int32) uintptr
	ov_pcm_total  func(vf *OggVorbis_File, link int32) int64
	ov_pcm_seek   func(vf *OggVorbis_File, pos int64) int32
	ov_read_float func(vf *OggVorbis_File, pcmChannels ***float32, samples int32, bitstream *int32) int
)

// OggVorbis_File is an opaque C struct (~700-1000 bytes). 2048 bytes covers all platforms.
type OggVorbis_File [2048]byte

/*
initVorbisFile loads libvorbisfile and registers all required C function symbols.
Called once via sync.Once on first use.
*/
func initVorbisFile() error {
	vorbisfileOnce.Do(func() {
		var filename string
		switch runtime.GOOS {
		case "linux", "freebsd":
			filename = "libvorbisfile.so.3"
		case "windows":
			filename = "libvorbisfile.dll"
		case "darwin":
			filename = "libvorbisfile.dylib"
		}

		lib, err := shared.Load(filename)
		if err != nil {
			vorbisfileInitErr = fmt.Errorf("failed to load vorbisfile library (%s): %w", filename, err)
			return
		}

		purego.RegisterLibFunc(&ov_fopen, lib, "ov_fopen")
		purego.RegisterLibFunc(&ov_clear, lib, "ov_clear")
		purego.RegisterLibFunc(&ov_info, lib, "ov_info")
		purego.RegisterLibFunc(&ov_pcm_total, lib, "ov_pcm_total")
		purego.RegisterLibFunc(&ov_pcm_seek, lib, "ov_pcm_seek")
		purego.RegisterLibFunc(&ov_read_float, lib, "ov_read_float")
	})
	return vorbisfileInitErr
}

type VorbisDecoder struct{}

/*
NewVorbisDecoder creates a new Vorbis decoder instance.

	returns:
	      *VorbisDecoder
*/
func NewVorbisDecoder() *VorbisDecoder {
	return &VorbisDecoder{}
}

func (d *VorbisDecoder) Name() string { return "vorbis" }

func (d *VorbisDecoder) CanHandle(ext string) bool {
	lower := strings.ToLower(ext)
	return lower == ".ogg" || lower == ".oga"
}

/*
isVorbisFile checks if the file extension is handled by VorbisDecoder.

	params:
	      path: filesystem path
	returns:
	      bool
*/
func isVorbisFile(path string) bool {
	ext := filepath.Ext(path)
	decoder := NewVorbisDecoder()
	return decoder.CanHandle(ext)
}

type VorbisChunkDecoder struct {
	vf          OggVorbis_File
	channels    int
	sampleRate  int
	totalFrames int64
}

/*
openVorbisChunkDecoder returns an openFn factory for Vorbis files.

	params:
	      path: filesystem path to the .ogg file
	returns:
	      openFn
*/
func openVorbisChunkDecoder(path string) openFn {
	return func(seekTo float64) (ChunkDecoder, error) {
		if err := initVorbisFile(); err != nil {
			return nil, err
		}

		cPath := append([]byte(path), 0)
		cd := &VorbisChunkDecoder{}

		if res := ov_fopen(&cPath[0], &cd.vf); res != 0 {
			return nil, fmt.Errorf("ov_fopen failed with code %d", res)
		}

		infoPtr := ov_info(&cd.vf, -1)
		if infoPtr == 0 {
			ov_clear(&cd.vf)
			return nil, fmt.Errorf("ov_info returned null")
		}

		// vorbis_info layout: version (int32 @0), channels (int32 @4)
		// rate (long @8) varies by platform, so we use DefaultSampleRate.
		cd.channels = int(*(*int32)(unsafe.Pointer(infoPtr + 4)))
		cd.sampleRate = DefaultSampleRate
		cd.totalFrames = ov_pcm_total(&cd.vf, -1)

		if seekTo > 0 {
			targetFrame := int64(seekTo * float64(cd.sampleRate))
			if err := cd.SeekToFrame(targetFrame); err != nil {
				ov_clear(&cd.vf)
				return nil, err
			}
		}

		return cd, nil
	}
}

/*
ReadSamples decodes interleaved float32 PCM from the Vorbis stream.
Converts the planar C float arrays into Go interleaved layout.

	params:
	      buf: destination buffer for interleaved samples
	returns:
	      int:   number of float32 values written
	      error: io.EOF on end of stream
*/
func (c *VorbisChunkDecoder) ReadSamples(buf []float32) (int, error) {
	maxSamples := len(buf) / c.channels

	var pcmChannels **float32
	var bitstream int32

	read := ov_read_float(&c.vf, &pcmChannels, int32(maxSamples), &bitstream)
	if read == 0 {
		return 0, io.EOF
	}
	if read < 0 {
		return 0, fmt.Errorf("ov_read_float failed: %d", read)
	}

	ptrArray := unsafe.Slice(pcmChannels, c.channels)

	for ch := 0; ch < c.channels; ch++ {
		channelSlice := unsafe.Slice(ptrArray[ch], int(read))
		for i := 0; i < int(read); i++ {
			buf[i*c.channels+ch] = channelSlice[i]
		}
	}

	return int(read) * c.channels, nil
}

/*
SeekToFrame seeks to the specified PCM frame position.

	params:
	      frame: target frame offset
	returns:
	      error
*/
func (c *VorbisChunkDecoder) SeekToFrame(frame int64) error {
	if res := ov_pcm_seek(&c.vf, frame); res != 0 {
		return fmt.Errorf("ov_pcm_seek failed: %d", res)
	}
	return nil
}

func (c *VorbisChunkDecoder) Channels() int      { return c.channels }
func (c *VorbisChunkDecoder) SampleRate() int    { return c.sampleRate }
func (c *VorbisChunkDecoder) TotalFrames() int64 { return c.totalFrames }
func (c *VorbisChunkDecoder) Close() error {
	ov_clear(&c.vf)
	return nil
}
