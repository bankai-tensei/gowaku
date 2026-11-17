package interact

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

// benchViewer returns a tree with a viewer over a canvas, laid out and painted
// once, and a function that runs a frame.
func benchViewer(b *testing.B, layouts *int) (*core.Tree, *ViewerController, func()) {
	var ctl ViewerController
	tree := core.NewTree(InteractiveViewer{
		Controller: &ctl, Boundary: ViewerUnbounded,
		OnChange: func(float32, geom.Offset) {},
		Child:    wtest.LayoutCount{N: layouts, Child: canvas(func(geom.Offset) {})},
	})
	b.Cleanup(tree.Dispose)
	size := wtest.Screen
	cv := &paint.Canvas{}
	now := time.Second
	frame := func() {
		now += 8 * time.Millisecond
		tree.Tick(now)
		tree.Build()
		tree.Layout(geom.Tight(size))
		tree.Repaint(cv)
	}
	tree.Build()
	tree.Layout(geom.Tight(size))
	cv.Reset()
	tree.Paint(cv)
	return tree, &ctl, frame
}

// BenchmarkInteractiveViewer_PanFrame drags the background by a pixel and
// draws the frame. The moves go to the viewer's render object directly: see
// BenchmarkInteractiveViewer_DispatchedPanFrame for the way through the tree.
func BenchmarkInteractiveViewer_PanFrame(b *testing.B) {
	var layouts int
	tree, ctl, frame := benchViewer(b, &layouts)
	at := geom.Offset{X: 300, Y: 250}
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at})
	frame()
	ro := ctl.state.ro
	dx := float32(1)
	b.ReportAllocs()
	for b.Loop() {
		at.X += dx
		if at.X > 380 || at.X < 220 {
			dx = -dx
		}
		e := core.PointerEvent{Kind: core.PointerMove, Pos: at}
		ro.HandlePointer(&e)
		frame()
	}
	b.StopTimer()
	if layouts != 1 {
		b.Errorf("the child was laid out %d times, want 1", layouts)
	}
}

// BenchmarkInteractiveViewer_DispatchedPanFrame is PanFrame with the moves
// dispatched through the tree, hit test included. It makes 1 allocation of 64
// bytes per frame, in core.Tree.deliver: the copy of the event it hands to
// HandlePointer escapes through the interface call. Every pointer event of
// every widget pays it; the viewer's own part is PanFrame.
func BenchmarkInteractiveViewer_DispatchedPanFrame(b *testing.B) {
	var layouts int
	tree, _, frame := benchViewer(b, &layouts)
	at := geom.Offset{X: 300, Y: 250}
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at})
	frame()
	dx := float32(1)
	b.ReportAllocs()
	for b.Loop() {
		at.X += dx
		if at.X > 380 || at.X < 220 {
			dx = -dx
		}
		tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: at})
		frame()
	}
}

// BenchmarkInteractiveViewer_ZoomFrame pinches in and out and draws the frame,
// handing the pinch to the render object as PanFrame does.
func BenchmarkInteractiveViewer_ZoomFrame(b *testing.B) {
	var layouts int
	_, ctl, frame := benchViewer(b, &layouts)
	ro := ctl.state.ro
	at := geom.Offset{X: 200, Y: 150}
	pinch := float32(1.01)
	n := 0
	b.ReportAllocs()
	for b.Loop() {
		if n++; n%50 == 0 {
			pinch = 1 / pinch
		}
		e := core.PointerEvent{Kind: core.PointerPinch, Pos: at, Pinch: pinch}
		ro.HandlePointer(&e)
		frame()
	}
	b.StopTimer()
	if layouts != 1 {
		b.Errorf("the child was laid out %d times, want 1", layouts)
	}
}

// BenchmarkInteractiveViewer_HitTest hit tests the button inside a zoomed
// viewer.
func BenchmarkInteractiveViewer_HitTest(b *testing.B) {
	var layouts int
	_, ctl, frame := benchViewer(b, &layouts)
	ctl.Set(2, geom.Offset{X: -100, Y: -100}, false)
	frame()
	ro := ctl.state.ro
	var result core.HitTestResult
	b.ReportAllocs()
	for b.Loop() {
		result.Path = result.Path[:0]
		ro.HitTest(geom.Offset{X: 150, Y: 150}, &result)
	}
	b.StopTimer()
	if len(result.Path) < 3 {
		b.Errorf("hit %d objects, want the button inside", len(result.Path))
	}
}

// BenchmarkInteractiveViewer_AnimatedFrame runs frames of an animated zoom.
func BenchmarkInteractiveViewer_AnimatedFrame(b *testing.B) {
	var layouts int
	_, ctl, frame := benchViewer(b, &layouts)
	n := 0
	b.ReportAllocs()
	for b.Loop() {
		if n%30 == 0 {
			if n%60 == 0 {
				ctl.ZoomBy(2, geom.Offset{X: 200, Y: 150}, true)
			} else {
				ctl.ZoomBy(0.5, geom.Offset{X: 200, Y: 150}, true)
			}
		}
		n++
		frame()
	}
}

func TestInteractiveViewer_FramesDoNotAllocate(t *testing.T) {
	for name, bench := range map[string]func(*testing.B){
		"pan":      BenchmarkInteractiveViewer_PanFrame,
		"zoom":     BenchmarkInteractiveViewer_ZoomFrame,
		"animated": BenchmarkInteractiveViewer_AnimatedFrame,
		"hit test": BenchmarkInteractiveViewer_HitTest,
	} {
		if n := testing.Benchmark(bench).AllocsPerOp(); n != 0 {
			t.Errorf("%s: %d allocations per frame, want 0", name, n)
		}
	}
}
