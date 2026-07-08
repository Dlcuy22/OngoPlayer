#include <stdio.h>
#include <string.h>
#include <time.h>
#include <locale.h>
#include <ncurses.h>
#include "player.h"

#define SPEED_UP 1.5f
#define EQ_BANDS 15
#define SLIDER_W 50
#define EQ_MIN -12.0f
#define EQ_MAX 12.0f

static struct timespec last_s;
static int eq_mode = 0;
static int eq_band = 0;

static void draw_slider(int y, int x, int w, float db, int selected) {
    int center = w / 2;
    int pos = center + (int)(db / 12.0f * center + 0.5f);
    if (pos < 0) pos = 0;
    if (pos >= w) pos = w - 1;

    if (selected) attron(A_REVERSE);
    mvaddch(y, x, '[');
    if (selected) attroff(A_REVERSE);

    for (int i = 0; i < w; i++) {
        if (i == pos) {
            if (selected) attron(A_REVERSE);
            addch('|');
            if (selected) attroff(A_REVERSE);
        } else if (db > 0 && i > center && i < pos) {
            addch('#');
        } else if (db < 0 && i > pos && i < center) {
            addch('#');
        } else {
            addch(' ');
        }
    }

    if (selected) attron(A_REVERSE);
    mvaddch(y, x + w + 1, ']');
    if (selected) attroff(A_REVERSE);
}

static void draw_eq_ui(Player *p) {
    int cols = getmaxx(stdscr);

    mvprintw(0, 0, " EQ [e: close]  UP/DOWN band  J/K gain");
    mvprintw(0, cols - 20, " Band %d/%d  %s%+.1fdB",    
        eq_band + 1, EQ_BANDS,
        eq_band == 0 ? "Shelf " : eq_band == EQ_BANDS - 1 ? "Shelf " : "Peak  ",
        player_eq_get_gain(p, eq_band));

    int freq_hz = (int)player_eq_band_freq(p, eq_band);
    if (freq_hz >= 1000)
        mvprintw(0, cols - 20, " Band %d/%d  %4.1fkHz  %+.1fdB",
            eq_band + 1, EQ_BANDS, freq_hz / 1000.0f,
            player_eq_get_gain(p, eq_band));
    else
        mvprintw(0, cols - 20, " Band %d/%d  %4.0fHz  %+.1fdB",
            eq_band + 1, EQ_BANDS, player_eq_band_freq(p, eq_band),
            player_eq_get_gain(p, eq_band));

    mvprintw(1, 0, " %3.0f dB", EQ_MAX);
    mvprintw(1, cols - 8, "%+.0f dB", EQ_MIN);

    for (int b = 0; b < EQ_BANDS; b++) {
        int y = b + 3;
        float freq = player_eq_band_freq(p, b);
        if (freq >= 1000)
            mvprintw(y, 0, " %4.0fkHz", freq / 1000.0f);
        else
            mvprintw(y, 0, " %4.0fHz ", freq);

        float db = player_eq_get_gain(p, b);
        int sel = (b == eq_band);
        draw_slider(y, 8, SLIDER_W, db, sel);
        mvprintw(y, 8 + SLIDER_W + 3, " %+.1fdB", db);
    }
}

static void draw_player_ui(Player *p, const char *path) {
    int cols = getmaxx(stdscr);

    double pos = player_position_seconds(p);
    double len = player_length_seconds(p);

    int pos_m = (int)pos / 60;
    int pos_s = (int)pos % 60;
    int len_m = (int)len / 60;
    int len_s = (int)len % 60;

    const char *status = "STOPPED";
    if (player_is_playing(p)) status = "PLAYING";
    else if (player_is_paused(p)) status = "PAUSED";

    const char *fname = strrchr(path, '/');
    fname = fname ? fname + 1 : path;

    int y = 0;
    mvprintw(y, 0, " %s", status);
    mvprintw(y, cols - 8, "e:EQ");

    y += 2;
    mvprintw(y, 0, " File: %s", fname);
    float spd = player_get_speed(p);
    if (spd != 1.0f)
        mvprintw(y, cols - 10, "x%.1f ", spd);

    y += 1;
    if (len > 0) {
        double pct = pos / len;
        if (pct > 1.0) pct = 1.0;
        int bar_w = cols - 12;
        if (bar_w < 10) bar_w = 10;
        int fill = (int)(pct * bar_w);

        mvprintw(y, 0, " %02d:%02d / %02d:%02d ", pos_m, pos_s, len_m, len_s);
        mvprintw(y, 12, "[");
        for (int i = 0; i < bar_w; i++)
            mvprintw(y, 13 + i, "%c", i < fill ? '#' : '-');
        mvprintw(y, 13 + bar_w, "]");
    } else {
        mvprintw(y, 0, " %02d:%02d / --:--", pos_m, pos_s);
    }

    y += 3;
    mvprintw(y, 0, " Controls:");
    mvprintw(y + 1, 0, "  SPACE    - play/pause");
    mvprintw(y + 2, 0, "  <- / ->  - seek -/+ 5s");
    mvprintw(y + 3, 0, "  s s      - toggle speed (x%.1f)", SPEED_UP);
    mvprintw(y + 4, 0, "  e        - equalizer");
    mvprintw(y + 5, 0, "  q        - quit");
}

static void draw_status_bar(Player *p) {
    int cols = getmaxx(stdscr);
    double pos = player_position_seconds(p);
    double len = player_length_seconds(p);
    int pos_m = (int)pos / 60, pos_s = (int)pos % 60;
    int len_m = (int)len / 60, len_s = (int)len % 60;

    const char *status = "STOPPED";
    if (player_is_playing(p)) status = "PLAYING";
    else if (player_is_paused(p)) status = "PAUSED";
    float spd = player_get_speed(p);

    int bar_w = cols - 34;
    if (bar_w < 10) bar_w = 10;
    double pct = len > 0 ? pos / len : 0.0;
    if (pct > 1.0) pct = 1.0;
    int fill = (int)(pct * bar_w);

    int y = getmaxy(stdscr) - 3;
    mvprintw(y, 0, " %02d:%02d / %02d:%02d ", pos_m, pos_s, len_m, len_s);
    mvprintw(y, 13, "[");
    for (int i = 0; i < bar_w; i++)
        mvprintw(y, 14 + i, "%c", i < fill ? '#' : '-');
    mvprintw(y, 14 + bar_w, "] %s", status);
    if (spd != 1.0f)
        printw(" x%.1f", spd);
}

static void draw_controls(int eq) {
    int y = getmaxy(stdscr) - 1;
    move(y, 0);
    if (eq) {
        printw("  UP/DOWN band  <-/-> J/K gain  e close  SPACE pause  q quit");
    } else {
        printw("  SPACE pause  <-/-> seek  s s speed  e EQ  q quit");
    }
    clrtoeol();
}

static void draw_ui(Player *p, const char *path) {
    werase(stdscr);
    if (eq_mode) {
        draw_eq_ui(p);
    } else {
        draw_player_ui(p, path);
    }
    draw_status_bar(p);
    draw_controls(eq_mode);
    wnoutrefresh(stdscr);
}

static void handle_player_key(Player *p, int ch) {
    switch (ch) {
    case ' ':
        if (player_is_paused(p)) player_resume(p);
        else if (player_is_playing(p)) player_pause(p);
        else player_play(p);
        break;
    case KEY_LEFT: {
        double pos = player_position_seconds(p);
        player_seek_seconds(p, pos - 5.0);
        break;
    }
    case KEY_RIGHT: {
        double pos = player_position_seconds(p);
        player_seek_seconds(p, pos + 5.0);
        break;
    }
    case 's': {
        struct timespec now;
        clock_gettime(CLOCK_MONOTONIC, &now);
        double dt = (now.tv_sec - last_s.tv_sec)
                  + (now.tv_nsec - last_s.tv_nsec) / 1e9;
        last_s = now;
        if (dt < 2.0) {
            float cur = player_get_speed(p);
            player_set_speed(p, cur == 1.0f ? SPEED_UP : 1.0f);
        }
        break;
    }
    case 'e':
        eq_mode = 1;
        break;
    }
}

static void handle_eq_key(Player *p, int ch) {
    switch (ch) {
    case 'e':
        eq_mode = 0;
        break;
    case ' ':
        if (player_is_paused(p)) player_resume(p);
        else if (player_is_playing(p)) player_pause(p);
        else player_play(p);
        break;
    case KEY_UP:
    case KEY_SR:
        eq_band--;
        if (eq_band < 0) eq_band = EQ_BANDS - 1;
        break;
    case KEY_DOWN:
    case KEY_SF:
        eq_band++;
        if (eq_band >= EQ_BANDS) eq_band = 0;
        break;
    case KEY_LEFT:
    case 'j':
    case 'J': {
        float g = player_eq_get_gain(p, eq_band);
        player_eq_set_gain(p, eq_band, g - 0.5f);
        break;
    }
    case KEY_RIGHT:
    case 'k':
    case 'K': {
        float g = player_eq_get_gain(p, eq_band);
        player_eq_set_gain(p, eq_band, g + 0.5f);
        break;
    }
    }
}

int main(int argc, char **argv) {
    if (argc < 2) {
        fprintf(stderr, "usage: %s <file.opus>\n", argv[0]);
        return 1;
    }

    Player player;
    if (player_open(&player, argv[1]) != 0)
        return 1;

    setlocale(LC_ALL, "");
    initscr();
    cbreak();
    noecho();
    keypad(stdscr, 1);
    timeout(50);
    curs_set(0);

    int running = 1;
    player_play(&player);

    while (running) {
        int ch = getch();
        if (ch == 'q') {
            running = 0;
        } else if (ch != ERR) {
            if (eq_mode)
                handle_eq_key(&player, ch);
            else
                handle_player_key(&player, ch);
        }

        player_tick(&player);

        if (player_eof(&player)) {
            draw_ui(&player, argv[1]);
            doupdate();
            napms(500);
            break;
        }

        draw_ui(&player, argv[1]);
        doupdate();
    }

    endwin();
    player_close(&player);
    return 0;
}
