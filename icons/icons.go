package icons

import (
	"math"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// lineWidth is the width of the line of every icon, in units of the 24 unit
// grid: at 16 pixels it is a little over one pixel, as the hairlines of the
// design are.
const lineWidth = 1.75

// dotRadius is the radius of the dot of Info, Warning and Error, a little
// larger than the line so that it reads as a dot; moreRadius that of the dots
// of More, which stand alone.
const (
	dotRadius  = 1.25
	moreRadius = 1.5
)

// pen collects the lines of an icon and turns them into the outline to fill.
type pen struct {
	lines paint.Path
	dots  []geom.Offset
	r     float32
}

// open adds a line through the points, given as x, y pairs.
func (p *pen) open(xy ...float32) {
	p.lines.MoveTo(xy[0], xy[1])
	for i := 2; i+1 < len(xy); i += 2 {
		p.lines.LineTo(xy[i], xy[i+1])
	}
}

// closed adds a closed line through the points, given as x, y pairs.
func (p *pen) closed(xy ...float32) {
	p.open(xy...)
	p.lines.Close()
}

// box adds a rounded rectangle.
func (p *pen) box(x, y, w, h, r float32) {
	p.lines.AddRRect(geom.RRectAll(geom.Rect{X: x, Y: y, W: w, H: h}, r))
}

// circle adds a circle around (cx, cy).
func (p *pen) circle(cx, cy, r float32) { p.box(cx-r, cy-r, 2*r, 2*r, r) }

// dot adds a filled dot of radius r at (x, y).
func (p *pen) dot(x, y, r float32) {
	p.dots = append(p.dots, geom.Offset{X: x, Y: y})
	p.r = r
}

// path returns the outline of the lines, with the dots.
func (p *pen) path() *paint.Path {
	out := p.lines.Stroke(paint.StrokeStyle{Width: lineWidth, Cap: paint.CapRound, Join: paint.JoinRound})
	for _, d := range p.dots {
		out.AddRRect(geom.RRectAll(geom.Rect{X: d.X - p.r, Y: d.Y - p.r, W: 2 * p.r, H: 2 * p.r}, p.r))
	}
	return out
}

// polar returns the point at radius r and angle a, in degrees clockwise from
// the top, around (cx, cy).
func polar(cx, cy, r, a float32) (float32, float32) {
	rad := a * math.Pi / 180
	return cx + r*geom.Sin32(rad), cy - r*geom.Cos32(rad)
}

// Folder returns a folder with a tab at its top left.
func Folder() *paint.Path {
	var p pen
	l := &p.lines
	l.MoveTo(3, 6.5)
	l.QuadTo(3, 5, 4.5, 5)
	l.LineTo(9, 5)
	l.LineTo(11, 7.5)
	l.LineTo(19.5, 7.5)
	l.QuadTo(21, 7.5, 21, 9)
	l.LineTo(21, 17.5)
	l.QuadTo(21, 19, 19.5, 19)
	l.LineTo(4.5, 19)
	l.QuadTo(3, 19, 3, 17.5)
	l.Close()
	return p.path()
}

// page adds a sheet with its top right corner folded.
func (p *pen) page() {
	l := &p.lines
	l.MoveTo(14, 3)
	l.LineTo(19, 8)
	l.LineTo(19, 19.5)
	l.QuadTo(19, 21, 17.5, 21)
	l.LineTo(6.5, 21)
	l.QuadTo(5, 21, 5, 19.5)
	l.LineTo(5, 4.5)
	l.QuadTo(5, 3, 6.5, 3)
	l.Close()
	p.open(14, 3, 14, 8, 19, 8)
}

// File returns a blank sheet with a folded corner.
func File() *paint.Path {
	var p pen
	p.page()
	return p.path()
}

// Document returns a sheet with lines of text.
func Document() *paint.Path {
	var p pen
	p.page()
	p.open(8.5, 12.5, 15.5, 12.5)
	p.open(8.5, 16, 15.5, 16)
	return p.path()
}

// Image returns a picture: hills and a sun in a frame.
func Image() *paint.Path {
	var p pen
	p.box(3.5, 4.5, 17, 15, 2)
	p.circle(9, 9.5, 1.5)
	p.open(3.5, 17, 9, 12, 13, 15.5, 16, 13, 20.5, 17)
	return p.path()
}

// Search returns a magnifying glass.
func Search() *paint.Path {
	var p pen
	p.circle(10.5, 10.5, 6.5)
	p.open(15.5, 15.5, 20.5, 20.5)
	return p.path()
}

// Plus returns a plus sign, as for adding.
func Plus() *paint.Path {
	var p pen
	p.open(12, 5, 12, 19)
	p.open(5, 12, 19, 12)
	return p.path()
}

// Minus returns a minus sign, as for removing.
func Minus() *paint.Path {
	var p pen
	p.open(5, 12, 19, 12)
	return p.path()
}

// Close returns a cross, as for closing or clearing.
func Close() *paint.Path {
	var p pen
	p.open(6, 6, 18, 18)
	p.open(18, 6, 6, 18)
	return p.path()
}

// Check returns a check mark.
func Check() *paint.Path {
	var p pen
	p.open(5, 12.5, 10, 17.5, 19, 7)
	return p.path()
}

// ChevronLeft returns a chevron that points left, as for going back.
func ChevronLeft() *paint.Path {
	var p pen
	p.open(15, 5.5, 8.5, 12, 15, 18.5)
	return p.path()
}

// ChevronRight returns a chevron that points right, as between the items of a
// breadcrumb.
func ChevronRight() *paint.Path {
	var p pen
	p.open(9, 5.5, 15.5, 12, 9, 18.5)
	return p.path()
}

// ChevronUp returns a chevron that points up.
func ChevronUp() *paint.Path {
	var p pen
	p.open(5.5, 15, 12, 8.5, 18.5, 15)
	return p.path()
}

// ChevronDown returns a chevron that points down, as for opening a list.
func ChevronDown() *paint.Path {
	var p pen
	p.open(5.5, 9, 12, 15.5, 18.5, 9)
	return p.path()
}

// Settings returns a gear with eight teeth.
func Settings() *paint.Path {
	var p pen
	const cx, cy, outer, inner = 12, 12, 9, 6.75
	xy := make([]float32, 0, 8*4*2)
	for k := range 8 {
		a := float32(k) * 45
		for _, v := range [...]struct{ r, da float32 }{{inner, -14}, {outer, -8}, {outer, 8}, {inner, 14}} {
			x, y := polar(cx, cy, v.r, a+v.da)
			xy = append(xy, x, y)
		}
	}
	p.closed(xy...)
	p.circle(cx, cy, 2.75)
	return p.path()
}

// Trash returns a waste bin with its lid, as for deleting.
func Trash() *paint.Path {
	var p pen
	p.open(4, 6.5, 20, 6.5)
	l := &p.lines
	l.MoveTo(9.5, 6.5)
	l.LineTo(9.5, 4.5)
	l.QuadTo(9.5, 3.5, 10.5, 3.5)
	l.LineTo(13.5, 3.5)
	l.QuadTo(14.5, 3.5, 14.5, 4.5)
	l.LineTo(14.5, 6.5)
	l.MoveTo(6, 6.5)
	l.LineTo(7, 19)
	l.QuadTo(7.1, 20.5, 8.5, 20.5)
	l.LineTo(15.5, 20.5)
	l.QuadTo(16.9, 20.5, 17, 19)
	l.LineTo(18, 6.5)
	p.open(10, 10.5, 10, 16.5)
	p.open(14, 10.5, 14, 16.5)
	return p.path()
}

// Edit returns a pencil, as for editing or renaming.
func Edit() *paint.Path {
	var p pen
	p.closed(15.5, 4.5, 19.5, 8.5, 8.5, 19.5, 4, 20, 4.5, 15.5)
	p.open(13, 7, 17, 11)
	return p.path()
}

// Copy returns two sheets, one over the other.
func Copy() *paint.Path {
	var p pen
	p.box(8.5, 8.5, 11.5, 11.5, 2)
	l := &p.lines
	l.MoveTo(15.5, 8.5)
	l.LineTo(15.5, 5.5)
	l.QuadTo(15.5, 4, 14, 4)
	l.LineTo(5.5, 4)
	l.QuadTo(4, 4, 4, 5.5)
	l.LineTo(4, 14)
	l.QuadTo(4, 15.5, 5.5, 15.5)
	l.LineTo(8.5, 15.5)
	return p.path()
}

// Home returns a house.
func Home() *paint.Path {
	var p pen
	p.open(3.5, 11, 12, 4, 20.5, 11)
	l := &p.lines
	l.MoveTo(6, 9.5)
	l.LineTo(6, 19)
	l.QuadTo(6, 20, 7, 20)
	l.LineTo(17, 20)
	l.QuadTo(18, 20, 18, 19)
	l.LineTo(18, 9.5)
	p.open(10, 20, 10, 14.5, 14, 14.5, 14, 20)
	return p.path()
}

// Star returns a five-pointed star, as for favorites.
func Star() *paint.Path {
	var p pen
	const cx, cy, outer, inner = 12, 12.75, 9, 3.9
	xy := make([]float32, 0, 20)
	for k := range 10 {
		r := float32(outer)
		if k%2 == 1 {
			r = inner
		}
		x, y := polar(cx, cy, r, float32(k)*36)
		xy = append(xy, x, y)
	}
	p.closed(xy...)
	return p.path()
}

// Info returns an "i" in a circle, for information.
func Info() *paint.Path {
	var p pen
	p.circle(12, 12, 9)
	p.open(12, 11.5, 12, 16.5)
	p.dot(12, 8, dotRadius)
	return p.path()
}

// Warning returns an exclamation mark in a triangle, for a warning.
func Warning() *paint.Path {
	var p pen
	p.closed(12, 3.75, 21, 19.75, 3, 19.75)
	p.open(12, 9.5, 12, 14)
	p.dot(12, 17, dotRadius)
	return p.path()
}

// Error returns an exclamation mark in a circle, for an error.
func Error() *paint.Path {
	var p pen
	p.circle(12, 12, 9)
	p.open(12, 7.5, 12, 12.5)
	p.dot(12, 16, dotRadius)
	return p.path()
}

// Menu returns three lines, as for a menu that opens.
func Menu() *paint.Path {
	var p pen
	p.open(4, 7, 20, 7)
	p.open(4, 12, 20, 12)
	p.open(4, 17, 20, 17)
	return p.path()
}

// More returns three dots in a row, as for more actions.
func More() *paint.Path {
	var p pen
	p.dot(5.5, 12, moreRadius)
	p.dot(12, 12, moreRadius)
	p.dot(18.5, 12, moreRadius)
	return p.path()
}
