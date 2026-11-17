//go:build gpu && (linux || windows)

package vulkan

import (
	"image"
	"image/color"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/platform"
)

type fixture struct {
	t        *testing.T
	platform platform.Platform
	device   *Device
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	p := newPlatform(t)
	t.Cleanup(p.Destroy)
	d, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Destroy)
	t.Logf("adapter: %s, samples: %d", d.AdapterName(), d.samples)
	return &fixture{t, p, d}
}

type testWindow struct {
	window  platform.Window
	surface gpu.Surface
}

func (f *fixture) open() *testWindow {
	f.t.Helper()
	w, err := f.platform.CreateWindow(platform.WindowOptions{Title: "vulkan test", Width: 320, Height: 240})
	if err != nil {
		f.t.Fatal(err)
	}
	width, height := w.PhysicalSize()
	s, err := f.device.CreateSurface(w.NativeHandle(), width, height, 1)
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() {
		s.Destroy()
		w.Close()
	})
	return &testWindow{w, s}
}

func cover(c geom.Color, w, h float32) *gpu.Batch {
	return &gpu.Batch{
		Instances: []gpu.Instance{{Rect: geom.Rect{W: w, H: h}, Color: c}},
		Draws:     []gpu.DrawCall{{Kind: gpu.DrawQuads, Count: 1}},
	}
}

func (f *fixture) frame(tg *testWindow, batch *gpu.Batch) {
	f.t.Helper()
	if err := f.device.BeginFrame(tg.surface, geom.Hex(0xffffff)); err != nil {
		f.t.Fatal(err)
	}
	if err := f.device.Draw(tg.surface, batch); err != nil {
		f.t.Fatal(err)
	}
	if err := f.device.EndFrame(tg.surface); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) read(tg *testWindow, batch *gpu.Batch) *image.NRGBA {
	f.t.Helper()
	if err := f.device.BeginFrame(tg.surface, geom.Hex(0xffffff)); err != nil {
		f.t.Fatal(err)
	}
	if err := f.device.Draw(tg.surface, batch); err != nil {
		f.t.Fatal(err)
	}
	img, err := f.device.ReadPixels(tg.surface)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.device.EndFrame(tg.surface); err != nil {
		f.t.Fatal(err)
	}
	return img
}

func TestFrame_DoesNotAllocate(t *testing.T) {
	f := newFixture(t)
	tg := f.open()
	batch := cover(geom.Hex(0x0a84ff), 100, 100)
	background := geom.Hex(0xffffff)
	for range 8 {
		f.frame(tg, batch)
	}
	allocs := testing.AllocsPerRun(200, func() {
		if err := f.device.BeginFrame(tg.surface, background); err != nil {
			t.Fatal(err)
		}
		if err := f.device.Draw(tg.surface, batch); err != nil {
			t.Fatal(err)
		}
		if err := f.device.EndFrame(tg.surface); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Errorf("a frame allocates %v times", allocs)
	}
}

func TestUpdateTexture_DoesNotAllocate(t *testing.T) {
	f := newFixture(t)
	tg := f.open()
	tex, err := f.device.CreateTexture(64, 64, gpu.TextureR8)
	if err != nil {
		t.Fatal(err)
	}
	defer tex.Destroy()
	pix := make([]byte, 64*64)
	region := image.Rect(8, 8, 24, 24)
	batch := cover(geom.Hex(0x0a84ff), 100, 100)
	for range 8 {
		if err := f.device.UpdateTexture(tex, region, pix, 64); err != nil {
			t.Fatal(err)
		}
		f.frame(tg, batch)
	}
	allocs := testing.AllocsPerRun(100, func() {
		if err := f.device.UpdateTexture(tex, region, pix, 64); err != nil {
			t.Fatal(err)
		}
	})
	f.frame(tg, batch)
	if allocs != 0 {
		t.Errorf("an update allocates %v times", allocs)
	}
}

func TestDraw_GrowsBuffersWhileFramesAreInFlight(t *testing.T) {
	f := newFixture(t)
	tg := f.open()
	want := color.NRGBA{R: 255, A: 255}
	for _, n := range []int{1, 4000, 50000, 200000} {
		b := &gpu.Batch{Instances: make([]gpu.Instance, n), Draws: []gpu.DrawCall{{Kind: gpu.DrawQuads, Count: n}}}
		for i := range b.Instances {
			b.Instances[i] = gpu.Instance{Rect: geom.Rect{W: 100, H: 100}, Color: geom.Hex(0xff0000)}
		}
		img := f.read(tg, b)
		if got := img.NRGBAAt(50, 50); got != want {
			t.Errorf("%d instances: got %v", n, got)
		}
	}
}

func TestDraw_TwoBatchesInOneFrame(t *testing.T) {
	f := newFixture(t)
	tg := f.open()
	if err := f.device.BeginFrame(tg.surface, geom.Hex(0xffffff)); err != nil {
		t.Fatal(err)
	}
	if err := f.device.Draw(tg.surface, cover(geom.Hex(0xff0000), 100, 100)); err != nil {
		t.Fatal(err)
	}
	if err := f.device.Draw(tg.surface, cover(geom.Hex(0x0000ff), 50, 50)); err != nil {
		t.Fatal(err)
	}
	img, err := f.device.ReadPixels(tg.surface)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.device.EndFrame(tg.surface); err != nil {
		t.Fatal(err)
	}
	if got := img.NRGBAAt(75, 75); got != (color.NRGBA{R: 255, A: 255}) {
		t.Errorf("first batch: got %v", got)
	}
	if got := img.NRGBAAt(25, 25); got != (color.NRGBA{B: 255, A: 255}) {
		t.Errorf("second batch: got %v", got)
	}
}

func TestResize_Stress(t *testing.T) {
	f := newFixture(t)
	tg := f.open()
	batch := cover(geom.Hex(0x30d158), 4000, 4000)
	for i := range 120 {
		w, h := 200+(i*37)%400, 150+(i*53)%300
		if err := tg.surface.Resize(w, h, 1); err != nil {
			t.Fatal(err)
		}
		f.frame(tg, batch)
	}
}

func TestResize_EmptyIsIgnored(t *testing.T) {
	f := newFixture(t)
	tg := f.open()
	if err := tg.surface.Resize(0, 0, 1); err != nil {
		t.Fatal(err)
	}
	f.frame(tg, cover(geom.Hex(0x30d158), 100, 100))
}

func TestMultiWindow_SurfacesAreIndependent(t *testing.T) {
	f := newFixture(t)
	a, b := f.open(), f.open()
	for range 30 {
		red := f.read(a, cover(geom.Hex(0xff0000), 400, 400))
		blue := f.read(b, cover(geom.Hex(0x0000ff), 400, 400))
		if got := red.NRGBAAt(10, 10); got != (color.NRGBA{R: 255, A: 255}) {
			t.Fatalf("window a: got %v", got)
		}
		if got := blue.NRGBAAt(10, 10); got != (color.NRGBA{B: 255, A: 255}) {
			t.Fatalf("window b: got %v", got)
		}
	}
}

func TestTexture_ManyOutliveTheFirstDescriptorPool(t *testing.T) {
	f := newFixture(t)
	tg := f.open()
	textures := make([]gpu.Texture, 0, 3*descriptorsPerPool)
	for range cap(textures) {
		tex, err := f.device.CreateTexture(4, 4, gpu.TextureRGBA8)
		if err != nil {
			t.Fatal(err)
		}
		textures = append(textures, tex)
	}
	batch := cover(geom.Hex(0x0a84ff), 100, 100)
	f.frame(tg, batch)
	for _, tex := range textures {
		tex.Destroy()
	}
	// The textures go when the frames that could read them are done.
	for range framesInFlight + 1 {
		f.frame(tg, batch)
	}
	for range 3 {
		tex, err := f.device.CreateTexture(4, 4, gpu.TextureRGBA8)
		if err != nil {
			t.Fatal(err)
		}
		tex.Destroy()
	}
}

func TestTexture_UpdateReachesTheNextFrame(t *testing.T) {
	f := newFixture(t)
	tg := f.open()
	tex, err := f.device.CreateTexture(2, 1, gpu.TextureRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	defer tex.Destroy()
	batch := &gpu.Batch{
		Instances: []gpu.Instance{{
			Rect: geom.Rect{W: 100, H: 100}, Color: geom.Color{A: 1}, Kind: gpu.InstanceImage,
			GradientEnd: geom.Offset{X: 1, Y: 1},
		}},
		Draws: []gpu.DrawCall{{Kind: gpu.DrawQuads, Count: 1, Image: tex}},
	}
	for _, c := range []color.NRGBA{{R: 255, A: 255}, {G: 255, A: 255}, {B: 255, A: 255}} {
		pix := []byte{c.R, c.G, c.B, 255, c.R, c.G, c.B, 255}
		if err := f.device.UpdateTexture(tex, image.Rect(0, 0, 2, 1), pix, 8); err != nil {
			t.Fatal(err)
		}
		if got := f.read(tg, batch).NRGBAAt(50, 50); got != c {
			t.Errorf("got %v, want %v", got, c)
		}
	}
}

func TestContract_Panics(t *testing.T) {
	f := newFixture(t)
	tg := f.open()
	mustPanic := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s: no panic", name)
			}
		}()
		fn()
	}
	batch := cover(geom.Hex(0x0a84ff), 100, 100)
	mustPanic("Draw outside a frame", func() { _ = f.device.Draw(tg.surface, batch) })
	mustPanic("EndFrame without BeginFrame", func() { _ = f.device.EndFrame(tg.surface) })
	if err := f.device.BeginFrame(tg.surface, geom.Color{}); err != nil {
		t.Fatal(err)
	}
	mustPanic("BeginFrame twice", func() { _ = f.device.BeginFrame(tg.surface, geom.Color{}) })
	mustPanic("Resize in a frame", func() { _ = tg.surface.Resize(100, 100, 1) })
	mustPanic("CreateTexture in a frame", func() { _, _ = f.device.CreateTexture(1, 1, gpu.TextureR8) })
	if err := f.device.EndFrame(tg.surface); err != nil {
		t.Fatal(err)
	}
}

func TestCreateSurface_RejectsForeignHandle(t *testing.T) {
	f := newFixture(t)
	if _, err := f.device.CreateSurface(foreignHandle(), 10, 10, 1); err == nil {
		t.Error("a window of another system was accepted")
	}
}
