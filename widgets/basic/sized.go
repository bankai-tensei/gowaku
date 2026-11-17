package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
)

// ConstrainedBox adds limits to the size of its child. A zero maximum means no
// limit, so the zero value changes nothing.
type ConstrainedBox struct {
	MinWidth, MaxWidth   float32
	MinHeight, MaxHeight float32
	Child                core.Widget
}

// Key implements core.Widget.
func (ConstrainedBox) Key() core.Key { return nil }

func (b ConstrainedBox) extra() layout.Constraints {
	c := layout.Constraints{MinW: b.MinWidth, MaxW: b.MaxWidth, MinH: b.MinHeight, MaxH: b.MaxHeight}
	if c.MaxW == 0 {
		c.MaxW = geom.Inf
	}
	if c.MaxH == 0 {
		c.MaxH = geom.Inf
	}
	return c
}

// CreateRenderObject implements core.RenderWidget.
func (b ConstrainedBox) CreateRenderObject() core.RenderObject {
	return &layout.RenderConstrainedBox{Extra: b.extra()}
}

// UpdateRenderObject implements core.RenderWidget.
func (b ConstrainedBox) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderConstrainedBox)
	if extra := b.extra(); r.Extra != extra {
		r.Extra = extra
		r.MarkNeedsLayout()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (b ConstrainedBox) ChildWidget() core.Widget { return b.Child }

// SizedBox forces a width, a height or both. A zero length leaves that axis
// free, so the zero value with no child is an empty space of no size.
type SizedBox struct {
	core.StatelessBase
	Width, Height float32
	Child         core.Widget
}

// Build implements core.StatelessWidget.
func (s SizedBox) Build(core.BuildContext) core.Widget {
	return ConstrainedBox{MinWidth: s.Width, MaxWidth: s.Width, MinHeight: s.Height, MaxHeight: s.Height, Child: s.Child}
}
