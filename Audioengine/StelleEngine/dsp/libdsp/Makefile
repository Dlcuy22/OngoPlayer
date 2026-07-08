CC      = gcc
AR      = ar

# libdsp library flags
LIB_CFLAGS = -O2 -ffast-math -fno-builtin -fomit-frame-pointer \
             -Wall -Wextra -std=c99

# example flags (SDL3, opusfile, ncursesw)
EXA_CFLAGS  = -O2 -g -Wall -Wextra -std=c99
EXA_CFLAGS += $(shell pkg-config --cflags sdl3 opusfile ncursesw 2>/dev/null)
EXA_LDLIBS  = $(shell pkg-config --libs sdl3 opusfile ncursesw 2>/dev/null)

# sources
LIB_SRC = dsp_math.c dsp_fft.c dsp_filter.c dsp_eq.c
LIB_OBJ = $(LIB_SRC:.c=.o)
LIB     = libdsp.a

EXA_SRC = example/main.c example/player.c example/decoder.c
EXA_OBJ = $(EXA_SRC:.c=.o)
EXA     = example/player

# ---- targets ----

all: $(LIB) $(EXA)

# static library from separate source files
$(LIB): $(LIB_OBJ)
	$(AR) rcs $@ $^

dsp_math.o:   dsp_math.c dsp.h
dsp_fft.o:   dsp_fft.c dsp.h
dsp_filter.o: dsp_filter.c dsp.h
dsp_eq.o:    dsp_eq.c dsp.h

$(LIB_OBJ):
	$(CC) $(LIB_CFLAGS) -c $< -o $@

# example using single-header (default)
example/player.o: CFLAGS = $(EXA_CFLAGS)
example/main.o:   CFLAGS = $(EXA_CFLAGS)
example/decoder.o: CFLAGS = $(EXA_CFLAGS)

$(EXA): $(EXA_OBJ)
	$(CC) $^ $(EXA_LDLIBS) -o $@

$(EXA_OBJ): example/decoder.h example/player.h

# example linking against libdsp.a instead of single-header
player-lib: example/main.o example/player-lib.o example/decoder.o $(LIB)
	$(CC) $^ $(EXA_LDLIBS) -L. -ldsp -o example/player-lib

example/player-lib.o: example/player.c example/libdsp.h
	$(CC) $(EXA_CFLAGS) -DUSE_LIBDSP_LIB -c $< -o $@

# legacy demos
dsp_example: example.c $(LIB)
	$(CC) $(LIB_CFLAGS) example.c -L. -ldsp -o dsp_example -lm

eq10band: eq10band.c $(LIB)
	$(CC) $(LIB_CFLAGS) eq10band.c -L. -ldsp -o eq10band -lm

clean:
	rm -f $(LIB_OBJ) $(LIB)
	rm -f example/*.o example/player example/player-lib
	rm -f dsp_example eq10band

distclean: clean
	rm -rf docs

.PHONY: all clean distclean player-lib dsp_example eq10band
