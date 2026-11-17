package overlay

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/paint"
)

func TestDialog_GlassBlursBehindTheCard(t *testing.T) {
	ops := paintOps(t, basic.Align{Child: Dialog{Title: "Hi", Glass: true}})
	if len(ops) == 0 || ops[0].Kind != paint.OpBackdrop || ops[0].Blur != dialogGlassBlur {
		t.Fatalf("first op %v, want the blur", ops)
	}
	for _, op := range paintOps(t, basic.Align{Child: Dialog{Title: "Hi"}}) {
		if op.Kind == paint.OpBackdrop {
			t.Fatal("a dialog without Glass blurs")
		}
	}
}

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
