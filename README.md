# OngoPlayer [![](https://github.com/Dlcuy22/OngoPlayer/actions/workflows/build.yml/badge.svg)](https://github.com/Dlcuy22/OngoPlayer/actions/workflows/build.yml)

![OngoPlayer Demo](.github/assets/ongoplayer-demo-newui.webp)

A dead simple music player that just works.

---

## How it works

OngoPlayer uses [purego](https://github.com/ebitengine/purego) to call native audio libraries directly at runtime, with no CGo involved. The GUI is built with [Gio](https://gioui.org), an immediate-mode GUI toolkit written in pure Go (not to be confused with the purego library mentioned above). Together they keep the binary small, the build simple, and the runtime lean.

### Why C libraries for audio?

Every mature audio codec implementation (MP3, Opus, Vorbis, FLAC) is written in C. Reimplementing them in Go would mean slower decoding, missing edge-case handling, and a maintenance burden that isn't worth taking on. purego lets OngoPlayer use the real implementations without a CGo compile step, so the codec work happens in the system libraries and the Go code stays clean.

### No browser engine

Gio renders using OpenGL ES and Vulkan directly, with no Electron, no WebView, and no browser engine underneath. The result is a native window with GPU-accelerated drawing and a memory footprint that reflects what the app actually does, not what Chromium needs.

---

## Features

- **Synced lyrics**: loads `.lrc` files from disk; falls back to the [LRClib API](https://lrclib.net) if none is found, with auto-wrap and scroll.
- **Audio format support**: MP3, Opus, Ogg Vorbis, FLAC, via libmpg123, libopusfile, libvorbisfile, and libFLAC.
- **Native folder picker**: XDG Desktop Portal over D-Bus on Linux; WinAPI on Windows.
- **CJK font support**: full Unicode rendering for lyric content.
- **Cover art caching**, responsive layout, wide track view.

---

## Linux

### Build dependencies

Gio requires development headers for its GPU and windowing backends. Install these before building:

**Debian / Ubuntu:**

```bash
sudo apt install gcc pkg-config \
  libwayland-dev libx11-dev libx11-xcb-dev \
  libxkbcommon-x11-dev libxcursor-dev \
  libgles2-mesa-dev libegl1-mesa-dev \
  libffi-dev libvulkan-dev
```

**Arch Linux:**

```bash
sudo pacman -S base-devel wayland libx11 libxkbcommon libxcursor \
  mesa vulkan-headers
```

**Fedora / RHEL:**

```bash
sudo dnf install gcc pkg-config wayland-devel libX11-devel \
  libxkbcommon-x11-devel libXcursor-devel \
  mesa-libGLES-devel mesa-libEGL-devel \
  libffi-devel vulkan-headers
```

Vulkan support is optional but recommended for best rendering performance. You can verify it with `vulkaninfo`. On distributions like Arch, a Vulkan driver is not installed automatically; check your GPU vendor's package (`vulkan-radeon`, `vulkan-intel`, etc.).

### Runtime dependencies (audio codecs)

Because OngoPlayer loads codec libraries at runtime via purego, they do not need to be present at build time, only when you run the app. Install the shared libraries for whichever formats you want to play:

| Library                            | Format                          |
| ---------------------------------- | ------------------------------- |
| `libmpg123.so.0`                   | MP3                             |
| `libopusfile.so.0`                 | Opus                            |
| `libvorbisfile.so.3`               | Ogg Vorbis                      |
| `libFLAC.so.14` or `libFLAC.so.12` | FLAC                            |
| `libSDL3.so.0`                     | audio stream backend (required) |

**Debian / Ubuntu:**

```bash
sudo apt install libmpg123-0 libopusfile0 libvorbisfile3 libflac12 libsdl3-0
```

**Arch Linux:**

```bash
sudo pacman -S mpg123 opusfile libvorbis flac sdl3
```

**Fedora / RHEL:**

```bash
sudo dnf install mpg123-libs opusfile libvorbis flac-libs SDL3
```

SDL3 integrates with PulseAudio and PipeWire automatically. During development you can force a specific backend with `SDL_AUDIODRIVER=pulseaudio`.

---

## Windows

A packaged Windows installer is in development. It will bundle SDL3.dll and the required codec DLLs, so no manual setup will be needed.

---

## Roadmap

- [x] GPU-accelerated GUI via Gio
- [x] Synced lyric resolver and viewer (LRClib)
- [x] FLAC support
- [x] CJK font rendering
- [ ] Keyboard shortcuts (space, arrow keys, etc.)
- [ ] Online music streaming via yt-dlp (YouTube Music)
- [ ] Right panel with tabbed equalizer and settings

---

**Status:** early-stage, in active development.
