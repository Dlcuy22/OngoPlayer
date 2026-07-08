#ifndef DSP_H
#define DSP_H

/* libdsp - pure C DSP transformation library
   no stdlib, no libm dependency
   all input/output: float

   compile with:
     gcc -O2 -ffast-math -fno-builtin -nostdlib -nostartfiles -ffreestanding (embedded)
     gcc -O2 -ffast-math (hosted)
*/

/* ---- constants ---- */
#define DSP_PI   3.14159265358979323846f
#define DSP_2PI  6.28318530717958647692f

/* ---- types ---- */
typedef unsigned int  dsp_uint;
typedef int           dsp_int;

typedef struct { float r, i; } dsp_complex;

/* ---- math (dsp_math.c) ---- */
float dsp_sinf(float x);
float dsp_cosf(float x);
float dsp_sqrtf(float x);
float dsp_absf(float x);
float dsp_powf_int(float base, dsp_int exp);
float dsp_log10f(float x);
float dsp_db_to_linear(float db);
float dsp_linear_to_db(float linear);
float dsp_clampf(float x, float lo, float hi);

/* ---- FFT (dsp_fft.c) ---- */
/* n must be power of 2, max DSP_FFT_MAX_SIZE */
#define DSP_FFT_MAX_SIZE  4096

/* in-place forward FFT, interleaved [re,im,re,im,...] */
void dsp_fft(float *buf, dsp_uint n);

/* in-place inverse FFT, normalized */
void dsp_ifft(float *buf, dsp_uint n);

/* real FFT: input n real floats, output n/2+1 complex magnitudes */
void dsp_rfft_magnitude(const float *in, float *mag_out, dsp_uint n);

/* compute power spectrum (mag squared, no sqrt, faster) */
void dsp_rfft_power(const float *in, float *pow_out, dsp_uint n);

/* ---- windowing (dsp_filter.c) ---- */
typedef enum {
    DSP_WIN_RECT     = 0,
    DSP_WIN_HANN     = 1,
    DSP_WIN_HAMMING  = 2,
    DSP_WIN_BLACKMAN = 3,
    DSP_WIN_FLAT_TOP = 4
} DspWindow;

/* apply window in-place */
void dsp_apply_window(float *buf, dsp_uint n, DspWindow w);

/* ---- biquad filter (dsp_filter.c) ---- */
typedef enum {
    DSP_FILT_PEAKING    = 0,
    DSP_FILT_LOW_SHELF  = 1,
    DSP_FILT_HIGH_SHELF = 2,
    DSP_FILT_LOW_PASS   = 3,
    DSP_FILT_HIGH_PASS  = 4,
    DSP_FILT_BAND_PASS  = 5,
    DSP_FILT_NOTCH      = 6
} DspFilterType;

typedef struct {
    float b0, b1, b2;
    float a1, a2;
} DspBiquadCoeff;

typedef struct {
    float w1, w2;   /* direct form II transposed state */
} DspBiquadState;

typedef struct {
    DspBiquadCoeff coeff;
    DspBiquadState state;
} DspBiquad;

void dsp_biquad_design(DspBiquad *bq, DspFilterType type,
                       float freq, float gain_db, float q, float fs);
float dsp_biquad_tick(DspBiquad *bq, float x);
void  dsp_biquad_process(DspBiquad *bq, const float *in, float *out, dsp_uint n);
void  dsp_biquad_reset(DspBiquad *bq);

/* ---- FIR filter (dsp_filter.c) ---- */
#define DSP_FIR_MAX_TAPS  256

typedef struct {
    float    coeffs[DSP_FIR_MAX_TAPS];
    float    delay[DSP_FIR_MAX_TAPS];
    dsp_uint taps;
    dsp_uint write_idx;
} DspFIR;

/* design windowed sinc lowpass, then mirror for highpass if hp=1 */
void  dsp_fir_design_lowpass(DspFIR *fir, float cutoff, float fs, dsp_uint taps, DspWindow w);
void  dsp_fir_design_highpass(DspFIR *fir, float cutoff, float fs, dsp_uint taps, DspWindow w);
float dsp_fir_tick(DspFIR *fir, float x);
void  dsp_fir_process(DspFIR *fir, const float *in, float *out, dsp_uint n);
void  dsp_fir_reset(DspFIR *fir);

/* ---- EQ processor (dsp_eq.c) ---- */
#define DSP_EQ_MAX_BANDS  32

typedef struct {
    DspBiquad  bands[DSP_EQ_MAX_BANDS];
    float      gain_db[DSP_EQ_MAX_BANDS];
    float      freq[DSP_EQ_MAX_BANDS];
    float      q[DSP_EQ_MAX_BANDS];
    DspFilterType type[DSP_EQ_MAX_BANDS];
    dsp_uint   band_count;
    float      sample_rate;
    float      master_gain;
} DspEQ;

void  dsp_eq_init(DspEQ *eq, float sample_rate);
void  dsp_eq_add_band(DspEQ *eq, DspFilterType type, float freq, float gain_db, float q);
void  dsp_eq_set_gain(DspEQ *eq, dsp_uint band, float gain_db);
void  dsp_eq_set_master(DspEQ *eq, float gain_db);
float dsp_eq_tick(DspEQ *eq, float x);
void  dsp_eq_process(DspEQ *eq, const float *in, float *out, dsp_uint n);
void  dsp_eq_reset(DspEQ *eq);

/* preset 10-band ISO */
void  dsp_eq_init_10band(DspEQ *eq, float sample_rate);
void  dsp_eq_init_15band(DspEQ *eq, float sample_rate);

/* ---- utility ---- */
void dsp_interleave(const float *re, const float *im, float *out, dsp_uint n);
void dsp_deinterleave(const float *in, float *re, float *im, dsp_uint n);

#endif /* DSP_H */
