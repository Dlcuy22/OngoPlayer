// internal/ui/gio/controls.go
// Playback controls with Material Design icons: Rewind, Previous, Play/Pause, Next, FastForward.
//
// Dependencies:
//   - gioui.org: layout, widget, material
//   - golang.org/x/exp/shiny/materialdesign/icons: icon data

package gio

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	AudioEngine "github.com/dlcuy22/OngoPlayer/Audioengine"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

var (
	iconRewind  = mustIcon(icons.AVFastRewind)
	iconPrev    = mustIcon(icons.AVSkipPrevious)
	iconPlay    = mustIcon(icons.AVPlayArrow)
	iconPause   = mustIcon(icons.AVPause)
	iconNext    = mustIcon(icons.AVSkipNext)
	iconForward = mustIcon(icons.AVFastForward)
)

func mustIcon(data []byte) *widget.Icon {
	ic, err := widget.NewIcon(data)
	if err != nil {
		panic(err)
	}
	return ic
}

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
Layout renders the playback control buttons with Material icons.

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

	playIcon := iconPlay
	playDesc := "Play"
	state := c.player.Engine.GetState()
	if state == AudioEngine.StatePlaying {
		playIcon = iconPause
		playDesc = "Pause"
	}

	return layout.Inset{Top: 4, Bottom: 12, Left: 16, Right: 16}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{
				Axis:      layout.Horizontal,
				Spacing:   layout.SpaceEvenly,
				Alignment: layout.Middle,
			}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.IconButton(th, &c.seekLeft, iconRewind, "Rewind 5s")
					btn.Size = unit.Dp(22)
					btn.Color = ColorTextDim
					btn.Background = ColorSurface
					return btn.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.IconButton(th, &c.btnPrev, iconPrev, "Previous")
					btn.Size = unit.Dp(26)
					btn.Color = ColorText
					btn.Background = ColorSurface
					return btn.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.IconButton(th, &c.btnPlay, playIcon, playDesc)
					btn.Size = unit.Dp(32)
					btn.Color = ColorBg
					btn.Background = ColorAccent
					return btn.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.IconButton(th, &c.btnNext, iconNext, "Next")
					btn.Size = unit.Dp(26)
					btn.Color = ColorText
					btn.Background = ColorSurface
					return btn.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.IconButton(th, &c.seekRight, iconForward, "Forward 5s")
					btn.Size = unit.Dp(22)
					btn.Color = ColorTextDim
					btn.Background = ColorSurface
					return btn.Layout(gtx)
				}),
			)
		},
	)
}
