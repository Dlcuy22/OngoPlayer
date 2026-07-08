<script>
  import { queue, currentTrack, playerPlayYTMSong, playerInsertYTMSongAt, navigateTo } from "../../lib/playerStore.js";
  import { GetYTMArtist, GetYTMPlaylist, SearchYTMMore, SearchYTMViewMore, SearchYTM } from "../../../wailsjs/go/main/App.js";
  import Icon from "../Icon.svelte";
  import ContextMenu from "../ContextMenu.svelte";

  export let searchData = null;

  $: query = searchData ? searchData.query : "";
  $: results = searchData ? searchData.results : null;
  $: loading = searchData ? searchData.loading : false;
  $: error = searchData ? searchData.error : null;

  let activeChipIndex = -1;
  $: chips = results?.chips || [];

  $: activeChip = activeChipIndex >= 0 && activeChipIndex < chips.length
    ? chips[activeChipIndex]
    : null;

  $: activeParams = activeChip ? activeChip.params : "";
  $: isAllView = !activeParams;

  async function handleChipClick(index) {
    if (index === activeChipIndex) return;
    activeChipIndex = index;
    const chip = index >= 0 && index < chips.length ? chips[index] : null;
    const params = chip ? chip.params : "";
    if (params === "" && !results?.chips) return;
    searchData = { query, loading: true, results: null, error: null };
    try {
      const newResults = await SearchYTM(query, params);
      searchData = { query, results: newResults, loading: false, error: null };
    } catch (err) {
      searchData = { query, error: err.toString(), loading: false, results: null };
    }
  }

  function getItemType(item) {
    if (item.artists && Array.isArray(item.artists)) {
      return item.type === "VIDEO" ? "video" : "song";
    }
    if (item.subscriber_count !== undefined) return "artist";
    if (item.type === "ALBUM") return "album";
    return "playlist";
  }

  const groupLabels = {
    song: "Songs",
    video: "Videos",
    artist: "Artists",
    album: "Albums",
    playlist: "Playlists",
  };

  $: groupedResults = results && isAllView && results.categories
    ? buildGroups(results.categories)
    : [];

  function buildGroups(categories) {
    const groups = { song: [], video: [], artist: [], album: [], playlist: [] };
    for (const cat of categories) {
      if (!cat.layout?.items) continue;
      for (const item of cat.layout.items) {
        const t = getItemType(item);
        if (groups[t]) groups[t].push(item);
      }
    }
    return Object.entries(groups)
      .filter(([, items]) => items.length > 0)
      .map(([type, items]) => ({ type, label: groupLabels[type], items }));
  }

  $: filteredCat = results && !isAllView && results.categories
    ? results.categories.find(c => c.layout?.items?.length > 0)
    : null;

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
      navigateTo("artist", { error: err.toString(), loading: false });
    }
  }

  async function handleLoadPlaylist(playlistID) {
    navigateTo("playlist", { loading: true });
    try {
      const data = await GetYTMPlaylist(playlistID);
      navigateTo("playlist", { details: data, loading: false });
    } catch (err) {
      navigateTo("playlist", { error: err.toString(), loading: false });
    }
  }

  function getArtistNames(item) {
    if (item.artists && Array.isArray(item.artists)) {
      return item.artists.map((a) => a.name).filter(Boolean).join(", ");
    }
    return item.artist || "";
  }

  function getAlbumName(item) {
    return item.album && item.album.name ? item.album.name : "";
  }

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

  async function loadMore(cat) {
    try {
      if (cat.layout?.viewMore) {
        const items = await SearchYTMViewMore(cat.layout.viewMore.browseID);
        cat.layout.items = items;
        cat.layout.viewMore = null;
      } else if (cat.continuation) {
        const result = await SearchYTMMore(cat.continuation);
        cat.layout.items = [...cat.layout.items, ...result.items];
        cat.continuation = result.nextToken || "";
      }
      searchData = searchData;
    } catch (err) {
      console.error("load more failed:", err);
    }
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
      <h2>Results for "{query}"</h2>
    </div>

    {#if chips.length > 0}
      <div class="chip-bar">
        <button
          class="chip"
          class:chip-active={activeChipIndex === -1}
          on:click={() => handleChipClick(-1)}>All</button>
        {#each chips as chip, i}
          <button
            class="chip"
            class:chip-active={i === activeChipIndex}
            on:click={() => handleChipClick(i)}>{chip.name}</button>
        {/each}
      </div>
    {/if}

    <div class="results-content">
      {#if localMatches.length > 0}
        <section class="result-section">
          <h3>Local Library</h3>
          <div class="matches-list">
            {#each localMatches as track}
              <div class="result-row local-row" on:click={() => navigateTo("home", null, true)}>
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

      <!-- "All" view: grouped by type -->
      {#if isAllView}
        {#each groupedResults as group}
          {#if group.items.length > 0}
            <section class="result-section">
              <h3>{group.label}</h3>
              <div class="matches-list">
                {#each group.items as item}
                  {#if item.artists && Array.isArray(item.artists)}
                    <!-- Song/Video row -->
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
                              item.id, item.name,
                              getArtistNames(item), getAlbumName(item),
                              item.lyrics_browse_id, formatImgUrl(item.thumbnail)
                            )}
                        >
                          <img
                            src={formatImgUrl(item.thumbnail)}
                            alt={item.name}
                            loading="lazy"
                            class="track-cover"
                            on:error={(e) => (e.target.style.display = "none")}
                          />
                          <div class="cover-overlay">
                            <Icon name="play" size={16} />
                          </div>
                        </div>
                        <div class="row-meta">
                          <span class="track-title">{item.name}</span>
                          <span class="track-artist">{getArtistNames(item) || (item.type === "VIDEO" ? "Video" : "Song")}</span>
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
                    <div
                      class="result-row artist-row"
                      on:click={() => handleLoadArtist(item.id)}
                    >
                      <div class="row-left">
                        <img
                          src={formatImgUrl(item.thumbnail)}
                          alt={item.name}
                          loading="lazy"
                          class="track-cover round"
                          on:error={(e) => (e.target.style.display = "none")}
                        />
                        <div class="row-meta">
                          <span class="track-title">{item.name}</span>
                          <span class="track-artist">Artist &bull; {item.subscriber_count.toLocaleString()} subscribers</span>
                        </div>
                      </div>
                      <div class="row-right">
                        <Icon name="chevron-right" size={14} />
                      </div>
                    </div>
                  {:else}
                    <!-- Playlist/Album row -->
                    <div
                      class="result-row playlist-row"
                      on:click={() => handleLoadPlaylist(item.id)}
                    >
                      <div class="row-left">
                        <img
                          src={formatImgUrl(item.thumbnail)}
                          alt={item.name}
                          loading="lazy"
                          class="track-cover"
                          on:error={(e) => (e.target.style.display = "none")}
                        />
                        <div class="row-meta">
                          <span class="track-title">{item.name}</span>
                          <span class="track-artist">
                            {item.type === "ALBUM" ? "Album" : `Playlist • ${item.item_count || "?"} tracks`}
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
      {:else if filteredCat}
        <!-- Filtered view: single category in grid -->
        <section class="result-section">
          <h3>{groupLabels[activeChip?.type?.toLowerCase()] || filteredCat.layout.title || activeChip?.name || "Results"}</h3>
          <div class="matches-list">
            {#each filteredCat.layout.items as item}
              {#if item.artists && Array.isArray(item.artists)}
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
                          item.id, item.name,
                          getArtistNames(item), getAlbumName(item),
                          item.lyrics_browse_id, formatImgUrl(item.thumbnail)
                        )}
                    >
                      <img src={formatImgUrl(item.thumbnail)} alt={item.name} loading="lazy" class="track-cover"
                        on:error={(e) => (e.target.style.display = "none")} />
                      <div class="cover-overlay">
                        <Icon name="play" size={16} />
                      </div>
                    </div>
                    <div class="row-meta">
                      <span class="track-title">{item.name}</span>
                      <span class="track-artist">{getArtistNames(item) || (item.type === "VIDEO" ? "Video" : "Song")}</span>
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
                <div class="result-row artist-row" on:click={() => handleLoadArtist(item.id)}>
                  <div class="row-left">
                    <img src={formatImgUrl(item.thumbnail)} alt={item.name} loading="lazy" class="track-cover round"
                      on:error={(e) => (e.target.style.display = "none")} />
                    <div class="row-meta">
                      <span class="track-title">{item.name}</span>
                      <span class="track-artist">Artist &bull; {item.subscriber_count.toLocaleString()} subscribers</span>
                    </div>
                  </div>
                  <div class="row-right">
                    <Icon name="chevron-right" size={14} />
                  </div>
                </div>
              {:else}
                <div class="result-row playlist-row" on:click={() => handleLoadPlaylist(item.id)}>
                  <div class="row-left">
                    <img src={formatImgUrl(item.thumbnail)} alt={item.name} loading="lazy" class="track-cover"
                      on:error={(e) => (e.target.style.display = "none")} />
                    <div class="row-meta">
                      <span class="track-title">{item.name}</span>
                      <span class="track-artist">
                        {item.type === "ALBUM" ? "Album" : `Playlist • ${item.item_count || "?"} tracks`}
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
          {#if filteredCat.layout?.viewMore || filteredCat.continuation}
            <button class="load-more" on:click={() => loadMore(filteredCat)}>
              Show more
            </button>
          {/if}
        </section>
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
    onClose={() => { showMenu = false; }}
  />
{/if}

<style>
  .search-results-page {
    padding: 24px;
    flex: 1;
    overflow-y: auto;
    background-color: var(--bg);
  }

  .search-header h2 {
    font-size: 18px;
    font-weight: 600;
    margin: 0 0 12px;
    color: var(--text);
  }

  .chip-bar {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-bottom: 16px;
  }

  .chip {
    padding: 4px 14px;
    border-radius: 14px;
    border: 1px solid var(--border);
    background: var(--surface-1);
    color: var(--text-dim);
    font-size: 12px;
    cursor: pointer;
    transition: background 0.12s, color 0.12s, border-color 0.12s;
    white-space: nowrap;
  }

  .chip:hover {
    background: var(--surface-hover);
    color: var(--text);
    border-color: var(--border-strong);
  }

  .chip-active {
    background: var(--text);
    color: var(--bg);
    border-color: var(--text);
  }

  .results-content {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .result-section {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .result-section h3 {
    font-size: 14px;
    font-weight: 600;
    color: var(--text);
    margin: 0;
    padding-bottom: 6px;
    border-bottom: 1px solid var(--border);
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
    flex-shrink: 0;
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
    flex-shrink: 0;
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
    to { transform: rotate(360deg); }
  }

  .load-more {
    align-self: center;
    background: var(--surface-2);
    border: 1px solid var(--border);
    color: var(--text-dim);
    padding: 6px 16px;
    border-radius: 6px;
    font-size: 12px;
    cursor: pointer;
    margin-top: 4px;
    transition: background-color 0.16s ease, color 0.16s ease;
  }

  .load-more:hover {
    background: var(--surface-hover);
    color: var(--text);
  }

  .status-box.error {
    color: #ef4444;
  }

  @media (max-width: 600px) {
    .search-results-page {
      padding: 16px;
    }
  }
</style>
