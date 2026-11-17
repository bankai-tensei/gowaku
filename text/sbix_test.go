package text

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"os"
	"testing"

	"github.com/go-text/typesetting/font"
)

// appleColorEmoji is the sbix font of macOS, whose glyphs are PNG bitmaps in
// strikes of 20 to 160 pixels per em.
const appleColorEmoji = "/System/Library/Fonts/Apple Color Emoji.ttc"

func appleEmojiShaper(t *testing.T) *Shaper {
	t.Helper()
	data, err := os.ReadFile(appleColorEmoji)
	if err != nil {
		t.Skip(err)
	}
	s, err := NewShaper(data)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestColorEmoji_AppleSbix(t *testing.T) {
	s := appleEmojiShaper(t)
	tests := []struct {
		emoji string
		size  float32
	}{
		{"😀", 12},  // below the smallest strike
		{"😀", 16},  // scaled down from 20
		{"😀", 48},  // a strike of its own
		{"😀", 100}, // between strikes
		{"😀", 300}, // above the largest strike
		{"🇯🇵", 48},
		{"👍🏽", 48},
		{"❤️", 32},
		{"🍣", 26},
	}
	for _, tt := range tests {
		img := emojiGlyph(t, s, tt.emoji, tt.size)
		// Apple's bitmaps are an em square standing on the baseline.
		em := int(tt.size + 0.5)
		if img.Left != 0 || img.Width < em-1 || img.Width > em+1 || img.Height < em-1 || img.Height > em+1 {
			t.Errorf("%s at %v: %dx%d at left %d, want an em of %d", tt.emoji, tt.size, img.Width, img.Height, img.Left, em)
		}
		if img.Top+img.Height < -1 || img.Top+img.Height > 1 {
			t.Errorf("%s at %v: bottom at %d, want the baseline", tt.emoji, tt.size, img.Top+img.Height)
		}
	}
}

// TestColorEmoji_AppleSbixExact checks that a size with a strike of its own
// copies the strike pixel for pixel, and that a subpixel position moves the
// bitmap without stretching it.
func TestColorEmoji_AppleSbixExact(t *testing.T) {
	s := appleEmojiShaper(t)
	p := s.Shape("😀", Style{Size: 48})
	c := p.clusters[0]
	gid := p.glyphs[c.glyphStart].id
	face := s.faces[c.face]
	face.SetPpem(48, 48)
	bm, ok := face.GlyphDataBitmap(font.GID(gid))
	if !ok {
		t.Fatal("no bitmap at 48 ppem")
	}
	src, err := png.Decode(bytes.NewReader(bm.Data))
	if err != nil {
		t.Fatal(err)
	}
	want := image.NewRGBA(src.Bounds())
	draw.Draw(want, want.Bounds(), src, src.Bounds().Min, draw.Src)
	wantSum := 0
	for i := 3; i < len(want.Pix); i += 4 {
		wantSum += int(want.Pix[i])
	}

	a := NewAtlas(s)
	tests := []struct {
		subpixel uint8
		maxWidth int
	}{{0, 48}, {1, 49}, {2, 49}, {3, 49}}
	for _, tt := range tests {
		img := a.Glyph(GlyphKey{Face: c.face, ID: gid, Size: 48, Subpixel: tt.subpixel})
		if !img.Color || img.Width < 48 || img.Width > tt.maxWidth || img.Height != 48 {
			t.Fatalf("subpixel %d: %+v", tt.subpixel, img)
		}
		_, pix := a.TakeColorDirty(img.Page)
		// Moving keeps the total coverage, which stretching by a pixel would
		// raise by a forty-eighth.
		sum := 0
		for y := range img.Height {
			for x := range img.Width {
				sum += int(pix[((img.Y+y)*a.size+img.X+x)*4+3])
			}
		}
		if d := float64(sum)/float64(wantSum) - 1; d < -0.005 || d > 0.005 {
			t.Errorf("subpixel %d: coverage %d, want %d", tt.subpixel, sum, wantSum)
		}
		if tt.subpixel != 0 {
			continue
		}
		for y := range 48 {
			got := pix[((img.Y+y)*a.size+img.X)*4:][:48*4]
			if !bytes.Equal(got, want.Pix[y*want.Stride:][:48*4]) {
				t.Fatalf("row %d differs from the strike", y)
			}
		}
	}
}

// TestColorEmoji_AppleFallback checks that the system fonts give emoji to
// Apple Color Emoji, next to Japanese text.
func TestColorEmoji_AppleFallback(t *testing.T) {
	if _, err := os.Stat(appleColorEmoji); err != nil {
		t.Skip(err)
	}
	s, err := NewSystemShaper()
	if err != nil {
		t.Skip(err)
	}
	if _, ok := s.fonts.FindSystemFont("Apple Color Emoji"); !ok {
		t.Skip("Apple Color Emoji is not indexed")
	}
	tests := []struct {
		content string
		cluster int
	}{
		{"😀", 0},
		{"あ😀", 1},
		{"寿司🍣です", 2},
		{"☀️", 0},
		{"🇯🇵", 0},
	}
	for _, tt := range tests {
		p := s.Shape(tt.content, Style{Size: 32})
		c := p.clusters[tt.cluster]
		if got := s.faces[c.face].Describe().Family; got != "Apple Color Emoji" {
			t.Errorf("%q: cluster %d in %q", tt.content, tt.cluster, got)
			continue
		}
		a := NewAtlas(s)
		img := a.Glyph(GlyphKey{Face: c.face, ID: p.glyphs[c.glyphStart].id, Size: 32})
		if !img.Color || !colorful(a, img) {
			t.Errorf("%q: not a color glyph: %+v", tt.content, img)
		}
	}
}
