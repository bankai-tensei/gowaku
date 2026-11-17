package render

import (
	"image"
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
)

// manyStops returns a gradient of n stops in MoreStops, whose colors depend on
// seed, so that gradients of different seeds need different rows.
func manyStops(n, seed int) *paint.Gradient {
	stops := make([]paint.GradientStop, n)
	for i := range stops {
		stops[i] = paint.GradientStop{
			Offset: float32(i) / float32(n-1),
			Color:  geom.Color{R: float32(seed%256) / 255, G: float32(i) / 16, B: float32(seed/256) / 255, A: 1},
		}
	}
	return &paint.Gradient{End: geom.Offset{X: 10}, MoreStops: &stops}
}

// encodeGradients encodes a box per gradient with f's resources and finishes
// the batch.
func (f *fixture) encodeGradients(t *testing.T, gradients ...*paint.Gradient) {
	t.Helper()
	f.canvas.Reset()
	for _, g := range gradients {
		f.canvas.DrawRRect(box, paint.Paint{Gradient: g})
	}
	Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
	if err := f.glyphs.Finish(&f.batch); err != nil {
		t.Fatal(err)
	}
}

func TestEncode_ManyStopsTakeARampRow(t *testing.T) {
	f := newFixture(t, "")
	a, b := manyStops(6, 1), manyStops(16, 2)
	f.encodeGradients(t, a, b, manyStops(6, 1))
	rows := make([]float32, 0, 3)
	for _, in := range f.batch.Instances {
		if in.Gradient != gpu.GradientLinear|gpu.GradientRamp {
			t.Fatalf("mode: got %d, want linear from the ramps", in.Gradient)
		}
		rows = append(rows, in.StopOffsets[0])
	}
	if rows[0] != 0 || rows[1] != 1 || rows[2] != 0 {
		t.Errorf("rows: got %v, want 0, 1 and the first again", rows)
	}
	if f.batch.Ramps == nil {
		t.Error("the batch has no ramps")
	}
	if n := len(f.store.updates); n != 1 || f.store.updates[0] != image.Rect(0, 0, gpu.RampWidth, 2) {
		t.Errorf("uploads: got %v, want the first two rows", f.store.updates)
	}

	// The same stops next frame need neither a row nor an upload.
	f.store.updates = nil
	f.encodeGradients(t, b)
	if got := f.batch.Instances[0].StopOffsets[0]; got != 1 || len(f.store.updates) != 0 {
		t.Errorf("row %v, uploads %v: want row 1 and no upload", got, f.store.updates)
	}
}

func TestEncode_FourStopsStayInTheInstance(t *testing.T) {
	f := newFixture(t, "")
	f.encodeGradients(t, manyStops(4, 1))
	in := f.batch.Instances[0]
	if in.Gradient != gpu.GradientLinear || in.StopOffsets != [4]float32{0, 1.0 / 3, 2.0 / 3, 1} {
		t.Errorf("got %+v", in)
	}
	if f.glyphs.ramps != nil {
		t.Error("four stops made a ramp table")
	}
}

func TestEncode_RampRowsMakeWayForANewFrame(t *testing.T) {
	f := newFixture(t, "")
	first := make([]*paint.Gradient, rampRows)
	for i := range first {
		first[i] = manyStops(5, i)
	}
	f.encodeGradients(t, first...)
	// The rows of the frame before can go, oldest first.
	f.encodeGradients(t, first[7], manyStops(5, 9999))
	if got := f.batch.Instances[1]; got.Gradient&gpu.GradientRamp == 0 || got.StopOffsets[0] != 0 {
		t.Errorf("got %+v, want row 0, the oldest", got)
	}
}

func TestEncode_FullRampsFallBackToFourStops(t *testing.T) {
	f := newFixture(t, "")
	gradients := make([]*paint.Gradient, rampRows+1)
	for i := range gradients {
		gradients[i] = manyStops(7, i)
	}
	f.encodeGradients(t, gradients...)
	in := f.batch.Instances[rampRows]
	if in.Gradient != gpu.GradientLinear {
		t.Fatalf("mode: got %d, want linear without the ramps", in.Gradient)
	}
	s := *gradients[rampRows].MoreStops
	// The first, the last and two between them.
	want := [4]geom.Color{s[0].Color, s[2].Color, s[4].Color, s[6].Color}
	if got := [4]geom.Color{in.Color, in.MidColors[0], in.MidColors[1], in.EndColor}; got != want {
		t.Errorf("colors: got %v, want %v", got, want)
	}
}

func TestEncode_ManyStopsWithoutResources(t *testing.T) {
	b := record(func(c *paint.Canvas) { c.DrawRRect(box, paint.Paint{Gradient: manyStops(16, 3)}) })
	in := b.Instances[0]
	if in.Gradient != gpu.GradientLinear || in.StopOffsets != [4]float32{0, 5.0 / 15, 10.0 / 15, 1} {
		t.Errorf("got %+v", in)
	}
}

func TestEncode_Sweep(t *testing.T) {
	g := &paint.Gradient{Kind: paint.GradientSweep, Start: geom.Offset{X: 5, Y: 5}, End: geom.Offset{X: 10, Y: 5}, EndColor: red}
	in := record(func(c *paint.Canvas) { c.DrawRRect(box, paint.Paint{Gradient: g}) }).Instances[0]
	if in.Gradient != gpu.GradientSweep || in.GradientStart != g.Start || in.GradientEnd != g.End {
		t.Errorf("got %+v", in)
	}
}

func TestEncode_GradientTransform(t *testing.T) {
	g := &paint.Gradient{
		Kind:  paint.GradientRadial,
		End:   geom.Offset{X: 1},
		Stops: [paint.MaxGradientStops]paint.GradientStop{{Color: red}, {Offset: 1, Color: green}},
		// A unit circle, 4 wide and 2 tall, centered at 5, 5.
		Transform: geom.Translation(geom.Offset{X: 5, Y: 5}).Mul(geom.Scaling(4, 2)),
		StopCount: 2,
	}
	in := record(func(c *paint.Canvas) { c.DrawRRect(box, paint.Paint{Gradient: g}) }).Instances[0]
	if in.GradientStart != (geom.Offset{X: 5, Y: 5}) || in.GradientEnd != (geom.Offset{X: 6, Y: 5}) {
		t.Errorf("control points: got %v and %v", in.GradientStart, in.GradientEnd)
	}
	if want := [4]float32{0.25, 0, 0, 0.5}; in.GradientMatrix != want {
		t.Errorf("matrix: got %v, want %v", in.GradientMatrix, want)
	}
}

func TestEncode_FlatGradientTransformIsLeftOut(t *testing.T) {
	g := &paint.Gradient{End: geom.Offset{X: 10}, EndColor: red, Transform: geom.Scaling(0, 1)}
	in := record(func(c *paint.Canvas) { c.DrawRRect(box, paint.Paint{Gradient: g}) }).Instances[0]
	if in.GradientMatrix != [4]float32{} || in.GradientEnd != g.End {
		t.Errorf("got %+v", in)
	}
}

func TestEncode_RampsDoNotAllocate(t *testing.T) {
	f := newFixture(t, "")
	f.canvas.Reset()
	for i := range 1000 {
		f.canvas.DrawRRect(box, paint.Paint{Gradient: manyStops(16, i%50)})
	}
	dl := f.canvas.DisplayList()
	Encode(dl, f.glyphs, 1, &f.batch)
	if err := f.glyphs.Finish(&f.batch); err != nil {
		t.Fatal(err)
	}
	allocs := testing.AllocsPerRun(10, func() {
		Encode(dl, f.glyphs, 1, &f.batch)
		if err := f.glyphs.Finish(&f.batch); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Errorf("got %v allocs, want 0", allocs)
	}
}

func BenchmarkEncode_10kSixteenStopGradients(b *testing.B) {
	f := newFixture(b, "")
	f.canvas.Reset()
	g := manyStops(16, 1)
	for range 10000 {
		f.canvas.DrawRRect(box, paint.Paint{Gradient: g})
	}
	dl := f.canvas.DisplayList()
	Encode(dl, f.glyphs, 1, &f.batch)
	if err := f.glyphs.Finish(&f.batch); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		Encode(dl, f.glyphs, 1, &f.batch)
	}
}

func TestEncode_FewMoreStopsStayInTheInstance(t *testing.T) {
	f := newFixture(t, "")
	f.encodeGradients(t, manyStops(3, 1))
	in := f.batch.Instances[0]
	if in.Gradient != gpu.GradientLinear || in.StopOffsets != [4]float32{0, 0, 0.5, 1} {
		t.Errorf("got %+v", in)
	}
	if f.glyphs.ramps != nil {
		t.Error("three stops made a ramp table")
	}
}

func TestEncode_LongStopListsShareRows(t *testing.T) {
	f := newFixture(t, "")
	f.encodeGradients(t, manyStops(40, 1), manyStops(40, 2), manyStops(40, 1), manyStops(39, 1))
	var rows []float32
	for _, in := range f.batch.Instances {
		rows = append(rows, in.StopOffsets[0])
	}
	if want := []float32{0, 1, 0, 2}; !slices.Equal(rows, want) {
		t.Errorf("rows: got %v, want %v", rows, want)
	}
}
