package paint

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

// winding returns the winding number of p around q, from the flattened path.
func winding(p *Path, q geom.Offset) int {
	points, starts := p.Polygon()
	w := 0
	for i, start := range starts {
		end := len(points)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		n := end - start
		for k := range n {
			a, b := points[start+k], points[start+(k+1)%n]
			cross := (b.X-a.X)*(q.Y-a.Y) - (q.X-a.X)*(b.Y-a.Y)
			switch {
			case a.Y <= q.Y && b.Y > q.Y && cross > 0:
				w++
			case a.Y > q.Y && b.Y <= q.Y && cross < 0:
				w--
			}
		}
	}
	return w
}

func inside(p *Path, x, y float32) bool { return winding(p, geom.Offset{X: x, Y: y}) != 0 }

type probe struct {
	name string
	x, y float32
	in   bool
}

func checkProbes(t *testing.T, p *Path, probes []probe) {
	t.Helper()
	for _, pr := range probes {
		if got := inside(p, pr.x, pr.y); got != pr.in {
			t.Errorf("%s (%v, %v): inside %v, want %v", pr.name, pr.x, pr.y, got, pr.in)
		}
	}
}

func line(pts ...float32) *Path {
	p := &Path{}
	p.MoveTo(pts[0], pts[1])
	for i := 2; i < len(pts); i += 2 {
		p.LineTo(pts[i], pts[i+1])
	}
	return p
}

func TestStroke_Caps(t *testing.T) {
	src := line(10, 10, 30, 10)
	cases := []struct {
		name   string
		cap    LineCap
		probes []probe
	}{
		{"butt", CapButt, []probe{{"middle", 20, 11.5, true}, {"beyond end", 31, 10, false}, {"before start", 9, 10, false}, {"above", 20, 12.5, false}}},
		{"round", CapRound, []probe{{"beyond end", 31.5, 10, true}, {"cap corner", 31.6, 11.6, false}, {"before start", 8.5, 10, true}}},
		{"square", CapSquare, []probe{{"cap corner", 31.8, 11.8, true}, {"past cap", 32.2, 10, false}, {"start corner", 8.2, 8.2, true}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := src.Stroke(StrokeStyle{Width: 4, Cap: c.cap})
			checkProbes(t, out, c.probes)
		})
	}
}

func TestStroke_Joins(t *testing.T) {
	// A right angle: along x, then down. The outer corner is at (30, 8)
	// with a half width of 2.
	src := line(10, 10, 30, 10, 30, 30)
	cases := []struct {
		name   string
		join   LineJoin
		probes []probe
	}{
		{"miter", JoinMiter, []probe{{"miter corner", 31.9, 8.1, true}, {"inner corner", 28.1, 11.9, true}}},
		{"round", JoinRound, []probe{{"on the round", 31.3, 8.7, true}, {"miter corner", 31.9, 8.1, false}}},
		{"bevel", JoinBevel, []probe{{"inside the bevel", 30.8, 9.2, true}, {"cut corner", 31.6, 8.4, false}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := src.Stroke(StrokeStyle{Width: 4, Join: c.join})
			checkProbes(t, out, c.probes)
		})
	}
}

func TestStroke_MiterLimit(t *testing.T) {
	// A sharp turn of about 160 degrees: the miter is about 11 times the
	// width, past the default limit of 4.
	src := line(0, 0, 40, 0, 0, 7)
	tip := geom.Offset{X: 49, Y: -0.8}
	beveled := src.Stroke(StrokeStyle{Width: 2})
	if winding(beveled, tip) != 0 {
		t.Error("the default limit must bevel a sharp miter")
	}
	mitered := src.Stroke(StrokeStyle{Width: 2, MiterLimit: 20})
	if winding(mitered, tip) == 0 {
		t.Error("a limit of 20 must keep the miter")
	}
}

func TestStroke_ClosedHasHoleAndJoinAtStart(t *testing.T) {
	src := line(10, 10, 30, 10, 30, 30, 10, 30)
	src.Close()
	out := src.Stroke(StrokeStyle{Width: 4})
	checkProbes(t, out, []probe{
		{"hole", 20, 20, false},
		{"ring", 20, 11, true},
		{"closing edge", 10, 20, true},
		{"miter at the start", 8.2, 8.2, true},
	})
	open := line(10, 10, 30, 10, 30, 30, 10, 30, 10, 10)
	if inside(open.Stroke(StrokeStyle{Width: 4}), 8.2, 8.2) {
		t.Error("an open contour that comes back has butt ends, not a join")
	}
}

func TestStroke_Dots(t *testing.T) {
	zero := line(10, 10, 10, 10)
	closed := &Path{}
	closed.MoveTo(10, 10)
	closed.Close()
	for _, src := range []*Path{zero, closed} {
		if out := src.Stroke(StrokeStyle{Width: 4, Cap: CapButt}); !out.Empty() {
			t.Error("a butt dot draws nothing")
		}
		round := src.Stroke(StrokeStyle{Width: 4, Cap: CapRound})
		checkProbes(t, round, []probe{{"center", 10, 10, true}, {"rim", 11.9, 10, true}, {"square corner", 11.8, 11.8, false}})
		square := src.Stroke(StrokeStyle{Width: 4, Cap: CapSquare})
		checkProbes(t, square, []probe{{"corner", 11.9, 11.9, true}, {"outside", 12.1, 10, false}})
	}
	moveOnly := &Path{}
	moveOnly.MoveTo(5, 5)
	if out := moveOnly.Stroke(StrokeStyle{Width: 4, Cap: CapRound}); !out.Empty() {
		t.Error("a lone move draws nothing")
	}
}

func TestStroke_CurvesAndTightCircles(t *testing.T) {
	c := ellipsePath(50, 50, 20, 20)
	out := c.Stroke(StrokeStyle{Width: 4, Tolerance: 0.01})
	for i := range 16 {
		a := float32(i) * 2 * 3.14159265 / 16
		dx, dy := geom.Cos32(a), geom.Sin32(a)
		for _, r := range []struct {
			radius float32
			in     bool
		}{{18.2, true}, {21.8, true}, {17.8, false}, {22.2, false}} {
			if got := inside(out, 50+dx*r.radius, 50+dy*r.radius); got != r.in {
				t.Errorf("angle %v radius %v: inside %v", a, r.radius, got)
			}
		}
	}
	if inside(out, 50, 50) {
		t.Error("the middle of a ring is empty")
	}
	// A circle smaller than the half width fills in, as in a browser.
	tiny := ellipsePath(10, 10, 0.5, 0.5).Stroke(StrokeStyle{Width: 2})
	checkProbes(t, tiny, []probe{{"center", 10, 10, true}, {"rim", 11.4, 10, true}, {"outside", 11.6, 10, false}})
}

func TestStroke_PiecesWindTheSameWay(t *testing.T) {
	src := line(0, 0, 10, 0, 10, 10, 0, 10, 5, -5)
	for _, join := range []LineJoin{JoinMiter, JoinRound, JoinBevel} {
		out := src.Stroke(StrokeStyle{Width: 3, Join: join, Cap: CapRound})
		points, starts := out.Polygon()
		for i, start := range starts {
			end := len(points)
			if i+1 < len(starts) {
				end = starts[i+1]
			}
			var area float32
			for k := start; k < end; k++ {
				a, b := points[k], points[start+(k+1-start)%(end-start)]
				area += a.X*b.Y - b.X*a.Y
			}
			if area < 0 {
				t.Errorf("%v: contour %d winds the other way", join, i)
			}
		}
	}
}

func TestStroke_RepeatedPointsAndTurnBack(t *testing.T) {
	src := line(0, 0, 10, 0, 10, 0, 0, 0)
	out := src.Stroke(StrokeStyle{Width: 2, Join: JoinRound})
	checkProbes(t, out, []probe{{"round at the turn", 10.9, 0, true}, {"along", 5, 0.9, true}, {"start", -0.5, 0, false}})
	if empty := (&Path{}).Stroke(StrokeStyle{Width: 2}); !empty.Empty() {
		t.Error("an empty path has no stroke")
	}
	if none := src.Stroke(StrokeStyle{}); !none.Empty() {
		t.Error("no width, no stroke")
	}
}
