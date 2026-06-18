<script>
  // App.svelte is the root layout orchestrator for the OngoPlayer WebUI.
  //
  // Purpose: Defines the main visual layout (sidebar tracklist, center now
  // playing, bottom player bar) and wires the reactive Svelte stores to the
  // individual UI components.
  //
  // Key Components:
  //   - Tracklist:  sidebar queue + folder picker
  //   - NowPlaying: center cover art, title, lyrics area
  //   - Player:     bottom transport controls + progress + volume
  //
  // Dependencies:
  //   - lib/playerStore.js: state and backend API management

  import { onMount } from "svelte";
  import {
    initPlayerSync,
    position,
    duration,
    isPlaying,
    volume,
    currentTrack,
    queue,
    shuffle,
    loopMode,
    coverUrl,
    lyrics,
    showSettings,
    rpcEnabled,
    lyricsFontSize,
    animationsEnabled,
    playerToggle,
    playerSeekStart,
    playerSeekInput,
    playerSeekEnd,
    playerSeekTo,
    playerVolumeChange,
    playerNext,
    playerPrev,
    playerPickFolder,
    playerAppendFolder,
    playerPlayQueueIndex,
    toggleShuffle,
    cycleLoop,
    openSettings,
    closeSettings,
    setRpcEnabled,
    setLyricsFontSize,
    setAnimationsEnabled,
  } from "./lib/playerStore.js";

  import Tracklist from "./components/Tracklist.svelte";
  import NowPlaying from "./components/NowPlaying.svelte";
  import Player from "./components/Player.svelte";
  import Settings from "./components/Settings.svelte";
  import Icon from "./components/Icon.svelte";

  // The active queue position is carried on TrackInfo.index by the backend.
  $: currentIndex =
    $currentTrack && typeof $currentTrack.index === "number"
      ? $currentTrack.index
      : -1;

  onMount(() => {
    // Disable right-click globally for that native Gio feel.
    document.addEventListener("contextmenu", (event) => event.preventDefault());
    initPlayerSync();

    // Pause CSS animations while the window is hidden/minimized. WebKitGTK
    // keeps compositing animated elements (the equalizer) even when not
    // visible, which is a needless source of idle CPU.
    const onVisibility = () => {
      document.body.classList.toggle("app-hidden", document.hidden);
    };
    document.addEventListener("visibilitychange", onVisibility);
    return () => document.removeEventListener("visibilitychange", onVisibility);
  });
</script>

<div class="app-root">
  <header class="titlebar">
    <span class="titlebar-text">OngoPlayer</span>
    <button class="settings-btn" on:click={openSettings} title="Settings">
      <Icon name="settings" size={16} />
    </button>
  </header>

  <div class="main-content">
    <aside class="sidebar">
      <Tracklist
        queue={$queue}
        {currentIndex}
        isPlaying={$isPlaying}
        on:playTrack={(e) => playerPlayQueueIndex(e.detail)}
        on:pickFolder={playerPickFolder}
        on:appendFolder={playerAppendFolder}
      />
    </aside>

    <main class="center-panel">
      <NowPlaying
        track={$currentTrack}
        isPlaying={$isPlaying}
        cover={$coverUrl}
        lyrics={$lyrics}
        position={$position}
        fontSize={$lyricsFontSize}
        animations={$animationsEnabled}
        on:seek={(e) => playerSeekTo(e.detail)}
      />
    </main>

    <!-- Right panel: reserved for DSP and future tabs. -->
    <aside class="right-panel">
      <div class="rp-tabs">
        <button class="rp-tab active" title="Equalizer / DSP">
          <Icon name="sliders-horizontal" size={16} />
        </button>
      </div>
      <div class="rp-body">
        <div class="rp-placeholder">
          <Icon name="sliders-horizontal" size={22} strokeWidth={1.5} />
          <span>DSP and effects</span>
          <span class="rp-soon">Coming soon</span>
        </div>
      </div>
    </aside>
  </div>

  <footer class="player-bar">
    <Player
      position={$position}
      duration={$duration}
      isPlaying={$isPlaying}
      volume={$volume}
      track={$currentTrack}
      shuffle={$shuffle}
      loopMode={$loopMode}
      animations={$animationsEnabled}
      on:toggle={() => playerToggle($isPlaying)}
      on:prev={playerPrev}
      on:next={playerNext}
      on:seekStart={playerSeekStart}
      on:seekInput={(e) => playerSeekInput(e.detail)}
      on:seekEnd={(e) => playerSeekEnd(e.detail)}
      on:volumeChange={(e) => playerVolumeChange(e.detail)}
      on:shuffle={toggleShuffle}
      on:loop={cycleLoop}
    />
  </footer>
</div>

{#if $showSettings}
  <Settings
    rpcEnabled={$rpcEnabled}
    fontSize={$lyricsFontSize}
    animationsEnabled={$animationsEnabled}
    on:close={closeSettings}
    on:rpc={(e) => setRpcEnabled(e.detail)}
    on:fontSize={(e) => setLyricsFontSize(e.detail)}
    on:animations={(e) => setAnimationsEnabled(e.detail)}
  />
{/if}

<style>
  /* Monochrome design tokens. No color hues, no gradients beyond grayscale
     fills. Hover states lighten the background or text only, no glow. */
  :global(:root) {
    --bg: #0c0c0e;
    --surface-1: #111114;
    --surface-2: #16161a;
    --surface-3: #202026;
    --surface-hover: #2a2a31;
    --border: rgba(255, 255, 255, 0.07);
    --border-strong: rgba(255, 255, 255, 0.16);
    --text: rgba(255, 255, 255, 0.92);
    --text-dim: rgba(255, 255, 255, 0.62);
    --text-faint: rgba(255, 255, 255, 0.38);
  }

  :global(body) {
    margin: 0;
    padding: 0;
    background-color: var(--bg);
    color: var(--text);
    font-family: "NotoSansJP", -apple-system, BlinkMacSystemFont, "Segoe UI",
      Roboto, Helvetica, Arial, sans-serif;
    user-select: none;
    -webkit-user-select: none;
    cursor: default;
    overflow: hidden;
  }

  /* When the window is hidden, suspend all running animations to drop the
     compositor's idle CPU cost. */
  :global(body.app-hidden *) {
    animation-play-state: paused !important;
  }

  /* Slim monochrome scrollbar. */
  :global(*::-webkit-scrollbar) {
    width: 8px;
  }
  :global(*::-webkit-scrollbar-thumb) {
    background: var(--surface-3);
    border-radius: 4px;
  }
  :global(*::-webkit-scrollbar-thumb:hover) {
    background: var(--border-strong);
  }
  :global(*::-webkit-scrollbar-track) {
    background: transparent;
  }

  .app-root {
    display: flex;
    flex-direction: column;
    height: 100vh;
    width: 100vw;
  }

  .titlebar {
    height: 40px;
    background-color: var(--surface-1);
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 12px 0 16px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .settings-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    color: var(--text-dim);
    cursor: pointer;
    padding: 6px;
    border-radius: 6px;
    transition: color 0.16s ease, background 0.16s ease;
  }

  .settings-btn:hover {
    color: var(--text);
    background: var(--surface-2);
  }

  .titlebar-text {
    font-size: 13px;
    font-weight: 700;
    letter-spacing: 0.02em;
    color: var(--text);
  }

  .main-content {
    display: flex;
    flex: 1;
    overflow: hidden;
    min-height: 0;
  }

  .sidebar {
    width: 290px;
    flex-shrink: 0;
    background-color: var(--surface-1);
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .center-panel {
    flex: 1;
    background-color: var(--bg);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    min-width: 0;
  }

  .right-panel {
    width: 280px;
    flex-shrink: 0;
    background-color: var(--surface-1);
    border-left: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .rp-tabs {
    display: flex;
    gap: 4px;
    padding: 10px 12px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .rp-tab {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 34px;
    height: 30px;
    background: transparent;
    border: none;
    border-radius: 7px;
    color: var(--text-faint);
    cursor: pointer;
    transition: color 0.16s ease, background 0.16s ease;
  }

  .rp-tab:hover {
    color: var(--text);
    background: var(--surface-2);
  }

  .rp-tab.active {
    color: var(--text);
    background: var(--surface-3);
  }

  .rp-body {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 0;
  }

  .rp-placeholder {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    color: var(--text-faint);
    font-size: 12.5px;
  }

  .rp-soon {
    font-size: 11px;
    color: var(--text-faint);
    opacity: 0.7;
  }

  .player-bar {
    height: 84px;
    background-color: var(--surface-1);
    border-top: 1px solid var(--border);
    flex-shrink: 0;
    display: flex;
    align-items: center;
    padding: 0 24px;
  }

  /* Hide the right panel first when space is tight, then narrow the sidebar. */
  @media (max-width: 900px) {
    .right-panel {
      display: none;
    }
  }

  @media (max-width: 620px) {
    .sidebar {
      width: 180px;
    }
  }
</style>
