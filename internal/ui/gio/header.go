// internal/ui/gio/header.go
// Header bar with player controls and window frame buttons.
// Includes volume slider, settings button, and window management (minimize, maximize, close).
//
// Dependencies:
//   - gioui.org: layout, widget, op, io/system
//   - golang.org/x/exp/shiny: material design icons

package gio

import (
	"image"
	"image/color"

	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"golang.org/x/exp/shiny/materialdesign/icons"
)

var (
	iconSettings = mustIcon(icons.ActionSettings)
	iconVolume   = mustIcon(icons.AVVolumeUp)
	iconClose    = mustIcon(icons.NavigationClose)
	iconMinimize = mustIcon(icons.ContentRemove)
	iconMaximize = mustIcon(icons.NavigationFullscreen)
	iconRestore  = mustIcon(icons.NavigationFullscreenExit)
)

/*
Header represents the application header bar with volume control and window frame buttons.
*/
type Header struct {
	player      *Player
	window      interface{ Perform(system.Action) }
	volSlider   widget.Float
	btnSettings widget.Clickable
	btnClose    widget.Clickable
	btnMinimize widget.Clickable
	btnMaximize widget.Clickable
	maximized   bool
}

/*
SetMaximized updates the maximized state of the window header.

	params:
	      m: true if window is maximized, false otherwise
*/
func (h *Header) SetMaximized(m bool) {
	h.maximized = m
}

/*
NewHeader initializes a new Header instance with the given player and window.

	params:
	      player: pointer to the Player instance
	      w: window interface for system actions
	returns:
	      *Header
*/
func NewHeader(player *Player, w interface{ Perform(system.Action) }) *Header {
	h := &Header{
		player: player,
		window: w,
	}
	h.volSlider.Value = float32(player.Volume) / 100.0
	return h
}

/*
winBtn renders a styled window control button (minimize, maximize, close).

	params:
	      gtx: layout context
	      btn: clickable widget state
	      icon: icon to display in the button
	      iconColor: color of the icon
	      hoverColor: background color when button is hovered
	returns:
	      layout.Dimensions
*/
func (h *Header) winBtn(
	gtx layout.Context,
	btn *widget.Clickable,
	icon *widget.Icon,
	iconColor color.NRGBA,
	hoverColor color.NRGBA,
) layout.Dimensions {
	btnW := gtx.Dp(unit.Dp(28))
	btnH := gtx.Dp(unit.Dp(22))
	sz := image.Pt(btnW, btnH)

	gtx.Constraints.Min = sz
	gtx.Constraints.Max = sz

	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if btn.Hovered() {
			rr := clip.UniformRRect(image.Rectangle{Max: sz}, gtx.Dp(unit.Dp(4)))
			paint.FillShape(gtx.Ops, hoverColor, rr.Op(gtx.Ops))
		}

		iconSz := gtx.Dp(unit.Dp(25))
		gtx.Constraints.Min = image.Pt(iconSz, iconSz)
		gtx.Constraints.Max = image.Pt(iconSz, iconSz)

		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return icon.Layout(gtx, iconColor)
		})
	})
}

/*
Layout renders the header bar with volume control and window controls.

	params:
	      gtx: layout context
	      th: material theme for styling
	returns:
	      layout.Dimensions
*/
func (h *Header) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	// Volume sync
	if h.volSlider.Update(gtx) {
		h.player.SetVolume(int(h.volSlider.Value * 100.0))
	}
	if !h.volSlider.Dragging() {
		h.volSlider.Value = float32(h.player.Volume) / 100.0
	}

	// Window control events
	for h.btnClose.Clicked(gtx) {
		h.window.Perform(system.ActionClose)
	}
	for h.btnMinimize.Clicked(gtx) {
		h.window.Perform(system.ActionMinimize)
	}
	for h.btnMaximize.Clicked(gtx) {
		if h.maximized {
			h.window.Perform(system.ActionUnmaximize)
		} else {
			h.window.Perform(system.ActionMaximize)
		}
	}

	headerHeight := gtx.Dp(unit.Dp(40))
	bgSize := image.Pt(gtx.Constraints.Max.X, headerHeight)
	paint.FillShape(gtx.Ops, ColorSidebar, clip.Rect{Max: bgSize}.Op())

	gtx.Constraints.Min.Y = headerHeight
	gtx.Constraints.Max.Y = headerHeight

	return layout.Inset{Left: 12, Right: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{
			Axis:      layout.Horizontal,
			Alignment: layout.Middle,
		}.Layout(gtx,
			// Drag region, fills all unused space
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				sz := image.Pt(gtx.Constraints.Max.X, headerHeight)
				defer clip.Rect{Max: sz}.Push(gtx.Ops).Pop()
				system.ActionInputOp(system.ActionMove).Add(gtx.Ops)
				return layout.Dimensions{Size: sz}
			}),

			// Volume icon
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				sz := gtx.Dp(unit.Dp(14))
				gtx.Constraints.Min = image.Point{X: sz, Y: sz}
				gtx.Constraints.Max = image.Point{X: sz, Y: sz}
				return iconVolume.Layout(gtx, ColorTextDim)
			}),

			// Volume slider
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Dp(unit.Dp(80))
				gtx.Constraints.Max.X = gtx.Dp(unit.Dp(80))
				return layout.Inset{Left: 6, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					slider := material.Slider(th, &h.volSlider)
					slider.Color = ColorAccent
					return slider.Layout(gtx)
				})
			}),

			// Settings
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				btn := material.IconButton(th, &h.btnSettings, iconSettings, "Settings")
				btn.Size = unit.Dp(16)
				btn.Color = ColorTextDim
				btn.Background = color.NRGBA{A: 0}
				btn.Inset = layout.UniformInset(unit.Dp(6))
				return btn.Layout(gtx)
			}),

			// Minimize button
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return h.winBtn(gtx, &h.btnMinimize, iconMinimize,
						ColorTextDim,
						color.NRGBA{R: 255, G: 255, B: 255, A: 18},
					)
				})
			}),

			// Maximize/Restore button
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				icon := iconMaximize
				if h.maximized {
					icon = iconRestore
				}
				return h.winBtn(gtx, &h.btnMaximize, icon,
					ColorTextDim,
					color.NRGBA{R: 255, G: 255, B: 255, A: 18},
				)
			}),

			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return h.winBtn(gtx, &h.btnClose, iconClose,
					color.NRGBA{R: 255, G: 80, B: 80, A: 255},
					color.NRGBA{R: 255, G: 80, B: 80, A: 45},
				)
			}),
		)
	})
}
