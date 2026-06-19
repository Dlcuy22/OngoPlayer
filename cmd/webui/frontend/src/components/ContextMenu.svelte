<script>
  // ContextMenu.svelte renders a popup context menu.
  //
  // Purpose: Displays options (like play, remove) at the cursor's location.
  // Handles outside clicks and page borders dynamically.

  import { onMount } from "svelte";
  import Icon from "./Icon.svelte";

  export let x = 0;
  export let y = 0;
  export let items = []; // Array of { label, icon, action }
  export let onClose = () => {};

  let menuElement;

  function handleClickOutside(e) {
    if (menuElement && !menuElement.contains(e.target)) {
      onClose();
    }
  }

  function handleKeyDown(e) {
    if (e.key === "Escape") {
      onClose();
    }
  }

  onMount(() => {
    // Small delay to prevent the initial trigger click from closing the menu immediately
    const timer = setTimeout(() => {
      document.addEventListener("click", handleClickOutside);
    }, 50);

    document.addEventListener("keydown", handleKeyDown);

    // Adjust coordinates to keep the menu fully on-screen
    if (menuElement) {
      const rect = menuElement.getBoundingClientRect();
      const screenW = window.innerWidth;
      const screenH = window.innerHeight;

      if (x + rect.width > screenW) {
        x = screenW - rect.width - 8;
      }
      if (y + rect.height > screenH) {
        y = screenH - rect.height - 8;
      }
    }

    return () => {
      clearTimeout(timer);
      document.removeEventListener("click", handleClickOutside);
      document.removeEventListener("keydown", handleKeyDown);
    };
  });
</script>

<div
  class="context-menu"
  bind:this={menuElement}
  style="left: {x}px; top: {y}px;"
  on:contextmenu|preventDefault
>
  {#each items as item}
    <button
      class="menu-item"
      on:click={(e) => {
        e.stopPropagation();
        item.action();
        onClose();
      }}
    >
      {#if item.icon}
        <Icon name={item.icon} size={13} />
      {/if}
      <span>{item.label}</span>
    </button>
  {/each}
</div>

<style>
  .context-menu {
    position: fixed;
    background: var(--surface-1);
    border: 1px solid var(--border-strong);
    border-radius: 6px;
    padding: 4px;
    z-index: 1000;
    min-width: 150px;
    display: flex;
    flex-direction: column;
    gap: 2px;
    box-shadow: 0 6px 16px rgba(0, 0, 0, 0.6);
  }

  .menu-item {
    display: flex;
    align-items: center;
    gap: 10px;
    background: transparent;
    border: none;
    color: var(--text-dim);
    padding: 8px 12px;
    font-size: 12.5px;
    border-radius: 4px;
    width: 100%;
    text-align: left;
    cursor: pointer;
    outline: none;
    transition: background-color 0.12s ease, color 0.12s ease;
  }

  .menu-item:hover {
    background: var(--surface-hover);
    color: var(--text);
  }
</style>
