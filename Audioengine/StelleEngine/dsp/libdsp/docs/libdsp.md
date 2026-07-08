# libdsp — Single-Header DSP Library

## Portability

`libdsp.h` is a single-header C library with no code dependencies (`stdlib`, `libm`, etc.). It only needs `<stddef.h>` for `size_t` and standard `double`/`float` types.

### Usage

```c
// exactly ONE .c file in your project:
#define LIBDSP_IMPLEMENTATION
#include "libdsp.h"

// all other files (header only — no implementation code):
#include "libdsp.h"
```

### Build

```sh
# Hosted (desktop):
gcc -O2 -ffast-math main.c player.c -o player

# Embedded / bare-metal:
gcc -O2 -ffast-math -fno-builtin -nostdlib -nostartfiles -ffreestanding \
    -c my_dsp_code.c
```

No `-lm`, no `-ldsp`, no link-time dependencies of any kind. If `ffast-math` is undesirable, omit it — the library still works correctly, though `dsp_sqrtf` uses an NR iteration that expects fast denormals.

## API Reference

### Constants

| Macro | Value |
|-------|-------|
| `DSP_PI` | 3.14159274f |
| `DSP_2PI` | 6.28318548f |
| `DSP_FFT_MAX_SIZE` | 4096 |
| `DSP_FIR_MAX_TAPS` | 256 |
| `DSP_EQ_MAX_BANDS` | 32 |

### Types

| Name | Description |
|------|-------------|
| `dsp_uint` | `unsigned int` |
| `dsp_int` | `int` |
| `dsp_complex` | `{ float r, i; }` |

### Math (`dsp_math.c`)

```c
float dsp_sinf(float x);       // minimax polynomial, error < 5e-7 in [-pi, pi]
float dsp_cosf(float x);       // via dsp_sinf(x + pi/2)
float dsp_sqrtf(float x);      // 2 NR iterations on fast inverse sqrt, error ~1e-6
float dsp_absf(float x);       // bitmask sign bit
float dsp_powf_int(float base, dsp_int exp);  // exponentiation by squaring
float dsp_log10f(float x);     // IEEE-754 bit extraction + polynomial, no libm
float dsp_db_to_linear(float db);   // 10^(db/20)
float dsp_linear_to_db(float linear); // 20 * log10(linear)
float dsp_clampf(float x, float lo, float hi);
```

### FFT (`dsp_fft.c`)

All in-place, interleaved `[re, im, re, im, ...]`. `n` must be a power of 2 ≤ `DSP_FFT_MAX_SIZE`.

```c
void dsp_fft(float *buf, dsp_uint n);         // forward
void dsp_ifft(float *buf, dsp_uint n);         // inverse (normalized)
void dsp_rfft_magnitude(const float *in, float *mag_out, dsp_uint n);
void dsp_rfft_power(const float *in, float *pow_out, dsp_uint n);
```

### Windowing (`dsp_filter.c`)

```c
typedef enum {
    DSP_WIN_RECT, DSP_WIN_HANN, DSP_WIN_HAMMING,
    DSP_WIN_BLACKMAN, DSP_WIN_FLAT_TOP
} DspWindow;

void dsp_apply_window(float *buf, dsp_uint n, DspWindow w);
```

### Biquad IIR (`dsp_filter.c`)

Direct Form II Transposed. 2 multiply-accumulates per sample.

```c
typedef struct { float b0, b1, b2, a1, a2; } DspBiquadCoeff;
typedef struct { float w1, w2; } DspBiquadState;
typedef struct { DspBiquadCoeff coeff; DspBiquadState state; } DspBiquad;

typedef enum {
    DSP_FILT_PEAKING, DSP_FILT_LOW_SHELF, DSP_FILT_HIGH_SHELF,
    DSP_FILT_LOW_PASS, DSP_FILT_HIGH_PASS,
    DSP_FILT_BAND_PASS, DSP_FILT_NOTCH
} DspFilterType;

void  dsp_biquad_design(DspBiquad *bq, DspFilterType type,
                        float freq, float gain_db, float q, float fs);
float dsp_biquad_tick(DspBiquad *bq, float x);
void  dsp_biquad_process(DspBiquad *bq, const float *in, float *out, dsp_uint n);
void  dsp_biquad_reset(DspBiquad *bq);
```

### FIR (`dsp_filter.c`)

Windowed-sinc design, symmetric odd-length (Type I).

```c
typedef struct {
    float coeffs[DSP_FIR_MAX_TAPS];
    float delay[DSP_FIR_MAX_TAPS];
    dsp_uint taps;
    dsp_uint write_idx;
} DspFIR;

void  dsp_fir_design_lowpass(DspFIR *fir, float cutoff, float fs,
                             dsp_uint taps, DspWindow w);
void  dsp_fir_design_highpass(DspFIR *fir, float cutoff, float fs,
                              dsp_uint taps, DspWindow w);
float dsp_fir_tick(DspFIR *fir, float x);
void  dsp_fir_process(DspFIR *fir, const float *in, float *out, dsp_uint n);
void  dsp_fir_reset(DspFIR *fir);
```

### EQ (`dsp_eq.c`)

Cascaded biquad bands with a master gain. Up to `DSP_EQ_MAX_BANDS` (32).

```c
typedef struct {
    DspBiquad      bands[DSP_EQ_MAX_BANDS];
    float          gain_db[DSP_EQ_MAX_BANDS];
    float          freq[DSP_EQ_MAX_BANDS];
    float          q[DSP_EQ_MAX_BANDS];
    DspFilterType  type[DSP_EQ_MAX_BANDS];
    dsp_uint       band_count;
    float          sample_rate;
    float          master_gain;
} DspEQ;

void  dsp_eq_init(DspEQ *eq, float sample_rate);
void  dsp_eq_add_band(DspEQ *eq, DspFilterType type,
                      float freq, float gain_db, float q);
void  dsp_eq_set_gain(DspEQ *eq, dsp_uint band, float gain_db);
void  dsp_eq_set_master(DspEQ *eq, float gain_db);
float dsp_eq_tick(DspEQ *eq, float x);
void  dsp_eq_process(DspEQ *eq, const float *in, float *out, dsp_uint n);
void  dsp_eq_reset(DspEQ *eq);
void  dsp_eq_init_10band(DspEQ *eq, float sample_rate);
void  dsp_eq_init_15band(DspEQ *eq, float sample_rate);
```

### Utility (`dsp_math.c`)

```c
void dsp_interleave(const float *re, const float *im, float *out, dsp_uint n);
void dsp_deinterleave(const float *in, float *re, float *im, dsp_uint n);
```

## Common Issues

### 1. `dsp_log10f` / `dsp_linear_to_db` returns `-144.0f` for zero or negative input

This is intentional to avoid `-inf`. Check before calling if you need different behavior.

### 2. FFT `n` must be power of 2 and ≤ `DSP_FFT_MAX_SIZE` (4096)

There is no runtime check — passing an invalid size writes past the static scratch buffer. Always validate your FFT size.

### 3. FIR `taps` must be ≥ 3

Passing `taps = 0` wraps the unsigned count to `UINT_MAX`, causing catastrophic array overrun. The minimum usable value is 3 (odd length).

### 4. Biquad `q ≤ 0` or `freq ≥ fs/2` produces NaN coefficients

Always pass a positive Q and keep frequency below Nyquist. There is no validation in the design functions.

### 5. Static scratch buffers in `dsp_rfft_magnitude` / `dsp_rfft_power`

These functions use a `static float scratch[DSP_FFT_MAX_SIZE * 2]` buffer, making them **non-reentrant and thread-unsafe**. Do not call them from multiple threads simultaneously, and never with `n > DSP_FFT_MAX_SIZE`.

### 6. `dsp_sinf` range reduction loses precision for |x| > 1e7

The float-precision range reduction catastrophically cancels for very large phase accumulators. In normal audio use (filter coefficients, FFT twiddles) this never triggers. If you need to track phase over millions of samples, wrap manually.

### 7. EQ changes do not flush signal state

Changing a biquad coefficient via `dsp_eq_set_gain` or `dsp_biquad_design` does **not** flush the filter's internal state (`w1`, `w2`). This is intentional — it avoids a pop/click. But the new coefficients take effect immediately on the next sample, which may cause a brief transient if the state was large. Call `dsp_eq_reset` if you want a clean slate.

### 8. Real-world bug: EQ + audio stream interaction

When using libdsp with an audio framework that queues samples (SDL, PortAudio, JACK), see [`docs/eq-seek-bug.md`](eq-seek-bug.md) for a detailed postmortem.

The short version: changing EQ gain calls `dsp_eq_set_gain`, which recomputes coefficients. If the audio output has a deep queue, the change is delayed by queue depth. Do **not** clear the audio stream or re-read the decoder on EQ change — this causes seek artifacts. Instead, cap the stream queue and let the new coefficients drain naturally.
