// internal/ui/gio/app.go
// Main application window and event loop.
// Layout: thin header bar at top, then two-panel layout below:
// fixed-width sidebar (playlist) on the left, main panel on the right.
//
// Dependencies:
//   - gioui.org: app, layout, op, paint, clip
//   - internal/service/lyrics: synced lyrics

package gio

import (
	"fmt"
	"image"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"

	AudioEngine "github.com/dlcuy22/OngoPlayer/Audioengine"
	"github.com/dlcuy22/OngoPlayer/internal/service/discordrpc"
	"github.com/dlcuy22/OngoPlayer/internal/service/lyrics"
	"github.com/dlcuy22/OngoPlayer/internal/shared"
)

const sidebarWidthDp = 240

type App struct {
	window     *app.Window
	theme      *material.Theme
	player     *Player
	header     *Header
	trackList  *TrackList
	nowPlaying *NowPlaying
	playerBar  *PlayerBar
	rpc        *discordrpc.Manager
	EnableRPC  bool
}

/*
NewApp initializes a new OngoPlayer application window.

	params:
	      player: pointer to the initialized Player instance
	returns:
	      *App
*/
func NewApp(player *Player) *App {
	w := new(app.Window)
	w.Option(
		app.Title("OngoPlayer"),
		app.Size(730, 650),
		app.Decorated(false), // disable window decorations, TODO: create custom ones for dragging/resizing/close/minimize
	)

	th := NewTheme()

	a := &App{
		window:     w,
		theme:      th,
		player:     player,
		header:     NewHeader(player, w),
		trackList:  NewTrackList(player),
		nowPlaying: NewNowPlaying(player),
		playerBar:  NewPlayerBar(player),
	}

	player.OnUpdate = func() {
		w.Invalidate()
	}

	player.OnTrackChange = func(track TrackMeta) {
		a.nowPlaying.SetLoading(track.Path)
		a.window.Invalidate()
		go a.loadLyrics(track)
		go a.updateRPC(track)
	}

	if len(player.Queue) > 0 {
		track := player.Queue[player.Current]
		a.nowPlaying.SetLoading(track.Path)
		go a.loadLyrics(track)
	}

	return a
}

/*
loadLyrics attempts to load lyrics for the given track, first from a local file, then from an API.

	params:
	      track: metadata of the track to load lyrics for
*/
func (a *App) loadLyrics(track TrackMeta) {
	musicDir := a.player.MusicDir

	if shared.Debug {
		fmt.Println("[DEBUG][lyrics] > track changed")
		fmt.Printf("[DEBUG][lyrics]   title:  %q\n", track.Title)
		fmt.Printf("[DEBUG][lyrics]   artist: %q\n", track.Artist)
		fmt.Printf("[DEBUG][lyrics]   album:  %q\n", track.Album)
		fmt.Printf("[DEBUG][lyrics]   path:   %s\n", track.Path)
		fmt.Printf("[DEBUG][lyrics]   dir:    %s\n", musicDir)
	}

	if shared.Debug {
		fmt.Println("[DEBUG][lyrics] trying local .lrc file...")
	}
	if lr, ok := lyrics.LoadFromFile(track.Path, musicDir); ok {
		if shared.Debug {
			fmt.Printf("[DEBUG][lyrics] loaded from local file: %d lines\n", len(lr.Lines))
		}
		a.nowPlaying.SetLyrics(lr.Lines, track.Path)
		a.window.Invalidate()
		return
	}

	if shared.Debug {
		fmt.Println("[DEBUG][lyrics] no local file found")
	}

	if track.Artist != "" && track.Title != "" {
		if shared.Debug {
			fmt.Printf("[DEBUG][lyrics] fetching from lrclib.net (artist=%q, title=%q, album=%q)\n",
				track.Artist, track.Title, track.Album)
		}
		content, err := lyrics.FetchFromAPI(track.Artist, track.Title, track.Album, a.player.Engine.GetDuration())
		if err == nil && content != "" {
			lines := lyrics.Parse(content)
			if shared.Debug {
				fmt.Printf("[DEBUG][lyrics] API returned %d bytes, parsed %d lines\n", len(content), len(lines))
			}

			savePath, saveErr := lyrics.SaveToFile(track.Path, musicDir, content)
			if shared.Debug {
				if saveErr != nil {
					fmt.Printf("[DEBUG][lyrics] save failed: %v\n", saveErr)
				} else {
					fmt.Printf("[DEBUG][lyrics] saved to: %s\n", savePath)
				}
			}

			a.nowPlaying.SetLyrics(lines, track.Path)
			a.window.Invalidate()
			return
		}

		if shared.Debug && err != nil {
			fmt.Printf("[DEBUG][lyrics] API error: %v\n", err)
		}
	} else if shared.Debug {
		fmt.Println("[DEBUG][lyrics] skipping API fetch: missing artist or title")
	}

	if shared.Debug {
		fmt.Println("[DEBUG][lyrics] no lyrics found, clearing panel")
	}
	a.nowPlaying.ClearLyrics(track.Path)
	a.window.Invalidate()
}

/*
Run starts the main application event loop.

	returns:
	      error: any error encountered during execution
*/
func (a *App) Run() error {
	if a.EnableRPC {
		a.rpc = discordrpc.New()
		a.rpc.GetPosition = func() float64 {
			return a.player.Engine.GetPosition()
		}
		a.rpc.IsPaused = func() bool {
			return a.player.Engine.GetState() == AudioEngine.StatePaused
		}
		a.rpc.Start()

		// Send initial track info so the presence appears immediately
		if len(a.player.Queue) > 0 && a.player.Current >= 0 {
			go a.updateRPC(a.player.Queue[a.player.Current])
		}
	}

	go func() {
		ticker := time.NewTicker(33 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			a.window.Invalidate()
		}
	}()

	var ops op.Ops

	for {
		switch e := a.window.Event().(type) {
		case app.DestroyEvent:
			if a.rpc != nil {
				a.rpc.Stop()
			}
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			a.layout(gtx)
			e.Frame(gtx.Ops)

		case app.ConfigEvent:
			a.header.SetMaximized(e.Config.Mode == app.Maximized)
		}
	}
}

/*
updateRPC sends the current track metadata to the Discord RPC manager.
Skipped silently if RPC is disabled.

	params:
	      track: metadata of the currently playing track
*/
func (a *App) updateRPC(track TrackMeta) {
	if a.rpc == nil {
		if shared.Debug {
			fmt.Println("[DEBUG][rpc] skipped: rpc manager is nil")
		}
		return
	}

	dur := a.player.Engine.GetDuration()
	pos := a.player.Engine.GetPosition()
	if shared.Debug {
		fmt.Printf("[DEBUG][rpc] sending update: title=%q artist=%q elapsed=%.1fs duration=%.1fs\n",
			track.Title, track.Artist, pos, dur)
	}

	a.rpc.Update(discordrpc.TrackInfo{
		Title:       track.Title,
		Artist:      track.Artist,
		Album:       track.Album,
		Cover:       track.Thumb,
		DurationSec: dur,
		ElapsedSec:  pos,
		IsPaused:    a.player.Engine.GetState() == AudioEngine.StatePaused,
	})
}

/*
layout defines the main UI structure of the application.

	params:
	      gtx: layout context
	returns:
	      layout.Dimensions
*/
func (a *App) layout(gtx layout.Context) layout.Dimensions {
	bgSize := image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	paint.FillShape(gtx.Ops, ColorBg, clip.Rect{Max: bgSize}.Op())

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Header bar (top, spans full width)
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.header.Layout(gtx, a.theme)
		}),

		// Divider under header
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return LayoutDivider(gtx)
		}),

		// Body: sidebar + divider + main panel
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			sidebarW := gtx.Dp(unit.Dp(sidebarWidthDp))
			mainW := gtx.Constraints.Max.X - sidebarW - gtx.Dp(unit.Dp(1))

			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				// Sidebar
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = sidebarW
					gtx.Constraints.Max.X = sidebarW
					return a.trackList.Layout(gtx, a.theme)
				}),

				// Vertical divider
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return LayoutVerticalDivider(gtx)
				}),

				// Main panel
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = mainW
					gtx.Constraints.Max.X = mainW
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.nowPlaying.Layout(gtx, a.theme)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.playerBar.Layout(gtx, a.theme)
						}),
					)
				}),
			)
		}),
	)
}
