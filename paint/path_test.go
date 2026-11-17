package paint

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/geom"
)

func TestPath_BoundsAndContours(t *testing.T) {
	p := &Path{}
	p.MoveTo(1, 2)
	p.LineTo(11, 2)
	p.LineTo(11, 22)
	p.Close()
	p.MoveTo(50, 50)
	p.LineTo(60, 60)
	if got, want := p.Bounds(), (geom.Rect{X: 1, Y: 2, W: 59, H: 58}); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	points, starts := p.Polygon()
	if len(starts) != 2 || starts[1] != 3 || len(points) != 5 {
		t.Errorf("got %d points, starts %v", len(points), starts)
	}
	if !p.Closed(0) || p.Closed(1) {
		t.Error("only the first contour was closed")
	}
}

func TestPath_LineAfterCloseStartsAtTheStart(t *testing.T) {
	p := &Path{}
	p.MoveTo(5, 5)
	p.LineTo(10, 5)
	p.Close()
	p.LineTo(5, 20)
	points, starts := p.Polygon()
	if len(starts) != 2 || points[starts[1]] != (geom.Offset{X: 5, Y: 5}) {
		t.Errorf("got %v, starts %v", points, starts)
	}
}

func TestPath_CurvesStayWithinTolerance(t *testing.T) {
	const r = 100
	p := &Path{}
	p.AddRRect(geom.RRectAll(geom.Rect{W: 2 * r, H: 2 * r}, r))
	points, _ := p.Polygon()
	if len(points) < 20 {
		t.Fatalf("a circle of %d points", len(points))
	}
	for _, q := range points {
		dx, dy := q.X-r, q.Y-r
		if d := geom.Sqrt32(dx*dx+dy*dy) - r; d > 0.2 || d < -0.2 {
			t.Fatalf("point %v is %v off the circle", q, d)
		}
	}
}

func TestPath_ChangingFlattensAgain(t *testing.T) {
	p := &Path{}
	p.MoveTo(0, 0)
	p.LineTo(10, 0)
	_ = p.Bounds() // caches
	p.LineTo(10, 30)
	if got := p.Bounds().H; got != 30 {
		t.Errorf("got %v", got)
	}
	p.Reset()
	if !p.Empty() {
		t.Error("want an empty path")
	}
}

func TestPath_Scaled(t *testing.T) {
	p := &Path{Rule: FillEvenOdd}
	p.MoveTo(1, 2)
	p.LineTo(3, 4)
	q := p.Scaled(10)
	if got, want := q.Bounds(), (geom.Rect{X: 10, Y: 20, W: 20, H: 20}); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if q.Rule != FillEvenOdd {
		t.Error("the rule must stay")
	}
	if p.Bounds().X != 1 {
		t.Error("the source must not change")
	}
}

func TestPath_SetCopiesEverything(t *testing.T) {
	p := &Path{Rule: FillEvenOdd}
	p.MoveTo(0, 0)
	p.QuadTo(10, 0, 10, 10)
	p.Close()
	var q Path
	q.MoveTo(5, 5)
	q.Set(p)
	if !q.Equal(p) || q.Hash() != p.Hash() || q.Bounds() != p.Bounds() || !q.Closed(0) {
		t.Fatal("the copy must hold the same path")
	}
	qp, _ := q.Polygon()
	pp, _ := p.Polygon()
	if len(qp) != len(pp) {
		t.Errorf("got %d flattened points, want %d", len(qp), len(pp))
	}
	p.Reset()
	p.LineTo(1, 1)
	if q.Equal(p) || q.Bounds().W != 10 {
		t.Error("the copy must not follow the source")
	}
}

func TestPath_HashFollowsTheContents(t *testing.T) {
	var a, b Path
	a.MoveTo(0, 0)
	a.LineTo(1, 2)
	b.MoveTo(0, 0)
	b.LineTo(1, 2)
	if a.Hash() != b.Hash() || !a.Equal(&b) {
		t.Error("equal paths must hash the same")
	}
	b.LineTo(3, 3)
	if a.Hash() == b.Hash() || a.Equal(&b) {
		t.Error("a changed path must hash differently")
	}
	var z Path
	z.MoveTo(float32(math.Copysign(0, -1)), 0)
	z.LineTo(1, 2)
	if z.Hash() != a.Hash() || !z.Equal(&a) {
		t.Error("-0 equals 0")
	}
}

func TestSVG_Commands(t *testing.T) {
	tests := []struct {
		name string
		d    string
		want geom.Rect
	}{
		{"絶対座標の直線", "M10 10L30 10L30 40Z", geom.Rect{X: 10, Y: 10, W: 20, H: 30}},
		{"相対座標", "m10 10l20 0l0 30z", geom.Rect{X: 10, Y: 10, W: 20, H: 30}},
		{"M の後の数字は直線", "M0 0 10 0 10 10", geom.Rect{W: 10, H: 10}},
		{"H と V", "M1 1H11V21h-5v-5", geom.Rect{X: 1, Y: 1, W: 10, H: 20}},
		{"区切りの省略", "M.5.5L1-1", geom.Rect{X: .5, Y: -1, W: .5, H: 1.5}},
		{"カンマ", "M1,2,L3,4", geom.Rect{X: 1, Y: 2, W: 2, H: 2}},
		{"指数", "M1e1 0L2e1 0", geom.Rect{X: 10, W: 10}},
		{"3 次ベジェ", "M0 0C0 10 10 10 10 0", geom.Rect{W: 10, H: 7.5}},
		{"S は制御点を反射", "M0 0C0 10 10 10 10 0S20 -10 20 0", geom.Rect{Y: -7.5, W: 20, H: 15}},
		{"2 次ベジェ", "M0 0Q5 10 10 0", geom.Rect{W: 10, H: 5}},
		{"T は制御点を反射", "M0 0Q5 10 10 0T20 0", geom.Rect{Y: -5, W: 20, H: 10}},
		{"半円の弧", "M0 0A10 10 0 0 0 20 0", geom.Rect{W: 20, H: 10}},
		{"逆向きの弧", "M0 0A10 10 0 0 1 20 0", geom.Rect{Y: -10, W: 20, H: 10}},
		{"フラグが密着した弧", "M0 0a10 10 0 0020 0", geom.Rect{W: 20, H: 10}},
		{"半径が足りない弧は広がる", "M0 0A1 1 0 0 0 20 0", geom.Rect{W: 20, H: 10}},
		{"半径 0 の弧は直線", "M0 0A0 5 0 0 0 20 0", geom.Rect{W: 20}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := ParseSVGPath(tt.d)
			if err != nil {
				t.Fatal(err)
			}
			got := p.Bounds()
			// Flattening may cut a curve short by its tolerance on each side.
			near := func(a, b float32) bool { return geom.Abs32(a-b) < 2*flattenTolerance }
			if !near(got.X, tt.want.X) || !near(got.Y, tt.want.Y) || !near(got.W, tt.want.W) || !near(got.H, tt.want.H) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSVG_Errors(t *testing.T) {
	for _, d := range []string{"10 10", "M1", "M1 1 Q1", "X1 1", "M1 1A1 1 0 2 0 1 1"} {
		if _, err := ParseSVGPath(d); err == nil {
			t.Errorf("%q: want an error", d)
		}
	}
}

func TestImage_PremultipliesAndDecodes(t *testing.T) {
	m := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	m.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 128})
	m.SetNRGBA(1, 0, color.NRGBA{G: 255, A: 255})
	var file bytes.Buffer
	if err := png.Encode(&file, m); err != nil {
		t.Fatal(err)
	}
	img, err := DecodeImage(&file)
	if err != nil {
		t.Fatal(err)
	}
	if img.Size() != (geom.Size{W: 2, H: 1}) {
		t.Errorf("got %v", img.Size())
	}
	if got := img.At(0, 0); got.A != 128 || got.R != 128 {
		t.Errorf("got %v, want premultiplied red", got)
	}
	if _, err := DecodeImage(bytes.NewReader([]byte("no image"))); err == nil {
		t.Error("want an error")
	}
}

func TestCanvas_ShapedClipPushesAndRestorePops(t *testing.T) {
	c := newCanvas()
	c.Translate(10, 10)
	c.Save()
	c.ClipRRect(geom.RRectAll(geom.Rect{W: 100, H: 100}, 8))
	c.DrawRRect(unitRect, Paint{Color: opaqueRed})
	c.Save()
	c.ClipRRect(geom.RRectAll(geom.Rect{X: 20, Y: 20, W: 20, H: 20}, 4))
	c.DrawRRect(geom.RRect{Rect: geom.Rect{X: 25, Y: 25, W: 5, H: 5}}, Paint{Color: opaqueRed})
	c.Restore()
	c.Restore()
	c.DrawRRect(unitRect, Paint{Color: opaqueRed})
	kinds := []OpKind{OpClipPush, OpRRect, OpClipPush, OpRRect, OpClipPop, OpClipPop, OpRRect}
	levels := []uint8{0, 1, 1, 2, 1, 0, 0}
	ops := c.DisplayList().Ops
	if len(ops) != len(kinds) {
		t.Fatalf("got %d ops", len(ops))
	}
	for i, op := range ops {
		if op.Kind != kinds[i] || op.ClipLevel != levels[i] {
			t.Errorf("op %d: got %v level %d, want %v level %d", i, op.Kind, op.ClipLevel, kinds[i], levels[i])
		}
	}
	if got, want := ops[0].RRect.Rect, (geom.Rect{X: 10, Y: 10, W: 100, H: 100}); got != want {
		t.Errorf("bounds: got %v, want %v", got, want)
	}
	if !ops[1].Clipped || ops[1].Clip != ops[0].RRect.Rect {
		t.Error("a shaped clip also limits the scissor")
	}
}

func TestCanvas_RRectWithoutCornersStaysAScissor(t *testing.T) {
	c := newCanvas()
	c.ClipRRect(geom.RRect{Rect: geom.Rect{W: 10, H: 10}})
	c.DrawRRect(unitRect, Paint{Color: opaqueRed})
	if ops := c.DisplayList().Ops; len(ops) != 1 || ops[0].ClipLevel != 0 {
		t.Errorf("got %+v", ops)
	}
}

func TestCanvas_ClipsDoNotAllocateOnceWarm(t *testing.T) {
	c := newCanvas()
	frame := func() {
		c.Reset()
		c.Save()
		c.ClipRRect(geom.RRectAll(geom.Rect{W: 50, H: 50}, 8))
		c.DrawRRect(unitRect, Paint{Color: opaqueRed})
		c.Restore()
	}
	frame()
	if got := testing.AllocsPerRun(20, frame); got != 0 {
		t.Errorf("got %v allocs", got)
	}
}

func TestCanvas_DrawPathCullsAndFades(t *testing.T) {
	p := &Path{}
	p.MoveTo(0, 0)
	p.LineTo(10, 0)
	p.LineTo(10, 10)
	p.Close()
	c := newCanvas()
	c.ClipRect(geom.Rect{X: 100, Y: 100, W: 10, H: 10})
	c.DrawPath(p, Paint{Color: opaqueRed})
	if n := len(c.DisplayList().Ops); n != 0 {
		t.Errorf("a path outside the clip must go: %d ops", n)
	}
	c = newCanvas()
	c.PushOpacity(0.5)
	c.Translate(3, 4)
	c.DrawPath(p, Paint{Color: opaqueRed, Stroke: &Stroke{Width: 2, Color: opaqueRed}})
	op := c.DisplayList().Ops[0]
	if op.Color.A != 0.5 || op.Stroke.Color.A != 0.5 {
		t.Errorf("got %+v", op)
	}
	if got, want := op.RRect.Rect, (geom.Rect{X: 2, Y: 3, W: 12, H: 12}); got != want {
		t.Errorf("a stroke widens the bounds: got %v, want %v", got, want)
	}
	if op.Origin != (geom.Offset{X: 3, Y: 4}) {
		t.Errorf("got %v", op.Origin)
	}
}

// A close takes no numbers, so a number after it must be an error; it used to
// repeat the close without consuming input and never return.
func TestParseSVGPath_NumberAfterCloseIsAnError(t *testing.T) {
	for _, d := range []string{"M0 0Z0", "M0 0z0", "M0 0L1 1Z 2 2", "M0 0Z-1"} {
		done := make(chan error, 1)
		go func() {
			_, err := ParseSVGPath(d)
			done <- err
		}()
		select {
		case err := <-done:
			if err == nil {
				t.Errorf("ParseSVGPath(%q) returned no error", d)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("ParseSVGPath(%q) did not return", d)
		}
	}
	for _, d := range []string{"M0 0L1 1ZM2 2L3 3z", "M0 0L1 1Z", "M0 0L1 1z ", "M0 0ZZ", "M0 0L1 1ZL2 2"} {
		if _, err := ParseSVGPath(d); err != nil {
			t.Errorf("ParseSVGPath(%q): %v", d, err)
		}
	}
}
