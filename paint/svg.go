package paint

import (
	"fmt"
	"math"
	"strconv"

	"github.com/bankai-tensei/gowaku/geom"
)

// ParseSVGPath reads the d attribute of an SVG path element. It knows every path
// command of SVG 1.1, in absolute and relative form.
func ParseSVGPath(d string) (*Path, error) {
	s := svgParser{in: d, path: &Path{}}
	if err := s.run(); err != nil {
		return nil, fmt.Errorf("paint: parse svg path: %w", err)
	}
	return s.path, nil
}

type svgParser struct {
	in   string
	at   int
	path *Path

	pen, start geom.Offset
	// lastCubic and lastQuad are the control points that S and T reflect.
	lastCubic, lastQuad geom.Offset
	hadCubic, hadQuad   bool

	// verts, if set, receives the vertices of the path, where markers go.
	verts *svgVertices
	// arcStart is the first control point of the last arc.
	arcStart geom.Offset
}

func (s *svgParser) run() error {
	var cmd byte
	for {
		s.skipSpace()
		if s.at == len(s.in) {
			return nil
		}
		c := s.in[s.at]
		switch {
		case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'):
			cmd = c
			s.at++
		case cmd == 0:
			return fmt.Errorf("expected a command at %d", s.at)
		case cmd == 'Z' || cmd == 'z':
			// A close takes no numbers, so it cannot repeat: repeating it
			// would consume nothing and never end.
			return fmt.Errorf("expected a command after %c at %d", cmd, s.at)
		case cmd == 'M':
			cmd = 'L' // numbers after a move are lines
		case cmd == 'm':
			cmd = 'l'
		}
		if err := s.command(cmd); err != nil {
			return err
		}
	}
}

func (s *svgParser) command(cmd byte) error {
	relative := cmd >= 'a'
	upper := cmd &^ 0x20
	var origin geom.Offset
	if relative {
		origin = s.pen
	}
	wasCubic, wasQuad := s.hadCubic, s.hadQuad
	s.hadCubic, s.hadQuad = false, false
	var n [7]float32
	need := map[byte]int{'M': 2, 'L': 2, 'H': 1, 'V': 1, 'C': 6, 'S': 4, 'Q': 4, 'T': 2, 'A': 7, 'Z': 0}
	count, ok := need[upper]
	if !ok {
		return fmt.Errorf("unknown command %q", cmd)
	}
	for i := range count {
		if upper == 'A' && (i == 3 || i == 4) {
			flag, err := s.flag()
			if err != nil {
				return err
			}
			n[i] = flag
			continue
		}
		v, err := s.number()
		if err != nil {
			return err
		}
		n[i] = v
	}
	point := func(i int) geom.Offset { return geom.Offset{X: n[i], Y: n[i+1]}.Add(origin) }
	if s.verts != nil {
		from := s.pen
		defer func() { s.vertex(upper, from) }()
	}
	switch upper {
	case 'M':
		s.pen = point(0)
		s.start = s.pen
		s.path.MoveTo(s.pen.X, s.pen.Y)
	case 'L':
		s.lineTo(point(0))
	case 'H':
		s.lineTo(geom.Offset{X: n[0] + origin.X, Y: s.pen.Y})
	case 'V':
		s.lineTo(geom.Offset{X: s.pen.X, Y: n[0] + origin.Y})
	case 'C':
		s.cubicTo(point(0), point(2), point(4))
	case 'S':
		a := s.pen
		if wasCubic {
			a = s.pen.Scale(2).Sub(s.lastCubic)
		}
		s.cubicTo(a, point(0), point(2))
	case 'Q':
		s.quadTo(point(0), point(2))
	case 'T':
		c := s.pen
		if wasQuad {
			c = s.pen.Scale(2).Sub(s.lastQuad)
		}
		s.quadTo(c, point(0))
	case 'A':
		s.arcTo(n[0], n[1], n[2], n[3] != 0, n[4] != 0, point(5))
	case 'Z':
		s.path.Close()
		s.pen = s.start
	}
	return nil
}

func (s *svgParser) lineTo(p geom.Offset) {
	s.path.LineTo(p.X, p.Y)
	s.pen = p
}

func (s *svgParser) cubicTo(a, b, to geom.Offset) {
	s.path.CubeTo(a.X, a.Y, b.X, b.Y, to.X, to.Y)
	s.pen, s.lastCubic, s.hadCubic = to, b, true
}

func (s *svgParser) quadTo(c, to geom.Offset) {
	s.path.QuadTo(c.X, c.Y, to.X, to.Y)
	s.pen, s.lastQuad, s.hadQuad = to, c, true
}

// arcTo converts an elliptical arc to cubics, after SVG 1.1 appendix F.6.
func (s *svgParser) arcTo(rx, ry, rotation float32, large, sweep bool, to geom.Offset) {
	from := s.pen
	if from == to {
		return
	}
	if rx == 0 || ry == 0 {
		s.lineTo(to)
		return
	}
	a, b := math.Abs(float64(rx)), math.Abs(float64(ry))
	phi := float64(rotation) * math.Pi / 180
	sin, cos := math.Sincos(phi)
	dx, dy := float64(from.X-to.X)/2, float64(from.Y-to.Y)/2
	x1 := cos*dx + sin*dy
	y1 := -sin*dx + cos*dy
	// Radii too small to reach are scaled up until they just do.
	if l := x1*x1/(a*a) + y1*y1/(b*b); l > 1 {
		a *= math.Sqrt(l)
		b *= math.Sqrt(l)
	}
	num := a*a*b*b - a*a*y1*y1 - b*b*x1*x1
	den := a*a*y1*y1 + b*b*x1*x1
	k := math.Sqrt(math.Max(num/den, 0))
	if large == sweep {
		k = -k
	}
	cx1, cy1 := k*a*y1/b, -k*b*x1/a
	cx := cos*cx1 - sin*cy1 + float64(from.X+to.X)/2
	cy := sin*cx1 + cos*cy1 + float64(from.Y+to.Y)/2
	start := math.Atan2((y1-cy1)/b, (x1-cx1)/a)
	delta := math.Atan2((-y1-cy1)/b, (-x1-cx1)/a) - start
	if sweep && delta < 0 {
		delta += 2 * math.Pi
	} else if !sweep && delta > 0 {
		delta -= 2 * math.Pi
	}
	// Extreme input can leave no sweep at all, or no number: such an arc
	// is drawn as the line it nearly is.
	if delta == 0 || math.IsNaN(delta) || math.IsInf(delta, 0) || math.IsNaN(cx) || math.IsNaN(cy) {
		s.lineTo(to)
		return
	}
	segments := int(math.Ceil(math.Abs(delta) / (math.Pi / 2)))
	defer func() { s.arcStart = s.path.points[len(s.path.points)-3*segments] }()
	step := delta / float64(segments)
	t := 4.0 / 3 * math.Tan(step/4)
	onEllipse := func(x, y float64) geom.Offset {
		return geom.Offset{X: float32(cos*a*x - sin*b*y + cx), Y: float32(sin*a*x + cos*b*y + cy)}
	}
	for i := range segments {
		t0 := start + float64(i)*step
		t1 := t0 + step
		s0, c0 := math.Sincos(t0)
		s1, c1 := math.Sincos(t1)
		p1 := onEllipse(c0-t*s0, s0+t*c0)
		p2 := onEllipse(c1+t*s1, s1-t*c1)
		end := onEllipse(c1, s1)
		if i == segments-1 {
			end = to
		}
		s.path.CubeTo(p1.X, p1.Y, p2.X, p2.Y, end.X, end.Y)
	}
	s.pen = to
}

func (s *svgParser) skipSpace() {
	for s.at < len(s.in) {
		switch s.in[s.at] {
		case ' ', '\t', '\n', '\r', ',':
			s.at++
		default:
			return
		}
	}
}

// flag reads an arc flag, which may touch what follows it.
func (s *svgParser) flag() (float32, error) {
	s.skipSpace()
	if s.at < len(s.in) && (s.in[s.at] == '0' || s.in[s.at] == '1') {
		v := float32(s.in[s.at] - '0')
		s.at++
		return v, nil
	}
	return 0, fmt.Errorf("expected a flag at %d", s.at)
}

func (s *svgParser) number() (float32, error) {
	s.skipSpace()
	start := s.at
	if s.at < len(s.in) && (s.in[s.at] == '-' || s.in[s.at] == '+') {
		s.at++
	}
	dot := false
	for s.at < len(s.in) {
		c := s.in[s.at]
		switch {
		case c >= '0' && c <= '9':
		case c == '.' && !dot:
			dot = true
		case (c == 'e' || c == 'E') && s.at > start:
			s.at++
			if s.at < len(s.in) && (s.in[s.at] == '-' || s.in[s.at] == '+') {
				s.at++
			}
			continue
		default:
			return s.parsed(start)
		}
		s.at++
	}
	return s.parsed(start)
}

func (s *svgParser) parsed(start int) (float32, error) {
	v, err := strconv.ParseFloat(s.in[start:s.at], 32)
	if err != nil {
		return 0, fmt.Errorf("expected a number at %d", start)
	}
	return float32(v), nil
}

// vertex records the vertex that command cmd, which started at from, ended
// at, with the directions of the curve on both sides of it.
func (s *svgParser) vertex(cmd byte, from geom.Offset) {
	v := s.verts
	switch cmd {
	case 'M':
		v.move(s.pen)
		return
	case 'Z':
		v.close()
		return
	}
	// The direction at each end of the last segment, from its control
	// points; a control point on its end point falls back to the next.
	pts := s.path.points
	in, out := s.pen.Sub(from), s.pen.Sub(from)
	switch s.path.verbs[len(s.path.verbs)-1] {
	case verbCube:
		a, b := pts[len(pts)-3], pts[len(pts)-2]
		if cmd == 'A' {
			// An arc is several cubics; its start lies on the first.
			a = s.arcStart
		}
		out = firstNonZero(a.Sub(from), b.Sub(from), s.pen.Sub(from))
		in = firstNonZero(s.pen.Sub(b), s.pen.Sub(a), s.pen.Sub(from))
	case verbQuad:
		c := pts[len(pts)-2]
		out = firstNonZero(c.Sub(from), s.pen.Sub(from))
		in = firstNonZero(s.pen.Sub(c), s.pen.Sub(from))
	}
	if s.pen == from && (cmd == 'L' || cmd == 'H' || cmd == 'V' || cmd == 'A') {
		in, out = geom.Offset{}, geom.Offset{}
	}
	v.segment(from, s.pen, out, in)
}

func firstNonZero(ds ...geom.Offset) geom.Offset {
	for _, d := range ds {
		if d != (geom.Offset{}) {
			return d
		}
	}
	return geom.Offset{}
}
