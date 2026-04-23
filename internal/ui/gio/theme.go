// internal/ui/gio/theme.go
// Defines the dark theme constants and helper functions for the Gio UI.
//
// Dependencies:
//   - gioui.org: color, unit types

package gio

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

var (
	ColorBg        = color.NRGBA{R: 30, G: 30, B: 46, A: 255}   // #1e1e2e
	ColorSurface   = color.NRGBA{R: 42, G: 42, B: 62, A: 255}   // #2a2a3e
	ColorAccent    = color.NRGBA{R: 137, G: 180, B: 250, A: 255} // #89b4fa soft blue
	ColorText      = color.NRGBA{R: 205, G: 214, B: 244, A: 255} // #cdd6f4
	ColorTextDim   = color.NRGBA{R: 147, G: 153, B: 178, A: 255} // #9399b2
	ColorBar       = color.NRGBA{R: 69, G: 71, B: 90, A: 255}    // #45475a
	ColorBarFilled = color.NRGBA{R: 137, G: 180, B: 250, A: 255} // #89b4fa
	ColorHover     = color.NRGBA{R: 49, G: 50, B: 68, A: 255}    // #313244
	ColorSelected  = color.NRGBA{R: 59, G: 60, B: 80, A: 255}    // #3b3c50
)

/*
NewTheme creates a material theme with the dark color palette.

	returns:
	      *material.Theme
*/
func NewTheme() *material.Theme {
	th := material.NewTheme()
	th.Palette.Bg = ColorBg
	th.Palette.Fg = ColorText
	th.Palette.ContrastBg = ColorAccent
	th.Palette.ContrastFg = ColorBg
	th.TextSize = 14
	return th
}

/*
LabelStyle creates a styled label with specified size and color.

	params:
	      th:    theme
	      size:  font size in sp
	      text:  label text
	      col:   text color
	returns:
	      material.LabelStyle
*/
func LabelStyle(th *material.Theme, size float32, txt string, col color.NRGBA) material.LabelStyle {
	l := material.Label(th, unit.Sp(size), txt)
	l.Color = col
	return l
}

/*
BoldLabel creates a bold label with specified size and color.

	params:
	      th:    theme
	      size:  font size in sp
	      text:  label text
	      col:   text color
	returns:
	      material.LabelStyle
*/
func BoldLabel(th *material.Theme, size float32, txt string, col color.NRGBA) material.LabelStyle {
	l := material.Label(th, unit.Sp(size), txt)
	l.Color = col
	l.Font.Weight = font.Normal
	l.MaxLines = 1
	l.Truncator = "..."
	return l
}

/*
DimLabel creates a dim secondary label.

	params:
	      th:   theme
	      size: font size in sp
	      text: label text
	returns:
	      material.LabelStyle
*/
func DimLabel(th *material.Theme, size float32, txt string) material.LabelStyle {
	l := material.Label(th, unit.Sp(size), txt)
	l.Color = ColorTextDim
	l.MaxLines = 1
	l.Truncator = "..."
	l.Alignment = text.Start
	return l
}
