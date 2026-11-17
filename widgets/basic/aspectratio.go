package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
)

// AspectRatio sizes its child to a width over height of Ratio: 16.0 / 9 for a
// video, 1 for a square. It takes all the width it is allowed and the height
// that goes with it, and where that height is too much, the most height and the
// width that goes with it, so that in a bounded box it is the largest rectangle
// of that shape that fits. Tight constraints, such as those of a window or of
// CrossStretch, leave no room for the ratio, so put it in a Center or an Align
// there:
//
//	basic.Center{Child: basic.AspectRatio{Ratio: 16.0 / 9, Child: video}}
//
// The zero Ratio is 1. With an unbounded width, as in a Row, it takes its width
// from a bounded height; with neither bounded it panics. See
// layout.AspectRatioSize.
type AspectRatio struct {
	Ratio float32
	Child core.Widget
}

// Key implements core.Widget.
func (AspectRatio) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (a AspectRatio) CreateRenderObject() core.RenderObject {
	return &layout.RenderAspectRatio{Ratio: a.Ratio}
}

// UpdateRenderObject implements core.RenderWidget.
func (a AspectRatio) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderAspectRatio)
	if r.Ratio != a.Ratio {
		r.Ratio = a.Ratio
		r.MarkNeedsLayout()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (a AspectRatio) ChildWidget() core.Widget { return a.Child }
