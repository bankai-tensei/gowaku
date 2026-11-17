package paint

import (
	"math"
	"slices"

	"github.com/bankai-tensei/gowaku/geom"
)

// flattenTolerance is how far a flattened curve may stray from the real one, in
// logical pixels. It stays under a quarter of a physical pixel up to a scale of
// 2.5.
const flattenTolerance = 0.1

// maxCurveSteps bounds the segments of one curve.
const maxCurveSteps = 64

// circleControl places the control points of a cubic that approximates a
// quarter circle.
const circleControl = 0.5522848

// FillRule decides what is inside a path that crosses itself.
type FillRule uint8

// The fill rules.
const (
	FillNonZero FillRule = iota
	FillEvenOdd
)

type pathVerb uint8

const (
	verbMove pathVerb = iota
	verbLine
	verbQuad
	verbCube
	verbClose
)

// Path is a shape made of lines and curves. A Canvas copies a path when it
// records it, so a path can be changed, or reset and refilled for the next
// shape, right after drawing it.
type Path struct {
	Rule   FillRule
	verbs  []pathVerb
	points []geom.Offset

	// flat is the flattened path, which every frame reuses until the path
	// changes.
	flat   []geom.Offset
	starts []int
	closed []bool
	bounds geom.Rect
	// hash is Hash of verbs and points, computed with flat.
	hash  uint64
	clean bool
	// gen counts the changes, so that a Canvas can tell that a path it copied
	// in an earlier frame is still the same.
	gen uint64
}

// Reset empties the path and keeps its storage.
func (p *Path) Reset() {
	p.verbs, p.points = p.verbs[:0], p.points[:0]
	p.clean = false
	p.gen++
}

// MoveTo starts a new contour at (x, y).
func (p *Path) MoveTo(x, y float32) { p.add(verbMove, geom.Offset{X: x, Y: y}) }

// LineTo adds a line to (x, y).
func (p *Path) LineTo(x, y float32) { p.add(verbLine, geom.Offset{X: x, Y: y}) }

// QuadTo adds a quadratic curve to (x, y) with control point (cx, cy).
func (p *Path) QuadTo(cx, cy, x, y float32) {
	p.add(verbQuad, geom.Offset{X: cx, Y: cy}, geom.Offset{X: x, Y: y})
}

// CubeTo adds a cubic curve to (x, y) with control points (ax, ay) and (bx, by).
func (p *Path) CubeTo(ax, ay, bx, by, x, y float32) {
	p.add(verbCube, geom.Offset{X: ax, Y: ay}, geom.Offset{X: bx, Y: by}, geom.Offset{X: x, Y: y})
}

// Close ends the contour with a line to its start.
func (p *Path) Close() { p.add(verbClose) }

func (p *Path) add(v pathVerb, pts ...geom.Offset) {
	p.verbs = append(p.verbs, v)
	p.points = append(p.points, pts...)
	p.clean = false
	p.gen++
}

// AddRRect adds rr as a closed contour.
func (p *Path) AddRRect(rr geom.RRect) {
	r := rr.Rect
	right, bottom := r.X+r.W, r.Y+r.H
	limit := min(r.W, r.H) / 2
	tl, tr := min(rr.TopLeft, limit), min(rr.TopRight, limit)
	br, bl := min(rr.BottomRight, limit), min(rr.BottomLeft, limit)
	const k = 1 - circleControl
	p.MoveTo(r.X+tl, r.Y)
	p.LineTo(right-tr, r.Y)
	if tr > 0 {
		p.CubeTo(right-tr*k, r.Y, right, r.Y+tr*k, right, r.Y+tr)
	}
	p.LineTo(right, bottom-br)
	if br > 0 {
		p.CubeTo(right, bottom-br*k, right-br*k, bottom, right-br, bottom)
	}
	p.LineTo(r.X+bl, bottom)
	if bl > 0 {
		p.CubeTo(r.X+bl*k, bottom, r.X, bottom-bl*k, r.X, bottom-bl)
	}
	p.LineTo(r.X, r.Y+tl)
	if tl > 0 {
		p.CubeTo(r.X, r.Y+tl*k, r.X+tl*k, r.Y, r.X+tl, r.Y)
	}
	p.Close()
}

// Scaled returns a copy of p with every coordinate multiplied by s.
func (p *Path) Scaled(s float32) *Path {
	q := &Path{Rule: p.Rule, verbs: append([]pathVerb(nil), p.verbs...), points: make([]geom.Offset, len(p.points))}
	for i, pt := range p.points {
		q.points[i] = geom.Offset{X: pt.X * s, Y: pt.Y * s}
	}
	return q
}

// Empty reports whether the path has nothing to draw.
func (p *Path) Empty() bool { return len(p.verbs) == 0 }

// Bounds returns the rectangle around the flattened path.
func (p *Path) Bounds() geom.Rect {
	p.flatten()
	return p.bounds
}

// Polygon returns the flattened path: all points, and where each contour starts
// in them. Contours count as closed. The slices are owned by p.
func (p *Path) Polygon() (points []geom.Offset, starts []int) {
	p.flatten()
	return p.flat, p.starts
}

// Closed reports whether contour i of Polygon ended with Close. It matters to
// strokes; a fill closes every contour.
func (p *Path) Closed(i int) bool { return p.closed[i] }

// Walk calls the functions for each segment, for code that wants the curves
// instead of their flattened form. Close arrives as a call to closePath.
func (p *Path) Walk(moveTo, lineTo func(geom.Offset), quadTo func(c, to geom.Offset),
	cubeTo func(a, b, to geom.Offset), closePath func()) {
	i := 0
	for _, v := range p.verbs {
		switch v {
		case verbMove:
			moveTo(p.points[i])
			i++
		case verbLine:
			lineTo(p.points[i])
			i++
		case verbQuad:
			quadTo(p.points[i], p.points[i+1])
			i += 2
		case verbCube:
			cubeTo(p.points[i], p.points[i+1], p.points[i+2])
			i += 3
		case verbClose:
			closePath()
		}
	}
}

func (p *Path) flatten() {
	if p.clean {
		return
	}
	p.clean = true
	p.flat, p.starts, p.closed = p.flat[:0], p.starts[:0], p.closed[:0]
	var pen, start geom.Offset
	i := 0
	begin := func(at geom.Offset) {
		p.starts = append(p.starts, len(p.flat))
		p.closed = append(p.closed, false)
		p.flat = append(p.flat, at)
	}
	open := false
	for _, v := range p.verbs {
		if v != verbMove && v != verbClose && !open {
			begin(pen)
			open = true
		}
		switch v {
		case verbMove:
			pen = p.points[i]
			start = pen
			i++
			begin(pen)
			open = true
		case verbLine:
			pen = p.points[i]
			i++
			p.flat = append(p.flat, pen)
		case verbQuad:
			c, to := p.points[i], p.points[i+1]
			i += 2
			p.flattenQuad(pen, c, to)
			pen = to
		case verbCube:
			a, b, to := p.points[i], p.points[i+1], p.points[i+2]
			i += 3
			p.flattenCube(pen, a, b, to)
			pen = to
		case verbClose:
			pen = start
			open = false
			if n := len(p.closed); n > 0 {
				p.closed[n-1] = true
			}
		}
	}
	p.computeBounds()
	p.hash = p.hashContents()
}

// FNV-1a, 64 bits.
const (
	hashOffset = 14695981039346656037
	hashPrime  = 1099511628211
)

func (p *Path) hashContents() uint64 {
	h := uint64(hashOffset)
	for _, v := range p.verbs {
		h = (h ^ uint64(v)) * hashPrime
	}
	for _, q := range p.points {
		h = (h ^ hashBits(q.X)) * hashPrime
		h = (h ^ hashBits(q.Y)) * hashPrime
	}
	return h
}

// hashBits returns the bits of f, with -0 as 0 since the two are equal.
func hashBits(f float32) uint64 {
	if f == 0 {
		return 0
	}
	return uint64(math.Float32bits(f))
}

// Hash returns a hash of the lines and curves of p, for caches keyed by what a
// path holds rather than by where it lives. Paths that are Equal have the same
// hash. It ignores Rule. It is computed again only after p changes.
func (p *Path) Hash() uint64 {
	p.flatten()
	return p.hash
}

// Equal reports whether p and q hold the same lines and curves with the same
// fill rule.
func (p *Path) Equal(q *Path) bool {
	return p.Rule == q.Rule && slices.Equal(p.verbs, q.verbs) && slices.Equal(p.points, q.points)
}

// Set makes p a copy of q, flattened form included, and returns p. It reuses
// the storage of p, so it does not allocate once p has grown.
func (p *Path) Set(q *Path) *Path {
	if p == q {
		return p
	}
	q.flatten()
	p.Rule = q.Rule
	p.verbs = append(p.verbs[:0], q.verbs...)
	p.points = append(p.points[:0], q.points...)
	p.flat = append(p.flat[:0], q.flat...)
	p.starts = append(p.starts[:0], q.starts...)
	p.closed = append(p.closed[:0], q.closed...)
	p.bounds, p.hash, p.clean = q.bounds, q.hash, true
	p.gen++
	return p
}

func (p *Path) computeBounds() {
	p.bounds = geom.Rect{}
	if len(p.flat) == 0 {
		return
	}
	lo, hi := p.flat[0], p.flat[0]
	for _, q := range p.flat[1:] {
		lo.X, lo.Y = min(lo.X, q.X), min(lo.Y, q.Y)
		hi.X, hi.Y = max(hi.X, q.X), max(hi.Y, q.Y)
	}
	p.bounds = geom.Rect{X: lo.X, Y: lo.Y, W: hi.X - lo.X, H: hi.Y - lo.Y}
}

// steps returns how many segments keep a curve within the tolerance, given the
// largest second difference d of its control points.
func steps(d, scale float32) int {
	n := int(geom.Sqrt32(d*scale/flattenTolerance)) + 1
	return min(max(n, 1), maxCurveSteps)
}

func (p *Path) flattenQuad(a, c, b geom.Offset) {
	d := a.Sub(c.Scale(2)).Add(b)
	n := steps(geom.Abs32(d.X)+geom.Abs32(d.Y), 0.25)
	for i := 1; i <= n; i++ {
		t := float32(i) / float32(n)
		u := 1 - t
		p.flat = append(p.flat, a.Scale(u*u).Add(c.Scale(2*u*t)).Add(b.Scale(t*t)))
	}
}

func (p *Path) flattenCube(a, b, c, d geom.Offset) {
	d1 := a.Sub(b.Scale(2)).Add(c)
	d2 := b.Sub(c.Scale(2)).Add(d)
	dd := max(geom.Abs32(d1.X)+geom.Abs32(d1.Y), geom.Abs32(d2.X)+geom.Abs32(d2.Y))
	n := steps(dd, 0.75)
	for i := 1; i <= n; i++ {
		t := float32(i) / float32(n)
		u := 1 - t
		p.flat = append(p.flat, a.Scale(u*u*u).Add(b.Scale(3*u*u*t)).Add(c.Scale(3*u*t*t)).Add(d.Scale(t*t*t)))
	}
}
