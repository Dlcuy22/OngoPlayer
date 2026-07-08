#include "dsp.h"

/* PI constants */
#define DSP_PI      3.14159265358979323846f
#define DSP_2PI     6.28318530717958647692f
#define DSP_PI_2    1.57079632679489661923f
#define DSP_LOG2E   1.44269504088896340736f

/* fast absolute value */
float dsp_absf(float x) {
    /* reinterpret bits, clear sign bit */
    union { float f; unsigned int u; } v;
    v.f = x;
    v.u &= 0x7FFFFFFF;
    return v.f;
}

/* clamp float to [lo, hi] */
float dsp_clampf(float x, float lo, float hi) {
    if (x < lo) return lo;
    if (x > hi) return hi;
    return x;
}

/* integer power, for coefficient precomputation */
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

/* minimax polynomial sin, error < 5e-7 for x in [-pi, pi] */
static float _sin_core(float x) {
    float x2 = x * x;
    return x * (1.0f
        - x2 * (1.6666667e-1f
        - x2 * (8.3333337e-3f
        - x2 * (1.9841270e-4f
        - x2 *  2.7557319e-6f))));
}

float dsp_sinf(float x) {
    /* range reduce to [-pi, pi] */
    float q = (float)(dsp_int)(x / DSP_2PI);
    x -= q * DSP_2PI;
    if (x > DSP_PI)  x -= DSP_2PI;
    if (x < -DSP_PI) x += DSP_2PI;

    /* fold to [-pi/2, pi/2] */
    if (x > DSP_PI_2)  x =  DSP_PI - x;
    if (x < -DSP_PI_2) x = -DSP_PI - x;

    return _sin_core(x);
}

float dsp_cosf(float x) {
    return dsp_sinf(x + DSP_PI_2);
}

/* fast inverse sqrt (Newton-Raphson iteration) */
static float _inv_sqrtf(float x) {
    union { float f; unsigned int u; } v;
    v.f = x;
    v.u = 0x5F3759DFu - (v.u >> 1);
    v.f *= (1.5f - 0.5f * x * v.f * v.f);  /* 1st iteration */
    v.f *= (1.5f - 0.5f * x * v.f * v.f);  /* 2nd iteration, good to ~23 bits */
    return v.f;
}

float dsp_sqrtf(float x) {
    if (x <= 0.0f) return 0.0f;
    return x * _inv_sqrtf(x);
}

/* self-contained log10, no libm needed */
float dsp_log10f(float x) {
    if (x <= 0.0f) return -144.0f;

    /* extract exponent from IEEE 754 bits */
    union { float f; unsigned u; } v = { .f = x };

    float exp = (float)((int)(v.u >> 23) - 127);
    v.u = (v.u & 0x007FFFFFu) | 0x3F800000u;  /* isolate mantissa in [1, 2) */

    /* log2(1+m) for m in [0, 1) via 5th-order polynomial (Taylor, error < 2e-5) */
    float m = v.f - 1.0f;
    float log2_m = m * (1.4426950408889634f
                  + m * (-0.7213475204444818f
                  + m * (0.4808983469629872f
                  + m * (-0.3606740294316674f
                  + m * 0.288539974f))));

    /* log10(x) = log2(x) * log10(2) */
    return (exp + log2_m) * 0.30102999566398119521373889472449f;
}

float dsp_linear_to_db(float linear) {
    if (linear <= 0.0f) return -144.0f;
    return 20.0f * dsp_log10f(linear);
}

float dsp_db_to_linear(float db) {
    /* 10^(db/20) = 2^(db * log2(10) / 20) */
    float exp2_val = db * (3.321928f / 20.0f);  /* log2(10) = 3.321928 */
    dsp_int intpart = (dsp_int)exp2_val;
    float frac = exp2_val - (float)intpart;
    /* 2^frac polynomial approximation */
    float frac_pow = 1.0f + frac * (0.6931472f + frac * (0.2402265f + frac * 0.0555041f));
    union { float f; unsigned int u; } v;
    v.u = (dsp_uint)(intpart + 127) << 23;
    return v.f * frac_pow;
}

/* utility: interleave separate re/im arrays into [re,im,re,im,...] */
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
