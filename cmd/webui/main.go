package main

import (
	"embed"
	"os"
	"os/signal"
	"syscall"

	"github.com/dlcuy22/OngoPlayer/internal/logging"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var mainLog = logging.NewLogger("main")

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Listen for SIGINT / SIGTERM signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		mainLog.Info("Received signal, initiating graceful shutdown...", "signal", sig.String())
		
		app.CancelAllDownloads()
		
		if app.engine != nil {
			app.engine.Close()
		}
		
		app.mu.Lock()
		ctx := app.ctx
		app.mu.Unlock()
		
		if ctx != nil {
			runtime.Quit(ctx)
		} else {
			os.Exit(0)
		}
	}()

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "Ongoplayer-webui",
		Width:     1100,
		Height:    720,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
