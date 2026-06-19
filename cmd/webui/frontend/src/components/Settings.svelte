<script>
  // Settings.svelte is a modal panel for app preferences: Discord RPC,
  // playback loop/shuffle behavior, and lyrics styling. Organized into tabs.
  //
  // Props:
  //   rpcEnabled: current RPC state
  //   fontSize: current lyrics font size in px
  //   animationsEnabled: current UI animation state
  //   loopMode: current playback loop mode (0 off, 1 all, 2 one)
  //   shuffle: current shuffle state (boolean)

  import { createEventDispatcher } from "svelte";
  import Icon from "./Icon.svelte";

  export let rpcEnabled = false;
  export let fontSize = 16;
  export let animationsEnabled = true;
  export let loopMode = 0;
  export let shuffle = false;

  const dispatch = createEventDispatcher();
  let activeTab = "general"; // "general" | "playback" | "appearance"

  function onFont(e) {
    dispatch("fontSize", parseInt(e.target.value, 10));
  }

  function onBackdrop(e) {
    if (e.target === e.currentTarget) dispatch("close");
  }

  function onKey(e) {
    if (e.key === "Escape") dispatch("close");
  }

  function getLoopModeLabel(mode) {
    if (mode === 1) return "Loop All";
    if (mode === 2) return "Loop One";
    return "Loop Off";
  }
</script>

<svelte:window on:keydown={onKey} />

<!-- svelte-ignore a11y-click-events-have-key-events -->
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

    <!-- Tabbed navigation header -->
    <div class="tabs">
      <button
        class="tab-item"
        class:active={activeTab === "general"}
        on:click={() => (activeTab = "general")}
      >
        General
      </button>
      <button
        class="tab-item"
        class:active={activeTab === "playback"}
        on:click={() => (activeTab = "playback")}
      >
        Playback
      </button>
      <button
        class="tab-item"
        class:active={activeTab === "appearance"}
        on:click={() => (activeTab = "appearance")}
      >
        Appearance
      </button>
    </div>

    <div class="settings-content">
      {#if activeTab === "general"}
        <div class="tab-pane">
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
        </div>
      {:else if activeTab === "playback"}
        <div class="tab-pane">
          <div class="row">
            <div class="row-text">
              <span class="row-label">Pre-shuffle Queue</span>
              <span class="row-desc">Physically shuffle/reorder queue tracks visually</span>
            </div>
            <button
              class="toggle"
              class:on={shuffle}
              role="switch"
              aria-checked={shuffle}
              on:click={() => dispatch("shuffle")}
            >
              <span class="knob"></span>
            </button>
          </div>

          <div class="row">
            <div class="row-text">
              <span class="row-label">Repeat Mode</span>
              <span class="row-desc">Current: {getLoopModeLabel(loopMode)}</span>
            </div>
            <button
              class="action-btn"
              on:click={() => dispatch("loop")}
            >
              <Icon name="repeat" size={14} />
              <span>Cycle Mode</span>
            </button>
          </div>
        </div>
      {:else if activeTab === "appearance"}
        <div class="tab-pane">
          <div class="row">
            <div class="row-text">
              <span class="row-label">Animations</span>
              <span class="row-desc">Smooth lyrics scrolling and control transitions</span>
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
      {/if}
    </div>
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 50;
  }

  .panel {
    width: 400px;
    max-width: 90vw;
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: 12px;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.6);
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  .panel-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 18px 14px;
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

  /* Tabs styling */
  .tabs {
    display: flex;
    border-bottom: 1px solid var(--border);
    background: var(--surface-1);
  }

  .tab-item {
    flex: 1;
    background: transparent;
    border: none;
    padding: 12px;
    font-size: 12.5px;
    color: var(--text-dim);
    cursor: pointer;
    text-align: center;
    border-bottom: 2px solid transparent;
    transition: color 0.16s ease, border-color 0.16s ease;
  }

  .tab-item:hover {
    color: var(--text);
  }

  .tab-item.active {
    color: var(--text);
    border-bottom-color: var(--text-dim);
    font-weight: 600;
  }

  .settings-content {
    min-height: 200px;
    max-height: 320px;
    overflow-y: auto;
  }

  .tab-pane {
    display: flex;
    flex-direction: column;
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
    font-weight: 500;
  }

  .row-desc {
    font-size: 11.5px;
    color: var(--text-faint);
  }

  /* Toggle switch */
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

  /* General action button */
  .action-btn {
    display: flex;
    align-items: center;
    gap: 6px;
    background: var(--surface-2);
    border: 1px solid var(--border);
    color: var(--text-dim);
    padding: 6px 12px;
    border-radius: 6px;
    font-size: 12px;
    cursor: pointer;
    transition: background-color 0.16s ease, color 0.16s ease;
  }

  .action-btn:hover {
    background: var(--surface-hover);
    color: var(--text);
  }

  /* Range input styling */
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
</style>
