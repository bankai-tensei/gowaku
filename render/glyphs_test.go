package render

import (
	"errors"
	"image"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
)

type fakeTexture struct {
	id        int
	destroyed *int
}

func (f fakeTexture) Destroy() { *f.destroyed++ }

type fakeStore struct {
	created, destroyed   int
	updates              []image.Rectangle
	targets, targetDraws int
	formats              []gpu.TextureFormat
	// prepared counts PrepareShader, which fails if refuse is set.
	prepared int
	refuse   bool
}

func (s *fakeStore) CreateTarget(int, int) (gpu.Texture, error) {
	s.targets++
	return fakeTexture{s.created, &s.destroyed}, nil
}

func (s *fakeStore) PrepareShader(*gpu.Shader) error {
	s.prepared++
	if s.refuse {
		return errors.New("refused")
	}
	return nil
}

func (s *fakeStore) DrawToTarget(gpu.Texture, *gpu.Batch, float32) error {
	s.targetDraws++
	return nil
}

func (s *fakeStore) CreateTexture(_, _ int, format gpu.TextureFormat) (gpu.Texture, error) {
	s.created++
	s.formats = append(s.formats, format)
	return fakeTexture{s.created, &s.destroyed}, nil
}

func (s *fakeStore) UpdateTexture(_ gpu.Texture, r image.Rectangle, _ []byte, _ int) error {
	s.updates = append(s.updates, r)
	return nil
}

type fixture struct {
	store  *fakeStore
	glyphs *Resources
	layout *text.Layout
	canvas paint.Canvas
	batch  gpu.Batch
}

func newFixture(t testing.TB, content string) *fixture {
	t.Helper()
	s := testutil.NewShaper(t)
	f := &fixture{store: &fakeStore{}}
	f.glyphs = NewResources(text.NewAtlas(s), f.store)
	f.layout = s.Shape(content, text.Style{Size: 16}).Layout(1000)
	return f
}

func (f *fixture) paint(off geom.Offset) *paint.DisplayList {
	f.canvas.Reset()
	runs := f.layout.Runs()
	for i := range runs {
		f.canvas.DrawGlyphs(&runs[i], off, geom.Hex(0x102030))
	}
	return f.canvas.DisplayList()
}

func TestEncode_GlyphsBecomeInstances(t *testing.T) {
	f := newFixture(t, "ab c")
	Encode(f.paint(geom.Offset{X: 10, Y: 20}), f.glyphs, 1, &f.batch)
	if err := f.glyphs.Finish(&f.batch); err != nil {
		t.Fatal(err)
	}
	if got := len(f.batch.Instances); got != 3 {
		t.Fatalf("got %d instances; the space has no ink and must be skipped", got)
	}
	if len(f.batch.Draws) != 1 || f.batch.Draws[0].Count != 3 {
		t.Fatalf("got draws %+v", f.batch.Draws)
	}
	if _, ok := f.batch.Draws[0].Texture.(fakeTexture); !ok {
		t.Errorf("got texture %T, want the page's texture", f.batch.Draws[0].Texture)
	}
	for _, in := range f.batch.Instances {
		if in.Kind != gpu.InstanceGlyph || in.Color != geom.Hex(0x102030) {
			t.Errorf("got %+v", in)
		}
		if in.Rect.X < 10 || in.Rect.Y < 20 || in.Rect.W == 0 {
			t.Errorf("got rect %+v", in.Rect)
		}
	}
}

func TestEncode_GlyphsSitOnPhysicalPixels(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		f := newFixture(t, "abcdef")
		Encode(f.paint(geom.Offset{X: 3.3, Y: 7.7}), f.glyphs, scale, &f.batch)
		for _, in := range f.batch.Instances {
			for _, v := range []float32{in.Rect.X * scale, in.Rect.Y * scale} {
				if d := v - geom.Floor32(v+0.5); d > 1e-3 || d < -1e-3 {
					t.Errorf("scale %v: %v is off the pixel grid", scale, v)
				}
			}
		}
	}
}

func TestEncode_LogicalSizeStaysAcrossScales(t *testing.T) {
	f := newFixture(t, "M")
	Encode(f.paint(geom.Offset{}), f.glyphs, 1, &f.batch)
	one := f.batch.Instances[0].Rect
	Encode(f.paint(geom.Offset{}), f.glyphs, 2, &f.batch)
	two := f.batch.Instances[0].Rect
	// Both are in logical pixels, so only rounding tells them apart.
	if d := one.W - two.W; d > 1 || d < -1 {
		t.Errorf("logical widths %v and %v differ", one.W, two.W)
	}
}

func TestEncode_ShapesShareTheGlyphDraw(t *testing.T) {
	f := newFixture(t, "ab")
	dl := f.paint(geom.Offset{})
	f.canvas.DrawRRect(geom.RRectAll(geom.Rect{W: 10, H: 10}, 0), paint.Paint{Color: geom.Hex(0)})
	runs := f.layout.Runs()
	f.canvas.DrawGlyphs(&runs[0], geom.Offset{}, geom.Hex(0))
	Encode(dl, f.glyphs, 1, &f.batch)
	if len(f.batch.Draws) != 1 || f.batch.Draws[0].Count != 5 {
		t.Errorf("got %+v", f.batch.Draws)
	}
}

func TestEncode_ClipSplitsGlyphDraws(t *testing.T) {
	f := newFixture(t, "ab")
	f.paint(geom.Offset{})
	f.canvas.ClipRect(geom.Rect{W: 1000, H: 1000})
	runs := f.layout.Runs()
	f.canvas.DrawGlyphs(&runs[0], geom.Offset{}, geom.Hex(0))
	Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
	if len(f.batch.Draws) != 2 || f.batch.Draws[1].First != 2 {
		t.Errorf("got %+v", f.batch.Draws)
	}
}

func TestGlyphs_UploadsOnlyWhatChanged(t *testing.T) {
	f := newFixture(t, "abc")
	dl := f.paint(geom.Offset{})
	for range 3 {
		Encode(dl, f.glyphs, 1, &f.batch)
		if err := f.glyphs.Finish(&f.batch); err != nil {
			t.Fatal(err)
		}
	}
	if f.store.created != 1 {
		t.Errorf("created %d textures", f.store.created)
	}
	if len(f.store.updates) != 1 {
		t.Errorf("got %d updates, want the one of the first frame", len(f.store.updates))
	}
	f.glyphs.Destroy()
	if f.store.destroyed != 1 {
		t.Errorf("destroyed %d textures", f.store.destroyed)
	}
}

func TestEncode_WarmGlyphsDoNotAllocate(t *testing.T) {
	f := newFixture(t, "The quick brown fox jumps over the lazy dog")
	dl := f.paint(geom.Offset{})
	Encode(dl, f.glyphs, 1, &f.batch)
	if err := f.glyphs.Finish(&f.batch); err != nil {
		t.Fatal(err)
	}
	allocs := testing.AllocsPerRun(10, func() {
		Encode(dl, f.glyphs, 1, &f.batch)
		if err := f.glyphs.Finish(&f.batch); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Errorf("got %v allocs", allocs)
	}
}

func BenchmarkEncode_10kGlyphs(b *testing.B) {
	f := newFixture(b, "The quick brown fox jumps over the lazy dog, again and again and ever")
	f.canvas.Reset()
	runs := f.layout.Runs()
	for i := range 150 {
		for k := range runs {
			f.canvas.DrawGlyphs(&runs[k], geom.Offset{Y: float32(i)}, geom.Hex(0))
		}
	}
	dl := f.canvas.DisplayList()
	Encode(dl, f.glyphs, 1, &f.batch)
	b.ReportAllocs()
	for b.Loop() {
		Encode(dl, f.glyphs, 1, &f.batch)
	}
}

func TestRasterSize_LandsOnWholePixelsWhenZoomed(t *testing.T) {
	if got := rasterSize(21, 1); got != 21 {
		t.Errorf("got %v", got)
	}
	seen := map[float32]bool{}
	for z := float32(0.9); z < 1; z += 0.002 {
		seen[rasterSize(21, z)] = true
	}
	if len(seen) > 4 {
		t.Errorf("a transition must not make a size per frame, got %d", len(seen))
	}
}
