<script>
  // Player.svelte is the bottom transport bar: previous / play-pause / next,
  // a scrubbable progress bar with elapsed and total times, and a volume
  // slider. All state is owned by the store; this component is presentational
  // and emits intent via events.
  //
  // Props:
  //   position: current playback position in seconds
  //   duration: track length in seconds
  //   isPlaying: playback state
  //   volume: 0-100
  //   track: TrackInfo or null (drives the compact title)
  //   shuffle: shuffle enabled
  //   loopMode: 0 off, 1 all, 2 one
  //
  // Events:
  //   toggle, prev, next            -> no detail
  //   seekStart                     -> no detail (user grabbed the bar)
  //   seekInput -> detail: seconds  (live drag, no IPC)
  //   seekEnd   -> detail: seconds  (commit seek)
  //   volumeChange -> detail: 0-100
  //   shuffle, loop                 -> no detail (cycle requests)

  import { createEventDispatcher } from "svelte";
  import Icon from "./Icon.svelte";

  export let position = 0;
  export let duration = 0;
  export let isPlaying = false;
  export let volume = 30;
  export let track = null;
  export let shuffle = false;
  export let loopMode = 0;
  export let animations = true;
  export let isLocked = false;
  export let cover = "";

  const dispatch = createEventDispatcher();

  $: progressPct = duration > 0 ? (position / duration) * 100 : 0;

  function fmt(secs) {
    if (!secs || secs < 0 || isNaN(secs)) return "0:00";
    const m = Math.floor(secs / 60);
    const s = Math.floor(secs % 60);
    return `${m}:${s.toString().padStart(2, "0")}`;
  }

  function trackTitle(t) {
    if (!t) return "";
    if (t.title) return t.title;
    const name = t.name || "";
    const dot = name.lastIndexOf(".");
    return dot > 0 ? name.slice(0, dot) : name;
  }

  $: loopTitle =
    loopMode === 2 ? "Repeat one" : loopMode === 1 ? "Repeat all" : "Repeat off";

  function onSeekInput(e) {
    dispatch("seekInput", parseFloat(e.target.value));
  }

  function onSeekChange(e) {
    dispatch("seekEnd", parseFloat(e.target.value));
  }

  function onVolume(e) {
    dispatch("volumeChange", parseInt(e.target.value, 10));
  }
</script>

<div class="player" class:anim={animations} class:locked={isLocked}>
  <!-- Left: compact now-playing label -->
  <div class="np">
    {#if track}
      <div class="np-cover">
        {#if cover}
          <img src={cover} alt="" />
        {:else}
          <Icon name="music" size={14} strokeWidth={1.5} />
        {/if}
      </div>
      <div class="np-meta">
        <span class="np-title" title={trackTitle(track)}>
          {trackTitle(track)}
        </span>
        <span class="np-sub">{track.artist || "Local file"}</span>
      </div>
    {:else}
      <div class="np-cover empty">
        <Icon name="music" size={14} strokeWidth={1.5} />
      </div>
      <div class="np-meta">
        <span class="np-title muted">Not playing</span>
      </div>
    {/if}
  </div>

  <!-- Center: controls + progress -->
  <div class="center">
    <div class="controls">
      <button
        class="ctl tiny"
        class:on={shuffle}
        on:click={() => !isLocked && dispatch("shuffle")}
        disabled={isLocked}
        title={shuffle ? "Shuffle on" : "Shuffle off"}
      >
        <Icon name="shuffle" size={16} />
      </button>

      <button
        class="ctl"
        on:click={() => !isLocked && dispatch("prev")}
        disabled={isLocked}
        title="Previous"
      >
        <Icon name="skip-back" size={18} />
      </button>

      <button
        class="ctl play"
        on:click={() => !isLocked && dispatch("toggle")}
        disabled={isLocked}
        title={isPlaying ? "Pause" : "Play"}
      >
        {#if isPlaying}
          <Icon name="pause" size={20} />
        {:else}
          <Icon name="play" size={20} />
        {/if}
      </button>

      <button
        class="ctl"
        on:click={() => !isLocked && dispatch("next")}
        disabled={isLocked}
        title="Next"
      >
        <Icon name="skip-forward" size={18} />
      </button>

      <button
        class="ctl tiny"
        class:on={loopMode !== 0}
        on:click={() => !isLocked && dispatch("loop")}
        disabled={isLocked}
        title={loopTitle}
      >
        <Icon name={loopMode === 2 ? "repeat-1" : "repeat"} size={16} />
      </button>
    </div>

    <div class="progress">
      <span class="time">{fmt(position)}</span>
      <div class="bar-wrap" style="--pct: {progressPct}%">
        <input
          type="range"
          min="0"
          max={duration || 0}
          step="0.1"
          value={position}
          on:mousedown={() => !isLocked && dispatch("seekStart")}
          on:input={onSeekInput}
          on:change={onSeekChange}
          disabled={isLocked}
          aria-label="Seek"
        />
      </div>
      <span class="time">{fmt(duration)}</span>
    </div>
  </div>

  <!-- Right: volume -->
  <div class="volume">
    <button
      class="ctl small"
      on:click={() => dispatch("volumeChange", volume > 0 ? 0 : 30)}
      title={volume > 0 ? "Mute" : "Unmute"}
    >
      <Icon name={volume > 0 ? "volume-2" : "volume-x"} size={16} />
    </button>
    <div class="vol-wrap" style="--pct: {volume}%">
      <input
        type="range"
        min="0"
        max="100"
        step="1"
        value={volume}
        on:input={onVolume}
        aria-label="Volume"
      />
    </div>
  </div>
</div>

<style>
  .player {
    display: grid;
    grid-template-columns: 1fr minmax(320px, 2fr) 1fr;
    align-items: center;
    gap: 16px;
    width: 100%;
    height: 100%;
  }

  .player.locked .controls .ctl,
  .player.locked .progress {
    opacity: 0.4;
    pointer-events: none;
    cursor: not-allowed;
  }

  .np {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }

  .np-cover {
    width: 32px;
    height: 32px;
    border-radius: 4px;
    background: var(--surface-3);
    border: 1px solid var(--border);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-faint);
    flex-shrink: 0;
    overflow: hidden;
  }

  .np-cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }

  .np-meta {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .np-title {
    font-size: 13px;
    font-weight: 600;
    color: var(--text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .np-title.muted {
    color: var(--text-faint);
    font-weight: 500;
  }

  .np-sub {
    font-size: 11px;
    color: var(--text-faint);
  }

  .center {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }

  .controls {
    display: flex;
    align-items: center;
    gap: 14px;
  }

  .ctl {
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    color: var(--text-dim);
    cursor: pointer;
    padding: 6px;
    border-radius: 50%;
    transition: color 0.16s ease, background 0.16s ease;
  }

  .ctl:hover {
    color: var(--text);
    background: var(--surface-2);
  }

  /* Micro-animations: subtle scale on hover and press, only when enabled. */
  .player.anim .ctl {
    transition: color 0.16s ease, background 0.16s ease, transform 0.16s ease;
  }

  .player.anim .ctl:hover {
    transform: scale(1.12);
  }

  .player.anim .ctl:active {
    transform: scale(0.92);
  }

  .player.anim .ctl.play:active {
    transform: scale(0.9);
  }

  .ctl.play {
    width: 38px;
    height: 38px;
    background: var(--text);
    color: var(--bg);
  }

  .ctl.play:hover {
    background: #fff;
  }

  .ctl.small {
    padding: 5px;
  }

  .ctl.tiny {
    padding: 5px;
    color: var(--text-faint);
  }

  .ctl.tiny:hover {
    color: var(--text);
  }

  /* Active mode (shuffle on, loop all/one): brighter text, no glow. */
  .ctl.tiny.on {
    color: var(--text);
  }

  .progress {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
  }

  .time {
    font-size: 11px;
    font-variant-numeric: tabular-nums;
    color: var(--text-faint);
    width: 36px;
    flex-shrink: 0;
    text-align: center;
  }

  .bar-wrap,
  .vol-wrap {
    flex: 1;
    display: flex;
    align-items: center;
  }

  .volume {
    display: flex;
    align-items: center;
    gap: 8px;
    justify-content: flex-end;
  }

  .vol-wrap {
    max-width: 110px;
  }

  /* Range input: monochrome track with a filled portion via --pct. */
  input[type="range"] {
    -webkit-appearance: none;
    appearance: none;
    width: 100%;
    height: 4px;
    border-radius: 2px;
    background: linear-gradient(
      to right,
      var(--text) 0%,
      var(--text) var(--pct),
      var(--surface-3) var(--pct),
      var(--surface-3) 100%
    );
    cursor: pointer;
  }

  input[type="range"]::-webkit-slider-thumb {
    -webkit-appearance: none;
    appearance: none;
    width: 12px;
    height: 12px;
    border-radius: 50%;
    background: var(--text);
    border: none;
    box-shadow: 0 0 0 3px var(--bg);
    transition: transform 0.14s ease;
  }

  input[type="range"]:hover::-webkit-slider-thumb {
    transform: scale(1.2);
  }

  /* Volume thumb: no black separation ring at rest; restore it on hover so the
     knob stays legible while dragging over the white filled portion. */
  .vol-wrap input[type="range"]::-webkit-slider-thumb {
    box-shadow: none;
  }

  .vol-wrap input[type="range"]:hover::-webkit-slider-thumb {
    box-shadow: 0 0 0 3px var(--bg);
  }

  input[type="range"]::-moz-range-thumb {
    width: 12px;
    height: 12px;
    border-radius: 50%;
    background: var(--text);
    border: none;
  }
</style>
