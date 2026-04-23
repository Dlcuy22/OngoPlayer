// internal/ui/gio/controls.go
// Playback controls: Previous, Play/Pause, Next, and seek buttons.

package gio

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	AudioEngine "github.com/dlcuy22/OngoPlayer/Audioengine"
)

type Controls struct {
	player    *Player
	btnPrev   widget.Clickable
	btnPlay   widget.Clickable
	btnNext   widget.Clickable
	seekLeft  widget.Clickable
	seekRight widget.Clickable
}

/*
NewControls creates a new controls panel.

	params:
	      player: player state
	returns:
	      *Controls
*/
func NewControls(player *Player) *Controls {
	return &Controls{player: player}
}

/*
LayoutControls renders the playback control buttons.

	params:
	      gtx: layout context
	      th:  material theme
	returns:
	      layout.Dimensions
*/
func (c *Controls) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if c.btnPrev.Clicked(gtx) {
		c.player.Prev()
	}
	if c.btnPlay.Clicked(gtx) {
		c.player.TogglePause()
	}
	if c.btnNext.Clicked(gtx) {
		c.player.Next()
	}
	if c.seekLeft.Clicked(gtx) {
		pos := c.player.Engine.GetPosition()
		newPos := pos - 5
		if newPos < 0 {
			newPos = 0
		}
		_ = c.player.Engine.Seek(newPos, c.player.Volume)
	}
	if c.seekRight.Clicked(gtx) {
		pos := c.player.Engine.GetPosition()
		_ = c.player.Engine.Seek(pos+5, c.player.Volume)
	}
	// Play Pause label, currently using unicode characters (emojis)
	playLabel := "▶"
	state := c.player.Engine.GetState()
	if state == AudioEngine.StatePlaying {
		playLabel = "⏸"
	}
	// UI CONTROL BOTTOM
	return layout.Inset{Top: 4, Bottom: 12, Left: 16, Right: 16}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{
				Axis:    layout.Horizontal,
				Spacing: layout.SpaceEvenly,
			}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.Button(th, &c.seekLeft, "<-5s")
					btn.Color = ColorTextDim
					btn.Background = ColorSurface
					return btn.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.Button(th, &c.btnPrev, "Prev")
					btn.Color = ColorText
					btn.Background = ColorSurface
					return btn.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.Button(th, &c.btnPlay, playLabel)
					btn.Color = ColorText
					btn.Background = ColorSurface
					return btn.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.Button(th, &c.btnNext, "Next")
					btn.Color = ColorText
					btn.Background = ColorSurface
					return btn.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.Button(th, &c.seekRight, "+5s->")
					btn.Color = ColorTextDim
					btn.Background = ColorSurface
					return btn.Layout(gtx)
				}),
			)
		},
	)
}
