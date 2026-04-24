// internal/ui/gio/lyricspanel.go
// Collapsible lyrics panel with synced line highlighting and smooth scroll.
// Uses lerp-based offset interpolation for animation in Gio's immediate-mode loop.
//
// States: idle (no lyrics), loading (fetching), active (displaying lines).
// Race protection: trackPath is checked before applying fetched results.
//
// Dependencies:
//   - internal/service/lyrics: LRC parsing and fetching
//   - gioui.org: layout, widget, material, op, paint, clip

package gio

import (
	"fmt"
	"image"
	"sort"
	"strings"
	"sync"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/dlcuy22/OngoPlayer/internal/service/lyrics"
	"github.com/dlcuy22/OngoPlayer/internal/shared"
)

const (
	lineHeightDp  = 40
	lyricsLerpFac = 0.15
	visibleBefore = 8
	visibleAfter  = 8
)

type lyricsState int

const (
	lyricsIdle    lyricsState = iota // no lyrics, nothing happening
	lyricsLoading                    // fetching from file/API
	lyricsActive                     // lyrics loaded and displaying
)

type LyricsPanel struct {
	mu          sync.Mutex
	player      *Player
	lines       []lyrics.Line
	state       lyricsState
	currentLine int
	scrollY     float32
	trackPath   string
}

/*
NewLyricsPanel creates a new lyrics panel.

	params:
	      player: player state
	returns:
	      *LyricsPanel
*/
func NewLyricsPanel(player *Player) *LyricsPanel {
	return &LyricsPanel{
		player:      player,
		currentLine: -1,
		state:       lyricsIdle,
	}
}

/*
SetLoading transitions to the loading state for a new track.
Clears any existing lyrics immediately to prevent stale display.

	params:
	      path: track file path (for race protection)
*/
func (lp *LyricsPanel) SetLoading(path string) {
	lp.mu.Lock()
	defer lp.mu.Unlock()
	lp.lines = nil
	lp.state = lyricsLoading
	lp.trackPath = path
	lp.currentLine = -1
	lp.scrollY = 0

	if shared.Debug {
		fmt.Printf("[DEBUG][lyrics-panel] SetLoading: path=%s\n", path)
	}
}

/*
SetLyrics sets the lyrics for a track. Only applies if path matches
the current trackPath (race protection against slow goroutines).

	params:
	      lines: parsed LRC lines
	      path:  track file path
	returns:
	      bool: true if applied, false if stale
*/
func (lp *LyricsPanel) SetLyrics(lines []lyrics.Line, path string) bool {
	lp.mu.Lock()
	defer lp.mu.Unlock()

	if lp.trackPath != path {
		if shared.Debug {
			fmt.Printf("[DEBUG][lyrics-panel] SetLyrics REJECTED (stale): got path=%s, want path=%s\n", path, lp.trackPath)
		}
		return false
	}

	// THIS FEATURE IS NOT 100% ACCURATE, TO TO 9999 TO DISABLE IT
	lp.lines = splitLyricsWrap(lines, 25)
	if len(lp.lines) > 0 {
		lp.state = lyricsActive
	} else {
		lp.state = lyricsIdle
	}
	lp.currentLine = -1
	lp.scrollY = 0

	if shared.Debug {
		fmt.Printf("[DEBUG][lyrics-panel] SetLyrics: %d lines, path=%s\n", len(lines), path)
	}
	return true
}

/*
ClearLyrics clears the lyrics panel and sets state to idle.
Only applies if path matches (race protection).

	params:
	      path: track file path
*/
func (lp *LyricsPanel) ClearLyrics(path string) {
	lp.mu.Lock()
	defer lp.mu.Unlock()

	if lp.trackPath != path {
		if shared.Debug {
			fmt.Printf("[DEBUG][lyrics-panel] ClearLyrics REJECTED (stale): got path=%s, want path=%s\n", path, lp.trackPath)
		}
		return
	}

	lp.lines = nil
	lp.state = lyricsIdle
	lp.currentLine = -1
	lp.scrollY = 0

	if shared.Debug {
		fmt.Println("[DEBUG][lyrics-panel] ClearLyrics: panel cleared")
	}
}

/*
Layout renders the lyrics panel based on current state.

	params:
	      gtx: layout context
	      th:  material theme
	returns:
	      layout.Dimensions
*/
func (lp *LyricsPanel) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	lp.mu.Lock()
	lines := lp.lines
	state := lp.state
	lp.mu.Unlock()

	bgSize := image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	bgR := clip.Rect{Max: bgSize}
	paint.FillShape(gtx.Ops, ColorBg, bgR.Op())

	switch state {
	case lyricsLoading:
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					loader := material.Loader(th)
					loader.Color = ColorAccent
					gtx.Constraints.Max.X = gtx.Dp(unit.Dp(28))
					gtx.Constraints.Max.Y = gtx.Dp(unit.Dp(28))
					return loader.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return DimLabel(th, 12, "Loading lyrics...").Layout(gtx)
					})
				}),
			)
		})

	case lyricsIdle:
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return DimLabel(th, 13, "No lyrics available").Layout(gtx)
		})
	}

	// lyricsActive — render synced lines
	if len(lines) == 0 {
		return layout.Dimensions{Size: bgSize}
	}

	pos := lp.player.Engine.GetPosition()
	newLine := findCurrentLine(lines, pos)

	lp.mu.Lock()
	prevLine := lp.currentLine
	lp.currentLine = newLine
	lp.mu.Unlock()

	if shared.Debug && newLine != prevLine && newLine >= 0 && newLine < len(lines) {
		fmt.Printf("[DEBUG][lyrics-panel] line %d → %d: [%.2fs] %q\n",
			prevLine, newLine, lines[newLine].Time, lines[newLine].Text)
	}

	lineH := float32(gtx.Dp(unit.Dp(lineHeightDp)))
	panelH := float32(bgSize.Y)
	centerOffset := panelH / 2

	targetY := float32(newLine)*lineH - centerOffset + lineH/2
	if targetY < 0 {
		targetY = 0
	}

	lp.scrollY += (targetY - lp.scrollY) * lyricsLerpFac

	defer clip.Rect{Max: bgSize}.Push(gtx.Ops).Pop()

	startIdx := newLine - visibleBefore
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := newLine + visibleAfter
	if endIdx >= len(lines) {
		endIdx = len(lines) - 1
	}

	for i := startIdx; i <= endIdx; i++ {
		yPos := float32(i)*lineH - lp.scrollY
		if yPos < -lineH || yPos > panelH {
			continue
		}

		stack := op.Offset(image.Pt(0, int(yPos))).Push(gtx.Ops)

		lineGtx := gtx
		lineGtx.Constraints.Min.X = 0
		lineGtx.Constraints.Max.Y = int(lineH)

		layout.Inset{Left: 16, Right: 16}.Layout(lineGtx, func(gtx layout.Context) layout.Dimensions {
			color := ColorTextDim
			size := float32(16)
			if i == newLine {
				color = ColorAccent
				size = 20
			}
			l := LabelStyle(th, size, lines[i].Text, color)
			l.Font.Weight = font.Weight(font.Regular)
			l.MaxLines = 1
			l.Truncator = "..."
			return l.Layout(gtx)
		})

		stack.Pop()
	}

	return layout.Dimensions{Size: bgSize}
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

func wordWrap(text string, limit int) []string {
	if len(text) <= limit {
		return []string{text}
	}
	var parts []string
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{text}
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
