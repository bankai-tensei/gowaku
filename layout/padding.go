package layout

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
)

// RenderPadding lays out a single child inset by Insets.
type RenderPadding struct {
	RenderProxy
	Insets Insets
}

// Layout implements core.RenderObject.
func (r *RenderPadding) Layout(c Constraints) geom.Size {
	if r.child == nil {
		return c.Constrain(r.Insets.Size())
	}
	sz := core.LayoutChild(r.child, c.Deflate(r.Insets))
	r.child.SetOffset(geom.Offset{X: r.Insets.Left, Y: r.Insets.Top})
	return c.Constrain(sz.Add(r.Insets.Size()))
}
