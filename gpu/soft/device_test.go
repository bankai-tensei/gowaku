package soft

import (
	"image/color"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

func mustPanic(t *testing.T, name string, f func() error) {
	t.Helper()
	defer func() {
		t.Helper()
		r := recover()
		if r == nil {
			t.Errorf("%s: no panic", name)
			return
		}
		if s, ok := r.(string); !ok || !strings.HasPrefix(s, "gowaku: ") {
			t.Errorf("%s: panic value %v", name, r)
		}
	}()
	t.Errorf("%s: returned %v", name, f())
}

func surface(t *testing.T, d *Device, width, height int) gpu.Surface {
	t.Helper()
	s, err := d.CreateSurface(nil, width, height, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Destroy)
	return s
}

func TestDevice_AdapterNameIsConstant(t *testing.T) {
	if New().AdapterName() == "" {
		t.Error("empty adapter name")
	}
}

func TestDevice_FramePanics(t *testing.T) {
	d := New()
	s := surface(t, d, 2, 2)
	b := quads()
	mustPanic(t, "Draw outside a frame", func() error { return d.Draw(s, b) })
	mustPanic(t, "EndFrame without BeginFrame", func() error { return d.EndFrame(s) })
	mustPanic(t, "ReadPixels outside a frame", func() error { _, err := d.ReadPixels(s); return err })
	if err := d.BeginFrame(s, geom.Color{}); err != nil {
		t.Fatal(err)
	}
	mustPanic(t, "BeginFrame twice", func() error { return d.BeginFrame(s, geom.Color{}) })
	mustPanic(t, "Resize in a frame", func() error { return s.Resize(4, 4, 1) })
	if err := d.EndFrame(s); err != nil {
		t.Fatal(err)
	}
}

func TestDevice_DestroyPanicsWithLiveSurfaces(t *testing.T) {
	d := New()
	s, err := d.CreateSurface(nil, 2, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	mustPanic(t, "Destroy", func() error { d.Destroy(); return nil })
	s.Destroy()
	d.Destroy()
}

func TestCreateSurface_EmptySizeFails(t *testing.T) {
	if _, err := New().CreateSurface(nil, 0, 2, 1); err == nil {
		t.Error("no error")
	}
}

func TestBeginFrame_FillsWithBackgroundAsItIs(t *testing.T) {
	d := New()
	s := surface(t, d, 2, 2)
	if err := d.BeginFrame(s, geom.Color{R: 0.2, G: 0.4, B: 1, A: 0.5}); err != nil {
		t.Fatal(err)
	}
	img, err := d.ReadPixels(s)
	if err != nil {
		t.Fatal(err)
	}
	// Not premultiplied, and opaque when read.
	checkPixel(t, img, 1, 1, color.NRGBA{R: 51, G: 102, B: 255, A: 255})
	if err := d.EndFrame(s); err != nil {
		t.Fatal(err)
	}
}

func TestReadPixels_SeesTheFrameSoFar(t *testing.T) {
	d := New()
	s := surface(t, d, 4, 4)
	if err := d.BeginFrame(s, geom.Hex(0xffffff)); err != nil {
		t.Fatal(err)
	}
	if err := d.Draw(s, quads(solid(0, 0, 2, 4, geom.Hex(0x000000)))); err != nil {
		t.Fatal(err)
	}
	first, err := d.ReadPixels(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Draw(s, quads(solid(2, 0, 2, 4, geom.Hex(0x000000)))); err != nil {
		t.Fatal(err)
	}
	second, err := d.ReadPixels(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.EndFrame(s); err != nil {
		t.Fatal(err)
	}
	checkPixel(t, first, 1, 1, black)
	checkPixel(t, first, 3, 1, white)
	checkPixel(t, second, 3, 1, black)
	checkPixel(t, d.Frame(s), 3, 1, black)
}

func TestFrame_NilBeforeTheFirstPresent(t *testing.T) {
	d := New()
	if d.Frame(surface(t, d, 2, 2)) != nil {
		t.Error("got a frame")
	}
}

func TestFrame_StaysValidAfterTheNextFrame(t *testing.T) {
	d := New()
	s := surface(t, d, 2, 2)
	present := func(c uint32) {
		t.Helper()
		if err := d.BeginFrame(s, geom.Hex(c)); err != nil {
			t.Fatal(err)
		}
		if err := d.EndFrame(s); err != nil {
			t.Fatal(err)
		}
	}
	present(0xff0000)
	first := d.Frame(s)
	present(0x0000ff)
	checkPixel(t, first, 0, 0, red)
	checkPixel(t, d.Frame(s), 0, 0, blue)
}

func TestDraw_WithoutInstancesDoesNothing(t *testing.T) {
	b := &gpu.Batch{Vertices: square(0, 0, 4, 4), Draws: []gpu.DrawCall{{Kind: gpu.DrawFill, VertexCount: 6}}}
	checkPixel(t, render(t, frame{width: 4, height: 4}, b), 2, 2, white)
}

func TestSurfaces_AreIndependent(t *testing.T) {
	d := New()
	a, b := surface(t, d, 8, 4), surface(t, d, 3, 6)
	if err := d.BeginFrame(a, geom.Hex(0xff0000)); err != nil {
		t.Fatal(err)
	}
	if err := d.BeginFrame(b, geom.Hex(0x0000ff)); err != nil {
		t.Fatal(err)
	}
	if err := d.Draw(a, quads(solid(0, 0, 4, 4, geom.Hex(0x000000)))); err != nil {
		t.Fatal(err)
	}
	if err := d.Draw(b, quads(solid(0, 0, 3, 2, geom.Hex(0xffffff)))); err != nil {
		t.Fatal(err)
	}
	if err := d.EndFrame(a); err != nil {
		t.Fatal(err)
	}
	// One surface in a frame still keeps DrawToTarget away.
	target := newTarget(t, d, 1, 1)
	mustPanic(t, "DrawToTarget", func() error { return d.DrawToTarget(target, quads(), 1) })
	if err := d.EndFrame(b); err != nil {
		t.Fatal(err)
	}

	first, second := d.Frame(a), d.Frame(b)
	if got := first.Bounds().Size(); got.X != 8 || got.Y != 4 {
		t.Errorf("first size: %v", got)
	}
	if got := second.Bounds().Size(); got.X != 3 || got.Y != 6 {
		t.Errorf("second size: %v", got)
	}
	checkPixel(t, first, 1, 1, black)
	checkPixel(t, first, 6, 1, red)
	checkPixel(t, second, 1, 1, white)
	checkPixel(t, second, 1, 4, blue)
}

func TestSurfaces_ShareTextures(t *testing.T) {
	d := New()
	tx := imageOf(t, d, 1, 1, []byte{0, 255, 0, 255})
	defer tx.Destroy()
	for _, size := range []int{2, 5} {
		img := renderOn(t, d, frame{width: size, height: size}, imageBatch(tx, 0, 0, float32(size), float32(size), 1))
		checkPixel(t, img, size-1, size-1, green)
	}
}

func TestSurface_Resize(t *testing.T) {
	d := New()
	s := surface(t, d, 4, 4)
	if err := s.Resize(8, 2, 2); err != nil {
		t.Fatal(err)
	}
	if err := s.Resize(0, 0, 3); err != nil { // minimized: ignored
		t.Fatal(err)
	}
	if err := d.BeginFrame(s, geom.Hex(0xffffff)); err != nil {
		t.Fatal(err)
	}
	if err := d.Draw(s, quads(solid(0, 0, 2, 1, geom.Hex(0x000000)))); err != nil {
		t.Fatal(err)
	}
	if err := d.EndFrame(s); err != nil {
		t.Fatal(err)
	}
	img := d.Frame(s)
	if got := img.Bounds().Size(); got.X != 8 || got.Y != 2 {
		t.Fatalf("size: %v", got)
	}
	checkPixel(t, img, 3, 1, black)
	checkPixel(t, img, 4, 1, white)
}

func TestSetMultisample_TakesEffectAtTheNextFrame(t *testing.T) {
	d := New()
	s := surface(t, d, 8, 2)
	b := quads(cover(2.5, 0, 3, 2, geom.Hex(0x000000)))
	draw := func() color.NRGBA {
		t.Helper()
		if err := d.BeginFrame(s, geom.Hex(0xffffff)); err != nil {
			t.Fatal(err)
		}
		if err := d.Draw(s, b); err != nil {
			t.Fatal(err)
		}
		if err := d.EndFrame(s); err != nil {
			t.Fatal(err)
		}
		return d.Frame(s).NRGBAAt(2, 1)
	}
	if got, want := draw(), mix(white, black, 2); got != want {
		t.Errorf("new surface: got %v, want %v", got, want)
	}
	s.SetMultisample(false)
	if got := draw(); got != black {
		t.Errorf("off: got %v", got)
	}
	s.SetMultisample(true)
	if got, want := draw(), mix(white, black, 2); got != want {
		t.Errorf("on again: got %v, want %v", got, want)
	}
}

// typicalUI is a frame of a dense window: panels with shadows, rows and text.
func typicalUI(atlas gpu.Texture) *gpu.Batch {
	b := &gpu.Batch{}
	for y := float32(20); y < 660; y += 160 {
		for x := float32(20); x < 900; x += 320 {
			shadow := solid(x, y, 300, 140, geom.Color{A: 0.3})
			shadow.Kind, shadow.Blur = gpu.InstanceShadow, 24
			shadow.Radii = [4]float32{12, 12, 12, 12}
			panel := solid(x, y, 300, 140, geom.Hex(0xffffff))
			panel.Radii = shadow.Radii
			panel.BorderWidth, panel.BorderColor = 1, geom.Hex(0xd0d0d8)
			b.Instances = append(b.Instances, shadow, panel)
		}
	}
	for y := float32(30); y < 680; y += 18 {
		for x := float32(30); x < 960; x += 9 {
			glyph := solid(x, y, 8, 14, geom.Hex(0x1d1d1f))
			glyph.Kind = gpu.InstanceGlyph
			b.Instances = append(b.Instances, glyph)
		}
	}
	b.Draws = []gpu.DrawCall{{Kind: gpu.DrawQuads, Count: len(b.Instances), Texture: atlas}}
	return b
}

// BenchmarkFrame_TypicalUI allocates the targets of a frame on every frame, which
// this package prefers to reuse for being simpler.
func BenchmarkFrame_TypicalUI(b *testing.B) {
	d := New()
	s, err := d.CreateSurface(nil, 1000, 700, 1)
	if err != nil {
		b.Fatal(err)
	}
	defer s.Destroy()
	atlas, err := d.CreateTexture(64, 64, gpu.TextureR8)
	if err != nil {
		b.Fatal(err)
	}
	defer atlas.Destroy()
	batch := typicalUI(atlas)
	b.ReportAllocs()
	for b.Loop() {
		if err := d.BeginFrame(s, geom.Hex(0xf5f5f7)); err != nil {
			b.Fatal(err)
		}
		if err := d.Draw(s, batch); err != nil {
			b.Fatal(err)
		}
		if err := d.EndFrame(s); err != nil {
			b.Fatal(err)
		}
	}
}
