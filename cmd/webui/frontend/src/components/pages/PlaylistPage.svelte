<script>
  // PlaylistPage.svelte displays YouTube Music playlist/album details.
  //
  // Purpose: Renders playlist tracks and headers, allowing tracks to be played.
  //
  // Dependencies:
  //   - lib/playerStore.js: play actions

  import { playerPlayYTMSong } from "../../lib/playerStore.js";
  import Icon from "../Icon.svelte";

  export let playlistData = null; // { details, loading, error }

  $: details = playlistData ? playlistData.details : null;
  $: loading = playlistData ? playlistData.loading : false;
  $: error = playlistData ? playlistData.error : null;

  function formatImgUrl(provider) {
    if (!provider) return "";
    return provider.url_a
      ? provider.url_b
        ? provider.url_a + "180-h180" + provider.url_b
        : provider.url_a
      : "";
  }

  function formatDuration(ms) {
    if (!ms) return "";
    const mins = Math.floor(ms / 60000);
    const secs = Math.floor((ms % 60000) / 1000)
      .toString()
      .padStart(2, "0");
    return `${mins}:${secs}`;
  }

  function getArtistNames(track) {
    if (track.artists && Array.isArray(track.artists)) {
      return track.artists.map((a) => a.name).join(", ");
    }
    return track.artist || "";
  }

  function getPlaylistArtists() {
    if (details && details.artists && Array.isArray(details.artists)) {
      return details.artists.map((a) => a.name).join(", ");
    }
    return "";
  }
</script>

<div class="playlist-page">
  {#if loading}
    <div class="status-box">
      <span class="spinner"></span>
      <p>Loading playlist/album tracks...</p>
    </div>
  {:else if error}
    <div class="status-box error">
      <Icon name="x" size={24} />
      <p>Error: {error}</p>
    </div>
  {:else if details}
    <div class="playlist-header">
      <img
        src={formatImgUrl(details.thumbnail)}
        alt={details.name}
        class="playlist-cover"
        on:error={(e) => (e.target.style.display = "none")}
      />
      <div class="playlist-meta">
        <span class="playlist-type">{details.type || "Playlist"}</span>
        <h2>{details.name}</h2>
        <div class="playlist-sub">
          {#if getPlaylistArtists()}
            <span class="playlist-author">By {getPlaylistArtists()}</span>
            <span class="separator">•</span>
          {/if}
          {#if details.item_count}
            <span class="track-count">{details.item_count} tracks</span>
          {/if}
        </div>
      </div>
    </div>

    {#if details.description}
      <div class="playlist-desc">
        <p>{details.description}</p>
      </div>
    {/if}

    <div class="tracks-list">
      {#if details.items && details.items.length > 0}
        <div class="list-header">
          <span class="col-num">#</span>
          <span class="col-title">Title</span>
          <span class="col-artist">Artist</span>
          <span class="col-duration">
            <Icon name="clock" size={12} strokeWidth={1.5} />
          </span>
        </div>
        <div class="tracks-rows">
          {#each details.items as track, index}
            <!-- svelte-ignore a11y-click-events-have-key-events -->
            <!-- svelte-ignore a11y-no-static-element-interactions -->
            <div
              class="track-row"
              on:click={() =>
                playerPlayYTMSong(
                  track.id,
                  track.name,
                  getArtistNames(track),
                  details.name,
                  track.lyrics_browse_id,
                  formatImgUrl(track.thumbnail)
                )}
            >
              <span class="col-num">{index + 1}</span>
              <span class="col-title">{track.name}</span>
              <span class="col-artist">{getArtistNames(track) || "Unknown"}</span>
              <span class="col-duration">{formatDuration(track.duration_ms)}</span>
            </div>
          {/each}
        </div>
      {:else}
        <div class="status-box empty">
          <p>No tracks in this playlist.</p>
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .playlist-page {
    padding: 24px;
    flex: 1;
    overflow-y: auto;
    background-color: var(--bg);
    display: flex;
    flex-direction: column;
    gap: 24px;
  }

  .playlist-header {
    display: flex;
    align-items: flex-end;
    gap: 20px;
  }

  .playlist-cover {
    width: 100px;
    height: 100px;
    border-radius: 6px;
    background-color: var(--surface-1);
    border: 1px solid var(--border);
    object-fit: cover;
  }

  .playlist-meta {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .playlist-type {
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-faint);
  }

  .playlist-meta h2 {
    font-size: 20px;
    font-weight: 700;
    margin: 0;
    color: var(--text);
  }

  .playlist-sub {
    font-size: 12.5px;
    color: var(--text-dim);
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .separator {
    color: var(--text-faint);
  }

  .playlist-desc {
    font-size: 12.5px;
    color: var(--text-dim);
    line-height: 1.5;
    background: var(--surface-1);
    border: 1px solid var(--border);
    padding: 12px 16px;
    border-radius: 6px;
  }

  .playlist-desc p {
    margin: 0;
  }

  .tracks-list {
    display: flex;
    flex-direction: column;
  }

  .list-header {
    display: grid;
    grid-template-columns: 40px 2fr 1fr 60px;
    padding: 8px 12px;
    border-bottom: 1px solid var(--border);
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-faint);
  }

  .col-duration {
    display: flex;
    justify-content: flex-end;
    align-items: center;
  }

  .tracks-rows {
    display: flex;
    flex-direction: column;
    margin-top: 4px;
  }

  .track-row {
    display: grid;
    grid-template-columns: 40px 2fr 1fr 60px;
    padding: 10px 12px;
    border-radius: 6px;
    font-size: 13px;
    cursor: pointer;
    align-items: center;
    transition: background-color 0.12s ease;
  }

  .track-row:hover {
    background: var(--surface-hover);
  }

  .col-num {
    color: var(--text-faint);
  }

  .col-title {
    color: var(--text);
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    padding-right: 12px;
  }

  .col-artist {
    color: var(--text-dim);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    padding-right: 12px;
  }

  .col-duration {
    text-align: right;
    color: var(--text-dim);
  }

  .status-box {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    min-height: 200px;
    color: var(--text-dim);
  }

  .spinner {
    width: 24px;
    height: 24px;
    border: 2px solid var(--border);
    border-top-color: var(--text);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  .status-box.error {
    color: #ef4444;
  }
</style>
