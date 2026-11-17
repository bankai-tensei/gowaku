package text

import (
	"math"
	"sort"

	"github.com/go-text/typesetting/font/opentype/tables"
)

// colorStop is a resolved stop of a color line.
type colorStop struct {
	offset float32
	color  rgba
}

// colorLine is a resolved color line of a gradient.
type colorLine struct {
	extend tables.Extend
	stops  []colorStop
}

func (cp *colrPainter) colorLine(extend tables.Extend, n int, stop func(i int) (offset float32, palette uint16, alpha float32)) colorLine {
	cl := colorLine{extend: extend, stops: make([]colorStop, n)}
	for i := range n {
		off, idx, a := stop(i)
		cl.stops[i] = colorStop{offset: off, color: cp.color(idx, a)}
	}
	sort.SliceStable(cl.stops, func(i, j int) bool { return cl.stops[i].offset < cl.stops[j].offset })
	return cl
}

func (cp *colrPainter) plainLine(l tables.ColorLine) colorLine {
	return cp.colorLine(l.Extend, len(l.ColorStops), func(i int) (float32, uint16, float32) {
		s := l.ColorStops[i]
		return f214(s.StopOffset), s.PaletteIndex, f214(s.Alpha)
	})
}

func (cp *colrPainter) varLine(l tables.VarColorLine) colorLine {
	return cp.colorLine(l.Extend, len(l.ColorStops), func(i int) (float32, uint16, float32) {
		s := l.ColorStops[i]
		return f214(s.StopOffset), s.PaletteIndex, f214(s.Alpha)
	})
}

// at returns the color at t, extended past the stops as the line says.
func (cl *colorLine) at(t float32) rgba {
	n := len(cl.stops)
	if n == 0 {
		return rgba{}
	}
	first, last := cl.stops[0].offset, cl.stops[n-1].offset
	if span := last - first; span > 0 {
		switch cl.extend {
		case tables.ExtendRepeat:
			u := (t - first) / span
			t = first + (u-float32(math.Floor(float64(u))))*span
		case tables.ExtendReflect:
			u := (t - first) / span
			k := float32(math.Floor(float64(u)))
			frac := u - k
			if int(k)%2 != 0 {
				frac = 1 - frac
			}
			t = first + frac*span
		}
	}
	if t <= first {
		return cl.stops[0].color
	}
	if t >= last {
		return cl.stops[n-1].color
	}
	for i := 1; i < n; i++ {
		a, b := cl.stops[i-1], cl.stops[i]
		if t <= b.offset {
			if b.offset == a.offset {
				return b.color
			}
			f := (t - a.offset) / (b.offset - a.offset)
			var c rgba
			for k := range 4 {
				c[k] = a.color[k] + (b.color[k]-a.color[k])*f
			}
			return c
		}
	}
	return cl.stops[n-1].color
}

// gradient fills dst with a linear, radial or sweep gradient. The positions of
// a gradient are in font units, so every pixel center maps back through m.
func (cp *colrPainter) gradient(p tables.PaintTable, m affine, dst *colorLayer) {
	inv, ok := m.invert()
	if !ok {
		return
	}
	var line colorLine
	var param func(x, y float32) (float32, bool)
	switch p := p.(type) {
	case tables.PaintLinearGradient:
		line = cp.plainLine(p.ColorLine)
		param = linearParam(p.X0, p.Y0, p.X1, p.Y1, p.X2, p.Y2)
	case tables.PaintVarLinearGradient:
		line = cp.varLine(p.ColorLine)
		param = linearParam(p.X0, p.Y0, p.X1, p.Y1, p.X2, p.Y2)
	case tables.PaintRadialGradient:
		line = cp.plainLine(p.ColorLine)
		param = radialParam(p.X0, p.Y0, p.Radius0, p.X1, p.Y1, p.Radius1)
	case tables.PaintVarRadialGradient:
		line = cp.varLine(p.ColorLine)
		param = radialParam(p.X0, p.Y0, p.Radius0, p.X1, p.Y1, p.Radius1)
	case tables.PaintSweepGradient:
		line = cp.plainLine(p.ColorLine)
		param = sweepParam(p.CenterX, p.CenterY, p.StartAngle, p.EndAngle)
	case tables.PaintVarSweepGradient:
		line = cp.varLine(p.ColorLine)
		param = sweepParam(p.CenterX, p.CenterY, p.StartAngle, p.EndAngle)
	default:
		return
	}
	for y := range dst.h {
		for x := range dst.w {
			fx, fy := inv.apply(float32(x)+0.5, float32(y)+0.5)
			t, ok := param(fx, fy)
			if !ok {
				continue
			}
			c := line.at(t)
			d := dst.pix[(y*dst.w+x)*4:][:4]
			inv := 1 - c[3]
			for k := range 4 {
				d[k] = c[k] + d[k]*inv
			}
		}
	}
}

// linearParam follows the COLRv1 definition: p0 to p1 is the color line, turned
// so that lines of equal color run parallel to p0p2.
func linearParam(x0, y0, x1, y1, x2, y2 int16) func(x, y float32) (float32, bool) {
	p0x, p0y := float32(x0), float32(y0)
	// n is perpendicular to p0p2, and p3 the projection of p1 onto it.
	nx, ny := -(float32(y2) - p0y), float32(x2)-p0x
	dx, dy := float32(x1)-p0x, float32(y1)-p0y
	if nn := nx*nx + ny*ny; nn > 0 {
		k := (dx*nx + dy*ny) / nn
		dx, dy = nx*k, ny*k
	}
	l2 := dx*dx + dy*dy
	return func(x, y float32) (float32, bool) {
		if l2 == 0 {
			return 0, false
		}
		return ((x-p0x)*dx + (y-p0y)*dy) / l2, true
	}
}

// radialParam solves the two-point conical gradient: the largest t whose circle
// c(t), r(t) ≥ 0 passes through the point.
func radialParam(x0, y0 int16, r0 uint16, x1, y1 int16, r1 uint16) func(x, y float32) (float32, bool) {
	cx, cy, cr := float64(x0), float64(y0), float64(r0)
	dx, dy, dr := float64(x1)-cx, float64(y1)-cy, float64(r1)-cr
	a := dx*dx + dy*dy - dr*dr
	return func(x, y float32) (float32, bool) {
		px, py := float64(x)-cx, float64(y)-cy
		b := px*dx + py*dy + cr*dr
		c := px*px + py*py - cr*cr
		if math.Abs(a) < 1e-9 {
			if b == 0 {
				return 0, false
			}
			t := c / (2 * b)
			return float32(t), cr+t*dr >= 0
		}
		disc := b*b - a*c
		if disc < 0 {
			return 0, false
		}
		sq := math.Sqrt(disc)
		for _, t := range [2]float64{(b + sq) / a, (b - sq) / a} {
			if cr+t*dr >= 0 {
				return float32(t), true
			}
		}
		return 0, false
	}
}

// sweepParam measures the angle around the center, counterclockwise in font
// space, against the range from start to end.
func sweepParam(cx, cy int16, start, end tables.Fixed214) func(x, y float32) (float32, bool) {
	a0 := (float64(f214(start)) + 1) * 180
	a1 := (float64(f214(end)) + 1) * 180
	return func(x, y float32) (float32, bool) {
		if a1 == a0 {
			return 0, false
		}
		deg := math.Atan2(float64(y)-float64(cy), float64(x)-float64(cx)) * 180 / math.Pi
		if deg < 0 {
			deg += 360
		}
		return float32((deg - a0) / (a1 - a0)), true
	}
}
