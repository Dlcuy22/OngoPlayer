<script>
  // NowPlaying.svelte is the center panel: cover art, title/artist, and a synced
  // lyrics view. The lyrics auto-scroll to keep the active line centered using a
  // spring (svelte/motion) for smooth physics-based motion. Users can click a
  // line to seek, and manually scroll to browse; auto-scroll resumes 2s after
  // the last manual scroll. Instrumental gaps (no lyric for >2s) render a music
  // note placeholder.
  //
  // Note: framer-motion is a React library and cannot run in this Svelte
  // project, so we use Svelte's built-in spring, which gives equivalent
  // physics-based smoothness with no extra dependency.
  //
  // Props:
  //   track: TrackInfo or null
  //   isPlaying: whether playback is active
  //   cover: data URL for the active track cover, or "" when none
  //   lyrics: array of { time, text } ([] when none)
  //   position: current playback position in seconds
  //   fontSize: lyrics font size in px
  //   animations: whether smooth motion is enabled

  import { onDestroy, createEventDispatcher } from "svelte";
  import { spring } from "svelte/motion";
  import Icon from "./Icon.svelte";

  export let track = null;
  export let isPlaying = false;
  export let cover = "";
  export let lyrics = [];
  export let position = 0;
  export let fontSize = 16;
  export let animations = true;

  const dispatch = createEventDispatcher();

  const INSTRUMENTAL_GAP = 2.0; // seconds with no lyric before showing the note
  const RESUME_DELAY = 2000; // ms after manual scroll before auto-scroll resumes

  let viewport; // scroll container
  let lineEls = []; // per-line element refs
  let noteEl; // instrumental intro note ref
  let userScrolling = false;
  let resumeTimer = null;
  let suppressScrollEvent = false;

  // Spring drives the scrollTop so motion is smooth and physics-based.
  const scrollY = spring(0, { stiffness: 0.08, damping: 0.32 });
  $: scrollY.stiffness = animations ? 0.08 : 1;
  $: scrollY.damping = animations ? 0.32 : 1;
  const unsubScroll = scrollY.subscribe((y) => {
    if (!viewport) return;
    suppressScrollEvent = true;
    viewport.scrollTop = y;
  });
  onDestroy(unsubScroll);

  function displayTitle(t) {
    if (!t) return "Nothing playing";
    if (t.title) return t.title;
    const name = t.name || "";
    const dot = name.lastIndexOf(".");
    return dot > 0 ? name.slice(0, dot) : name || "Unknown";
  }

  $: subtitle = track
    ? [track.artist, track.album].filter(Boolean).join("  -  ") ||
      (track.format ? track.format : "Local file")
    : "Pick a folder to start listening";

  $: hasLyrics = Array.isArray(lyrics) && lyrics.length > 0;

  // Reset scroll/browse state whenever a new lyrics set arrives (track change),
  // so the spring does not carry the previous track's scroll position.
  let prevLyrics = null;
  $: if (lyrics !== prevLyrics) {
    prevLyrics = lyrics;
    userScrolling = false;
    if (resumeTimer) clearTimeout(resumeTimer);
    scrollY.set(0, { hard: true });
  }

  // Active line is the last line whose timestamp has been reached.
  $: activeIndex = (() => {
    if (!hasLyrics) return -1;
    let idx = -1;
    for (let i = 0; i < lyrics.length; i++) {
      if (lyrics[i].time <= position) idx = i;
      else break;
    }
    return idx;
  })();

  // Instrumental intro: playing before the first line arrives. We only flag the
  // intro (and not inter-line gaps) because LRC has no per-line end time, so a
  // long-held line is indistinguishable from a gap; treating held lines as
  // instrumental made the active line shrink then grow. The active line stays
  // highlighted and centered through long sections instead.
  $: instrumental =
    hasLyrics && activeIndex < 0 && lyrics[0].time - position > INSTRUMENTAL_GAP;

  // Recenter whenever the active line (or intro note) changes, unless browsing.
  $: if (hasLyrics) recenter(activeIndex, instrumental);

  function recenter(i, intro) {
    if (userScrolling || !viewport) return;
    const el = intro && i < 0 ? noteEl : lineEls[i];
    if (!el) return;
    const target = el.offsetTop - viewport.clientHeight / 2 + el.clientHeight / 2;
    scrollY.set(Math.max(0, target));
  }

  // Distinguish programmatic scrolls (from the spring) from real user scrolls.
  function onScroll() {
    if (suppressScrollEvent) {
      suppressScrollEvent = false;
      return;
    }
    userScrolling = true;
    if (resumeTimer) clearTimeout(resumeTimer);
    resumeTimer = setTimeout(() => {
      userScrolling = false;
      recenter(activeIndex, instrumental);
    }, RESUME_DELAY);
  }

  onDestroy(() => resumeTimer && clearTimeout(resumeTimer));

  function seekToLine(line) {
    dispatch("seek", line.time);
  }
</script>

<div class="now-playing">
  <div class="art" class:playing={isPlaying && track} class:compact={hasLyrics}>
    {#if cover}
      <img src={cover} alt="Cover art" />
    {:else}
      <Icon name="music" size={hasLyrics ? 44 : 64} strokeWidth={1.25} />
    {/if}
  </div>

  <div class="meta">
    <h1 class="title" title={displayTitle(track)}>{displayTitle(track)}</h1>
    <p class="subtitle">{subtitle}</p>
  </div>

  {#if hasLyrics}
    <div
      class="lyrics-viewport"
      class:no-anim={!animations}
      bind:this={viewport}
      on:scroll={onScroll}
      style="--lyric-size: {fontSize}px"
    >
      <!-- Top/bottom spacers let the first and last lines reach the center. -->
      <div class="lyric-pad"></div>

      {#if instrumental}
        <div class="instrumental" bind:this={noteEl} aria-hidden="true">
          <Icon name="music" size={Math.round(fontSize * 1.4)} strokeWidth={1.5} />
        </div>
      {/if}

      {#each lyrics as line, i}
        <button
          type="button"
          class="lyric-line"
          class:active={i === activeIndex}
          class:past={i < activeIndex}
          bind:this={lineEls[i]}
          on:click={() => seekToLine(line)}
          title="Seek to this line"
        >
          {line.text}
        </button>
      {/each}

      <div class="lyric-pad"></div>
    </div>
  {:else}
    <div class="lyrics-empty-wrap">
      <span class="lyrics-empty">Lyrics unavailable</span>
    </div>
  {/if}
</div>

<style>
  .now-playing {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    padding: 28px 32px;
    overflow: hidden;
  }

  .art {
    width: 220px;
    height: 220px;
    flex-shrink: 0;
    border-radius: 14px;
    background: var(--surface-2);
    border: 1px solid var(--border);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-faint);
    overflow: hidden;
    box-shadow: 0 10px 40px rgba(0, 0, 0, 0.35);
    transition: color 0.4s ease, border-color 0.4s ease, width 0.3s ease,
      height 0.3s ease;
  }

  .art.compact {
    width: 132px;
    height: 132px;
  }

  .art.playing {
    color: var(--text-dim);
    border-color: var(--border-strong);
  }

  .art img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }

  .meta {
    margin-top: 20px;
    text-align: center;
    max-width: 90%;
    flex-shrink: 0;
  }

  .title {
    margin: 0;
    font-size: 20px;
    font-weight: 700;
    color: var(--text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .subtitle {
    margin: 6px 0 0;
    font-size: 13px;
    color: var(--text-faint);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .lyrics-empty-wrap {
    margin-top: 32px;
    flex-shrink: 0;
  }

  .lyrics-empty {
    font-size: 12px;
    color: var(--text-faint);
    letter-spacing: 0.04em;
  }

  .lyrics-viewport {
    position: relative;
    margin-top: 20px;
    width: 100%;
    max-width: 560px;
    flex: 1;
    min-height: 0;
    box-sizing: border-box;
    overflow-y: auto;
    overflow-x: hidden;
    text-align: center;
    /* Fade top and bottom edges so scrolled lines dissolve. */
    -webkit-mask-image: linear-gradient(
      to bottom,
      transparent 0,
      #000 56px,
      #000 calc(100% - 56px),
      transparent 100%
    );
    mask-image: linear-gradient(
      to bottom,
      transparent 0,
      #000 56px,
      #000 calc(100% - 56px),
      transparent 100%
    );
  }

  /* Hide both scrollbars; navigation is automatic or via drag. */
  .lyrics-viewport::-webkit-scrollbar {
    width: 0;
    height: 0;
  }

  /* Half the viewport height of padding so the first and last lyric lines can
     scroll all the way to the vertical center. */
  .lyric-pad {
    height: 50%;
    flex-shrink: 0;
  }

  .instrumental {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 10px 0;
    color: var(--text-dim);
  }

  .lyric-line {
    display: block;
    width: 100%;
    margin: 0;
    padding: 9px 12px;
    background: transparent;
    border: none;
    font: inherit;
    font-size: var(--lyric-size, 16px);
    line-height: 1.45;
    color: var(--text-faint);
    text-align: center;
    cursor: pointer;
    overflow-wrap: break-word;
    word-break: break-word;
    transition: color 0.25s ease, transform 0.25s ease;
  }

  .lyric-line:hover {
    color: var(--text-dim);
  }

  .lyric-line.past {
    color: var(--text-faint);
  }

  .lyric-line.active {
    color: var(--text);
    font-weight: 700;
    transform: scale(1.05);
  }

  .no-anim .lyric-line {
    transition: none;
  }
</style>
