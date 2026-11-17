package layout

import (
	"math"
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

func laidOutTransform(m geom.Affine, origin Alignment) *RenderTransform {
	child := &RenderProxy{}
	r := &RenderTransform{Transform: m, Origin: origin}
	r.SetChild(child)
	core.LayoutChild(r, geom.Tight(geom.Size{W: 40, H: 20}))
	return r
}

func TestRenderTransform_ChildTransformTurnsAroundOrigin(t *testing.T) {
	r := laidOutTransform(geom.Rotation(math.Pi), AlignCenter)
	// A half turn around the middle (20, 10) swaps the corners.
	got := r.ChildTransform().Apply(geom.Offset{})
	if geom.Abs32(got.X-40) > 1e-4 || geom.Abs32(got.Y-20) > 1e-4 {
		t.Errorf("got %v, want (40, 20)", got)
	}
	if z := laidOutTransform(geom.Affine{}, AlignCenter); z.ChildTransform() != (geom.Affine{}) {
		t.Error("the zero transform must stay the identity")
	}
}

func TestRenderTransform_FlatPaintsAndHitsNothing(t *testing.T) {
	r := laidOutTransform(geom.Scaling(0, 1), AlignTopLeft)
	var res core.HitTestResult
	if r.HitTest(geom.Offset{}, &res) {
		t.Error("a flat transform must not be hit")
	}
	var cv paint.Canvas
	cv.Reset()
	r.Paint(&cv, geom.Offset{})
	if n := len(cv.DisplayList().Ops); n != 0 {
		t.Errorf("got %d ops", n)
	}
}
