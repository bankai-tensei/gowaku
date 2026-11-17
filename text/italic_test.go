package text

import (
	"image"
	"testing"

	"github.com/go-text/typesetting/font"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/bankai-tensei/gowaku/geom"
)

// drawLayout draws the glyphs of l onto a coverage image of size w by h, at
// one physical pixel per logical pixel, the way render places them. Color
// glyphs count as full coverage over their box.
func drawLayout(a *Atlas, l *Layout, w, h int) *image.Alpha {
	dst := image.NewAlpha(image.Rect(0, 0, w, h))
	a.BeginFrame()
	for _, run := range l.Runs() {
		for _, gl := range run.Glyphs {
			x := run.Origin.X + gl.X
			pixel := geom.Floor32(x)
			step := uint8((x - pixel) * SubpixelSteps)
			y := geom.Floor32(run.Origin.Y + gl.Y + 0.5)
			img := a.Glyph(GlyphKey{Face: run.Face, ID: gl.ID, Size: run.Size, Subpixel: step, Sideways: run.Sideways})
			for r := range img.Height {
				for c := range img.Width {
					v := byte(255)
					if !img.Color {
						v = a.gray.pages[img.Page].pix[(img.Y+r)*a.size+img.X+c]
					}
					px, py := int(pixel)+img.Left+c, int(y)+img.Top+r
					if image.Pt(px, py).In(dst.Rect) {
						dst.Pix[dst.PixOffset(px, py)] = max(dst.Pix[dst.PixOffset(px, py)], v)
					}
				}
			}
		}
	}
	return dst
}

// inkReach returns the rightmost inked column of the rows from y0 to y1, or -1.
func inkReach(img *image.Alpha, y0, y1 int) int {
	right := -1
	for y := y0; y < y1; y++ {
		for x := img.Rect.Dx() - 1; x > right; x-- {
			if img.Pix[img.PixOffset(x, y)] > 64 {
				right = x
				break
			}
		}
	}
	return right
}

// inkRows returns the first and last rows with ink, plus one.
func inkRows(img *image.Alpha) (top, bottom int) {
	top, bottom = -1, -1
	for y := range img.Rect.Dy() {
		for x := range img.Rect.Dx() {
			if img.Pix[img.PixOffset(x, y)] > 64 {
				if top < 0 {
					top = y
				}
				bottom = y + 1
				break
			}
		}
	}
	return top, bottom
}

func samePixels(a, b *image.Alpha) bool { return string(a.Pix) == string(b.Pix) }

// leanOf is how many pixels further right the ink of the top quarter of img
// reaches than that of its bottom quarter.
func leanOf(t *testing.T, img *image.Alpha) int {
	t.Helper()
	top, bottom := inkRows(img)
	if top < 0 {
		t.Fatal("no ink")
	}
	q := max((bottom-top)/4, 1)
	return inkReach(img, top, top+q) - inkReach(img, bottom-q, bottom)
}

func TestSyntheticItalic_SlantsAFaceWithoutItalic(t *testing.T) {
	s := goShaper(t)
	const content = "Hill"
	plain := s.Shape(content, Style{Size: 40}).Layout(inf)
	italic := s.Shape(content, Style{Size: 40, Italic: true}).Layout(inf)
	for _, cl := range italic.paragraph.clusters {
		if !s.slanted[cl.face] {
			t.Fatalf("cluster %v is not slanted", cl)
		}
	}
	if plain.Size() != italic.Size() {
		t.Errorf("size %v, upright %v: advances must not change", italic.Size(), plain.Size())
	}
	for _, off := range []int{0, 2, len(content)} {
		if p, i := plain.CaretRect(off), italic.CaretRect(off); p != i {
			t.Errorf("caret at %d: %v, upright %v", off, i, p)
		}
	}
	if p, i := plain.SelectionRects(1, 3, nil), italic.SelectionRects(1, 3, nil); len(p) != 1 || len(i) != 1 || p[0] != i[0] {
		t.Errorf("selection %v, upright %v", i, p)
	}

	a := NewAtlas(s)
	up, slant := drawLayout(a, plain, 160, 60), drawLayout(a, italic, 160, 60)
	if samePixels(up, slant) {
		t.Fatal("italic draws the same pixels as upright")
	}
	// H is about 29 pixels tall here; three quarters of its height lean by
	// about 0.2126 × 22 ≈ 4.6 pixels.
	if l := leanOf(t, up); l > 1 {
		t.Errorf("upright leans by %d pixels", l)
	}
	if l := leanOf(t, slant); l < 3 {
		t.Errorf("italic leans by %d pixels, want about 5", l)
	}
	// The baseline stays put: the bottom rows of ink start where the
	// upright ones do.
	upTop, upBottom := inkRows(up)
	slTop, slBottom := inkRows(slant)
	if upTop != slTop || upBottom != slBottom {
		t.Errorf("ink rows %d-%d, upright %d-%d", slTop, slBottom, upTop, upBottom)
	}
}

func TestSyntheticItalic_KeepsARealItalic(t *testing.T) {
	s, err := NewShaper(goregular.TTF, goitalic.TTF)
	if err != nil {
		t.Fatal(err)
	}
	p := s.Shape("Hill", Style{Size: 16, Italic: true})
	for _, cl := range p.clusters {
		face := s.faces[cl.face]
		if s.slanted[cl.face] || face.Describe().Aspect.Style != font.StyleItalic {
			t.Fatalf("cluster %v: slanted %v, style %v", cl, s.slanted[cl.face], face.Describe().Aspect.Style)
		}
		if id := s.faceID(face); id != cl.face {
			t.Errorf("italic face takes ID %d, but its own ID is %d", cl.face, id)
		}
	}
	for _, cl := range s.Shape("Hill", Style{Size: 16}).clusters {
		if s.slanted[cl.face] || s.faces[cl.face].Describe().Aspect.Style == font.StyleItalic {
			t.Fatalf("upright cluster %v drawn in an italic", cl)
		}
	}
}

// TestSyntheticItalic_SkipsVerticalLines checks that columns stay upright,
// sideways Latin included.
func TestSyntheticItalic_SkipsVerticalLines(t *testing.T) {
	s := goShaper(t)
	p := s.Shape("Hill", Style{Size: 16, Italic: true, Orientation: Vertical})
	for _, cl := range p.clusters {
		if s.slanted[cl.face] {
			t.Fatalf("cluster %v of a vertical line is slanted", cl)
		}
	}
}

func TestSyntheticItalic_RubyFollowsTheLean(t *testing.T) {
	s := goShaper(t)
	shape := func(italic bool) *Layout {
		return s.ShapeSpans([]Span{{Text: "word", Ruby: "ruby", Style: Style{Italic: italic}}}, Style{Size: 20}).Layout(inf)
	}
	plain, italic := shape(false), shape(true)
	p, i := plain.runs[len(plain.runs)-1], italic.runs[len(italic.runs)-1]
	if want := p.Origin.X + obliqueShear*rubyLift*20; geom.Abs32(i.Origin.X-want) > 1e-4 || i.Origin.Y != p.Origin.Y {
		t.Errorf("ruby at %v, want x %v y %v", i.Origin, want, p.Origin.Y)
	}
}

func TestSyntheticItalic_PaintDoesNotAllocate(t *testing.T) {
	s := goShaper(t)
	l := s.Shape("Hill side", Style{Size: 16, Italic: true}).Layout(inf)
	a := NewAtlas(s)
	drawLayout(a, l, 100, 30)
	runs := l.Runs()
	frame := func() {
		a.BeginFrame()
		for _, run := range runs {
			for _, gl := range run.Glyphs {
				a.Glyph(GlyphKey{Face: run.Face, ID: gl.ID, Size: run.Size})
			}
		}
	}
	frame()
	if got := testing.AllocsPerRun(100, frame); got != 0 {
		t.Errorf("got %v allocs", got)
	}
}

// TestSyntheticItalic_MixedJapanese checks that kana, kanji and Latin of one
// italic span all lean, with the fallback faces of the system.
func TestSyntheticItalic_MixedJapanese(t *testing.T) {
	s := systemShaper(t)
	const content = "日本語 abc"
	plain := s.Shape(content, Style{Size: 40}).Layout(inf)
	italic := s.Shape(content, Style{Size: 40, Italic: true}).Layout(inf)
	for _, cl := range italic.paragraph.clusters {
		if content[cl.start:cl.end] != " " && !s.slanted[cl.face] {
			t.Errorf("%q is not slanted", content[cl.start:cl.end])
		}
	}
	if plain.Size() != italic.Size() {
		t.Errorf("size %v, upright %v", italic.Size(), plain.Size())
	}
	a := NewAtlas(s)
	w := int(plain.Size().W) + 40
	up, slant := drawLayout(a, plain, w, 70), drawLayout(a, italic, w, 70)
	if samePixels(up, slant) {
		t.Fatal("italic draws the same pixels as upright")
	}
	if lu, ls := leanOf(t, up), leanOf(t, slant); ls < lu+3 {
		t.Errorf("italic leans by %d pixels, upright by %d", ls, lu)
	}
}

// BenchmarkShape_ShortJapaneseItalic is BenchmarkShape_ShortJapanese in italic,
// every face of it synthetic.
func BenchmarkShape_ShortJapaneseItalic(b *testing.B) {
	s := systemShaper(b)
	b.ReportAllocs()
	for b.Loop() {
		s.Shape("項目の説明文 Item 123", Style{Size: 16, Italic: true}).Layout(300)
	}
}

// BenchmarkAtlas_RasterizeOblique is BenchmarkAtlas_Rasterize for a glyph of a
// synthetic oblique.
func BenchmarkAtlas_RasterizeOblique(b *testing.B) {
	s := systemShaper(b)
	p := s.Shape("語", Style{Size: 16, Italic: true})
	face, id := p.clusters[0].face, p.glyphs[0].id
	a := NewAtlas(s)
	a.BeginFrame()
	size := float32(16)
	b.ReportAllocs()
	for b.Loop() {
		size += 0.001 // a new key each time
		a.Glyph(GlyphKey{Face: face, ID: id, Size: size})
	}
}

// TestSyntheticItalic_SkipsEmojiAndRightToLeft checks that color emoji and
// right-to-left scripts stay upright in an italic span, while the Latin
// around them leans.
func TestSyntheticItalic_SkipsEmojiAndRightToLeft(t *testing.T) {
	s := systemShaper(t)
	tests := []struct {
		content, upright, slanted string
	}{
		{"a😀b", "😀", "a"},
		{"a שלום b", "ש", "a"},
		{"a مرحبا b", "م", "a"},
	}
	for _, tt := range tests {
		p := s.Shape(tt.content, Style{Size: 16, Italic: true})
		for _, cl := range p.clusters {
			got := tt.content[cl.start:cl.end]
			switch {
			case got == tt.upright && s.slanted[cl.face]:
				t.Errorf("%q: %q is slanted", tt.content, got)
			case got == tt.slanted && !s.slanted[cl.face]:
				t.Errorf("%q: %q is not slanted", tt.content, got)
			}
		}
	}
}
