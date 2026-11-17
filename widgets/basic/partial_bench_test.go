package basic

import (
	"fmt"
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

// partialTree is n cards and one small widget of its own state, whose color
// changes: with boundary set, the widget paints into a layer of its own.
func partialTree(n int, boundary bool, color *geom.Color, leaf, root **wtest.SwapState) core.Widget {
	return wtest.Swap{State: root, Build: func() core.Widget {
		children := make([]core.Widget, 0, n+1)
		for range n {
			children = append(children, Padding{Insets: geom.All(1), Child: Container{Width: 10, Height: 10, Color: wtest.Red, Radius: 2}})
		}
		var one core.Widget = wtest.Swap{State: leaf, Build: func() core.Widget {
			return Container{Width: 10, Height: 10, Color: *color}
		}}
		if boundary {
			one = RepaintBoundary{Child: one}
		}
		return Stack{Children: append(children, one)}
	}}
}

// benchmarkFrame measures a frame (build, layout, paint) after one state
// changed a color: the small widget when whole is false, the root otherwise.
func benchmarkFrame(b *testing.B, n int, boundary, whole bool) {
	color := wtest.Red
	var leaf, root *wtest.SwapState
	tree := core.NewTree(partialTree(n, boundary, &color, &leaf, &root))
	testutil.RenderTree(tree, wtest.Page)
	var cv paint.Canvas
	cv.Reset()
	tree.Paint(&cv)
	c := geom.Tight(wtest.Page)
	s := leaf
	if whole {
		s = root
	}
	b.ReportAllocs()
	flip := false
	for b.Loop() {
		flip = !flip
		s.SetState(func() {
			if flip {
				color = wtest.Green
			} else {
				color = wtest.Red
			}
		})
		tree.Build()
		tree.Layout(c)
		tree.Repaint(&cv)
	}
}

// BenchmarkFrame_OneOfN is the cost of a frame in which one widget of n+1
// changed, against the cost of building all of them again.
func BenchmarkFrame_OneOfN(b *testing.B) {
	for _, n := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("leaf-in-layer/%d", n), func(b *testing.B) { benchmarkFrame(b, n, true, false) })
		b.Run(fmt.Sprintf("leaf/%d", n), func(b *testing.B) { benchmarkFrame(b, n, false, false) })
		b.Run(fmt.Sprintf("whole/%d", n), func(b *testing.B) { benchmarkFrame(b, n, false, true) })
	}
}
