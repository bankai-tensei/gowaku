package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
)

// Padding insets its child by the given Insets.
// The zero value draws the child without any inset.
type Padding struct {
	Insets layout.Insets
	Child  core.Widget
}

// Key implements core.Widget. Padding is never keyed directly; wrap it with Keyed.
func (Padding) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (p Padding) CreateRenderObject() core.RenderObject {
	return &layout.RenderPadding{Insets: p.Insets}
}

// UpdateRenderObject implements core.RenderWidget.
func (p Padding) UpdateRenderObject(ro core.RenderObject) {
	rp := ro.(*layout.RenderPadding)
	if rp.Insets != p.Insets {
		rp.Insets = p.Insets
		rp.MarkNeedsLayout()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (p Padding) ChildWidget() core.Widget { return p.Child }
