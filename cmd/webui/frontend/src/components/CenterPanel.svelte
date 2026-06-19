<script>
  // CenterPanel.svelte orchestrates the multi-page layout.
  //
  // Purpose: Manages subpage views, breadcrumb navigation, and tab switching.
  //
  // Dependencies:
  //   - lib/playerStore.js: activeView, navigationStack, searchResults,
  //     artistDetail, playlistDetail stores and handlers.

  import {
    activeView,
    navigationStack,
    searchResults,
    artistDetail,
    playlistDetail,
    navigateTo,
    navigateBack,
  } from "../lib/playerStore.js";

  import HomePage from "./pages/HomePage.svelte";
  import SearchResultsPage from "./pages/SearchResultsPage.svelte";
  import ArtistPage from "./pages/ArtistPage.svelte";
  import PlaylistPage from "./pages/PlaylistPage.svelte";
  import DspPage from "./pages/DspPage.svelte";
  import Icon from "./Icon.svelte";

  function getLabelFor(view, data) {
    if (view === "home") return "Home";
    if (view === "dsp") return "DSP";
    if (view === "search") return `Search "${data?.query || ""}"`;
    if (view === "artist") return data?.details?.name || "Artist";
    if (view === "playlist") return data?.details?.name || "Playlist";
    return view;
  }

  function getActiveData(view) {
    if (view === "search") return $searchResults;
    if (view === "artist") return $artistDetail;
    if (view === "playlist") return $playlistDetail;
    return null;
  }

  $: rawBreadcrumbs = [
    ...$navigationStack.map((item) => ({
      view: item.view,
      data: item.data,
      label: getLabelFor(item.view, item.data),
    })),
    {
      view: $activeView,
      data: getActiveData($activeView),
      label: getLabelFor($activeView, getActiveData($activeView)),
    },
  ];

  // post-process to remove consecutive duplicates and apply collapse limit
  $: breadcrumbs = (() => {
    // 1. Remove consecutive duplicates by label
    const deduped = [];
    for (const bc of rawBreadcrumbs) {
      if (deduped.length === 0 || deduped[deduped.length - 1].label !== bc.label) {
        deduped.push(bc);
      }
    }

    // 2. Limit the depth. If exceeds 3 items, collapse middle.
    const LIMIT = 3;
    if (deduped.length <= LIMIT) {
      return deduped;
    }

    const collapsed = [
      deduped[0],
      { isCollapsedSpacer: true, label: "..." },
      ...deduped.slice(deduped.length - (LIMIT - 1))
    ];
    return collapsed;
  })();

  function clickBreadcrumb(bc) {
    if (bc.isCollapsedSpacer) return;
    const rawIndex = rawBreadcrumbs.findIndex(
      (raw) => raw.label === bc.label && raw.view === bc.view
    );
    if (rawIndex === -1 || rawIndex === rawBreadcrumbs.length - 1) return;
    navigationStack.set($navigationStack.slice(0, rawIndex));
    navigateTo(bc.view, bc.data);
  }
</script>

<div class="center-panel-container">
  <!-- Navigation bar (back, breadcrumbs, tabs) -->
  <div class="nav-bar">
    <div class="nav-left">
      {#if $navigationStack.length > 0}
        <button class="nav-btn" on:click={navigateBack} title="Go back">
          <Icon name="arrow-left" size={14} />
        </button>
      {/if}

      <div class="breadcrumbs-trail">
        {#each breadcrumbs as bc, i}
          {#if i > 0}
            <span class="bc-separator">/</span>
          {/if}
          {#if bc.isCollapsedSpacer}
            <span class="bc-item collapsed" title="More pages">{bc.label}</span>
          {:else}
            <!-- svelte-ignore a11y-click-events-have-key-events -->
            <!-- svelte-ignore a11y-no-static-element-interactions -->
            <span
              class="bc-item"
              class:active={i === breadcrumbs.length - 1}
              on:click={() => clickBreadcrumb(bc)}
            >
              {bc.label}
            </span>
          {/if}
        {/each}
      </div>
    </div>

    <div class="nav-tabs">
      <button
        class="tab-btn"
        class:active={$activeView === "home"}
        on:click={() => navigateTo("home", null, true)}
        title="Home"
      >
        <Icon name="home" size={15} />
      </button>
      <button
        class="tab-btn"
        class:active={$activeView === "dsp"}
        on:click={() => navigateTo("dsp", null, true)}
        title="DSP & Equalizer"
      >
        <Icon name="sliders-horizontal" size={15} />
      </button>
    </div>
  </div>

  <!-- Active sub-page content viewport -->
  <div class="panel-content">
    {#if $activeView === "home"}
      <HomePage />
    {:else if $activeView === "search"}
      <SearchResultsPage searchData={$searchResults} />
    {:else if $activeView === "artist"}
      <ArtistPage artistData={$artistDetail} />
    {:else if $activeView === "playlist"}
      <PlaylistPage playlistData={$playlistDetail} />
    {:else if $activeView === "dsp"}
      <DspPage />
    {/if}
  </div>
</div>

<style>
  .center-panel-container {
    display: flex;
    flex-direction: column;
    flex: 1;
    height: 100%;
    min-height: 0;
    background-color: var(--bg);
  }

  .nav-bar {
    height: 44px;
    background-color: var(--surface-1);
    border-bottom: 1px solid var(--border);
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 16px;
    flex-shrink: 0;
  }

  .nav-left {
    display: flex;
    align-items: center;
    gap: 12px;
    min-width: 0;
  }

  .nav-btn {
    background: transparent;
    border: none;
    color: var(--text-dim);
    padding: 6px;
    border-radius: 6px;
    cursor: pointer;
    display: flex;
    align-items: center;
    transition: background-color 0.16s ease, color 0.16s ease;
  }

  .nav-btn:hover {
    background: var(--surface-hover);
    color: var(--text);
  }

  .breadcrumbs-trail {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12.5px;
    color: var(--text-dim);
    min-width: 0;
  }

  .bc-item {
    cursor: pointer;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 140px;
    transition: color 0.12s ease;
  }

  .bc-item:hover {
    color: var(--text);
  }

  .bc-item.active {
    color: var(--text);
    font-weight: 500;
    pointer-events: none;
  }

  .bc-item.collapsed {
    cursor: default;
    color: var(--text-faint);
    pointer-events: none;
  }

  .bc-separator {
    color: var(--text-faint);
    user-select: none;
  }

  .nav-tabs {
    display: flex;
    gap: 4px;
  }

  .tab-btn {
    background: transparent;
    border: none;
    color: var(--text-faint);
    padding: 6px 10px;
    border-radius: 6px;
    cursor: pointer;
    display: flex;
    align-items: center;
    transition: background-color 0.16s ease, color 0.16s ease;
  }

  .tab-btn:hover {
    background: var(--surface-hover);
    color: var(--text);
  }

  .tab-btn.active {
    background: var(--surface-3);
    color: var(--text);
  }

  .panel-content {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
</style>
