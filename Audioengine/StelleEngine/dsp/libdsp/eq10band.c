#include "dsp.h"
#include <stdio.h>

#define SAMPLE_RATE  48000.0f
#define BLOCK_SIZE   256

/* representasi satu preset EQ: 10 nilai gain dB untuk tiap band ISO */
typedef struct {
    const char *name;
    float gain[10];
/*   band:  31   63   125  250  500   1k   2k   4k   8k  16k  */
} EQPreset;

/* beberapa preset siap pakai */
static const EQPreset PRESETS[] = {
    { "flat",
      { 0.0f,  0.0f,  0.0f,  0.0f,  0.0f,  0.0f,  0.0f,  0.0f,  0.0f,  0.0f } },

    { "bass_boost",
      { 6.0f,  5.0f,  4.0f,  2.0f,  0.0f,  0.0f,  0.0f,  0.0f,  0.0f,  0.0f } },

    { "treble_boost",
      { 0.0f,  0.0f,  0.0f,  0.0f,  0.0f,  0.0f,  2.0f,  4.0f,  5.0f,  6.0f } },

    { "vocal_presence",
      {-2.0f, -1.0f,  0.0f,  1.0f,  2.0f,  3.0f,  3.0f,  2.0f,  1.0f,  0.0f } },

    { "v_shape",
      { 5.0f,  3.0f,  1.0f, -1.0f, -3.0f, -3.0f, -1.0f,  1.0f,  3.0f,  5.0f } },

    { "loudness",
      { 6.0f,  4.0f,  2.0f,  0.0f, -1.0f, -1.0f,  0.0f,  1.0f,  3.0f,  4.0f } },
};

#define PRESET_COUNT  (sizeof(PRESETS) / sizeof(PRESETS[0]))

/* apply preset ke EQ processor, recalculate semua koefisien */
static void eq_apply_preset(DspEQ *eq, const EQPreset *p) {
    for (dsp_uint i = 0; i < 10; i++)
        dsp_eq_set_gain(eq, i, p->gain[i]);
    printf("preset applied: %s\n", p->name);
}

/* print respons frekuensi: kirim impulse, FFT hasilnya */
static void print_freq_response(DspEQ *eq) {
    static float impulse_buf[BLOCK_SIZE];
    static float fft_buf[BLOCK_SIZE * 2];
    static float mag[BLOCK_SIZE / 2 + 1];

    /* reset state agar impulse bersih */
    dsp_eq_reset(eq);

    /* buat impulse: satu sample = 1.0, sisanya 0 */
    impulse_buf[0] = 1.0f;
    for (dsp_uint i = 1; i < BLOCK_SIZE; i++) impulse_buf[i] = 0.0f;

    /* proses impulse melalui EQ */
    static float ir[BLOCK_SIZE];
    dsp_eq_process(eq, impulse_buf, ir, BLOCK_SIZE);

    /* pack ke complex buffer untuk FFT */
    for (dsp_uint i = 0; i < BLOCK_SIZE; i++) {
        fft_buf[2*i]   = ir[i];
        fft_buf[2*i+1] = 0.0f;
    }
    dsp_fft(fft_buf, BLOCK_SIZE);

    /* magnitude tiap bin */
    for (dsp_uint i = 0; i <= BLOCK_SIZE/2; i++) {
        float re = fft_buf[2*i];
        float im = fft_buf[2*i+1];
        mag[i] = dsp_sqrtf(re*re + im*im);
    }

    /* tampilkan di frekuensi ISO band saja */
    static const float iso[10] = {
        31.5f, 63.0f, 125.0f, 250.0f, 500.0f,
        1000.0f, 2000.0f, 4000.0f, 8000.0f, 16000.0f
    };

    printf("  freq(Hz)  gain(dB)\n");
    for (int b = 0; b < 10; b++) {
        /* cari bin paling dekat ke frekuensi ISO */
        dsp_uint bin = (dsp_uint)(iso[b] * BLOCK_SIZE / SAMPLE_RATE + 0.5f);
        if (bin > BLOCK_SIZE/2) bin = BLOCK_SIZE/2;
        float db = dsp_linear_to_db(mag[bin]);
        printf("  %7.1f   %+6.2f dB\n", iso[b], db);
    }
}

/* proses blok audio dengan EQ, kembalikan RMS level */
static float process_block(DspEQ *eq, float *buf, dsp_uint n) {
    static float out[BLOCK_SIZE];
    dsp_eq_process(eq, buf, out, n);

    /* hitung RMS output */
    float sum = 0.0f;
    for (dsp_uint i = 0; i < n; i++) sum += out[i] * out[i];
    return dsp_sqrtf(sum / (float)n);
}

int main(void) {
    DspEQ eq;

    /* init 10-band ISO standar */
    dsp_eq_init_10band(&eq, SAMPLE_RATE);

    printf("=== libdsp EQ 10-band @ %.0f Hz ===\n\n", SAMPLE_RATE);
    printf("bands: 31 | 63 | 125 | 250 | 500 | 1k | 2k | 4k | 8k | 16k Hz\n\n");

    /* --- contoh 1: manual set per band --- */
    printf("[ manual set: bass boost + vocal cut ]\n");
    dsp_eq_set_gain(&eq, 0,  5.0f);   /* 31  Hz */
    dsp_eq_set_gain(&eq, 1,  4.0f);   /* 63  Hz */
    dsp_eq_set_gain(&eq, 2,  3.0f);   /* 125 Hz */
    dsp_eq_set_gain(&eq, 3,  1.0f);   /* 250 Hz */
    dsp_eq_set_gain(&eq, 4, -1.0f);   /* 500 Hz */
    dsp_eq_set_gain(&eq, 5, -2.0f);   /* 1   kHz */
    dsp_eq_set_gain(&eq, 6,  0.0f);   /* 2   kHz */
    dsp_eq_set_gain(&eq, 7,  1.0f);   /* 4   kHz */
    dsp_eq_set_gain(&eq, 8,  2.0f);   /* 8   kHz */
    dsp_eq_set_gain(&eq, 9,  1.0f);   /* 16  kHz */
    print_freq_response(&eq);

    /* --- contoh 2: load dari preset --- */
    printf("\n[ preset loop ]\n");
    for (dsp_uint p = 0; p < PRESET_COUNT; p++) {
        eq_apply_preset(&eq, &PRESETS[p]);
        print_freq_response(&eq);
        printf("\n");
    }

    /* --- contoh 3: proses audio buffer nyata --- */
    printf("[ process audio block, 256 samples @ 1kHz sine ]\n");

    static float audio_in[BLOCK_SIZE];
    for (dsp_uint i = 0; i < BLOCK_SIZE; i++)
        audio_in[i] = dsp_sinf(DSP_2PI * 1000.0f * (float)i / SAMPLE_RATE);

    /* flat: RMS harus ~0.707 (sine amplitude 1.0) */
    eq_apply_preset(&eq, &PRESETS[0]);
    dsp_eq_reset(&eq);
    float rms_flat = process_block(&eq, audio_in, BLOCK_SIZE);
    printf("  flat RMS    : %.4f (expect ~0.707)\n", rms_flat);

    /* vocal presence boost di 1kHz: RMS harus naik */
    eq_apply_preset(&eq, &PRESETS[3]);
    dsp_eq_reset(&eq);
    float rms_vocal = process_block(&eq, audio_in, BLOCK_SIZE);
    printf("  vocal RMS   : %.4f (naik karena +3dB di 1kHz)\n", rms_vocal);

    /* --- contoh 4: master gain --- */
    printf("\n[ master gain -6dB ]\n");
    eq_apply_preset(&eq, &PRESETS[0]);
    dsp_eq_set_master(&eq, -6.0f);
    dsp_eq_reset(&eq);
    float rms_attenuated = process_block(&eq, audio_in, BLOCK_SIZE);
    printf("  flat -6dB RMS: %.4f (expect ~0.354)\n", rms_attenuated);

    /* reset master ke unity */
    dsp_eq_set_master(&eq, 0.0f);

    printf("\ndone.\n");
    return 0;
}
