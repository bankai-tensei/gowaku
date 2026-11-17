package basic

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
)

func paintOps(t *testing.T, w core.Widget) []paint.Op {
	t.Helper()
	tree := core.NewTree(testutil.WithShaper(testutil.NewShaper(t), w))
	t.Cleanup(tree.Dispose)
	tree.Layout(geom.Tight(geom.Size{W: 200, H: 200}))
	var cv paint.Canvas
	cv.Reset()
	tree.Paint(&cv)
	return append([]paint.Op(nil), cv.DisplayList().Ops...)
}

func TestBackdropFilter_BlursBeforeTheChild(t *testing.T) {
	ops := paintOps(t, Align{Child: BackdropFilter{Sigma: 8, Radius: 12, Child: Container{
		Width: 60, Height: 40, Color: geom.Hex(0xffffff).WithAlpha(0.5),
	}}})
	if len(ops) != 2 || ops[0].Kind != paint.OpBackdrop || ops[1].Kind != paint.OpRRect {
		t.Fatalf("got %v, want the blur, then the fill", ops)
	}
	if ops[0].Blur != 8 || ops[0].RRect.TopLeft != 12 || ops[0].RRect.Rect != ops[1].RRect.Rect {
		t.Errorf("blur %+v over %+v", ops[0].RRect, ops[1].RRect)
	}
	if ops := paintOps(t, BackdropFilter{Child: SizedBox{Width: 10, Height: 10}}); len(ops) != 0 {
		t.Errorf("a blur of zero records %v", ops)
	}
}

func TestBackdropFilter_UpdateRepaints(t *testing.T) {
	ro := BackdropFilter{Sigma: 4}.CreateRenderObject().(*layout.RenderBackdropFilter)
	BackdropFilter{Sigma: 6, Radius: 2}.UpdateRenderObject(ro)
	if ro.Sigma != 6 || ro.Radius != 2 {
		t.Errorf("got %v and %v", ro.Sigma, ro.Radius)
	}
}

func TestImageFiltered_RecordsABlurredGroup(t *testing.T) {
	ops := paintOps(t, ImageFiltered{Sigma: 3, Child: Container{Width: 20, Height: 20, Color: geom.Hex(0x336699)}})
	if len(ops) != 3 || ops[0].Kind != paint.OpGroupPush || ops[1].Kind != paint.OpLayer {
		t.Fatalf("got %v, want one layer in a blurred group", ops)
	}
	// Under a scale transition the child draws by itself, still blurred.
	ops = paintOps(t, Scale{Scale: 0.9, Child: ImageFiltered{Sigma: 3, Child: Container{Width: 20, Height: 20, Color: geom.Hex(0x336699)}}})
	if len(ops) != 3 || ops[0].Kind != paint.OpGroupPush || ops[1].Kind != paint.OpRRect {
		t.Errorf("got %v, want the box itself in the group", ops)
	}
	ro := ImageFiltered{Sigma: 3}.CreateRenderObject().(*core.RenderRepaintBoundary)
	ImageFiltered{Sigma: -1}.UpdateRenderObject(ro)
	if ro.Blur() != 0 {
		t.Errorf("blur %v, want a negative sigma to be none", ro.Blur())
	}
}
