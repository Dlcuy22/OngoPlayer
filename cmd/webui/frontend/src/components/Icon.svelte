<script>
  // Icon.svelte renders inlined lucide icon paths so the WebUI gets real
  // lucide iconography without adding a runtime dependency (svelte 3 + no
  // build-time icon plugin). Paths are copied verbatim from lucide.dev.
  //
  // Props:
  //   name: lucide icon id (see ICONS map below)
  //   size: pixel width/height (default 20)
  //   strokeWidth: stroke thickness (default 2)

  export let name = "";
  export let size = 20;
  export let strokeWidth = 2;

  // Each entry is an array of SVG child markup strings on a 24x24 viewBox.
  const ICONS = {
    play: ['<polygon points="6 3 20 12 6 21 6 3"/>'],
    pause: [
      '<rect x="14" y="4" width="4" height="16" rx="1"/>',
      '<rect x="6" y="4" width="4" height="16" rx="1"/>',
    ],
    "skip-back": [
      '<polygon points="19 20 9 12 19 4 19 20"/>',
      '<line x1="5" x2="5" y1="19" y2="5"/>',
    ],
    "skip-forward": [
      '<polygon points="5 4 15 12 5 20 5 4"/>',
      '<line x1="19" x2="19" y1="5" y2="19"/>',
    ],
    folder: [
      '<path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/>',
    ],
    music: [
      '<path d="M9 18V5l12-2v13"/>',
      '<circle cx="6" cy="18" r="3"/>',
      '<circle cx="18" cy="16" r="3"/>',
    ],
    "volume-2": [
      '<polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"/>',
      '<path d="M15.54 8.46a5 5 0 0 1 0 7.07"/>',
      '<path d="M19.07 4.93a10 10 0 0 1 0 14.14"/>',
    ],
    "volume-x": [
      '<polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"/>',
      '<line x1="22" x2="16" y1="9" y2="15"/>',
      '<line x1="16" x2="22" y1="9" y2="15"/>',
    ],
    "list-music": [
      '<path d="M21 15V6"/>',
      '<path d="M18.5 18a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5Z"/>',
      '<path d="M12 12H3"/>',
      '<path d="M16 6H3"/>',
      '<path d="M12 18H3"/>',
    ],
    "folder-plus": [
      '<path d="M12 10v6"/>',
      '<path d="M9 13h6"/>',
      '<path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/>',
    ],
    shuffle: [
      '<path d="M2 18h1.4c1.3 0 2.5-.6 3.3-1.7l6.1-8.6c.7-1.1 2-1.7 3.3-1.7H22"/>',
      '<path d="m18 2 4 4-4 4"/>',
      '<path d="M2 6h1.9c1.5 0 2.9.9 3.6 2.2"/>',
      '<path d="M22 18h-5.9c-1.3 0-2.6-.7-3.3-1.8l-.5-.8"/>',
      '<path d="m18 14 4 4-4 4"/>',
    ],
    repeat: [
      '<path d="m17 2 4 4-4 4"/>',
      '<path d="M3 11v-1a4 4 0 0 1 4-4h14"/>',
      '<path d="m7 22-4-4 4-4"/>',
      '<path d="M21 13v1a4 4 0 0 1-4 4H3"/>',
    ],
    "repeat-1": [
      '<path d="m17 2 4 4-4 4"/>',
      '<path d="M3 11v-1a4 4 0 0 1 4-4h14"/>',
      '<path d="m7 22-4-4 4-4"/>',
      '<path d="M21 13v1a4 4 0 0 1-4 4H3"/>',
      '<path d="M11 10h1v4"/>',
    ],
    settings: [
      '<path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/>',
      '<circle cx="12" cy="12" r="3"/>',
    ],
    x: [
      '<path d="M18 6 6 18"/>',
      '<path d="m6 6 12 12"/>',
    ],
    "sliders-horizontal": [
      '<line x1="21" x2="14" y1="4" y2="4"/>',
      '<line x1="10" x2="3" y1="4" y2="4"/>',
      '<line x1="21" x2="12" y1="12" y2="12"/>',
      '<line x1="8" x2="3" y1="12" y2="12"/>',
      '<line x1="21" x2="16" y1="20" y2="20"/>',
      '<line x1="12" x2="3" y1="20" y2="20"/>',
      '<line x1="14" x2="14" y1="2" y2="6"/>',
      '<line x1="8" x2="8" y1="10" y2="14"/>',
      '<line x1="16" x2="16" y1="18" y2="22"/>',
    ],
  };

  $: parts = ICONS[name] || [];
</script>

<svg
  xmlns="http://www.w3.org/2000/svg"
  width={size}
  height={size}
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width={strokeWidth}
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
>
  {@html parts.join("")}
</svg>

<style>
  svg {
    display: block;
    flex-shrink: 0;
  }
</style>
