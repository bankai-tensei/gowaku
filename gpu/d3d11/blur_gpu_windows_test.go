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

// recordBlur draws the left half black and blurs a box across the edge, plain
// and under a shaped clip, which makes the frame multisampled.
func recordBlur(c *paint.Canvas, clipped bool) *gpu.Batch {
	c.Reset()
	c.DrawRRect(geom.RRect{Rect: geom.Rect{W: 100, H: 200}}, paint.Paint{Color: geom.Color{A: 1}})
	if clipped {
		c.ClipRRect(geom.RRectAll(geom.Rect{X: 40, Y: 40, W: 120, H: 120}, 30))
	}
	c.DrawBackdrop(geom.RRectAll(geom.Rect{X: 50, Y: 50, W: 100, H: 100}, 12), 6)
	batch := &gpu.Batch{}
	render.Encode(c.DisplayList(), nil, 1, batch)
	return batch
}

func TestBlur_FramesBlurAndReuseTheirTargets(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	tg := f.open()
	if err := tg.surface.Resize(400, 300, 1); err != nil {
		t.Fatal(err)
	}
	var c paint.Canvas
	frame := func(batch *gpu.Batch, read bool) {
		if err := f.device.BeginFrame(tg.surface, geom.Hex(0xffffff)); err != nil {
			t.Fatal(err)
		}
		if err := f.device.Draw(tg.surface, batch); err != nil {
			t.Fatal(err)
		}
		if read {
			img, err := f.device.ReadPixels(tg.surface)
			if err != nil {
				t.Fatal(err)
			}
			name := map[bool]string{false: "plain", true: "clipped"}[batch.Multisampled()]
			expectNear(t, img, name+" sharp black", 20, 100, color.NRGBA{A: 255}, 0)
			expectNear(t, img, name+" sharp white", 180, 100, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, 0)
			expectNear(t, img, name+" edge", 100, 100, color.NRGBA{R: 128, G: 128, B: 128, A: 255}, 12)
			expectNear(t, img, name+" inside black", 60, 100, color.NRGBA{R: 3, G: 3, B: 3, A: 255}, 4)
		}
		if err := f.device.EndFrame(tg.surface); err != nil {
			t.Fatal(err)
		}
	}
	for _, clipped := range []bool{false, true} {
		batch := recordBlur(&c, clipped)
		tg.surface.SetMultisample(batch.Multisampled())
		frame(batch, true)
	}
	a, b := f.device.blurA, f.device.blurB
	if a == nil || b == nil {
		t.Fatal("no scratch targets")
	}
	batch := recordBlur(&c, true)
	tg.surface.SetMultisample(true)
	allocs := testing.AllocsPerRun(20, func() { frame(batch, false) })
	if allocs != 0 {
		t.Errorf("a frame with a blur allocates %v times", allocs)
	}
	if f.device.blurA != a || f.device.blurB != b {
		t.Error("the scratch targets were made again")
	}
}
