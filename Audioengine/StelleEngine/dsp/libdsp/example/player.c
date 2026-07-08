#ifndef USE_LIBDSP_LIB
#define LIBDSP_IMPLEMENTATION
#endif
#include "player.h"
#include <stdio.h>
#include <string.h>

int player_open(Player *p, const char *path) {
    memset(p, 0, sizeof(*p));

    if (!SDL_Init(SDL_INIT_AUDIO)) {
        fprintf(stderr, "SDL_Init: %s\n", SDL_GetError());
        return -1;
    }

    if (decoder_open(&p->decoder, path) != 0) {
        fprintf(stderr, "failed to open decoder\n");
        SDL_Quit();
        return -1;
    }

    SDL_AudioSpec spec = {0};
    spec.format = SDL_AUDIO_F32LE;
    spec.channels = decoder_channels(&p->decoder);
    spec.freq = 48000;

    p->stream = SDL_OpenAudioDeviceStream(
        SDL_AUDIO_DEVICE_DEFAULT_PLAYBACK, &spec, NULL, NULL);
    if (!p->stream) {
        fprintf(stderr, "SDL_OpenAudioDeviceStream: %s\n", SDL_GetError());
        decoder_close(&p->decoder);
        SDL_Quit();
        return -1;
    }

    p->spec = spec;
    p->playing = 0;
    p->speed = 1.0f;
    dsp_eq_init_15band(&p->eq, 48000.0f);
    return 0;
}

void player_play(Player *p) {
    if (p->playing) return;
    decoder_start(&p->decoder);
    SDL_ResumeAudioStreamDevice(p->stream);
    p->playing = 1;
}

void player_pause(Player *p) {
    if (!p->playing) return;
    decoder_pause(&p->decoder);
    SDL_PauseAudioStreamDevice(p->stream);
}

void player_resume(Player *p) {
    if (!p->playing) return;
    decoder_resume(&p->decoder);
    SDL_ResumeAudioStreamDevice(p->stream);
}

void player_stop(Player *p) {
    decoder_stop(&p->decoder);
    SDL_ClearAudioStream(p->stream);
    p->playing = 0;
}

void player_seek_seconds(Player *p, double sec) {
    ogg_int64_t pcm = (ogg_int64_t)(sec * 48000.0);
    decoder_seek(&p->decoder, pcm);
    SDL_ClearAudioStream(p->stream);
}

void player_tick(Player *p) {
    if (SDL_GetAudioStreamQueued(p->stream) > 192000)
        return;
    int samples = decoder_read(&p->decoder, p->eq_scratch, 8192);
    if (samples > 0) {
        dsp_eq_process(&p->eq, p->eq_scratch, p->eq_scratch, (dsp_uint)samples);
        int bytes = samples * (int)sizeof(float);
        SDL_PutAudioStreamData(p->stream, p->eq_scratch, bytes);
    }
}

void player_close(Player *p) {
    decoder_close(&p->decoder);
    if (p->stream) SDL_DestroyAudioStream(p->stream);
    SDL_Quit();
    memset(p, 0, sizeof(*p));
}

double player_position_seconds(Player *p) {
    ogg_int64_t pcm = decoder_tell(&p->decoder);
    return (double)pcm / 48000.0;
}

double player_length_seconds(Player *p) {
    return (double)decoder_total(&p->decoder) / 48000.0;
}

int player_is_playing(Player *p) {
    return p->playing && !decoder_eof(&p->decoder);
}

int player_is_paused(Player *p) {
    return p->playing && p->decoder.state == DECODER_STATE_PAUSED && !decoder_eof(&p->decoder);
}

int player_eof(Player *p) {
    return p->playing && decoder_eof(&p->decoder) && SDL_GetAudioStreamQueued(p->stream) == 0;
}

void player_set_speed(Player *p, float ratio) {
    if (ratio < 0.01f) ratio = 0.01f;
    if (ratio > 100.0f) ratio = 100.0f;
    p->speed = ratio;
    SDL_SetAudioStreamFrequencyRatio(p->stream, ratio);
}

float player_get_speed(Player *p) {
    return p->speed;
}

void player_eq_set_gain(Player *p, int band, float db) {
    if (band < 0 || band >= (int)p->eq.band_count) return;
    dsp_eq_set_gain(&p->eq, (dsp_uint)band, db);
}

float player_eq_get_gain(Player *p, int band) {
    if (band < 0 || band >= (int)p->eq.band_count) return 0;
    return p->eq.gain_db[band];
}

int player_eq_band_count(Player *p) {
    return (int)p->eq.band_count;
}

float player_eq_band_freq(Player *p, int band) {
    if (band < 0 || band >= (int)p->eq.band_count) return 0;
    return p->eq.freq[band];
}

void player_eq_reset(Player *p) {
    dsp_eq_reset(&p->eq);
}
