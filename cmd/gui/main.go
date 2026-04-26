package main

import (
	"flag"
	"fmt"
	"os"

	stelleengine "github.com/dlcuy22/OngoPlayer/Audioengine/StelleEngine"
	"github.com/dlcuy22/OngoPlayer/internal/shared"
	ui "github.com/dlcuy22/OngoPlayer/internal/ui/gio"
)

func main() {
	debug := flag.Bool("debug", false, "enable debug logging")
	songfolder := flag.String("playlist", "", "Path to playlist folder")
	enableRPC := flag.Bool("rpc", false, "enable Discord Rich Presence")
	flag.Parse()
	shared.Debug = *debug

	var folder string
	var err error
	if *songfolder == "" {
		folder, err = shared.PickFolder()
		if err != nil {
			fmt.Println("error while picking folder", err)
			return
		}
	} else {
		folder = *songfolder
	}

	args := flag.Args()
	if len(args) > 0 {
		folder = args[0]
	}

	engine := stelleengine.NewStelleEngine(1.0)
	player := ui.NewPlayer(engine, 25)

	if err := player.LoadFolder(folder); err != nil {
		fmt.Println("Error loading folder:", err)
		os.Exit(1)
	}

	if len(player.Queue) == 0 {
		fmt.Println("No audio files found in", folder)
		os.Exit(1)
	}

	fmt.Printf("Loaded %d tracks from %s\n", len(player.Queue), folder)

	if shared.Debug {
		fmt.Println("[DEBUG] Debug mode enabled")
		fmt.Printf("[DEBUG] MusicDir: %s\n", player.MusicDir)
	}

	player.PlayTrack(0)

	app := ui.NewApp(player)
	app.EnableRPC = *enableRPC
	if err := app.Run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
