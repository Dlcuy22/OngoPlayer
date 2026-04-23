package stelleengine

import (
	"fmt"
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

// Decode loads and decodes an MP3 file into PCM samples.
func (d *MP3Decoder) Decode(path string) (*AudioSource, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	decoder, err := mp3.NewDecoder(f)
	if err != nil {
		return nil, fmt.Errorf("decode mp3: %w", err)
	}

	// go-mp3 returns raw decoded bytes (16-bit little endian)
	data, err := io.ReadAll(decoder)
	if err != nil {
		return nil, fmt.Errorf("read mp3 data: %w", err)
	}

	// go-mp3 always outputs 16-bit little-endian stereo.
	sampleRate := decoder.SampleRate()
	channels := 2 // go-mp3 is always stereo

	// Convert bytes to float32 using our new util
	floatSamples := ConvertInt16BytesToFloat32(data)

	// Resample if necessary
	if sampleRate != DefaultSampleRate {
		floatSamples = Resample(floatSamples, sampleRate, DefaultSampleRate, channels)
		sampleRate = DefaultSampleRate
	}

	// Convert channels to the engine's default via util
	finalSamples := ConvertChannels(floatSamples, channels, DefaultChannels)

	return &AudioSource{
		samples:    finalSamples,
		posFrame:   0,
		channels:   DefaultChannels,
		sampleRate: sampleRate,
	}, nil
}
