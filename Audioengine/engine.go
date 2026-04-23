// Audioengine/engine.go
// Defines the audio playback engine contract and common types.
// All engine implementations (StelleEngine, etc.) must satisfy the Engine interface.
//
// Dependencies:
//   - None (interface-only file)

package AudioEngine

type PlaybackState int

const (
	StateStopped PlaybackState = iota
	StatePlaying
	StatePaused
)

/*
Engine defines the interface for audio playback backends.

	Note: Implementations can use native audio libraries, FFplay, etc.
*/
type Engine interface {
	Play(filePath string, seekTo float64, volume int) error
	Stop() error
	Pause() error
	Resume(seekTo float64, volume int) error
	Seek(position float64, volume int) error
	GetState() PlaybackState
	SetOnComplete(callback func())
	GetPosition() float64
	GetDuration() float64
}
