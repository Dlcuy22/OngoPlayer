<script>
  // HomePage.svelte is the default dashboard view.
  //
  // Purpose: Displays queue stats, quick folder picking buttons, and current
  // track quick access.
  //
  // Dependencies:
  //   - lib/playerStore.js: queue, currentTrack, and play actions

  import {
    queue,
    currentTrack,
    playerPickFolder,
    playerAppendFolder,
    playerClearQueue,
  } from "../../lib/playerStore.js";
  import Icon from "../Icon.svelte";
</script>

<div class="home-page">
  <div class="hero-section">
    <div class="logo-outer">
      <Icon name="music" size={40} strokeWidth={1.5} />
    </div>
    <h1>OngoPlayer <span class="version-badge">v1.0.0</span></h1>
    <p>Minimalist local library & YouTube Music player.</p>
  </div>

  <div class="dashboard-grid">
    <div class="card stat-card">
      <h3>Active Queue</h3>
      <div class="stat-number">{$queue.length}</div>
      <p class="stat-desc">Tracks ready for playback</p>
      {#if $queue.length > 0}
        <button class="action-btn text-btn" on:click={playerClearQueue}>
          <Icon name="trash-2" size={13} />
          <span>Clear Queue</span>
        </button>
      {/if}
    </div>

    <div class="card action-card">
      <h3>Add Music</h3>
      <div class="actions-list">
        <button class="action-btn" on:click={playerPickFolder}>
          <Icon name="folder" size={14} />
          <span>Open Folder</span>
        </button>
        <button class="action-btn" on:click={playerAppendFolder}>
          <Icon name="folder-plus" size={14} />
          <span>Append Folder</span>
        </button>
      </div>
    </div>

    {#if $currentTrack}
      <div class="card playing-card">
        <h3>Now Playing</h3>
        <div class="track-info">
          <div class="track-title">{$currentTrack.title || $currentTrack.name}</div>
          <div class="track-artist">{$currentTrack.artist || "Unknown Artist"}</div>
          <div class="track-source">
            <Icon name={$currentTrack.format === "YTM" ? "globe" : "disc"} size={12} strokeWidth={1.5} />
            <span>{$currentTrack.format || "Local"}</span>
          </div>
        </div>
      </div>
    {/if}
  </div>
</div>

<style>
  .home-page {
    padding: 32px;
    flex: 1;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 32px;
    background-color: var(--bg);
  }

  .hero-section {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }

  .logo-outer {
    background: var(--surface-1);
    border: 1px solid var(--border);
    padding: 12px;
    border-radius: 12px;
    color: var(--text);
  }

  h1 {
    font-size: 24px;
    font-weight: 700;
    margin: 0;
    color: var(--text);
  }

  .hero-section p {
    font-size: 13.5px;
    color: var(--text-dim);
    margin: 0;
  }

  .dashboard-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
    gap: 16px;
  }

  .card {
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 20px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .card h3 {
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-faint);
    margin: 0;
  }

  .stat-number {
    font-size: 36px;
    font-weight: 700;
    color: var(--text);
    line-height: 1;
  }

  .stat-desc {
    font-size: 12.5px;
    color: var(--text-faint);
    margin: 0;
  }

  .actions-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .action-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    background: var(--surface-2);
    border: 1px solid var(--border);
    color: var(--text-dim);
    padding: 8px 12px;
    border-radius: 6px;
    font-size: 12.5px;
    font-weight: 500;
    cursor: pointer;
    transition: background-color 0.16s ease, color 0.16s ease, border-color 0.16s ease;
  }

  .action-btn:hover {
    background: var(--surface-hover);
    color: var(--text);
    border-color: var(--border-strong);
  }

  .text-btn {
    background: transparent;
    border: 1px solid transparent;
    padding: 4px 8px;
    align-self: flex-start;
  }

  .text-btn:hover {
    background: var(--surface-2);
    border-color: var(--border);
  }

  .track-info {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .track-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .track-artist {
    font-size: 12.5px;
    color: var(--text-dim);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .track-source {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 11px;
    color: var(--text-faint);
    margin-top: 4px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .version-badge {
    font-size: 11px;
    font-weight: 500;
    color: var(--text-dim);
    background: var(--surface-2);
    padding: 2px 6px;
    border-radius: 4px;
    vertical-align: middle;
    margin-left: 6px;
    border: 1px solid var(--border);
  }
</style>
