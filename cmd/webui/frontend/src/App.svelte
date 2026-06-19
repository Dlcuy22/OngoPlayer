<script>
  // App.svelte is the root layout orchestrator for the OngoPlayer WebUI.
  //
  // Purpose: Defines the main visual layout (sidebar tracklist, center multi-page
  // viewport, resizable right now-playing panel, bottom player bar) and wires
  // reactive stores to the Svelte components.

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
    loadingIndex,
    lyricsLoading,
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
    playerReorderQueue,
    playerRemoveTrack,
    playerInsertYTMSongAt,
    toggleShuffle,
    cycleLoop,
    openSettings,
    closeSettings,
    setRpcEnabled,
    setLyricsFontSize,
    setAnimationsEnabled,
    navigateTo,
  } from "./lib/playerStore.js";

  import Tracklist from "./components/Tracklist.svelte";
  import CenterPanel from "./components/CenterPanel.svelte";
  import NowPlaying from "./components/NowPlaying.svelte";
  import Player from "./components/Player.svelte";
  import Settings from "./components/Settings.svelte";
  import SearchBox from "./components/SearchBox.svelte";
  import Icon from "./components/Icon.svelte";

  // The active queue position is carried on TrackInfo.index by the backend.
  $: currentIndex =
    $currentTrack && typeof $currentTrack.index === "number"
      ? $currentTrack.index
      : -1;

  // Resizable Right Panel
  let rightPanelWidth = 340;
  const MIN_WIDTH = 280;
  const MAX_WIDTH = 460;

  function startResize(e) {
    e.preventDefault();
    const startWidth = rightPanelWidth;
    const startX = e.clientX;

    function onMouseMove(moveEvent) {
      const diff = moveEvent.clientX - startX;
      // Dragging left (negative diff) increases right panel width
      rightPanelWidth = Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, startWidth - diff));
    }

    function onMouseUp() {
      if (typeof localStorage !== "undefined") {
        localStorage.setItem("ongo.rightPanelWidth", rightPanelWidth.toString());
      }
      window.removeEventListener("mousemove", onMouseMove);
      window.removeEventListener("mouseup", onMouseUp);
    }

    window.addEventListener("mousemove", onMouseMove);
    window.addEventListener("mouseup", onMouseUp);
  }

  onMount(() => {
    // Disable right-click globally for that native desktop app feel.
    document.addEventListener("contextmenu", (event) => event.preventDefault());
    
    // Load persisted panel width
    if (typeof localStorage !== "undefined") {
      const saved = localStorage.getItem("ongo.rightPanelWidth");
      if (saved) {
        const parsed = parseInt(saved, 10);
        if (!isNaN(parsed)) {
          rightPanelWidth = Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, parsed));
        }
      }
    }

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
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <span class="titlebar-text" on:click={() => navigateTo("home", null, true)}>OngoPlayer</span>
    <div class="search-container">
      <SearchBox />
    </div>
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
        on:reorder={(e) => playerReorderQueue(e.detail.from, e.detail.to)}
        on:removeTrack={(e) => playerRemoveTrack(e.detail)}
        on:insertYTMTrack={(e) => playerInsertYTMSongAt(
          e.detail.index,
          e.detail.track.id,
          e.detail.track.name,
          e.detail.track.artist,
          e.detail.track.album,
          e.detail.track.lyrics_browse_id,
          e.detail.track.thumbnail
        )}
      />
    </aside>

    <main class="center-panel">
      <CenterPanel />
    </main>

    <!-- Resizable player-panel on the right -->
    <aside class="player-panel" style="width: {rightPanelWidth}px">
      <!-- Drag handle -->
      <div class="resize-handle" on:mousedown={startResize}></div>
      <div class="player-panel-content">
        <NowPlaying
          track={$currentTrack}
          isPlaying={$isPlaying}
          cover={$coverUrl}
          lyrics={$lyrics}
          position={$position}
          fontSize={$lyricsFontSize}
          animations={$animationsEnabled}
          lyricsLoading={$lyricsLoading}
          on:seek={(e) => playerSeekTo(e.detail)}
        />
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
      isLocked={$loadingIndex !== -1 && $currentTrack && $loadingIndex === $currentTrack.index}
      cover={$coverUrl}
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
    loopMode={$loopMode}
    shuffle={$shuffle}
    on:close={closeSettings}
    on:rpc={(e) => setRpcEnabled(e.detail)}
    on:fontSize={(e) => setLyricsFontSize(e.detail)}
    on:animations={(e) => setAnimationsEnabled(e.detail)}
    on:loop={cycleLoop}
    on:shuffle={toggleShuffle}
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
    width: 6px;
    height: 6px;
  }
  :global(*::-webkit-scrollbar-thumb) {
    background: var(--surface-3);
    border-radius: 3px;
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
    height: 44px;
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
    cursor: pointer;
    user-select: none;
  }

  .search-container {
    flex: 1;
    display: flex;
    justify-content: center;
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

  /* Resizable player panel styling */
  .player-panel {
    position: relative;
    flex-shrink: 0;
    background-color: var(--surface-1);
    border-left: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    height: 100%;
  }

  .player-panel-content {
    flex: 1;
    overflow: hidden;
    min-height: 0;
    width: 100%;
  }

  .resize-handle {
    position: absolute;
    top: 0;
    bottom: 0;
    left: -4px;
    width: 8px;
    cursor: col-resize;
    z-index: 10;
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
    .player-panel {
      display: none;
    }
  }

  @media (max-width: 620px) {
    .sidebar {
      width: 180px;
    }
  }
</style>
