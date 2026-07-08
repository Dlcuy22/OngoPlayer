#include "decoder.h"
#include <string.h>
#include <stdlib.h>

static size_t ring_avail(RingBuffer *r) {
    if (r->write_pos >= r->read_pos)
        return r->write_pos - r->read_pos;
    return r->capacity - (r->read_pos - r->write_pos);
}

static size_t ring_free(RingBuffer *r) {
    size_t a = ring_avail(r);
    if (a >= r->capacity - 1) return 0;
    return r->capacity - 1 - a;
}

static size_t ring_write(RingBuffer *r, const float *src, size_t n) {
    size_t free = ring_free(r);
    if (n > free) n = free;
    if (n == 0) return 0;
    size_t first = r->capacity - r->write_pos;
    if (first > n) first = n;
    memcpy(r->data + r->write_pos, src, first * sizeof(float));
    r->write_pos += first;
    if (r->write_pos >= r->capacity) r->write_pos = 0;
    size_t second = n - first;
    if (second) {
        memcpy(r->data, src + first, second * sizeof(float));
        r->write_pos = second;
    }
    return n;
}

static size_t ring_read(RingBuffer *r, float *dst, size_t n) {
    size_t avail = ring_avail(r);
    if (n > avail) n = avail;
    if (n == 0) return 0;
    size_t first = r->capacity - r->read_pos;
    if (first > n) first = n;
    memcpy(dst, r->data + r->read_pos, first * sizeof(float));
    r->read_pos += first;
    if (r->read_pos >= r->capacity) r->read_pos = 0;
    size_t second = n - first;
    if (second) {
        memcpy(dst + first, r->data, second * sizeof(float));
        r->read_pos = second;
    }
    return n;
}

int decoder_open(Decoder *d, const char *path) {
    memset(d, 0, sizeof(*d));
    d->path = strdup(path);
    if (!d->path) return -1;

    int err;
    d->of = op_open_file(path, &err);
    if (!d->of) return err;

    const OpusHead *h = op_head(d->of, -1);
    d->channels = h->channel_count;
    d->total_pcm = op_pcm_total(d->of, -1);
    d->seek_pos = -1;
    d->state = DECODER_STATE_STOPPED;

    d->ring.capacity = RING_SAMPLES;
    d->ring.data = malloc(RING_SAMPLES * sizeof(float));
    d->ring.read_pos = 0;
    d->ring.write_pos = 0;

    d->mutex = SDL_CreateMutex();
    d->cond = SDL_CreateCondition();
    return 0;
}

static int decode_thread(void *arg) {
    Decoder *d = (Decoder *)arg;
    float buf[12000 * 2];
    int samples_per_call = 12000;

    while (1) {
        SDL_LockMutex(d->mutex);

        if (d->state == DECODER_STATE_STOPPED) {
            SDL_UnlockMutex(d->mutex);
            break;
        }

        while (d->state == DECODER_STATE_PAUSED && d->seek_pos < 0) {
            SDL_WaitCondition(d->cond, d->mutex);
            if (d->state == DECODER_STATE_STOPPED) {
                SDL_UnlockMutex(d->mutex);
                return 0;
            }
        }

        if (d->seek_pos >= 0) {
            op_pcm_seek(d->of, d->seek_pos);
            d->seek_pos = -1;
            d->eof = 0;
        }

        if (d->state == DECODER_STATE_STOPPED) {
            SDL_UnlockMutex(d->mutex);
            break;
        }

        if (d->eof) {
            SDL_UnlockMutex(d->mutex);
            SDL_Delay(10);
            continue;
        }

        int ret = op_read_float(d->of, buf, samples_per_call * d->channels, NULL);

        SDL_UnlockMutex(d->mutex);

        if (ret < 0) {
            if (ret == OP_HOLE) continue;
            d->eof = 1;
            continue;
        }
        if (ret == 0) {
            d->eof = 1;
            continue;
        }

        int samples = ret * d->channels;
        size_t written = 0;
        while (written < (size_t)samples) {
            SDL_LockMutex(d->mutex);
            if (d->state == DECODER_STATE_STOPPED) {
                SDL_UnlockMutex(d->mutex);
                return 0;
            }
            size_t n = ring_write(&d->ring, buf + written, (size_t)(samples - (int)written));
            SDL_UnlockMutex(d->mutex);

            written += n;
            if (n > 0) SDL_SignalCondition(d->cond);
            if (written < (size_t)samples) {
                SDL_Delay(1);
            }
        }
    }
    return 0;
}

void decoder_start(Decoder *d) {
    d->state = DECODER_STATE_PLAYING;
    d->thread = SDL_CreateThread(decode_thread, "decoder", d);
}

void decoder_pause(Decoder *d) {
    SDL_LockMutex(d->mutex);
    if (d->state == DECODER_STATE_PLAYING)
        d->state = DECODER_STATE_PAUSED;
    SDL_UnlockMutex(d->mutex);
}

void decoder_resume(Decoder *d) {
    SDL_LockMutex(d->mutex);
    if (d->state == DECODER_STATE_PAUSED) {
        d->state = DECODER_STATE_PLAYING;
        SDL_SignalCondition(d->cond);
    }
    SDL_UnlockMutex(d->mutex);
}

void decoder_stop(Decoder *d) {
    SDL_LockMutex(d->mutex);
    d->state = DECODER_STATE_STOPPED;
    SDL_SignalCondition(d->cond);
    SDL_UnlockMutex(d->mutex);
    if (d->thread) {
        SDL_WaitThread(d->thread, NULL);
        d->thread = NULL;
    }
}

void decoder_seek(Decoder *d, ogg_int64_t pcm) {
    SDL_LockMutex(d->mutex);
    if (pcm < 0) pcm = 0;
    if (pcm > d->total_pcm) pcm = d->total_pcm;
    d->seek_pos = pcm;
    d->ring.read_pos = 0;
    d->ring.write_pos = 0;
    SDL_SignalCondition(d->cond);
    SDL_UnlockMutex(d->mutex);
}

int decoder_read(Decoder *d, float *buf, int samples) {
    SDL_LockMutex(d->mutex);
    size_t n = ring_read(&d->ring, buf, (size_t)samples);
    SDL_UnlockMutex(d->mutex);
    if (n > 0) SDL_SignalCondition(d->cond);
    return (int)n;
}

void decoder_close(Decoder *d) {
    decoder_stop(d);
    if (d->of) op_free(d->of);
    free(d->ring.data);
    if (d->mutex) SDL_DestroyMutex(d->mutex);
    if (d->cond) SDL_DestroyCondition(d->cond);
    free(d->path);
    memset(d, 0, sizeof(*d));
}

ogg_int64_t decoder_tell(Decoder *d) {
    SDL_LockMutex(d->mutex);
    ogg_int64_t pos = op_pcm_tell(d->of);
    SDL_UnlockMutex(d->mutex);
    return pos;
}

ogg_int64_t decoder_total(Decoder *d) {
    return d->total_pcm;
}

int decoder_channels(Decoder *d) {
    return d->channels;
}

int decoder_eof(Decoder *d) {
    return d->eof;
}
