// internal/ui/gio/nowplaying.go
// Now Playing panel. Shows cover art, track title, artist, album,
// a progress bar, and current time / total time.

package gio

import (
	"fmt"
	"image"
	"image/draw"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type NowPlaying struct {
	player       *Player
	cachedPath   string        // path of the track whose cover is cached
	cachedImgOp  paint.ImageOp // GPU-uploaded texture, reused across frames
	hasCover     bool
	seekSlider   widget.Float
}

/*
NewNowPlaying creates a new now-playing panel.

	params:
	      player: player state
	returns:
	      *NowPlaying
*/
func NewNowPlaying(player *Player) *NowPlaying {
	return &NowPlaying{player: player}
}

/*
LayoutNowPlaying renders the now-playing panel.

	params:
	      gtx: layout context
	      th:  material theme
	returns:
	      layout.Dimensions
*/
func (np *NowPlaying) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	track := np.player.CurrentTrack()

	return layout.Inset{Top: 12, Bottom: 4, Left: 16, Right: 16}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceEnd}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					coverSize := gtx.Dp(unit.Dp(80))
					sz := image.Pt(coverSize, coverSize)

						// Rebuild cached ImageOp only when track changes
					trackPath := ""
					if track != nil {
						trackPath = track.Path
					}
					if trackPath != np.cachedPath {
						np.cachedPath = trackPath
						if track != nil && track.Cover != nil {
							np.cachedImgOp = paint.NewImageOp(toRGBA(track.Cover))
							np.hasCover = true
						} else {
							np.hasCover = false
						}
					}

					if np.hasCover {
						np.cachedImgOp.Add(gtx.Ops)

						imgW := float32(np.cachedImgOp.Size().X)
						imgH := float32(np.cachedImgOp.Size().Y)
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

				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: 12}.Layout(gtx,
						func(gtx layout.Context) layout.Dimensions {
							title := "No track"
							artist := ""
							album := ""
							if track != nil {
								title = track.Title
								artist = track.Artist
								album = track.Album
							}

							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return BoldLabel(th, 16, title, ColorText).Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if artist == "" {
										return layout.Dimensions{}
									}
									return layout.Inset{Top: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return DimLabel(th, 13, artist).Layout(gtx)
									})
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if album == "" {
										return layout.Dimensions{}
									}
									return layout.Inset{Top: 1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return DimLabel(th, 11, album).Layout(gtx)
									})
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Top: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return np.layoutProgressBar(gtx, th)
									})
								}),
							)
						},
					)
				}),
			)
		},
	)
}

func (np *NowPlaying) layoutProgressBar(gtx layout.Context, th *material.Theme) layout.Dimensions {
	pos := np.player.Engine.GetPosition()
	dur := np.player.Engine.GetDuration()

	if np.seekSlider.Update(gtx) {
		if dur > 0 {
			targetPos := float64(np.seekSlider.Value) * dur
			np.player.Engine.Seek(targetPos, np.player.Volume)
		}
	}

	if !np.seekSlider.Dragging() && dur > 0 {
		np.seekSlider.Value = float32(pos / dur)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			slider := material.Slider(th, &np.seekSlider)
			slider.Color = ColorBarFilled
			return slider.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return DimLabel(th, 11, formatTime(pos)).Layout(gtx)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{}
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return DimLabel(th, 11, formatTime(dur)).Layout(gtx)
					}),
				)
			})
		}),
	)
}

func formatTime(secs float64) string {
	if secs <= 0 {
		return "0:00"
	}
	m := int(secs) / 60
	s := int(secs) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

func toRGBA(src image.Image) *image.RGBA {
	if rgba, ok := src.(*image.RGBA); ok {
		return rgba
	}
	b := src.Bounds()
	rgba := image.NewRGBA(b)
	draw.Draw(rgba, b, src, b.Min, draw.Src)
	return rgba
}
