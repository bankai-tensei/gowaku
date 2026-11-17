package paint

import (
	"math"

	"github.com/bankai-tensei/gowaku/geom"
)

// Angles are in radians and measured from the positive x axis. The y axis
// points down, so a positive sweep turns clockwise on the screen, and -π/2 is
// twelve o'clock.

// AddArc starts a new contour with the arc of the circle around center from
// angle start through sweep, as cubic curves (one per quarter turn or less). The
// contour stays open: continue it with LineTo or ArcTo, or Close it. A sweep
// beyond a whole turn is cut to one.
//
// A slice of a donut is one arc out and one back:
//
//	p.AddArc(c, outer, start, sweep)
//	p.ArcTo(c, inner, start+sweep, -sweep)
//	p.Close()
func (p *Path) AddArc(center geom.Offset, radius, start, sweep float32) {
	x, y := arcPoint(center, radius, float64(start))
	p.MoveTo(x, y)
	p.arc(center, radius, float64(start), clampSweep(sweep))
}

// ArcTo continues the current contour with the arc of the circle around center
// from angle start through sweep, with a line from the current point to the
// start of the arc when it is elsewhere. On an empty path, or after Close, it
// starts a new contour as AddArc does.
func (p *Path) ArcTo(center geom.Offset, radius, start, sweep float32) {
	x, y := arcPoint(center, radius, float64(start))
	if len(p.verbs) == 0 || p.verbs[len(p.verbs)-1] == verbClose {
		p.MoveTo(x, y)
	} else if last := p.points[len(p.points)-1]; geom.Abs32(last.X-x) > arcJoin || geom.Abs32(last.Y-y) > arcJoin {
		p.LineTo(x, y)
	}
	p.arc(center, radius, float64(start), clampSweep(sweep))
}

// AddOval adds the ellipse that fills r as a closed contour of four cubic
// curves, clockwise from the right end of its middle. A hole cut in a shape
// needs FillEvenOdd, or a contour that turns the other way, such as an AddArc
// with a negative sweep.
func (p *Path) AddOval(r geom.Rect) {
	rx, ry := r.W/2, r.H/2
	cx, cy := r.X+rx, r.Y+ry
	kx, ky := rx*circleControl, ry*circleControl
	right, bottom := r.X+r.W, r.Y+r.H
	p.MoveTo(right, cy)
	p.CubeTo(right, cy+ky, cx+kx, bottom, cx, bottom)
	p.CubeTo(cx-kx, bottom, r.X, cy+ky, r.X, cy)
	p.CubeTo(r.X, cy-ky, cx-kx, r.Y, cx, r.Y)
	p.CubeTo(cx+kx, r.Y, right, cy-ky, right, cy)
	p.Close()
}

// arcJoin is how far the current point may be from the start of an arc
// before ArcTo draws a line to it, in logical pixels.
const arcJoin = 1e-3

func clampSweep(sweep float32) float64 {
	return max(-2*math.Pi, min(float64(sweep), 2*math.Pi))
}

func arcPoint(center geom.Offset, radius float32, a float64) (x, y float32) {
	return float32(float64(center.X) + float64(radius)*math.Cos(a)), float32(float64(center.Y) + float64(radius)*math.Sin(a))
}

// arc continues p along the circle from angle a0 through sweep, in pieces of
// at most a quarter turn.
func (p *Path) arc(center geom.Offset, radius float32, a0, sweep float64) {
	// The slack keeps a float32 π, a little more than π, at two pieces.
	pieces := int(math.Ceil(math.Abs(sweep)/(math.Pi/2) - 1e-6))
	if pieces == 0 {
		return
	}
	cx, cy, r := float64(center.X), float64(center.Y), float64(radius)
	step := sweep / float64(pieces)
	// k places the control points of a cubic that follows a circular arc.
	k := 4.0 / 3 * math.Tan(step/4)
	for i := range pieces {
		s := a0 + step*float64(i)
		e := s + step
		cs, ss, ce, se := math.Cos(s), math.Sin(s), math.Cos(e), math.Sin(e)
		p.CubeTo(
			float32(cx+r*(cs-k*ss)), float32(cy+r*(ss+k*cs)),
			float32(cx+r*(ce+k*se)), float32(cy+r*(se-k*ce)),
			float32(cx+r*ce), float32(cy+r*se),
		)
	}
}
