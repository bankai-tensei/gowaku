package paint

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

func nearOffset(a, b geom.Offset) bool {
	return geom.Abs32(a.X-b.X) < 1e-3 && geom.Abs32(a.Y-b.Y) < 1e-3
}

// gradientOf returns the gradient of shape i, which must have one.
func gradientOf(t *testing.T, s *SVG, i int) Gradient {
	t.Helper()
	if i >= len(s.shapes) || s.shapes[i].grad == 0 {
		t.Fatalf("shape %d has no gradient: %+v", i, s.shapes)
	}
	return s.grads[s.shapes[i].grad-1].g
}

func TestParseSVG_LinearGradientUnits(t *testing.T) {
	s := mustParseSVG(t, svgDoc(`<linearGradient id="g"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient>
		<linearGradient id="u" gradientUnits="userSpaceOnUse" x1="2" x2="12" gradientTransform="translate(1 0)">
			<stop offset="0" stop-color="red"/><stop offset="100%" stop-color="blue"/></linearGradient>
		<rect x="10" y="0" width="20" height="10" fill="url(#g)"/>
		<rect x="10" y="0" width="20" height="10" fill="url(#u)" transform="scale(2)"/>`))
	g := gradientOf(t, s, 0)
	if g.Kind != GradientLinear || !nearOffset(g.Transform.Apply(g.Start), geom.Offset{X: 10}) || !nearOffset(g.Transform.Apply(g.End), geom.Offset{X: 30}) {
		t.Errorf("bounding box units: got %+v", g)
	}
	stops := *g.MoreStops
	if len(stops) != 2 || stops[0].Color != svgRed || stops[1].Color != svgBlue || stops[1].Offset != 1 {
		t.Errorf("got stops %v", stops)
	}
	u := gradientOf(t, s, 1)
	if !nearOffset(u.Transform.Apply(u.Start), geom.Offset{X: 6}) || !nearOffset(u.Transform.Apply(u.End), geom.Offset{X: 26}) {
		t.Errorf("user space: got %v %v", u.Transform.Apply(u.Start), u.Transform.Apply(u.End))
	}
}

func TestParseSVG_GradientHrefAndStops(t *testing.T) {
	s := mustParseSVG(t, svgDoc(`
		<linearGradient id="base" spreadMethod="reflect" x2="0.5"><stop offset="0.6" stop-color="red" stop-opacity="0.5"/><stop offset="0.2" style="stop-color:blue"/><stop offset="2" stop-color="lime"/></linearGradient>
		<linearGradient id="child" href="#base" x1="0.25"/>
		<radialGradient id="radial" xmlns:xlink="http://www.w3.org/1999/xlink" xlink:href="#child" fx="0.3"/>
		<linearGradient id="loop1" href="#loop2"/><linearGradient id="loop2" href="#loop1"/>
		<rect width="10" height="10" fill="url(#child)" fill-opacity="0.5"/>
		<rect width="10" height="10" fill="url(#radial)"/>
		<rect width="10" height="10" fill="url(#loop1)"/>`))
	g := gradientOf(t, s, 0)
	if g.Spread != SpreadReflect || !nearOffset(g.Start, geom.Offset{X: 0.25}) || !nearOffset(g.End, geom.Offset{X: 0.5}) {
		t.Errorf("inherited attributes: %+v", g)
	}
	stops := *g.MoreStops
	want := []GradientStop{
		{Offset: 0.6, Color: geom.Color{R: 1, A: 0.25}},
		{Offset: 0.6, Color: geom.Color{B: 1, A: 0.5}},
		{Offset: 1, Color: geom.Color{G: 1, A: 0.5}},
	}
	if len(stops) != len(want) {
		t.Fatalf("got %v", stops)
	}
	for i := range want {
		if stops[i] != want[i] {
			t.Errorf("stop %d: got %v, want %v", i, stops[i], want[i])
		}
	}
	r := gradientOf(t, s, 1)
	// A radial gradient takes the stops and spread through a linear one, but
	// not x1 or x2. Its focus moves it to the focal kind.
	if r.Kind != GradientFocal || r.Spread != SpreadReflect || !nearOffset(r.Start, geom.Offset{X: 0.5, Y: 0.5}) ||
		!nearOffset(r.End, geom.Offset{X: 1, Y: 0.5}) || !nearOffset(r.Focus, geom.Offset{X: 0.3, Y: 0.5}) {
		t.Errorf("radial: %+v", r)
	}
	if len(s.shapes) != 2 {
		t.Errorf("a loop without stops paints nothing: got %d shapes", len(s.shapes))
	}
}

func TestParseSVG_GradientEdgeCases(t *testing.T) {
	s := mustParseSVG(t, svgDoc(`
		<linearGradient id="one"><stop offset="0" stop-color="red"/></linearGradient>
		<linearGradient id="none"/>
		<linearGradient id="g"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient>
		<radialGradient id="zero" r="0"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></radialGradient>
		<rect width="10" height="10" fill="url(#one)"/>
		<rect width="10" height="10" fill="url(#none) red"/>
		<line x2="10" stroke="url(#g)"/>
		<rect width="10" height="10" fill="url(#zero)"/>
		<rect width="10" height="10" stroke="url(#g)" fill="none" stroke-width="2"/>`))
	// One stop is a color; no stops is none, even with a fallback; a line has
	// a box without height, so a gradient in its units paints nothing; a
	// radial gradient of no radius paints its last color.
	got := shapeColors(s)
	if len(got) != 3 || got[0] != svgRed || got[1] != svgBlue || s.shapes[2].grad == 0 {
		t.Errorf("got %+v", s.shapes)
	}
	g := gradientOf(t, s, 2)
	// The stroke takes the box of the geometry, not of the stroke.
	if !nearOffset(g.Transform.Apply(g.Start), geom.Offset{}) || !nearOffset(g.Transform.Apply(g.End), geom.Offset{X: 10}) {
		t.Errorf("stroke box: %v %v", g.Transform.Apply(g.Start), g.Transform.Apply(g.End))
	}
}

func TestSVG_DrawScalesGradients(t *testing.T) {
	s := mustParseSVG(t, svgDoc(`<linearGradient id="g"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient>
		<rect x="12" width="12" height="24" fill="url(#g)"/>`))
	c := newCanvas()
	s.Draw(c, geom.Rect{X: 100, Y: 0, W: 48, H: 48}, geom.Color{A: 1})
	ops := c.DisplayList().Ops
	if len(ops) != 1 || !ops[0].HasGradient {
		t.Fatalf("got %+v", ops)
	}
	g := ops[0].Gradient
	if !nearOffset(g.Transform.Apply(g.Start), geom.Offset{X: 124}) || !nearOffset(g.Transform.Apply(g.End), geom.Offset{X: 148}) {
		t.Errorf("got %v %v", g.Transform.Apply(g.Start), g.Transform.Apply(g.End))
	}
	frame := func() {
		c.Reset()
		s.Draw(c, geom.Rect{X: 100, Y: 0, W: 48, H: 48}, geom.Color{A: 1})
	}
	frame()
	if got := testing.AllocsPerRun(20, frame); got != 0 {
		t.Errorf("got %v allocs", got)
	}
}
