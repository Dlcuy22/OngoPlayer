// Audioengine/StelleEngine/decoder.go
// Defines the shared constants and the ChunkDecoder interfaces used
// by all codec implementations (opus, vorbis, mp3).
//
// Dependencies:
//   - None (interface-only file)

package stelleengine

const (
	DefaultSampleRate = 48000
	DefaultChannels   = 2
)

/*
ChunkDecoder is a thin read-cursor over a decoded audio stream.
Each codec (opus, vorbis, mp3) wraps its native C library into this interface.
*/
type ChunkDecoder interface {
	ReadSamples(buf []float32) (int, error)
	Channels() int
	SampleRate() int
	TotalFrames() int64
	Close() error
}

/*
SeekableChunkDecoder is the optional extension for decoders with native seek.
Decoders that don't implement this get automatic re-open+skip seeking for free
via the streaming goroutine fallback in streaming.go.
*/
type SeekableChunkDecoder interface {
	ChunkDecoder
	SeekToFrame(frame int64) error
}
