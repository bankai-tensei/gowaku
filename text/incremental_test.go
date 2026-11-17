package text

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
)

// sameLayout reports how got differs from want, or "".
func sameLayout(got, want *Layout) string {
	if got.LineCount() != want.LineCount() {
		return fmt.Sprintf("%d lines, want %d", got.LineCount(), want.LineCount())
	}
	for i := range want.LineCount() {
		ga, gb := got.LineRange(i)
		wa, wb := want.LineRange(i)
		if ga != wa || gb != wb {
			return fmt.Sprintf("line %d is %d-%d, want %d-%d", i, ga, gb, wa, wb)
		}
		g, w := got.lines[i], want.lines[i]
		g.spanStart, g.spanEnd, w.spanStart, w.spanEnd = 0, 0, 0, 0
		// Tops add up in another order after an edit.
		if near(g.top, w.top) {
			g.top = w.top
		}
		if g != w {
			return fmt.Sprintf("line %d: %+v, want %+v", i, g, w)
		}
	}
	if gs, ws := got.Size(), want.Size(); !near(gs.W, ws.W) || !near(gs.H, ws.H) {
		return fmt.Sprintf("size %v, want %v", got.Size(), want.Size())
	}
	if !slices.Equal(got.xs, want.xs) || !slices.Equal(got.adv, want.adv) {
		return "positions differ"
	}
	gr, wr := got.Runs(), want.Runs()
	if len(gr) != len(wr) {
		return fmt.Sprintf("%d runs, want %d", len(gr), len(wr))
	}
	for i := range wr {
		if !near(gr[i].Origin.X, wr[i].Origin.X) || !near(gr[i].Origin.Y, wr[i].Origin.Y) || gr[i].Face != wr[i].Face || gr[i].Bounds != wr[i].Bounds || !slices.Equal(gr[i].Glyphs, wr[i].Glyphs) {
			return fmt.Sprintf("run %d differs", i)
		}
	}
	return ""
}

func near(a, b float32) bool { return a-b < 1e-3 && b-a < 1e-3 }

func TestReshape_MatchesAFreshLayout(t *testing.T) {
	for _, shaper := range []struct {
		name string
		s    func(testing.TB) *Shaper
		text string
	}{
		{"go", goShaper, "lines break between words\nand blocks\n\nend with a feed\n"},
		{"system", systemShaper, "親譲りの無鉄砲で小供の時から損ばかりしている。\nTypesetting, 2026.\n\nمرحبا بالعالم 123\n小学校に居る時分。"},
	} {
		for _, st := range []Style{
			{Size: 16, Hyphenate: true},
			{Size: 16, Orientation: Vertical},
			{Size: 16, Align: AlignCenter, Indent: 16},
		} {
			t.Run(shaper.name+"/"+st.Orientation.String(), func(t *testing.T) {
				s := shaper.s(t)
				content := shaper.text
				p := s.Shape(content, st)
				l := p.Layout(120)
				rng := rand.New(rand.NewPCG(1, 2))
				pieces := []string{"x", "あ", "\n", "word ", "。", "ب", ""}
				for step := range 300 {
					// Replace a random stretch with a random piece.
					runes := []rune(content)
					a := rng.IntN(len(runes) + 1)
					b := min(a+rng.IntN(3), len(runes))
					edited := string(runes[:a]) + pieces[rng.IntN(len(pieces))] + string(runes[b:])
					p = s.Reshape(p, edited, st)
					l = p.Relayout(l, 120)
					content = edited
					want := s.Shape(content, st).Layout(120)
					if diff := sameLayout(l, want); diff != "" {
						t.Fatalf("step %d, %q: %s", step, content, diff)
					}
				}
			})
		}
	}
}

func TestReshape_FallsBack(t *testing.T) {
	s := goShaper(t)
	p := s.Shape("abc", Style{Size: 10})
	if q := s.Reshape(p, "abd", Style{Size: 12}); q == p {
		t.Error("a new style must shape anew")
	}
	l := p.Layout(100)
	if l2 := p.Relayout(l, 50); l2 == l {
		t.Error("a new length must lay out anew")
	}
}
