// Audioengine/StelleEngine/utils_test.go
// Unit tests for the pure audio-processing helpers (no SDL, no codec libs).

package stelleengine

import (
	"testing"
	"unsafe"
)

func TestConvertInt16BytesToFloat32(t *testing.T) {
	// Little-endian int16: 0 -> 0.0, 16384 -> 0.5, -16384 -> -0.5.
	in := []byte{0x00, 0x00, 0x00, 0x40, 0x00, 0xC0}
	out := ConvertInt16BytesToFloat32(in)
	if len(out) != 3 {
		t.Fatalf("len = %d, want 3", len(out))
	}
	want := []float32{0.0, 0.5, -0.5}
	for i, w := range want {
		if d := out[i] - w; d > 1e-4 || d < -1e-4 {
			t.Errorf("out[%d] = %f, want %f", i, out[i], w)
		}
	}
}

func TestConvertChannelsMonoToStereo(t *testing.T) {
	in := []float32{1, 2, 3}
	out := ConvertChannels(in, 1, 2)
	want := []float32{1, 1, 2, 2, 3, 3}
	if len(out) != len(want) {
		t.Fatalf("len = %d, want %d", len(out), len(want))
	}
	for i := range want {
		if out[i] != want[i] {
			t.Errorf("out[%d] = %f, want %f", i, out[i], want[i])
		}
	}
}

func TestConvertChannelsStereoToMono(t *testing.T) {
	in := []float32{1, 3, 2, 4}
	out := ConvertChannels(in, 2, 1)
	want := []float32{2, 3}
	if len(out) != len(want) {
		t.Fatalf("len = %d, want %d", len(out), len(want))
	}
	for i := range want {
		if out[i] != want[i] {
			t.Errorf("out[%d] = %f, want %f", i, out[i], want[i])
		}
	}
}

func TestConvertChannelsSurroundToStereo(t *testing.T) {
	// 4-channel single frame: evens -> L, odds -> R.
	in := []float32{1, 2, 3, 4}
	out := ConvertChannels(in, 4, 2)
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
	// L = avg(1,3)=2, R = avg(2,4)=3.
	if out[0] != 2 || out[1] != 3 {
		t.Errorf("out = %v, want [2 3]", out)
	}
}

func TestConvertChannelsIdentity(t *testing.T) {
	in := []float32{1, 2, 3, 4}
	out := ConvertChannels(in, 2, 2)
	if &out[0] != &in[0] {
		t.Errorf("identity conversion should return the input slice unchanged")
	}
}

func TestResampleIdentity(t *testing.T) {
	in := []float32{1, 2, 3, 4}
	out := Resample(in, 48000, 48000, 2)
	if len(out) != len(in) {
		t.Fatalf("len = %d, want %d", len(out), len(in))
	}
}

func TestResampleDownThenLength(t *testing.T) {
	// 8 stereo frames at 96k -> ~4 frames at 48k.
	in := make([]float32, 8*2)
	out := Resample(in, 96000, 48000, 2)
	wantFrames := 4
	if len(out) != wantFrames*2 {
		t.Errorf("len = %d, want %d", len(out), wantFrames*2)
	}
}

func TestResampleUpProducesMoreFrames(t *testing.T) {
	// 441 stereo frames at 44100 -> 480 frames at 48000.
	in := make([]float32, 441*2)
	out := Resample(in, 44100, 48000, 2)
	wantFrames := 480
	if len(out) != wantFrames*2 {
		t.Errorf("len = %d, want %d", len(out), wantFrames*2)
	}
}

func TestCDSPBindings(t *testing.T) {
	// Load bindings
	if err := initDspBindings(); err != nil {
		t.Skipf("Skipping C DSP test, library not compiled/found: %v", err)
	}

	if resample_linear_c == nil || convert_channels_c == nil || apply_gain_c == nil {
		t.Fatal("DSP bindings loaded but functions are nil")
	}

	// 1. Test Resample C
	inResample := []float32{1.0, 2.0, 3.0, 4.0, 5.0, 6.0}
	goResampled := Resample(inResample, 44100, 48000, 2)
	
	inFrames := len(inResample) / 2
	ratio := 44100.0 / 48000.0
	outFrames := int(float64(inFrames) / ratio)
	cResampled := make([]float32, outFrames*2)
	
	resample_linear_c(
		uintptr(unsafe.Pointer(&inResample[0])),
		uintptr(unsafe.Pointer(&cResampled[0])),
		44100,
		48000,
		2,
		int32(inFrames),
		int32(outFrames),
	)

	if len(goResampled) != len(cResampled) {
		t.Fatalf("Resample size mismatch: Go=%d, C=%d", len(goResampled), len(cResampled))
	}
	for i := range goResampled {
		diff := goResampled[i] - cResampled[i]
		if diff > 1e-5 || diff < -1e-5 {
			t.Errorf("Resample output mismatch at [%d]: Go=%f, C=%f", i, goResampled[i], cResampled[i])
		}
	}

	// 2. Test Convert Channels C
	inChannels := []float32{1.0, 2.0, 3.0, 4.0}
	goConv := ConvertChannels(inChannels, 2, 1)
	cConv := make([]float32, 2)
	
	convert_channels_c(
		uintptr(unsafe.Pointer(&inChannels[0])),
		uintptr(unsafe.Pointer(&cConv[0])),
		2,
		2,
		1,
	)

	for i := range goConv {
		if goConv[i] != cConv[i] {
			t.Errorf("ConvertChannels output mismatch at [%d]: Go=%f, C=%f", i, goConv[i], cConv[i])
		}
	}

	// 3. Test Apply Gain C
	samples := []float32{1.0, 2.0, 3.0, 4.0}
	apply_gain_c(
		uintptr(unsafe.Pointer(&samples[0])),
		int32(len(samples)),
		0.5,
	)
	
	wantSamples := []float32{0.5, 1.0, 1.5, 2.0}
	for i := range samples {
		if samples[i] != wantSamples[i] {
			t.Errorf("apply_gain_c output mismatch at [%d]: got %f, want %f", i, samples[i], wantSamples[i])
		}
	}
}

