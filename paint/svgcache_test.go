package paint

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

// A drawing with groups draws in a cached group at the corner of dst, whose
// version changes with the size and the color, not with the place.
func TestSVG_DrawCachedKeepsTheGroupAcrossMoves(t *testing.T) {
	s := mustParseSVG(t, svgDoc(`<g opacity="0.5"><rect width="12" height="24" fill="currentColor"/><rect x="6" width="12" height="24" fill="blue"/></g>`))
	var cache SVGCache
	c := newCanvas()
	draw := func(dst geom.Rect, color geom.Color) (uint64, *DisplayList) {
		c.Reset()
		s.DrawCached(c, dst, color, &cache)
		dl := c.DisplayList()
		if len(dl.Groups) != 2 || dl.Groups[0].Style.Cache != &cache.group || dl.Ops[0].Origin != dst.Origin() {
			t.Fatalf("groups %+v, origin %v", dl.Groups, dl.Ops[0].Origin)
		}
		return dl.Groups[0].Style.Cache.Version(), dl
	}
	black := geom.Color{A: 1}
	v, dl := draw(geom.Rect{X: 10, Y: 20, W: 24, H: 24}, black)
	// The content draws from the corner, clipped to the viewport only.
	if p := dl.Ops[2]; !p.Clipped || p.Clip != (geom.Rect{X: 10, Y: 20, W: 24, H: 24}) {
		t.Errorf("content clip %v %v", p.Clipped, p.Clip)
	}
	if got, _ := draw(geom.Rect{X: 50, Y: 7, W: 24, H: 24}, black); got != v {
		t.Errorf("moving changed the version from %d to %d", v, got)
	}
	if got, _ := draw(geom.Rect{X: 50, Y: 7, W: 30, H: 24}, black); got == v {
		t.Error("a new size kept the version")
	}
	v, _ = draw(geom.Rect{X: 50, Y: 7, W: 30, H: 24}, black)
	if got, _ := draw(geom.Rect{X: 50, Y: 7, W: 30, H: 24}, geom.Color{R: 1, A: 1}); got == v {
		t.Error("a new color kept the version")
	}
	frame := func() {
		c.Reset()
		s.DrawCached(c, geom.Rect{X: 3, Y: 4, W: 30, H: 24}, black, &cache)
	}
	if n := testing.AllocsPerRun(20, frame); n != 0 {
		t.Errorf("%v allocs at a known size", n)
	}
}

// An icon, without groups, draws as Draw draws it.
func TestSVG_DrawCachedIconIsDraw(t *testing.T) {
	s := mustParseSVG(t, svgDoc(`<rect x="2" y="2" width="20" height="20"/>`))
	var cache SVGCache
	c := newCanvas()
	s.DrawCached(c, geom.Rect{X: 8, Y: 8, W: 24, H: 24}, geom.Color{A: 1}, &cache)
	if dl := c.DisplayList(); len(dl.Groups) != 0 || len(dl.Ops) != 1 || dl.Ops[0].Kind != OpPath {
		t.Errorf("got %d ops and %d groups, want the path", len(dl.Ops), len(dl.Groups))
	}
}
