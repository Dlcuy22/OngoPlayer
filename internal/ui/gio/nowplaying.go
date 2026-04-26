// internal/ui/gio/nowplaying.go
// Center panel: large album art with ambient glow, track metadata,
// and inline synced lyrics with fade effects.
//
// Dependencies:
//   - gioui.org: layout, op, clip, paint, f32, font, unit, widget/material
//   - internal/service/lyrics: LRC line data

package gio

import (
	"fmt"
	"image"
	"image/color"
	"sort"
	"strings"
	"sync"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/dlcuy22/OngoPlayer/internal/service/lyrics"
	"github.com/dlcuy22/OngoPlayer/internal/shared"
)

const (
	artSizeDp    = 240
	lineHeightDp = 36
	lyricsLerp   = 0.15
	lyricsBefore = 6
	lyricsAfter  = 6
)

type NowPlaying struct {
	player      *Player
	cachedPath  string
	cachedArtOp paint.ImageOp
	hasArt      bool

	// lyrics state
	lyricsMu    sync.Mutex
	lyricsLines []lyrics.Line
	lyricsState int // 0=idle, 1=loading, 2=active
	currentLine int
	scrollY     float32
	trackPath   string
}

/*
NewNowPlaying creates a new NowPlaying component instance.

	params:
	      player: pointer to the active Player
	returns:
	      *NowPlaying
*/
func NewNowPlaying(player *Player) *NowPlaying {
	return &NowPlaying{
		player:      player,
		currentLine: -1,
	}
}

// --- Lyrics management ---

/*
SetLoading indicates that lyrics for the given path are currently being loaded.

	params:
	      path: file path of the currently loading track
*/
func (np *NowPlaying) SetLoading(path string) {
	np.lyricsMu.Lock()
	defer np.lyricsMu.Unlock()
	np.lyricsLines = nil
	np.lyricsState = 1
	np.trackPath = path
	np.currentLine = -1
	np.scrollY = 0

	if shared.Debug {
		fmt.Printf("[DEBUG][lyrics] SetLoading: path=%s\n", path)
	}
}

/*
SetLyrics applies the loaded lyrics.

	params:
	      lines: parsed LRC lines
	      path: file path this belongs to (avoids race conditions)
	returns:
	      bool: true if applied successfully, false if track changed
*/
func (np *NowPlaying) SetLyrics(lines []lyrics.Line, path string) bool {
	np.lyricsMu.Lock()
	defer np.lyricsMu.Unlock()

	if np.trackPath != path {
		return false
	}

	np.lyricsLines = splitLyricsWrap(lines, 30)
	if len(np.lyricsLines) > 0 {
		np.lyricsState = 2
	} else {
		np.lyricsState = 0
	}
	np.currentLine = -1
	np.scrollY = 0
	return true
}

/*
ClearLyrics safely empties the stored lyrics if no data is found.

	params:
	      path: track path to clear lyrics for
*/
func (np *NowPlaying) ClearLyrics(path string) {
	np.lyricsMu.Lock()
	defer np.lyricsMu.Unlock()

	if np.trackPath != path {
		return
	}

	np.lyricsLines = nil
	np.lyricsState = 0
	np.currentLine = -1
	np.scrollY = 0
}

// --- Layout ---

/*
Layout renders the NowPlaying component, including album art and inline lyrics.

	params:
	      gtx: layout context
	      th: material theme
	returns:
	      layout.Dimensions
*/
func (np *NowPlaying) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	track := np.player.CurrentTrack()

	bgSize := image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	paint.FillShape(gtx.Ops, ColorBg, clip.Rect{Max: bgSize}.Op())

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Album art + metadata (rigid, centered)
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 32, Left: 32, Right: 32}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					// Album art with glow (centered via layout.Center)
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return np.layoutAlbumArt(gtx, track)
						})
					}),

					// Track title (full width, text centered)
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						title := "No track"
						if track != nil {
							title = track.Title
						}
						return layout.Inset{Top: 24}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							l := BoldLabel(th, 20, title, ColorWhite)
							l.Alignment = text.Middle
							l.MaxLines = 1
							return l.Layout(gtx)
						})
					}),

					// Artist · Album (full width, text centered)
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						sub := ""
						if track != nil {
							parts := []string{}
							if track.Artist != "" {
								parts = append(parts, track.Artist)
							}
							if track.Album != "" {
								parts = append(parts, track.Album)
							}
							sub = strings.Join(parts, " · ")
						}
						if sub == "" {
							return layout.Dimensions{}
						}
						return layout.Inset{Top: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							l := LabelStyle(th, 13, sub, ColorTextDim)
							l.Alignment = text.Middle
							return l.Layout(gtx)
						})
					}),
				)
			})
		}),

		// Inline lyrics (fills remaining space)
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 20}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return np.layoutLyrics(gtx, th)
			})
		}),
	)
}

func (np *NowPlaying) layoutAlbumArt(gtx layout.Context, track *TrackMeta) layout.Dimensions {
	artPx := gtx.Dp(unit.Dp(artSizeDp))
	sz := image.Pt(artPx, artPx)

	trackPath := ""
	if track != nil {
		trackPath = track.Path
	}
	if trackPath != np.cachedPath {
		np.cachedPath = trackPath
		if track != nil && track.Cover != nil {
			np.cachedArtOp = paint.NewImageOp(track.Cover)
			np.hasArt = true
		} else {
			np.hasArt = false
		}
	}

	// Album art with rounded corners
	radius := gtx.Dp(unit.Dp(20))
	rr := clip.UniformRRect(image.Rectangle{Max: sz}, radius)
	rrStack := rr.Push(gtx.Ops)

	if np.hasArt {
		np.cachedArtOp.Add(gtx.Ops)
		imgW := float32(np.cachedArtOp.Size().X)
		imgH := float32(np.cachedArtOp.Size().Y)
		scaleX := float32(artPx) / imgW
		scaleY := float32(artPx) / imgH

		aff := f32.Affine2D{}.Scale(f32.Pt(0, 0), f32.Pt(scaleX, scaleY))
		affStack := op.Affine(aff).Push(gtx.Ops)
		imgClip := clip.Rect{Max: image.Pt(int(imgW), int(imgH))}.Push(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		imgClip.Pop()
		affStack.Pop()
	} else {
		paint.FillShape(gtx.Ops, ColorSurface, clip.Rect{Max: sz}.Op())
	}

	rrStack.Pop()

	return layout.Dimensions{Size: sz}
}

func (np *NowPlaying) layoutLyrics(gtx layout.Context, th *material.Theme) layout.Dimensions {
	np.lyricsMu.Lock()
	lines := np.lyricsLines
	state := np.lyricsState
	np.lyricsMu.Unlock()

	panelSize := image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Max.Y)

	if state == 1 {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			loader := material.Loader(th)
			loader.Color = ColorAccent
			gtx.Constraints.Max.X = gtx.Dp(unit.Dp(24))
			gtx.Constraints.Max.Y = gtx.Dp(unit.Dp(24))
			return loader.Layout(gtx)
		})
	}

	if state == 0 || len(lines) == 0 {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			l := LabelStyle(th, 12, "No lyrics available", ColorTextDim)
			l.Alignment = text.Middle
			return l.Layout(gtx)
		})
	}

	pos := np.player.Engine.GetPosition()
	newLine := findCurrentLine(lines, pos)

	np.lyricsMu.Lock()
	np.currentLine = newLine
	np.lyricsMu.Unlock()

	lineH := float32(gtx.Dp(unit.Dp(lineHeightDp)))
	panelH := float32(panelSize.Y)
	centerOffset := panelH / 2

	targetY := float32(newLine)*lineH - centerOffset + lineH/2
	if targetY < 0 {
		targetY = 0
	}
	np.scrollY += (targetY - np.scrollY) * lyricsLerp

	defer clip.Rect{Max: panelSize}.Push(gtx.Ops).Pop()

	startIdx := newLine - lyricsBefore
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := newLine + lyricsAfter
	if endIdx >= len(lines) {
		endIdx = len(lines) - 1
	}

	for i := startIdx; i <= endIdx; i++ {
		yPos := float32(i)*lineH - np.scrollY
		if yPos < -lineH || yPos > panelH {
			continue
		}

		stack := op.Offset(image.Pt(0, int(yPos))).Push(gtx.Ops)

		lineGtx := gtx
		lineGtx.Constraints.Min.X = gtx.Constraints.Max.X // force full width for centering
		lineGtx.Constraints.Max.Y = int(lineH)

		layout.Inset{Left: 16, Right: 16}.Layout(lineGtx, func(gtx layout.Context) layout.Dimensions {
			var col color.NRGBA
			size := float32(13)

			dist := i - newLine
			if dist < 0 {
				dist = -dist
			}

			switch {
			case i == newLine:
				col = ColorWhite
				size = 15
			case dist == 1:
				col = color.NRGBA{R: 255, G: 255, B: 255, A: 128}
				size = 13
			default:
				col = color.NRGBA{R: 255, G: 255, B: 255, A: 51}
				size = 13
			}

			l := LabelStyle(th, size, lines[i].Text, col)
			l.Font.Typeface = "Noto Sans JP"
			if i == newLine {
				l.Font.Weight = font.Bold
			}
			l.Alignment = text.Middle
			l.MaxLines = 1
			l.Truncator = "..."
			return l.Layout(gtx)
		})

		stack.Pop()
	}

	// Top fade overlay
	fadeH := gtx.Dp(unit.Dp(32))
	topFade := clip.Rect{Max: image.Pt(panelSize.X, fadeH)}.Push(gtx.Ops)
	paint.LinearGradientOp{
		Stop1:  f32.Pt(0, 0),
		Stop2:  f32.Pt(0, float32(fadeH)),
		Color1: ColorBg,
		Color2: color.NRGBA{R: ColorBg.R, G: ColorBg.G, B: ColorBg.B, A: 0},
	}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	topFade.Pop()

	// Bottom fade overlay
	botFadeH := gtx.Dp(unit.Dp(40))
	botOff := op.Offset(image.Pt(0, panelSize.Y-botFadeH)).Push(gtx.Ops)
	botFade := clip.Rect{Max: image.Pt(panelSize.X, botFadeH)}.Push(gtx.Ops)
	paint.LinearGradientOp{
		Stop1:  f32.Pt(0, 0),
		Stop2:  f32.Pt(0, float32(botFadeH)),
		Color1: color.NRGBA{R: ColorBg.R, G: ColorBg.G, B: ColorBg.B, A: 0},
		Color2: ColorBg,
	}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	botFade.Pop()
	botOff.Pop()

	return layout.Dimensions{Size: panelSize}
}

func findCurrentLine(lines []lyrics.Line, pos float64) int {
	if len(lines) == 0 {
		return 0
	}
	idx := sort.Search(len(lines), func(i int) bool {
		return lines[i].Time > pos
	})
	if idx > 0 {
		idx--
	}
	return idx
}

func splitLyricsWrap(lines []lyrics.Line, maxChars int) []lyrics.Line {
	if len(lines) == 0 {
		return lines
	}

	var result []lyrics.Line
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		parts := wordWrap(line.Text, maxChars)
		if len(parts) <= 1 {
			result = append(result, line)
			continue
		}

		var nextTime float64
		if i < len(lines)-1 {
			nextTime = lines[i+1].Time
		} else {
			nextTime = line.Time + 5.0
		}

		totalDuration := nextTime - line.Time
		if totalDuration <= 0 {
			totalDuration = float64(len(parts)) * 0.5
		}

		totalChars := 0
		for _, p := range parts {
			totalChars += len(p)
		}

		currentTime := line.Time
		for _, p := range parts {
			result = append(result, lyrics.Line{
				Time: currentTime,
				Text: p,
			})
			fraction := float64(len(p)) / float64(totalChars)
			currentTime += totalDuration * fraction
		}
	}

	return result
}

func wordWrap(t string, limit int) []string {
	if len(t) <= limit {
		return []string{t}
	}
	var parts []string
	words := strings.Fields(t)
	if len(words) == 0 {
		return []string{t}
	}
	currentLine := words[0]
	for _, word := range words[1:] {
		if len(currentLine)+1+len(word) > limit {
			parts = append(parts, currentLine)
			currentLine = word
		} else {
			currentLine += " " + word
		}
	}
	if currentLine != "" {
		parts = append(parts, currentLine)
	}
	return parts
}
