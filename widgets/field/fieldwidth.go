package field

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
)

// fieldWidth gives a field the width rule of Input: the field takes all the
// width its parent gives it, so that the fields of a form line up, and takes
// width where the parent leaves the width open, as a Row does.
type fieldWidth struct {
	width float32
	child core.Widget
}

// Key implements core.Widget.
func (fieldWidth) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (f fieldWidth) CreateRenderObject() core.RenderObject {
	return &renderFieldWidth{width: f.width}
}

// UpdateRenderObject implements core.RenderWidget.
func (f fieldWidth) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderFieldWidth)
	if r.width != f.width {
		r.width = f.width
		r.MarkNeedsLayout()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (f fieldWidth) ChildWidget() core.Widget { return f.child }

// renderFieldWidth is the render object of fieldWidth.
type renderFieldWidth struct {
	layout.RenderProxy
	width float32
}

// Layout implements core.RenderObject.
func (r *renderFieldWidth) Layout(c geom.Constraints) geom.Size {
	w := c.MaxW
	if w >= geom.Inf {
		w = max(r.width, c.MinW)
	}
	inner := geom.Constraints{MinW: w, MaxW: w, MinH: c.MinH, MaxH: c.MaxH}
	if r.ChildCount() == 0 {
		return inner.Constrain(geom.Size{})
	}
	return core.LayoutChild(r.ChildAt(0), inner)
}
