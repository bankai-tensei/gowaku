//go:build gpu

package d3d11

import (
	"image/color"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/render"
)

// recordBlends draws a backdrop, and on it a square each in a mode that reads
// the target, one that the output merger does alone, and Clear.
func recordBlends(c *paint.Canvas, batch *gpu.Batch) {
	c.Reset()
	c.DrawRRect(geom.RRect{Rect: geom.Rect{W: 200, H: 100}}, paint.Paint{Color: geom.Color{R: 0.2, G: 0.6, B: 0.8, A: 1}})
	src := geom.Color{R: 0.9, G: 0.3, B: 0.5, A: 1}
	c.DrawRRect(geom.RRect{Rect: geom.Rect{X: 10, Y: 10, W: 40, H: 40}}, paint.Paint{Color: src, Blend: paint.BlendMultiply})
	c.DrawRRect(geom.RRect{Rect: geom.Rect{X: 60, Y: 10, W: 40, H: 40}}, paint.Paint{Color: src, Blend: paint.BlendScreen})
	c.DrawRRect(geom.RRect{Rect: geom.Rect{X: 110, Y: 10, W: 40, H: 40}}, paint.Paint{Color: src, Blend: paint.BlendClear})
	render.Encode(c.DisplayList(), nil, 1, batch)
}

func TestBlend_ModesDrawAndKeepTheirState(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	tg := f.open()
	if err := tg.surface.Resize(400, 300, 1); err != nil {
		t.Fatal(err)
	}
	var c paint.Canvas
	batch := &gpu.Batch{}
	recordBlends(&c, batch)
	for _, multisample := range []bool{true, false} {
		tg.surface.SetMultisample(multisample)
		if err := f.device.BeginFrame(tg.surface, geom.Hex(0xffffff)); err != nil {
			t.Fatal(err)
		}
		if err := f.device.Draw(tg.surface, batch); err != nil {
			t.Fatal(err)
		}
		img, err := f.device.ReadPixels(tg.surface)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.device.EndFrame(tg.surface); err != nil {
			t.Fatal(err)
		}
		name := map[bool]string{true: "multisampled", false: "plain"}[multisample]
		expectNear(t, img, name+" backdrop", 5, 5, color.NRGBA{R: 51, G: 153, B: 204, A: 255}, 1)
		expectNear(t, img, name+" multiply", 30, 30, color.NRGBA{R: 46, G: 46, B: 102, A: 255}, 1)
		expectNear(t, img, name+" screen", 80, 30, color.NRGBA{R: 235, G: 184, B: 230, A: 255}, 1)
		expectNear(t, img, name+" clear", 130, 30, color.NRGBA{A: 255}, 0)
		expectNear(t, img, name+" outside", 300, 200, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, 0)
	}
	screen := f.device.blends[gpu.BlendScreen]
	if screen == nil || f.device.blends[gpu.BlendClear] == nil {
		t.Fatal("the blend states were not made")
	}
	if f.device.blends[gpu.BlendDstOver] != nil {
		t.Error("a mode that was never drawn has a state")
	}
	allocs := testing.AllocsPerRun(20, func() {
		if err := f.device.BeginFrame(tg.surface, geom.Hex(0xffffff)); err != nil {
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
		t.Errorf("a frame with blends allocates %v times", allocs)
	}
	if f.device.blends[gpu.BlendScreen] != screen {
		t.Error("the state of a mode was made again")
	}
}

func TestBlend_ReadsATarget(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
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
	// Draw the target to the frame to read it.
	frame := &gpu.Batch{
		Instances: []gpu.Instance{{Rect: geom.Rect{W: 64, H: 64}, Color: geom.Color{A: 1}, GradientEnd: geom.Offset{X: 1, Y: 1}, Kind: gpu.InstanceImage}},
		Draws:     []gpu.DrawCall{{Count: 1, Image: target}},
	}
	tg.surface.SetMultisample(false)
	if err := tg.surface.Resize(400, 300, 1); err != nil {
		t.Fatal(err)
	}
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
	expectNear(t, img, "on the backdrop", 20, 30, color.NRGBA{R: 179, G: 77, B: 77, A: 255}, 1)
	// Where the target is transparent, the source shows as it is.
	expectNear(t, img, "on nothing", 40, 30, color.NRGBA{R: 230, G: 77, B: 128, A: 255}, 1)
}
