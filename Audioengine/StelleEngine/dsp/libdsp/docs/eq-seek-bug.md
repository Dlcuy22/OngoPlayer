# EQ Seek Bug — Full Postmortem

## Symptom

In EQ mode (activated by pressing `e`), pressing arrow keys to navigate bands and adjust gain caused the **playback position to jump** — the progress bar seeked forward by ~1 second, making it feel like the song was skipping. This happened on every gain adjustment.

---

## Iteration 1 — Wrong Assumption: Keybind Conflict

**Hypothesis:** The shifted arrow keys (`KEY_SLEFT`, `KEY_SRIGHT`, `KEY_SR`, `KEY_SF`) were not being recognized by ncurses, so the terminal fell back to regular arrow keys (`KEY_LEFT`, `KEY_RIGHT`), which were intercepted by the **player** key handler (triggering seek ±5s) instead of the **EQ** key handler.

**Fix:** Added regular arrow keys (`KEY_UP`/`KEY_DOWN`/`KEY_LEFT`/`KEY_RIGHT`) alongside shifted variants in `handle_eq_key`, and moved gain controls to J/K to eliminate any possible arrow key overlap.

**Result:** The "seek" persisted even with J/K — proving it was **never a keybinding issue**. Key logging confirmed `eq_mode` was `1` and the correct keycodes (393/402) were dispatched exclusively to `handle_eq_key`.

---

## Iteration 2 — Wrong Assumption: SDL Stream Clear + Decoder Read

**Hypothesis:** The "seek" was an audible/visible glitch caused by `player_eq_set_gain` calling `SDL_ClearAudioStream` (discarding buffered audio) followed by `decoder_read` (consuming 2048 samples from the ring buffer, freeing space, unblocking the decoder thread, which then decoded a large batch, advancing `op_pcm_tell` by ~1 second).

**Fix:** Removed `SDL_ClearAudioStream` + `decoder_read` from `player_eq_set_gain` — just set the coefficient.

**Result:** The seek disappeared, but the EQ change took **10–15 seconds** to become audible. The SDL audio stream had accumulated seconds of EQ-processed audio that had to drain before new coefficients took effect.

---

## Iteration 3 — Band-Aid: Stream Queue Limiter

**Hypothesis:** If the SDL stream is prevented from growing large, the EQ delay stays small. Skip pushing when the stream has enough queued.

**Fix:** Added a guard in `player_tick` — if `SDL_GetAudioStreamQueued` ≥ 96000 bytes, skip `SDL_PutAudioStreamData`.

**Bug:** `decoder_read` still ran on every tick (consuming 8192 samples from the ring buffer), but the push was skipped. Those 8192 samples were **silently discarded** — processed by `dsp_eq_process` but never sent to the audio device. This caused audio stutter, artifacts, and a perceived speed-up.

---

## Iteration 4 — Root Cause Found: Decoder Batch Size

**Final hypothesis:** The real "seek" was never about keybindings or stream clearing. The decoder thread decodes **48000 frames** per `op_read_float` call, advancing `op_pcm_tell` in **1-second increments**. This is visible as a discrete jump in the progress bar. When the ring buffer filled up (from stream-limiting) and was then drained (on the next push), the decoder unblocked and its batched 1-second advance became noticeable.

**Fix (two parts):**

1. **Reduce decoder batch size** from 48000 → **12000 frames** (250ms increments instead of 1s). `op_pcm_tell` now advances smoothly enough that no discrete jump is visible.

2. **Skip the entire tick** when the SDL stream has ≥ 192000 bytes (~0.5s). The guard is at the top of `player_tick` — if the stream is full enough, return immediately without reading from the ring buffer at all. This prevents unbounded stream growth (EQ delay stays ≤ 0.5s) without dropping any samples.

3. **`player_eq_set_gain` just sets the coefficient** — no `SDL_ClearAudioStream`, no `decoder_read`, no stream manipulation.

## Final State

| Concern | Behavior |
|---------|----------|
| Seek on gain change | None — no stream clearing, no decoder unblocking |
| EQ delay after change | ≤ 0.5 seconds (SDL stream drains naturally) |
| Audio artifacts | None — never drop processed samples |
| Stream growth | Capped at ~0.5s — stable |
| Decoder position advance | Smooth 250ms increments |

## Files Modified

- `decoder.c` — batch size 48000 → 12000 frames
- `player.c` — `player_eq_set_gain` simplified to just set coefficient; `player_tick` skips tick when stream ≥ 0.5s
- `main.c` — EQ keys: J/K for gain, UP/DOWN band, arrows ignored
