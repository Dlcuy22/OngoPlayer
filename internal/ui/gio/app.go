// internal/ui/gio/app.go
// Main application window and event loop.
// Composes the track list, now-playing panel, and controls into a single window.
//
// Dependencies:
//   - gioui.org: app, layout, op, paint

package gio

import (
	"image"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget/material"
)

type App struct {
	window     *app.Window
	theme      *material.Theme
	player     *Player
	trackList  *TrackList
	nowPlaying *NowPlaying
	controls   *Controls
}

/*
NewApp creates a new Gio application.

	params:
	      player: player state
	returns:
	      *App
*/
func NewApp(player *Player) *App {
	w := new(app.Window)
	w.Option(
		app.Title("DongoPlayer"),
		app.Size(800, 600),
	)

	th := NewTheme()

	a := &App{
		window:     w,
		theme:      th,
		player:     player,
		trackList:  NewTrackList(player),
		nowPlaying: NewNowPlaying(player),
		controls:   NewControls(player),
	}

	player.OnUpdate = func() {
		w.Invalidate()
	}

	return a
}

/*
Run starts the Gio event loop. Blocks until the window is closed.
*/
func (a *App) Run() error {
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			a.window.Invalidate()
		}
	}()

	var ops op.Ops

	for {
		switch e := a.window.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			a.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func (a *App) layout(gtx layout.Context) layout.Dimensions {
	bgSize := image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	bgR := clip.Rect{Max: bgSize}
	paint.FillShape(gtx.Ops, ColorBg, bgR.Op())

	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.trackList.Layout(gtx, a.theme)
		}),

		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return LayoutDivider(gtx)
		}),

		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.nowPlaying.Layout(gtx, a.theme)
		}),

		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return LayoutDivider(gtx)
		}),

		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.controls.Layout(gtx, a.theme)
		}),
	)
}
