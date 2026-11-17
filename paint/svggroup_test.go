package paint

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

// opKinds returns the kinds of the operations of s, as a string: s for a
// shape, ( for a push and ) for a pop.
func opKinds(ops []svgOp) string {
	b := make([]byte, len(ops))
	for i, o := range ops {
		b[i] = "s()"[o.kind]
	}
	return string(b)
}

func TestParseSVG_GroupsFold(t *testing.T) {
	cases := []struct {
		name, body, ops string
		composite       bool
	}{
		{"opacity on one shape folds", `<rect width="4" height="4" opacity="0.5"/>`, "s", false},
		{"opacity on a group of one", `<g opacity="0.5"><g><rect width="4" height="4"/></g></g>`, "s", false},
		{"fill and stroke composite", `<rect width="4" height="4" stroke="red" opacity="0.5"/>`, "(ss)", true},
		{"group of two composites", `<g opacity="0.5"><rect width="4" height="4"/><rect width="4" height="4"/></g>`, "(ss)", true},
		{"clip", `<clipPath id="c"><rect width="2" height="2"/></clipPath><rect width="4" height="4" clip-path="url(#c)"/>`, "(s)", true},
		{"clip to nothing", `<clipPath id="c"/><rect width="4" height="4" clip-path="url(#c)"/><rect width="4" height="4"/>`, "s", false},
		{"missing clip clips nothing", `<rect width="4" height="4" clip-path="url(#none)"/>`, "s", false},
		{"invisible group", `<g opacity="0"><rect width="4" height="4"/></g>`, "", false},
		{"empty group", `<g opacity="0.5"/><rect width="4" height="4"/>`, "s", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := mustParseSVG(t, svgDoc(c.body))
			if got := opKinds(s.ops); got != c.ops || s.composite != c.composite {
				t.Errorf("got %q composite %v, want %q %v", got, s.composite, c.ops, c.composite)
			}
		})
	}
	s := mustParseSVG(t, svgDoc(`<g opacity="0.5"><rect width="4" height="4" fill="red" fill-opacity="0.5"/></g>`))
	if s.shapes[0].color != (geom.Color{R: 1, A: 0.25}) {
		t.Errorf("folded opacity: got %v", s.shapes[0].color)
	}
}

func TestParseSVG_ClipPathUnitsAndNesting(t *testing.T) {
	s := mustParseSVG(t, svgDoc(`
		<clipPath id="box" clipPathUnits="objectBoundingBox" clip-rule="evenodd"><rect x="0.5" width="0.5" height="1"/></clipPath>
		<clipPath id="outer" clip-path="url(#box)"><circle cx="12" cy="12" r="6"/></clipPath>
		<g clip-path="url(#outer)"><rect x="4" y="4" width="16" height="16"/></g>`))
	if opKinds(s.ops) != "(s)" {
		t.Fatalf("got %q", opKinds(s.ops))
	}
	clip := s.groups[s.ops[0].index].clip
	if clip == nil || len(clip.paths) != 1 || clip.clip == nil || len(clip.clip.paths) != 1 {
		t.Fatalf("got %+v", clip)
	}
	inner := clip.clip.paths[0]
	// The box of the group is 4 to 20, so the right half starts at 12.
	if b := inner.Bounds(); b != (geom.Rect{X: 12, Y: 4, W: 8, H: 16}) || inner.Rule != FillEvenOdd {
		t.Errorf("got %v rule %v", b, inner.Rule)
	}
}

func TestParseSVG_ClipLoopsAreCaught(t *testing.T) {
	s := mustParseSVG(t, svgDoc(`<clipPath id="a" clip-path="url(#b)"><rect width="9" height="9"/></clipPath>
		<clipPath id="b" clip-path="url(#a)"><rect width="9" height="9"/></clipPath>
		<rect width="4" height="4" clip-path="url(#a)"/>
		<mask id="m"><rect width="9" height="9" fill="white" mask="url(#m)"/></mask><rect width="4" height="4" mask="url(#m)"/>`))
	if s.Skipped == 0 {
		t.Error("want the loops counted")
	}
}

func TestParseSVG_Mask(t *testing.T) {
	s := mustParseSVG(t, svgDoc(`<mask id="m" style="mask-type:alpha"><rect width="24" height="24" fill="white"/></mask>
		<rect x="2" y="2" width="10" height="10" mask="url(#m)"/>`))
	if opKinds(s.ops) != "(s)" {
		t.Fatalf("got %q", opKinds(s.ops))
	}
	m := s.groups[s.ops[0].index].mask
	if m == nil || !m.alpha || opKinds(m.ops) != "s" {
		t.Fatalf("got %+v", m)
	}
	// The default region is the box widened by 10% on every side.
	if b := m.region.Bounds(); geom.Abs32(b.X-1) > 1e-4 || geom.Abs32(b.Y-1) > 1e-4 || geom.Abs32(b.W-12) > 1e-4 || geom.Abs32(b.H-12) > 1e-4 {
		t.Errorf("got region %v", b)
	}
}

func TestParseSVG_MarkerClip(t *testing.T) {
	marker := func(content string) *SVG {
		return mustParseSVG(t, svgDoc(`<marker id="m" markerWidth="4" markerHeight="4" markerUnits="userSpaceOnUse">`+content+`</marker>
			<line x1="2" y1="2" x2="20" y2="2" stroke="black" marker-end="url(#m)"/>`))
	}
	if s := marker(`<rect width="4" height="4"/>`); s.composite {
		t.Error("content inside the viewport needs no clip")
	}
	if s := marker(`<rect width="8" height="8"/>`); !s.composite {
		t.Error("content past the viewport must be clipped")
	}
}

func TestSVG_DrawComposites(t *testing.T) {
	s := mustParseSVG(t, svgDoc(`<g opacity="0.5"><rect width="12" height="24" fill="red"/><rect x="6" width="12" height="24" fill="blue"/></g>`))
	if !s.composite {
		t.Fatal("want a composited drawing")
	}
	c := newCanvas()
	s.Draw(c, geom.Rect{X: 10, Y: 20, W: 24, H: 24}, geom.Color{A: 1})
	dl := c.DisplayList()
	var kinds []OpKind
	for _, op := range dl.Ops {
		kinds = append(kinds, op.Kind)
	}
	want := []OpKind{OpGroupPush, OpPath, OpPath, OpGroupPop}
	if len(kinds) != len(want) {
		t.Fatalf("got %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("got %v, want %v", kinds, want)
		}
	}
	// The group fades as one picture, and its content draws at full opacity,
	// inside the viewport.
	if a := dl.Ops[0].Color.A; a != 0.5 {
		t.Errorf("group opacity %v", a)
	}
	if a := dl.Ops[1].Color.A; a != 1 {
		t.Errorf("content opacity %v", a)
	}
	if !dl.Ops[0].Clipped || dl.Ops[0].Clip != (geom.Rect{X: 10, Y: 20, W: 24, H: 24}) {
		t.Errorf("clip %v %v", dl.Ops[0].Clipped, dl.Ops[0].Clip)
	}
	frame := func() {
		c.Reset()
		s.Draw(c, geom.Rect{X: 10, Y: 20, W: 24, H: 24}, geom.Color{A: 1})
	}
	if got := testing.AllocsPerRun(20, frame); got != 0 {
		t.Errorf("got %v allocs at a known size", got)
	}
}

// A clip and a mask on one element nest: the mask around the clip, whose
// paths draw as an alpha mask, and the filter on the inner group.
func TestSVG_DrawEffects(t *testing.T) {
	s := mustParseSVG(t, svgDoc(`<clipPath id="c"><circle cx="12" cy="12" r="8"/></clipPath>
		<mask id="m"><rect width="24" height="24" fill="white"/></mask>
		<filter id="f"><feGaussianBlur stdDeviation="1"/></filter>
		<rect x="2" y="2" width="20" height="20" clip-path="url(#c)" mask="url(#m)" filter="url(#f)" opacity="0.5"/>`))
	c := newCanvas()
	s.Draw(c, geom.Rect{W: 48, H: 48}, geom.Color{A: 1})
	dl := c.DisplayList()
	var kinds []OpKind
	for _, op := range dl.Ops {
		kinds = append(kinds, op.Kind)
	}
	want := []OpKind{OpGroupPush, OpGroupPush, OpPath, OpGroupMask, OpPath, OpGroupPop, OpGroupMask,
		OpClipPush, OpPath, OpClipPop, OpGroupPop}
	if len(kinds) != len(want) {
		t.Fatalf("got %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("got %v, want %v", kinds, want)
		}
	}
	outer, inner := &dl.Groups[dl.Ops[0].ShaderCall], &dl.Groups[dl.Ops[1].ShaderCall]
	if outer.Style.Mask != MaskLuminance || outer.Style.Filter != nil || dl.Ops[0].Color.A != 0.5 {
		t.Errorf("outer group %+v, opacity %v", outer.Style, dl.Ops[0].Color.A)
	}
	if inner.Style.Mask != MaskAlpha || inner.Style.Filter == nil || dl.Ops[1].Color.A != 1 {
		t.Errorf("inner group %+v, opacity %v", inner.Style, dl.Ops[1].Color.A)
	}
	// The filter is in user space, which the drawing scales by 2 onto the
	// canvas.
	f := inner.Style.Filter
	if f.Transform.A != 2 || f.Transform.D != 2 || len(f.Primitives) != 1 || f.Primitives[0].StdDev != (geom.Offset{X: 1, Y: 1}) {
		t.Errorf("filter %+v", f)
	}
}
