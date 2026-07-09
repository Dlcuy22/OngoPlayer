// playerStore.js manages the reactive state and Wails backend communication
// for the OngoPlayer WebUI. It decouples the UI from the audio engine IPC.
//
// Key Functions:
//   - initPlayerSync(): binds Wails events to Svelte stores
//   - playerSeekEnd(): handles robust seek synchronization
//   - playerToggle(): toggles play/pause state
//   - getCover(): lazy per-track cover fetch with in-memory cache
//   - cycleLoop()/toggleShuffle(): playback mode controls
//
// Dependencies:
//   - svelte/store: reactive state variables
//   - wailsjs/go/main/App.js: IPC bindings to the Go backend
//   - wailsjs/runtime/runtime.js: Wails event listener API

import { writable, get } from "svelte/store";
import {
  PlayFile, PlayTrack, Pause, Resume, Seek,
  SetVolume, GetVolume, Next, Prev,
  PickFolder, PickFolderAppend, GetCurrentTrack, GetCover,
  SetShuffle, GetShuffle, SetLoopMode, GetLoopMode,
  SetRPCEnabled, GetRPCEnabled,
  ClearQueue, GetQueue, GetYTMArtist, GetYTMPlaylist,
  PlayYTMSong, RemoveFromQueue, ReorderQueue, SearchYTM,
  InsertYTMSongAt,
} from "../../wailsjs/go/main/App.js";
import { EventsOn } from "../../wailsjs/runtime/runtime.js";

export const position = writable(0);
export const duration = writable(0);
export const isPlaying = writable(false);
export const volume = writable(30);
export const currentTrack = writable(null);
export const queue = writable([]);
export const shuffle = writable(false);
export const loopMode = writable(0); // 0 off, 1 all, 2 one
export const coverUrl = writable(""); // cover for the active track ("" = none)
export const lyrics = writable([]); // [{ time, text }] for the active track
export const loadingIndex = writable(-1);
export const lyricsLoading = writable(false);

// Multi-page routing stores
export const activeView = writable("home");
export const navigationStack = writable([]);
export const searchResults = writable(null);
export const artistDetail = writable(null);
export const playlistDetail = writable(null);

// Settings UI state and persisted preferences.
export const showSettings = writable(false);
export const rpcEnabled = writable(false);

const FONT_KEY = "ongo.lyricsFontSize";
const FONT_MIN = 12;
const FONT_MAX = 32;
const FONT_DEFAULT = 16;

const ANIM_KEY = "ongo.animationsEnabled";

function loadFontSize() {
  const raw = parseInt(
    typeof localStorage !== "undefined" ? localStorage.getItem(FONT_KEY) : "",
    10
  );
  if (isNaN(raw)) return FONT_DEFAULT;
  return Math.min(FONT_MAX, Math.max(FONT_MIN, raw));
}

function loadAnimations() {
  if (typeof localStorage === "undefined") return true;
  const raw = localStorage.getItem(ANIM_KEY);
  return raw === null ? true : raw === "1"; // default on
}

export const lyricsFontSize = writable(loadFontSize()); // px
export const animationsEnabled = writable(loadAnimations());

let isSeeking = false;
let expectedPosition = -1;

// In-memory cover cache keyed by queue index. Value is a data URL or "" (known
// to have no art). Avoids re-extracting covers on every selection.
const coverCache = new Map();

/*
initPlayerSync initializes the Wails event listeners and synchronizes the
frontend Svelte stores with the Go backend's audio engine state.

	Note: implements "Target Convergence" to prevent progress bar rubber-banding
	during far seeks.
*/
export function initPlayerSync() {
  GetVolume().then((v) => volume.set(v));
  GetShuffle().then((s) => shuffle.set(s));
  GetLoopMode().then((m) => loopMode.set(m));
  GetRPCEnabled().then((on) => rpcEnabled.set(on));
  GetQueue().then((tracks) => queue.set(tracks || []));

  EventsOn("playback_progress", (data) => {
    duration.set(data.duration);
    if (isSeeking) return;

    if (expectedPosition !== -1) {
      const diff = Math.abs(data.position - expectedPosition);
      if (diff > 2.0) {
        return;
      } else {
        expectedPosition = -1;
      }
    }
    position.set(data.position);
  });

  EventsOn("track_completed", () => {
    isPlaying.set(false);
    position.set(0);
  });

  EventsOn("track_loading", (data) => {
    loadingIndex.set(data.index);
  });

  EventsOn("track_loading_finished", (data) => {
    loadingIndex.set(-1);
  });

  EventsOn("track_changed", (payload) => {
    loadingIndex.set(-1);
    const track = payload ? payload.track : null;
    const playing = payload ? payload.playing : false;
    if (track) {
      currentTrack.set(track);
      isPlaying.set(playing);
      position.set(0);
      lyrics.set([]); // clear stale lyrics until the new ones arrive
      loadCoverFor(track);
      lyricsLoading.set(true);
    } else {
      currentTrack.set(null);
      isPlaying.set(false);
      position.set(0);
      lyrics.set([]);
      coverUrl.set("");
      lyricsLoading.set(false);
    }
  });

  EventsOn("lyrics_loaded", (data) => {
    if (!data) return;
    // Ignore results for a track that is no longer active (late API responses).
    const cur = get(currentTrack);
    if (cur && cur.path !== data.path) return;
    lyrics.set(Array.isArray(data.lines) ? data.lines : []);
    lyricsLoading.set(false);
  });

  EventsOn("queue_changed", (tracks) => {
    queue.set(tracks || []);
    // Evict old entries from coverCache that are no longer in the queue.
    if (tracks && tracks.length > 0) {
      const activePaths = new Set(tracks.map(t => t.path));
      for (const key of coverCache.keys()) {
        if (!activePaths.has(key)) {
          coverCache.delete(key);
        }
      }
    } else {
      coverCache.clear();
    }
  });
}

/*
loadCoverFor resolves the cover for a track and publishes it to coverUrl,
using the cache to avoid repeat backend calls.

	params:
	      track: TrackInfo (must carry index and hasCover)
*/
function loadCoverFor(track) {
  if (!track) {
    coverUrl.set("");
    return;
  }
  if (track.coverURL) {
    coverUrl.set(track.coverURL);
    return;
  }
  if (!track.hasCover) {
    coverUrl.set("");
    return;
  }
  const path = track.path;
  const idx = track.index;
  if (coverCache.has(path)) {
    coverUrl.set(coverCache.get(path));
    return;
  }
  // Clear stale art while the new one loads.
  coverUrl.set("");
  GetCover(idx).then((url) => {
    coverCache.set(path, url || "");
    // Only apply if this track is still the active one.
    const cur = get(currentTrack);
    if (cur && cur.path === path) {
      coverUrl.set(url || "");
    }
  });
}

/*
getCover returns a Promise<string> data URL for a queue index, caching results.
Used by the tracklist for mini thumbnails.

	params:
	      index:    queue index
	      hasCover: skip the backend call when the track has no embedded art
	returns:
	      Promise<string>
*/
export function getCover(index, hasCover) {
  const q = get(queue);
  if (!q || !q[index]) return Promise.resolve("");
  const track = q[index];
  if (track.coverURL) {
    return Promise.resolve(track.coverURL);
  }
  if (!hasCover) return Promise.resolve("");
  const path = track.path;
  if (coverCache.has(path)) return Promise.resolve(coverCache.get(path));
  return GetCover(index).then((url) => {
    coverCache.set(path, url || "");
    return url || "";
  });
}

/*
playerToggle pauses or resumes audio playback based on the current state.

	params:
	      currentlyPlaying: whether audio is currently playing
*/
export function playerToggle(currentlyPlaying) {
  if (currentlyPlaying) {
    Pause().then(() => isPlaying.set(false));
  } else {
    Resume().then(() => isPlaying.set(true));
  }
}

// playerSeekStart locks the progress bar from backend updates during a drag.
export function playerSeekStart() {
  isSeeking = true;
}

// playerSeekInput updates the local position during a drag, no IPC.
export function playerSeekInput(val) {
  position.set(val);
}

/*
playerSeekEnd commits the seek and locks UI state until the backend confirms.

	params:
	      seekTo: target position in seconds
	Note: uses expectedPosition to ignore stale backend events.
*/
export function playerSeekEnd(seekTo) {
  isSeeking = true;
  expectedPosition = seekTo;
  Seek(seekTo)
    .then(() => {
      position.set(seekTo);
      isSeeking = false;
    })
    .catch(() => {
      isSeeking = false;
      expectedPosition = -1;
    });
}

// playerVolumeChange updates local volume and pushes it to the backend.
export function playerVolumeChange(val) {
  volume.set(val);
  SetVolume(val);
}

// playerNext / playerPrev request track navigation.
export function playerNext() { Next(); }
export function playerPrev() { Prev(); }

// toggleShuffle flips shuffle state in the backend and store.
export function toggleShuffle() {
  const next = !get(shuffle);
  shuffle.set(next);
  SetShuffle(next);
  resetCovers(); // ponytail: cover cache keyed by index goes stale after shuffle
}

// cycleLoop advances loop mode 0 -> 1 -> 2 -> 0 in the backend and store.
export function cycleLoop() {
  const next = (get(loopMode) + 1) % 3;
  loopMode.set(next);
  SetLoopMode(next);
}

// openSettings / closeSettings toggle the settings panel.
export function openSettings() { showSettings.set(true); }
export function closeSettings() { showSettings.set(false); }

// setRpcEnabled flips Discord Rich Presence in the backend and store.
export function setRpcEnabled(on) {
  rpcEnabled.set(on);
  SetRPCEnabled(on);
}

/*
setLyricsFontSize clamps and persists the lyrics font size preference.

	params:
	      px: requested font size in pixels
*/
export function setLyricsFontSize(px) {
  const v = Math.min(FONT_MAX, Math.max(FONT_MIN, parseInt(px, 10) || FONT_DEFAULT));
  lyricsFontSize.set(v);
  if (typeof localStorage !== "undefined") {
    localStorage.setItem(FONT_KEY, String(v));
  }
}

/*
setAnimationsEnabled toggles UI micro-animations and persists the preference.

	params:
	      on: whether animations are enabled
*/
export function setAnimationsEnabled(on) {
  animationsEnabled.set(on);
  if (typeof localStorage !== "undefined") {
    localStorage.setItem(ANIM_KEY, on ? "1" : "0");
  }
}

/*
playerSeekTo seeks to an absolute position (used by lyric click-to-seek).
Reuses the convergence logic so the progress bar does not rubber-band.

	params:
	      seconds: target position in seconds
*/
export function playerSeekTo(seconds) {
  playerSeekEnd(seconds);
}

// resetCovers clears the cover cache; called when the queue is rebuilt.
function resetCovers() {
  coverCache.clear();
}

/*
playerPickFolder opens a native picker and REPLACES the queue.
*/
export function playerPickFolder() {
  PickFolder();
}

/*
playerAppendFolder opens a native picker and APPENDS to the queue.
*/
export function playerAppendFolder() {
  PickFolderAppend();
}

/*
playerPlayQueueIndex plays a specific track from the current queue.

	params:
	      index: array index of the track in the queue
*/
export function playerPlayQueueIndex(index) {
  PlayTrack(index);
}

/*
playerRemoveTrack deletes a track from the queue by index.
*/
export function playerRemoveTrack(index) {
  RemoveFromQueue(index);
}

/*
playerReorderQueue moves a track from fromIndex to toIndex.
*/
export function playerReorderQueue(fromIndex, toIndex) {
  ReorderQueue(fromIndex, toIndex);
}

/*
playerClearQueue clears all tracks from the queue.
*/
export function playerClearQueue() {
  ClearQueue();
}

/*
playerPlayYTMSong appends a YTM song to the queue and starts playing it.
*/
export function playerPlayYTMSong(songID, title, artist, album, lyricsBrowseID, coverURL) {
  PlayYTMSong(songID, title, artist, album, lyricsBrowseID || "", coverURL || "");
}

/*
playerInsertYTMSongAt inserts a YTM song at the specified queue index without playing.
*/
export function playerInsertYTMSongAt(index, songID, title, artist, album, lyricsBrowseID, coverURL) {
  InsertYTMSongAt(index, songID, title, artist, album, lyricsBrowseID || "", coverURL || "");
}

/*
navigateTo updates the active view and navigation stack.
*/
export function navigateTo(view, data = null, clearStack = false) {
  if (clearStack) {
    navigationStack.set([]);
  } else {
    const stack = get(navigationStack);
    const curView = get(activeView);
    let curData = null;
    if (curView === "search") curData = get(searchResults);
    else if (curView === "artist") curData = get(artistDetail);
    else if (curView === "playlist") curData = get(playlistDetail);

    // Prevent consecutive duplicate navigation items
    let shouldPush = true;
    if (curView === view) {
      if (view === "search") {
        shouldPush = false;
      } else if (curData && data) {
        const curID = curData.details?.id || curData.id;
        const newID = data.details?.id || data.id;
        if (curID === newID) {
          shouldPush = false;
        }
      } else {
        shouldPush = false;
      }
    }

    if (shouldPush && curView) {
      navigationStack.set([...stack, { view: curView, data: curData }]);
    }
  }

  activeView.set(view);
  if (view === "search") searchResults.set(data);
  else if (view === "artist") artistDetail.set(data);
  else if (view === "playlist") playlistDetail.set(data);
}

/*
navigateBack goes back one step in the navigation history.
*/
export function navigateBack() {
  const stack = get(navigationStack);
  if (stack.length === 0) {
    activeView.set("home");
    return;
  }
  const prev = stack[stack.length - 1];
  navigationStack.set(stack.slice(0, -1));
  activeView.set(prev.view);
  if (prev.view === "search") searchResults.set(prev.data);
  else if (prev.view === "artist") artistDetail.set(prev.data);
  else if (prev.view === "playlist") playlistDetail.set(prev.data);
}
