// internal/ui/gio/split.go
// Draggable horizontal split layout for resizing two side-by-side panels.
// Based on the official GioUI split-widget tutorial.
// The bar between the panels can be dragged left/right to adjust the ratio.
//
// Key types:
//   - Split: holds ratio, bar width, and drag state
//
// Dependencies:
//   - gioui.org: layout, op, clip, paint, unit, pointer, event

package gio

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

const (
	defaultBarWidth unit.Dp = 8
	minRatio                = -0.8
	maxRatio                = 0.8
)

type Split struct {
	Ratio float32
	Bar   unit.Dp

	drag   bool
	dragID pointer.ID
	dragX  float32
}

func (s *Split) Layout(gtx layout.Context, left, right layout.Widget) layout.Dimensions {
	bar := gtx.Dp(s.Bar)
	if bar <= 1 {
		bar = gtx.Dp(defaultBarWidth)
	}

	proportion := (s.Ratio + 1) / 2
	leftsize := int(proportion*float32(gtx.Constraints.Max.X) - float32(bar))
	if leftsize < 0 {
		leftsize = 0
	}

	rightoffset := leftsize + bar
	rightsize := gtx.Constraints.Max.X - rightoffset
	if rightsize < 0 {
		rightsize = 0
	}

	{ // handle drag input on the bar area
		barRect := image.Rect(leftsize, 0, rightoffset, gtx.Constraints.Max.Y)
		area := clip.Rect(barRect).Push(gtx.Ops)

		event.Op(gtx.Ops, s)
		pointer.CursorColResize.Add(gtx.Ops)

		for {
			ev, ok := gtx.Event(pointer.Filter{
				Target: s,
				Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel,
			})
			if !ok {
				break
			}

			e, ok := ev.(pointer.Event)
			if !ok {
				continue
			}

			switch e.Kind {
			case pointer.Press:
				if s.drag {
					break
				}
				s.dragID = e.PointerID
				s.dragX = e.Position.X
				s.drag = true

			case pointer.Drag:
				if s.dragID != e.PointerID {
					break
				}

				deltaX := e.Position.X - s.dragX
				s.dragX = e.Position.X

				deltaRatio := deltaX * 2 / float32(gtx.Constraints.Max.X)
				s.Ratio += deltaRatio

				if s.Ratio < minRatio {
					s.Ratio = minRatio
				}
				if s.Ratio > maxRatio {
					s.Ratio = maxRatio
				}

				if e.Priority < pointer.Grabbed {
					gtx.Execute(pointer.GrabCmd{
						Tag: s,
						ID:  s.dragID,
					})
				}

			case pointer.Release:
				fallthrough
			case pointer.Cancel:
				s.drag = false
			}
		}

		// paint the bar handle (1dp line in the center)
		lineW := gtx.Dp(unit.Dp(1))
		if lineW == 0 {
			lineW = 1
		}
		mid := bar / 2
		lineRect := image.Rect(mid, 0, mid+lineW, gtx.Constraints.Max.Y)
		paint.FillShape(gtx.Ops, ColorTextDim, clip.Rect(lineRect).Op())

		area.Pop()
	}

	{ // left panel
		gtx := gtx
		gtx.Constraints = layout.Exact(image.Pt(leftsize, gtx.Constraints.Max.Y))
		left(gtx)
	}

	{ // right panel
		off := op.Offset(image.Pt(rightoffset, 0)).Push(gtx.Ops)
		gtx := gtx
		gtx.Constraints = layout.Exact(image.Pt(rightsize, gtx.Constraints.Max.Y))
		right(gtx)
		off.Pop()
	}

	return layout.Dimensions{Size: gtx.Constraints.Max}
}
