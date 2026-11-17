package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
)

// Align places its child within the space it is given.
// The zero value keeps it at the top left. Alignment runs 0..1, not -1..1 as
// in Flutter; use the named values such as layout.AlignCenterLeft.
type Align struct {
	Alignment layout.Alignment
	Child     core.Widget
}

// Key implements core.Widget.
func (Align) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (a Align) CreateRenderObject() core.RenderObject {
	return &layout.RenderAlign{Alignment: a.Alignment}
}

// UpdateRenderObject implements core.RenderWidget.
func (a Align) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderAlign)
	if r.Alignment != a.Alignment {
		r.Alignment = a.Alignment
		r.MarkNeedsLayout()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (a Align) ChildWidget() core.Widget { return a.Child }

// Center places its child in the middle of the space it is given. The child may
// take less than that space, not more. A Column or Row takes all it may unless
// its Size is MainMin, so it fills the Center instead of sitting in its middle;
// VStack and HStack shrink to their children.
type Center struct {
	core.StatelessBase
	Child core.Widget
}

// Build implements core.StatelessWidget.
func (c Center) Build(core.BuildContext) core.Widget {
	return Align{Alignment: layout.AlignCenter, Child: c.Child}
}
