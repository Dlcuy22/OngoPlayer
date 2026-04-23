package main

import (
	"fmt"
	"os"

	stelleengine "github.com/dlcuy22/OngoPlayer/Audioengine/StelleEngine"
)

func main() {
	engine := stelleengine.NewStelleEngine()
	
	done := make(chan struct{})
	engine.SetOnComplete(func() {
		fmt.Println("Playback finished!")
		close(done)
	})

	filename := "Kokoroyohou.opus"
	fmt.Printf("Playing %s...\n", filename)
	
	err := engine.Play(filename, 0, 100)
	if err != nil {
		fmt.Printf("Error playing file: %v\n", err)
		os.Exit(1)
	}

	<-done
	fmt.Println("Exiting")
}
