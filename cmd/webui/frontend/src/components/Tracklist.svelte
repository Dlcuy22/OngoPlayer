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
  //   reorder      -> detail: { from, to } index reordering
  //   removeTrack  -> detail: index to remove

  import { createEventDispatcher } from "svelte";
  import Icon from "./Icon.svelte";
  import ContextMenu from "./ContextMenu.svelte";
  import { getCover, loadingIndex } from "../lib/playerStore.js";

  export let queue = [];
  export let currentIndex = -1;
  export let isPlaying = false;

  const dispatch = createEventDispatcher();

  // Drag and drop state
  let draggedIndex = null;
  let dragOverIndex = null;
  let scrollContainer;

  // Context menu state
  let showMenu = false;
  let menuX = 0;
  let menuY = 0;
  let contextTrackIndex = null;

  function displayTitle(track) {
    if (!track) return "Unknown";
    if (track.title) return track.title;
    const name = track.name || "";
    const dot = name.lastIndexOf(".");
    return dot > 0 ? name.slice(0, dot) : name || "Unknown";
  }

  // Resolve a thumbnail for a row; returns a Promise<string> ("" when none).
  function thumbFor(track) {
    if (track.coverURL) {
      return Promise.resolve(track.coverURL);
    }
    return getCover(track.index, track.hasCover);
  }

  // Drag handlers
  function handleDragStart(e, index) {
    draggedIndex = index;
    e.dataTransfer.effectAllowed = "move";
    e.dataTransfer.setData("text/plain", index);
  }

  function handleDragOver(e, index) {
    e.preventDefault();
    dragOverIndex = index;
  }

  function handleDragLeave(e, index) {
    if (dragOverIndex === index) {
      dragOverIndex = null;
    }
  }

  function handleDrop(e, index) {
    e.preventDefault();
    e.stopPropagation();
    dragOverIndex = null;

    const rawData = e.dataTransfer.getData("application/json");
    if (rawData) {
      try {
        const data = JSON.parse(rawData);
        if (data.type === "ytm-track") {
          dispatch("insertYTMTrack", { index, track: data.track });
          return;
        }
      } catch (err) {
        // ignore JSON parse errors
      }
    }

    if (draggedIndex === null) {
      return;
    }

    if (draggedIndex === index) {
      draggedIndex = null;
      return;
    }
    dispatch("reorder", { from: draggedIndex, to: index });
    draggedIndex = null;
  }

  function handleDragEnd() {
    draggedIndex = null;
    dragOverIndex = null;
  }

  function handleContainerDragOver(e) {
    if (draggedIndex === null || !scrollContainer) return;
    const rect = scrollContainer.getBoundingClientRect();
    const threshold = 40; // distance from top/bottom to start scrolling
    const speed = 10;

    const mouseY = e.clientY;
    if (mouseY < rect.top + threshold) {
      scrollContainer.scrollTop -= speed;
    } else if (mouseY > rect.bottom - threshold) {
      scrollContainer.scrollTop += speed;
    }
  }

  // Context Menu handlers
  function handleContextMenu(e, index) {
    e.preventDefault();
    contextTrackIndex = index;
    menuX = e.clientX;
    menuY = e.clientY;
    showMenu = true;
  }

  function menuPlayTrack() {
    if (contextTrackIndex !== null) {
      dispatch("playTrack", contextTrackIndex);
    }
  }

  function menuRemoveTrack() {
    if (contextTrackIndex !== null) {
      dispatch("removeTrack", contextTrackIndex);
    }
  }
</script>

<div class="tracklist">
  <div class="tl-header">
    <div class="tl-title">
      <Icon name="list-music" size={15} />
      <span>Queue</span>
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

  <div
    class="tl-scroll"
    bind:this={scrollContainer}
    on:dragover={handleContainerDragOver}
    on:dragover|preventDefault
    on:drop={(e) => handleDrop(e, queue.length)}
  >
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
          class:dragging={i === draggedIndex}
          class:drag-over-top={dragOverIndex === i && draggedIndex !== null && i < draggedIndex}
          class:drag-over-bottom={dragOverIndex === i && draggedIndex !== null && i > draggedIndex}
          class:loading={i === $loadingIndex}
          disabled={i === $loadingIndex}
          draggable={i !== $loadingIndex}
          on:dragstart={(e) => handleDragStart(e, i)}
          on:dragover={(e) => handleDragOver(e, i)}
          on:dragleave={(e) => handleDragLeave(e, i)}
          on:drop={(e) => handleDrop(e, i)}
          on:dragend={handleDragEnd}
          on:click={() => dispatch("playTrack", i)}
          on:contextmenu={(e) => handleContextMenu(e, i)}
          title={displayTitle(track)}
        >
          <!-- Grip Icon for reordering handle preview -->
          <div class="drag-grip">
            <Icon name="grip-vertical" size={12} strokeWidth={1.5} />
          </div>

          <div class="thumb">
            {#if i === $loadingIndex}
              <div class="dot-flashing">
                <span></span><span></span><span></span>
              </div>
            {:else}
              {#await thumbFor(track) then url}
                {#if url}
                  <img src={url} alt="" />
                {:else}
                  <span class="thumb-fallback">
                    <Icon name="music" size={16} strokeWidth={1.5} />
                  </span>
                {/if}
              {/await}
            {/if}

            {#if i === currentIndex && isPlaying && i !== $loadingIndex}
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

{#if showMenu}
  <ContextMenu
    x={menuX}
    y={menuY}
    items={[
      { label: "Play Track", icon: "play", action: menuPlayTrack },
      { label: "Remove from Queue", icon: "trash-2", action: menuRemoveTrack }
    ]}
    onClose={() => {
      showMenu = false;
    }}
  />
{/if}

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
    justify-content: center;
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
    gap: 10px;
    width: 100%;
    padding: 7px 10px;
    background: transparent;
    border: none;
    border-radius: 8px;
    color: var(--text-dim);
    text-align: left;
    cursor: pointer;
    position: relative;
    transition: background 0.15s ease, color 0.15s ease;
  }

  .track-row:disabled {
    opacity: 0.5;
    cursor: not-allowed;
    background: transparent !important;
    color: var(--text-faint) !important;
  }

  .track-row:hover {
    background: var(--surface-2);
    color: var(--text);
  }

  .track-row.active {
    background: var(--surface-3);
    color: var(--text);
  }

  .track-row.dragging {
    opacity: 0.4;
  }

  /* Dynamic reorder insertion visual feedback lines */
  .track-row.drag-over-top::before {
    content: "";
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    height: 2px;
    background-color: var(--text);
    z-index: 10;
  }

  .track-row.drag-over-bottom::after {
    content: "";
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
    height: 2px;
    background-color: var(--text);
    z-index: 10;
  }

  .drag-grip {
    color: var(--text-faint);
    opacity: 0;
    transition: opacity 0.15s ease;
    cursor: grab;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .track-row:hover .drag-grip {
    opacity: 1;
  }

  .thumb {
    position: relative;
    width: 36px;
    height: 36px;
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
