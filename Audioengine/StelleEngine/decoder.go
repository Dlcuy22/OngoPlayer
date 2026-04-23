// AudioEngine/StelleEngine/decoder.go
// Defines the decoder interface for different audio formats.
//
// Types:
//   - Decoder: interface for audio format decoders
//   - AudioSource: decoded PCM audio data with playback state
//
// Functions: None (interface-only file)

package stelleengine

import (
	"sync"
	"sync/atomic"
)

// Default audio parameters for SDL output.
const (
	DefaultSampleRate = 48000
	DefaultChannels   = 2
)

// AudioSource holds decoded PCM audio data and playback state.
type AudioSource struct {
	mu         sync.Mutex
	samples    []float32
	posFrame   int64
	channels   int
	sampleRate int
	done       atomic.Bool
	volume     float32
}

// Duration returns the total duration in seconds.
func (s *AudioSource) Duration() float64 {
	if s.sampleRate == 0 {
		return 0
	}
	totalFrames := int64(len(s.samples) / s.channels)
	return float64(totalFrames) / float64(s.sampleRate)
}

// Position returns the current playback position in seconds.
func (s *AudioSource) Position() float64 {
	if s.sampleRate == 0 {
		return 0
	}
	s.mu.Lock()
	pos := s.posFrame
	s.mu.Unlock()
	return float64(pos) / float64(s.sampleRate)
}

// SetPosition sets the playback position in seconds.
func (s *AudioSource) SetPosition(seekTo float64) {
	if s.sampleRate == 0 {
		return
	}
	totalFrames := int64(len(s.samples) / s.channels)
	newPos := int64(seekTo * float64(s.sampleRate))
	if newPos < 0 {
		newPos = 0
	}
	if newPos > totalFrames {
		newPos = totalFrames
	}
	s.mu.Lock()
	s.posFrame = newPos
	s.mu.Unlock()
}

// TotalFrames returns the total number of frames.
func (s *AudioSource) TotalFrames() int64 {
	return int64(len(s.samples) / s.channels)
}

// Decoder defines the interface for audio format decoders.
// Each decoder handles a specific audio format (ogg, mp3, flac, etc).
type Decoder interface {
	// Decode loads and decodes the file into PCM samples.
	// Returns an AudioSource ready for playback.
	Decode(path string) (*AudioSource, error)

	// CanHandle returns true if this decoder supports the given file extension.
	// The extension includes the dot (e.g., ".ogg", ".mp3").
	CanHandle(ext string) bool

	// Name returns the decoder name for logging/debugging.
	Name() string
}
