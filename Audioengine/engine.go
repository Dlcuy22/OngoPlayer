// AudioEngine/engine.go
// Defines the audio playback engine interface and common types.
//
// Types:
//   - PlaybackState: enum for stopped, playing, paused states
//   - Engine: interface for audio playback operations
//
// Functions: None (interface-only file)

package AudioEngine

// PlaybackState represents the current state of the audio engine.
type PlaybackState int

const (
	StateStopped PlaybackState = iota
	StatePlaying
	StatePaused
)

// Engine defines the interface for audio playback backends.
// Implementations can use FFplay, native audio libraries, etc.
type Engine interface {
	// Play starts playing the audio file from the given position with specified volume.
	// seekTo is in seconds, volume is 0-100.
	Play(filePath string, seekTo float64, volume int) error

	// Stop stops playback and resets position.
	Stop() error

	// Pause pauses playback, preserving current position.
	// This function returns an error if the audio engine is not in a state where it can be paused.
	Pause() error

	// Resume resumes playback from the given position with specified volume.
	Resume(seekTo float64, volume int) error

	// Seek jumps to the specified position while maintaining playback.
	Seek(position float64, volume int) error

	// GetState returns the current playback state.
	GetState() PlaybackState

	// SetOnComplete sets a callback to be invoked when playback finishes.
	SetOnComplete(callback func())
}
