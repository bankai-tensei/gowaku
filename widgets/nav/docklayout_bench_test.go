package nav

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
)

// ideArrangement is files at the left, the editors with output below them in
// the middle, and properties at the right.
func ideArrangement() DockArrangement {
	return DockArrangement{Root: DockNode{
		Weights: []float32{1, 3, 1},
		Children: []DockNode{
			{Panels: []string{"files"}},
			{Axis: layout.Vertical, Weights: []float32{3, 1}, Children: []DockNode{
				{Panels: []string{"main.go", "dock.go"}},
				{Panels: []string{"output"}},
			}},
			{Panels: []string{"props"}},
		},
	}}
}

// dockFrameRunner drags the divider between the files and the editors of an
// IDE arrangement back and forth, a pixel a frame, as a hand does.
type dockFrameRunner struct {
	tree  *core.Tree
	size  geom.Size
	cv    paint.Canvas
	at    geom.Offset
	step  float32
	files *dockNode
	// handle is the divider's strip, and viaTree sends the moves through the
	// tree instead of to the strip.
	handle  *kit.RenderSplitHandle
	viaTree bool
}

func newDockFrameRunner(t testing.TB) *dockFrameRunner {
	t.Helper()
	colors := []geom.Color{wtest.Red, wtest.Green, wtest.Blue}
	var panels []DockPanel
	for i, id := range []string{"files", "main.go", "dock.go", "output", "props"} {
		color := colors[i%len(colors)]
		panels = append(panels, DockPanel{ID: id, Title: id, Closable: true, Build: func(core.BuildContext) core.Widget {
			return basic.Container{Color: color, Padding: geom.All(8), Child: basic.Container{Color: wtest.Blue}}
		}})
	}
	c := &DockController{}
	tree := wtest.TextTree(t, basic.MediaQuery{
		Data:  basic.MediaQueryData{Scale: 1, ReduceMotion: true},
		Child: DockLayout{Panels: panels, Controller: c, Default: ideArrangement()},
	})
	r := &dockFrameRunner{tree: tree, size: geom.Size{W: 1600, H: 900}, step: 1}
	wtest.Ops(tree, r.size)
	r.files = c.groupOf("files")
	// The divider after the files.
	f := r.files.rect
	r.at = geom.Offset{X: f.X + f.W, Y: f.H / 2}
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: r.at})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: r.at, Button: input.ButtonPrimary})
	walkRender(tree.RootRenderObject(), func(ro core.RenderObject) {
		if h, ok := ro.(*kit.RenderSplitHandle); ok && r.handle == nil && core.GlobalOffset(h).X < r.at.X {
			r.handle = h
		}
	})
	// Warm up over many sweeps each way: the runtime fills the caches of its
	// type assertions now and then, at random, and those allocate.
	for range 20000 {
		r.frame()
	}
	return r
}

// frame moves the pointer a pixel, then lays out and paints.
func (r *dockFrameRunner) frame() {
	if r.at.X > 420 || r.at.X < 260 {
		r.step = -r.step
	}
	r.at.X += r.step
	if r.viaTree {
		r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: r.at, Button: input.ButtonPrimary})
	} else {
		e := core.PointerEvent{Kind: core.PointerMove, Pos: r.at.Sub(core.GlobalOffset(r.handle)), Button: input.ButtonPrimary}
		r.handle.HandlePointer(&e)
	}
	r.tree.Build()
	r.tree.Layout(geom.Tight(r.size))
	r.cv.Reset()
	r.tree.Paint(&r.cv)
}

func TestDockLayout_DraggingADividerDoesNotAllocate(t *testing.T) {
	r := newDockFrameRunner(t)
	before := r.files.rect.W
	r.frame()
	if r.files.rect.W == before {
		t.Fatal("the divider must move")
	}
	// The runtime allocates now and then on its own, and one stray
	// allocation in 1000 frames failed this test under -race. Ten runs of
	// 100 frames are averaged, so a stray shows as 0 and an allocation in
	// every frame as 100.
	allocs := testing.AllocsPerRun(10, func() {
		for range 100 {
			r.frame()
		}
	})
	if allocs != 0 {
		t.Errorf("got %v allocs per 100 frames", allocs)
	}
}

// BenchmarkDockLayout_DragDivider_5Panels is one frame of dragging a divider
// of an IDE arrangement of four groups: the move of the pointer on the strip
// of the divider, the layout of every group, and the paint.
func BenchmarkDockLayout_DragDivider_5Panels(b *testing.B) {
	r := newDockFrameRunner(b)
	b.ReportAllocs()
	for b.Loop() {
		r.frame()
	}
}

// BenchmarkDockLayout_DragDividerViaTree_5Panels is the same frame with the
// move delivered by core.Tree.DispatchPointer. Its one allocation is the copy
// of the event that core.Tree.deliver hands to every pointer target, which
// every widget pays, and not in the layout or the paint.
func BenchmarkDockLayout_DragDividerViaTree_5Panels(b *testing.B) {
	r := newDockFrameRunner(b)
	r.viaTree = true
	b.ReportAllocs()
	for b.Loop() {
		r.frame()
	}
}

// dockFlyoutRunner drags the handle of the properties, hidden at the right
// and slid out over an IDE arrangement, back and forth, a pixel a frame.
type dockFlyoutRunner struct {
	tree   *core.Tree
	size   geom.Size
	cv     paint.Canvas
	at     geom.Offset
	step   float32
	dock   *renderDock
	handle *kit.RenderSplitHandle
}

func newDockFlyoutRunner(t testing.TB) *dockFlyoutRunner {
	t.Helper()
	var panels []DockPanel
	for _, id := range []string{"files", "main.go", "dock.go", "output", "props"} {
		panels = append(panels, DockPanel{ID: id, Title: id, Closable: true, Build: func(core.BuildContext) core.Widget {
			return basic.Container{Color: wtest.Green, Padding: geom.All(8), Child: basic.Container{Color: wtest.Blue}}
		}})
	}
	c := &DockController{}
	tree := wtest.TextTree(t, basic.MediaQuery{
		Data:  basic.MediaQueryData{Scale: 1, ReduceMotion: true},
		Child: DockLayout{Panels: panels, Controller: c, Default: ideArrangement(), AutoHide: true},
	})
	r := &dockFlyoutRunner{tree: tree, size: geom.Size{W: 1600, H: 900}, step: 1}
	wtest.Ops(tree, r.size)
	c.Unpin("props", DockRight)
	wtest.Ops(tree, r.size)
	c.Show("props")
	wtest.Ops(tree, r.size)
	walkRender(tree.RootRenderObject(), func(ro core.RenderObject) {
		if d, ok := ro.(*renderDock); ok {
			r.dock = d
		}
	})
	for i := range r.dock.slots {
		if r.dock.slots[i].kind == dockSlotFlyoutHandle {
			walkRender(r.dock.children[i], func(ro core.RenderObject) {
				if h, ok := ro.(*kit.RenderSplitHandle); ok {
					r.handle = h
				}
			})
		}
	}
	if r.handle == nil {
		t.Fatal("no handle")
	}
	f := r.dock.fly
	r.at = geom.Offset{X: f.X, Y: f.Y + f.H/2}
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: r.at})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: r.at, Button: input.ButtonPrimary})
	// Warm up as for the divider: the caches of type assertions fill now and
	// then, at random, and those allocate.
	for range 20000 {
		r.frame()
	}
	return r
}

// frame moves the pointer a pixel, then lays out and paints.
func (r *dockFlyoutRunner) frame() {
	if r.at.X > 1300 || r.at.X < 1100 {
		r.step = -r.step
	}
	r.at.X += r.step
	e := core.PointerEvent{Kind: core.PointerMove, Pos: r.at.Sub(core.GlobalOffset(r.handle)), Button: input.ButtonPrimary}
	r.handle.HandlePointer(&e)
	r.tree.Build()
	r.tree.Layout(geom.Tight(r.size))
	r.cv.Reset()
	r.tree.Paint(&r.cv)
}

func TestDockLayout_ResizingAFlyoutDoesNotAllocate(t *testing.T) {
	r := newDockFlyoutRunner(t)
	before := r.dock.fly.W
	r.frame()
	if r.dock.fly.W == before {
		t.Fatal("the flyout must follow the handle")
	}
	// The runtime allocates now and then on its own, and one stray
	// allocation in 1000 frames failed this test under -race. Ten runs of
	// 100 frames are averaged, so a stray shows as 0 and an allocation in
	// every frame as 100.
	allocs := testing.AllocsPerRun(10, func() {
		for range 100 {
			r.frame()
		}
	})
	if allocs != 0 {
		t.Errorf("got %v allocs per 100 frames", allocs)
	}
}

// BenchmarkDockLayout_ResizeFlyout_5Panels is one frame of dragging the inner
// edge of a hidden panel slid out over an IDE arrangement: the move of the
// pointer on the handle, the layout of every group and of the flyout, and the
// paint.
func BenchmarkDockLayout_ResizeFlyout_5Panels(b *testing.B) {
	r := newDockFlyoutRunner(b)
	b.ReportAllocs()
	for b.Loop() {
		r.frame()
	}
}
