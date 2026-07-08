#include "dsp.h"
#include <stdio.h>

#define BUF_SIZE  1024
#define FS        48000.0f

int main(void) {
    /* --- math primitives --- */
    printf("=== math ===\n");
    printf("sin(pi/2) = %.6f  (expect 1.0)\n", dsp_sinf(3.14159f / 2.0f));
    printf("cos(0)    = %.6f  (expect 1.0)\n", dsp_cosf(0.0f));
    printf("sqrt(2)   = %.6f  (expect 1.41421)\n", dsp_sqrtf(2.0f));
    printf("db2lin(6) = %.6f  (expect ~2.0)\n", dsp_db_to_linear(6.0f));
    printf("lin2db(2) = %.6f  (expect ~6.0)\n", dsp_linear_to_db(2.0f));

    /* --- biquad lowpass --- */
    printf("\n=== biquad lowpass @ 1kHz ===\n");
    DspBiquad lp;
    dsp_biquad_design(&lp, DSP_FILT_LOW_PASS, 1000.0f, 0.0f, 0.707f, FS);

    float impulse_out[32] = {0};
    float x = 1.0f;
    for (int i = 0; i < 32; i++) {
        impulse_out[i] = dsp_biquad_tick(&lp, x);
        x = 0.0f;
    }
    printf("impulse response first 8: ");
    for (int i = 0; i < 8; i++) printf("%.4f ", impulse_out[i]);
    printf("\n");

    /* --- FFT magnitude --- */
    printf("\n=== FFT 1024-point ===\n");
    static float signal[BUF_SIZE];
    static float mag[BUF_SIZE / 2 + 1];

    /* generate 1kHz sine at 48kHz */
    for (int i = 0; i < BUF_SIZE; i++)
        signal[i] = dsp_sinf(DSP_2PI * 1000.0f * (float)i / FS);

    dsp_apply_window(signal, BUF_SIZE, DSP_WIN_HANN);
    dsp_rfft_magnitude(signal, mag, BUF_SIZE);

    /* find peak bin */
    dsp_uint peak_bin = 0;
    float peak_val = 0.0f;
    for (dsp_uint i = 0; i < BUF_SIZE/2; i++) {
        if (mag[i] > peak_val) { peak_val = mag[i]; peak_bin = i; }
    }
    float peak_hz = (float)peak_bin * FS / (float)BUF_SIZE;
    printf("peak bin %u = %.1f Hz (expect ~1000 Hz), mag = %.2f\n",
           peak_bin, peak_hz, peak_val);

    /* --- 10-band EQ --- */
    printf("\n=== 10-band EQ ===\n");
    DspEQ eq;
    dsp_eq_init_10band(&eq, FS);

    /* bass boost preset */
    dsp_eq_set_gain(&eq, 0, 6.0f);   /* 31 Hz  +6dB */
    dsp_eq_set_gain(&eq, 1, 5.0f);   /* 63 Hz  +5dB */
    dsp_eq_set_gain(&eq, 2, 3.0f);   /* 125 Hz +3dB */

    float in_buf[64], out_buf[64];
    for (int i = 0; i < 64; i++)
        in_buf[i] = dsp_sinf(DSP_2PI * 63.0f * (float)i / FS);

    dsp_eq_process(&eq, in_buf, out_buf, 64);
    printf("EQ 63Hz in[0]=%.4f out[0]=%.4f (bass boosted)\n",
           in_buf[0], out_buf[0]);

    /* --- FIR lowpass --- */
    printf("\n=== FIR lowpass 63-tap @ 4kHz ===\n");
    DspFIR fir;
    dsp_fir_design_lowpass(&fir, 4000.0f, FS, 63, DSP_WIN_BLACKMAN);

    float fir_out[64];
    /* 1kHz should pass through, 10kHz should be attenuated */
    for (int i = 0; i < 64; i++) {
        float s = dsp_sinf(DSP_2PI * 1000.0f * (float)i / FS);
        fir_out[i] = dsp_fir_tick(&fir, s);
    }
    printf("FIR 1kHz (should pass): last sample = %.4f\n", fir_out[63]);

    dsp_fir_reset(&fir);
    for (int i = 0; i < 64; i++) {
        float s = dsp_sinf(DSP_2PI * 10000.0f * (float)i / FS);
        fir_out[i] = dsp_fir_tick(&fir, s);
    }
    printf("FIR 10kHz (should attenuate): last sample = %.4f\n", fir_out[63]);

    printf("\ndone.\n");
    return 0;
}


