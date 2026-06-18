<script>
  // Tracklist.svelte renders the sidebar queue: a folder picker (replace) with
  // an append button beside it, and a scrollable list of tracks. Each row shows
  // a mini cover thumbnail (lazy loaded) left of the title; the active track is
  // marked with an equalizer-style indicator.
  //
  // Props:
  //   queue: array of TrackInfo
  //   currentIndex: index of the active track (-1 when none)
  //   isPlaying: whether the active track is currently playing
  //
  // Events:
  //   playTrack    -> detail: queue index to play
  //   pickFolder   -> request native folder picker (replace queue)
  //   appendFolder -> request native folder picker (append to queue)

  import { createEventDispatcher } from "svelte";
  import Icon from "./Icon.svelte";
  import { getCover } from "../lib/playerStore.js";

  export let queue = [];
  export let currentIndex = -1;
  export let isPlaying = false;

  const dispatch = createEventDispatcher();

  function displayTitle(track) {
    if (!track) return "Unknown";
    if (track.title) return track.title;
    const name = track.name || "";
    const dot = name.lastIndexOf(".");
    return dot > 0 ? name.slice(0, dot) : name || "Unknown";
  }

  // Resolve a thumbnail for a row; returns a Promise<string> ("" when none).
  function thumbFor(track) {
    return getCover(track.index, track.hasCover);
  }
</script>

<div class="tracklist">
  <div class="tl-header">
    <div class="tl-title">
      <Icon name="list-music" size={15} />
      <span>Library</span>
    </div>
    <span class="tl-count">{queue.length}</span>
  </div>

  <div class="tl-actions">
    <button class="pick-folder" on:click={() => dispatch("pickFolder")}>
      <Icon name="folder" size={15} />
      <span>Open folder</span>
    </button>
    <button
      class="append-folder"
      on:click={() => dispatch("appendFolder")}
      title="Add another folder"
    >
      <Icon name="folder-plus" size={16} />
    </button>
  </div>

  <div class="tl-scroll">
    {#if queue.length === 0}
      <div class="tl-empty">
        <Icon name="music" size={22} strokeWidth={1.5} />
        <span>No tracks loaded</span>
      </div>
    {:else}
      {#each queue as track, i (track.path + i)}
        <button
          class="track-row"
          class:active={i === currentIndex}
          on:click={() => dispatch("playTrack", i)}
          title={displayTitle(track)}
        >
          <div class="thumb">
            {#await thumbFor(track) then url}
              {#if url}
                <img src={url} alt="" />
              {:else}
                <span class="thumb-fallback">
                  <Icon name="music" size={16} strokeWidth={1.5} />
                </span>
              {/if}
            {/await}

            {#if i === currentIndex && isPlaying}
              <span class="eq" aria-hidden="true">
                <span></span><span></span><span></span>
              </span>
            {/if}
          </div>

          <div class="track-meta">
            <span class="track-title">{displayTitle(track)}</span>
            {#if track.artist}
              <span class="track-artist">{track.artist}</span>
            {/if}
          </div>
        </button>
      {/each}
    {/if}
  </div>
</div>

<style>
  .tracklist {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
  }

  .tl-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 16px 10px;
    flex-shrink: 0;
  }

  .tl-title {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--text-dim);
  }

  .tl-count {
    font-size: 11px;
    font-variant-numeric: tabular-nums;
    color: var(--text-faint);
    background: var(--surface-2);
    border-radius: 10px;
    padding: 1px 8px;
  }

  .tl-actions {
    display: flex;
    gap: 8px;
    margin: 0 12px 10px;
  }

  .pick-folder {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 9px 12px;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: 8px;
    color: var(--text);
    font-size: 12.5px;
    cursor: pointer;
    transition: background 0.16s ease, border-color 0.16s ease;
  }

  .pick-folder:hover {
    background: var(--surface-hover);
    border-color: var(--border-strong);
  }

  .append-folder {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 38px;
    flex-shrink: 0;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: 8px;
    color: var(--text-dim);
    cursor: pointer;
    transition: background 0.16s ease, border-color 0.16s ease, color 0.16s ease;
  }

  .append-folder:hover {
    background: var(--surface-hover);
    border-color: var(--border-strong);
    color: var(--text);
  }

  .tl-scroll {
    flex: 1;
    overflow-y: auto;
    padding: 0 8px 12px;
  }

  .tl-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 10px;
    margin-top: 48px;
    color: var(--text-faint);
    font-size: 12px;
  }

  .track-row {
    display: flex;
    align-items: center;
    gap: 12px;
    width: 100%;
    padding: 7px 10px;
    background: transparent;
    border: none;
    border-radius: 8px;
    color: var(--text-dim);
    text-align: left;
    cursor: pointer;
    transition: background 0.15s ease, color 0.15s ease;
  }

  .track-row:hover {
    background: var(--surface-2);
    color: var(--text);
  }

  .track-row.active {
    background: var(--surface-3);
    color: var(--text);
  }

  .thumb {
    position: relative;
    width: 40px;
    height: 40px;
    flex-shrink: 0;
    border-radius: 6px;
    overflow: hidden;
    background: var(--surface-3);
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .thumb img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }

  .thumb-fallback {
    color: var(--text-faint);
    display: flex;
  }

  .track-meta {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .track-title {
    font-size: 13px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .track-artist {
    font-size: 11px;
    color: var(--text-faint);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* Equalizer overlay on the active row's thumbnail. */
  .eq {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 2px;
    background: rgba(0, 0, 0, 0.5);
  }

  .eq span {
    width: 2px;
    height: 12px;
    background: var(--text);
    border-radius: 1px;
    animation: eq 0.9s ease-in-out infinite;
  }

  .eq span:nth-child(1) {
    animation-delay: -0.2s;
  }
  .eq span:nth-child(2) {
    animation-delay: -0.5s;
  }
  .eq span:nth-child(3) {
    animation-delay: -0.1s;
  }

  @keyframes eq {
    0%,
    100% {
      transform: scaleY(0.4);
    }
    50% {
      transform: scaleY(1);
    }
  }
</style>
