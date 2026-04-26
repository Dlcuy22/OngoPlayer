// internal/ui/gio/header.go
// Thin top header bar with search field, volume control, and settings button.
//
// Dependencies:
//   - gioui.org: layout, widget, material, unit, clip, paint
//   - golang.org/x/exp/shiny/materialdesign/icons

package gio

import (
	"image"
	"image/color"

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
)

type Header struct {
	player      *Player
	volSlider   widget.Float
	btnSettings widget.Clickable
}

/*
NewHeader creates a new Header component.

	params:
	      player: active Player instance
	returns:
	      *Header
*/
func NewHeader(player *Player) *Header {
	h := &Header{player: player}
	h.volSlider.Value = float32(player.Volume) / 100.0
	return h
}

/*
Layout renders the header bar and handles input for its widgets.

	params:
	      gtx: layout context
	      th: material theme
	returns:
	      layout.Dimensions
*/
func (h *Header) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	// Volume slider state
	if h.volSlider.Update(gtx) {
		vol := int(h.volSlider.Value * 100.0)
		h.player.SetVolume(vol)
	}
	if !h.volSlider.Dragging() {
		h.volSlider.Value = float32(h.player.Volume) / 100.0
	}

	headerHeight := gtx.Dp(unit.Dp(40))
	bgSize := image.Pt(gtx.Constraints.Max.X, headerHeight)
	paint.FillShape(gtx.Ops, ColorSidebar, clip.Rect{Max: bgSize}.Op())

	gtx.Constraints.Min.Y = headerHeight
	gtx.Constraints.Max.Y = headerHeight

	return layout.Inset{Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{
			Axis:      layout.Horizontal,
			Alignment: layout.Middle,
		}.Layout(gtx,
			// Spacer
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
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

			// Settings button
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				btn := material.IconButton(th, &h.btnSettings, iconSettings, "Settings")
				btn.Size = unit.Dp(16)
				btn.Color = ColorTextDim
				btn.Background = color.NRGBA{A: 0}
				btn.Inset = layout.UniformInset(unit.Dp(6))
				return btn.Layout(gtx)
			}),
		)
	})
}
