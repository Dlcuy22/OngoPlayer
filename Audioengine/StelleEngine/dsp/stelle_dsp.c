// Audioengine/StelleEngine/dsp/stelle_dsp.c
// Implementation of high-performance audio utilities.
// This library has zero external dependencies and is structured for auto-vectorization (SIMD) under Clang.

#include "stelle_dsp.h"

EXPORT void resample_linear_c(const float* in, float* out, int in_rate, int out_rate, int channels, int in_frames, int out_frames) {
    if (in_rate == out_rate) {
        for (int i = 0; i < in_frames * channels; i++) {
            out[i] = in[i];
        }
        return;
    }
    double ratio = (double)in_rate / (double)out_rate;
    for (int i = 0; i < out_frames; i++) {
        double in_pos = i * ratio;
        int in_idx = (int)in_pos;
        float frac = (float)(in_pos - in_idx);
        for (int c = 0; c < channels; c++) {
            int idx1 = in_idx * channels + c;
            int idx2 = idx1 + channels;
            float val1 = in[idx1];
            float val2 = (idx2 < in_frames * channels) ? in[idx2] : val1;
            out[i * channels + c] = val1 + frac * (val2 - val1);
        }
    }
}

EXPORT void convert_channels_c(const float* in, float* out, int frames, int in_ch, int out_ch) {
    if (in_ch == out_ch) {
        for (int i = 0; i < frames * in_ch; i++) {
            out[i] = in[i];
        }
        return;
    }
    if (in_ch == 1 && out_ch == 2) {
        for (int i = 0; i < frames; i++) {
            out[2 * i] = in[i];
            out[2 * i + 1] = in[i];
        }
        return;
    }
    if (in_ch == 2 && out_ch == 1) {
        for (int i = 0; i < frames; i++) {
            out[i] = (in[2 * i] + in[2 * i + 1]) * 0.5f;
        }
        return;
    }
    if (in_ch > 2 && out_ch == 2) {
        for (int f = 0; f < frames; f++) {
            int base = f * in_ch;
            float left = 0.0f;
            float right = 0.0f;
            int ln = 0;
            int rn = 0;
            for (int c = 0; c < in_ch; c++) {
                if (c % 2 == 0) {
                    left += in[base + c];
                    ln++;
                } else {
                    right += in[base + c];
                    rn++;
                }
            }
            out[2 * f] = (ln > 0) ? (left / ln) : 0.0f;
            out[2 * f + 1] = (rn > 0) ? (right / rn) : 0.0f;
        }
        return;
    }
    
    // Fallback copy for other unhandled cases
    int min_ch = (in_ch < out_ch) ? in_ch : out_ch;
    for (int f = 0; f < frames; f++) {
        for (int c = 0; c < out_ch; c++) {
            if (c < min_ch) {
                out[f * out_ch + c] = in[f * in_ch + c];
            } else {
                out[f * out_ch + c] = 0.0f;
            }
        }
    }
}

EXPORT void apply_gain_c(float* samples, int num_samples, float volume) {
    for (int i = 0; i < num_samples; i++) {
        samples[i] *= volume;
    }
}
