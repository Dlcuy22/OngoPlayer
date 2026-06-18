// Audioengine/StelleEngine/dsp/stelle_dsp.h
// Header for zero-dependency high-performance audio utility library.
//
// Key Functions:
//   - resample_linear_c: Linear resampling on interleaved float32 samples.
//   - convert_channels_c: Convert layout formats (mono, stereo, downmix).
//   - apply_gain_c: In-place gain multiplication (volume scaling).

#ifndef STELLE_DSP_H
#define STELLE_DSP_H

#ifdef _WIN32
#define EXPORT __declspec(dllexport)
#else
#define EXPORT __attribute__((visibility("default")))
#endif

#ifdef __cplusplus
extern "C" {
#endif

EXPORT void resample_linear_c(const float* in, float* out, int in_rate, int out_rate, int channels, int in_frames, int out_frames);

EXPORT void convert_channels_c(const float* in, float* out, int frames, int in_ch, int out_ch);

EXPORT void apply_gain_c(float* samples, int num_samples, float volume);

#ifdef __cplusplus
}
#endif

#endif // STELLE_DSP_H
