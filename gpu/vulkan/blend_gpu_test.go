//go:build gpu && (linux || windows)

package vulkan

import (
	"image"
	"image/color"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/render"
)

// newFastFixture is newFixture on the fast GPU, as the goldens use: the Windows
// driver of Iris Xe damages some draws. See docs/ADR/0013-vulkan-backend.md.
func newFastFixture(t *testing.T) *fixture {
	t.Helper()
	p := newPlatform(t)
	t.Cleanup(p.Destroy)
	d, err := New(Options{Power: gpu.HighPerformance})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Destroy)
	t.Logf("adapter: %s, samples: %d", d.AdapterName(), d.samples)
	return &fixture{t, p, d}
}

func near(t *testing.T, img *image.NRGBA, name string, x, y int, want color.NRGBA, tolerance int) {
	t.Helper()
	got := img.NRGBAAt(x, y)
	for _, d := range []int{int(got.R) - int(want.R), int(got.G) - int(want.G), int(got.B) - int(want.B)} {
		if d < -tolerance || d > tolerance {
			t.Errorf("%s (%d, %d): got %v, want %v within %d", name, x, y, got, want, tolerance)
			return
		}
	}
}

func TestBlend_ModesDrawAndKeepTheirPipelines(t *testing.T) {
	f := newFastFixture(t)
	tg := f.open()
	var c paint.Canvas
	c.Reset()
	c.DrawRRect(geom.RRect{Rect: geom.Rect{W: 200, H: 100}}, paint.Paint{Color: geom.Color{R: 0.2, G: 0.6, B: 0.8, A: 1}})
	src := geom.Color{R: 0.9, G: 0.3, B: 0.5, A: 1}
	c.DrawRRect(geom.RRect{Rect: geom.Rect{X: 10, Y: 10, W: 40, H: 40}}, paint.Paint{Color: src, Blend: paint.BlendMultiply})
	c.DrawRRect(geom.RRect{Rect: geom.Rect{X: 60, Y: 10, W: 40, H: 40}}, paint.Paint{Color: src, Blend: paint.BlendScreen})
	c.DrawRRect(geom.RRect{Rect: geom.Rect{X: 110, Y: 10, W: 40, H: 40}}, paint.Paint{Color: src, Blend: paint.BlendClear})
	batch := &gpu.Batch{}
	render.Encode(c.DisplayList(), nil, 1, batch)
	for _, multisample := range []bool{true, false} {
		tg.surface.SetMultisample(multisample)
		img := f.read(tg, batch)
		name := map[bool]string{true: "multisampled", false: "plain"}[multisample]
		near(t, img, name+" backdrop", 5, 5, color.NRGBA{R: 51, G: 153, B: 204}, 1)
		near(t, img, name+" multiply", 30, 30, color.NRGBA{R: 46, G: 46, B: 102}, 1)
		near(t, img, name+" screen", 80, 30, color.NRGBA{R: 235, G: 184, B: 230}, 1)
		near(t, img, name+" clear", 130, 30, color.NRGBA{}, 0)
		near(t, img, name+" outside", 300, 200, color.NRGBA{R: 255, G: 255, B: 255}, 0)
	}
	plain := &f.device.passes[passPlain]
	screen := plain.blended[blendQuad][gpu.BlendScreen]
	if screen == 0 || plain.blended[blendQuad][gpu.BlendClear] == 0 {
		t.Fatal("the pipelines of the modes were not made")
	}
	if plain.blended[blendQuad][gpu.BlendMultiply] != 0 {
		t.Error("a mode that reads the target composites source-over, with the pipeline of the pass")
	}
	if f.device.samples > 1 && f.device.passes[passMulti].blended[blendQuad][gpu.BlendScreen] == 0 {
		t.Error("the multisampled pass has no pipeline of its own")
	}
	for range 4 {
		f.frame(tg, batch)
	}
	allocs := testing.AllocsPerRun(50, func() { f.frame(tg, batch) })
	if allocs != 0 {
		t.Errorf("a frame with blends allocates %v times", allocs)
	}
	if plain.blended[blendQuad][gpu.BlendScreen] != screen {
		t.Error("the pipeline of a mode was made again")
	}
}

func TestBlend_ReadsATarget(t *testing.T) {
	f := newFastFixture(t)
	tg := f.open()
	target, err := f.device.CreateTarget(64, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Destroy()
	var c paint.Canvas
	c.Reset()
	c.DrawRRect(geom.RRect{Rect: geom.Rect{W: 32, H: 64}}, paint.Paint{Color: geom.Color{R: 0.2, G: 0.6, B: 0.8, A: 1}})
	c.DrawRRect(geom.RRect{Rect: geom.Rect{X: 16, Y: 16, W: 32, H: 32}}, paint.Paint{Color: geom.Color{R: 0.9, G: 0.3, B: 0.5, A: 1}, Blend: paint.BlendDifference})
	batch := &gpu.Batch{}
	render.Encode(c.DisplayList(), nil, 1, batch)
	if err := f.device.DrawToTarget(target, batch, 1); err != nil {
		t.Fatal(err)
	}
	frame := &gpu.Batch{
		Instances: []gpu.Instance{{Rect: geom.Rect{W: 64, H: 64}, Color: geom.Color{A: 1}, GradientEnd: geom.Offset{X: 1, Y: 1}, Kind: gpu.InstanceImage}},
		Draws:     []gpu.DrawCall{{Count: 1, Image: target}},
	}
	tg.surface.SetMultisample(false)
	if err := f.device.BeginFrame(tg.surface, geom.Hex(0x000000)); err != nil {
		t.Fatal(err)
	}
	if err := f.device.Draw(tg.surface, frame); err != nil {
		t.Fatal(err)
	}
	img, err := f.device.ReadPixels(tg.surface)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.device.EndFrame(tg.surface); err != nil {
		t.Fatal(err)
	}
	near(t, img, "on the backdrop", 20, 30, color.NRGBA{R: 179, G: 77, B: 77}, 1)
	near(t, img, "on nothing", 40, 30, color.NRGBA{R: 230, G: 77, B: 128}, 1)
}
