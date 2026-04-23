package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	stelleengine "github.com/dlcuy22/OngoPlayer/Audioengine/StelleEngine"
	utils "github.com/dlcuy22/OngoPlayer/internal/utils"
	"golang.org/x/term"
)

const (
	barWidth = 30
	volume   = 100
)

var supportedExts = map[string]bool{
	".opus": true,
	".mp3":  true,
	".ogg":  true,
	".oga":  true,
}

func main() {
	art := `
▄▖      ▄▖▜         
▌▌▛▌▛▌▛▌▙▌▐ ▀▌▌▌█▌▛▘
▙▌▌▌▙▌▙▌▌ ▐▖█▌▙▌▙▖▌ 
    ▄▌        ▄▌    
	`
	fmt.Println(art)

	var folder string
	fmt.Print("Enter folder to play: ")
	fmt.Scan(&folder)

	queue, err := utils.ScanFolder(folder, supportedExts)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if len(queue) == 0 {
		fmt.Println("No supported audio files found in folder.")
		os.Exit(1)
	}
	fmt.Printf("Found %d tracks.\n\n", len(queue))

	engine := stelleengine.NewStelleEngine(float32(volume) / 100.0)

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Printf("Warning: could not set raw mode: %v\n", err)
	} else {
		defer term.Restore(int(os.Stdin.Fd()), oldState)
	}

	trackDone := make(chan struct{}) // fired by engine.SetOnComplete
	skip := make(chan struct{}, 1)   // fired by 'n' key
	quit := make(chan struct{})      // fired by 'q' / Ctrl+C

	buf := make([]byte, 3)
	go func() {
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				continue
			}
			switch {
			case n == 3 && buf[0] == 0x1b && buf[1] == '[' && buf[2] == 'C': // →
				pos := engine.GetPosition()
				_ = engine.Seek(pos+5, volume)

			case n == 3 && buf[0] == 0x1b && buf[1] == '[' && buf[2] == 'D': // ←
				pos := engine.GetPosition()
				if newPos := pos - 5; newPos >= 0 {
					_ = engine.Seek(newPos, volume)
				} else {
					_ = engine.Seek(0, volume)
				}

			case n == 1 && buf[0] == ' ':
				if engine.GetState() == 1 {
					_ = engine.Pause()
				} else {
					_ = engine.Resume(engine.GetPosition(), volume)
				}

			case n == 1 && buf[0] == 'n': // next track
				select {
				case skip <- struct{}{}:
				default:
				}

			case n == 1 && (buf[0] == 'q' || buf[0] == 3): // quit
				engine.Stop()
				close(quit)
				return
			}
		}
	}()

	fmt.Println("Controls: → +5s  ← -5s  space pause/resume  n next  q quit")
	fmt.Println()

outer:
	for i, path := range queue {
		// Reset trackDone for each track
		trackDone = make(chan struct{})
		engine.SetOnComplete(func() {
			close(trackDone)
		})

		fmt.Printf("\r\033[K[%d/%d] %s\n", i+1, len(queue), filepath.Base(path))

		if err := engine.Play(path, 0, volume); err != nil {
			fmt.Printf("Error playing %s: %v, skipping\n", filepath.Base(path), err)
			continue
		}

		// Progress bar for this track
		barDone := make(chan struct{})
		go func() {
			ticker := time.NewTicker(300 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-barDone:
					return
				case <-ticker.C:
					fmt.Print(utils.RenderBar(engine.GetPosition(), engine.GetDuration(), barWidth))
				}
			}
		}()

		select {
		case <-trackDone:

		case <-skip:
			engine.Stop()
		case <-quit:
			close(barDone)
			break outer
		}

		close(barDone)
		fmt.Println()
	}

	fmt.Println("\nBye.")
}
