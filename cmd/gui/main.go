package main

import (
	"fmt"
	"os"

	stelleengine "github.com/dlcuy22/OngoPlayer/Audioengine/StelleEngine"
	shared "github.com/dlcuy22/OngoPlayer/internal/shared"
	ui "github.com/dlcuy22/OngoPlayer/internal/ui/gio"
)

func main() {
	folder, err := shared.PickFolder()
	if err != nil {
		fmt.Println("Error picking folder:", err)
		os.Exit(1)
	}
	if len(os.Args) > 1 {
		folder = os.Args[1]
	}

	engine := stelleengine.NewStelleEngine(1.0)
	player := ui.NewPlayer(engine, 100)

	if err := player.LoadFolder(folder); err != nil {
		fmt.Println("Error loading folder:", err)
		os.Exit(1)
	}

	if len(player.Queue) == 0 {
		fmt.Println("No audio files found in", folder)
		os.Exit(1)
	}

	fmt.Printf("Loaded %d tracks from %s\n", len(player.Queue), folder)

	// Play the first track on startup
	player.PlayTrack(0)

	app := ui.NewApp(player)
	if err := app.Run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
