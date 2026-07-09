<script>
  // ArtistPage.svelte displays YouTube Music artist details.
  //
  // Purpose: Renders artist metadata and layout shelves (popular songs, albums).
  //
  // Dependencies:
  //   - lib/playerStore.js: player actions, navigation helpers
  //   - wailsjs/go/main/App.js: GetYTMPlaylist, PlayYTMSong

  import { playerPlayYTMSong, navigateTo } from "../../lib/playerStore.js";
  import { GetYTMPlaylist } from "../../../wailsjs/go/main/App.js";
  import Icon from "../Icon.svelte";

  export let artistData = null; // { details, loading, error }

  $: details = artistData ? artistData.details : null;
  $: loading = artistData ? artistData.loading : false;
  $: error = artistData ? artistData.error : null;

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

  async function handleLoadPlaylist(playlistID) {
    navigateTo("playlist", { loading: true });
    try {
      const data = await GetYTMPlaylist(playlistID);
      navigateTo("playlist", { details: data, loading: false });
    } catch (err) {
      console.error("load playlist failed:", err);
      navigateTo("playlist", { error: err.toString(), loading: false });
    }
  }

  function isSong(item) {
    // Items with duration or in Popular Songs shelf are typically songs
    return item.type === "SONG" || item.type === "VIDEO" || item.duration_ms !== undefined || item.lyrics_browse_id !== undefined;
  }
</script>

<div class="artist-page">
  {#if loading}
    <div class="status-box">
      <span class="spinner"></span>
      <p>Loading artist details...</p>
    </div>
  {:else if error}
    <div class="status-box error">
      <Icon name="x" size={24} />
      <p>Error: {error}</p>
    </div>
  {:else if details}
    <div class="artist-header">
      <img
        src={formatImgUrl(details.thumbnail)}
        alt={details.name}
        class="artist-avatar"
        on:error={(e) => (e.target.style.display = "none")}
      />
      <div class="artist-meta">
        <h2>{details.name}</h2>
        {#if details.subscriber_count}
          <span class="subs-count">
            {details.subscriber_count.toLocaleString()} subscribers
          </span>
        {/if}
      </div>
    </div>

    {#if details.description}
      <div class="artist-bio">
        <p>{details.description}</p>
      </div>
    {/if}

    <div class="artist-content">
      {#if details.layouts && details.layouts.length > 0}
        {#each details.layouts as layout}
          {#if layout.items && layout.items.length > 0}
            <div class="shelf-section">
              <h3>{layout.title || "Releases"}</h3>
              <div class="shelf-grid">
                {#each layout.items as item}
                  {#if isSong(item)}
                    <!-- svelte-ignore a11y-click-events-have-key-events -->
                    <!-- svelte-ignore a11y-no-static-element-interactions -->
                    <div
                      class="shelf-item song-card"
                      on:click={() =>
                        playerPlayYTMSong(
                          item.id,
                          item.name,
                          details.name,
                          "",
                          item.lyrics_browse_id,
                          formatImgUrl(item.thumbnail)
                        )}
                    >
                      <img
                        src={formatImgUrl(item.thumbnail)}
                        alt={item.name}
                        class="shelf-cover"
                        on:error={(e) => (e.target.style.display = "none")}
                      />
                      <div class="shelf-info">
                        <span class="shelf-title">{item.name}</span>
                        {#if item.duration_ms}
                          <span class="shelf-sub">{formatDuration(item.duration_ms)}</span>
                        {/if}
                      </div>
                    </div>
                  {:else}
                    <!-- svelte-ignore a11y-click-events-have-key-events -->
                    <!-- svelte-ignore a11y-no-static-element-interactions -->
                    <div
                      class="shelf-item album-card"
                      on:click={() => handleLoadPlaylist(item.id)}
                    >
                      <img
                        src={formatImgUrl(item.thumbnail)}
                        alt={item.name}
                        class="shelf-cover"
                        on:error={(e) => (e.target.style.display = "none")}
                      />
                      <div class="shelf-info">
                        <span class="shelf-title">{item.name}</span>
                        <span class="shelf-sub">Album</span>
                      </div>
                    </div>
                  {/if}
                {/each}
              </div>
            </div>
          {/if}
        {/each}
      {/if}
    </div>
  {/if}
</div>

<style>
  .artist-page {
    padding: 24px;
    flex: 1;
    overflow-y: auto;
    background-color: var(--bg);
    display: flex;
    flex-direction: column;
    gap: 24px;
  }

  .artist-header {
    display: flex;
    align-items: center;
    gap: 20px;
  }

  .artist-avatar {
    width: 80px;
    height: 80px;
    border-radius: 50%;
    background-color: var(--surface-1);
    border: 1px solid var(--border);
    object-fit: cover;
  }

  .artist-meta h2 {
    font-size: 20px;
    font-weight: 700;
    margin: 0 0 4px 0;
    color: var(--text);
  }

  .subs-count {
    font-size: 13px;
    color: var(--text-dim);
  }

  .artist-bio {
    font-size: 12.5px;
    line-height: 1.6;
    color: var(--text-dim);
    background: var(--surface-1);
    border: 1px solid var(--border);
    padding: 16px;
    border-radius: 6px;
    max-height: 120px;
    overflow-y: auto;
  }

  .artist-bio p {
    margin: 0;
  }

  .artist-content {
    display: flex;
    flex-direction: column;
    gap: 24px;
  }

  .shelf-section {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .shelf-section h3 {
    font-size: 12.5px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-faint);
    margin: 0;
    border-bottom: 1px solid var(--border);
    padding-bottom: 6px;
  }

  .shelf-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
    gap: 12px;
  }

  .shelf-item {
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 10px;
    cursor: pointer;
    display: flex;
    flex-direction: column;
    gap: 8px;
    transition: background-color 0.16s ease, border-color 0.16s ease;
  }

  .shelf-item:hover {
    background: var(--surface-hover);
    border-color: var(--border-strong);
  }

  .shelf-cover {
    width: 100%;
    aspect-ratio: 1;
    border-radius: 4px;
    background-color: var(--surface-2);
    object-fit: cover;
  }

  .shelf-info {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .shelf-title {
    font-size: 12.5px;
    font-weight: 500;
    color: var(--text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .shelf-sub {
    font-size: 11px;
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
