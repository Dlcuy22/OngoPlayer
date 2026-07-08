#ifndef LIBDSP_H
#define LIBDSP_H

/*
 * libdsp - single-header pure C DSP library
 *   no stdlib, no libm dependency
 *   all input/output: float
 *
 * Usage:
 *   #define LIBDSP_IMPLEMENTATION
 *   #include "libdsp.h"
 *
 * Compile:
 *   gcc -O2 -ffast-math (hosted)
 *   gcc -O2 -ffast-math -fno-builtin -nostdlib -nostartfiles -ffreestanding (embedded)
 */

/* ---- constants ---- */
#define DSP_PI   3.14159265358979323846f
#define DSP_2PI  6.28318530717958647692f

/* ---- types ---- */
typedef unsigned int  dsp_uint;
typedef int           dsp_int;

typedef struct { float r, i; } dsp_complex;

/* ---- math ---- */
float dsp_sinf(float x);
float dsp_cosf(float x);
float dsp_sqrtf(float x);
float dsp_absf(float x);
float dsp_powf_int(float base, dsp_int exp);
float dsp_log10f(float x);
float dsp_db_to_linear(float db);
float dsp_linear_to_db(float linear);
float dsp_clampf(float x, float lo, float hi);

/* ---- FFT ---- */
#define DSP_FFT_MAX_SIZE  4096

void dsp_fft(float *buf, dsp_uint n);
void dsp_ifft(float *buf, dsp_uint n);
void dsp_rfft_magnitude(const float *in, float *mag_out, dsp_uint n);
void dsp_rfft_power(const float *in, float *pow_out, dsp_uint n);

/* ---- windowing ---- */
typedef enum {
    DSP_WIN_RECT     = 0,
    DSP_WIN_HANN     = 1,
    DSP_WIN_HAMMING  = 2,
    DSP_WIN_BLACKMAN = 3,
    DSP_WIN_FLAT_TOP = 4
} DspWindow;

void dsp_apply_window(float *buf, dsp_uint n, DspWindow w);

/* ---- biquad filter ---- */
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
    float w1, w2;
} DspBiquadState;

typedef struct {
    DspBiquadCoeff coeff;
    DspBiquadState state;
} DspBiquad;

void  dsp_biquad_design(DspBiquad *bq, DspFilterType type,
                        float freq, float gain_db, float q, float fs);
float dsp_biquad_tick(DspBiquad *bq, float x);
void  dsp_biquad_process(DspBiquad *bq, const float *in, float *out, dsp_uint n);
void  dsp_biquad_reset(DspBiquad *bq);

/* ---- FIR filter ---- */
#define DSP_FIR_MAX_TAPS  256

typedef struct {
    float    coeffs[DSP_FIR_MAX_TAPS];
    float    delay[DSP_FIR_MAX_TAPS];
    dsp_uint taps;
    dsp_uint write_idx;
} DspFIR;

void  dsp_fir_design_lowpass(DspFIR *fir, float cutoff, float fs, dsp_uint taps, DspWindow w);
void  dsp_fir_design_highpass(DspFIR *fir, float cutoff, float fs, dsp_uint taps, DspWindow w);
float dsp_fir_tick(DspFIR *fir, float x);
void  dsp_fir_process(DspFIR *fir, const float *in, float *out, dsp_uint n);
void  dsp_fir_reset(DspFIR *fir);

/* ---- EQ processor ---- */
#define DSP_EQ_MAX_BANDS  32

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
void  dsp_eq_add_band(DspEQ *eq, DspFilterType type, float freq, float gain_db, float q);
void  dsp_eq_set_gain(DspEQ *eq, dsp_uint band, float gain_db);
void  dsp_eq_set_master(DspEQ *eq, float gain_db);
float dsp_eq_tick(DspEQ *eq, float x);
void  dsp_eq_process(DspEQ *eq, const float *in, float *out, dsp_uint n);
void  dsp_eq_reset(DspEQ *eq);
void  dsp_eq_init_10band(DspEQ *eq, float sample_rate);
void  dsp_eq_init_15band(DspEQ *eq, float sample_rate);

/* ---- utility ---- */
void dsp_interleave(const float *re, const float *im, float *out, dsp_uint n);
void dsp_deinterleave(const float *in, float *re, float *im, dsp_uint n);

/* ====================================================================
 * Implementation (define LIBDSP_IMPLEMENTATION before including)
 * ==================================================================== */
#ifdef LIBDSP_IMPLEMENTATION

/* ---- math ---- */

#define DSP_PI_2  1.57079632679489661923f

static float _sin_core(float x) {
    float x2 = x * x;
    return x * (1.0f
        - x2 * (1.6666667e-1f
        - x2 * (8.3333337e-3f
        - x2 * (1.9841270e-4f
        - x2 *  2.7557319e-6f))));
}

float dsp_sinf(float x) {
    float q = (float)(dsp_int)(x / DSP_2PI);
    x -= q * DSP_2PI;
    if (x > DSP_PI)  x -= DSP_2PI;
    if (x < -DSP_PI) x += DSP_2PI;
    if (x > DSP_PI_2)  x =  DSP_PI - x;
    if (x < -DSP_PI_2) x = -DSP_PI - x;
    return _sin_core(x);
}

float dsp_cosf(float x) {
    return dsp_sinf(x + DSP_PI_2);
}

static float _inv_sqrtf(float x) {
    union { float f; unsigned int u; } v;
    v.f = x;
    v.u = 0x5F3759DFu - (v.u >> 1);
    v.f *= (1.5f - 0.5f * x * v.f * v.f);
    v.f *= (1.5f - 0.5f * x * v.f * v.f);
    return v.f;
}

float dsp_sqrtf(float x) {
    if (x <= 0.0f) return 0.0f;
    return x * _inv_sqrtf(x);
}

float dsp_absf(float x) {
    union { float f; unsigned int u; } v;
    v.f = x;
    v.u &= 0x7FFFFFFFu;
    return v.f;
}

float dsp_clampf(float x, float lo, float hi) {
    if (x < lo) return lo;
    if (x > hi) return hi;
    return x;
}

float dsp_powf_int(float base, dsp_int exp) {
    float result = 1.0f;
    int   neg    = (exp < 0);
    dsp_uint e   = (dsp_uint)(neg ? -exp : exp);
    while (e) {
        if (e & 1) result *= base;
        base *= base;
        e >>= 1;
    }
    return neg ? 1.0f / result : result;
}

float dsp_log10f(float x) {
    if (x <= 0.0f) return -144.0f;
    union { float f; unsigned u; } v = { .f = x };
    float exp = (float)((int)(v.u >> 23) - 127);
    v.u = (v.u & 0x007FFFFFu) | 0x3F800000u;
    float m = v.f - 1.0f;
    float log2_m = m * (1.4426950408889634f
                  + m * (-0.7213475204444818f
                  + m * (0.4808983469629872f
                  + m * (-0.3606740294316674f
                  + m * 0.288539974f))));
    return (exp + log2_m) * 0.30102999566398119521373889472449f;
}

float dsp_linear_to_db(float linear) {
    if (linear <= 0.0f) return -144.0f;
    return 20.0f * dsp_log10f(linear);
}

float dsp_db_to_linear(float db) {
    float exp2_val = db * (3.321928f / 20.0f);
    dsp_int intpart = (dsp_int)exp2_val;
    float frac = exp2_val - (float)intpart;
    float frac_pow = 1.0f + frac * (0.6931472f + frac * (0.2402265f + frac * 0.0555041f));
    union { float f; unsigned int u; } v;
    v.u = (dsp_uint)(intpart + 127) << 23;
    return v.f * frac_pow;
}

void dsp_interleave(const float *re, const float *im, float *out, dsp_uint n) {
    for (dsp_uint i = 0; i < n; i++) {
        out[2*i]   = re[i];
        out[2*i+1] = im[i];
    }
}

void dsp_deinterleave(const float *in, float *re, float *im, dsp_uint n) {
    for (dsp_uint i = 0; i < n; i++) {
        re[i] = in[2*i];
        im[i] = in[2*i+1];
    }
}

/* ---- FFT ---- */

static void _bit_reverse(float *buf, dsp_uint n) {
    dsp_uint j = 0;
    for (dsp_uint i = 1; i < n; i++) {
        dsp_uint bit = n >> 1;
        while (j & bit) { j ^= bit; bit >>= 1; }
        j ^= bit;
        if (i < j) {
            float tr = buf[2*i];   buf[2*i]   = buf[2*j];   buf[2*j]   = tr;
            float ti = buf[2*i+1]; buf[2*i+1] = buf[2*j+1]; buf[2*j+1] = ti;
        }
    }
}

static void _fft_core(float *buf, dsp_uint n, int inverse) {
    _bit_reverse(buf, n);
    for (dsp_uint s = 1; s < n; s <<= 1) {
        dsp_uint m   = s << 1;
        float    ang = (inverse ? DSP_2PI : -DSP_2PI) / (float)m;
        float    wr  = dsp_cosf(ang);
        float    wi  = dsp_sinf(ang);
        for (dsp_uint k = 0; k < n; k += m) {
            float cur_wr = 1.0f, cur_wi = 0.0f;
            for (dsp_uint j = 0; j < s; j++) {
                dsp_uint e = 2*(k+j), o = 2*(k+j+s);
                float tr = cur_wr*buf[o]   - cur_wi*buf[o+1];
                float ti = cur_wr*buf[o+1] + cur_wi*buf[o];
                buf[o]   = buf[e]   - tr;
                buf[o+1] = buf[e+1] - ti;
                buf[e]  += tr;
                buf[e+1]+= ti;
                float nwr = cur_wr*wr - cur_wi*wi;
                cur_wi    = cur_wr*wi + cur_wi*wr;
                cur_wr    = nwr;
            }
        }
    }
}

void dsp_fft(float *buf, dsp_uint n) {
    _fft_core(buf, n, 0);
}

void dsp_ifft(float *buf, dsp_uint n) {
    _fft_core(buf, n, 1);
    float norm = 1.0f / (float)n;
    for (dsp_uint i = 0; i < 2*n; i++) buf[i] *= norm;
}

void dsp_rfft_magnitude(const float *in, float *mag_out, dsp_uint n) {
    static float scratch[DSP_FFT_MAX_SIZE * 2];
    for (dsp_uint i = 0; i < n; i++) {
        scratch[2*i]   = in[i];
        scratch[2*i+1] = 0.0f;
    }
    _fft_core(scratch, n, 0);
    for (dsp_uint k = 0; k <= n/2; k++) {
        float re = scratch[2*k];
        float im = scratch[2*k+1];
        mag_out[k] = dsp_sqrtf(re*re + im*im);
    }
}

void dsp_rfft_power(const float *in, float *pow_out, dsp_uint n) {
    static float scratch[DSP_FFT_MAX_SIZE * 2];
    for (dsp_uint i = 0; i < n; i++) {
        scratch[2*i]   = in[i];
        scratch[2*i+1] = 0.0f;
    }
    _fft_core(scratch, n, 0);
    for (dsp_uint k = 0; k <= n/2; k++) {
        float re = scratch[2*k];
        float im = scratch[2*k+1];
        pow_out[k] = re*re + im*im;
    }
}

/* ---- windowing, biquad, FIR ---- */

void dsp_apply_window(float *buf, dsp_uint n, DspWindow w) {
    float N1 = (float)(n - 1);
    for (dsp_uint i = 0; i < n; i++) {
        float t = (float)i / N1;
        float coef;
        switch (w) {
            case DSP_WIN_RECT:    coef = 1.0f; break;
            case DSP_WIN_HANN:    coef = 0.5f - 0.5f * dsp_cosf(DSP_2PI * t); break;
            case DSP_WIN_HAMMING: coef = 0.54f - 0.46f * dsp_cosf(DSP_2PI * t); break;
            case DSP_WIN_BLACKMAN:
                coef = 0.42f - 0.5f*dsp_cosf(DSP_2PI*t) + 0.08f*dsp_cosf(2.0f*DSP_2PI*t);
                break;
            case DSP_WIN_FLAT_TOP:
                coef = 0.21557895f - 0.41663158f*dsp_cosf(DSP_2PI*t)
                     + 0.27726316f*dsp_cosf(2.0f*DSP_2PI*t)
                     - 0.08357895f*dsp_cosf(3.0f*DSP_2PI*t)
                     + 0.00694737f*dsp_cosf(4.0f*DSP_2PI*t);
                break;
            default: coef = 1.0f;
        }
        buf[i] *= coef;
    }
}

static void _design_peaking(DspBiquadCoeff *c, float freq, float gain_db, float q, float fs) {
    float A   = dsp_db_to_linear(gain_db * 0.5f);
    float w0  = DSP_2PI * freq / fs;
    float cw  = dsp_cosf(w0);
    float sw  = dsp_sinf(w0);
    float al  = sw / (2.0f * q);
    float inv = 1.0f / (1.0f + al / A);
    c->b0 = (1.0f + al * A) * inv;
    c->b1 = (-2.0f * cw)    * inv;
    c->b2 = (1.0f - al * A) * inv;
    c->a1 = (-2.0f * cw)    * inv;
    c->a2 = (1.0f - al / A) * inv;
}

static void _design_low_shelf(DspBiquadCoeff *c, float freq, float gain_db, float q, float fs) {
    float A   = dsp_db_to_linear(gain_db * 0.5f);
    float w0  = DSP_2PI * freq / fs;
    float cw  = dsp_cosf(w0);
    float sw  = dsp_sinf(w0);
    float sqA = dsp_sqrtf(A);
    float al  = sw / (2.0f * q);
    float inv = 1.0f / ((A+1.0f) + (A-1.0f)*cw + 2.0f*sqA*al);
    c->b0 =  A * ((A+1.0f) - (A-1.0f)*cw + 2.0f*sqA*al) * inv;
    c->b1 =  2.0f*A*((A-1.0f) - (A+1.0f)*cw)            * inv;
    c->b2 =  A * ((A+1.0f) - (A-1.0f)*cw - 2.0f*sqA*al) * inv;
    c->a1 = -2.0f*((A-1.0f) + (A+1.0f)*cw)              * inv;
    c->a2 =  ((A+1.0f) + (A-1.0f)*cw - 2.0f*sqA*al)     * inv;
}

static void _design_high_shelf(DspBiquadCoeff *c, float freq, float gain_db, float q, float fs) {
    float A   = dsp_db_to_linear(gain_db * 0.5f);
    float w0  = DSP_2PI * freq / fs;
    float cw  = dsp_cosf(w0);
    float sw  = dsp_sinf(w0);
    float sqA = dsp_sqrtf(A);
    float al  = sw / (2.0f * q);
    float inv = 1.0f / ((A+1.0f) - (A-1.0f)*cw + 2.0f*sqA*al);
    c->b0 =  A * ((A+1.0f) + (A-1.0f)*cw + 2.0f*sqA*al) * inv;
    c->b1 = -2.0f*A*((A-1.0f) + (A+1.0f)*cw)            * inv;
    c->b2 =  A * ((A+1.0f) + (A-1.0f)*cw - 2.0f*sqA*al) * inv;
    c->a1 =  2.0f*((A-1.0f) - (A+1.0f)*cw)              * inv;
    c->a2 =  ((A+1.0f) - (A-1.0f)*cw - 2.0f*sqA*al)     * inv;
}

static void _design_low_pass(DspBiquadCoeff *c, float freq, float q, float fs) {
    float w0  = DSP_2PI * freq / fs;
    float cw  = dsp_cosf(w0);
    float sw  = dsp_sinf(w0);
    float al  = sw / (2.0f * q);
    float inv = 1.0f / (1.0f + al);
    c->b0 = ((1.0f - cw) * 0.5f) * inv;
    c->b1 =  (1.0f - cw)         * inv;
    c->b2 = ((1.0f - cw) * 0.5f) * inv;
    c->a1 = (-2.0f * cw)         * inv;
    c->a2 =  (1.0f - al)         * inv;
}

static void _design_high_pass(DspBiquadCoeff *c, float freq, float q, float fs) {
    float w0  = DSP_2PI * freq / fs;
    float cw  = dsp_cosf(w0);
    float sw  = dsp_sinf(w0);
    float al  = sw / (2.0f * q);
    float inv = 1.0f / (1.0f + al);
    c->b0 =  ((1.0f + cw) * 0.5f) * inv;
    c->b1 = -(1.0f + cw)          * inv;
    c->b2 =  ((1.0f + cw) * 0.5f) * inv;
    c->a1 = (-2.0f * cw)          * inv;
    c->a2 =  (1.0f - al)          * inv;
}

static void _design_band_pass(DspBiquadCoeff *c, float freq, float q, float fs) {
    float w0  = DSP_2PI * freq / fs;
    float cw  = dsp_cosf(w0);
    float sw  = dsp_sinf(w0);
    float al  = sw / (2.0f * q);
    float inv = 1.0f / (1.0f + al);
    c->b0 =  al         * inv;
    c->b1 =  0.0f;
    c->b2 = -al         * inv;
    c->a1 = (-2.0f*cw)  * inv;
    c->a2 =  (1.0f - al)* inv;
}

static void _design_notch(DspBiquadCoeff *c, float freq, float q, float fs) {
    float w0  = DSP_2PI * freq / fs;
    float cw  = dsp_cosf(w0);
    float sw  = dsp_sinf(w0);
    float al  = sw / (2.0f * q);
    float inv = 1.0f / (1.0f + al);
    c->b0 =  1.0f        * inv;
    c->b1 = (-2.0f * cw) * inv;
    c->b2 =  1.0f        * inv;
    c->a1 = (-2.0f * cw) * inv;
    c->a2 =  (1.0f - al) * inv;
}

void dsp_biquad_design(DspBiquad *bq, DspFilterType type,
                       float freq, float gain_db, float q, float fs) {
    switch (type) {
        case DSP_FILT_PEAKING:   _design_peaking(&bq->coeff, freq, gain_db, q, fs); break;
        case DSP_FILT_LOW_SHELF: _design_low_shelf(&bq->coeff, freq, gain_db, q, fs); break;
        case DSP_FILT_HIGH_SHELF:_design_high_shelf(&bq->coeff, freq, gain_db, q, fs); break;
        case DSP_FILT_LOW_PASS:  _design_low_pass(&bq->coeff, freq, q, fs); break;
        case DSP_FILT_HIGH_PASS: _design_high_pass(&bq->coeff, freq, q, fs); break;
        case DSP_FILT_BAND_PASS: _design_band_pass(&bq->coeff, freq, q, fs); break;
        case DSP_FILT_NOTCH:     _design_notch(&bq->coeff, freq, q, fs); break;
    }
    dsp_biquad_reset(bq);
}

float dsp_biquad_tick(DspBiquad *bq, float x) {
    float y     = bq->coeff.b0 * x + bq->state.w1;
    bq->state.w1 = bq->coeff.b1 * x - bq->coeff.a1 * y + bq->state.w2;
    bq->state.w2 = bq->coeff.b2 * x - bq->coeff.a2 * y;
    return y;
}

void dsp_biquad_process(DspBiquad *bq, const float *in, float *out, dsp_uint n) {
    for (dsp_uint i = 0; i < n; i++)
        out[i] = dsp_biquad_tick(bq, in[i]);
}

void dsp_biquad_reset(DspBiquad *bq) {
    bq->state.w1 = 0.0f;
    bq->state.w2 = 0.0f;
}

/* ---- FIR filter ---- */

static float _sinc(float x) {
    if (x == 0.0f) return 1.0f;
    float px = DSP_PI * x;
    return dsp_sinf(px) / px;
}

void dsp_fir_design_lowpass(DspFIR *fir, float cutoff, float fs,
                             dsp_uint taps, DspWindow w) {
    if (taps > DSP_FIR_MAX_TAPS) taps = DSP_FIR_MAX_TAPS;
    if (!(taps & 1)) taps--;
    fir->taps      = taps;
    fir->write_idx = 0;
    float fc   = cutoff / fs;
    dsp_int M  = (dsp_int)(taps - 1);
    for (dsp_uint i = 0; i < taps; i++) {
        float n = (float)i - (float)M * 0.5f;
        fir->coeffs[i] = 2.0f * fc * _sinc(2.0f * fc * n);
        fir->delay[i]  = 0.0f;
    }
    dsp_apply_window(fir->coeffs, taps, w);
}

void dsp_fir_design_highpass(DspFIR *fir, float cutoff, float fs,
                              dsp_uint taps, DspWindow w) {
    dsp_fir_design_lowpass(fir, cutoff, fs, taps, w);
    dsp_int M = (dsp_int)(taps - 1);
    for (dsp_uint i = 0; i < taps; i++)
        fir->coeffs[i] = -fir->coeffs[i];
    fir->coeffs[M / 2] += 1.0f;
}

float dsp_fir_tick(DspFIR *fir, float x) {
    fir->delay[fir->write_idx] = x;
    float acc = 0.0f;
    dsp_uint idx = fir->write_idx;
    for (dsp_uint i = 0; i < fir->taps; i++) {
        acc += fir->coeffs[i] * fir->delay[idx];
        if (idx == 0) idx = fir->taps - 1;
        else          idx--;
    }
    fir->write_idx++;
    if (fir->write_idx >= fir->taps) fir->write_idx = 0;
    return acc;
}

void dsp_fir_process(DspFIR *fir, const float *in, float *out, dsp_uint n) {
    for (dsp_uint i = 0; i < n; i++)
        out[i] = dsp_fir_tick(fir, in[i]);
}

void dsp_fir_reset(DspFIR *fir) {
    for (dsp_uint i = 0; i < fir->taps; i++)
        fir->delay[i] = 0.0f;
    fir->write_idx = 0;
}

/* ---- EQ ---- */

void dsp_eq_init(DspEQ *eq, float sample_rate) {
    eq->band_count  = 0;
    eq->sample_rate = sample_rate;
    eq->master_gain = 1.0f;
}

void dsp_eq_add_band(DspEQ *eq, DspFilterType type, float freq, float gain_db, float q) {
    if (eq->band_count >= DSP_EQ_MAX_BANDS) return;
    dsp_uint i = eq->band_count++;
    eq->type[i]    = type;
    eq->freq[i]    = freq;
    eq->gain_db[i] = gain_db;
    eq->q[i]       = q;
    dsp_biquad_design(&eq->bands[i], type, freq, gain_db, q, eq->sample_rate);
}

void dsp_eq_set_gain(DspEQ *eq, dsp_uint band, float gain_db) {
    if (band >= eq->band_count) return;
    eq->gain_db[band] = gain_db;
    dsp_biquad_design(&eq->bands[band], eq->type[band],
                      eq->freq[band], gain_db, eq->q[band], eq->sample_rate);
}

void dsp_eq_set_master(DspEQ *eq, float gain_db) {
    eq->master_gain = dsp_db_to_linear(gain_db);
}

float dsp_eq_tick(DspEQ *eq, float x) {
    for (dsp_uint i = 0; i < eq->band_count; i++)
        x = dsp_biquad_tick(&eq->bands[i], x);
    return x * eq->master_gain;
}

void dsp_eq_process(DspEQ *eq, const float *in, float *out, dsp_uint n) {
    for (dsp_uint s = 0; s < n; s++) {
        float x = in[s];
        for (dsp_uint i = 0; i < eq->band_count; i++)
            x = dsp_biquad_tick(&eq->bands[i], x);
        out[s] = x * eq->master_gain;
    }
}

void dsp_eq_reset(DspEQ *eq) {
    for (dsp_uint i = 0; i < eq->band_count; i++)
        dsp_biquad_reset(&eq->bands[i]);
}

void dsp_eq_init_15band(DspEQ *eq, float sample_rate) {
    dsp_eq_init(eq, sample_rate);
    static const float freqs[15] = {
        25.0f, 40.0f, 63.0f, 100.0f, 160.0f,
        250.0f, 400.0f, 630.0f, 1000.0f, 1600.0f,
        2500.0f, 4000.0f, 6300.0f, 10000.0f, 16000.0f
    };
    dsp_eq_add_band(eq, DSP_FILT_LOW_SHELF,  freqs[0], 0.0f, 2.15f);
    for (dsp_uint i = 1; i < 14; i++)
        dsp_eq_add_band(eq, DSP_FILT_PEAKING, freqs[i], 0.0f, 2.15f);
    dsp_eq_add_band(eq, DSP_FILT_HIGH_SHELF, freqs[14], 0.0f, 2.15f);
}

void dsp_eq_init_10band(DspEQ *eq, float sample_rate) {
    dsp_eq_init(eq, sample_rate);
    static const float freqs[10] = {
        31.5f, 63.0f, 125.0f, 250.0f, 500.0f,
        1000.0f, 2000.0f, 4000.0f, 8000.0f, 16000.0f
    };
    dsp_eq_add_band(eq, DSP_FILT_LOW_SHELF,  freqs[0], 0.0f, 1.41f);
    for (dsp_uint i = 1; i < 9; i++)
        dsp_eq_add_band(eq, DSP_FILT_PEAKING, freqs[i], 0.0f, 1.41f);
    dsp_eq_add_band(eq, DSP_FILT_HIGH_SHELF, freqs[9], 0.0f, 1.41f);
}

#endif /* LIBDSP_IMPLEMENTATION */
#endif /* LIBDSP_H */
