package render

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
)

func boxAt(x, y float32) geom.RRect { return geom.RRect{Rect: geom.Rect{X: x, Y: y, W: 10, H: 10}} }

func TestEncode_BlendSplitsDrawCalls(t *testing.T) {
	b := record(func(c *paint.Canvas) {
		c.DrawRRect(boxAt(0, 0), paint.Paint{Color: red})
		c.DrawRRect(boxAt(20, 0), paint.Paint{Color: red, Blend: paint.BlendScreen})
		c.DrawRRect(boxAt(40, 0), paint.Paint{Color: red, Blend: paint.BlendScreen})
		c.DrawRRect(boxAt(60, 0), paint.Paint{Color: red})
	})
	if len(b.Draws) != 3 {
		t.Fatalf("got %d draw calls, want 3: %+v", len(b.Draws), b.Draws)
	}
	want := []struct {
		blend gpu.BlendMode
		count int
	}{{gpu.BlendSrcOver, 1}, {gpu.BlendScreen, 2}, {gpu.BlendSrcOver, 1}}
	for i, w := range want {
		if d := b.Draws[i]; d.Blend != w.blend || d.Count != w.count {
			t.Errorf("call %d: blend %d count %d, want %d and %d", i, d.Blend, d.Count, w.blend, w.count)
		}
	}
	for i, in := range b.Instances {
		blend := gpu.BlendSrcOver
		if i == 1 || i == 2 {
			blend = gpu.BlendScreen
		}
		if in.Kind.Blend() != blend || in.Kind.Base() != gpu.InstanceRRect {
			t.Errorf("instance %d: kind %#x, want the blend %d of its call", i, in.Kind, blend)
		}
	}
}

func TestEncode_BackdropCallsJoinOnlyApart(t *testing.T) {
	b := record(func(c *paint.Canvas) {
		c.DrawRRect(boxAt(0, 0), paint.Paint{Color: red, Blend: paint.BlendMultiply})
		// Far away: it joins the copy of the first.
		c.DrawRRect(boxAt(100, 0), paint.Paint{Color: red, Blend: paint.BlendMultiply})
		// On top of the first: it must see what the first drew.
		c.DrawRRect(boxAt(5, 5), paint.Paint{Color: red, Blend: paint.BlendMultiply})
	})
	if len(b.Draws) != 2 || b.Draws[0].Count != 2 || b.Draws[1].Count != 1 {
		t.Fatalf("got calls %+v, want 2 instances and 1", b.Draws)
	}
	// The bounds are the quads, with their antialiased margin of a pixel.
	if got, want := b.Draws[0].Bounds, (geom.Rect{X: -1, Y: -1, W: 112, H: 12}); got != want {
		t.Errorf("bounds: got %v, want %v", got, want)
	}
	if got, want := b.Draws[1].Bounds, (geom.Rect{X: 4, Y: 4, W: 12, H: 12}); got != want {
		t.Errorf("bounds: got %v, want %v", got, want)
	}
}

func TestEncode_FixedBlendsIgnoreOverlap(t *testing.T) {
	b := record(func(c *paint.Canvas) {
		c.DrawRRect(boxAt(0, 0), paint.Paint{Color: red, Blend: paint.BlendPlus})
		c.DrawRRect(boxAt(5, 5), paint.Paint{Color: red, Blend: paint.BlendPlus})
	})
	if len(b.Draws) != 1 || b.Draws[0].Count != 2 {
		t.Errorf("got calls %+v, want one of 2: the GPU blends each instance in turn", b.Draws)
	}
}

func TestEncode_DstDrawsNothing(t *testing.T) {
	b := record(func(c *paint.Canvas) {
		c.DrawRRect(boxAt(0, 0), paint.Paint{Color: red, Blend: paint.BlendDst})
	})
	if len(b.Instances) != 0 || len(b.Draws) != 0 {
		t.Errorf("got %d instances and %d calls", len(b.Instances), len(b.Draws))
	}
}

func TestEncode_PushBlendReachesEveryCommand(t *testing.T) {
	b := record(func(c *paint.Canvas) {
		c.Save()
		c.PushBlend(paint.BlendDifference)
		c.DrawRRect(boxAt(0, 0), paint.Paint{Color: red})
		c.DrawRRect(boxAt(20, 0), paint.Paint{Color: red, Blend: paint.BlendScreen})
		c.Restore()
		c.DrawRRect(boxAt(40, 0), paint.Paint{Color: red})
	})
	if len(b.Draws) != 3 {
		t.Fatalf("got calls %+v", b.Draws)
	}
	for i, want := range []gpu.BlendMode{gpu.BlendDifference, gpu.BlendScreen, gpu.BlendSrcOver} {
		if b.Draws[i].Blend != want {
			t.Errorf("call %d: blend %d, want %d", i, b.Draws[i].Blend, want)
		}
	}
}

func TestEncode_PathBlendsReadTheBackdrop(t *testing.T) {
	var p paint.Path
	p.AddRRect(geom.RRect{Rect: geom.Rect{W: 200, H: 100}})
	b := record(func(c *paint.Canvas) {
		c.DrawPath(&p, paint.Paint{Color: red, Blend: paint.BlendOverlay})
	})
	if len(b.Draws) != 1 || b.Draws[0].Kind != gpu.DrawFill || b.Draws[0].Blend != gpu.BlendOverlay {
		t.Fatalf("got calls %+v", b.Draws)
	}
	if got := b.Draws[0].Bounds; got.W < 200 || got.H < 100 {
		t.Errorf("bounds %v must hold the cover", got)
	}
	if b.Instances[0].Kind.Blend() != gpu.BlendOverlay {
		t.Errorf("cover kind %#x", b.Instances[0].Kind)
	}
}

func TestEncode_BlendsDoNotAllocate(t *testing.T) {
	var c paint.Canvas
	b := &gpu.Batch{}
	encode := func() {
		c.Reset()
		for i := range 50 {
			mode := paint.BlendMode(i % paint.BlendModeCount)
			c.DrawRRect(boxAt(float32(i%10)*12, float32(i/10)*12), paint.Paint{Color: red, Blend: mode})
		}
		Encode(c.DisplayList(), nil, 1, b)
	}
	encode()
	if allocs := testing.AllocsPerRun(50, encode); allocs != 0 {
		t.Errorf("got %v allocs, want 0", allocs)
	}
}

func BenchmarkEncode_10kBlendedRects(b *testing.B) {
	var c paint.Canvas
	c.Reset()
	// One row, so that each rectangle stays clear of those before it and all
	// share one copy of the backdrop.
	for i := range 10000 {
		c.DrawRRect(boxAt(float32(i)*16, 0), paint.Paint{Color: red, Blend: paint.BlendMultiply})
	}
	batch := &gpu.Batch{}
	b.ReportAllocs()
	for b.Loop() {
		Encode(c.DisplayList(), nil, 1, batch)
	}
	if len(batch.Draws) != 1 {
		b.Fatalf("got %d draw calls, want 1", len(batch.Draws))
	}
}
