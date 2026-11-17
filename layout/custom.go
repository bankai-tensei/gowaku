package layout

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// Delegate is the algorithm of a RenderCustomLayout.
type Delegate interface {
	// Layout sizes children with core.LayoutChild, positions them with
	// SetOffset, and returns the size of the whole.
	Layout(c Constraints, children []core.RenderObject) geom.Size
}

// RenderCustomLayout hands its layout to a Delegate. A nil Delegate takes the
// smallest size allowed and leaves the children unplaced.
type RenderCustomLayout struct {
	core.RenderBase
	Delegate Delegate
	children []core.RenderObject
}

// SetChildren implements core.MultiChildRenderObject.
func (r *RenderCustomLayout) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *RenderCustomLayout) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *RenderCustomLayout) ChildAt(i int) core.RenderObject { return r.children[i] }

// Layout implements core.RenderObject.
func (r *RenderCustomLayout) Layout(c Constraints) geom.Size {
	if r.Delegate == nil {
		for _, child := range r.children {
			core.LayoutChild(child, c.Loosen())
		}
		return c.Constrain(geom.Size{})
	}
	return c.Constrain(r.Delegate.Layout(c, r.children))
}

// Paint implements core.RenderObject.
func (r *RenderCustomLayout) Paint(cv *paint.Canvas, off geom.Offset) {
	for _, child := range r.children {
		child.Paint(cv, off.Add(child.Offset()))
	}
}
