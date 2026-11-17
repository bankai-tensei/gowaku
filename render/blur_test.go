package render

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
)

func TestEncode_BackdropHasACallOfItsOwn(t *testing.T) {
	b := record(func(c *paint.Canvas) {
		c.DrawRRect(boxAt(0, 0), paint.Paint{Color: red})
		c.DrawBackdrop(geom.RRectAll(geom.Rect{X: 10, Y: 10, W: 40, H: 30}, 8), 6)
		c.DrawRRect(boxAt(20, 20), paint.Paint{Color: red})
	})
	if len(b.Draws) != 3 {
		t.Fatalf("got %d calls, want the blur apart: %+v", len(b.Draws), b.Draws)
	}
	d, in := b.Draws[1], b.Instances[1]
	if d.Blur != 6 || d.Count != 1 || in.Kind != gpu.InstanceBackdrop || in.Blur != 6 || in.Color.A != 1 {
		t.Errorf("call %+v, instance %+v", d, in)
	}
	if in.Radii != [4]float32{8, 8, 8, 8} {
		t.Errorf("radii %v", in.Radii)
	}
	if want := (geom.Rect{X: 9, Y: 9, W: 42, H: 32}); d.Bounds != want {
		t.Errorf("bounds %v, want %v", d.Bounds, want)
	}
}

func TestLayer_BackdropGoesInline(t *testing.T) {
	f := newLayerFixture(t)
	var l paint.Layer
	cv := l.Begin()
	cv.DrawRRect(geom.RRect{Rect: geom.Rect{W: 100, H: 40}}, paint.Paint{Color: red})
	cv.DrawBackdrop(geom.RRect{Rect: geom.Rect{X: 10, Y: 10, W: 50, H: 20}}, 4)
	l.End()
	for range layerCalmFrames + 2 {
		f.frame(t, 1, func(cv *paint.Canvas) { cv.DrawLayer(&l, geom.Offset{X: 5, Y: 5}) })
	}
	if f.store.targetDraws != 0 {
		t.Errorf("drew a layer that reads the surface to a page %d times", f.store.targetDraws)
	}
	var blur *gpu.DrawCall
	for i := range f.batch.Draws {
		if f.batch.Draws[i].Blur > 0 {
			blur = &f.batch.Draws[i]
		}
	}
	if blur == nil {
		t.Fatal("the blur is not in the frame")
	}
	if want := (geom.Rect{X: 14, Y: 14, W: 52, H: 22}); blur.Bounds != want {
		t.Errorf("bounds %v, want %v moved with the layer", blur.Bounds, want)
	}
}

func TestEncode_BackdropDoesNotAllocate(t *testing.T) {
	var c paint.Canvas
	b := &gpu.Batch{}
	encode := func() {
		c.Reset()
		for i := range 20 {
			c.DrawRRect(boxAt(float32(i)*12, 0), paint.Paint{Color: red})
			c.DrawBackdrop(geom.RRectAll(geom.Rect{X: float32(i) * 12, Y: 5, W: 30, H: 30}, 6), 8)
		}
		Encode(c.DisplayList(), nil, 1, b)
	}
	encode()
	if allocs := testing.AllocsPerRun(50, encode); allocs != 0 {
		t.Errorf("got %v allocs, want 0", allocs)
	}
}
