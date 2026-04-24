// internal/ui/gio/header.go
// Top header bar with icon buttons for settings, lyrics toggle, and app title.
//
// Dependencies:
//   - gioui.org: layout, widget, material
//   - golang.org/x/exp/shiny/materialdesign/icons

package gio

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"golang.org/x/exp/shiny/materialdesign/icons"
)

var (
	iconSettings  = mustIcon(icons.ActionSettings)
	iconLyrics    = mustIcon(icons.AVQueueMusic)
	iconChevLeft  = mustIcon(icons.NavigationChevronLeft)
	iconChevRight = mustIcon(icons.NavigationChevronRight)
	iconVolume    = mustIcon(icons.AVVolumeUp)
)

type Header struct {
	player      *Player
	btnSettings widget.Clickable
	btnLyrics   widget.Clickable
	volSlider   widget.Float
	lyricsOpen  *bool
}

/*
NewHeader creates a new header bar.

	params:
	      lyricsOpen: pointer to the lyrics panel open state
	returns:
	      *Header
*/
func NewHeader(player *Player, lyricsOpen *bool) *Header {
	h := &Header{
		player:     player,
		lyricsOpen: lyricsOpen,
	}
	h.volSlider.Value = float32(player.Volume) / 100.0
	return h
}

/*
Layout renders the header bar.

	params:
	      gtx: layout context
	      th:  material theme
	returns:
	      layout.Dimensions
*/
func (h *Header) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if h.btnLyrics.Clicked(gtx) {
		*h.lyricsOpen = !*h.lyricsOpen
	}

	headerHeight := gtx.Dp(unit.Dp(36))
	bgSize := image.Pt(gtx.Constraints.Max.X, headerHeight)
	bgR := clip.Rect{Max: bgSize}
	paint.FillShape(gtx.Ops, ColorSurface, bgR.Op())

	gtx.Constraints.Min.Y = headerHeight
	gtx.Constraints.Max.Y = headerHeight

	return layout.Inset{Left: 8, Right: 12}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{
				Axis:      layout.Horizontal,
				Alignment: layout.Middle,
			}.Layout(gtx,
				// Settings button
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.IconButton(th, &h.btnSettings, iconSettings, "Settings")
					btn.Size = unit.Dp(18)
					btn.Color = ColorTextDim
					btn.Background = ColorSurface
					btn.Inset = layout.UniformInset(unit.Dp(6))
					return btn.Layout(gtx)
				}),

				// Lyrics toggle button
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					chevron := iconChevLeft
					if *h.lyricsOpen {
						chevron = iconChevRight
					}
					_ = chevron

					btn := material.IconButton(th, &h.btnLyrics, iconLyrics, "Lyrics")
					btn.Size = unit.Dp(18)
					btn.Color = ColorTextDim
					if *h.lyricsOpen {
						btn.Color = ColorAccent
					}
					btn.Background = ColorSurface
					btn.Inset = layout.UniformInset(unit.Dp(6))
					return btn.Layout(gtx)
				}),

				// Spacer
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
				}),

				// Volume Icon
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						sz := gtx.Dp(unit.Dp(18))
						gtx.Constraints.Min = image.Point{X: sz, Y: sz}
						gtx.Constraints.Max = image.Point{X: sz, Y: sz}
						return iconVolume.Layout(gtx, ColorTextDim)
					})
				}),

				// Volume Slider
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					// Read state from input
					if h.volSlider.Update(gtx) {
						vol := int(h.volSlider.Value * 100.0)
						h.player.SetVolume(vol)
					}
					// Sync slider visually when changed externally (but not while dragging)
					if !h.volSlider.Dragging() {
						h.volSlider.Value = float32(h.player.Volume) / 100.0
					}

					gtx.Constraints.Min.X = gtx.Dp(unit.Dp(80))
					gtx.Constraints.Max.X = gtx.Dp(unit.Dp(80))
					slider := material.Slider(th, &h.volSlider)
					slider.Color = ColorAccent
					return layout.Inset{Right: unit.Dp(12)}.Layout(gtx, slider.Layout)
				}),

				// App title
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return DimLabel(th, 20, "OngoPlayer").Layout(gtx)
				}),
			)
		},
	)
}
