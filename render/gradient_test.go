package render

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
)

func TestEncode_TwoColorGradientStops(t *testing.T) {
	b := record(func(c *paint.Canvas) {
		c.DrawRRect(box, paint.Paint{Gradient: &paint.Gradient{End: geom.Offset{X: 7}, StartColor: red, EndColor: green}})
	})
	in := b.Instances[0]
	if in.Color != red || in.MidColors != [2]geom.Color{red, red} || in.EndColor != green {
		t.Errorf("colors: got %+v", in)
	}
	if want := [4]float32{0, 0, 0, 1}; in.StopOffsets != want {
		t.Errorf("offsets: got %v, want %v", in.StopOffsets, want)
	}
}

func TestEncode_GradientStops(t *testing.T) {
	blue := geom.Hex(0x0000ff)
	white := geom.Hex(0xffffff)
	tests := []struct {
		name    string
		stops   []paint.GradientStop
		colors  [4]geom.Color
		offsets [4]float32
	}{
		{
			"3 つ",
			[]paint.GradientStop{{Offset: 0, Color: red}, {Offset: 0.4, Color: green}, {Offset: 1, Color: blue}},
			[4]geom.Color{red, red, green, blue}, [4]float32{0, 0, 0.4, 1},
		},
		{
			"4 つ",
			[]paint.GradientStop{{Offset: 0.1, Color: red}, {Offset: 0.2, Color: green}, {Offset: 0.3, Color: blue}, {Offset: 0.9, Color: white}},
			[4]geom.Color{red, green, blue, white}, [4]float32{0.1, 0.2, 0.3, 0.9},
		},
		{
			"逆順のオフセットは前に揃う",
			[]paint.GradientStop{{Offset: 0.5, Color: red}, {Offset: 0.2, Color: green}},
			[4]geom.Color{red, red, red, green}, [4]float32{0.5, 0.5, 0.5, 0.5},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &paint.Gradient{Kind: paint.GradientRadial, End: geom.Offset{X: 7}, StopCount: len(tt.stops)}
			copy(g.Stops[:], tt.stops)
			b := record(func(c *paint.Canvas) { c.DrawRRect(box, paint.Paint{Gradient: g}) })
			in := b.Instances[0]
			got := [4]geom.Color{in.Color, in.MidColors[0], in.MidColors[1], in.EndColor}
			if in.Gradient != gpu.GradientRadial || got != tt.colors {
				t.Errorf("colors: got %v, want %v", got, tt.colors)
			}
			if in.StopOffsets != tt.offsets {
				t.Errorf("offsets: got %v, want %v", in.StopOffsets, tt.offsets)
			}
		})
	}
}

func TestEncode_PathGradientStops(t *testing.T) {
	var p paint.Path
	p.MoveTo(0, 0)
	p.LineTo(100, 0)
	p.LineTo(100, 100)
	p.Close()
	g := &paint.Gradient{End: geom.Offset{X: 100}, StopCount: 3}
	g.Stops[0].Color, g.Stops[1].Color, g.Stops[2].Color = red, green, red
	g.Stops[1].Offset, g.Stops[2].Offset = 0.5, 1
	b := record(func(c *paint.Canvas) { c.DrawPath(&p, paint.Paint{Gradient: g}) })
	in := b.Instances[len(b.Instances)-1]
	if in.Kind != gpu.InstanceCover || in.MidColors[1] != green || in.StopOffsets[2] != 0.5 {
		t.Errorf("got %+v", in)
	}
}

func TestEncode_GradientStopsDoNotAllocate(t *testing.T) {
	var c paint.Canvas
	c.Reset()
	g := &paint.Gradient{End: geom.Offset{X: 10}, StopCount: 4}
	for i := range g.Stops {
		g.Stops[i] = paint.GradientStop{Offset: float32(i) / 3, Color: red}
	}
	for range 1000 {
		c.DrawRRect(box, paint.Paint{Gradient: g})
	}
	b := &gpu.Batch{}
	Encode(c.DisplayList(), nil, 1, b)
	allocs := testing.AllocsPerRun(10, func() { Encode(c.DisplayList(), nil, 1, b) })
	if allocs != 0 {
		t.Errorf("got %v allocs, want 0", allocs)
	}
}

func BenchmarkEncode_10kFourStopGradients(b *testing.B) {
	var c paint.Canvas
	c.Reset()
	g := &paint.Gradient{End: geom.Offset{X: 10}, StopCount: 4}
	for i := range g.Stops {
		g.Stops[i] = paint.GradientStop{Offset: float32(i) / 3, Color: green}
	}
	for range 10000 {
		c.DrawRRect(box, paint.Paint{Gradient: g})
	}
	batch := &gpu.Batch{}
	Encode(c.DisplayList(), nil, 1, batch)
	b.ReportAllocs()
	for b.Loop() {
		Encode(c.DisplayList(), nil, 1, batch)
	}
}
