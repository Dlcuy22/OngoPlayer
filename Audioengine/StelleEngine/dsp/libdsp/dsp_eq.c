#include "dsp.h"

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

/* 15-band ISO 2/3-octave, low/high shelf on ends */
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

/* 10-band ISO 1/1-octave, low/high shelf on ends */
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
