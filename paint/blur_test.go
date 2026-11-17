package paint

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

func TestCanvas_DrawBackdrop(t *testing.T) {
	var c Canvas
	c.Reset()
	c.Translate(10, 20)
	c.Scale(2)
	c.PushOpacity(0.5)
	c.ClipRect(geom.Rect{W: 100, H: 100})
	c.DrawBackdrop(geom.RRectAll(geom.Rect{X: 5, Y: 5, W: 30, H: 20}, 4), 6)
	ops := c.DisplayList().Ops
	if len(ops) != 1 {
		t.Fatalf("got %d ops, want 1", len(ops))
	}
	op := ops[0]
	if op.Kind != OpBackdrop || op.Kind.String() != "backdrop" {
		t.Errorf("kind %v", op.Kind)
	}
	if want := geom.RRectAll(geom.Rect{X: 20, Y: 30, W: 60, H: 40}, 8); op.RRect != want {
		t.Errorf("shape %v, want %v", op.RRect, want)
	}
	if op.Blur != 12 || op.Color.A != 0.5 || !op.Clipped {
		t.Errorf("blur %v, opacity %v, clipped %v; want 12, 0.5 and true", op.Blur, op.Color.A, op.Clipped)
	}
}

func TestCanvas_DrawBackdropSkipsNothing(t *testing.T) {
	var c Canvas
	c.Reset()
	c.DrawBackdrop(geom.RRect{Rect: geom.Rect{W: 10, H: 10}}, 0)
	c.ClipRect(geom.Rect{X: 100, Y: 100, W: 10, H: 10})
	c.DrawBackdrop(geom.RRect{Rect: geom.Rect{W: 10, H: 10}}, 4)
	if n := len(c.DisplayList().Ops); n != 0 {
		t.Errorf("got %d ops for a blur of zero and one outside the clip", n)
	}
}
