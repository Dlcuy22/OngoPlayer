<script>
  // SearchResultsPage.svelte displays YouTube Music and local search results.
  //
  // Purpose: Segregates and displays search matches. Displays clickable items
  // to play songs, navigate to artists, or load playlists/albums.
  //
  // Dependencies:
  //   - lib/playerStore.js: queue store, play actions, navigation helpers
  //   - wailsjs/go/main/App.js: GetYTMArtist, GetYTMPlaylist, PlayYTMSong

  import { queue, currentTrack, playerPlayYTMSong, playerInsertYTMSongAt, navigateTo } from "../../lib/playerStore.js";
  import { GetYTMArtist, GetYTMPlaylist } from "../../../wailsjs/go/main/App.js";
  import Icon from "../Icon.svelte";
  import ContextMenu from "../ContextMenu.svelte";

  export let searchData = null; // { query, results, loading, error }

  $: query = searchData ? searchData.query : "";
  $: results = searchData ? searchData.results : null;
  $: loading = searchData ? searchData.loading : false;
  $: error = searchData ? searchData.error : null;

  // Filter local queue matches
  $: localMatches = query
    ? $queue.filter(
        (t) =>
          (t.title && t.title.toLowerCase().includes(query.toLowerCase())) ||
          (t.artist && t.artist.toLowerCase().includes(query.toLowerCase())) ||
          (t.name && t.name.toLowerCase().includes(query.toLowerCase()))
      )
    : [];

  function formatImgUrl(provider) {
    if (!provider) return "";
    return provider.url_a
      ? provider.url_b
        ? provider.url_a + "120-h120" + provider.url_b
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

  async function handleLoadArtist(artistID) {
    navigateTo("artist", { loading: true });
    try {
      const data = await GetYTMArtist(artistID);
      navigateTo("artist", { details: data, loading: false });
    } catch (err) {
      console.error("load artist failed:", err);
      navigateTo("artist", { error: err.toString(), loading: false });
    }
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

  function getArtistNames(item) {
    if (item.artists && Array.isArray(item.artists)) {
      return item.artists.map((a) => a.name).join(", ");
    }
    return item.artist || "Unknown Artist";
  }

  function getAlbumName(item) {
    return item.album && item.album.name ? item.album.name : "";
  }

  // Drag and drop handlers
  function handleDragStart(e, item) {
    const trackData = {
      type: "ytm-track",
      track: {
        id: item.id,
        name: item.name,
        artist: getArtistNames(item),
        album: getAlbumName(item),
        lyrics_browse_id: item.lyrics_browse_id,
        thumbnail: formatImgUrl(item.thumbnail),
      }
    };
    e.dataTransfer.setData("application/json", JSON.stringify(trackData));
    e.dataTransfer.effectAllowed = "copyMove";
  }

  // Context Menu State
  let showMenu = false;
  let menuX = 0;
  let menuY = 0;
  let contextItem = null;

  function handleContextMenu(e, item) {
    e.preventDefault();
    contextItem = item;
    menuX = e.clientX;
    menuY = e.clientY;
    showMenu = true;
  }

  function menuAddToNext() {
    if (!contextItem) return;
    const curIndex = $currentTrack && typeof $currentTrack.index === "number" ? $currentTrack.index : -1;
    const insertIndex = curIndex >= 0 ? curIndex + 1 : $queue.length;
    playerInsertYTMSongAt(
      insertIndex,
      contextItem.id,
      contextItem.name,
      getArtistNames(contextItem),
      getAlbumName(contextItem),
      contextItem.lyrics_browse_id,
      formatImgUrl(contextItem.thumbnail)
    );
    showMenu = false;
  }

  function menuAddToQueue() {
    if (!contextItem) return;
    playerInsertYTMSongAt(
      $queue.length,
      contextItem.id,
      contextItem.name,
      getArtistNames(contextItem),
      getAlbumName(contextItem),
      contextItem.lyrics_browse_id,
      formatImgUrl(contextItem.thumbnail)
    );
    showMenu = false;
  }
</script>

<div class="search-results-page">
  {#if loading}
    <div class="status-box">
      <span class="spinner"></span>
      <p>Searching YouTube Music...</p>
    </div>
  {:else if error}
    <div class="status-box error">
      <Icon name="x" size={24} />
      <p>Search failed: {error}</p>
    </div>
  {:else}
    <div class="search-header">
      <h2>Search Results for "{query}"</h2>
    </div>

    <div class="results-content">
      <!-- Local library matches -->
      {#if localMatches.length > 0}
        <section class="result-section">
          <h3>Local Library Matches</h3>
          <div class="matches-list">
            {#each localMatches as track}
              <!-- svelte-ignore a11y-click-events-have-key-events -->
              <!-- svelte-ignore a11y-no-static-element-interactions -->
              <div
                class="result-row local-row"
                on:click={() => navigateTo("home", null, true)}
              >
                <div class="row-left">
                  <Icon name="music" size={14} strokeWidth={1.5} />
                  <div class="row-meta">
                    <span class="track-title">{track.title || track.name}</span>
                    <span class="track-artist">{track.artist || "Local"}</span>
                  </div>
                </div>
                <div class="row-right">
                  <span class="badge">Local</span>
                </div>
              </div>
            {/each}
          </div>
        </section>
      {/if}

      <!-- YouTube Music Categories -->
      {#if results && results.categories && results.categories.length > 0}
        {#each results.categories as cat}
          {#if cat.layout && cat.layout.items && cat.layout.items.length > 0}
            <section class="result-section">
              <h3>{cat.layout.title || "YouTube Music"}</h3>
              <div class="matches-list">
                {#each cat.layout.items as item}
                  {#if item.artists && Array.isArray(item.artists)}
                    <!-- Song row -->
                    <!-- svelte-ignore a11y-click-events-have-key-events -->
                    <!-- svelte-ignore a11y-no-static-element-interactions -->
                    <div
                      class="result-row song-row"
                      draggable="true"
                      on:dragstart={(e) => handleDragStart(e, item)}
                      on:contextmenu|preventDefault={(e) => handleContextMenu(e, item)}
                    >
                      <div class="row-left">
                        <div
                          class="cover-container"
                          on:click|stopPropagation={() =>
                            playerPlayYTMSong(
                              item.id,
                              item.name,
                              getArtistNames(item),
                              getAlbumName(item),
                              item.lyrics_browse_id,
                              formatImgUrl(item.thumbnail)
                            )}
                        >
                          <img
                            src={formatImgUrl(item.thumbnail)}
                            alt={item.name}
                            class="track-cover"
                            on:error={(e) => (e.target.style.display = "none")}
                          />
                          <div class="cover-overlay">
                            <Icon name="play" size={16} />
                          </div>
                        </div>
                        <div class="row-meta">
                          <span class="track-title">{item.name}</span>
                          <span class="track-artist">{getArtistNames(item)}</span>
                        </div>
                      </div>
                      <div class="row-right">
                        {#if item.is_explicit}
                          <span class="badge explicit">E</span>
                        {/if}
                        <span class="track-duration">{formatDuration(item.duration_ms)}</span>
                      </div>
                    </div>
                  {:else if item.subscriber_count !== undefined}
                    <!-- Artist row -->
                    <!-- svelte-ignore a11y-click-events-have-key-events -->
                    <!-- svelte-ignore a11y-no-static-element-interactions -->
                    <div
                      class="result-row artist-row"
                      on:click={() => handleLoadArtist(item.id)}
                    >
                      <div class="row-left">
                        <img
                          src={formatImgUrl(item.thumbnail)}
                          alt={item.name}
                          class="track-cover round"
                          on:error={(e) => (e.target.style.display = "none")}
                        />
                        <div class="row-meta">
                          <span class="track-title">{item.name}</span>
                          <span class="track-artist">Artist • {item.subscriber_count.toLocaleString()} subscribers</span>
                        </div>
                      </div>
                      <div class="row-right">
                        <Icon name="chevron-right" size={14} />
                      </div>
                    </div>
                  {:else}
                    <!-- Playlist/Album row -->
                    <!-- svelte-ignore a11y-click-events-have-key-events -->
                    <!-- svelte-ignore a11y-no-static-element-interactions -->
                    <div
                      class="result-row playlist-row"
                      on:click={() => handleLoadPlaylist(item.id)}
                    >
                      <div class="row-left">
                        <img
                          src={formatImgUrl(item.thumbnail)}
                          alt={item.name}
                          class="track-cover"
                          on:error={(e) => (e.target.style.display = "none")}
                        />
                        <div class="row-meta">
                          <span class="track-title">{item.name}</span>
                          <span class="track-artist">
                            {item.item_count !== undefined ? `Playlist • ${item.item_count} tracks` : "Album"}
                          </span>
                        </div>
                      </div>
                      <div class="row-right">
                        <Icon name="chevron-right" size={14} />
                      </div>
                    </div>
                  {/if}
                {/each}
              </div>
            </section>
          {/if}
        {/each}
      {:else if !localMatches.length}
        <div class="status-box empty">
          <p>No results found for "{query}".</p>
        </div>
      {/if}
    </div>
  {/if}
</div>

{#if showMenu}
  <ContextMenu
    x={menuX}
    y={menuY}
    items={[
      { label: "Add to Next", icon: "skip-forward", action: menuAddToNext },
      { label: "Add to Queue", icon: "list-music", action: menuAddToQueue }
    ]}
    onClose={() => {
      showMenu = false;
    }}
  />
{/if}

<style>
  .search-results-page {
    padding: 24px;
    flex: 1;
    overflow-y: auto;
    background-color: var(--bg);
    display: flex;
    flex-direction: column;
    gap: 24px;
  }

  .search-header h2 {
    font-size: 18px;
    font-weight: 600;
    margin: 0;
    color: var(--text);
  }

  .results-content {
    display: flex;
    flex-direction: column;
    gap: 24px;
  }

  .result-section {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .result-section h3 {
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-faint);
    margin: 0;
    border-bottom: 1px solid var(--border);
    padding-bottom: 6px;
  }

  .matches-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .result-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: 6px;
    cursor: pointer;
    transition: background-color 0.16s ease, border-color 0.16s ease;
  }

  .result-row:hover {
    background: var(--surface-hover);
    border-color: var(--border-strong);
  }

  .row-left {
    display: flex;
    align-items: center;
    gap: 12px;
    min-width: 0;
  }

  .track-cover {
    width: 36px;
    height: 36px;
    border-radius: 4px;
    background-color: var(--surface-2);
    object-fit: cover;
  }

  .track-cover.round {
    border-radius: 50%;
  }

  .cover-container {
    position: relative;
    width: 36px;
    height: 36px;
    border-radius: 4px;
    overflow: hidden;
    flex-shrink: 0;
    cursor: pointer;
  }

  .cover-container .track-cover {
    width: 100%;
    height: 100%;
  }

  .cover-overlay {
    position: absolute;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    color: #ffffff;
    opacity: 0;
    transition: opacity 0.15s ease;
  }

  .cover-container:hover .cover-overlay {
    opacity: 1;
  }

  .row-meta {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .track-title {
    font-size: 13.5px;
    font-weight: 500;
    color: var(--text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .track-artist {
    font-size: 12px;
    color: var(--text-dim);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .row-right {
    display: flex;
    align-items: center;
    gap: 10px;
    color: var(--text-dim);
  }

  .track-duration {
    font-size: 12px;
  }

  .badge {
    font-size: 10px;
    padding: 2px 6px;
    background: var(--surface-3);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-dim);
    font-weight: 500;
  }

  .badge.explicit {
    background: #2a1b1b;
    border-color: #5a2e2e;
    color: #ef4444;
    font-weight: 700;
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
