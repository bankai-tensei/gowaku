package layout

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
)

// RenderConstrainedBox adds Extra to the constraints from its parent, which
// take priority where the two conflict.
type RenderConstrainedBox struct {
	RenderProxy
	Extra Constraints
}

// Layout implements core.RenderObject.
func (r *RenderConstrainedBox) Layout(c Constraints) geom.Size {
	inner := c.Enforce(r.Extra)
	if r.child == nil {
		return inner.Constrain(geom.Size{})
	}
	return core.LayoutChild(r.child, inner)
}
