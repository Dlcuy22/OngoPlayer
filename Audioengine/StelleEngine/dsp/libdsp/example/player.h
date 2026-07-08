#ifndef PLAYER_H
#define PLAYER_H

#include <stddef.h>
#include <SDL3/SDL_audio.h>
#include <SDL3/SDL_init.h>
#include "decoder.h"
#include "libdsp.h"

typedef struct {
    Decoder decoder;
    SDL_AudioStream *stream;
    SDL_AudioSpec spec;
    int playing;
    float speed;
    DspEQ eq;
    float eq_scratch[8192];
} Player;

int  player_open(Player *p, const char *path);
void player_play(Player *p);
void player_pause(Player *p);
void player_resume(Player *p);
void player_stop(Player *p);
void player_seek_seconds(Player *p, double sec);
void player_tick(Player *p);
void player_close(Player *p);
double player_position_seconds(Player *p);
double player_length_seconds(Player *p);
int player_is_playing(Player *p);
int player_is_paused(Player *p);
int player_eof(Player *p);
void player_set_speed(Player *p, float ratio);
float player_get_speed(Player *p);
void player_eq_set_gain(Player *p, int band, float db);
float player_eq_get_gain(Player *p, int band);
int player_eq_band_count(Player *p);
float player_eq_band_freq(Player *p, int band);
void player_eq_reset(Player *p);

#endif
