// playerStore.js manages the reactive state and Wails backend communication
// for the OngoPlayer WebUI. It decouples the UI from the audio engine IPC.
//
// Key Functions:
//   - initPlayerSync(): Binds Wails events to Svelte stores
//   - playerSeekEnd(): Handles robust seek synchronization
//   - playerToggle(): Toggles play/pause state
//
// Dependencies:
//   - svelte/store: Provides reactive state variables
//   - wailsjs/go/main/App.js: IPC bindings to the Go backend
//   - wailsjs/runtime/runtime.js: Wails event listener API
//
// Error Types:
//   - Backend sync failures are handled silently by reverting to known state.
//
// Example:
//   import { position, playerToggle } from './playerStore.js';
//   playerToggle($isPlaying);

import { writable } from 'svelte/store';
import {
  PlayFile, PlayTrack, Pause, Resume, Seek,
  SetVolume, GetVolume, Next, Prev,
  PickFolder, GetCurrentTrack
} from "../../wailsjs/go/main/App.js";
import { EventsOn } from "../../wailsjs/runtime/runtime.js";

export const position = writable(0);
export const duration = writable(0);
export const isPlaying = writable(false);
export const volume = writable(30);
export const currentTrack = writable(null);
export const queue = writable([]);

let isSeeking = false;
let expectedPosition = -1;

/*
initPlayerSync initializes the Wails event listeners and synchronizes the frontend Svelte stores with the Go backend's audio engine state.

	params:
	      none
	returns:
	      void
	Note:
	      Implements "Target Convergence" to prevent progress bar rubber-banding during far seeks.
*/
export function initPlayerSync() {
  GetVolume().then(v => volume.set(v));

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

  EventsOn("track_changed", (track) => {
    if (track) {
      currentTrack.set(track);
      isPlaying.set(true);
      position.set(0);
    }
  });
}

/*
playerToggle pauses or resumes the audio playback based on the current state.

	params:
	      currentlyPlaying: boolean indicating if audio is currently playing
	returns:
	      void
*/
export function playerToggle(currentlyPlaying) {
  if (currentlyPlaying) {
    Pause().then(() => isPlaying.set(false));
  } else {
    Resume().then(() => isPlaying.set(true));
  }
}

/*
playerSeekStart locks the progress bar from receiving backend updates during a user drag interaction.

	params:
	      none
	returns:
	      void
*/
export function playerSeekStart() {
  isSeeking = true;
}

/*
playerSeekInput updates the local position store during a seek drag for smooth UI scrubbing without triggering IPC calls.

	params:
	      val: the new position in seconds
	returns:
	      void
*/
export function playerSeekInput(val) {
  position.set(val);
}

/*
playerSeekEnd sends the seek command to the backend and locks the UI state until the backend confirms the new position.

	params:
	      seekTo: the target position in seconds
	returns:
	      void
	Note:
	      Uses expectedPosition to ignore stale backend events.
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

/*
playerVolumeChange updates the local volume store and sends the command to the backend.

	params:
	      val: volume level (0-100)
	returns:
	      void
*/
export function playerVolumeChange(val) {
  volume.set(val);
  SetVolume(val);
}

/*
playerNext requests the backend to play the next track in the queue.

	params:
	      none
	returns:
	      void
*/
export function playerNext() { Next(); }

/*
playerPrev requests the backend to play the previous track in the queue.

	params:
	      none
	returns:
	      void
*/
export function playerPrev() { Prev(); }

/*
playerPickFolder opens a native OS directory picker dialog and loads the resulting tracks into the queue.

	params:
	      none
	returns:
	      void
*/
export function playerPickFolder() {
  PickFolder().then((tracks) => {
    if (tracks && tracks.length > 0) {
      queue.set(tracks);
    }
  });
}

/*
playerPlayQueueIndex plays a specific track from the current queue based on its index.

	params:
	      index: the array index of the track in the queue
	returns:
	      void
*/
export function playerPlayQueueIndex(index) {
  PlayTrack(index).then(() => isPlaying.set(true));
}

