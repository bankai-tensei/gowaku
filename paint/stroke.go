package paint

import (
	"math"

	"github.com/bankai-tensei/gowaku/geom"
)

// defaultMiterLimit is the miter limit of SVG when none is given.
const defaultMiterLimit = 4

// maxStrokeSteps bounds the segments of one curve that Stroke flattens. It is
// larger than maxCurveSteps because a stroke may be flattened finer than the
// path it comes from.
const maxStrokeSteps = 1024

// LineCap is the shape at the ends of an open stroke.
type LineCap uint8

// The caps, as in SVG. The zero value is butt, the default of SVG.
const (
	// CapButt ends the stroke at the end of the path.
	CapButt LineCap = iota
	// CapRound adds a half circle at each end.
	CapRound
	// CapSquare adds half a square at each end.
	CapSquare
)

// String returns the SVG name of the cap.
func (c LineCap) String() string {
	switch c {
	case CapButt:
		return "butt"
	case CapRound:
		return "round"
	case CapSquare:
		return "square"
	}
	return "unknown"
}

// LineJoin is the shape where two segments of a stroke meet.
type LineJoin uint8

// The joins, as in SVG. The zero value is miter, the default of SVG.
const (
	// JoinMiter extends the outer edges until they meet, unless that is
	// longer than the miter limit allows, in which case it bevels.
	JoinMiter LineJoin = iota
	// JoinRound rounds the corner with a circle.
	JoinRound
	// JoinBevel cuts the corner off.
	JoinBevel
)

// String returns the SVG name of the join.
func (j LineJoin) String() string {
	switch j {
	case JoinMiter:
		return "miter"
	case JoinRound:
		return "round"
	case JoinBevel:
		return "bevel"
	}
	return "unknown"
}

// StrokeStyle describes the outline that Path.Stroke builds. The zero value
// has no width, and otherwise follows the defaults of SVG: butt caps and miter
// joins with a limit of 4.
type StrokeStyle struct {
	Width float32
	Cap   LineCap
	Join  LineJoin
	// MiterLimit bounds the length of a miter, as a multiple of Width. Zero
	// selects 4, and anything below 1 counts as 1.
	MiterLimit float32
	// Tolerance is how far the outline may stray from the curves of the path,
	// in the units of the path. Zero selects 0.1, the tolerance of a fill.
	// Rounded joins and caps stay curves, which the fill flattens.
	Tolerance float32
}

// Stroke returns the outline of a stroke along p, as a path to fill with the
// non-zero rule. The stroke is centered on p. Contours that p closed get joins
// all around; open ones get caps. A contour of no length gets a dot with round
// or square caps, as in SVG.
//
// The outline is a union of pieces, one per segment, join and cap, that all
// wind the same way, so that crossings and tight curves need no special care.
// See docs/ADR/0045-shaders-svg-animated-images.md.
func (p *Path) Stroke(st StrokeStyle) *Path {
	s := stroker{out: &Path{}, half: st.Width / 2, style: st}
	if s.half <= 0 || p.Empty() {
		return s.out
	}
	s.limit = st.MiterLimit
	if s.limit == 0 {
		s.limit = defaultMiterLimit
	}
	s.limit = max(s.limit, 1)
	s.tol = st.Tolerance
	if s.tol <= 0 {
		s.tol = flattenTolerance
	}
	s.eps = s.tol * 1e-3
	p.Walk(s.moveTo, s.lineTo, s.quadTo, s.cubeTo, s.closePath)
	s.finish()
	return s.out
}

// stroker builds the outline of one path.
type stroker struct {
	out   *Path
	half  float32
	style StrokeStyle
	limit float32
	tol   float32
	// eps is the distance below which two points count as one.
	eps float32

	// The contour being collected: its points, whether each lies inside a
	// curve, and the unit direction and offset normal of each segment.
	pts    []geom.Offset
	smooth []bool
	dirs   []geom.Offset
	norms  []geom.Offset
	pen    geom.Offset
	start  geom.Offset
	drawn  bool
	closed bool
}

func (s *stroker) moveTo(p geom.Offset) {
	s.finish()
	s.pen, s.start = p, p
	s.add(p, false)
}

func (s *stroker) begin() {
	if len(s.pts) == 0 {
		s.add(s.pen, false)
	}
	s.drawn = true
}

func (s *stroker) lineTo(p geom.Offset) {
	s.begin()
	s.add(p, false)
	s.pen = p
}

func (s *stroker) quadTo(c, to geom.Offset) {
	s.begin()
	a := s.pen
	d := a.Sub(c.Scale(2)).Add(to)
	n := s.steps(geom.Abs32(d.X)+geom.Abs32(d.Y), 0.25)
	for i := 1; i <= n; i++ {
		t := float32(i) / float32(n)
		u := 1 - t
		s.add(a.Scale(u*u).Add(c.Scale(2*u*t)).Add(to.Scale(t*t)), i < n)
	}
	s.pts[len(s.pts)-1] = to
	s.pen = to
}

func (s *stroker) cubeTo(b, c, to geom.Offset) {
	s.begin()
	a := s.pen
	d1 := a.Sub(b.Scale(2)).Add(c)
	d2 := b.Sub(c.Scale(2)).Add(to)
	dd := max(geom.Abs32(d1.X)+geom.Abs32(d1.Y), geom.Abs32(d2.X)+geom.Abs32(d2.Y))
	n := s.steps(dd, 0.75)
	for i := 1; i <= n; i++ {
		t := float32(i) / float32(n)
		u := 1 - t
		s.add(a.Scale(u*u*u).Add(b.Scale(3*u*u*t)).Add(c.Scale(3*u*t*t)).Add(to.Scale(t*t*t)), i < n)
	}
	s.pts[len(s.pts)-1] = to
	s.pen = to
}

func (s *stroker) closePath() {
	s.begin()
	s.closed = true
	s.finish()
	// A segment after Close starts at the start of the closed contour.
	s.pen = s.start
}

func (s *stroker) steps(d, k float32) int {
	n := int(geom.Sqrt32(d*k/s.tol)) + 1
	return min(max(n, 1), maxStrokeSteps)
}

// add appends a point, unless it repeats the last one.
func (s *stroker) add(p geom.Offset, smooth bool) {
	if n := len(s.pts); n > 0 {
		d := p.Sub(s.pts[n-1])
		if geom.Abs32(d.X) <= s.eps && geom.Abs32(d.Y) <= s.eps {
			s.smooth[n-1] = s.smooth[n-1] && smooth
			return
		}
	}
	s.pts = append(s.pts, p)
	s.smooth = append(s.smooth, smooth)
}

// finish outlines the collected contour and starts over.
func (s *stroker) finish() {
	defer s.clear()
	if !s.drawn {
		return
	}
	pts := s.pts
	if s.closed && len(pts) > 1 {
		if d := pts[len(pts)-1].Sub(pts[0]); geom.Abs32(d.X) <= s.eps && geom.Abs32(d.Y) <= s.eps {
			pts = pts[:len(pts)-1]
			s.smooth[0] = false
		}
	}
	n := len(pts)
	if n == 1 {
		s.dot(pts[0])
		return
	}
	segments := n - 1
	if s.closed {
		segments = n
	}
	s.dirs, s.norms = s.dirs[:0], s.norms[:0]
	for i := range segments {
		d := pts[(i+1)%n].Sub(pts[i])
		l := geom.Sqrt32(d.X*d.X + d.Y*d.Y)
		u := geom.Offset{X: d.X / l, Y: d.Y / l}
		s.dirs = append(s.dirs, u)
		s.norms = append(s.norms, geom.Offset{X: -u.Y * s.half, Y: u.X * s.half})
	}
	for i := range segments {
		a, b, nm := pts[i], pts[(i+1)%n], s.norms[i]
		s.polygon(a.Add(nm), b.Add(nm), b.Sub(nm), a.Sub(nm))
	}
	if s.closed {
		for i := range n {
			in := (i + segments - 1) % segments
			s.join(pts[i], in, i, s.smooth[i])
		}
		return
	}
	for i := 1; i < n-1; i++ {
		s.join(pts[i], i-1, i, s.smooth[i])
	}
	first, last := s.dirs[0], s.dirs[segments-1]
	s.cap(pts[0], first.Scale(-1), s.norms[0].Scale(-1))
	s.cap(pts[n-1], last, s.norms[segments-1])
}

func (s *stroker) clear() {
	s.pts, s.smooth = s.pts[:0], s.smooth[:0]
	s.drawn, s.closed = false, false
}

// join fills the outer corner at v between segment in and segment out.
func (s *stroker) join(v geom.Offset, in, out int, smooth bool) {
	u, w := s.dirs[in], s.dirs[out]
	cross := u.X*w.Y - u.Y*w.X
	dot := u.X*w.X + u.Y*w.Y
	if cross == 0 && dot > 0 {
		return
	}
	// The outer side is to the right of a left turn and to the left of a
	// right turn. Its corners are the very points of the segment pieces, so
	// that the pieces share their edges exactly.
	var a, b geom.Offset
	if cross > 0 {
		a, b = v.Sub(s.norms[in]), v.Sub(s.norms[out])
	} else {
		a, b = v.Add(s.norms[in]), v.Add(s.norms[out])
	}
	kind := s.style.Join
	if smooth {
		// Inside a curve the corners come from flattening, not from the
		// drawing, and the stroke of the curve is round there.
		kind = JoinRound
	}
	switch kind {
	case JoinMiter:
		// The miter is 1/sin(θ/2) times the width, where θ is the angle
		// between the segments, and sin(θ/2) = sqrt((1+dot)/2).
		if c := (1 + dot) / 2; c > 0 && 1/geom.Sqrt32(c) <= s.limit {
			na, nb := a.Sub(v), b.Sub(v)
			tip := v.Add(na.Add(nb).Scale(1 / (1 + dot)))
			s.polygon(v, a, tip, b)
			return
		}
		s.polygon(v, a, b)
	case JoinRound:
		angle := float32(math.Atan2(float64(cross), float64(dot)))
		if cross == 0 {
			// A turn back: the round goes around the front.
			a, b = v.Add(s.norms[in]), v.Sub(s.norms[in])
			angle = -math.Pi
		}
		s.sector(v, a, b, angle)
	case JoinBevel:
		s.polygon(v, a, b)
	}
}

// cap adds the cap at end e of a contour that leaves in direction u, where nm is
// the offset normal of the direction u.
func (s *stroker) cap(e, u, nm geom.Offset) {
	switch s.style.Cap {
	case CapRound:
		s.sector(e, e.Add(nm), e.Sub(nm), -math.Pi)
	case CapSquare:
		ext := u.Scale(s.half)
		s.polygon(e.Add(nm), e.Add(nm).Add(ext), e.Sub(nm).Add(ext), e.Sub(nm))
	case CapButt:
	}
}

// dot draws a contour of no length, whose caps make a circle or a square
// aligned with the axes.
func (s *stroker) dot(c geom.Offset) {
	h := s.half
	switch s.style.Cap {
	case CapRound:
		e := geom.Offset{X: h}
		s.sector(c, c.Add(e), c.Sub(e), math.Pi)
		s.sector(c, c.Sub(e), c.Add(e), math.Pi)
	case CapSquare:
		s.polygon(geom.Offset{X: c.X - h, Y: c.Y - h}, geom.Offset{X: c.X + h, Y: c.Y - h},
			geom.Offset{X: c.X + h, Y: c.Y + h}, geom.Offset{X: c.X - h, Y: c.Y + h})
	case CapButt:
	}
}

// polygon adds a closed contour that winds the positive way.
func (s *stroker) polygon(pts ...geom.Offset) {
	var area float32
	for i, p := range pts {
		q := pts[(i+1)%len(pts)]
		area += p.X*q.Y - q.X*p.Y
	}
	if area == 0 {
		return
	}
	s.out.MoveTo(pts[0].X, pts[0].Y)
	if area > 0 {
		for _, p := range pts[1:] {
			s.out.LineTo(p.X, p.Y)
		}
	} else {
		for i := len(pts) - 1; i > 0; i-- {
			s.out.LineTo(pts[i].X, pts[i].Y)
		}
	}
	s.out.Close()
}

// sector adds the slice of the circle around c from a to b, which turns by
// angle, as a contour that winds the positive way. A slice too thin to round
// becomes a triangle.
func (s *stroker) sector(c, a, b geom.Offset, angle float32) {
	if angle < 0 {
		a, b, angle = b, a, -angle
	}
	// A chord strays h(1 - cos(angle/2)) from the arc.
	if s.half*(1-geom.Cos32(angle/2)) <= s.tol/2 {
		s.polygon(c, a, b)
		return
	}
	s.out.MoveTo(c.X, c.Y)
	s.out.LineTo(a.X, a.Y)
	arcTo(s.out, c, a, b, angle)
	s.out.Close()
}

// arcTo adds cubics along the circle around c from a, the current point, to b,
// turning by angle in the positive direction.
func arcTo(p *Path, c, a, b geom.Offset, angle float32) {
	n := int(geom.Ceil32(angle / (math.Pi / 2)))
	n = max(n, 1)
	step := angle / float32(n)
	k := 4.0 / 3 * geom.Tan32(step/4)
	sin, cos := geom.Sin32(step), geom.Cos32(step)
	r := a.Sub(c)
	for i := range n {
		next := geom.Offset{X: r.X*cos - r.Y*sin, Y: r.X*sin + r.Y*cos}
		c1 := c.Add(r).Add(geom.Offset{X: -r.Y * k, Y: r.X * k})
		c2 := c.Add(next).Add(geom.Offset{X: next.Y * k, Y: -next.X * k})
		end := c.Add(next)
		if i == n-1 {
			end = b
		}
		p.CubeTo(c1.X, c1.Y, c2.X, c2.Y, end.X, end.Y)
		r = next
	}
}

// transformed returns a copy of p with m applied to every point.
func (p *Path) transformed(m geom.Affine) *Path {
	q := &Path{Rule: p.Rule, verbs: append([]pathVerb(nil), p.verbs...), points: make([]geom.Offset, len(p.points))}
	for i, pt := range p.points {
		q.points[i] = m.Apply(pt)
	}
	return q
}
