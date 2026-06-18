// cmd/ytm-test/main.go
//
// Purpose:
//
//	Search and play the song "MURI MURI EVOLUTION" by Nanao Akari using ytm-go and AudioEngine.
//	Remuxes the WebM stream using ffmpeg to an Ogg/Opus file to satisfy libopusfile.
//
// Key Components:
//   - main: Orchestrates searching, downloading, remuxing, and playback of the Opus audio file
//
// Dependencies:
//   - context
//   - fmt
//   - io
//   - net/http
//   - os
//   - os/exec
//   - os/signal
//   - strings
//   - syscall
//   - time
//   - github.com/dlcuy22/OngoPlayer/Audioengine
//   - github.com/dlcuy22/OngoPlayer/Audioengine/StelleEngine
//   - github.com/dlcuy22/ytm-go
//
// Error Types:
//   - None
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	AudioEngine "github.com/dlcuy22/OngoPlayer/Audioengine"
	stelleengine "github.com/dlcuy22/OngoPlayer/Audioengine/StelleEngine"
	"github.com/dlcuy22/ytm-go"
	"github.com/lrstanley/go-ytdlp"
)

/*
main is the entry point for the ytm-test application.

	returns:
	      None
*/
func main() {
	ctx := context.Background()
	client := ytm.NewClient()

	fmt.Println("Searching for song: 'TEREK BALE'...")
	results, err := client.Search(ctx, "TEREK BALE", "", false)
	if err != nil {
		fmt.Printf("Search failed: %v\n", err)
		os.Exit(1)
	}

	var targetSong *ytm.Song
	fmt.Println("Search categories found:")
	for _, category := range results.Categories {
		fmt.Printf("Category Title: %s, Item count: %d\n", category.Layout.Title, len(category.Layout.Items))
		for _, item := range category.Layout.Items {
			switch it := item.(type) {
			case *ytm.Song:
				fmt.Printf(" - [Song] %s (Artist: %s, ID: %s, Type: %s)\n", it.Name, getArtistNames(it.Artists), it.ID, it.Type)
				if targetSong == nil {
					targetSong = it
				}
			case *ytm.Artist:
				fmt.Printf(" - [Artist] %s (ID: %s)\n", it.Name, it.ID)
			case *ytm.Playlist:
				fmt.Printf(" - [Playlist] %s (ID: %s, Type: %s)\n", it.Name, it.ID, it.Type)
			default:
				fmt.Printf(" - [Unknown] %s\n", item.GetName())
			}
		}
	}

	if targetSong == nil {
		fmt.Println("Target song not found in search results.")
		os.Exit(1)
	}

	fmt.Printf("Found Song: %s (ID: %s)\n", targetSong.Name, targetSong.ID)

	fmt.Println("Installing/checking go-ytdlp dependencies...")
	ytdlp.MustInstallAll(ctx)

	tempOpus := "./temp_muri.opus"
	fmt.Printf("Downloading and converting track to Opus directly via go-ytdlp to %s...\n", tempOpus)

	dl := ytdlp.New().
		Format("bestaudio").
		ExtractAudio().
		AudioFormat("opus").
		AudioQuality("0").
		Output(tempOpus).
		NoPlaylist()

	_, err = dl.Run(ctx, "https://www.youtube.com/watch?v="+targetSong.ID)
	if err != nil {
		fmt.Printf("Download failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Download and conversion completed.")

	fmt.Println("Initializing StelleEngine...")
	engine, err := stelleengine.NewStelleEngine(0.5)
	if err != nil {
		fmt.Printf("Failed to initialize StelleEngine: %v\n", err)
		os.Remove(tempOpus)
		os.Exit(1)
	}
	defer engine.Close()

	fmt.Printf("Playing audio: %s\n", tempOpus)
	err = engine.Play(tempOpus, 0.0, 50)
	if err != nil {
		fmt.Printf("Playback failed: %v\n", err)
		os.Remove(tempOpus)
		os.Exit(1)
	}

	fmt.Println("Playing audio. Press Ctrl+C to stop.")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-sigChan:
			fmt.Println("\nStopping playback...")
			engine.Stop()
			os.Remove(tempOpus)
			return
		case <-ticker.C:
			pos := engine.GetPosition()
			dur := engine.GetDuration()
			state := engine.GetState()
			fmt.Printf("\rPlayback Progress: %.1f / %.1f seconds (State: %v)", pos, dur, state)
			if state == AudioEngine.StateStopped && pos >= dur && dur > 0 {
				fmt.Println("\nPlayback completed.")
				os.Remove(tempOpus)
				return
			}
		}
	}
}

func getArtistNames(artists []ytm.Artist) string {
	var names []string
	for _, a := range artists {
		names = append(names, a.Name)
	}
	return strings.Join(names, ", ")
}
