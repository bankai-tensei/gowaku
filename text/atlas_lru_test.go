package text

import (
	"image"
	"math/rand/v2"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// fakeGlyph is Glyph for an image of w by h blank texels, which spares the
// tests of the packing a font.
func fakeGlyph(a *Atlas, k GlyphKey, w, h int) GlyphImage {
	if i, ok := a.entries[k]; ok {
		a.touch(i)
		return a.items[i].img
	}
	a.clearScratch(w, h)
	img, shelf := a.store(GlyphImage{Width: w, Height: h})
	i := a.newItem(img, shelf)
	a.items[i].kind = itemGlyph
	a.items[i].glyph = k
	a.entries[k] = i
	return img
}

func fakeKey(n int) GlyphKey { return GlyphKey{ID: uint32(n), Size: 16} }

func has(a *Atlas, k GlyphKey) bool {
	_, ok := a.entries[k]
	return ok
}

// checkAtlas fails if two images overlap, an image leaves its page, or a key
// maps to an item of another key.
func checkAtlas(t *testing.T, a *Atlas) {
	t.Helper()
	for k, i := range a.entries {
		if i == noItem {
			continue
		}
		if it := &a.items[i]; it.kind != itemGlyph || it.glyph != k {
			t.Fatalf("key %v maps to item %d of kind %d for %v", k, i, it.kind, it.glyph)
		}
	}
	for _, set := range []*pageSet{&a.gray, &a.color} {
		byPage := make([][]image.Rectangle, len(set.pages))
		n := 0
		for i := set.head; i != noItem; i = a.items[i].next {
			img := a.items[i].img
			r := image.Rect(img.X, img.Y, img.X+img.Width, img.Y+img.Height)
			if !r.In(image.Rect(0, 0, a.size, a.size)) {
				t.Fatalf("image %v leaves the page", r)
			}
			for _, o := range byPage[img.Page] {
				if o.Overlaps(r) {
					t.Fatalf("images %v and %v overlap on page %d", o, r, img.Page)
				}
			}
			byPage[img.Page] = append(byPage[img.Page], r)
			n++
		}
		live := 0
		for _, p := range set.pages {
			for _, s := range p.shelves {
				if !s.dead {
					live += s.live
				}
			}
		}
		if live != n {
			t.Fatalf("the shelves count %d images, the list %d", live, n)
		}
	}
}

func TestAtlas_EvictsTheLeastRecentlyUsedGlyph(t *testing.T) {
	a := newAtlas(nil, 64, 1)
	// Sixteen 16 by 16 images fill the page.
	for n := range 16 {
		a.BeginFrame()
		fakeGlyph(a, fakeKey(n), 16, 16)
	}
	a.BeginFrame()
	fakeGlyph(a, fakeKey(0), 16, 16) // the oldest, now the most recent
	fakeGlyph(a, fakeKey(16), 16, 16)
	if !has(a, fakeKey(0)) || has(a, fakeKey(1)) {
		t.Fatal("the glyph used least recently must go, and no other")
	}
	for n := 2; n <= 16; n++ {
		if !has(a, fakeKey(n)) {
			t.Errorf("glyph %d is gone", n)
		}
	}
	if a.PageCount() != 1 {
		t.Errorf("got %d pages", a.PageCount())
	}
	checkAtlas(t, a)
}

func TestAtlas_NeverEvictsAGlyphOfTheFrame(t *testing.T) {
	a := newAtlas(nil, 64, 1)
	a.BeginFrame()
	imgs := make([]GlyphImage, 20)
	for n := range imgs {
		imgs[n] = fakeGlyph(a, fakeKey(n), 16, 16)
	}
	for n, img := range imgs {
		if got := fakeGlyph(a, fakeKey(n), 16, 16); got != img {
			t.Errorf("glyph %d moved from %+v to %+v", n, img, got)
		}
	}
	if a.PageCount() != 2 {
		t.Errorf("got %d pages", a.PageCount())
	}
	checkAtlas(t, a)
}

func TestAtlas_KeepsToTheLimitWhileTheFrameAddsGlyphs(t *testing.T) {
	// The page that takes the new glyphs of a frame is in use by that frame,
	// which must not stop older glyphs on it from making room.
	a := newAtlas(nil, 64, 1)
	for n := range 200 {
		a.BeginFrame()
		fakeGlyph(a, fakeKey(n), 16, 16)
		fakeGlyph(a, fakeKey(1000+n), 16, 16)
	}
	if a.PageCount() != 1 {
		t.Errorf("got %d pages", a.PageCount())
	}
	checkAtlas(t, a)
}

func TestAtlas_ATallGlyphTakesTheOldestShelves(t *testing.T) {
	a := newAtlas(nil, 64, 1)
	for n := range 32 {
		a.BeginFrame()
		fakeGlyph(a, fakeKey(n), 8, 16) // four shelves of eight
	}
	a.BeginFrame()
	for n := 8; n < 32; n++ {
		fakeGlyph(a, fakeKey(n), 8, 16)
	}
	// No shelf suits 30 texels, so the two used least recently go together.
	a.BeginFrame()
	for n := 16; n < 32; n++ {
		fakeGlyph(a, fakeKey(n), 8, 16)
	}
	img := fakeGlyph(a, fakeKey(100), 20, 30)
	if a.PageCount() != 1 || img.Height != 30 {
		t.Fatalf("got %d pages, %+v", a.PageCount(), img)
	}
	for n := 16; n < 32; n++ {
		if !has(a, fakeKey(n)) {
			t.Errorf("glyph %d of the frame is gone", n)
		}
	}
	checkAtlas(t, a)
}

func TestAtlas_ReusesTheRoomOfEvictedGlyphs(t *testing.T) {
	a := newAtlas(nil, 128, 3)
	rng := rand.New(rand.NewPCG(1, 2))
	for frame := range 3000 {
		a.BeginFrame()
		// A hot set every frame, and a few of a large set at random sizes.
		for n := range 20 {
			fakeGlyph(a, fakeKey(n), 10, 12)
		}
		for range 5 {
			n := 100 + rng.IntN(2000)
			w, h := 4+n%13, 6+n%11
			if n%23 == 0 {
				// Now and then one far taller, that only a band of whole
				// shelves makes room for.
				w, h = 20+n%30, 30+n%40
			}
			fakeGlyph(a, fakeKey(n), w, h)
		}
		if frame%100 == 0 {
			checkAtlas(t, a)
		}
	}
	if a.PageCount() != 3 {
		t.Errorf("got %d pages", a.PageCount())
	}
	for n := range 20 {
		if !has(a, fakeKey(n)) {
			t.Errorf("hot glyph %d is gone", n)
		}
	}
	checkAtlas(t, a)
}

func TestAtlas_EvictionDoesNotAllocate(t *testing.T) {
	a := newAtlas(nil, 64, 1)
	n := 0
	frame := func() {
		a.BeginFrame()
		fakeGlyph(a, fakeKey(n), 16, 16)
		n++
	}
	for range 100 {
		frame()
	}
	if got := testing.AllocsPerRun(1000, frame); got != 0 {
		t.Errorf("got %v allocs", got)
	}
}

func TestAtlas_EvictedGlyphComesBack(t *testing.T) {
	s := goShaper(t)
	a := newAtlas(s, 64, 1)
	id := glyphID(t, s, "M")
	a.BeginFrame()
	big := GlyphKey{ID: id, Size: 50}
	first := a.Glyph(big)
	a.BeginFrame()
	a.Glyph(GlyphKey{ID: id, Size: 49}) // takes the place of the first
	if has(a, big) {
		t.Fatal("the older glyph must make room")
	}
	a.BeginFrame()
	before := a.rasters
	if got := a.Glyph(big); got.Width != first.Width || got.Height != first.Height {
		t.Errorf("got %+v, want the size of %+v", got, first)
	}
	if a.rasters != before+1 {
		t.Errorf("drew %d images", a.rasters-before)
	}
	checkAtlas(t, a)
}

func TestAtlas_EvictsShapes(t *testing.T) {
	a := newAtlas(goShaper(t), 64, 1)
	square := func(side float32) ShapeKey {
		p := &paint.Path{}
		p.AddRRect(geom.RRect{Rect: geom.Rect{W: side, H: side}})
		return ShapeKey{Path: p, Scale: 1}
	}
	a.BeginFrame()
	a.Shape(square(40))
	a.BeginFrame()
	a.Shape(square(41)) // takes the place of the first
	if len(a.shapes) != 1 || a.PageCount() != 1 {
		t.Fatalf("got %d shapes on %d pages", len(a.shapes), a.PageCount())
	}
	a.BeginFrame()
	before := a.rasters
	if got := a.Shape(square(40)); got.Width != 41 {
		t.Errorf("got %+v", got)
	}
	if a.rasters != before+1 {
		t.Errorf("drew %d images", a.rasters-before)
	}
}
