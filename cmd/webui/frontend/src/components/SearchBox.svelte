<script>
  // SearchBox.svelte implements the search input with real-time autocompletion.
  //
  // Purpose: Fetches autocomplete suggestions from YouTube Music as the user
  // types (debounced), and triggers search navigation on submit.
  //
  // Dependencies:
  //   - lib/playerStore.js: navigation handlers
  //   - wailsjs/go/main/App.js: GetYTMSuggestions, SearchYTM bindings

  import { onMount } from "svelte";
  import { GetYTMSuggestions, SearchYTM } from "../../wailsjs/go/main/App.js";
  import { navigateTo } from "../lib/playerStore.js";
  import Icon from "./Icon.svelte";

  let query = "";
  let suggestions = [];
  let showDropdown = false;
  let debounceTimer;
  let wrapperRef;

  function handleInput() {
    clearTimeout(debounceTimer);
    if (!query.trim()) {
      suggestions = [];
      return;
    }
    debounceTimer = setTimeout(async () => {
      try {
        suggestions = await GetYTMSuggestions(query);
      } catch (err) {
        console.error("autocomplete failed:", err);
      }
    }, 300);
  }

  async function triggerSearch(q) {
    if (!q || !q.trim()) return;
    query = q;
    suggestions = [];
    showDropdown = false;

    // Blur input
    const input = document.getElementById("search-input");
    if (input) input.blur();

    // Set view to search in loading state
    navigateTo("search", { query: q, loading: true });

    try {
      const results = await SearchYTM(q);
      navigateTo("search", { query: q, results: results, loading: false });
    } catch (err) {
      console.error("search failed:", err);
      navigateTo("search", { query: q, error: err.toString(), loading: false });
    }
  }

  function handleKeyDown(e) {
    if (e.key === "Enter") {
      triggerSearch(query);
    } else if (e.key === "Escape") {
      showDropdown = false;
    }
  }

  function clearQuery() {
    query = "";
    suggestions = [];
    showDropdown = false;
    const input = document.getElementById("search-input");
    if (input) input.focus();
  }

  // Close suggestions dropdown on click outside
  function handleClickOutside(e) {
    if (wrapperRef && !wrapperRef.contains(e.target)) {
      showDropdown = false;
    }
  }

  onMount(() => {
    document.addEventListener("click", handleClickOutside);
    return () => {
      document.removeEventListener("click", handleClickOutside);
    };
  });
</script>

<div class="search-box-wrapper" bind:this={wrapperRef}>
  <span class="search-icon">
    <Icon name="search" size={14} />
  </span>
  <input
    id="search-input"
    type="text"
    placeholder="Search local library or YT Music..."
    bind:value={query}
    on:input={handleInput}
    on:focus={() => showDropdown = true}
    on:keydown={handleKeyDown}
    autocomplete="off"
  />
  {#if query}
    <button class="clear-btn" on:click={clearQuery} title="Clear search">
      <Icon name="x" size={14} />
    </button>
  {/if}

  {#if showDropdown && suggestions.length > 0}
    <div class="suggestions-dropdown">
      {#each suggestions as sug}
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div class="suggestion-item" on:click={() => triggerSearch(sug)}>
          <Icon name="search" size={12} strokeWidth={1.5} />
          <span>{sug}</span>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .search-box-wrapper {
    position: relative;
    width: 280px;
    display: flex;
    align-items: center;
  }

  input {
    width: 100%;
    background: var(--surface-2);
    border: 1px solid var(--border);
    color: var(--text);
    padding: 6px 32px 6px 30px;
    border-radius: 6px;
    font-size: 12.5px;
    outline: none;
    transition: border-color 0.16s ease, background-color 0.16s ease;
  }

  input:focus {
    border-color: var(--border-strong);
    background: var(--surface-3);
  }

  .search-icon {
    position: absolute;
    left: 10px;
    color: var(--text-faint);
    pointer-events: none;
    display: flex;
    align-items: center;
  }

  .clear-btn {
    position: absolute;
    right: 8px;
    background: transparent;
    border: none;
    color: var(--text-dim);
    cursor: pointer;
    padding: 2px;
    display: flex;
    align-items: center;
    border-radius: 4px;
  }

  .clear-btn:hover {
    color: var(--text);
    background: var(--surface-hover);
  }

  .suggestions-dropdown {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    right: 0;
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: 6px;
    z-index: 100;
    max-height: 240px;
    overflow-y: auto;
  }

  .suggestion-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 12px;
    font-size: 13px;
    color: var(--text-dim);
    cursor: pointer;
    transition: background-color 0.12s ease, color 0.12s ease;
  }

  .suggestion-item:hover {
    background: var(--surface-hover);
    color: var(--text);
  }

  .suggestion-item span {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
