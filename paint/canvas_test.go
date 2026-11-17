package paint

import (
	"image"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

var (
	opaqueRed = geom.Hex(0xff0000)
	unitRect  = geom.RRect{Rect: geom.Rect{W: 10, H: 10}}
)

func newCanvas() *Canvas {
	c := &Canvas{}
	c.Reset()
	return c
}

func TestCanvas_DrawRectIsASquareRRect(t *testing.T) {
	a, b := newCanvas(), newCanvas()
	a.DrawRect(unitRect.Rect, Paint{Color: opaqueRed})
	b.DrawRRect(geom.RRectAll(unitRect.Rect, 0), Paint{Color: opaqueRed})
	if got, want := a.DisplayList().Ops[0], b.DisplayList().Ops[0]; got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestCanvas_TranslateAppliesToShapeAndGradient(t *testing.T) {
	c := newCanvas()
	c.Translate(10, 20)
	c.Translate(1, 2)
	c.DrawRRect(unitRect, Paint{Gradient: &Gradient{
		End:        geom.Offset{X: 10},
		StartColor: opaqueRed,
		EndColor:   opaqueRed,
	}})
	op := c.DisplayList().Ops[0]
	if got, want := op.RRect.Rect, (geom.Rect{X: 11, Y: 22, W: 10, H: 10}); got != want {
		t.Errorf("rect: got %v, want %v", got, want)
	}
	if got, want := op.Gradient.End, (geom.Offset{X: 21, Y: 22}); got != want {
		t.Errorf("gradient end: got %v, want %v", got, want)
	}
}

func TestCanvas_RestoreBringsBackEveryState(t *testing.T) {
	c := newCanvas()
	c.Save()
	c.Translate(5, 5)
	c.PushOpacity(0.5)
	c.ClipRect(geom.Rect{W: 1, H: 1})
	c.Restore()
	c.DrawRRect(unitRect, Paint{Color: opaqueRed})
	op := c.DisplayList().Ops[0]
	if op.RRect.Rect.X != 0 || op.Color.A != 1 || op.Clipped {
		t.Errorf("state leaked through Restore: %+v", op)
	}
}

func TestCanvas_RestoreWithoutSavePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("got no panic")
		}
	}()
	newCanvas().Restore()
}

func TestCanvas_ResetClearsStackAndOps(t *testing.T) {
	c := newCanvas()
	c.Save()
	c.PushOpacity(0)
	c.DrawRRect(unitRect, Paint{Color: opaqueRed})
	c.Reset()
	c.DrawRRect(unitRect, Paint{Color: opaqueRed})
	if got := len(c.DisplayList().Ops); got != 1 {
		t.Fatalf("got %d ops, want 1", got)
	}
	if got := c.DisplayList().Ops[0].Color.A; got != 1 {
		t.Errorf("alpha: got %v, want 1", got)
	}
}

func TestCanvas_OpacityMultipliesAllColors(t *testing.T) {
	c := newCanvas()
	c.PushOpacity(0.5)
	c.PushOpacity(0.5)
	c.DrawRRect(unitRect, Paint{
		Color:    opaqueRed,
		Stroke:   &Stroke{Width: 1, Color: opaqueRed},
		Gradient: &Gradient{StartColor: opaqueRed, EndColor: opaqueRed.WithAlpha(0.5)},
	})
	op := c.DisplayList().Ops[0]
	if op.Color.A != 0.25 || op.BorderColor.A != 0.25 || op.Gradient.StartColor.A != 0.25 || op.Gradient.EndColor.A != 0.125 {
		t.Errorf("got %+v", op)
	}
}

func TestCanvas_ClipRectsIntersect(t *testing.T) {
	c := newCanvas()
	c.Translate(10, 0)
	c.ClipRect(geom.Rect{W: 20, H: 20})
	c.ClipRect(geom.Rect{X: 5, Y: 5, W: 100, H: 100})
	c.DrawRRect(unitRect, Paint{Color: opaqueRed})
	op := c.DisplayList().Ops[0]
	if got, want := op.Clip, (geom.Rect{X: 15, Y: 5, W: 15, H: 15}); got != want || !op.Clipped {
		t.Errorf("got %v %v, want %v", got, op.Clipped, want)
	}
}

func TestCanvas_SkipsWhatCannotShow(t *testing.T) {
	stroke := &Stroke{Width: 1}
	tests := []struct {
		name string
		draw func(c *Canvas)
	}{
		{"幅ゼロ", func(c *Canvas) { c.DrawRRect(geom.RRect{Rect: geom.Rect{H: 10}}, Paint{Color: opaqueRed}) }},
		{"ゼロ値の Paint", func(c *Canvas) { c.DrawRRect(unitRect, Paint{}) }},
		{"透明なボーダーだけ", func(c *Canvas) { c.DrawRRect(unitRect, Paint{Stroke: stroke}) }},
		{"透明なグラデーション", func(c *Canvas) { c.DrawRRect(unitRect, Paint{Color: opaqueRed, Gradient: &Gradient{}}) }},
		{"不透明度ゼロ", func(c *Canvas) {
			c.PushOpacity(0)
			c.DrawRRect(unitRect, Paint{Color: opaqueRed})
		}},
		{"クリップの外", func(c *Canvas) {
			c.ClipRect(geom.Rect{X: 100, Y: 100, W: 1, H: 1})
			c.DrawRRect(unitRect, Paint{Color: opaqueRed})
		}},
		{"空のクリップ", func(c *Canvas) {
			c.ClipRect(geom.Rect{W: 1, H: 1})
			c.ClipRect(geom.Rect{X: 5, W: 1, H: 1})
			c.DrawRRect(unitRect, Paint{Color: opaqueRed})
		}},
		{"クリップの外の影", func(c *Canvas) {
			c.ClipRect(geom.Rect{X: 100, Y: 100, W: 1, H: 1})
			c.DrawShadow(unitRect, 4, opaqueRed, geom.Offset{})
		}},
		{"透明な影", func(c *Canvas) { c.DrawShadow(unitRect, 4, geom.Color{}, geom.Offset{}) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCanvas()
			tt.draw(c)
			if got := len(c.DisplayList().Ops); got != 0 {
				t.Errorf("got %d ops, want 0", got)
			}
		})
	}
}

func TestCanvas_BorderOnlyIsKept(t *testing.T) {
	c := newCanvas()
	c.DrawRRect(unitRect, Paint{Stroke: &Stroke{Width: 1, Color: opaqueRed}})
	if got := len(c.DisplayList().Ops); got != 1 {
		t.Errorf("got %d ops, want 1", got)
	}
}

func TestCanvas_ShadowReachesThroughClipBeyondItsShape(t *testing.T) {
	c := newCanvas()
	// The shape ends at x=10, but a blur of 8 reaches 12 further.
	c.ClipRect(geom.Rect{X: 20, Y: 0, W: 10, H: 10})
	c.DrawShadow(unitRect, 8, opaqueRed, geom.Offset{})
	if got := len(c.DisplayList().Ops); got != 1 {
		t.Errorf("got %d ops, want 1", got)
	}
}

func TestCanvas_ShadowAppliesOffsetAndTranslation(t *testing.T) {
	c := newCanvas()
	c.Translate(1, 1)
	c.DrawShadow(unitRect, 4, opaqueRed, geom.Offset{X: 2, Y: 3})
	op := c.DisplayList().Ops[0]
	if got, want := op.RRect.Rect, (geom.Rect{X: 3, Y: 4, W: 10, H: 10}); got != want || op.Kind != OpShadow || op.Blur != 4 {
		t.Errorf("got %+v, want rect %v", op, want)
	}
}

func TestCanvas_ShadowWithoutBlurIsTheShape(t *testing.T) {
	c := newCanvas()
	c.DrawShadow(geom.RRectAll(unitRect.Rect, 3), 0, opaqueRed, geom.Offset{X: 2})
	op := c.DisplayList().Ops[0]
	if op.Kind != OpRRect || op.RRect.Rect.X != 2 || op.RRect.TopLeft != 3 {
		t.Errorf("got %+v", op)
	}
}

func recordDecorated(c *Canvas, n int) {
	c.Reset()
	for i := range n {
		x := float32(i % 100)
		rr := geom.RRectAll(geom.Rect{X: x, Y: x, W: 20, H: 20}, 4)
		c.Save()
		c.Translate(1, 1)
		c.DrawShadow(rr, 8, opaqueRed, geom.Offset{Y: 2})
		c.DrawRRect(rr, Paint{
			Stroke:   &Stroke{Width: 1, Color: opaqueRed},
			Gradient: &Gradient{End: geom.Offset{X: 20}, StartColor: opaqueRed, EndColor: opaqueRed},
		})
		c.Restore()
	}
}

func TestCanvas_RecordingDoesNotAllocate(t *testing.T) {
	c := newCanvas()
	recordDecorated(c, 100)
	allocs := testing.AllocsPerRun(10, func() { recordDecorated(c, 100) })
	if allocs != 0 {
		t.Errorf("got %v allocs, want 0", allocs)
	}
}

func BenchmarkCanvas_10kRects(b *testing.B) {
	c := newCanvas()
	recordDecorated(c, 5000) // two ops each
	b.ReportAllocs()
	for b.Loop() {
		recordDecorated(c, 5000)
	}
}

func TestCanvas_ScaleAppliesToEverything(t *testing.T) {
	var cv Canvas
	cv.Reset()
	cv.Translate(100, 100)
	cv.Scale(0.5)
	cv.Translate(20, 0)
	cv.DrawRRect(geom.RRectAll(geom.Rect{W: 40, H: 40}, 8), Paint{Color: geom.Hex(0xff0000), Stroke: &Stroke{Width: 2, Color: geom.Hex(0)}})
	cv.DrawShadow(geom.RRectAll(geom.Rect{W: 40, H: 40}, 8), 10, geom.Hex(0), geom.Offset{Y: 4})
	cv.ClipRect(geom.Rect{W: 40, H: 40})
	ops := cv.DisplayList().Ops
	r := ops[0]
	if want := (geom.Rect{X: 110, Y: 100, W: 20, H: 20}); r.RRect.Rect != want {
		t.Errorf("got %+v", r.RRect.Rect)
	}
	if r.RRect.TopLeft != 4 || r.BorderWidth != 1 {
		t.Errorf("got radius %v and border %v", r.RRect.TopLeft, r.BorderWidth)
	}
	s := ops[1]
	if want := (geom.Rect{X: 110, Y: 102, W: 20, H: 20}); s.RRect.Rect != want || s.Blur != 5 {
		t.Errorf("got %+v blur %v", s.RRect.Rect, s.Blur)
	}
	if want := (geom.Rect{X: 110, Y: 100, W: 20, H: 20}); cv.state.clip != want {
		t.Errorf("got %+v", cv.state.clip)
	}
}

func TestCanvas_DrawImageRecordsItsQuality(t *testing.T) {
	img := NewImage(image.NewRGBA(image.Rect(0, 0, 4, 4)))
	var c Canvas
	c.Reset()
	c.DrawImage(img, geom.Rect{W: 4, H: 4}, 1)
	c.DrawImageQuality(img, geom.Rect{W: 4, H: 4}, 1, FilterNearest)
	ops := c.DisplayList().Ops
	if len(ops) != 2 || ops[0].Filter != FilterDefault || ops[1].Filter != FilterNearest {
		t.Fatalf("got %+v", ops)
	}
	if FilterDefault.String() != "default" || FilterMipmap.String() != "mipmap" {
		t.Error("names")
	}
}
