package paint

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

func threeStops() *Gradient {
	return &Gradient{
		End: geom.Offset{X: 10},
		Stops: [MaxGradientStops]GradientStop{
			{Offset: 0, Color: geom.Hex(0x7b2ff7)},
			{Offset: 0.5, Color: geom.Hex(0xf107a3)},
			{Offset: 1, Color: geom.Hex(0xff8a00)},
		},
		StopCount: 3,
	}
}

func TestCanvas_GradientStopsFade(t *testing.T) {
	c := newCanvas()
	c.PushOpacity(0.5)
	g := threeStops()
	c.DrawRRect(unitRect, Paint{Gradient: g})
	op := c.DisplayList().Ops[0]
	if !op.HasGradient || op.Gradient.StopCount != 3 {
		t.Fatalf("got %+v", op.Gradient)
	}
	for i := range 3 {
		got, want := op.Gradient.Stops[i], g.Stops[i]
		if got.Offset != want.Offset || got.Color.A != 0.5 || got.Color.R != want.Color.R {
			t.Errorf("stop %d: got %+v, want %+v at half alpha", i, got, want)
		}
	}
	if g.Stops[0].Color.A != 1 {
		t.Error("drawing changed the caller's gradient")
	}
}

func TestCanvas_GradientOffsetsNeverDecrease(t *testing.T) {
	c := newCanvas()
	g := threeStops()
	g.Stops[1].Offset = -1
	g.Stops[2].Offset = 0.25
	g.Stops[0].Offset = 0.5
	c.DrawRRect(unitRect, Paint{Gradient: g})
	got := c.DisplayList().Ops[0].Gradient.Stops
	for i, want := range []float32{0.5, 0.5, 0.5} {
		if got[i].Offset != want {
			t.Errorf("stop %d: got offset %v, want %v", i, got[i].Offset, want)
		}
	}
}

func TestCanvas_GradientVisibility(t *testing.T) {
	clear := threeStops()
	for i := range clear.Stops {
		clear.Stops[i].Color = geom.Color{}
	}
	// StartColor is not a stop when StopCount is set.
	clear.StartColor = opaqueRed
	oneShows := threeStops()
	for i := range 2 {
		oneShows.Stops[i].Color = geom.Color{}
	}
	tests := []struct {
		name string
		g    *Gradient
		want bool
	}{
		{"透明なストップだけ", clear, false},
		{"最後のストップだけ見える", oneShows, true},
		{"2 色の指定", &Gradient{EndColor: opaqueRed}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCanvas()
			c.DrawRRect(unitRect, Paint{Gradient: tt.g})
			if got := len(c.DisplayList().Ops) == 1; got != tt.want {
				t.Errorf("drawn: got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCanvas_TooManyGradientStopsPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("no panic")
		}
	}()
	g := threeStops()
	g.StopCount = MaxGradientStops + 1
	newCanvas().DrawRRect(unitRect, Paint{Gradient: g})
}

func TestGradient_Comparable(t *testing.T) {
	// layout compares decorations with ==, which a slice of stops would break.
	a, b := *threeStops(), *threeStops()
	if a != b {
		t.Error("equal gradients compare unequal")
	}
	b.Stops[1].Offset = 0.6
	if a == b {
		t.Error("different stops compare equal")
	}
}

func TestCanvas_MoreStopsAreCopied(t *testing.T) {
	c := newCanvas()
	c.PushOpacity(0.5)
	stops := make([]GradientStop, 20)
	for i := range stops {
		stops[i] = GradientStop{Offset: float32(i) / 19, Color: opaqueRed}
	}
	stops[7].Offset = 0 // moves up to the stop before it
	g := &Gradient{End: geom.Offset{X: 10}, MoreStops: &stops}
	c.DrawRRect(unitRect, Paint{Gradient: g})
	c.DrawRRect(unitRect, Paint{Gradient: g})
	ops := c.DisplayList().Ops
	if len(ops) != 2 {
		t.Fatalf("got %d ops, want 2", len(ops))
	}
	got := *ops[0].Gradient.MoreStops
	if len(got) != 20 || got[19].Color.A != 0.5 || got[7].Offset != got[6].Offset {
		t.Errorf("got %+v", got)
	}
	if ops[0].Gradient.MoreStops == g.MoreStops || stops[0].Color.A != 1 || stops[7].Offset != 0 {
		t.Error("drawing changed or kept the caller's stops")
	}
	// The caller may reuse its slice once drawn.
	stops[0].Color = geom.Color{}
	if (*ops[1].Gradient.MoreStops)[0].Color.A != 0.5 {
		t.Error("the copy follows the caller's slice")
	}
}

func TestCanvas_MoreStopsVisibility(t *testing.T) {
	clear := []GradientStop{{Offset: 0}, {Offset: 1}}
	c := newCanvas()
	// StopCount and StartColor are ignored once MoreStops has stops.
	c.DrawRRect(unitRect, Paint{Gradient: &Gradient{StartColor: opaqueRed, MoreStops: &clear}})
	if n := len(c.DisplayList().Ops); n != 0 {
		t.Errorf("got %d ops of clear stops, want 0", n)
	}
	empty := []GradientStop{}
	c.DrawRRect(unitRect, Paint{Gradient: &Gradient{StartColor: opaqueRed, MoreStops: &empty}})
	if n := len(c.DisplayList().Ops); n != 1 {
		t.Errorf("got %d ops with empty MoreStops, want the two colors", n)
	}
}

func TestCanvas_MoreStopsDoNotAllocate(t *testing.T) {
	c := newCanvas()
	stops := make([]GradientStop, 8)
	g := &Gradient{End: geom.Offset{X: 10}, MoreStops: &stops}
	draw := func() {
		c.Reset()
		for range 100 {
			c.DrawRRect(unitRect, Paint{Gradient: g, Color: opaqueRed})
		}
	}
	draw()
	if allocs := testing.AllocsPerRun(10, draw); allocs != 0 {
		t.Errorf("got %v allocs, want 0", allocs)
	}
}

func TestCanvas_GradientTransformMovesWithTheShape(t *testing.T) {
	c := newCanvas()
	c.Translate(100, 50)
	c.Scale(2)
	g := threeStops()
	g.Kind = GradientRadial
	g.Start, g.End = geom.Offset{X: 5, Y: 5}, geom.Offset{X: 15, Y: 5}
	// Twice as wide, about the start.
	g.Transform = geom.Translation(g.Start).Mul(geom.Scaling(2, 1)).Mul(geom.Translation(geom.Offset{X: -5, Y: -5}))
	c.DrawRRect(unitRect, Paint{Gradient: g})
	got := c.DisplayList().Ops[0].Gradient
	// A point of the gradient space, placed, lands where the transformed point
	// lands when placed.
	p := geom.Offset{X: 7, Y: 9}
	place := func(q geom.Offset) geom.Offset { return q.Scale(2).Add(geom.Offset{X: 100, Y: 50}) }
	want := place(g.Transform.Apply(p))
	if have := got.Transform.Apply(place(p)); geom.Abs32(have.X-want.X) > 1e-4 || geom.Abs32(have.Y-want.Y) > 1e-4 {
		t.Errorf("got %v, want %v", have, want)
	}
	if got.Start != place(g.Start) || got.End != place(g.End) {
		t.Errorf("control points: got %v and %v", got.Start, got.End)
	}
}

func TestCanvas_NoGradientTransformStaysZero(t *testing.T) {
	c := newCanvas()
	c.Translate(10, 10)
	c.DrawRRect(unitRect, Paint{Gradient: threeStops()})
	if got := c.DisplayList().Ops[0].Gradient.Transform; got != (geom.Affine{}) {
		t.Errorf("got %+v, want the zero value", got)
	}
}
