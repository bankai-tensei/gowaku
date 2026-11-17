package paint

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

var (
	svgRed  = geom.Color{R: 1, A: 1}
	svgBlue = geom.Color{B: 1, A: 1}
)

func shapeColors(s *SVG) []geom.Color {
	out := make([]geom.Color, len(s.shapes))
	for i := range s.shapes {
		out[i] = s.shapes[i].color
	}
	return out
}

func TestParseSVG_StyleElement(t *testing.T) {
	cases := []struct {
		name, body string
		want       []geom.Color
	}{
		{"type", `<style>rect{fill:red}</style><rect width="4" height="4"/>`, []geom.Color{svgRed}},
		{"class", `<style>.a{fill:red}</style><rect class="b a" width="4" height="4"/><rect class="b" width="4" height="4"/>`,
			[]geom.Color{svgRed, {A: 1}}},
		{"id beats class beats type", `<style>#x{fill:red} .a{fill:blue} rect{fill:green}</style><rect id="x" class="a" width="4" height="4"/><rect class="a" width="4" height="4"/>`,
			[]geom.Color{svgRed, svgBlue}},
		{"later rule wins at equal specificity", `<style>.a{fill:red} .a{fill:blue}</style><rect class="a" width="4" height="4"/>`, []geom.Color{svgBlue}},
		{"compound", `<style>rect.a.b{fill:red}</style><rect class="a b" width="4" height="4"/><rect class="a" width="4" height="4"/>`,
			[]geom.Color{svgRed, {A: 1}}},
		{"list", `<style>circle, .a {fill:red}</style><rect class="a" width="4" height="4"/>`, []geom.Color{svgRed}},
		{"descendant and child", `<style>g .a{fill:red} svg > .a{fill:blue}</style><g><g><rect class="a" width="4" height="4"/></g></g><rect class="a" width="4" height="4"/>`,
			[]geom.Color{svgRed, svgBlue}},
		{"attribute loses to rule", `<style>.a{fill:red}</style><rect class="a" fill="blue" width="4" height="4"/>`, []geom.Color{svgRed}},
		{"style attribute beats rule", `<style>.a{fill:red}</style><rect class="a" style="fill:blue" width="4" height="4"/>`, []geom.Color{svgBlue}},
		{"important beats style attribute", `<style>.a{fill:red !important}</style><rect class="a" style="fill:blue" width="4" height="4"/>`, []geom.Color{svgRed}},
		{"comments and at-rules", `<style>/* .a{fill:blue} */ @media print { .a{fill:blue} } @import "x.css"; .a{fill:red}</style><rect class="a" width="4" height="4"/>`,
			[]geom.Color{svgRed}},
		{"unsupported selectors match nothing", `<style>rect:hover{fill:red} rect[x]{fill:red} g + rect{fill:red} .a{fill:blue}</style><rect class="a" width="4" height="4"/>`,
			[]geom.Color{svgBlue}},
		{"cdata and inheritance", `<style><![CDATA[ g.k { fill: red } ]]></style><g class="k"><rect width="4" height="4"/></g>`, []geom.Color{svgRed}},
		{"style after use", `<rect class="a" width="4" height="4"/><style>.a{fill:red}</style>`, []geom.Color{svgRed}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := mustParseSVG(t, svgDoc(c.body))
			got := shapeColors(s)
			if fmt.Sprint(got) != fmt.Sprint(c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestParseSVG_Use(t *testing.T) {
	cases := []struct {
		name, body string
		bounds     []geom.Rect
	}{
		{"href with x and y", `<defs><rect id="r" width="4" height="4"/></defs><use href="#r" x="10" y="2"/>`,
			[]geom.Rect{{X: 10, Y: 2, W: 4, H: 4}}},
		{"xlink:href, forward", `<use xmlns:xlink="http://www.w3.org/1999/xlink" xlink:href="#r" x="1"/><rect id="r" width="4" height="4"/>`,
			[]geom.Rect{{X: 1, W: 4, H: 4}, {W: 4, H: 4}}},
		{"transform then x", `<defs><rect id="r" width="4" height="4"/></defs><use href="#r" transform="scale(2)" x="1"/>`,
			[]geom.Rect{{X: 2, W: 8, H: 8}}},
		{"group", `<defs><g id="g"><rect width="1" height="1"/><rect x="2" width="1" height="1"/></g></defs><use href="#g" y="5"/>`,
			[]geom.Rect{{Y: 5, W: 1, H: 1}, {X: 2, Y: 5, W: 1, H: 1}}},
		{"symbol with viewBox", `<symbol id="s" viewBox="0 0 2 2"><rect width="2" height="2"/></symbol><use href="#s" x="4" y="4" width="10" height="10"/>`,
			[]geom.Rect{{X: 4, Y: 4, W: 10, H: 10}}},
		{"symbol without size fills the viewport", `<symbol id="s" viewBox="0 0 1 1"><rect width="1" height="1"/></symbol><use href="#s"/>`,
			[]geom.Rect{{W: 24, H: 24}}},
		{"symbol alone draws nothing", `<symbol id="s"><rect width="1" height="1"/></symbol>`, nil},
		{"nested use", `<defs><rect id="r" width="1" height="1"/><use id="u" href="#r" x="1"/></defs><use href="#u" x="1"/>`,
			[]geom.Rect{{X: 2, W: 1, H: 1}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := mustParseSVG(t, svgDoc(c.body))
			if len(s.shapes) != len(c.bounds) {
				t.Fatalf("got %d shapes, want %d", len(s.shapes), len(c.bounds))
			}
			for i, want := range c.bounds {
				got := s.shapes[i].path.Bounds()
				if geom.Abs32(got.X-want.X) > 1e-3 || geom.Abs32(got.Y-want.Y) > 1e-3 || geom.Abs32(got.W-want.W) > 1e-3 || geom.Abs32(got.H-want.H) > 1e-3 {
					t.Errorf("shape %d: got %v, want %v", i, got, want)
				}
			}
			if s.Skipped != 0 {
				t.Errorf("got %d skipped", s.Skipped)
			}
		})
	}
}

func TestParseSVG_UseInheritsFromTheUse(t *testing.T) {
	s := mustParseSVG(t, svgDoc(`<defs><rect id="r" width="4" height="4"/><rect id="own" fill="blue" width="4" height="4"/></defs>
		<use href="#r" fill="red"/><use href="#own" fill="red"/>`))
	if got := shapeColors(s); fmt.Sprint(got) != fmt.Sprint([]geom.Color{svgRed, svgBlue}) {
		t.Errorf("got %v", got)
	}
}

func TestParseSVG_UseLoopsAreCaught(t *testing.T) {
	for _, body := range []string{
		`<g id="a"><rect width="1" height="1"/><use href="#a"/></g>`,
		`<use id="u1" href="#u2"/><use id="u2" href="#u1"/>`,
		`<use id="self" href="#self"/>`,
		`<symbol id="s"><use href="#s"/></symbol><use href="#s"/>`,
	} {
		s := mustParseSVG(t, svgDoc(body))
		if s.Skipped == 0 {
			t.Errorf("%s: want the loop counted", body)
		}
	}
}

func TestParseSVG_UseExpansionIsBounded(t *testing.T) {
	// Each level uses the one before ten times: 10^8 rectangles in full.
	var b strings.Builder
	b.WriteString(`<defs><rect id="l0" width="1" height="1"/>`)
	for level := 1; level <= 8; level++ {
		fmt.Fprintf(&b, `<g id="l%d">`, level)
		for range 10 {
			fmt.Fprintf(&b, `<use href="#l%d"/>`, level-1)
		}
		b.WriteString(`</g>`)
	}
	b.WriteString(`</defs><use href="#l8"/>`)
	s := mustParseSVG(t, svgDoc(b.String()))
	if s.Skipped == 0 || len(s.shapes) > svgVisitBudget {
		t.Errorf("got %d shapes, %d skipped", len(s.shapes), s.Skipped)
	}
}

func TestParseSVG_DeepNestingIsCut(t *testing.T) {
	depth := 2000
	doc := svgDoc(strings.Repeat("<g>", depth) + `<rect width="1" height="1"/>` + strings.Repeat("</g>", depth))
	s := mustParseSVG(t, doc)
	if s.Skipped == 0 {
		t.Error("want the deep elements counted")
	}
}

func TestParseSVG_ElementsDrawnOnlyByReference(t *testing.T) {
	s := mustParseSVG(t, svgDoc(`<defs><rect width="1" height="1"/></defs><clipPath><rect width="1" height="1"/></clipPath>
		<mask><rect width="1" height="1"/></mask><marker><rect width="1" height="1"/></marker><pattern><rect width="1" height="1"/></pattern>
		<a><rect width="2" height="2"/></a><switch><rect width="3" height="3"/><rect width="5" height="5"/></switch>`))
	if len(s.shapes) != 2 || s.Skipped != 0 {
		t.Errorf("got %d shapes, %d skipped", len(s.shapes), s.Skipped)
	}
}

func FuzzParseSVG(f *testing.F) {
	for _, seed := range []string{
		svgDoc(`<style>.a{fill:red} g > rect.b#c{stroke:blue}</style><g class="a"><rect class="b" id="c" width="4" height="4"/></g>`),
		svgDoc(`<defs><symbol id="s" viewBox="0 0 2 2"><circle cx="1" cy="1" r="1"/></symbol></defs><use href="#s" x="2" width="8" height="8"/>`),
		svgDoc(`<g id="a"><use href="#a"/></g><use href="#b"/><use id="b" href="#a"/>`),
		svgDoc(`<linearGradient id="g" x2="1" spreadMethod="reflect" gradientTransform="rotate(30)"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient><rect width="9" height="9" fill="url(#g)"/>`),
		svgDoc(`<radialGradient id="r" fx="0.2" href="#g"/><circle r="4" cx="5" cy="5" fill="url(#r)" stroke="url(#g)"/>`),
		svgDoc(`<path d="M2 2h20v20" stroke="black" stroke-dasharray="2 1 3" stroke-dashoffset="1" marker-end="url(#m)"/><marker id="m" orient="auto-start-reverse" viewBox="0 0 4 4" refX="2" refY="2"><path d="M0 0L4 2L0 4z"/></marker>`),
		svgDoc(`<clipPath id="c" clipPathUnits="objectBoundingBox"><circle cx=".5" cy=".5" r=".5"/></clipPath><mask id="m"><rect width="24" height="24" fill="white"/></mask><g clip-path="url(#c)" mask="url(#m)" opacity=".5"><rect width="24" height="24"/></g>`),
		svgDoc(`<filter id="f" x="-1" y="-1" width="3" height="3"><feGaussianBlur stdDeviation="2"/><feOffset dx="1" result="o"/><feMerge><feMergeNode in="o"/><feMergeNode in="SourceGraphic"/></feMerge></filter><rect width="9" height="9" filter="url(#f)"/>`),
		svgDoc(`<text x="2 4 6" y="12" dx="1" text-anchor="middle" font-size="8">a<tspan dy="2" font-weight="bold">b</tspan>c</text>`),
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		s, err := ParseSVG(data)
		if err != nil {
			return
		}
		c := newCanvas()
		s.Draw(c, geom.Rect{W: 32, H: 32}, geom.Color{A: 1})
		if err := s.SetText(boxOutliner{}); err != nil {
			t.Fatalf("parsed once, then not: %v", err)
		}
		s.Draw(c, geom.Rect{W: 32, H: 32}, geom.Color{A: 1})
	})
}
