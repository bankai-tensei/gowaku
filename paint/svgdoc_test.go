package paint

import (
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

func mustParseSVG(t testing.TB, doc string) *SVG {
	t.Helper()
	s, err := ParseSVG([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func svgDoc(body string) string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24">` + body + `</svg>`
}

func TestParseSVG_Elements(t *testing.T) {
	cases := []struct {
		name, body string
		in, out    [][2]float32
	}{
		{"path", `<path d="M2 2h10v10H2z"/>`, [][2]float32{{5, 5}}, [][2]float32{{13, 5}}},
		{"rect", `<rect x="2" y="3" width="10" height="5"/>`, [][2]float32{{11, 7}}, [][2]float32{{11, 9}}},
		{"rounded rect", `<rect x="2" y="2" width="20" height="20" rx="6"/>`, [][2]float32{{12, 3}, {4, 4}}, [][2]float32{{2.3, 2.3}}},
		{"circle", `<circle cx="12" cy="12" r="5"/>`, [][2]float32{{12, 8}, {15, 15}}, [][2]float32{{16, 16}}},
		{"ellipse", `<ellipse cx="12" cy="12" rx="8" ry="3"/>`, [][2]float32{{19, 12}}, [][2]float32{{12, 16}}},
		{"polygon", `<polygon points="2,2 20,2 2,20"/>`, [][2]float32{{5, 5}}, [][2]float32{{15, 15}}},
		{"polyline fills", `<polyline points="2 2 20 2 2 20"/>`, [][2]float32{{5, 5}}, [][2]float32{{15, 15}}},
		{"line strokes", `<line x1="2" y1="12" x2="22" y2="12" stroke="red" stroke-width="2"/>`, [][2]float32{{12, 12.8}}, [][2]float32{{12, 13.2}, {1.5, 12}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := mustParseSVG(t, svgDoc(c.body))
			if len(s.shapes) != 1 {
				t.Fatalf("got %d shapes", len(s.shapes))
			}
			p := s.shapes[0].path
			for _, q := range c.in {
				if !inside(p, q[0], q[1]) {
					t.Errorf("%v must be inside", q)
				}
			}
			for _, q := range c.out {
				if inside(p, q[0], q[1]) {
					t.Errorf("%v must be outside", q)
				}
			}
		})
	}
}

func TestParseSVG_SizeAndViewBox(t *testing.T) {
	s := mustParseSVG(t, `<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 -960 960 960"><path d="M0-960h960v960H0z"/></svg>`)
	if s.ViewBox != (geom.Rect{Y: -960, W: 960, H: 960}) || s.Width != 48 || s.Height != 48 {
		t.Errorf("got %v %v×%v", s.ViewBox, s.Width, s.Height)
	}
	s = mustParseSVG(t, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 30 15"/>`)
	if s.Width != 30 || s.Height != 15 {
		t.Errorf("the view box gives the size: %v×%v", s.Width, s.Height)
	}
	s = mustParseSVG(t, `<svg xmlns="http://www.w3.org/2000/svg" width="10mm" height="2in"/>`)
	if geom.Abs32(s.Width-37.795) > 0.01 || s.Height != 192 || s.ViewBox.W != s.Width {
		t.Errorf("got %v×%v", s.Width, s.Height)
	}
	s = mustParseSVG(t, `<svg xmlns="http://www.w3.org/2000/svg" width="100%"/>`)
	if s.Width != 300 || s.Height != 150 {
		t.Errorf("got %v×%v", s.Width, s.Height)
	}
}

func TestParseSVG_InheritanceAndCurrentColor(t *testing.T) {
	s := mustParseSVG(t, svgDoc(`
		<g fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
			<g opacity="0.5"><path d="M4 4L20 4"/></g>
			<path d="M4 8h16" stroke="#ff0000" stroke-opacity="0.5"/>
			<path d="M4 12h16" color="#00ff00"/>
			<path d="M4 16h16" stroke="inherit" fill="blue"/>
		</g>`))
	if len(s.shapes) != 5 {
		t.Fatalf("got %d shapes", len(s.shapes))
	}
	if sh := s.shapes[0]; !sh.current || sh.color.A != 0.5 {
		t.Errorf("first: %+v", sh)
	}
	if !inside(s.shapes[0].path, 3.2, 4) {
		t.Error("round caps are inherited")
	}
	if sh := s.shapes[1]; sh.current || sh.color != (geom.Color{R: 1, A: 0.5}) {
		t.Errorf("second: %+v", sh)
	}
	if sh := s.shapes[2]; sh.current || sh.color != (geom.Color{G: 1, A: 1}) {
		t.Errorf("color sets currentColor: %+v", sh)
	}
	if sh := s.shapes[3]; sh.color != (geom.Color{B: 1, A: 1}) {
		t.Errorf("fill comes first: %+v", sh)
	}
	if sh := s.shapes[4]; !sh.current {
		t.Errorf("inherit: %+v", sh)
	}
}

func TestParseSVG_StyleAttributeWins(t *testing.T) {
	s := mustParseSVG(t, svgDoc(`<rect width="10" height="10" fill="red" style="fill: rgb(0, 0, 255); fill-opacity:50%"/>`))
	if len(s.shapes) != 1 || s.shapes[0].color != (geom.Color{B: 1, A: 0.5}) {
		t.Errorf("got %+v", s.shapes)
	}
	s = mustParseSVG(t, svgDoc(`<rect width="10" height="10" style="display:none"/><rect width="10" height="10" display="none"/>`))
	if len(s.shapes) != 0 {
		t.Error("display none draws nothing")
	}
}

func TestParseSVG_Transforms(t *testing.T) {
	cases := []struct {
		transform string
		in, out   [2]float32
	}{
		{"translate(10 0)", [2]float32{11, 1}, [2]float32{1, 1}},
		{"translate(10)", [2]float32{11, 1}, [2]float32{1, 1}},
		{"scale(2)", [2]float32{3.5, 3.5}, [2]float32{4.5, 1}},
		{"scale(2, 1)", [2]float32{3.5, 1}, [2]float32{1, 3}},
		{"rotate(90)", [2]float32{-1, 1}, [2]float32{1, 1}},
		{"rotate(90 2 2)", [2]float32{3, 1}, [2]float32{1, 1}},
		{"matrix(1 0 0 1 5 5)", [2]float32{6, 6}, [2]float32{1, 1}},
		{"skewX(45)", [2]float32{3, 1.5}, [2]float32{0.5, 1.5}},
		{"skewY(45)", [2]float32{1.5, 3}, [2]float32{1.5, 0.5}},
		{"translate(10,0) scale(2)", [2]float32{13, 3}, [2]float32{3, 3}},
	}
	for _, c := range cases {
		t.Run(c.transform, func(t *testing.T) {
			s := mustParseSVG(t, svgDoc(`<g transform="`+c.transform+`"><rect width="2" height="2"/></g>`))
			p := s.shapes[0].path
			if !inside(p, c.in[0], c.in[1]) || inside(p, c.out[0], c.out[1]) {
				t.Errorf("got bounds %v", p.Bounds())
			}
		})
	}
	// A stroke under a scale is scaled with it, as SVG says.
	s := mustParseSVG(t, svgDoc(`<line transform="scale(1 3)" x1="0" y1="2" x2="20" y2="2" stroke="black"/>`))
	if b := s.shapes[0].path.Bounds(); geom.Abs32(b.H-3) > 0.01 || geom.Abs32(b.Y-4.5) > 0.01 {
		t.Errorf("got %v", b)
	}
}

func TestParseSVG_Skipped(t *testing.T) {
	s := mustParseSVG(t, svgDoc(`
		<title>t</title><desc>d</desc>
		<pattern id="p" width="2" height="2"><rect width="1" height="1"/></pattern>
		<image href="x.png" width="4" height="4"/>
		<foreignObject width="4" height="4"/>
		<rect width="4" height="4" fill="url(#p)"/>
		<rect width="4" height="4" fill="url(#p) red"/>
		<rect width="4" height="4" fill="url(#missing)"/>
		<use href="other.svg#a"/>
		<foo:bar xmlns:foo="urn:x"/>`))
	// The image, the foreign object, the two references to a pattern and the
	// use of another document. A reference to nothing draws nothing in a
	// browser either.
	if s.Skipped != 5 {
		t.Errorf("got %d skipped, want 5", s.Skipped)
	}
	if len(s.shapes) != 1 || s.shapes[0].color != (geom.Color{R: 1, A: 1}) {
		t.Errorf("the fallback color applies: %+v", s.shapes)
	}
}

func TestParseSVG_Errors(t *testing.T) {
	for _, doc := range []string{
		`<svg xmlns="http://www.w3.org/2000/svg"><path d="M0 0"></svg>`,
		svgDoc(`<rect width="1x0" height="4"/>`),
		svgDoc(`<path d="M0 0 Q"/>`),
		svgDoc(`<g transform="spin(4)"/>`),
		svgDoc(`<g transform="translate(1 2 3)"/>`),
		svgDoc(`<rect width="4" height="4" opacity="half"/>`),
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 0 4"/>`,
		`<html/>`,
		``,
	} {
		if _, err := ParseSVG([]byte(doc)); err == nil {
			t.Errorf("want an error for %q", doc)
		} else if !strings.HasPrefix(err.Error(), "paint: parse svg") {
			t.Errorf("got %v", err)
		}
	}
}

func TestParseSVG_ColorsAndRules(t *testing.T) {
	for in, want := range map[string]geom.Color{
		"#f00":                 {R: 1, A: 1},
		"#ff000080":            {R: 1, A: 128.0 / 255},
		"rebeccapurple":        {R: 0x66 / 255.0, G: 0x33 / 255.0, B: 0x99 / 255.0, A: 1},
		"RGBA(255, 0, 0, 0.5)": {R: 1, A: 0.5},
		"rgb(100% 0% 0%)":      {R: 1, A: 1},
	} {
		if got, ok := parseColor(in); !ok || got != want {
			t.Errorf("%s: got %v %v", in, got, ok)
		}
	}
	s := mustParseSVG(t, svgDoc(`<path fill-rule="evenodd" d="M0 0h10v10H0zM2 2h6v6H2z"/>`))
	if s.shapes[0].path.Rule != FillEvenOdd || inside(s.shapes[0].path, 5, 5) && s.shapes[0].path.Rule == FillNonZero {
		t.Error("fill-rule")
	}
}

func TestSVG_DrawFitsAndUsesCurrentColor(t *testing.T) {
	s := mustParseSVG(t, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 -10 10 10"><path d="M0-10h10v10H0z" fill="currentColor" fill-opacity="0.5"/></svg>`)
	c := newCanvas()
	s.Draw(c, geom.Rect{X: 100, Y: 0, W: 40, H: 20}, geom.Color{G: 1, A: 1})
	ops := c.DisplayList().Ops
	if len(ops) != 1 {
		t.Fatalf("got %d ops", len(ops))
	}
	// Meet: 20 by 20, centered in 40 by 20.
	if got, want := ops[0].RRect.Rect, (geom.Rect{X: 110, W: 20, H: 20}); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if ops[0].Color != (geom.Color{G: 1, A: 0.5}) {
		t.Errorf("got %v", ops[0].Color)
	}
}

func TestSVG_DrawDoesNotAllocateAtAKnownSize(t *testing.T) {
	s := mustParseSVG(t, svgDoc(`<g fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="9"/><path d="M8 12h8"/></g>`))
	c := newCanvas()
	frame := func(size float32) func() {
		return func() {
			c.Reset()
			s.Draw(c, geom.Rect{W: size, H: size}, geom.Color{A: 1})
		}
	}
	for _, size := range []float32{24, 48, 16, 32} {
		frame(size)()
	}
	for _, size := range []float32{24, 48, 16, 32} {
		if got := testing.AllocsPerRun(20, frame(size)); got != 0 {
			t.Errorf("size %v: got %v allocs", size, got)
		}
	}
}

func BenchmarkSVG_DrawIcon(b *testing.B) {
	s := mustParseSVG(b, `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
  <path d="M15 21v-8a1 1 0 0 0-1-1h-4a1 1 0 0 0-1 1v8" />
  <path d="M3 10a2 2 0 0 1 .709-1.528l7-6a2 2 0 0 1 2.582 0l7 6A2 2 0 0 1 21 10v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
</svg>`)
	c := newCanvas()
	b.ReportAllocs()
	for b.Loop() {
		c.Reset()
		s.Draw(c, geom.Rect{X: 8, Y: 8, W: 24, H: 24}, geom.Color{A: 1})
	}
}

func BenchmarkStroke_Icon(b *testing.B) {
	p, err := ParseSVGPath("M3 10a2 2 0 0 1 .709-1.528l7-6a2 2 0 0 1 2.582 0l7 6A2 2 0 0 1 21 10v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		p.Stroke(StrokeStyle{Width: 2, Cap: CapRound, Join: JoinRound, Tolerance: 0.01})
	}
}

func TestSVG_DrawClipsOnlyWhatReachesOut(t *testing.T) {
	inside := mustParseSVG(t, svgDoc(`<rect x="2" y="2" width="20" height="20"/>`))
	outside := mustParseSVG(t, svgDoc(`<rect x="-4" y="2" width="20" height="20"/>`))
	for _, c := range []struct {
		s       *SVG
		clipped bool
	}{{inside, false}, {outside, true}} {
		cv := newCanvas()
		c.s.Draw(cv, geom.Rect{X: 10, Y: 10, W: 48, H: 48}, geom.Color{A: 1})
		op := cv.DisplayList().Ops[0]
		if op.Clipped != c.clipped {
			t.Errorf("clipped %v, want %v", op.Clipped, c.clipped)
		}
		if c.clipped && op.Clip != (geom.Rect{X: 10, Y: 10, W: 48, H: 48}) {
			t.Errorf("got clip %v", op.Clip)
		}
	}
}

// TestSVG_DrawOffsetRoundsAlikeOnEveryArch pins a value that FMA changes: 3 *
// (10/3) rounds to 10, so a 1 by 3 view box fills a 10 by 10 box from Y 0.
// Fused, 10 - 3*(10/3) is 1.2e-7 instead of 0, and on arm64 the icons moved
// by a rounding step and missed the goldens (ADR 0086).
func TestSVG_DrawOffsetRoundsAlikeOnEveryArch(t *testing.T) {
	s := mustParseSVG(t, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 3"><path d="M0 0h1v3H0z"/></svg>`)
	c := newCanvas()
	s.Draw(c, geom.Rect{W: 10, H: 10}, geom.Color{A: 1})
	ops := c.DisplayList().Ops
	if len(ops) != 1 {
		t.Fatalf("got %d ops", len(ops))
	}
	if got := ops[0].RRect.Rect.Y; got != 0 {
		t.Errorf("got Y %g, want 0", got)
	}
}
