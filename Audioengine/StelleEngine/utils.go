// AudioEngine/StelleEngine/utils.go
// This file contains audio processing utilities for the engine, primarily focused on
// format conversion, resampling, and channel manipulation to prepare decoded audio for playback.
//
// Functions:
//   - ConvertInt16ToFloat32: Converts a slice of int16 audio samples to a slice of float32 samples [-1.0, 1.0].
//   - ConvertInt16BytesToFloat32: Converts a byte slice containing little-endian int16 audio samples to a slice of float32 samples.
//   - Resample: Performs linear interpolation resampling on a slice of float32 samples from any rate to a target rate.
//   - ConvertChannels: Converts a slice of float32 samples between different channel counts (e.g., Mono to Stereo).

package stelleengine

func ConvertInt16ToFloat32(in []int16) []float32 {
	out := make([]float32, len(in))
	for i, v := range in {
		out[i] = float32(v) / 32768.0
	}
	return out
}

func ConvertInt16BytesToFloat32(in []byte) []float32 {
	out := make([]float32, len(in)/2)
	for i := 0; i < len(out); i++ {
		val := int16(in[2*i]) | int16(in[2*i+1])<<8
		out[i] = float32(val) / 32768.0
	}
	return out
}

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

// ConvertChannels converts audio samples between different channel layouts.
// Currently supports Mono (1) to Stereo (2) and Stereo (2) to Mono (1).
func ConvertChannels(in []float32, inCh, outCh int) []float32 {
	if inCh == outCh || len(in) == 0 {
		return in
	}

	if inCh == 1 && outCh == 2 {
		// Mono to Stereo: duplicate samples
		out := make([]float32, len(in)*2)
		for i, s := range in {
			out[2*i] = s
			out[2*i+1] = s
		}
		return out
	}

	if inCh == 2 && outCh == 1 {
		// Stereo to Mono: average left and right
		outFrames := len(in) / 2
		out := make([]float32, outFrames)
		for i := 0; i < outFrames; i++ {
			out[i] = (in[2*i] + in[2*i+1]) * 0.5
		}
		return out
	}

	return in
}
