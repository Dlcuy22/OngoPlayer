// internal/ui/gio/theme.go
// Dark theme palette and helper label functions for the redesigned UI.
// Embeds and loads NotoSansJP-Bold for CJK lyrics rendering.
//
// Palette based on the mockup: very dark backgrounds (#0e0e12, #111116),
// soft white text, and a purple accent (#b8a0f0) as default.
//
// Dependencies:
//   - gioui.org: color, unit, text, font, font/opentype, widget/material

package gio

import (
	_ "embed"
	"image/color"
	"log"

	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

//go:embed fonts/NotoSansJP-Bold.ttf
var notoSansJPBold []byte

var (
	ColorBg      = color.NRGBA{R: 14, G: 14, B: 18, A: 255}   // #0e0e12 main background
	ColorSidebar = color.NRGBA{R: 17, G: 17, B: 22, A: 255}   // #111116 sidebar background
	ColorSurface = color.NRGBA{R: 26, G: 26, B: 34, A: 255}   // #1a1a22 hover/card states
	ColorAccent  = color.NRGBA{R: 184, G: 160, B: 240, A: 255} // #b8a0f0 purple accent
	ColorText    = color.NRGBA{R: 224, G: 224, B: 224, A: 255} // rgba(255,255,255,0.88)
	ColorTextDim = color.NRGBA{R: 97, G: 97, B: 97, A: 255}   // rgba(255,255,255,0.38)
	ColorDivider = color.NRGBA{R: 255, G: 255, B: 255, A: 15}  // rgba(255,255,255,0.06)
	ColorBar     = color.NRGBA{R: 255, G: 255, B: 255, A: 26}  // rgba(255,255,255,0.1)
	ColorWhite   = color.NRGBA{R: 255, G: 255, B: 255, A: 255} // pure white for play btn
	ColorDark    = color.NRGBA{R: 14, G: 14, B: 18, A: 255}    // dark icon on white btn
)

/*
NewTheme creates and structures the material design colors and font defaults.

	returns:
	      *material.Theme
*/
func NewTheme() *material.Theme {
	th := material.NewTheme()
	th.Palette.Bg = ColorBg
	th.Palette.Fg = ColorText
	th.Palette.ContrastBg = ColorAccent
	th.Palette.ContrastFg = ColorBg
	th.TextSize = 13

	faces, err := opentype.ParseCollection(notoSansJPBold)
	if err != nil {
		log.Printf("[WARN] failed to parse NotoSansJP-Bold: %v", err)
		return th
	}

	th.Shaper = text.NewShaper(text.WithCollection(faces))
	return th
}

/*
LabelStyle returns a basic configured text label.

	params:
	      th: material theme instance
	      size: text scale in SP 
	      txt: literal string to render
	      col: NRGBA text color
	returns:
	      material.LabelStyle
*/
func LabelStyle(th *material.Theme, size float32, txt string, col color.NRGBA) material.LabelStyle {
	l := material.Label(th, unit.Sp(size), txt)
	l.Color = col
	return l
}

/*
BoldLabel returns a font-weighted text label.

	params:
	      th: material theme instance
	      size: text scale in SP 
	      txt: literal string to render
	      col: NRGBA text color
	returns:
	      material.LabelStyle
*/
func BoldLabel(th *material.Theme, size float32, txt string, col color.NRGBA) material.LabelStyle {
	l := material.Label(th, unit.Sp(size), txt)
	l.Color = col
	l.Font.Weight = font.Bold
	l.MaxLines = 1
	l.Truncator = "..."
	return l
}

/*
DimLabel returns a dimly lit text label, suited for subtexts.

	params:
	      th: material theme instance
	      size: text scale in SP 
	      txt: literal string to render
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

/*
CenteredLabel returns a label configured to align its text in the center.

	params:
	      th: material theme instance
	      size: text scale in SP 
	      txt: literal string to render
	      col: NRGBA text color
	returns:
	      material.LabelStyle
*/
func CenteredLabel(th *material.Theme, size float32, txt string, col color.NRGBA) material.LabelStyle {
	l := material.Label(th, unit.Sp(size), txt)
	l.Color = col
	l.Alignment = text.Middle
	return l
}
