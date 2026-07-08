#ifndef DECODER_H
#define DECODER_H

#include <stddef.h>
#include <ogg/ogg.h>
#include <opus/opusfile.h>
#include <SDL3/SDL_mutex.h>
#include <SDL3/SDL_thread.h>
#include <SDL3/SDL_timer.h>
#include <SDL3/SDL_init.h>

#define DECODER_STATE_STOPPED 0
#define DECODER_STATE_PLAYING 1
#define DECODER_STATE_PAUSED  2

#define RING_SAMPLES (48000 * 2 * 4)

typedef struct {
    float *data;
    size_t capacity;
    size_t read_pos;
    size_t write_pos;
} RingBuffer;

typedef struct {
    char *path;
    OggOpusFile *of;
    RingBuffer ring;
    SDL_Thread *thread;
    SDL_Mutex *mutex;
    SDL_Condition *cond;

    volatile int state;
    volatile ogg_int64_t seek_pos;

    int channels;
    ogg_int64_t total_pcm;
    int eof;
} Decoder;

int  decoder_open(Decoder *d, const char *path);
void decoder_start(Decoder *d);
void decoder_pause(Decoder *d);
void decoder_resume(Decoder *d);
void decoder_stop(Decoder *d);
void decoder_seek(Decoder *d, ogg_int64_t pcm);
int  decoder_read(Decoder *d, float *buf, int samples);
void decoder_close(Decoder *d);
ogg_int64_t decoder_tell(Decoder *d);
ogg_int64_t decoder_total(Decoder *d);
int  decoder_channels(Decoder *d);
int  decoder_eof(Decoder *d);

#endif
