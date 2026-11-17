package paint

import (
	"math"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

func nearPoint(a, b geom.Offset) bool {
	return geom.Abs32(a.X-b.X) < 1e-4 && geom.Abs32(a.Y-b.Y) < 1e-4
}

func TestCanvas_TransformScaleTranslateTakesFastPath(t *testing.T) {
	a, b := newCanvas(), newCanvas()
	a.Transform(geom.Translation(geom.Offset{X: 10, Y: 20}).Mul(geom.Scaling(2, 2)))
	b.Translate(10, 20)
	b.Scale(2)
	for _, c := range []*Canvas{a, b} {
		c.DrawRRect(unitRect, Paint{Color: opaqueRed})
	}
	got, want := a.DisplayList().Ops[0], b.DisplayList().Ops[0]
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if !got.Transform.IsIdentity() {
		t.Errorf("a scale and a move must not reach Op.Transform: %+v", got.Transform)
	}
}

func TestCanvas_TransformZeroValueChangesNothing(t *testing.T) {
	c := newCanvas()
	c.Transform(geom.Affine{})
	c.DrawRRect(unitRect, Paint{Color: opaqueRed})
	if op := c.DisplayList().Ops[0]; op.RRect != unitRect || op.Transform != (geom.Affine{}) {
		t.Errorf("got %+v", op)
	}
}

func TestCanvas_RotationGoesIntoOp(t *testing.T) {
	c := newCanvas()
	c.Translate(100, 50)
	c.Transform(geom.Rotation(math.Pi / 2))
	c.DrawRRect(unitRect, Paint{Color: opaqueRed})
	op := c.DisplayList().Ops[0]
	if op.Transform.IsIdentity() {
		t.Fatal("a rotation must reach Op.Transform")
	}
	// The local point (10, 0), placed at (110, 50), turns to (0, 10) and moves
	// by (100, 50).
	if got := op.Transform.Apply(geom.Offset{X: 110, Y: 50}); !nearPoint(got, geom.Offset{X: 100, Y: 60}) {
		t.Errorf("got %v, want (100, 60)", got)
	}
	b := op.Bounds()
	if geom.Abs32(b.X-90) > 1e-4 || geom.Abs32(b.Y-50) > 1e-4 || geom.Abs32(b.W-10) > 1e-4 {
		t.Errorf("bounds %+v, want x=90 y=50 w=10", b)
	}
}

func TestCanvas_TranslateAfterRotationMovesAlongTurnedAxes(t *testing.T) {
	c := newCanvas()
	c.Transform(geom.Rotation(math.Pi / 2))
	c.Translate(10, 0)
	c.DrawRRect(unitRect, Paint{Color: opaqueRed})
	op := c.DisplayList().Ops[0]
	origin := op.Transform.Apply(op.RRect.Rect.Origin())
	if !nearPoint(origin, geom.Offset{Y: 10}) {
		t.Errorf("got %v, want (0, 10)", origin)
	}
}

func TestCanvas_RotationKeepsScaleForGlyphs(t *testing.T) {
	c := newCanvas()
	c.Scale(2)
	c.Transform(geom.Rotation(0.3))
	c.DrawRRect(unitRect, Paint{Color: opaqueRed})
	op := c.DisplayList().Ops[0]
	if op.Scale != 2 {
		t.Errorf("scale %v, want 2: glyphs rasterize at the scale of the fast path", op.Scale)
	}
	// The local corner (10, 10) is (20, 20) after the scale, then turned.
	want := geom.Rotation(0.3).Apply(geom.Offset{X: 20, Y: 20})
	if got := op.Transform.Apply(geom.Offset{X: 20, Y: 20}); !nearPoint(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCanvas_RestoreEndsRotation(t *testing.T) {
	c := newCanvas()
	c.Save()
	c.Transform(geom.Rotation(1))
	c.Restore()
	c.DrawRRect(unitRect, Paint{Color: opaqueRed})
	if op := c.DisplayList().Ops[0]; op.Transform != (geom.Affine{}) {
		t.Errorf("got %+v", op.Transform)
	}
}

func TestCanvas_TurnedClipRectBecomesStencilClip(t *testing.T) {
	c := newCanvas()
	c.Save()
	c.Transform(geom.Rotation(math.Pi / 4))
	c.ClipRect(geom.Rect{W: 10, H: 10})
	c.DrawRRect(unitRect, Paint{Color: opaqueRed})
	c.Restore()
	ops := c.DisplayList().Ops
	if len(ops) != 3 || ops[0].Kind != OpClipPush || ops[2].Kind != OpClipPop {
		t.Fatalf("got %d ops: %v", len(ops), ops)
	}
	if ops[0].Transform.IsIdentity() || ops[2].Transform != ops[0].Transform {
		t.Error("the push and the pop must carry the rotation")
	}
	// The scissor holds the upright bounds of the turned square.
	d := float32(10 / math.Sqrt2)
	if clip := ops[1].Clip; !ops[1].Clipped || geom.Abs32(clip.X+d) > 1e-4 || geom.Abs32(clip.W-2*d) > 1e-4 {
		t.Errorf("clip %+v, want x=%v w=%v", clip, -d, 2*d)
	}
	if ops[1].ClipLevel != 1 {
		t.Errorf("level %d, want 1", ops[1].ClipLevel)
	}
}

func TestCanvas_FlatTransformDrawsNothing(t *testing.T) {
	c := newCanvas()
	c.Save()
	c.Transform(geom.Affine{A: 1, C: 1, B: 1, D: 1})
	c.ClipRect(geom.Rect{W: 10, H: 10})
	c.DrawRRect(unitRect, Paint{Color: opaqueRed})
	c.Restore()
	c.DrawRRect(unitRect, Paint{Color: opaqueRed})
	if ops := c.DisplayList().Ops; len(ops) != 1 {
		t.Errorf("got %d ops, want only the one after Restore", len(ops))
	}
}

func TestCanvas_RotatedShapeOutsideClipIsSkipped(t *testing.T) {
	c := newCanvas()
	c.ClipRect(geom.Rect{W: 10, H: 10})
	c.Translate(100, 0)
	c.Transform(geom.Rotation(math.Pi / 2))
	// Turned by a quarter, the square at (0, 0) lands left of x = 100 but
	// still far right of the clip.
	c.DrawRRect(unitRect, Paint{Color: opaqueRed})
	if n := len(c.DisplayList().Ops); n != 0 {
		t.Errorf("got %d ops, want 0", n)
	}
}

func TestCanvas_TurnedLayerIsAllowed(t *testing.T) {
	var l Layer
	l.Begin().DrawRRect(unitRect, Paint{Color: opaqueRed})
	l.End()
	c := newCanvas()
	c.Transform(geom.Rotation(0.5))
	if !c.CanDrawLayer() {
		t.Fatal("a rotation must not stop layers")
	}
	c.DrawLayer(&l, geom.Offset{})
	if op := c.DisplayList().Ops[0]; op.Kind != OpLayer || op.Transform.IsIdentity() {
		t.Errorf("got %+v", op)
	}
}

func TestLayer_ReachFollowsTransform(t *testing.T) {
	var l Layer
	cv := l.Begin()
	cv.Transform(geom.Rotation(math.Pi / 4))
	cv.DrawRRect(geom.RRect{Rect: geom.Rect{W: 100, H: 100}}, Paint{Color: opaqueRed})
	l.End()
	// The square turns into a diamond reaching from x = -71 to 71.
	if b := l.bounds; b.X > -70 || b.X+b.W < 70 {
		t.Errorf("bounds %+v must hold the turned square", b)
	}
}

func recordTurned(c *Canvas, n int) {
	c.Reset()
	for i := range n {
		x := float32(i % 100)
		rr := geom.RRectAll(geom.Rect{X: x, Y: x, W: 20, H: 20}, 4)
		c.Save()
		c.Translate(1, 1)
		c.Transform(geom.Rotation(0.2))
		c.ClipRect(geom.Rect{W: 200, H: 200})
		c.DrawShadow(rr, 8, opaqueRed, geom.Offset{Y: 2})
		c.DrawRRect(rr, Paint{Color: opaqueRed})
		c.Restore()
	}
}

func TestCanvas_TurnedRecordingDoesNotAllocate(t *testing.T) {
	c := newCanvas()
	recordTurned(c, 100)
	if allocs := testing.AllocsPerRun(10, func() { recordTurned(c, 100) }); allocs != 0 {
		t.Errorf("got %v allocs, want 0", allocs)
	}
}

func BenchmarkCanvas_10kTurnedRects(b *testing.B) {
	c := newCanvas()
	recordTurned(c, 5000)
	b.ReportAllocs()
	for b.Loop() {
		recordTurned(c, 5000)
	}
}
