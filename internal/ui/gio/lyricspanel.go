// internal/ui/gio/lyricspanel.go
// Collapsible lyrics panel with synced line highlighting and smooth scroll.
// Uses lerp-based offset interpolation for animation in Gio's immediate-mode loop.
//
// Dependencies:
//   - internal/service/lyrics: LRC parsing and fetching
//   - gioui.org: layout, widget, material, op, paint, clip

package gio

import (
	"fmt"
	"image"
	"sort"
	"sync"

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
	lineHeightDp  = 32
	lyricsLerpFac = 0.10
	visibleBefore = 6
	visibleAfter  = 6
)

type LyricsPanel struct {
	mu          sync.Mutex
	player      *Player
	lines       []lyrics.Line
	loaded      bool
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
	}
}

/*
SetLyrics sets the lyrics for the current track.

	params:
	      lines: parsed LRC lines
	      path:  track file path (used to detect track changes)
*/
func (lp *LyricsPanel) SetLyrics(lines []lyrics.Line, path string) {
	lp.mu.Lock()
	defer lp.mu.Unlock()
	lp.lines = lines
	lp.loaded = len(lines) > 0
	lp.trackPath = path
	lp.currentLine = -1
	lp.scrollY = 0

	if shared.Debug {
		fmt.Printf("[DEBUG][lyrics-panel] SetLyrics: %d lines, path=%s\n", len(lines), path)
	}
}

/*
ClearLyrics clears the lyrics panel.
*/
func (lp *LyricsPanel) ClearLyrics() {
	lp.mu.Lock()
	defer lp.mu.Unlock()
	lp.lines = nil
	lp.loaded = false
	lp.currentLine = -1
	lp.scrollY = 0
	lp.trackPath = ""

	if shared.Debug {
		fmt.Println("[DEBUG][lyrics-panel] ClearLyrics: panel cleared")
	}
}

/*
Layout renders the lyrics panel with synced highlighting and smooth scroll.

	params:
	      gtx: layout context
	      th:  material theme
	returns:
	      layout.Dimensions
*/
func (lp *LyricsPanel) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	lp.mu.Lock()
	lines := lp.lines
	loaded := lp.loaded
	lp.mu.Unlock()

	// Fill background
	bgSize := image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	bgR := clip.Rect{Max: bgSize}
	paint.FillShape(gtx.Ops, ColorBg, bgR.Op())

	if !loaded || len(lines) == 0 {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return DimLabel(th, 13, "No lyrics available").Layout(gtx)
		})
	}

	// Find current line from playback position
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

	// Calculate target scroll and lerp
	lineH := float32(gtx.Dp(unit.Dp(lineHeightDp)))
	panelH := float32(bgSize.Y)
	centerOffset := panelH / 2

	targetY := float32(newLine)*lineH - centerOffset + lineH/2
	if targetY < 0 {
		targetY = 0
	}

	lp.scrollY += (targetY - lp.scrollY) * lyricsLerpFac

	// Clip the lyrics area
	defer clip.Rect{Max: bgSize}.Push(gtx.Ops).Pop()

	// Render visible lines centered around current
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
			size := float32(13)
			if i == newLine {
				color = ColorAccent
				size = 15
			}
			l := LabelStyle(th, size, lines[i].Text, color)
			l.MaxLines = 1
			l.Truncator = "..."
			return l.Layout(gtx)
		})

		stack.Pop()
	}

	// Request redraw for smooth animation
	// Redraw is driven by the 250ms ticker in app.go

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
