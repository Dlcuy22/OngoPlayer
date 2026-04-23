// Audioengine/StelleEngine/utils.go
// Audio processing utilities for format conversion, resampling, and channel manipulation.
// These are used by all codec ChunkDecoders to normalize decoded audio for the SDL pipeline.
//
// Dependencies:
//   - None (pure Go math)

package stelleengine

/*
ConvertInt16ToFloat32 converts a slice of int16 audio samples to float32 [-1.0, 1.0].

	params:
	      in: raw int16 PCM samples
	returns:
	      []float32
*/
func ConvertInt16ToFloat32(in []int16) []float32 {
	out := make([]float32, len(in))
	for i, v := range in {
		out[i] = float32(v) / 32768.0
	}
	return out
}

/*
ConvertInt16BytesToFloat32 converts a byte slice containing little-endian
int16 audio samples to a slice of float32 [-1.0, 1.0].

	params:
	      in: raw bytes (2 bytes per sample, little-endian)
	returns:
	      []float32
*/
func ConvertInt16BytesToFloat32(in []byte) []float32 {
	out := make([]float32, len(in)/2)
	for i := 0; i < len(out); i++ {
		val := int16(in[2*i]) | int16(in[2*i+1])<<8
		out[i] = float32(val) / 32768.0
	}
	return out
}

/*
Resample performs linear interpolation resampling on interleaved float32 samples.

	params:
	      in:       input samples (interleaved)
	      inRate:   source sample rate
	      outRate:  target sample rate
	      channels: number of interleaved channels
	returns:
	      []float32: resampled output
*/
func Resample(in []float32, inRate, outRate, channels int) []float32 {
	if inRate == outRate || len(in) == 0 {
		return in
	}

	ratio := float64(inRate) / float64(outRate)
	inFrames := len(in) / channels
	outFrames := int(float64(inFrames) / ratio)

	out := make([]float32, outFrames*channels)

	for i := 0; i < outFrames; i++ {
		inPos := float64(i) * ratio
		inIdx := int(inPos)
		frac := float32(inPos - float64(inIdx))

		for c := 0; c < channels; c++ {
			idx1 := inIdx*channels + c
			idx2 := idx1 + channels

			var val1, val2 float32
			if idx1 < len(in) {
				val1 = in[idx1]
			}
			if idx2 < len(in) {
				val2 = in[idx2]
			} else {
				val2 = val1
			}

			out[i*channels+c] = val1 + frac*(val2-val1)
		}
	}

	return out
}

/*
ConvertChannels converts audio samples between different channel layouts.

	params:
	      in:    input samples (interleaved)
	      inCh:  source channel count
	      outCh: target channel count
	returns:
	      []float32
	Note: Currently supports Mono (1) to Stereo (2) and Stereo (2) to Mono (1).
*/
func ConvertChannels(in []float32, inCh, outCh int) []float32 {
	if inCh == outCh || len(in) == 0 {
		return in
	}

	if inCh == 1 && outCh == 2 {
		out := make([]float32, len(in)*2)
		for i, s := range in {
			out[2*i] = s
			out[2*i+1] = s
		}
		return out
	}

	if inCh == 2 && outCh == 1 {
		outFrames := len(in) / 2
		out := make([]float32, outFrames)
		for i := 0; i < outFrames; i++ {
			out[i] = (in[2*i] + in[2*i+1]) * 0.5
		}
		return out
	}

	return in
}
