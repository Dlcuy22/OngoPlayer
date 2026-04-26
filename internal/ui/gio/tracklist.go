// internal/ui/gio/tracklist.go
// Compact sidebar playlist. Displays track thumbnail, title, artist, and duration.
// Cover ImageOps are cached on first build and reused across frames.
//
// The hover effect is drawn manually via pointer tracking to ensure the
// full card row lights up, not just a partial area.
//
// Dependencies:
//   - gioui.org: layout, widget, material, paint, clip, f32, op, pointer

package gio

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type TrackList struct {
	list      widget.List
	clicks    []widget.Clickable
	player    *Player
	thumbOps  []paint.ImageOp
	thumbsLen int
}

/*
NewTrackList initializes a new TrackList component.

	params:
	      player: pointer to the initialized Player instance
	returns:
	      *TrackList
*/
func NewTrackList(player *Player) *TrackList {
	tl := &TrackList{player: player}
	tl.list.Axis = layout.Vertical
	return tl
}

/*
ensureThumbOps checks if new thumb operations need to be generated for the list.
*/
func (tl *TrackList) ensureThumbOps() {
	if tl.thumbsLen == len(tl.player.Queue) {
		return
	}

	tl.thumbOps = make([]paint.ImageOp, len(tl.player.Queue))
	for i, track := range tl.player.Queue {
		if track.Thumb != nil {
			tl.thumbOps[i] = paint.NewImageOp(track.Thumb)
		}
	}
	tl.thumbsLen = len(tl.player.Queue)
}

/*
Layout renders the tracklist UI.

	params:
	      gtx: layout context
	      th: material theme
	returns:
	      layout.Dimensions
*/
func (tl *TrackList) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	queue := tl.player.Queue

	tl.ensureThumbOps()

	for len(tl.clicks) < len(queue) {
		tl.clicks = append(tl.clicks, widget.Clickable{})
	}

	for i := range tl.clicks {
		if tl.clicks[i].Clicked(gtx) {
			tl.player.PlayTrack(i)
		}
	}

	// Sidebar background
	bgSize := image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	paint.FillShape(gtx.Ops, ColorSidebar, clip.Rect{Max: bgSize}.Op())

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Sidebar header
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 14, Bottom: 8, Left: 16, Right: 16}.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					l := material.Label(th, unit.Sp(13), "ONGOPLAYER")
					l.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 89}
					return l.Layout(gtx)
				},
			)
		}),

		// Track list
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				listStyle := material.List(th, &tl.list)
				listStyle.AnchorStrategy = material.Overlay
				listStyle.Indicator.MinorWidth = unit.Dp(4)
				listStyle.Indicator.CornerRadius = unit.Dp(2)
				listStyle.Indicator.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 30}

				return listStyle.Layout(gtx, len(queue), func(gtx layout.Context, index int) layout.Dimensions {
					return tl.layoutTrackItem(gtx, th, index)
				})
			})
		}),
	)
}

/*
layoutTrackItem renders a single track item in the list.

	params:
	      gtx: layout context
	      th: material theme
	      index: position of the track in the queue
	returns:
	      layout.Dimensions
*/
func (tl *TrackList) layoutTrackItem(gtx layout.Context, th *material.Theme, index int) layout.Dimensions {
	track := tl.player.Queue[index]
	isActive := index == tl.player.Current

	// Use Clickable.Hovered() for hover detection
	isHovered := tl.clicks[index].Hovered()

	// Determine background color
	bgColor := color.NRGBA{A: 0}
	if isActive {
		bgColor = color.NRGBA{R: 255, G: 255, B: 255, A: 20}
	} else if isHovered {
		bgColor = color.NRGBA{R: 255, G: 255, B: 255, A: 13}
	}

	// Wrap in clickable for click detection
	return material.Clickable(gtx, &tl.clicks[index], func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{}.Layout(gtx,
			// Background (stretches to match content size)
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				sz := image.Pt(gtx.Constraints.Min.X, gtx.Constraints.Min.Y)
				if bgColor.A > 0 {
					rr := clip.UniformRRect(image.Rectangle{Max: sz}, gtx.Dp(unit.Dp(8)))
					paint.FillShape(gtx.Ops, bgColor, rr.Op(gtx.Ops))
				}
				return layout.Dimensions{Size: sz}
			}),

			// Content (determines the row size)
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{
					Top: 6, Bottom: 6,
					Left: 8, Right: 8,
				}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{
						Axis:      layout.Horizontal,
						Alignment: layout.Middle,
					}.Layout(gtx,
						// Thumbnail (38dp, rounded)
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							thumbSize := gtx.Dp(unit.Dp(38))
							sz := image.Pt(thumbSize, thumbSize)

							hasThumb := index < len(tl.thumbOps) && track.Thumb != nil
							if hasThumb {
								imgOp := tl.thumbOps[index]
								imgOp.Add(gtx.Ops)

								imgW := float32(imgOp.Size().X)
								imgH := float32(imgOp.Size().Y)
								scaleX := float32(thumbSize) / imgW
								scaleY := float32(thumbSize) / imgH

								rr := clip.UniformRRect(image.Rectangle{Max: sz}, gtx.Dp(unit.Dp(8)))
								rrStack := rr.Push(gtx.Ops)

								aff := f32.Affine2D{}.Scale(f32.Pt(0, 0), f32.Pt(scaleX, scaleY))
								affStack := op.Affine(aff).Push(gtx.Ops)
								clipStack := clip.Rect{Max: image.Pt(int(imgW), int(imgH))}.Push(gtx.Ops)
								paint.PaintOp{}.Add(gtx.Ops)
								clipStack.Pop()
								affStack.Pop()
								rrStack.Pop()
							} else {
								rr := clip.UniformRRect(image.Rectangle{Max: sz}, gtx.Dp(unit.Dp(8)))
								paint.FillShape(gtx.Ops, ColorSurface, rr.Op(gtx.Ops))
							}

							return layout.Dimensions{Size: sz}
						}),

						// Title + Artist
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										titleColor := ColorText
										if isActive {
											titleColor = ColorAccent
										}
										return BoldLabel(th, 13, track.Title, titleColor).Layout(gtx)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										if track.Artist == "" {
											return layout.Dimensions{}
										}
										return layout.Inset{Top: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return DimLabel(th, 11, track.Artist).Layout(gtx)
										})
									}),
								)
							})
						}),

						// Duration
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							dur := tl.player.Engine.GetDuration()
							durStr := ""
							if isActive && dur > 0 {
								durStr = formatDuration(dur)
							}
							if durStr == "" {
								return layout.Dimensions{}
							}
							l := material.Label(th, unit.Sp(11), durStr)
							l.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 77}
							return l.Layout(gtx)
						}),
					)
				})
			}),
		)
	})
}

/*
formatDuration formats duration in seconds to M:SS.

	params:
	      secs: duration in seconds
	returns:
	      string: formatted time
*/
func formatDuration(secs float64) string {
	if secs <= 0 {
		return ""
	}
	m := int(secs) / 60
	s := int(secs) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

/*
LayoutDivider draws a thin horizontal line separator.

	params:
	      gtx: layout context
	returns:
	      layout.Dimensions
*/
func LayoutDivider(gtx layout.Context) layout.Dimensions {
	sz := image.Pt(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(1)))
	r := clip.Rect{Max: sz}
	paint.FillShape(gtx.Ops, ColorDivider, r.Op())
	return layout.Dimensions{Size: sz}
}
