package kit

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// The glyphs of the fields are drawn on the 24 unit grid of Icon. They are
// paths, not text, because no font is sure to have them, and each State makes
// its own once, because the package keeps no variables.

// GlyphSize is the size of the glyphs inside a field.
const GlyphSize = 16

// MagnifierGlyph returns the lens and handle of a search field.
func MagnifierGlyph() *paint.Path {
	p := &paint.Path{}
	const cx, cy float32 = 10.5, 10.5
	circle(p, cx, cy, 6.6, true)
	circle(p, cx, cy, 4.7, false)
	// The handle starts inside the rim, where the fills add up, and ends in a
	// round cap.
	const s = 0.70710677
	const from, to, half float32 = 5.6, 11.6, 1.05
	ax, ay := cx+s*from, cy+s*from
	bx, by := cx+s*to, cy+s*to
	nx, ny := -s*half, s*half
	p.MoveTo(ax-nx, ay-ny)
	p.LineTo(bx-nx, by-ny)
	p.LineTo(bx+nx, by+ny)
	p.LineTo(ax+nx, ay+ny)
	p.Close()
	circle(p, bx, by, half, true)
	return p
}

// ChevronGlyph returns the downward chevron of a dropdown.
func ChevronGlyph() *paint.Path {
	p := &paint.Path{}
	p.MoveTo(6.3, 8.3)
	p.LineTo(12, 14)
	p.LineTo(17.7, 8.3)
	p.LineTo(19.1, 9.7)
	p.LineTo(12, 16.8)
	p.LineTo(4.9, 9.7)
	p.Close()
	return p
}

// ClearGlyph returns the disc with a cross cut out that clears a search field.
func ClearGlyph() *paint.Path {
	p := &paint.Path{Rule: paint.FillEvenOdd}
	circle(p, 12, 12, 8, true)
	// A plus sign turned by 45 degrees.
	const a, t float32 = 4.2, 0.95
	plus := [...]geom.Offset{
		{X: t, Y: -a}, {X: t, Y: -t}, {X: a, Y: -t}, {X: a, Y: t}, {X: t, Y: t}, {X: t, Y: a},
		{X: -t, Y: a}, {X: -t, Y: t}, {X: -a, Y: t}, {X: -a, Y: -t}, {X: -t, Y: -t}, {X: -t, Y: -a},
	}
	const s float32 = 0.70710677
	for i, q := range plus {
		x, y := 12+(q.X-q.Y)*s, 12+(q.X+q.Y)*s
		if i == 0 {
			p.MoveTo(x, y)
		} else {
			p.LineTo(x, y)
		}
	}
	p.Close()
	return p
}

// circle adds a circle to p, clockwise on screen or not, so that holes can be
// cut with the nonzero rule.
func circle(p *paint.Path, cx, cy, r float32, clockwise bool) {
	k := r * 0.5522848
	p.MoveTo(cx, cy-r)
	if clockwise {
		p.CubeTo(cx+k, cy-r, cx+r, cy-k, cx+r, cy)
		p.CubeTo(cx+r, cy+k, cx+k, cy+r, cx, cy+r)
		p.CubeTo(cx-k, cy+r, cx-r, cy+k, cx-r, cy)
		p.CubeTo(cx-r, cy-k, cx-k, cy-r, cx, cy-r)
	} else {
		p.CubeTo(cx-k, cy-r, cx-r, cy-k, cx-r, cy)
		p.CubeTo(cx-r, cy+k, cx-k, cy+r, cx, cy+r)
		p.CubeTo(cx+k, cy+r, cx+r, cy+k, cx+r, cy)
		p.CubeTo(cx+r, cy-k, cx+k, cy-r, cx, cy-r)
	}
	p.Close()
}
