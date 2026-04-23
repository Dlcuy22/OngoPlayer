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

//		return &AudioSource{
//			samples:    finalSamples,
//			posFrame:   0,
//			channels:   DefaultChannels,
//			sampleRate: sampleRate,
//		}, nil
//	}
type Mp3ChunkDecoder struct {
	f           *os.File
	dec         *mp3.Decoder
	rawBuf      []byte
	totalFrames int64
}

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

		// dec.Length() = total decoded bytes (16-bit stereo = 4 bytes per frame)
		// Scale to DefaultSampleRate if the file needs resampling
		rawFrames := dec.Length() / 4
		totalFrames := rawFrames * int64(DefaultSampleRate) / int64(dec.SampleRate())

		cd := &Mp3ChunkDecoder{
			f:           f,
			dec:         dec,
			rawBuf:      make([]byte, DecodeChunkSize*2),
			totalFrames: totalFrames,
		}

		if seekTo > 0 {
			if err := cd.SeekToFrame(int64(seekTo * float64(DefaultSampleRate))); err != nil {
				f.Close()
				return nil, err
			}
		}

		return cd, nil
	}
}

func (c *Mp3ChunkDecoder) ReadSamples(buf []float32) (int, error) {
	// Read enough raw bytes to fill buf after conversion (and possible upsampling)
	// Worst case upsampling ratio: DefaultSampleRate / min(SampleRate) — 48000/44100 ≈ 1.09
	// Reading exactly len(buf)*2 bytes is safe: after upsample the result fits in buf.
	bytesNeeded := len(buf) * 2
	if len(c.rawBuf) < bytesNeeded {
		c.rawBuf = make([]byte, bytesNeeded)
	}

	n, err := c.dec.Read(c.rawBuf[:bytesNeeded])
	if n == 0 {
		if err == io.EOF {
			return 0, io.EOF
		}
		return 0, err
	}

	converted := ConvertInt16BytesToFloat32(c.rawBuf[:n])

	sr := c.dec.SampleRate()
	if sr != DefaultSampleRate {
		converted = Resample(converted, sr, DefaultSampleRate, 2)
	}

	// converted may now be larger or smaller than buf — copy only what fits
	copied := copy(buf, converted)
	return copied, nil
}

// SeekToFrame uses go-mp3's native io.Seeker to jump directly to the target frame.
func (c *Mp3ChunkDecoder) SeekToFrame(frame int64) error {
	// Convert from output frames (DefaultSampleRate) back to source frames
	srcFrame := frame * int64(c.dec.SampleRate()) / int64(DefaultSampleRate)
	// go-mp3 Seek operates on bytes: 4 bytes per frame (16-bit stereo)
	byteOffset := srcFrame * 4
	_, err := c.dec.Seek(byteOffset, io.SeekStart)
	return err
}

func (c *Mp3ChunkDecoder) Channels() int      { return DefaultChannels }
func (c *Mp3ChunkDecoder) SampleRate() int    { return DefaultSampleRate }
func (c *Mp3ChunkDecoder) TotalFrames() int64 { return c.totalFrames }
func (c *Mp3ChunkDecoder) Close() error       { return c.f.Close() }
