// internal/ui/gio/app.go
// Main application window and event loop.
// Composes header, track list, lyrics panel, now-playing, and controls.
//
// Dependencies:
//   - gioui.org: app, layout, op, paint, clip
//   - internal/service/lyrics: synced lyrics

package gio

import (
	"fmt"
	"image"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget/material"

	"github.com/dlcuy22/OngoPlayer/internal/service/lyrics"
	"github.com/dlcuy22/OngoPlayer/internal/shared"
)

type App struct {
	window      *app.Window
	theme       *material.Theme
	player      *Player
	header      *Header
	trackList   *TrackList
	nowPlaying  *NowPlaying
	controls    *Controls
	lyricsPanel *LyricsPanel
	lyricsOpen  bool
}

/*
NewApp creates a new Gio application.

	params:
	      player: player state
	returns:
	      *App
*/
func NewApp(player *Player) *App {
	w := new(app.Window)
	w.Option(
		app.Title("DongoPlayer"),
		app.Size(900, 640),
	)

	th := NewTheme()

	a := &App{
		window:      w,
		theme:       th,
		player:      player,
		trackList:   NewTrackList(player),
		nowPlaying:  NewNowPlaying(player),
		controls:    NewControls(player),
		lyricsPanel: NewLyricsPanel(player),
		lyricsOpen:  false,
	}

	a.header = NewHeader(&a.lyricsOpen)

	player.OnUpdate = func() {
		w.Invalidate()
	}

	player.OnTrackChange = func(track TrackMeta) {
		a.lyricsPanel.SetLoading(track.Path)
		a.window.Invalidate()
		go a.loadLyrics(track)
	}

	return a
}

/*
loadLyrics tries to load lyrics from file, then falls back to API.
Runs in a goroutine to avoid blocking the UI.
*/
func (a *App) loadLyrics(track TrackMeta) {
	musicDir := a.player.MusicDir

	if shared.Debug {
		fmt.Println("[DEBUG][lyrics] ── track changed ──")
		fmt.Printf("[DEBUG][lyrics]   title:  %q\n", track.Title)
		fmt.Printf("[DEBUG][lyrics]   artist: %q\n", track.Artist)
		fmt.Printf("[DEBUG][lyrics]   album:  %q\n", track.Album)
		fmt.Printf("[DEBUG][lyrics]   path:   %s\n", track.Path)
		fmt.Printf("[DEBUG][lyrics]   dir:    %s\n", musicDir)
	}

	// Try local .lrc file
	if shared.Debug {
		fmt.Println("[DEBUG][lyrics] trying local .lrc file...")
	}
	if lr, ok := lyrics.LoadFromFile(track.Path, musicDir); ok {
		if shared.Debug {
			fmt.Printf("[DEBUG][lyrics] loaded from local file: %d lines\n", len(lr.Lines))
			if len(lr.Lines) > 0 {
				fmt.Printf("[DEBUG][lyrics]   first: [%.2fs] %q\n", lr.Lines[0].Time, lr.Lines[0].Text)
				fmt.Printf("[DEBUG][lyrics]   last:  [%.2fs] %q\n", lr.Lines[len(lr.Lines)-1].Time, lr.Lines[len(lr.Lines)-1].Text)
			}
		}
		a.lyricsPanel.SetLyrics(lr.Lines, track.Path)
		a.window.Invalidate()
		return
	}

	if shared.Debug {
		fmt.Println("[DEBUG][lyrics] no local file found")
	}

	// Fallback: fetch from lrclib.net
	if track.Artist != "" && track.Title != "" {
		if shared.Debug {
			fmt.Printf("[DEBUG][lyrics] fetching from lrclib.net (artist=%q, title=%q, album=%q)\n",
				track.Artist, track.Title, track.Album)
		}
		content, err := lyrics.FetchFromAPI(track.Artist, track.Title, track.Album)
		if err == nil && content != "" {
			lines := lyrics.Parse(content)
			if shared.Debug {
				fmt.Printf("[DEBUG][lyrics] API returned %d bytes, parsed %d lines\n", len(content), len(lines))
			}

			savePath, saveErr := lyrics.SaveToFile(track.Path, musicDir, content)
			if shared.Debug {
				if saveErr != nil {
					fmt.Printf("[DEBUG][lyrics] save failed: %v\n", saveErr)
				} else {
					fmt.Printf("[DEBUG][lyrics] saved to: %s\n", savePath)
				}
			}

			a.lyricsPanel.SetLyrics(lines, track.Path)
			a.window.Invalidate()
			return
		}

		if shared.Debug && err != nil {
			fmt.Printf("[DEBUG][lyrics] API error: %v\n", err)
		}
	} else if shared.Debug {
		fmt.Println("[DEBUG][lyrics] skipping API fetch: missing artist or title")
	}

	if shared.Debug {
		fmt.Println("[DEBUG][lyrics] no lyrics found, clearing panel")
	}
	a.lyricsPanel.ClearLyrics(track.Path)
	a.window.Invalidate()
}

/*
Run starts the Gio event loop. Blocks until the window is closed.
*/
func (a *App) Run() error {
	go func() {
		ticker := time.NewTicker(33 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			a.window.Invalidate()
		}
	}()

	var ops op.Ops

	for {
		switch e := a.window.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			a.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func (a *App) layout(gtx layout.Context) layout.Dimensions {
	bgSize := image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	bgR := clip.Rect{Max: bgSize}
	paint.FillShape(gtx.Ops, ColorBg, bgR.Op())

	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		// Header bar
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.header.Layout(gtx, a.theme)
		}),

		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return LayoutDivider(gtx)
		}),

		// Middle area: TrackList (+ optional LyricsPanel)
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if !a.lyricsOpen {
				return a.trackList.Layout(gtx, a.theme)
			}

			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				// Track list (55%)
				layout.Flexed(0.55, func(gtx layout.Context) layout.Dimensions {
					return a.trackList.Layout(gtx, a.theme)
				}),

				// Vertical divider
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return LayoutVerticalDivider(gtx)
				}),

				// Lyrics panel (45%)
				layout.Flexed(0.45, func(gtx layout.Context) layout.Dimensions {
					return a.lyricsPanel.Layout(gtx, a.theme)
				}),
			)
		}),

		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return LayoutDivider(gtx)
		}),

		// Now playing
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.nowPlaying.Layout(gtx, a.theme)
		}),

		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return LayoutDivider(gtx)
		}),

		// Controls
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.controls.Layout(gtx, a.theme)
		}),
	)
}
