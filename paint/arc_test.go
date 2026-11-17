package paint

import (
	"math"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

// onCircle fails unless every flattened point of p is radius from center,
// within the flattening tolerance.
func onCircle(t *testing.T, points []geom.Offset, center geom.Offset, radius float32) {
	t.Helper()
	for _, pt := range points {
		d := geom.Sqrt32((pt.X-center.X)*(pt.X-center.X) + (pt.Y-center.Y)*(pt.Y-center.Y))
		if geom.Abs32(d-radius) > 0.05 {
			t.Fatalf("point %v is %g from the center, want %g", pt, d, radius)
		}
	}
}

func arcNear(a, b geom.Offset) bool {
	return geom.Abs32(a.X-b.X) < 1e-3 && geom.Abs32(a.Y-b.Y) < 1e-3
}

func TestPath_AddArcQuarterTurn(t *testing.T) {
	var p Path
	c := geom.Offset{X: 50, Y: 50}
	p.AddArc(c, 40, 0, math.Pi/2)
	points, starts := p.Polygon()
	if len(starts) != 1 || p.Closed(0) {
		t.Fatalf("starts %v closed %v: want one open contour", starts, p.Closed(0))
	}
	onCircle(t, points, c, 40)
	if !arcNear(points[0], geom.Offset{X: 90, Y: 50}) {
		t.Errorf("starts at %v, want the right end", points[0])
	}
	// A positive sweep turns clockwise on the screen: from three o'clock down
	// to six.
	if last := points[len(points)-1]; !arcNear(last, geom.Offset{X: 50, Y: 90}) {
		t.Errorf("ends at %v, want the bottom", last)
	}
	if got, want := p.Bounds(), (geom.Rect{X: 50, Y: 50, W: 40, H: 40}); geom.Abs32(got.W-want.W) > 0.01 || geom.Abs32(got.H-want.H) > 0.01 {
		t.Errorf("bounds %v, want %v", got, want)
	}
}

func TestPath_AddArcCutsAtAWholeTurn(t *testing.T) {
	var a, b Path
	a.AddArc(geom.Offset{}, 10, 0, 2*math.Pi)
	b.AddArc(geom.Offset{}, 10, 0, 7*math.Pi)
	if !a.Equal(&b) {
		t.Error("a sweep past a whole turn must draw one turn")
	}
	var n int
	a.Walk(func(geom.Offset) {}, func(geom.Offset) {}, func(_, _ geom.Offset) {}, func(_, _, _ geom.Offset) { n++ }, func() {})
	if n != 4 {
		t.Errorf("a whole turn is %d cubics, want 4", n)
	}
}

func TestPath_ArcToJoinsWithALine(t *testing.T) {
	var p Path
	c := geom.Offset{X: 0, Y: 0}
	p.AddArc(c, 20, 0, math.Pi/2)
	p.ArcTo(c, 10, math.Pi/2, -math.Pi/2)
	p.Close()
	var lines, moves int
	p.Walk(func(geom.Offset) { moves++ }, func(geom.Offset) { lines++ }, func(_, _ geom.Offset) {}, func(_, _, _ geom.Offset) {}, func() {})
	if moves != 1 || lines != 1 {
		t.Errorf("moves %d lines %d: want one contour and one line from the outer arc to the inner", moves, lines)
	}
}

func TestPath_ArcToSkipsTheLineWhenItStartsThere(t *testing.T) {
	var p Path
	p.MoveTo(10, 0)
	p.ArcTo(geom.Offset{}, 10, 0, math.Pi)
	var lines int
	p.Walk(func(geom.Offset) {}, func(geom.Offset) { lines++ }, func(_, _ geom.Offset) {}, func(_, _, _ geom.Offset) {}, func() {})
	if lines != 0 {
		t.Errorf("%d lines, want none", lines)
	}
}

func TestPath_ArcToOnAnEmptyPathMoves(t *testing.T) {
	var p, q Path
	p.ArcTo(geom.Offset{X: 5, Y: 5}, 3, 1, 1)
	q.AddArc(geom.Offset{X: 5, Y: 5}, 3, 1, 1)
	if !p.Equal(&q) {
		t.Error("ArcTo on an empty path must start as AddArc")
	}
}

func TestPath_AddOval(t *testing.T) {
	var p Path
	p.AddOval(geom.Rect{X: 10, Y: 20, W: 60, H: 60})
	points, starts := p.Polygon()
	if len(starts) != 1 || !p.Closed(0) {
		t.Fatal("an oval is one closed contour")
	}
	onCircle(t, points, geom.Offset{X: 40, Y: 50}, 30)
	got := p.Bounds()
	if geom.Abs32(got.X-10) > 0.01 || geom.Abs32(got.Y-20) > 0.01 || geom.Abs32(got.W-60) > 0.01 || geom.Abs32(got.H-60) > 0.01 {
		t.Errorf("bounds %v", got)
	}

	var e Path
	e.AddOval(geom.Rect{W: 100, H: 40})
	if b := e.Bounds(); geom.Abs32(b.W-100) > 0.01 || geom.Abs32(b.H-40) > 0.01 {
		t.Errorf("ellipse bounds %v", b)
	}
}

func TestPath_DonutWithEvenOdd(t *testing.T) {
	var p Path
	p.Rule = FillEvenOdd
	p.AddOval(geom.Rect{W: 40, H: 40})
	p.AddOval(geom.Rect{X: 10, Y: 10, W: 20, H: 20})
	if _, starts := p.Polygon(); len(starts) != 2 {
		t.Errorf("%d contours, want 2", len(starts))
	}
}

func TestPath_HalfTurnIsTwoCubics(t *testing.T) {
	// float32(math.Pi) is a little more than π; it must not take a third
	// piece.
	var p Path
	p.AddArc(geom.Offset{}, 10, 0, math.Pi)
	var n int
	p.Walk(func(geom.Offset) {}, func(geom.Offset) {}, func(_, _ geom.Offset) {}, func(_, _, _ geom.Offset) { n++ }, func() {})
	if n != 2 {
		t.Errorf("a half turn is %d cubics, want 2", n)
	}
}
