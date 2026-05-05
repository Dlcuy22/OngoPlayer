<script>
  import { onMount } from 'svelte';
  import {
    PlayFile, PlayTrack, Pause, Resume, Seek,
    SetVolume, GetVolume, Next, Prev,
    PickFolder, GetCurrentTrack
  } from '../wailsjs/go/main/App.js';
  import { EventsOn } from '../wailsjs/runtime/runtime.js';

  let filePath = '';
  let position = 0;
  let duration = 0;
  let isPlaying = false;
  let isSeeking = false;
  let volume = 100;
  let currentTrack = null;
  let queue = [];

  onMount(() => {
    document.addEventListener('contextmenu', event => event.preventDefault());

    GetVolume().then(v => volume = v);

    EventsOn("playback_progress", (data) => {
      if (!isSeeking) {
        position = data.position;
        duration = data.duration;
      } else {
        duration = data.duration;
      }
    });

    EventsOn("track_completed", () => {
      isPlaying = false;
      position = 0;
    });

    EventsOn("track_changed", (track) => {
      if (track) {
        currentTrack = track;
        isPlaying = true;
        position = 0;
      }
    });
  });

  function handlePlayFile() {
    if (filePath) {
      PlayFile(filePath).then(() => {
        isPlaying = true;
        currentTrack = { name: filePath.split(/[/\\]/).pop(), path: filePath };
      }).catch(err => console.error("Play error:", err));
    }
  }

  function handleToggle() {
    if (isPlaying) {
      Pause().then(() => isPlaying = false);
    } else {
      Resume().then(() => isPlaying = true);
    }
  }

  function handleSeekStart() {
    isSeeking = true;
  }

  function handleSeekInput(e) {
    position = parseFloat(e.target.value);
  }

  function handleSeekEnd(e) {
    const seekTo = parseFloat(e.target.value);
    Seek(seekTo).then(() => {
      position = seekTo;
      isSeeking = false;
    }).catch(() => isSeeking = false);
  }

  function handleSeekForward() {
    const target = Math.min(position + 5, duration);
    Seek(target).then(() => position = target);
  }

  function handleSeekBackward() {
    const target = Math.max(position - 5, 0);
    Seek(target).then(() => position = target);
  }

  function handleVolumeChange(e) {
    volume = parseInt(e.target.value);
    SetVolume(volume);
  }

  function handleNext() {
    Next();
  }

  function handlePrev() {
    Prev();
  }

  function handlePickFolder() {
    PickFolder().then(tracks => {
      if (tracks && tracks.length > 0) {
        queue = tracks;
      }
    });
  }

  function handleQueueClick(index) {
    PlayTrack(index).then(() => isPlaying = true);
  }

  function formatTime(seconds) {
    const m = Math.floor(seconds / 60);
    const s = Math.floor(seconds % 60);
    return `${m}:${s.toString().padStart(2, '0')}`;
  }
</script>

<div class="titlebar">
  <span class="titlebar-text">OngoPlayer</span>
</div>

<main>
  <!-- File input for quick testing -->
  <div class="file-input-row">
    <input
      type="text"
      bind:value={filePath}
      placeholder="Path to audio file..."
      class="path-input"
      on:keydown={(e) => e.key === 'Enter' && handlePlayFile()}
    />
    <button class="btn btn-sm" on:click={handlePlayFile}>Play</button>
    <button class="btn btn-sm" on:click={handlePickFolder}>Folder</button>
  </div>

  <!-- Now Playing -->
  <div class="now-playing">
    <span class="track-name">{currentTrack ? currentTrack.name : 'No track loaded'}</span>
  </div>

  <!-- Seek bar -->
  <div class="seek-container">
    <span class="time">{formatTime(position)}</span>
    <input
      type="range"
      min="0"
      max={duration || 1}
      step="0.1"
      value={position}
      on:pointerdown={handleSeekStart}
      on:input={handleSeekInput}
      on:change={handleSeekEnd}
      class="seek-slider"
      disabled={duration === 0}
    />
    <span class="time">{formatTime(duration)}</span>
  </div>

  <!-- Transport controls -->
  <div class="transport">
    <button class="btn btn-ctrl" on:click={handlePrev} title="Previous">&#x23EE;</button>
    <button class="btn btn-ctrl" on:click={handleSeekBackward} title="-5s">&#x23EA;</button>
    <button class="btn btn-play" on:click={handleToggle} disabled={duration === 0 && !filePath}>
      {isPlaying ? '\u23F8' : '\u23F5'}
    </button>
    <button class="btn btn-ctrl" on:click={handleSeekForward} title="+5s">&#x23E9;</button>
    <button class="btn btn-ctrl" on:click={handleNext} title="Next">&#x23ED;</button>
  </div>

  <!-- Volume -->
  <div class="volume-row">
    <span class="volume-label">Vol</span>
    <input
      type="range"
      min="0"
      max="100"
      step="1"
      value={volume}
      on:input={handleVolumeChange}
      class="volume-slider"
    />
    <span class="volume-value">{volume}</span>
  </div>

  <!-- Queue list -->
  {#if queue.length > 0}
    <div class="queue-container">
      <div class="queue-header">Queue ({queue.length})</div>
      <div class="queue-list">
        {#each queue as track}
          <button
            class="queue-item"
            class:active={currentTrack && currentTrack.path === track.path}
            on:click={() => handleQueueClick(track.index)}
          >
            <span class="queue-index">{track.index + 1}</span>
            <span class="queue-name">{track.name}</span>
          </button>
        {/each}
      </div>
    </div>
  {/if}
</main>

<style>
  .titlebar {
    --wails-draggable: drag;
    height: 32px;
    background-color: #181825;
    width: 100%;
    display: flex;
    align-items: center;
    padding: 0 12px;
    box-sizing: border-box;
    flex-shrink: 0;
  }

  .titlebar-text {
    font-size: 12px;
    font-weight: 700;
    color: #888;
  }

  main {
    padding: 16px 20px;
    display: flex;
    flex-direction: column;
    align-items: center;
    flex-grow: 1;
    overflow: hidden;
  }

  /* File input row */
  .file-input-row {
    display: flex;
    gap: 8px;
    width: 100%;
    max-width: 500px;
  }

  .path-input {
    flex: 1;
    padding: 8px 10px;
    border-radius: 4px;
    border: 1px solid #3a3a4e;
    background: #2a2a3e;
    color: #e0e0e0;
    font-size: 12px;
    box-sizing: border-box;
  }

  /* Now Playing */
  .now-playing {
    margin: 20px 0 8px;
    text-align: center;
    width: 100%;
    max-width: 500px;
  }

  .track-name {
    font-size: 14px;
    font-weight: 700;
    color: #ccc;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    display: block;
  }

  /* Seek bar */
  .seek-container {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    max-width: 500px;
    margin: 8px 0;
  }

  .time {
    font-size: 11px;
    color: #666;
    min-width: 36px;
    text-align: center;
  }

  .seek-slider {
    flex: 1;
    cursor: pointer;
    accent-color: #888;
    height: 4px;
  }

  .seek-slider:disabled {
    cursor: not-allowed;
    opacity: 0.3;
  }

  /* Transport */
  .transport {
    display: flex;
    align-items: center;
    gap: 12px;
    margin: 12px 0;
  }

  .btn {
    border: 1px solid #3a3a4e;
    background: #2a2a3e;
    color: #e0e0e0;
    cursor: pointer;
    font-family: inherit;
    border-radius: 4px;
  }

  .btn:hover {
    background: #3a3a4e;
  }

  .btn:disabled {
    background: #1e1e2e;
    color: #444;
    cursor: not-allowed;
    border-color: #2a2a3e;
  }

  .btn-sm {
    padding: 8px 12px;
    font-size: 12px;
    font-weight: 600;
  }

  .btn-ctrl {
    width: 36px;
    height: 36px;
    font-size: 16px;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0;
  }

  .btn-play {
    width: 48px;
    height: 48px;
    border-radius: 50%;
    font-size: 22px;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0;
    border: 1px solid #3a3a4e;
    background: #2a2a3e;
    color: #e0e0e0;
    cursor: pointer;
  }

  .btn-play:hover {
    background: #3a3a4e;
  }

  .btn-play:disabled {
    background: #1e1e2e;
    color: #444;
    cursor: not-allowed;
  }

  /* Volume */
  .volume-row {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    max-width: 240px;
    margin: 4px 0;
  }

  .volume-label {
    font-size: 11px;
    color: #666;
  }

  .volume-slider {
    flex: 1;
    cursor: pointer;
    accent-color: #888;
    height: 4px;
  }

  .volume-value {
    font-size: 11px;
    color: #666;
    min-width: 24px;
    text-align: right;
  }

  /* Queue */
  .queue-container {
    margin-top: 16px;
    width: 100%;
    max-width: 500px;
    flex: 1;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  .queue-header {
    font-size: 12px;
    color: #666;
    padding: 6px 0;
    border-bottom: 1px solid #2a2a3e;
  }

  .queue-list {
    overflow-y: auto;
    flex: 1;
  }

  .queue-item {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 8px 10px;
    border: none;
    background: transparent;
    color: #aaa;
    cursor: pointer;
    text-align: left;
    font-size: 12px;
    font-family: inherit;
    box-sizing: border-box;
  }

  .queue-item:hover {
    background: #2a2a3e;
  }

  .queue-item.active {
    color: #e0e0e0;
    background: #2a2a3e;
  }

  .queue-index {
    color: #555;
    min-width: 20px;
  }

  .queue-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
