// Package frame holds what the Linux platforms share about windows whose title
// bar the application draws: which edge of the window the pointer resizes.
// platform/x11 and platform/wayland use it.
package frame

// Edge is a set of the sides of a window. The values are those of the
// resize_edge of xdg_toplevel.
type Edge uint32

// The edges.
const (
	None        Edge = 0
	Top         Edge = 1
	Bottom      Edge = 2
	Left        Edge = 4
	TopLeft     Edge = 5
	BottomLeft  Edge = 6
	Right       Edge = 8
	TopRight    Edge = 9
	BottomRight Edge = 10
)

// Border is how far inside its edges a window drawn by the application takes
// the pointer to resize, in logical pixels. It is wider at the corners.
const (
	Border = 5
	Corner = 12
)

// At returns the edge of a w×h window that resizes at (x, y), all logical.
func At(x, y, w, h float64) Edge {
	if x < 0 || y < 0 || x >= w || y >= h {
		return None
	}
	var e Edge
	switch {
	case y < Border:
		e |= Top
	case y >= h-Border:
		e |= Bottom
	}
	switch {
	case x < Border:
		e |= Left
	case x >= w-Border:
		e |= Right
	}
	// Near a corner, a border counts for both sides.
	if e == Top || e == Bottom {
		if x < Corner {
			e |= Left
		} else if x >= w-Corner {
			e |= Right
		}
	}
	if e == Left || e == Right {
		if y < Corner {
			e |= Top
		} else if y >= h-Corner {
			e |= Bottom
		}
	}
	return e
}
