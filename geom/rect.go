package geom

// Rect is an axis-aligned rectangle with its origin at the top left.
type Rect struct {
	X, Y, W, H float32
}

// RectFromSize returns the rectangle of size s at the origin.
func RectFromSize(s Size) Rect { return Rect{W: s.W, H: s.H} }

// Origin returns the top-left corner of r.
func (r Rect) Origin() Offset { return Offset{r.X, r.Y} }

// Size returns the width and height of r.
func (r Rect) Size() Size { return Size{r.W, r.H} }

// Center returns the midpoint of r.
func (r Rect) Center() Offset { return Offset{r.X + r.W/2, r.Y + r.H/2} }

// Translate returns r moved by d.
func (r Rect) Translate(d Offset) Rect { return Rect{r.X + d.X, r.Y + d.Y, r.W, r.H} }

// Scale returns r with its position and size multiplied by s.
func (r Rect) Scale(s float32) Rect { return Rect{r.X * s, r.Y * s, r.W * s, r.H * s} }

// Contains reports whether p lies inside r. The right and bottom edges are exclusive.
func (r Rect) Contains(p Offset) bool {
	return p.X >= r.X && p.Y >= r.Y && p.X < r.X+r.W && p.Y < r.Y+r.H
}

// Intersect returns the overlap of r and o. It is empty when they do not overlap.
func (r Rect) Intersect(o Rect) Rect {
	x0, y0 := max(r.X, o.X), max(r.Y, o.Y)
	x1, y1 := min(r.X+r.W, o.X+o.W), min(r.Y+r.H, o.Y+o.H)
	if x1 <= x0 || y1 <= y0 {
		return Rect{}
	}
	return Rect{x0, y0, x1 - x0, y1 - y0}
}

// RRect is a rectangle with an independent circular radius at each corner.
type RRect struct {
	Rect                                       Rect
	TopLeft, TopRight, BottomRight, BottomLeft float32
}

// RRectAll returns r with the same radius at all four corners.
func RRectAll(r Rect, radius float32) RRect {
	return RRect{Rect: r, TopLeft: radius, TopRight: radius, BottomRight: radius, BottomLeft: radius}
}
