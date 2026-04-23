# OngoPlayer

## A dead simple audio player that's actually good

OngoPlayer uses [purego](https://github.com/ebitengine/purego) to call native audio libraries directly, avoiding the overhead of CGo and providing a fast, efficient audio playback experience.

### Why use C libs in the first place?

Because almost all audio codec implementations that are actually efficient and fast are written in C,
and I don't want to reimplement all of them in Go, nor can I guarantee the same speed and performance as the official C implementations.

### Roadmap

- [ ] Add more audio format support
- [ ] Add a usable user interface
- [ ] Add automatic synced lyric resolver ([LRClib](https://lrclib.net))

### Status

This project is in early development. Basic features are still being implemented.
