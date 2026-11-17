package text

import (
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/bankai-tensei/gowaku/paint"
)

func TestShaper_OutlineText(t *testing.T) {
	s, err := NewShaper(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	var o paint.TextOutliner = s
	var glyphs []*paint.Path
	m := o.OutlineText("Hi ", paint.SVGFont{Size: 20}, func(p *paint.Path) { glyphs = append(glyphs, p) })
	if len(glyphs) != 2 {
		t.Fatalf("got %d glyphs, want 2 (a space has no outline)", len(glyphs))
	}
	// The H stands on the baseline at the origin, y down.
	b := glyphs[0].Bounds()
	if b.Y+b.H > 0.5 || b.Y > -10 || b.X < 0 || b.X > 4 {
		t.Errorf("H at %v", b)
	}
	if m.Ascent < 15 || m.Descent < 2 {
		t.Errorf("metrics %+v", m)
	}
	// The trailing space counts toward the advance.
	without := o.OutlineText("Hi", paint.SVGFont{Size: 20}, func(*paint.Path) {})
	if m.Advance <= without.Advance+3 {
		t.Errorf("advance %v with a space, %v without", m.Advance, without.Advance)
	}
	spaced := o.OutlineText("Hi", paint.SVGFont{Size: 20, LetterSpacing: 5}, func(*paint.Path) {})
	if d := spaced.Advance - without.Advance; d < 9.9 || d > 10.1 {
		t.Errorf("letter spacing added %v, want 10", d)
	}
}
