#include "dsp.h"

#define DSP_PI      3.14159265358979323846f
#define DSP_2PI     6.28318530717958647692f

/* ---- windowing ---- */

void dsp_apply_window(float *buf, dsp_uint n, DspWindow w) {
    float N1 = (float)(n - 1);
    for (dsp_uint i = 0; i < n; i++) {
        float t = (float)i / N1;
        float coef;
        switch (w) {
            case DSP_WIN_RECT:
                coef = 1.0f;
                break;
            case DSP_WIN_HANN:
                coef = 0.5f - 0.5f * dsp_cosf(DSP_2PI * t);
                break;
            case DSP_WIN_HAMMING:
                coef = 0.54f - 0.46f * dsp_cosf(DSP_2PI * t);
                break;
            case DSP_WIN_BLACKMAN:
                coef = 0.42f
                     - 0.5f  * dsp_cosf(DSP_2PI * t)
                     + 0.08f * dsp_cosf(2.0f * DSP_2PI * t);
                break;
            case DSP_WIN_FLAT_TOP:
                coef = 0.21557895f
                     - 0.41663158f * dsp_cosf(DSP_2PI * t)
                     + 0.27726316f * dsp_cosf(2.0f * DSP_2PI * t)
                     - 0.08357895f * dsp_cosf(3.0f * DSP_2PI * t)
                     + 0.00694737f * dsp_cosf(4.0f * DSP_2PI * t);
                break;
            default:
                coef = 1.0f;
        }
        buf[i] *= coef;
    }
}

/* ---- biquad design ---- */

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

/* direct form II transposed: 2 multiply-accumulate per sample */
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

/* windowed sinc: h[n] = sinc(2*fc*n) * window[n] */
static float _sinc(float x) {
    if (x == 0.0f) return 1.0f;
    float px = DSP_PI * x;
    return dsp_sinf(px) / px;
}

void dsp_fir_design_lowpass(DspFIR *fir, float cutoff, float fs,
                             dsp_uint taps, DspWindow w) {
    if (taps > DSP_FIR_MAX_TAPS) taps = DSP_FIR_MAX_TAPS;
    if (!(taps & 1)) taps--;   /* force odd for symmetric type I */

    fir->taps      = taps;
    fir->write_idx = 0;

    float fc   = cutoff / fs;
    dsp_int M  = (dsp_int)(taps - 1);

    for (dsp_uint i = 0; i < taps; i++) {
        float n = (float)i - (float)M * 0.5f;
        fir->coeffs[i] = 2.0f * fc * _sinc(2.0f * fc * n);
        fir->delay[i]  = 0.0f;
    }
    /* apply window in-place on coefficients */
    dsp_apply_window(fir->coeffs, taps, w);
}

void dsp_fir_design_highpass(DspFIR *fir, float cutoff, float fs,
                              dsp_uint taps, DspWindow w) {
    /* design lowpass then spectral invert */
    dsp_fir_design_lowpass(fir, cutoff, fs, taps, w);
    dsp_int M = (dsp_int)(taps - 1);
    for (dsp_uint i = 0; i < taps; i++) {
        fir->coeffs[i] = -fir->coeffs[i];
    }
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
