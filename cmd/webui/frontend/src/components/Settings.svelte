<script>
  // Settings.svelte is a modal panel for app preferences: a Discord Rich
  // Presence toggle and a lyrics font-size slider. It is presentational and
  // drives the store via the passed-in callbacks.
  //
  // Props:
  //   rpcEnabled: current RPC state
  //   fontSize: current lyrics font size in px
  //   animationsEnabled: current UI animation state
  //
  // Events:
  //   close                       -> dismiss the panel
  //   rpc -> detail: boolean       -> toggle RPC
  //   fontSize -> detail: px       -> change lyrics font size
  //   animations -> detail: boolean -> toggle UI micro-animations

  import { createEventDispatcher } from "svelte";
  import Icon from "./Icon.svelte";

  export let rpcEnabled = false;
  export let fontSize = 16;
  export let animationsEnabled = true;

  const dispatch = createEventDispatcher();

  function onFont(e) {
    dispatch("fontSize", parseInt(e.target.value, 10));
  }

  // Close when the backdrop (not the panel) is clicked.
  function onBackdrop(e) {
    if (e.target === e.currentTarget) dispatch("close");
  }

  // Allow Escape to dismiss the panel.
  function onKey(e) {
    if (e.key === "Escape") dispatch("close");
  }
</script>

<svelte:window on:keydown={onKey} />

<div
  class="backdrop"
  role="button"
  tabindex="-1"
  aria-label="Close settings"
  on:click={onBackdrop}
>
  <div class="panel" role="dialog" aria-label="Settings">
    <div class="panel-head">
      <span class="panel-title">Settings</span>
      <button class="close" on:click={() => dispatch("close")} title="Close">
        <Icon name="x" size={18} />
      </button>
    </div>

    <div class="row">
      <div class="row-text">
        <span class="row-label">Discord Rich Presence</span>
        <span class="row-desc">Show the current track on your Discord profile</span>
      </div>
      <button
        class="toggle"
        class:on={rpcEnabled}
        role="switch"
        aria-checked={rpcEnabled}
        on:click={() => dispatch("rpc", !rpcEnabled)}
      >
        <span class="knob"></span>
      </button>
    </div>

    <div class="row">
      <div class="row-text">
        <span class="row-label">Animations</span>
        <span class="row-desc">Smooth lyrics scrolling and control micro-animations</span>
      </div>
      <button
        class="toggle"
        class:on={animationsEnabled}
        role="switch"
        aria-checked={animationsEnabled}
        on:click={() => dispatch("animations", !animationsEnabled)}
      >
        <span class="knob"></span>
      </button>
    </div>

    <div class="row column">
      <div class="row-text">
        <span class="row-label">Lyrics font size</span>
        <span class="row-desc">{fontSize}px</span>
      </div>
      <input
        type="range"
        min="12"
        max="32"
        step="1"
        value={fontSize}
        on:input={onFont}
        aria-label="Lyrics font size"
        style="--pct: {((fontSize - 12) / (32 - 12)) * 100}%"
      />
    </div>
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.55);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 50;
  }

  .panel {
    width: 380px;
    max-width: 90vw;
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: 12px;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
    overflow: hidden;
  }

  .panel-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 18px;
    border-bottom: 1px solid var(--border);
  }

  .panel-title {
    font-size: 14px;
    font-weight: 700;
    color: var(--text);
  }

  .close {
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    color: var(--text-dim);
    cursor: pointer;
    padding: 4px;
    border-radius: 6px;
    transition: color 0.16s ease, background 0.16s ease;
  }

  .close:hover {
    color: var(--text);
    background: var(--surface-2);
  }

  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 16px 18px;
  }

  .row.column {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }

  .row + .row {
    border-top: 1px solid var(--border);
  }

  .row-text {
    display: flex;
    flex-direction: column;
    gap: 3px;
    min-width: 0;
  }

  .row.column .row-text {
    flex-direction: row;
    align-items: baseline;
    justify-content: space-between;
  }

  .row-label {
    font-size: 13px;
    color: var(--text);
  }

  .row-desc {
    font-size: 11.5px;
    color: var(--text-faint);
  }

  /* Toggle switch: monochrome, lightens when on. */
  .toggle {
    position: relative;
    width: 40px;
    height: 22px;
    flex-shrink: 0;
    border-radius: 11px;
    background: var(--surface-3);
    border: 1px solid var(--border);
    cursor: pointer;
    padding: 0;
    transition: background 0.18s ease;
  }

  .toggle.on {
    background: var(--text);
  }

  .knob {
    position: absolute;
    top: 2px;
    left: 2px;
    width: 16px;
    height: 16px;
    border-radius: 50%;
    background: var(--text-dim);
    transition: transform 0.18s ease, background 0.18s ease;
  }

  .toggle.on .knob {
    transform: translateX(18px);
    background: var(--bg);
  }

  /* Range input matches the player sliders. */
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
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background: var(--text);
    border: none;
    transition: transform 0.14s ease;
  }

  input[type="range"]:hover::-webkit-slider-thumb {
    transform: scale(1.15);
  }

  input[type="range"]::-moz-range-thumb {
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background: var(--text);
    border: none;
  }
</style>
