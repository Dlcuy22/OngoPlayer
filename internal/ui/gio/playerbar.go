// internal/ui/gio/playerbar.go
// Bottom player bar: progress slider with time labels and
// controls (rewind, prev, play/pause, next, forward).
// Volume is handled by the Header component.
//
// Dependencies:
//   - gioui.org: layout, widget, material, unit, clip, paint
//   - golang.org/x/exp/shiny/materialdesign/icons

package gio

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	AudioEngine "github.com/dlcuy22/OngoPlayer/Audioengine"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

var (
	iconPlay    = mustIcon(icons.AVPlayArrow)
	iconPause   = mustIcon(icons.AVPause)
	iconPrev    = mustIcon(icons.AVSkipPrevious)
	iconNext    = mustIcon(icons.AVSkipNext)
	iconRewind  = mustIcon(icons.AVFastRewind)
	iconForward = mustIcon(icons.AVFastForward)
)

func mustIcon(data []byte) *widget.Icon {
	ic, err := widget.NewIcon(data)
	if err != nil {
		panic(err)
	}
	return ic
}

type PlayerBar struct {
	player     *Player
	seekSlider widget.Float
	btnPrev    widget.Clickable
	btnPlay    widget.Clickable
	btnNext    widget.Clickable
	btnRewind  widget.Clickable
	btnForward widget.Clickable
}

/*
NewPlayerBar initializes a new PlayerBar component.

	params:
	      player: active Player instance
	returns:
	      *PlayerBar
*/
func NewPlayerBar(player *Player) *PlayerBar {
	return &PlayerBar{player: player}
}

/*
Layout renders the player bar controls and progress slider.

	params:
	      gtx: layout context
	      th: material theme
	returns:
	      layout.Dimensions
*/
func (pb *PlayerBar) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if pb.btnPrev.Clicked(gtx) {
		pb.player.Prev()
	}
	if pb.btnPlay.Clicked(gtx) {
		pb.player.TogglePause()
	}
	if pb.btnNext.Clicked(gtx) {
		pb.player.Next()
	}
	if pb.btnRewind.Clicked(gtx) {
		pos := pb.player.Engine.GetPosition()
		newPos := pos - 5
		if newPos < 0 {
			newPos = 0
		}
		_ = pb.player.Engine.Seek(newPos, pb.player.Volume)
	}
	if pb.btnForward.Clicked(gtx) {
		pos := pb.player.Engine.GetPosition()
		_ = pb.player.Engine.Seek(pos+5, pb.player.Volume)
	}

	pos := pb.player.Engine.GetPosition()
	dur := pb.player.Engine.GetDuration()

	if pb.seekSlider.Update(gtx) {
		if dur > 0 {
			targetPos := float64(pb.seekSlider.Value) * dur
			pb.player.Engine.Seek(targetPos, pb.player.Volume)
		}
	}
	if !pb.seekSlider.Dragging() && dur > 0 {
		pb.seekSlider.Value = float32(pos / dur)
	}

	playIcon := iconPlay
	playDesc := "Play"
	state := pb.player.Engine.GetState()
	if state == AudioEngine.StatePlaying {
		playIcon = iconPause
		playDesc = "Pause"
	}

	return layout.Inset{Left: 24, Right: 24, Top: 8, Bottom: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// Progress slider + time labels
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						slider := material.Slider(th, &pb.seekSlider)
						slider.Color = ColorAccent
						return slider.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(th, unit.Sp(11), formatTime(pos))
									l.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 77}
									return l.Layout(gtx)
								}),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return layout.Dimensions{}
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(th, unit.Sp(11), formatTime(dur))
									l.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 77}
									return l.Layout(gtx)
								}),
							)
						})
					}),
				)
			}),

			// Control buttons: rewind, prev, play/pause, next, forward
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: -2, Bottom: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{
							Axis:      layout.Horizontal,
							Alignment: layout.Middle,
						}.Layout(gtx,
							// Rewind 5s
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								btn := material.IconButton(th, &pb.btnRewind, iconRewind, "Rewind 5s")
								btn.Size = unit.Dp(18)
								btn.Color = ColorTextDim
								btn.Background = color.NRGBA{A: 0}
								btn.Inset = layout.UniformInset(unit.Dp(8))
								return btn.Layout(gtx)
							}),

							// Prev
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								btn := material.IconButton(th, &pb.btnPrev, iconPrev, "Previous")
								btn.Size = unit.Dp(22)
								btn.Color = ColorTextDim
								btn.Background = color.NRGBA{A: 0}
								btn.Inset = layout.UniformInset(unit.Dp(8))
								return btn.Layout(gtx)
							}),

							// Play/Pause (white circle)
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									btn := material.IconButton(th, &pb.btnPlay, playIcon, playDesc)
									btn.Size = unit.Dp(24)
									btn.Color = ColorDark
									btn.Background = ColorWhite
									btn.Inset = layout.UniformInset(unit.Dp(12))
									return btn.Layout(gtx)
								})
							}),

							// Next
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								btn := material.IconButton(th, &pb.btnNext, iconNext, "Next")
								btn.Size = unit.Dp(22)
								btn.Color = ColorTextDim
								btn.Background = color.NRGBA{A: 0}
								btn.Inset = layout.UniformInset(unit.Dp(8))
								return btn.Layout(gtx)
							}),

							// Forward 5s
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								btn := material.IconButton(th, &pb.btnForward, iconForward, "Forward 5s")
								btn.Size = unit.Dp(18)
								btn.Color = ColorTextDim
								btn.Background = color.NRGBA{A: 0}
								btn.Inset = layout.UniformInset(unit.Dp(8))
								return btn.Layout(gtx)
							}),
						)
					})
				})
			}),

			// Audio details text
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						codec := "Unknown"
						if t := pb.player.CurrentTrack(); t != nil && t.Format != "" {
							codec = t.Format
						}

						sr := pb.player.Engine.GetSampleRate()
						ch := pb.player.Engine.GetChannels()
						chStr := "Stereo"
						if ch == 1 {
							chStr = "Mono"
						} else if ch > 2 {
							chStr = fmt.Sprintf("%d ch", ch)
						}

						detailStr := fmt.Sprintf("%v %.1fkHz %s SDL3 F32", codec, float64(sr)/1000.0, chStr)
						l := material.Label(th, unit.Sp(10), detailStr)
						l.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 60}
						return l.Layout(gtx)
					})
				})
			}),
		)
	})
}

/*
formatTime computes the MM:SS string interpretation of audio time offsets.

	params:
	      secs: total seconds elapsed
	returns:
	      string: time in M:SS format
*/
func formatTime(secs float64) string {
	if secs <= 0 {
		return "0:00"
	}
	m := int(secs) / 60
	s := int(secs) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

/*
LayoutVerticalDivider draws a layout line separating vertical panels.

	params:
	      gtx: layout context
	returns:
	      layout.Dimensions
*/
func LayoutVerticalDivider(gtx layout.Context) layout.Dimensions {
	sz := image.Pt(gtx.Dp(unit.Dp(1)), gtx.Constraints.Max.Y)
	r := clip.Rect{Max: sz}
	paint.FillShape(gtx.Ops, ColorDivider, r.Op())
	return layout.Dimensions{Size: sz}
}
