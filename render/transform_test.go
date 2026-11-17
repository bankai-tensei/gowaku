package render

import (
	"math"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
)

func nearOffset(a, b geom.Offset) bool {
	return geom.Abs32(a.X-b.X) < 1e-3 && geom.Abs32(a.Y-b.Y) < 1e-3
}

func TestEncode_UprightKeepsZeroTransform(t *testing.T) {
	b := record(func(c *paint.Canvas) {
		c.Transform(geom.Translation(geom.Offset{X: 3}))
		c.DrawRRect(box, paint.Paint{Color: red})
	})
	if _, ok := b.Instances[0].Affine(); ok {
		t.Errorf("an upright box must not carry a transform: %v", b.Instances[0].Transform)
	}
}

func TestEncode_TurnedRRectKeepsLocalRect(t *testing.T) {
	b := record(func(c *paint.Canvas) {
		c.Translate(50, 50)
		c.Transform(geom.Rotation(0.3))
		c.DrawRRect(geom.RRect{Rect: geom.Rect{X: 0.3, Y: 0.3, W: 10.4, H: 10}}, paint.Paint{
			Color: red, Stroke: &paint.Stroke{Width: 0.7, Color: green},
		})
	})
	in := b.Instances[0]
	m, ok := in.Affine()
	if !ok {
		t.Fatal("a turned box must carry its transform")
	}
	// Turned boxes are not snapped to the pixel grid.
	if in.Rect != (geom.Rect{X: 50.3, Y: 50.3, W: 10.4, H: 10}) || in.BorderWidth != 0.7 {
		t.Errorf("got rect %+v border %v", in.Rect, in.BorderWidth)
	}
	// The placed origin (50, 50) is the pivot, which stays.
	if got := m.Apply(geom.Offset{X: 50, Y: 50}); !nearOffset(got, geom.Offset{X: 50, Y: 50}) {
		t.Errorf("pivot moved to %v", got)
	}
}

func TestEncode_TurnedPathTransformsVertices(t *testing.T) {
	f := newFixture(t, "")
	f.canvas.Reset()
	f.canvas.Translate(100, 0)
	f.canvas.Transform(geom.Rotation(math.Pi / 2))
	// Small enough for the atlas, which a turned path must not use.
	f.canvas.DrawPath(triangle(20), paint.Paint{Color: red})
	Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
	if len(f.batch.Draws) != 1 || f.batch.Draws[0].Kind != gpu.DrawFill {
		t.Fatalf("got %+v", f.batch.Draws)
	}
	// The local corner (20, 0) turns to (0, 20) around the origin at x = 100.
	if got := f.batch.Vertices[1]; !nearOffset(got, geom.Offset{X: 100, Y: 20}) {
		t.Errorf("vertex %v, want (100, 20)", got)
	}
	cover := f.batch.Instances[f.batch.Draws[0].First]
	if _, ok := cover.Affine(); !ok || cover.Kind != gpu.InstanceCover {
		t.Errorf("the cover must turn with the path: %+v", cover)
	}
}

func TestEncode_TurnedStrokeKeepsItsWidthInLocalSpace(t *testing.T) {
	b := record(func(c *paint.Canvas) {
		c.Transform(geom.Scaling(1, 3))
		p := &paint.Path{}
		p.MoveTo(0, 0)
		p.LineTo(100, 0)
		c.DrawPath(p, paint.Paint{Stroke: &paint.Stroke{Width: 2, Color: red}})
	})
	// A horizontal line two pixels wide, stretched three times vertically, is
	// six pixels tall.
	top, bottom := float32(math.MaxFloat32), float32(-math.MaxFloat32)
	for _, v := range b.Vertices {
		top, bottom = min(top, v.Y), max(bottom, v.Y)
	}
	if geom.Abs32(bottom-top-6) > 1e-3 {
		t.Errorf("height %v, want 6", bottom-top)
	}
}

func TestEncode_TurnedClipPushAndPopShareTheirCover(t *testing.T) {
	b := record(func(c *paint.Canvas) {
		c.Save()
		c.Transform(geom.Rotation(0.4))
		c.ClipRect(geom.Rect{W: 30, H: 30})
		c.DrawRRect(box, paint.Paint{Color: red})
		c.Restore()
	})
	if len(b.Draws) != 3 || b.Draws[0].Kind != gpu.DrawClipPush || b.Draws[2].Kind != gpu.DrawClipPop {
		t.Fatalf("got %+v", b.Draws)
	}
	push, pop := b.Instances[b.Draws[0].First], b.Instances[b.Draws[2].First]
	if push != pop {
		t.Errorf("push %+v and pop %+v must cover the same pixels", push, pop)
	}
	if _, ok := push.Affine(); !ok {
		t.Error("the clip cover must turn")
	}
	if b.Draws[0].VertexCount == 0 {
		t.Error("the turned rectangle must mark the stencil")
	}
}

func TestEncode_TurnedGlyphsCarryTransform(t *testing.T) {
	f := newFixture(t, "gowaku")
	f.canvas.Reset()
	f.canvas.Transform(geom.Rotation(0.2))
	runs := f.layout.Runs()
	for i := range runs {
		f.canvas.DrawGlyphs(&runs[i], geom.Offset{Y: 20}, red)
	}
	Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
	if len(f.batch.Instances) == 0 {
		t.Fatal("no glyphs")
	}
	for _, in := range f.batch.Instances {
		if _, ok := in.Affine(); !ok {
			t.Fatalf("glyph %+v without transform", in)
		}
	}
}

func TestEncode_TurnedLayerDrawsItsPixelsTurned(t *testing.T) {
	f := newLayerFixture(t)
	var l paint.Layer
	filled(&l, 100, 20)
	draw := func(cv *paint.Canvas) {
		cv.Transform(geom.Rotation(0.5))
		cv.DrawLayer(&l, geom.Offset{X: 10, Y: 30})
	}
	for range layerCalmFrames + 1 {
		f.frame(t, 1, draw)
	}
	if len(f.batch.Instances) != 1 {
		t.Fatalf("got %d instances, want the one image of the page", len(f.batch.Instances))
	}
	in := f.batch.Instances[0]
	if _, ok := in.Affine(); !ok || in.Kind != gpu.InstanceImage {
		t.Errorf("got %+v", in)
	}
}

func TestAppendMoved_ComposesTransforms(t *testing.T) {
	var src, dst gpu.Batch
	in := gpu.Instance{Rect: geom.Rect{W: 10, H: 10}}
	src.Instances = append(src.Instances, in)
	src.Vertices = append(src.Vertices, geom.Offset{X: 1})
	src.Draws = append(src.Draws, gpu.DrawCall{Count: 1})
	m := geom.Rotation(math.Pi / 2)
	appendMoved(&dst, &src, geom.Offset{X: 5}, m, geom.Rect{}, false, 0)
	got, ok := dst.Instances[0].Affine()
	if !ok || dst.Instances[0].Rect != in.Rect {
		t.Fatalf("got %+v", dst.Instances[0])
	}
	// Moved by (5, 0), then turned: (0, 0) lands on (0, 5).
	if p := got.Apply(geom.Offset{}); !nearOffset(p, geom.Offset{Y: 5}) {
		t.Errorf("origin at %v, want (0, 5)", p)
	}
	if v := dst.Vertices[0]; !nearOffset(v, geom.Offset{Y: 6}) {
		t.Errorf("vertex at %v, want (0, 6)", v)
	}
}

func TestReachOf_HoldsTheTurnedQuad(t *testing.T) {
	in := gpu.Instance{Rect: geom.Rect{W: 10, H: 10}}
	in.SetTransform(geom.Rotation(math.Pi / 4))
	r := reachOf(&in, 1)
	d := float32(10 / math.Sqrt2)
	if r.X > -d || r.X+r.W < d || r.H < 2*d {
		t.Errorf("reach %+v must hold the diamond of half width %v", r, d)
	}
}

func turnedList(n int) *paint.DisplayList {
	var c paint.Canvas
	c.Reset()
	for i := range n {
		x := float32(i % 100)
		c.Save()
		c.Translate(x, x)
		c.Transform(geom.Rotation(0.3))
		c.DrawShadow(box, 8, red, geom.Offset{Y: 2})
		c.DrawRRect(box, paint.Paint{Color: red, Stroke: &paint.Stroke{Width: 1, Color: green}})
		c.Restore()
	}
	return c.DisplayList()
}

func TestEncode_TurnedSteadyStateDoesNotAllocate(t *testing.T) {
	dl := turnedList(100)
	var b gpu.Batch
	Encode(dl, nil, 1, &b)
	if allocs := testing.AllocsPerRun(10, func() { Encode(dl, nil, 1, &b) }); allocs != 0 {
		t.Errorf("got %v allocs, want 0", allocs)
	}
}

func BenchmarkEncode_10kTurnedRects(b *testing.B) {
	dl := turnedList(5000)
	var batch gpu.Batch
	Encode(dl, nil, 1, &batch)
	b.ReportAllocs()
	for b.Loop() {
		Encode(dl, nil, 1, &batch)
	}
}
