#include "dsp.h"

#define DSP_PI  3.14159265358979323846f
#define DSP_2PI 6.28318530717958647692f

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

/* Cooley-Tukey radix-2 DIT, buf = interleaved [re,im,...], length = 2*n */
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

/* real FFT magnitude: zero-pad imaginary, run full FFT, output n/2+1 bins */
void dsp_rfft_magnitude(const float *in, float *mag_out, dsp_uint n) {
    static float scratch[DSP_FFT_MAX_SIZE * 2];

    /* pack real input as complex with zero imaginary */
    for (dsp_uint i = 0; i < n; i++) {
        scratch[2*i]   = in[i];
        scratch[2*i+1] = 0.0f;
    }

    _fft_core(scratch, n, 0);

    /* output unique bins 0 .. n/2 */
    for (dsp_uint k = 0; k <= n/2; k++) {
        float re = scratch[2*k];
        float im = scratch[2*k+1];
        mag_out[k] = dsp_sqrtf(re*re + im*im);
    }
}

/* power spectrum: mag^2, no sqrt */
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
