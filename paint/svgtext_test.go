package paint

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

// boxOutliner sets every character as a box of width size/2 and height size
// above the baseline, which keeps positions easy to check.
type boxOutliner struct {
	calls *[]string
}

func (b boxOutliner) OutlineText(content string, font SVGFont, glyph func(*Path)) TextMetrics {
	if b.calls != nil {
		*b.calls = append(*b.calls, content)
	}
	w := font.Size / 2
	x := float32(0)
	for _, r := range content {
		if r != ' ' {
			p := &Path{}
			p.AddRRect(geom.RRect{Rect: geom.Rect{X: x, Y: -font.Size, W: w, H: font.Size}})
			glyph(p)
		}
		x += w + font.LetterSpacing
	}
	return TextMetrics{Advance: x, Ascent: font.Size, Descent: font.Size / 4}
}

func parseWithText(t *testing.T, body string) *SVG {
	t.Helper()
	s := mustParseSVG(t, svgDoc(body))
	if !s.HasText() {
		t.Fatal("want text")
	}
	if err := s.SetText(boxOutliner{}); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestParseSVG_TextNeedsAnOutliner(t *testing.T) {
	s := mustParseSVG(t, svgDoc(`<text x="2" y="12">hi</text><rect width="2" height="2"/>`))
	if s.Skipped != 1 || len(s.shapes) != 1 || !s.HasText() {
		t.Fatalf("got %d skipped, %d shapes", s.Skipped, len(s.shapes))
	}
	if err := s.SetText(boxOutliner{}); err != nil {
		t.Fatal(err)
	}
	if s.Skipped != 0 || len(s.shapes) != 3 {
		t.Errorf("after SetText: %d skipped, %d shapes", s.Skipped, len(s.shapes))
	}
	if n := mustParseSVG(t, svgDoc(`<rect width="2" height="2"/>`)); n.HasText() {
		t.Error("no text")
	}
}

func TestParseSVG_TextLayout(t *testing.T) {
	cases := []struct {
		name, body string
		xs         []float32 // left edge of each glyph
		y          float32   // top of the first glyph
	}{
		{"start", `<text x="2" y="12" font-size="4">ab</text>`, []float32{2, 4}, 8},
		{"middle", `<text x="12" y="12" font-size="4" text-anchor="middle">abcd</text>`, []float32{8, 10, 12, 14}, 8},
		{"end", `<text x="12" y="12" font-size="4" text-anchor="end">ab</text>`, []float32{8, 10}, 8},
		{"lists", `<text x="1 5 9" y="12" font-size="4">abcd</text>`, []float32{1, 5, 9, 11}, 8},
		{"dx and tspan", `<text x="0" y="12" font-size="4">a<tspan dx="3" font-size="8">b</tspan>c</text>`, []float32{0, 5, 9}, 8},
		{"spaces collapse", "<text x=\"0\" y=\"12\" font-size=\"4\">\n  a  \t b  </text>", []float32{0, 4}, 8},
		{"space between runs", `<text x="0" y="12" font-size="4">a <tspan fill="red">b</tspan></text>`, []float32{0, 4}, 8},
		{"letter spacing", `<text x="0" y="12" font-size="4" letter-spacing="1">ab</text>`, []float32{0, 3}, 8},
		{"baseline", `<text x="0" y="12" font-size="4" dominant-baseline="text-before-edge">a</text>`, []float32{0}, 12},
		{"em", `<g font-size="4"><text x="0" y="12" font-size="2em">a</text></g>`, []float32{0}, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := parseWithText(t, c.body)
			if len(s.shapes) != len(c.xs) {
				t.Fatalf("got %d glyphs, want %d", len(s.shapes), len(c.xs))
			}
			for i, x := range c.xs {
				if b := s.shapes[i].path.Bounds(); geom.Abs32(b.X-x) > 1e-3 {
					t.Errorf("glyph %d at %v, want x %v", i, b, x)
				}
			}
			if b := s.shapes[0].path.Bounds(); geom.Abs32(b.Y-c.y) > 1e-3 {
				t.Errorf("top %v, want %v", b.Y, c.y)
			}
		})
	}
}

func TestParseSVG_TextPaintAndClip(t *testing.T) {
	s := parseWithText(t, `<linearGradient id="g"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient>
		<text x="0" y="12" font-size="4" fill="url(#g)">ab</text>`)
	// Both glyphs share the box of the whole text.
	for i := range 2 {
		g := gradientOf(t, s, i)
		if !nearOffset(g.Transform.Apply(g.Start), geom.Offset{Y: 8}) || !nearOffset(g.Transform.Apply(g.End), geom.Offset{X: 4, Y: 8}) {
			t.Errorf("glyph %d: %v to %v", i, g.Transform.Apply(g.Start), g.Transform.Apply(g.End))
		}
	}
	c := parseWithText(t, `<clipPath id="c"><text x="0" y="12" font-size="4">ab</text></clipPath><rect width="24" height="24" clip-path="url(#c)"/>`)
	if opKinds(c.ops) != "(s)" || len(c.groups[c.ops[0].index].clip.paths) != 2 {
		t.Errorf("text clip: %q", opKinds(c.ops))
	}
}

func TestSVG_SetTextOnlyOnce(t *testing.T) {
	var calls []string
	s := mustParseSVG(t, svgDoc(`<text x="0" y="12">a b</text>`))
	o := boxOutliner{calls: &calls}
	for range 3 {
		if err := s.SetText(o); err != nil {
			t.Fatal(err)
		}
	}
	if len(calls) != 1 || calls[0] != "a b" {
		t.Errorf("got calls %q", calls)
	}
}
