package paint

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

func TestParseSVG_Filter(t *testing.T) {
	s := mustParseSVG(t, svgDoc(`<filter id="f" primitiveUnits="objectBoundingBox">
			<feGaussianBlur in="SourceAlpha" stdDeviation="0.1" result="b"/>
			<feOffset dx="0.5" result="o"/>
			<feMerge><feMergeNode in="o"/><feMergeNode in="SourceGraphic"/><feMergeNode in="nothing"/></feMerge>
		</filter>
		<rect x="2" y="2" width="10" height="20" filter="url(#f)"/>`))
	if opKinds(s.ops) != "(s)" || s.Skipped != 0 {
		t.Fatalf("got %q, %d skipped", opKinds(s.ops), s.Skipped)
	}
	f := s.groups[s.ops[0].index].filter
	if f == nil || len(f.prims) != 3 {
		t.Fatalf("got %+v", f)
	}
	// The default region is the box widened by 10%; primitive units scale
	// lengths by the box.
	if r := f.region; geom.Abs32(r.X-1) > 1e-4 || geom.Abs32(r.Y) > 1e-4 || geom.Abs32(r.W-12) > 1e-4 || geom.Abs32(r.H-24) > 1e-4 {
		t.Errorf("region %v", r)
	}
	b, o, m := f.prims[0], f.prims[1], f.prims[2]
	if b.in != feSourceAlpha || b.stdDev != [2]float32{1, 2} || !b.linear {
		t.Errorf("blur %+v", b)
	}
	if o.in != 0 || o.dx != 5 {
		t.Errorf("offset %+v", o)
	}
	if len(m.merge) != 3 || m.merge[0] != 1 || m.merge[1] != feSourceGraphic || m.merge[2] != 1 {
		t.Errorf("merge %+v", m.merge)
	}
}

func TestParseSVG_FilterFallbacks(t *testing.T) {
	cases := []struct {
		name, body, ops string
		skipped         int
	}{
		{"unsupported primitive draws unfiltered", `<filter id="f"><feTurbulence baseFrequency="0.1"/></filter><rect width="4" height="4" filter="url(#f)"/>`, "s", 1},
		{"missing filter draws unfiltered", `<rect width="4" height="4" filter="url(#none)"/>`, "s", 0},
		{"css function is not read", `<rect width="4" height="4" filter="blur(2px)"/>`, "s", 1},
		{"empty filter draws nothing", `<filter id="f"/><rect width="4" height="4" filter="url(#f)"/>`, "(s)", 0},
		{"flood draws without content", `<filter id="f" filterUnits="userSpaceOnUse" x="0" y="0" width="4" height="4"><feFlood/></filter><g filter="url(#f)"/>`, "()", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := mustParseSVG(t, svgDoc(c.body))
			if got := opKinds(s.ops); got != c.ops || s.Skipped != c.skipped {
				t.Errorf("got %q, %d skipped; want %q, %d", got, s.Skipped, c.ops, c.skipped)
			}
		})
	}
}
