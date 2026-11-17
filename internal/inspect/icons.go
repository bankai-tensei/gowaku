//go:build gowakudebug

package inspect

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// The icons of the toolbar, on the 24 unit grid of ToolbarItem.Icon. They are
// drawn as thin outlines, the weight of the other icons of the widgets, and
// never change once made.
var (
	iconPick     = pickIcon()
	iconBounds   = boundsIcon()
	iconRepaints = repaintsIcon()
	iconPause    = pauseIcon()
)

// iconLine is the weight of the outlines, in units of the grid.
const iconLine = 1.6

// pickIcon is a pointer over the corners of a box: click what you want to see.
func pickIcon() *paint.Path {
	p := &paint.Path{}
	const lo, hi, arm, t = 3, 21, 5, iconLine
	// The corners of the box, but the one the pointer takes.
	corner(p, lo, lo, arm, arm, t)
	corner(p, hi, lo, -arm, arm, t)
	corner(p, lo, hi, arm, -arm, t)
	// The pointer, its tip at the middle of the box.
	p.MoveTo(11, 11)
	p.LineTo(21.5, 14.6)
	p.LineTo(16.9, 16.9)
	p.LineTo(14.6, 21.5)
	p.Close()
	return p
}

// boundsIcon is a box in a box: the outlines of the layout.
func boundsIcon() *paint.Path {
	p := &paint.Path{Rule: paint.FillEvenOdd}
	ring(p, geom.Rect{X: 3, Y: 3, W: 18, H: 18}, 2.5, iconLine)
	ring(p, geom.Rect{X: 8, Y: 8, W: 8, H: 8}, 1, iconLine)
	return p
}

// repaintsIcon is a box that flashes at its corner: a layer recorded again.
func repaintsIcon() *paint.Path {
	p := &paint.Path{Rule: paint.FillEvenOdd}
	ring(p, geom.Rect{X: 3, Y: 8, W: 13, H: 13}, 2, iconLine)
	// The rays of the flash, out of the top right corner of the box. They do
	// not touch, as overlaps would leave holes under FillEvenOdd.
	const t = iconLine / 2
	bar(p, 16, 5.8, 16, 2.2, t)
	bar(p, 17.56, 6.44, 20.1, 3.9, t)
	bar(p, 18.2, 8, 21.8, 8, t)
	return p
}

// pauseIcon is two upright bars.
func pauseIcon() *paint.Path {
	p := &paint.Path{}
	p.AddRRect(geom.RRectAll(geom.Rect{X: 7, Y: 5, W: 3, H: 14}, 1))
	p.AddRRect(geom.RRectAll(geom.Rect{X: 14, Y: 5, W: 3, H: 14}, 1))
	return p
}

// ring adds the outline of r, t thick inside its edge, with corners of radius
// rad. The path must fill by FillEvenOdd for the inside to stay empty.
func ring(p *paint.Path, r geom.Rect, rad, t float32) {
	p.AddRRect(geom.RRectAll(r, rad))
	in := geom.Rect{X: r.X + t, Y: r.Y + t, W: r.W - 2*t, H: r.H - 2*t}
	p.AddRRect(geom.RRectAll(in, max(rad-t, 0)))
}

// corner adds an L, t thick, whose bend is at (x, y) and whose arms reach dx
// across and dy down; negative lengths reach the other way.
func corner(p *paint.Path, x, y, dx, dy, t float32) {
	sx, sy := sign(dx)*t, sign(dy)*t
	p.MoveTo(x, y)
	p.LineTo(x+dx, y)
	p.LineTo(x+dx, y+sy)
	p.LineTo(x+sx, y+sy)
	p.LineTo(x+sx, y+dy)
	p.LineTo(x, y+dy)
	p.Close()
}

// bar adds a line from (x0, y0) to (x1, y1), with square ends, h thick on each
// side.
func bar(p *paint.Path, x0, y0, x1, y1, h float32) {
	dx, dy := x1-x0, y1-y0
	n := h / geom.Sqrt32(dx*dx+dy*dy)
	nx, ny := -dy*n, dx*n
	p.MoveTo(x0+nx, y0+ny)
	p.LineTo(x1+nx, y1+ny)
	p.LineTo(x1-nx, y1-ny)
	p.LineTo(x0-nx, y0-ny)
	p.Close()
}

func sign(v float32) float32 {
	if v < 0 {
		return -1
	}
	return 1
}
