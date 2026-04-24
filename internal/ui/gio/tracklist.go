// internal/ui/gio/tracklist.go
// Scrollable track list view. Displays cover art, title, and artist per row.
// Cover ImageOps are cached on first build and reused across frames.
//
// Dependencies:
//   - gioui.org: layout, widget, material, paint, clip, f32, op

package gio

import (
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
	coverOps  []paint.ImageOp // cached GPU textures, one per track
	coversLen int             // tracks how many covers were cached
}

/*
NewTrackList creates a new track list view.

	params:
	      player: player state
	returns:
	      *TrackList
*/
func NewTrackList(player *Player) *TrackList {
	tl := &TrackList{player: player}
	tl.list.Axis = layout.Vertical
	return tl
}

/*
ensureCoverOps builds paint.ImageOp for each track that has cover art.
Only runs when the queue length changes (i.e. after LoadFolder).
*/
func (tl *TrackList) ensureCoverOps() {
	if tl.coversLen == len(tl.player.Queue) {
		return
	}

	tl.coverOps = make([]paint.ImageOp, len(tl.player.Queue))
	for i, track := range tl.player.Queue {
		if track.Cover != nil {
			tl.coverOps[i] = paint.NewImageOp(toRGBA(track.Cover))
		}
	}
	tl.coversLen = len(tl.player.Queue)
}

/*
Layout renders the scrollable track list with cover thumbnails.

	params:
	      gtx: layout context
	      th:  material theme
	returns:
	      layout.Dimensions
*/
func (tl *TrackList) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	queue := tl.player.Queue

	tl.ensureCoverOps()

	for len(tl.clicks) < len(queue) {
		tl.clicks = append(tl.clicks, widget.Clickable{})
	}

	for i := range tl.clicks {
		if tl.clicks[i].Clicked(gtx) {
			tl.player.PlayTrack(i)
		}
	}

	listStyle := material.List(th, &tl.list)
	listStyle.AnchorStrategy = material.Overlay

	return listStyle.Layout(gtx, len(queue), func(gtx layout.Context, index int) layout.Dimensions {
		track := queue[index]
		isActive := index == tl.player.Current

		return material.Clickable(gtx, &tl.clicks[index], func(gtx layout.Context) layout.Dimensions {
			bgColor := ColorBg
			if isActive {
				bgColor = ColorSelected
			}

			return layout.Background{}.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					sz := image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Min.Y)
					r := clip.Rect{Max: sz}
					paint.FillShape(gtx.Ops, bgColor, r.Op())
					return layout.Dimensions{Size: sz}
				},
				func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{
						Top: 6, Bottom: 6,
						Left: 12, Right: 12,
					}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{
							Axis:      layout.Horizontal,
							Alignment: layout.Middle,
						}.Layout(gtx,
							// Cover thumbnail
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								coverSize := gtx.Dp(unit.Dp(40))
								sz := image.Pt(coverSize, coverSize)

								hasCover := index < len(tl.coverOps) && track.Cover != nil
								if hasCover {
									imgOp := tl.coverOps[index]
									imgOp.Add(gtx.Ops)

									imgW := float32(imgOp.Size().X)
									imgH := float32(imgOp.Size().Y)
									scaleX := float32(coverSize) / imgW
									scaleY := float32(coverSize) / imgH

									aff := f32.Affine2D{}.Scale(f32.Pt(0, 0), f32.Pt(scaleX, scaleY))
									affStack := op.Affine(aff).Push(gtx.Ops)
									clipStack := clip.Rect{Max: image.Pt(int(imgW), int(imgH))}.Push(gtx.Ops)
									paint.PaintOp{}.Add(gtx.Ops)
									clipStack.Pop()
									affStack.Pop()
								} else {
									r := clip.Rect{Max: sz}
									paint.FillShape(gtx.Ops, ColorBar, r.Op())
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
											return BoldLabel(th, 14, track.Title, titleColor).Layout(gtx)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											if track.Artist == "" {
												return layout.Dimensions{}
											}
											return layout.Inset{Top: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
												return DimLabel(th, 12, track.Artist).Layout(gtx)
											})
										}),
									)
								})
							}),
						)
					})
				},
			)
		})
	})
}

/*
LayoutDivider draws a thin horizontal line separator.
*/
func LayoutDivider(gtx layout.Context) layout.Dimensions {
	sz := image.Pt(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(1)))
	r := clip.Rect{Max: sz}
	paint.FillShape(gtx.Ops, color.NRGBA{R: 60, G: 60, B: 80, A: 255}, r.Op())
	return layout.Dimensions{Size: sz}
}

/*
LayoutVerticalDivider draws a thin vertical line separator.
*/
func LayoutVerticalDivider(gtx layout.Context) layout.Dimensions {
	sz := image.Pt(gtx.Dp(unit.Dp(1)), gtx.Constraints.Max.Y)
	r := clip.Rect{Max: sz}
	paint.FillShape(gtx.Ops, color.NRGBA{R: 60, G: 60, B: 80, A: 255}, r.Op())
	return layout.Dimensions{Size: sz}
}
