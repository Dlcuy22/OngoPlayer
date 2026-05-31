// Audioengine/StelleEngine/utils_test.go
// Unit tests for the pure audio-processing helpers (no SDL, no codec libs).

package stelleengine

import "testing"

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
