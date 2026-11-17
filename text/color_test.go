package text

import (
	"os"
	"testing"

	"github.com/go-text/typesetting/font/opentype/tables"
)

// colorful reports whether the color glyph img has pixels of more than one hue,
// which a glyph drawn in one color does not.
func colorful(a *Atlas, img GlyphImage) bool {
	_, pix := a.TakeColorDirty(img.Page)
	var first [3]uint8
	seen := false
	for y := range img.Height {
		for x := range img.Width {
			o := ((img.Y+y)*a.size + img.X + x) * 4
			if pix[o+3] < 200 {
				continue
			}
			c := [3]uint8{pix[o], pix[o+1], pix[o+2]}
			if !seen {
				first, seen = c, true
			} else if c != first {
				return true
			}
		}
	}
	return false
}

func emojiGlyph(t *testing.T, s *Shaper, emoji string, size float32) GlyphImage {
	t.Helper()
	p := s.Shape(emoji, Style{Size: size})
	if len(p.clusters) != 1 {
		t.Fatalf("%s: %d clusters", emoji, len(p.clusters))
	}
	c := p.clusters[0]
	a := NewAtlas(s)
	img := a.Glyph(GlyphKey{Face: c.face, ID: p.glyphs[c.glyphStart].id, Size: size})
	if !img.Color {
		t.Fatalf("%s did not come out in color: %+v", emoji, img)
	}
	if !colorful(a, img) {
		t.Errorf("%s has a single color", emoji)
	}
	if img.Width < int(size)/2 || img.Width > int(size)*2 {
		t.Errorf("%s is %d wide at size %v", emoji, img.Width, size)
	}
	return img
}

func TestColorEmoji_SegoeCOLR(t *testing.T) {
	s := systemShaper(t)
	if _, ok := s.fonts.FindSystemFont("Segoe UI Emoji"); !ok {
		t.Skip("no Segoe UI Emoji")
	}
	for _, e := range []string{"😀", "👍🏽", "❤️", "🍣"} {
		emojiGlyph(t, s, e, 48)
	}
}

// TestColorEmoji_NotoCBDT reads the font named by GOWAKU_CBDT_FONT, such as
// NotoColorEmoji.ttf, whose glyphs are PNG bitmaps.
func TestColorEmoji_NotoCBDT(t *testing.T) {
	path := os.Getenv("GOWAKU_CBDT_FONT")
	if path == "" {
		t.Skip("set GOWAKU_CBDT_FONT to a CBDT font")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewShaper(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []float32{16, 48, 160} {
		img := emojiGlyph(t, s, "😀", size)
		// The bitmap sits on the baseline, about an em tall.
		if img.Top > 0 || img.Top < -int(size) {
			t.Errorf("size %v: top %d", size, img.Top)
		}
	}
}

func TestEmojiPresentation(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"😀", true},
		{"☀", false},
		{"☀️", true},
		{"😀︎", false},
		{"1️⃣", true},
		{"🇯🇵", true},
		{"a", false},
		{"漢", false},
	}
	for _, tt := range tests {
		if got := isEmojiCluster([]rune(tt.s)); got != tt.want {
			t.Errorf("%q: got %v", tt.s, got)
		}
	}
}

func TestCOLR_Gradients(t *testing.T) {
	// A linear gradient from x=0 to x=100, turned by nothing.
	lin := linearParam(0, 0, 100, 0, 0, 100)
	for _, tt := range []struct{ x, want float32 }{{0, 0}, {50, 0.5}, {100, 1}, {150, 1.5}} {
		if got, _ := lin(tt.x, 37); got != tt.want {
			t.Errorf("linear at %v: %v, want %v", tt.x, got, tt.want)
		}
	}
	// Concentric circles of radius 0 to 100.
	rad := radialParam(0, 0, 0, 0, 0, 100)
	if got, ok := rad(30, 40); !ok || got < 0.499 || got > 0.501 {
		t.Errorf("radial at distance 50: %v %v", got, ok)
	}
	// A sweep from 0° to 360°: straight up is a quarter.
	sw := sweepParam(0, 0, tables.Fixed214(-1<<14), tables.Fixed214(1<<14))
	if got, _ := sw(0, 10); got < 0.249 || got > 0.251 {
		t.Errorf("sweep at 90°: %v", got)
	}
	line := colorLine{extend: tables.ExtendReflect, stops: []colorStop{{0, rgba{0, 0, 0, 1}}, {1, rgba{1, 1, 1, 1}}}}
	if got := line.at(1.25); got[0] < 0.749 || got[0] > 0.751 {
		t.Errorf("reflect at 1.25: %v", got)
	}
	line.extend = tables.ExtendRepeat
	if got := line.at(1.25); got[0] < 0.249 || got[0] > 0.251 {
		t.Errorf("repeat at 1.25: %v", got)
	}
}

func TestCOLR_Compose(t *testing.T) {
	red, half := rgba{1, 0, 0, 1}, rgba{0, 0, 0.5, 0.5}
	if got := compose(tables.CompositeSrcOver, half, red); got != (rgba{0.5, 0, 0.5, 1}) {
		t.Errorf("src over: %v", got)
	}
	if got := compose(tables.CompositeDestOut, half, red); got != (rgba{0.5, 0, 0, 0.5}) {
		t.Errorf("dest out: %v", got)
	}
	if got := compose(tables.CompositeMultiply, rgba{0.5, 0.5, 0.5, 1}, rgba{1, 0.5, 0, 1}); got != (rgba{0.5, 0.25, 0, 1}) {
		t.Errorf("multiply: %v", got)
	}
}
