package layout

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
)

// Alignment is a point in a rectangle, as fractions of its width and height:
// 0 is the left or top edge, 0.5 the middle, 1 the right or bottom edge. The
// zero value is the top left corner.
//
// The range is 0..1, NOT -1..1 as in Flutter: the middle is 0.5, not 0, and
// Alignment{X: -1} lies a whole free width to the left of the box, not at its
// left edge. Prefer the named values AlignTopLeft .. AlignBottomRight to
// literals. Values outside 0..1 are allowed, because an AnimatedAlign with an
// overshooting curve such as motion.SpringOut passes through them on purpose.
type Alignment struct {
	// X is 0 at the left edge, 0.5 in the middle and 1 at the right edge.
	X float32
	// Y is 0 at the top edge, 0.5 in the middle and 1 at the bottom edge.
	Y float32
}

// The nine usual alignments. Use these instead of Alignment literals, whose
// range (0..1) differs from Flutter's (-1..1).
var (
	AlignTopLeft      = Alignment{X: 0, Y: 0}
	AlignTopCenter    = Alignment{X: 0.5, Y: 0}
	AlignTopRight     = Alignment{X: 1, Y: 0}
	AlignCenterLeft   = Alignment{X: 0, Y: 0.5}
	AlignCenter       = Alignment{X: 0.5, Y: 0.5}
	AlignCenterRight  = Alignment{X: 1, Y: 0.5}
	AlignBottomLeft   = Alignment{X: 0, Y: 1}
	AlignBottomCenter = Alignment{X: 0.5, Y: 1}
	AlignBottomRight  = Alignment{X: 1, Y: 1}
)

// Within returns the offset that places a child of size child in size outer.
func (a Alignment) Within(outer, child geom.Size) geom.Offset {
	return geom.Offset{
		X: (outer.W - child.W) * a.X,
		Y: (outer.H - child.H) * a.Y,
	}
}

// RenderAlign takes all the space it can and places its child in it. In an
// unbounded direction it shrinks to the child.
type RenderAlign struct {
	RenderProxy
	Alignment Alignment
}

// Layout implements core.RenderObject.
func (r *RenderAlign) Layout(c Constraints) geom.Size {
	if r.child == nil {
		return c.Constrain(boundedPart(c.Biggest(), geom.Size{}))
	}
	sz := core.LayoutChild(r.child, c.Loosen())
	own := c.Constrain(boundedPart(c.Biggest(), sz))
	r.child.SetOffset(r.Alignment.Within(own, sz))
	return own
}

// boundedPart returns limit, with unbounded lengths replaced by those of fallback.
func boundedPart(limit, fallback geom.Size) geom.Size {
	if limit.W >= geom.Inf {
		limit.W = fallback.W
	}
	if limit.H >= geom.Inf {
		limit.H = fallback.H
	}
	return limit
}
