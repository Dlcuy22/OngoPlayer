package stelleengine

import (
	"io"
	"os"
	"strings"

	"github.com/hajimehoshi/go-mp3"
)

// MP3Decoder implements the Decoder interface for MP3 files.
type MP3Decoder struct{}

// NewMp3Decoder creates a new MP3 decoder instance.
func NewMp3Decoder() *MP3Decoder {
	return &MP3Decoder{}
}

// Name returns the decoder name.
func (d *MP3Decoder) Name() string {
	return "mp3"
}

// CanHandle returns true if the file extension is supported by this decoder.
func (d *MP3Decoder) CanHandle(ext string) bool {
	lower := strings.ToLower(ext)
	return lower == ".mp3"
}

// // Decode loads and decodes an MP3 file into PCM samples.
// func (d *MP3Decoder) Decode(path string) (*AudioSource, error) {
// 	f, err := os.Open(path)
// 	if err != nil {
// 		return nil, fmt.Errorf("open file: %w", err)
// 	}
// 	defer f.Close()

// 	decoder, err := mp3.NewDecoder(f)
// 	if err != nil {
// 		return nil, fmt.Errorf("decode mp3: %w", err)
// 	}

// 	// go-mp3 returns raw decoded bytes (16-bit little endian)
// 	data, err := io.ReadAll(decoder)
// 	if err != nil {
// 		return nil, fmt.Errorf("read mp3 data: %w", err)
// 	}

// 	// go-mp3 always outputs 16-bit little-endian stereo.
// 	sampleRate := decoder.SampleRate()
// 	channels := 2 // go-mp3 is always stereo

// 	// Convert bytes to float32 using our new util
// 	floatSamples := ConvertInt16BytesToFloat32(data)

// 	// Resample if necessary
// 	if sampleRate != DefaultSampleRate {
// 		floatSamples = Resample(floatSamples, sampleRate, DefaultSampleRate, channels)
// 		sampleRate = DefaultSampleRate
// 	}

// 	// Convert channels to the engine's default via util
// 	finalSamples := ConvertChannels(floatSamples, channels, DefaultChannels)

// 	return &AudioSource{
// 		samples:    finalSamples,
// 		posFrame:   0,
// 		channels:   DefaultChannels,
// 		sampleRate: sampleRate,
// 	}, nil
// }

type Mp3ChunkDecoder struct {
	f           *os.File
	dec         *mp3.Decoder
	buf         []byte
	totalFrames int64
}

// openMp3ChunkDecoder is the openFn for mp3.
// seekTo is handled by re-opening + decoding-to-position in NewStreamingSource.
func openMp3ChunkDecoder(path string) openFn {
	return func(seekTo float64) (ChunkDecoder, error) {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		dec, err := mp3.NewDecoder(f)
		if err != nil {
			f.Close()
			return nil, err
		}

		cd := &Mp3ChunkDecoder{
			f:   f,
			dec: dec,
			buf: make([]byte, DecodeChunkSize*2), // 2 bytes per int16
		}

		if seekTo > 0 {
			skipSamples := int64(seekTo*float64(dec.SampleRate())) * 2 // stereo
			skipBytes := skipSamples * 2                               // int16
			discardBuf := make([]byte, 8192)
			for skipBytes > 0 {
				toRead := int64(len(discardBuf))
				if skipBytes < toRead {
					toRead = skipBytes
				}
				n, err := dec.Read(discardBuf[:toRead])
				skipBytes -= int64(n)
				if err != nil {
					break
				}
			}
		}

		return cd, nil
	}
}

func (c *Mp3ChunkDecoder) ReadSamples(buf []float32) (int, error) {
	bytesNeeded := len(buf) * 2 // 1 float32 per int16
	if len(c.buf) < bytesNeeded {
		c.buf = make([]byte, bytesNeeded)
	}

	n, err := c.dec.Read(c.buf[:bytesNeeded])
	if n == 0 && err != nil {
		if err == io.EOF {
			return 0, io.EOF
		}
		return 0, err
	}

	converted := ConvertInt16BytesToFloat32(c.buf[:n])
	// Resample if needed
	sr := c.dec.SampleRate()
	if sr != DefaultSampleRate {
		converted = Resample(converted, sr, DefaultSampleRate, 2)
	}
	copy(buf, converted)
	return len(converted), nil
}

func (c *Mp3ChunkDecoder) Channels() int      { return DefaultChannels }
func (c *Mp3ChunkDecoder) SampleRate() int    { return DefaultSampleRate }
func (c *Mp3ChunkDecoder) TotalFrames() int64 { return c.totalFrames }
func (c *Mp3ChunkDecoder) Close() error       { return c.f.Close() }
