package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
)

// Translate paints its child moved by X and Y times its own size, for
// transitions such as a page sliding in: X: 1 is a whole width to the right.
// Like Scale, it moves only the picture; layout, the pointer and assistive
// technology see the child in place, so keep it for motions that end at zero.
// Clip keeps the painting inside the box, also at rest; to cut a child without
// moving it, use ClipRect.
type Translate struct {
	X, Y  float32
	Clip  bool
	Child core.Widget
}

// Key implements core.Widget.
func (Translate) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t Translate) CreateRenderObject() core.RenderObject {
	return &layout.RenderTranslate{X: t.X, Y: t.Y, Clip: t.Clip}
}

// UpdateRenderObject implements core.RenderWidget.
func (t Translate) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderTranslate)
	if r.X != t.X || r.Y != t.Y || r.Clip != t.Clip {
		r.X, r.Y, r.Clip = t.X, t.Y, t.Clip
		r.MarkNeedsPaint()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (t Translate) ChildWidget() core.Widget { return t.Child }
