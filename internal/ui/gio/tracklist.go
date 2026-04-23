// internal/ui/gio/tracklist.go
// Scrollable track list view. Displays each track's title and artist.

package gio

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type TrackList struct {
	list   widget.List
	clicks []widget.Clickable
	player *Player
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
LayoutTrackList renders the scrollable track list.

	params:
	      gtx: layout context
	      th:  material theme
	returns:
	      layout.Dimensions
*/
func (tl *TrackList) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	queue := tl.player.Queue

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
						Top: 8, Bottom: 8,
						Left: 16, Right: 16,
					}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceStart}.Layout(gtx,
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
