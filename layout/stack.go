package layout

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// StackFit decides the size of a RenderStack.
type StackFit uint8

// The stack fits.
const (
	// StackLoose shrinks to the largest unpositioned child.
	StackLoose StackFit = iota
	// StackExpand takes all the space allowed, and passes that on as tight.
	StackExpand
)

// RenderPositioned places its child at a fixed spot of a RenderStack. Like
// RenderFlexible, it is a marker for its parent.
type RenderPositioned struct {
	RenderProxy
	Left, Top, Right, Bottom float32
	// AnchorRight and AnchorBottom measure from the far edges, using Right and
	// Bottom instead of Left and Top.
	AnchorRight, AnchorBottom bool
	// Width and Height force the size of the child. Zero leaves it free.
	Width, Height float32
}

func (r *RenderPositioned) childConstraints() Constraints {
	c := Constraints{MaxW: geom.Inf, MaxH: geom.Inf}
	if r.Width > 0 {
		c.MinW, c.MaxW = r.Width, r.Width
	}
	if r.Height > 0 {
		c.MinH, c.MaxH = r.Height, r.Height
	}
	return c
}

func (r *RenderPositioned) offsetIn(stack, own geom.Size) geom.Offset {
	off := geom.Offset{X: r.Left, Y: r.Top}
	if r.AnchorRight {
		off.X = stack.W - r.Right - own.W
	}
	if r.AnchorBottom {
		off.Y = stack.H - r.Bottom - own.H
	}
	return off
}

// RenderPositionedFill stretches its child over a RenderStack, less Insets. The
// child gets tight constraints of the stack's size minus the insets, and sits at
// the top left inset. Like RenderPositioned, it plays no part in the stack's own
// size.
type RenderPositionedFill struct {
	RenderProxy
	Insets Insets
}

func (r *RenderPositionedFill) childConstraints(stack geom.Size) Constraints {
	return geom.Tight(geom.Size{
		W: max(0, stack.W-r.Insets.Left-r.Insets.Right),
		H: max(0, stack.H-r.Insets.Top-r.Insets.Bottom),
	})
}

// RenderStack paints its children on top of each other, the last one on top.
type RenderStack struct {
	core.RenderBase
	Alignment Alignment
	Fit       StackFit
	children  []core.RenderObject
}

// SetChildren implements core.MultiChildRenderObject.
func (r *RenderStack) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *RenderStack) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *RenderStack) ChildAt(i int) core.RenderObject { return r.children[i] }

// Layout implements core.RenderObject.
func (r *RenderStack) Layout(c Constraints) geom.Size {
	inner := c.Loosen()
	if r.Fit == StackExpand {
		inner = geom.Tight(c.Biggest())
	}
	var own geom.Size
	for _, child := range r.children {
		if positioned(child) {
			continue
		}
		sz := core.LayoutChild(child, inner)
		own = geom.Size{W: max(own.W, sz.W), H: max(own.H, sz.H)}
	}
	if r.Fit == StackExpand {
		own = c.Biggest()
	}
	own = c.Constrain(own)
	for _, child := range r.children {
		switch p := child.(type) {
		case *RenderPositioned:
			sz := core.LayoutChild(child, p.childConstraints())
			child.SetOffset(p.offsetIn(own, sz))
		case *RenderPositionedFill:
			core.LayoutChild(child, p.childConstraints(own))
			child.SetOffset(geom.Offset{X: p.Insets.Left, Y: p.Insets.Top})
		default:
			child.SetOffset(r.Alignment.Within(own, child.Size()))
		}
	}
	return own
}

// positioned reports whether child is placed by its own fields rather than by
// the stack's Alignment, and so leaves the stack's size alone.
func positioned(child core.RenderObject) bool {
	switch child.(type) {
	case *RenderPositioned, *RenderPositionedFill:
		return true
	}
	return false
}

// Paint implements core.RenderObject.
func (r *RenderStack) Paint(cv *paint.Canvas, off geom.Offset) {
	for _, child := range r.children {
		child.Paint(cv, off.Add(child.Offset()))
	}
}
