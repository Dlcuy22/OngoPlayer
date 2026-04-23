// AudioEngine/StelleEngine/vorbis.go
// Vorbis decoder implementation using oggvorbis library.
//
// Functions:
//   - NewVorbisDecoder: creates a new Vorbis decoder instance.
//   - Name: returns the decoder name.
//   - CanHandle: returns true if the file extension is supported by this decoder.
//   - Decode: loads and decodes an OGG Vorbis file into PCM samples.
//   - SupportedExtensions: returns the list of file extensions this decoder supports.
//   - decodeVorbisFile: kept for backward compatibility but delegates to the new interface.
//   - isVorbisFile: Helper to check if a file can be decoded by VorbisDecoder based on extension.

package stelleengine

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jfreymuth/oggvorbis"
)

// VorbisDecoder implements the Decoder interface for OGG Vorbis files.
type VorbisDecoder struct{}

// NewVorbisDecoder creates a new Vorbis decoder instance.
func NewVorbisDecoder() *VorbisDecoder {
	return &VorbisDecoder{}
}

// Name returns the decoder name.
func (d *VorbisDecoder) Name() string {
	return "vorbis"
}

// CanHandle returns true if the file extension is supported by this decoder.
func (d *VorbisDecoder) CanHandle(ext string) bool {
	lower := strings.ToLower(ext)
	return lower == ".ogg" || lower == ".oga"
}

// Decode loads and decodes an OGG Vorbis file into PCM samples.
func (d *VorbisDecoder) Decode(path string) (*AudioSource, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	data, format, err := oggvorbis.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("decode ogg: %w", err)
	}

	if format.SampleRate != DefaultSampleRate {
		return nil, fmt.Errorf("sample rate mismatch: file=%d expected=%d (resample required)", format.SampleRate, DefaultSampleRate)
	}

	// Use our new utility for channel conversion
	samples := ConvertChannels(data, format.Channels, DefaultChannels)

	return &AudioSource{
		samples:    samples,
		posFrame:   0,
		channels:   DefaultChannels,
		sampleRate: format.SampleRate,
	}, nil
}

// SupportedExtensions returns the list of file extensions this decoder supports.
func (d *VorbisDecoder) SupportedExtensions() []string {
	return []string{".ogg", ".oga"}
}

// decodeVorbisFile is kept for backward compatibility but delegates to the new interface.
func decodeVorbisFile(path string) (*AudioSource, error) {
	decoder := NewVorbisDecoder()
	return decoder.Decode(path)
}

// Helper to check if a file can be decoded by VorbisDecoder based on extension.
func isVorbisFile(path string) bool {
	ext := filepath.Ext(path)
	decoder := NewVorbisDecoder()
	return decoder.CanHandle(ext)
}

type VorbisChunkDecoder struct {
	f          *os.File
	reader     *oggvorbis.Reader
	channels   int
	sampleRate int
}

func openVorbisChunkDecoder(path string) openFn {
	return func(seekTo float64) (ChunkDecoder, error) {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		r, err := oggvorbis.NewReader(f)
		if err != nil {
			f.Close()
			return nil, err
		}

		cd := &VorbisChunkDecoder{
			f:          f,
			reader:     r,
			channels:   r.Channels(),
			sampleRate: r.SampleRate(),
		}

		if seekTo > 0 {
			targetSample := int64(seekTo * float64(r.SampleRate()))
			if err := r.SetPosition(targetSample); err != nil {
				_ = err
			}
		}

		return cd, nil
	}
}

func (c *VorbisChunkDecoder) ReadSamples(buf []float32) (int, error) {
	n, err := c.reader.Read(buf)
	if err == io.EOF {
		return n, io.EOF
	}
	return n, err
}

func (c *VorbisChunkDecoder) Channels() int      { return c.channels }
func (c *VorbisChunkDecoder) SampleRate() int    { return c.sampleRate }
func (c *VorbisChunkDecoder) TotalFrames() int64 { return -1 }
func (c *VorbisChunkDecoder) Close() error       { return c.f.Close() }
