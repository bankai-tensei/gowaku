package geom

import "math"

// Inf is the length of an unbounded constraint.
const Inf float32 = math.MaxFloat32

// Insets are distances from the four edges of a rectangle.
type Insets struct {
	Left, Top, Right, Bottom float32
}

// All returns insets of v on every edge.
func All(v float32) Insets { return Insets{v, v, v, v} }

// Symmetric returns insets of horizontal on the left and right, and vertical on
// the top and bottom.
func Symmetric(horizontal, vertical float32) Insets {
	return Insets{horizontal, vertical, horizontal, vertical}
}

// Size returns the total width and height that i takes up.
func (i Insets) Size() Size { return Size{i.Left + i.Right, i.Top + i.Bottom} }

// Constraints are the sizes a parent allows its child to take.
// Use Inf for an unbounded maximum.
type Constraints struct {
	MinW, MaxW, MinH, MaxH float32
}

// Tight returns constraints that allow only s.
func Tight(s Size) Constraints { return Constraints{s.W, s.W, s.H, s.H} }

// Loose returns constraints that allow anything from zero up to s.
func Loose(s Size) Constraints { return Constraints{MaxW: s.W, MaxH: s.H} }

// IsTight reports whether c allows exactly one size.
func (c Constraints) IsTight() bool { return c.MinW >= c.MaxW && c.MinH >= c.MaxH }

// Constrain returns the size allowed by c that is nearest to s.
func (c Constraints) Constrain(s Size) Size {
	return Size{Clamp32(s.W, c.MinW, c.MaxW), Clamp32(s.H, c.MinH, c.MaxH)}
}

// Biggest returns the largest size c allows.
func (c Constraints) Biggest() Size { return Size{c.MaxW, c.MaxH} }

// Loosen returns c with its minimums removed.
func (c Constraints) Loosen() Constraints { return Constraints{MaxW: c.MaxW, MaxH: c.MaxH} }

// Deflate returns the constraints for what is left inside i.
func (c Constraints) Deflate(i Insets) Constraints {
	inset := i.Size()
	return Constraints{
		MinW: max(c.MinW-inset.W, 0),
		MaxW: shrink(c.MaxW, inset.W),
		MinH: max(c.MinH-inset.H, 0),
		MaxH: shrink(c.MaxH, inset.H),
	}
}

// Enforce returns o limited to what c allows, so that c always wins.
func (c Constraints) Enforce(o Constraints) Constraints {
	return Constraints{
		MinW: Clamp32(o.MinW, c.MinW, c.MaxW),
		MaxW: Clamp32(o.MaxW, c.MinW, c.MaxW),
		MinH: Clamp32(o.MinH, c.MinH, c.MaxH),
		MaxH: Clamp32(o.MaxH, c.MinH, c.MaxH),
	}
}

func shrink(limit, by float32) float32 {
	if limit >= Inf {
		return Inf
	}
	return max(limit-by, 0)
}
