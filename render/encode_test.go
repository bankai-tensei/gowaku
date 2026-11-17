package render

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
)

var (
	red   = geom.Hex(0xff0000)
	green = geom.Hex(0x00ff00)
	box   = geom.RRect{Rect: geom.Rect{W: 10, H: 10}, TopLeft: 1, TopRight: 2, BottomRight: 3, BottomLeft: 4}
)

func record(draw func(c *paint.Canvas)) *gpu.Batch {
	var c paint.Canvas
	c.Reset()
	draw(&c)
	b := &gpu.Batch{}
	Encode(c.DisplayList(), nil, 1, b)
	return b
}

func TestEncode_RRect(t *testing.T) {
	b := record(func(c *paint.Canvas) {
		c.DrawRRect(box, paint.Paint{Color: red, Stroke: &paint.Stroke{Width: 2, Color: green}})
	})
	in := b.Instances[0]
	if in.Kind != gpu.InstanceRRect || in.Gradient != gpu.GradientNone {
		t.Errorf("mode: got %+v", in)
	}
	if want := [4]float32{1, 2, 3, 4}; in.Radii != want {
		t.Errorf("radii: got %v, want %v", in.Radii, want)
	}
	if in.Color != red || in.BorderColor != green || in.BorderWidth != 2 {
		t.Errorf("colors: got %+v", in)
	}
}

func TestEncode_Gradient(t *testing.T) {
	tests := []struct {
		name string
		kind paint.GradientKind
		want gpu.GradientMode
	}{
		{"線形", paint.GradientLinear, gpu.GradientLinear},
		{"放射", paint.GradientRadial, gpu.GradientRadial},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := record(func(c *paint.Canvas) {
				c.DrawRRect(box, paint.Paint{Gradient: &paint.Gradient{
					Kind: tt.kind, End: geom.Offset{X: 7}, StartColor: red, EndColor: green,
				}})
			})
			in := b.Instances[0]
			if in.Gradient != tt.want || in.Color != red || in.EndColor != green || in.GradientEnd.X != 7 {
				t.Errorf("got %+v", in)
			}
		})
	}
}

func TestEncode_Shadow(t *testing.T) {
	b := record(func(c *paint.Canvas) { c.DrawShadow(box, 6, red, geom.Offset{}) })
	in := b.Instances[0]
	if in.Kind != gpu.InstanceShadow || in.Blur != 6 || in.Color != red {
		t.Errorf("got %+v", in)
	}
}

func TestEncode_SameClipSharesADrawCall(t *testing.T) {
	b := record(func(c *paint.Canvas) {
		c.DrawRRect(box, paint.Paint{Color: red})
		c.DrawRRect(box, paint.Paint{Color: red})
		c.Save()
		c.ClipRect(geom.Rect{W: 5, H: 5})
		c.DrawRRect(box, paint.Paint{Color: red})
		c.DrawRRect(box, paint.Paint{Color: red})
		c.Restore()
		c.DrawRRect(box, paint.Paint{Color: red})
	})
	want := []gpu.DrawCall{
		{First: 0, Count: 2},
		{First: 2, Count: 2, Clip: geom.Rect{W: 5, H: 5}, Clipped: true},
		{First: 4, Count: 1},
	}
	if len(b.Draws) != len(want) {
		t.Fatalf("got %d draws, want %d: %+v", len(b.Draws), len(want), b.Draws)
	}
	for i := range want {
		if b.Draws[i] != want[i] {
			t.Errorf("draw %d: got %+v, want %+v", i, b.Draws[i], want[i])
		}
	}
}

func TestEncode_EmptyListClearsBatch(t *testing.T) {
	b := record(func(c *paint.Canvas) { c.DrawRRect(box, paint.Paint{Color: red}) })
	Encode(&paint.DisplayList{}, nil, 1, b)
	if len(b.Instances) != 0 || len(b.Draws) != 0 {
		t.Errorf("got %+v", b)
	}
}

func decoratedList(n int) *paint.DisplayList {
	var c paint.Canvas
	c.Reset()
	for i := range n {
		if i%100 == 0 {
			c.ClipRect(geom.Rect{W: float32(1000 - i%1000), H: 1000})
		}
		c.DrawRRect(box, paint.Paint{
			Stroke:   &paint.Stroke{Width: 1, Color: green},
			Gradient: &paint.Gradient{End: geom.Offset{X: 10}, StartColor: red, EndColor: green},
		})
	}
	return c.DisplayList()
}

func TestEncode_SteadyStateDoesNotAllocate(t *testing.T) {
	dl := decoratedList(1000)
	b := &gpu.Batch{}
	Encode(dl, nil, 1, b)
	allocs := testing.AllocsPerRun(10, func() { Encode(dl, nil, 1, b) })
	if allocs != 0 {
		t.Errorf("got %v allocs, want 0", allocs)
	}
}

func BenchmarkEncode_10kRects(b *testing.B) {
	dl := decoratedList(10000)
	batch := &gpu.Batch{}
	Encode(dl, nil, 1, batch)
	b.ReportAllocs()
	for b.Loop() {
		Encode(dl, nil, 1, batch)
	}
}

// drawn encodes one box at the given scale.
func drawn(t *testing.T, draw func(cv *paint.Canvas), scale float32) []gpu.Instance {
	t.Helper()
	var cv paint.Canvas
	cv.Reset()
	draw(&cv)
	var b gpu.Batch
	Encode(cv.DisplayList(), nil, scale, &b)
	return b.Instances
}

func TestSnap_EveryHairlineIsOnePhysicalPixel(t *testing.T) {
	const scale = 1.5
	for _, y := range []float32{10, 10.2, 10.5, 11, 173.2, 353.2, 405} {
		in := drawn(t, func(cv *paint.Canvas) {
			cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: 0.4, Y: y, W: 100, H: 1}}, paint.Paint{Color: geom.Hex(0)})
		}, scale)[0]
		if top := in.Rect.Y * scale; top != geom.Floor32(top) {
			t.Errorf("y=%v: lands on %v", y, top)
		}
		if h := in.Rect.H * scale; h != 1 {
			t.Errorf("y=%v: %v physical pixels high", y, h)
		}
	}
}

func TestSnap_BordersAreAlike(t *testing.T) {
	for _, tt := range []struct{ scale, width, want float32 }{
		{1, 1, 1}, {1.25, 1, 1}, {1.5, 1, 1}, {2, 1, 2}, {1.5, 3, 5}, {1, 0.5, 1},
	} {
		in := drawn(t, func(cv *paint.Canvas) {
			cv.DrawRRect(geom.RRectAll(geom.Rect{W: 50, H: 30}, 8), paint.Paint{Stroke: &paint.Stroke{Width: tt.width, Color: geom.Hex(0)}})
		}, tt.scale)[0]
		if got := in.BorderWidth * tt.scale; got != tt.want {
			t.Errorf("%v at %v: got %v physical pixels, want %v", tt.width, tt.scale, got, tt.want)
		}
	}
}

func TestSnap_LeavesMotionAlone(t *testing.T) {
	box := geom.Rect{X: 10.3, Y: 10.3, W: 50, H: 30}
	scaled := drawn(t, func(cv *paint.Canvas) {
		cv.Scale(0.97)
		cv.DrawRRect(geom.RRectAll(box, 8), paint.Paint{Color: geom.Hex(0), Stroke: &paint.Stroke{Width: 1, Color: geom.Hex(0)}})
	}, 1.5)[0]
	if scaled.Rect.X != 10.3*float32(0.97) {
		t.Errorf("a scaled box must keep moving smoothly, got %v", scaled.Rect.X)
	}
	if got := scaled.BorderWidth * 1.5; got != 1 {
		t.Errorf("its border must not change, got %v", got)
	}
	thumb := drawn(t, func(cv *paint.Canvas) {
		cv.DrawRRect(geom.RRectAll(geom.Rect{X: 10.3, W: 18, H: 18}, 9), paint.Paint{Color: geom.Hex(0)})
	}, 1.5)[0]
	if thumb.Rect.X != 10.3 {
		t.Errorf("a circle must slide freely, got %v", thumb.Rect.X)
	}
}
