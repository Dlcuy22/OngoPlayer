<script>
  // App.svelte is the root layout orchestrator for the OngoPlayer WebUI.
  //
  // Purpose: Defines the main visual layout (Sidebar, Center Panel, Player Bar)
  // and wires the reactive Svelte stores to the individual UI components.
  //
  // Key Components:
  //   - Tracklist: Displays the current queue
  //   - PlayerBar: Displays transport controls and progress
  //
  // Dependencies:
  //   - lib/playerStore.js: State and API management
  //
  // Example:
  //   <App />

  import { onMount } from "svelte";
  import { 
    initPlayerSync, position, duration, isPlaying, volume, currentTrack, queue,
    playerToggle, playerSeekStart, playerSeekInput, playerSeekEnd,
    playerVolumeChange, playerNext, playerPrev, playerPickFolder, playerPlayQueueIndex
  } from "./lib/playerStore.js";

  import PlayerBar from "./components/Player.svelte";
  import Tracklist from "./components/Tracklist.svelte";

  onMount(() => {
    // Disable right-click globally for that native Gio feel
    document.addEventListener("contextmenu", (event) => event.preventDefault());

    // Start listening to Wails events and syncing state
    initPlayerSync();
  });
</script>

<!-- The root layout replicating Gio's app.go layout -->
<div class="app-root">
  <!-- Header / Titlebar -->
  <header class="titlebar">
    <span class="titlebar-text">OngoPlayer</span>
    <!-- Add window controls, volume slider, etc. here -->
  </header>

  <div class="main-content">
    <!-- Sidebar -->
    <aside class="sidebar">
      <!-- 
        <Tracklist queue={$queue} currentTrack={$currentTrack} on:playTrack={(e) => playerPlayQueueIndex(e.detail)} /> 
      -->
      <div class="placeholder">Tracklist Panel</div>
    </aside>

    <!-- Main Panel (Now Playing + Lyrics) -->
    <main class="center-panel">
      <div class="placeholder">Now Playing & Lyrics Panel</div>
    </main>
  </div>

  <!-- Bottom Player Bar -->
  <footer class="player-bar">
    <!-- 
      <PlayerBar 
        position={$position} duration={$duration} isPlaying={$isPlaying} volume={$volume}
        on:toggle={() => playerToggle($isPlaying)}
        on:seekStart={playerSeekStart}
        on:seekInput={(e) => playerSeekInput(e.detail)}
        on:seekEnd={(e) => playerSeekEnd(e.detail)}
        on:next={playerNext}
        on:prev={playerPrev}
        on:volumeChange={(e) => playerVolumeChange(e.detail)}
      /> 
    -->
    <div class="placeholder">Player Controls & Progress</div>
  </footer>
</div>

<style>
  /* Global solid UI resets */
  :global(body) {
    margin: 0;
    padding: 0;
    background-color: var(--bg-color, #0e0e12);
    color: #e0e0e0;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto,
      Helvetica, Arial, sans-serif;
    user-select: none;
    -webkit-user-select: none;
    cursor: default;
    overflow: hidden;
  }

  /* Layout exactly like Gio's app.go Flex layout */
  .app-root {
    display: flex;
    flex-direction: column;
    height: 100vh;
    width: 100vw;
  }

  .titlebar {
    --wails-draggable: drag;
    height: 40px;
    background-color: #111116; /* Sidebar/Header color from Gio theme */
    display: flex;
    align-items: center;
    padding: 0 16px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.06); /* Divider */
    flex-shrink: 0;
  }

  .titlebar-text {
    font-size: 13px;
    font-weight: 600;
    color: rgba(255, 255, 255, 0.88);
  }

  .main-content {
    display: flex;
    flex: 1;
    overflow: hidden; /* Prevent body scrolling */
  }

  .sidebar {
    width: 240px;
    background-color: #111116;
    border-right: 1px solid rgba(255, 255, 255, 0.06);
    display: flex;
    flex-direction: column;
  }

  .center-panel {
    flex: 1;
    background-color: #0e0e12;
    display: flex;
    flex-direction: column;
  }

  .player-bar {
    height: 80px;
    background-color: #0e0e12;
    border-top: 1px solid rgba(255, 255, 255, 0.06);
    flex-shrink: 0;
    display: flex;
    align-items: center;
    padding: 0 24px;
  }

  /* Just for testing canvas */
  .placeholder {
    color: #616161;
    font-size: 12px;
    margin: auto;
    text-align: center;
    padding: 20px;
  }
</style>
