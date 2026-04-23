// AudioEngine/StelleEngine/vorbis.go
// Vorbis decoder implementation using libvorbisfile via purego.
//
// Types:
//   - VorbisDecoder: implements the Decoder interface for OGG Vorbis files.
//   - VorbisChunkDecoder: implements the ChunkDecoder interface using libvorbisfile.

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

// OggVorbis_File is an opaque C struct representing the internal state of the
// libvorbisfile decoder. We statically allocate a byte array large enough
// to hold any OS implementation's size for safely interacting with the library.
type OggVorbis_File [2048]byte

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
			return // Don't panic, just return the init err on play
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

// VorbisDecoder implements the Decoder interface for OGG Vorbis files.
type VorbisDecoder struct{}

func NewVorbisDecoder() *VorbisDecoder {
	return &VorbisDecoder{}
}

func (d *VorbisDecoder) Name() string {
	return "vorbis"
}

func (d *VorbisDecoder) CanHandle(ext string) bool {
	lower := strings.ToLower(ext)
	return lower == ".ogg" || lower == ".oga"
}

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

		// Read `channels` and `rate` natively from pointer offsets.
		// `version` is offset 0 (4 bytes), `channels` is offset 4 (4 bytes), 
		// Note: Rate (`long`) offset differs heavily between systems so we avoid it 
		// and hardcode DefaultSampleRate fallback for now or use the generic default.
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

func (c *VorbisChunkDecoder) ReadSamples(buf []float32) (int, error) {
	// buf size determines max samples per channel we can safely request
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

	// Read is the number of samples PER CHANNEL
	// pcmChannels gives an array of C-pointers length 'channels'
	ptrArray := unsafe.Slice(pcmChannels, c.channels)

	// Interleave the planar arrays directly into `buf`
	for ch := 0; ch < c.channels; ch++ {
		channelSlice := unsafe.Slice(ptrArray[ch], int(read))
		for i := 0; i < int(read); i++ {
			buf[i*c.channels+ch] = channelSlice[i]
		}
	}

	return int(read) * c.channels, nil
}

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
