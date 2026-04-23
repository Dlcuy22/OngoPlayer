// AudioEngine/StelleEngine/decoder.go
// Defines the decoder interface for different audio formats.
//
// Types:
//   - Decoder: interface for audio format decoders
//   - AudioSource: decoded PCM audio data with playback state
//
// Functions: None (interface-only file)

package stelleengine

// Default audio parameters for SDL output.
const (
	DefaultSampleRate = 48000
	DefaultChannels   = 2
)

// Decoder defines the interface for audio format decoders.
// Each decoder handles a specific audio format (ogg, mp3, flac, etc).
type Decoder interface {
	// CanHandle returns true if this decoder supports the given file extension.
	// The extension includes the dot (e.g., ".ogg", ".mp3").
	CanHandle(ext string) bool

	// Name returns the decoder name for logging/debugging.
	Name() string
}
