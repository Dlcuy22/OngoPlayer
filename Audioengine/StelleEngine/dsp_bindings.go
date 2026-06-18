// Audioengine/StelleEngine/dsp_bindings.go
// Purego bindings to load and link the custom C DSP library (stelle_dsp).
//
// Key Components:
//   - initDspBindings: initializes and registers bindings once on startup.
//   - resample_linear_c: compiled C resampler.
//   - convert_channels_c: compiled C channel layout converter.
//   - apply_gain_c: compiled C volume scaler.

package stelleengine

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/dlcuy22/OngoPlayer/Audioengine/loader"
	"github.com/ebitengine/purego"
)

var (
	dspOnce    sync.Once
	dspInitErr error

	resample_linear_c  func(in, out uintptr, in_rate, out_rate, channels, in_frames, out_frames int32)
	convert_channels_c func(in, out uintptr, frames, in_ch, out_ch int32)
	apply_gain_c       func(samples uintptr, num_samples int32, volume float32)
)

/*
initDspBindings loads the custom C DSP shared library (stelle_dsp) and registers the symbols.
*/
func initDspBindings() error {
	dspOnce.Do(func() {
		var filename string
		switch runtime.GOOS {
		case "linux", "freebsd":
			filename = "stelle_dsp.so"
		case "windows":
			filename = "stelle_dsp.dll"
		case "darwin":
			filename = "stelle_dsp.dylib"
		default:
			dspInitErr = fmt.Errorf("unsupported platform for C DSP: %s", runtime.GOOS)
			return
		}

		lib, err := loader.Load(filename)
		if err != nil {
			dspInitErr = fmt.Errorf("failed to load custom DSP library %s: %w", filename, err)
			return
		}

		purego.RegisterLibFunc(&resample_linear_c, lib, "resample_linear_c")
		purego.RegisterLibFunc(&convert_channels_c, lib, "convert_channels_c")
		purego.RegisterLibFunc(&apply_gain_c, lib, "apply_gain_c")
	})
	return dspInitErr
}
