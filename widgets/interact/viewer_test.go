package interact

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

// viewerRig shows an InteractiveViewer filling a 400×300 window.
type viewerRig struct {
	t       *testing.T
	tree    *core.Tree
	ctl     *ViewerController
	size    geom.Size
	now     time.Duration
	layouts int
	changes int
	clock   uint32
}

// canvas is a 1000×800 child with a button at (100, 100), 50 on a side.
func canvas(tap func(geom.Offset)) core.Widget {
	return basic.Stack{Children: []core.Widget{
		basic.SizedBox{Width: 1000, Height: 800, Child: basic.Container{Color: wtest.Blue}},
		basic.Positioned{Left: 100, Top: 100, Width: 50, Height: 50, Child: GestureDetector{
			OnTapDownAt: func(t layout.Tap) { tap(t.Pos) },
			Child:       basic.Container{Color: wtest.Red},
		}},
	}}
}

func newViewerRig(t *testing.T, v InteractiveViewer, reduce bool) *viewerRig {
	t.Helper()
	r := &viewerRig{t: t, size: wtest.Screen, now: time.Second, clock: 1000}
	if v.Controller == nil {
		v.Controller = &ViewerController{}
	}
	r.ctl = v.Controller
	if v.OnChange == nil {
		v.OnChange = func(float32, geom.Offset) { r.changes++ }
	}
	v.Child = wtest.LayoutCount{N: &r.layouts, Child: v.Child}
	r.tree = wtest.TextTree(t, basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1, ReduceMotion: reduce}, Child: v})
	r.frame()
	return r
}

func (r *viewerRig) frame() { wtest.Ops(r.tree, r.size) }

func (r *viewerRig) settle() {
	r.t.Helper()
	wtest.Settled(r.t, r.tree, r.size, &r.now)
}

func (r *viewerRig) pointer(kind core.PointerKind, x, y float32) {
	r.clock += 10
	r.tree.DispatchPointer(core.PointerEvent{Kind: kind, Pos: geom.Offset{X: x, Y: y}, Time: r.clock})
}

// drag presses at from, moves to to in steps of 10 ms, and releases there
// after rest milliseconds.
func (r *viewerRig) drag(from, to geom.Offset, steps int, rest uint32) {
	r.pointer(core.PointerDown, from.X, from.Y)
	for i := 1; i <= steps; i++ {
		f := float32(i) / float32(steps)
		r.pointer(core.PointerMove, from.X+(to.X-from.X)*f, from.Y+(to.Y-from.Y)*f)
	}
	r.clock += rest
	r.pointer(core.PointerUp, to.X, to.Y)
}

func near(a, b float32) bool { return geom.Abs32(a-b) < 1e-3 }

func nearOffset(a, b geom.Offset) bool { return near(a.X, b.X) && near(a.Y, b.Y) }

func (r *viewerRig) wantView(scale float32, off geom.Offset) {
	r.t.Helper()
	if !near(r.ctl.Scale(), scale) || !nearOffset(r.ctl.Offset(), off) {
		r.t.Errorf("view at scale %v offset %v, want %v %v", r.ctl.Scale(), r.ctl.Offset(), scale, off)
	}
}

func TestInteractiveViewer_DragPans(t *testing.T) {
	r := newViewerRig(t, InteractiveViewer{Child: canvas(nil)}, false)
	r.drag(geom.Offset{X: 300, Y: 250}, geom.Offset{X: 250, Y: 220}, 5, 200)
	r.wantView(1, geom.Offset{X: -50, Y: -30})
	if r.changes == 0 {
		t.Error("OnChange did not run")
	}
	// The middle button pans too.
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: geom.Offset{X: 300, Y: 250}, Button: input.ButtonMiddle})
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 290, Y: 250}, Button: input.ButtonMiddle})
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: geom.Offset{X: 290, Y: 250}, Button: input.ButtonMiddle})
	r.wantView(1, geom.Offset{X: -60, Y: -30})
}

func TestInteractiveViewer_CtrlWheelKeepsThePointFixed(t *testing.T) {
	r := newViewerRig(t, InteractiveViewer{Boundary: ViewerUnbounded, Child: canvas(nil)}, false)
	r.ctl.Set(1.5, geom.Offset{X: -30, Y: 12}, false)
	at := geom.Offset{X: 170, Y: 90}
	before := r.ctl.ToChild(at)
	for _, notches := range []float32{1, 2, -1} {
		r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: at, Scroll: geom.Offset{Y: notches}, Modifiers: input.ModControl})
		if got := r.ctl.ToChild(at); !nearOffset(got, before) {
			t.Errorf("after %v notches the point under the pointer is %v, want %v", notches, got, before)
		}
	}
	// 1.1 to the power of 2 notches in all.
	if want := float32(1.5 * 1.1 * 1.1); geom.Abs32(r.ctl.Scale()-want) > 1e-4 {
		t.Errorf("scale %v, want %v", r.ctl.Scale(), want)
	}
}

func TestInteractiveViewer_PinchZoomsAboutTheFocus(t *testing.T) {
	r := newViewerRig(t, InteractiveViewer{Child: canvas(nil)}, false)
	at := geom.Offset{X: 120, Y: 80}
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerPinch, Pos: at, Pinch: 2})
	r.wantView(2, geom.Offset{X: -120, Y: -80})
	// Two fingers on a touch screen.
	r.tree.DispatchPointer(finger(core.PointerDown, 1, 100, 100))
	r.tree.DispatchPointer(finger(core.PointerDown, 2, 200, 100))
	r.tree.DispatchPointer(finger(core.PointerMove, 2, 300, 100))
	r.tree.DispatchPointer(finger(core.PointerUp, 2, 300, 100))
	r.tree.DispatchPointer(finger(core.PointerUp, 1, 100, 100))
	if !near(r.ctl.Scale(), 4) {
		t.Errorf("two fingers moving twice as far apart made scale %v, want 4", r.ctl.Scale())
	}
}

func TestInteractiveViewer_ScaleStopsAtTheLimits(t *testing.T) {
	r := newViewerRig(t, InteractiveViewer{MinScale: 0.5, MaxScale: 3, Boundary: ViewerUnbounded, Child: canvas(nil)}, false)
	at := geom.Offset{X: 200, Y: 150}
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerPinch, Pos: at, Pinch: 10})
	if r.ctl.Scale() != 3 {
		t.Errorf("scale %v, want the maximum 3", r.ctl.Scale())
	}
	// The point under the pinch still stays put at the limit.
	if got := r.ctl.ToChild(at); !nearOffset(got, at) {
		t.Errorf("point under the pinch moved to %v", got)
	}
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerPinch, Pos: at, Pinch: 0.001})
	if r.ctl.Scale() != 0.5 {
		t.Errorf("scale %v, want the minimum 0.5", r.ctl.Scale())
	}
	r.ctl.Set(100, geom.Offset{}, false)
	if r.ctl.Scale() != 3 {
		t.Errorf("Set went past the maximum: %v", r.ctl.Scale())
	}
}

func TestInteractiveViewer_DefaultLimits(t *testing.T) {
	r := newViewerRig(t, InteractiveViewer{Child: canvas(nil)}, false)
	r.ctl.Set(1000, geom.Offset{}, false)
	if r.ctl.Scale() != defaultMaxScale {
		t.Errorf("scale %v, want %v", r.ctl.Scale(), defaultMaxScale)
	}
	r.ctl.Set(0.0001, geom.Offset{}, false)
	if r.ctl.Scale() != defaultMinScale {
		t.Errorf("scale %v, want %v", r.ctl.Scale(), float32(defaultMinScale))
	}
}

func TestInteractiveViewer_Bounds(t *testing.T) {
	for _, c := range []struct {
		name     string
		boundary ViewerBoundary
		margin   float32
		want     geom.Offset
	}{
		{"境界あり", ViewerBounded, 0, geom.Offset{}},
		{"余白 20", ViewerBounded, 20, geom.Offset{X: 20, Y: 20}},
		{"境界なし", ViewerUnbounded, 0, geom.Offset{X: 100, Y: 100}},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newViewerRig(t, InteractiveViewer{Boundary: c.boundary, BoundaryMargin: c.margin, Child: canvas(nil)}, false)
			r.drag(geom.Offset{X: 300, Y: 250}, geom.Offset{X: 400 - 1, Y: 350 - 1}, 4, 200)
			want := c.want
			if c.boundary == ViewerUnbounded {
				want = geom.Offset{X: 99, Y: 99}
			}
			r.wantView(1, want)
		})
	}
	// The far edge: the 1000×800 child at scale 1 in 400×300 goes to -600, -500.
	r := newViewerRig(t, InteractiveViewer{Child: canvas(nil)}, false)
	r.ctl.Set(1, geom.Offset{X: -5000, Y: -5000}, false)
	r.wantView(1, geom.Offset{X: -600, Y: -500})
}

func TestInteractiveViewer_SmallChildIsCentered(t *testing.T) {
	r := newViewerRig(t, InteractiveViewer{Child: basic.SizedBox{Width: 100, Height: 50, Child: basic.Container{Color: wtest.Red}}}, false)
	r.wantView(1, geom.Offset{X: 150, Y: 125})
	r.drag(geom.Offset{X: 200, Y: 150}, geom.Offset{X: 260, Y: 100}, 3, 200)
	r.wantView(1, geom.Offset{X: 150, Y: 125})
	// Zoomed in past the viewer along x only, it pans along x only.
	r.ctl.Set(6, geom.Offset{}, false)
	r.wantView(6, geom.Offset{X: 0, Y: 0})
	r.ctl.PanBy(geom.Offset{X: -50, Y: -50}, false)
	r.wantView(6, geom.Offset{X: -50, Y: 0})
}

func TestInteractiveViewer_FlingGlidesOnAndSettles(t *testing.T) {
	r := newViewerRig(t, InteractiveViewer{Boundary: ViewerUnbounded, Child: canvas(nil)}, false)
	// 100 px in 50 ms is 2000 px per second.
	r.drag(geom.Offset{X: 300, Y: 150}, geom.Offset{X: 200, Y: 150}, 5, 0)
	released := r.ctl.Offset()
	if !nearOffset(released, geom.Offset{X: -100}) || !r.ctl.Moving() {
		t.Fatalf("released at %v, moving %v", released, r.ctl.Moving())
	}
	r.settle()
	got := r.ctl.Offset()
	if got.X > -500 || got.X < -800 || got.Y != 0 {
		t.Errorf("a fling of 2000 px/s ended at %v, want about -100 - 2000/√12", got)
	}
	if r.ctl.Moving() {
		t.Error("still moving after it settled")
	}
	// A drag that rests before the release does not fling.
	r.drag(geom.Offset{X: 300, Y: 150}, geom.Offset{X: 200, Y: 150}, 5, 200)
	if r.ctl.Moving() {
		t.Error("a release after a rest flung")
	}
	// A press stops a fling where it is.
	r.drag(geom.Offset{X: 300, Y: 150}, geom.Offset{X: 200, Y: 150}, 5, 0)
	r.now += 8 * time.Millisecond
	r.tree.Tick(r.now)
	r.pointer(core.PointerDown, 300, 150)
	if r.ctl.Moving() {
		t.Error("a press did not stop the fling")
	}
	r.pointer(core.PointerUp, 300, 150)
}

func TestInteractiveViewer_FlingStopsAtTheEdge(t *testing.T) {
	r := newViewerRig(t, InteractiveViewer{Child: canvas(nil)}, false)
	r.drag(geom.Offset{X: 300, Y: 150}, geom.Offset{X: 100, Y: 150}, 5, 0)
	r.settle()
	r.wantView(1, geom.Offset{X: -600, Y: 0})
}

func TestInteractiveViewer_ReduceMotionHasNoFlingAndJumps(t *testing.T) {
	r := newViewerRig(t, InteractiveViewer{Boundary: ViewerUnbounded, Child: canvas(nil)}, true)
	r.drag(geom.Offset{X: 300, Y: 150}, geom.Offset{X: 200, Y: 150}, 5, 0)
	if r.ctl.Moving() {
		t.Error("flung under ReduceMotion")
	}
	r.ctl.Set(2, geom.Offset{X: 10, Y: 10}, true)
	r.wantView(2, geom.Offset{X: 10, Y: 10})
	wtest.Wheel(r.tree, -1)
	r.wantView(2, geom.Offset{X: 10, Y: -90})
}

func TestInteractiveViewer_ChildIsHitThroughTheTransform(t *testing.T) {
	var taps []geom.Offset
	r := newViewerRig(t, InteractiveViewer{Child: canvas(func(p geom.Offset) { taps = append(taps, p) })}, false)
	// At scale 2 with the child moved by (-100, -100), the button at (100,
	// 100)-(150, 150) of the child shows at (100, 100)-(200, 200).
	r.ctl.Set(2, geom.Offset{X: -100, Y: -100}, false)
	r.frame()
	wtest.Tap(r.tree, geom.Offset{X: 150, Y: 150})
	if len(taps) != 1 || !nearOffset(taps[0], geom.Offset{X: 25, Y: 25}) {
		t.Fatalf("taps %v, want one at (25, 25)", taps)
	}
	wtest.Tap(r.tree, geom.Offset{X: 90, Y: 150})
	wtest.Tap(r.tree, geom.Offset{X: 210, Y: 150})
	if len(taps) != 1 {
		t.Errorf("taps beside the button hit it: %v", taps)
	}
	// A press on the button that moves does not pan with the mouse.
	r.drag(geom.Offset{X: 150, Y: 150}, geom.Offset{X: 50, Y: 50}, 4, 200)
	r.wantView(2, geom.Offset{X: -100, Y: -100})
}

func TestInteractiveViewer_FingerOnAChildPansPastTheSlop(t *testing.T) {
	var taps int
	r := newViewerRig(t, InteractiveViewer{Child: basic.Stack{Children: []core.Widget{
		basic.SizedBox{Width: 1000, Height: 800},
		basic.Positioned{Left: 100, Top: 100, Width: 50, Height: 50, Child: GestureDetector{
			OnTap: func() { taps++ }, Child: basic.Container{Color: wtest.Red},
		}},
	}}}, false)
	r.tree.DispatchPointer(finger(core.PointerDown, 1, 120, 120))
	r.tree.DispatchPointer(finger(core.PointerMove, 1, 116, 117))
	r.wantView(1, geom.Offset{})
	r.tree.DispatchPointer(finger(core.PointerMove, 1, 100, 90))
	r.tree.DispatchPointer(finger(core.PointerMove, 1, 80, 70))
	r.tree.DispatchPointer(finger(core.PointerUp, 1, 80, 70))
	r.wantView(1, geom.Offset{X: -40, Y: -50})
	if taps != 0 {
		t.Errorf("the pan tapped the button %d times", taps)
	}
	// A finger that does not move taps it.
	r.tree.DispatchPointer(finger(core.PointerDown, 1, 70, 60))
	r.tree.DispatchPointer(finger(core.PointerUp, 1, 70, 60))
	if taps != 1 {
		t.Errorf("taps %d, want 1", taps)
	}
}

func TestInteractiveViewer_WheelPans(t *testing.T) {
	r := newViewerRig(t, InteractiveViewer{Child: canvas(nil)}, false)
	// A notch toward the user shows what is below: it glides there.
	wtest.Wheel(r.tree, -1)
	if !r.ctl.Moving() {
		t.Error("a notch did not glide")
	}
	r.settle()
	r.wantView(1, geom.Offset{X: 0, Y: -100})
	// A touchpad moves at once.
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: geom.Offset{X: 10, Y: 10}, Scroll: geom.Offset{X: 0.25, Y: -0.5}})
	r.wantView(1, geom.Offset{X: -25, Y: -150})
	// Shift turns the wheel sideways.
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: geom.Offset{X: 10, Y: 10}, Scroll: geom.Offset{Y: -0.5}, Modifiers: input.ModShift})
	r.wantView(1, geom.Offset{X: -75, Y: -150})
}

func TestInteractiveViewer_BoundedWheelAtTheEdgePassesOn(t *testing.T) {
	var outer int
	tree := wtest.TextTree(t, Listener{
		OnPointer: func(e *core.PointerEvent) {
			if e.Kind == core.PointerScroll {
				outer++
			}
		},
		Child: InteractiveViewer{Child: canvas(nil)},
	})
	wtest.Ops(tree, wtest.Screen)
	wtest.Wheel(tree, 1)
	if outer != 1 {
		t.Errorf("the wheel at the top edge reached the parent %d times, want 1", outer)
	}
	wtest.Wheel(tree, -1)
	if outer != 1 {
		t.Errorf("a wheel the viewer used reached the parent")
	}
}

func TestInteractiveViewer_Controller(t *testing.T) {
	r := newViewerRig(t, InteractiveViewer{Child: canvas(nil)}, false)
	r.ctl.Fit(false)
	// 1000×800 in 400×300: the height decides, 0.375, centered across.
	r.wantView(0.375, geom.Offset{X: 12.5, Y: 0})
	r.ctl.ZoomToRect(geom.Rect{X: 100, Y: 100, W: 200, H: 100}, false)
	r.wantView(2, geom.Offset{X: 200 - 200*2, Y: 150 - 150*2})
	r.ctl.Reset(false)
	r.wantView(1, geom.Offset{})
	r.ctl.ZoomBy(2, geom.Offset{X: 200, Y: 150}, false)
	r.wantView(2, geom.Offset{X: -200, Y: -150})
	r.ctl.PanBy(geom.Offset{X: 50, Y: -10}, false)
	r.wantView(2, geom.Offset{X: -150, Y: -160})
	if got := r.ctl.FromChild(r.ctl.ToChild(geom.Offset{X: 33, Y: 44})); !nearOffset(got, geom.Offset{X: 33, Y: 44}) {
		t.Errorf("ToChild and FromChild disagree: %v", got)
	}
	if m := r.ctl.Transform(); m.Apply(geom.Offset{X: 10, Y: 10}) != r.ctl.FromChild(geom.Offset{X: 10, Y: 10}) {
		t.Error("Transform disagrees with FromChild")
	}

	// Animated, the same ends are reached after a motion.
	n := r.changes
	r.ctl.Fit(true)
	if !r.ctl.Moving() {
		t.Fatal("an animated Fit did not move")
	}
	r.now += 8 * time.Millisecond
	r.tree.Tick(r.now)
	r.now += 8 * time.Millisecond
	r.tree.Tick(r.now)
	if s := r.ctl.Scale(); s >= 2 || s <= 0.375 {
		t.Errorf("halfway scale %v", s)
	}
	r.settle()
	r.wantView(0.375, geom.Offset{X: 12.5, Y: 0})
	if r.changes <= n+2 {
		t.Errorf("OnChange ran %d times during the motion", r.changes-n)
	}
	r.ctl.ZoomToRect(geom.Rect{X: 100, Y: 100, W: 200, H: 100}, true)
	r.settle()
	r.wantView(2, geom.Offset{X: -200, Y: -150})
	r.ctl.Reset(true)
	r.ctl.Stop()
	if r.ctl.Moving() {
		t.Error("Stop did not stop")
	}
}

func TestInteractiveViewer_ControllerBeforeTheViewer(t *testing.T) {
	var c ViewerController
	c.ZoomBy(2, geom.Offset{X: 10, Y: 10}, true)
	if c.Scale() != 2 || c.Offset() != (geom.Offset{X: -10, Y: -10}) {
		t.Errorf("unattached ZoomBy: %v %v", c.Scale(), c.Offset())
	}
	c.Fit(true)
	r := newViewerRig(t, InteractiveViewer{Controller: &c, Child: canvas(nil)}, false)
	r.wantView(0.375, geom.Offset{X: 12.5, Y: 0})
	// OnChange is held back from the layout to the next tick.
	if r.changes != 0 {
		t.Errorf("OnChange ran during the layout")
	}
	r.settle()
	if r.changes != 1 {
		t.Errorf("OnChange ran %d times, want 1", r.changes)
	}
	// A rectangle asked for before the layout: at 3, centered on the
	// rectangle would put the child at x = 50, which the boundary pulls back
	// to the left edge.
	var d ViewerController
	d.ZoomToRect(geom.Rect{X: 0, Y: 0, W: 100, H: 100}, false)
	r2 := newViewerRig(t, InteractiveViewer{Controller: &d, Child: canvas(nil)}, false)
	r2.wantView(3, geom.Offset{})
	// A controller keeps the view for a viewer built anew.
	r3 := newViewerRig(t, InteractiveViewer{Controller: &d, Child: canvas(nil)}, false)
	r3.wantView(3, geom.Offset{})
}

func TestInteractiveViewer_ResizeKeepsTheChildInBounds(t *testing.T) {
	r := newViewerRig(t, InteractiveViewer{Child: canvas(nil)}, false)
	r.ctl.Set(1, geom.Offset{X: -600, Y: -500}, false)
	r.size = geom.Size{W: 800, H: 600}
	r.frame()
	r.wantView(1, geom.Offset{X: -200, Y: -200})
	if r.layouts != 1 {
		t.Errorf("the child was laid out %d times, want 1: its room did not change", r.layouts)
	}
	n := r.changes
	r.settle()
	if r.changes != n+1 {
		t.Errorf("the clamp of the resize ran OnChange %d times, want 1", r.changes-n)
	}
}

func TestInteractiveViewer_ConstrainedChildFillsTheViewer(t *testing.T) {
	r := newViewerRig(t, InteractiveViewer{Constrained: true, Child: basic.SizedBox{Width: 1e4, Height: 1e4}}, false)
	r.ctl.Fit(false)
	r.wantView(1, geom.Offset{})
	r.size = geom.Size{W: 500, H: 300}
	r.frame()
	if r.layouts != 2 {
		t.Errorf("a constrained child was laid out %d times, want 2 after a resize", r.layouts)
	}
}

func TestInteractiveViewer_PanAndZoomDoNotLayOutTheChild(t *testing.T) {
	r := newViewerRig(t, InteractiveViewer{Child: canvas(nil)}, false)
	r.drag(geom.Offset{X: 300, Y: 250}, geom.Offset{X: 200, Y: 200}, 8, 200)
	r.frame()
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerPinch, Pos: geom.Offset{X: 100, Y: 100}, Pinch: 1.5})
	r.frame()
	r.ctl.Fit(true)
	r.settle()
	if r.layouts != 1 {
		t.Errorf("the child was laid out %d times, want 1", r.layouts)
	}
	if r.tree.NeedsBuild() {
		t.Error("a pan left a build behind")
	}
}

func TestInteractiveViewer_PaintsThroughTheTransform(t *testing.T) {
	r := newViewerRig(t, InteractiveViewer{Child: canvas(nil)}, false)
	r.ctl.Set(2, geom.Offset{X: -100, Y: -100}, false)
	var cv = wtest.Ops(r.tree, r.size)
	var red *geom.Rect
	for i := range cv {
		if cv[i].Color == wtest.Red {
			red = &cv[i].RRect.Rect
		}
	}
	if red == nil {
		t.Fatal("the button was not painted")
	}
	// A scale and a move take the fast path of the canvas, which places the
	// shapes in the window: (100, 100) of the child at 2 lands at (100, 100).
	if !wtest.NearRect(*red, geom.Rect{X: 100, Y: 100, W: 100, H: 100}) {
		t.Errorf("button painted at %v", *red)
	}
}

func TestInteractiveViewer_Keyboard(t *testing.T) {
	r := newViewerRig(t, InteractiveViewer{Boundary: ViewerUnbounded, Child: canvas(nil)}, false)
	wtest.Tap(r.tree, geom.Offset{X: 300, Y: 250})
	if r.tree.Focused() == nil {
		t.Fatal("a press on the background did not focus the viewer")
	}
	key := func(k input.KeyCode, m input.Modifiers) {
		r.tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: m})
		r.settle()
	}
	key(input.KeyRight, 0)
	key(input.KeyDown, 0)
	r.wantView(1, geom.Offset{X: -viewerKeyStep, Y: -viewerKeyStep})
	key(input.KeyLeft, 0)
	key(input.KeyUp, 0)
	r.wantView(1, geom.Offset{})
	r.tree.DispatchText("+")
	r.settle()
	// About the middle (200, 150).
	r.wantView(1.25, geom.Offset{X: -50, Y: -37.5})
	r.tree.DispatchText("-")
	r.settle()
	r.wantView(1, geom.Offset{})
	key(input.KeyEqual, input.ModControl)
	r.wantView(1.25, geom.Offset{X: -50, Y: -37.5})
	r.tree.DispatchText("0")
	r.settle()
	r.wantView(1, geom.Offset{})
	key(input.KeyMinus, input.ModControl)
	r.wantView(0.8, geom.Offset{X: 40, Y: 30})
	key(input.Key0, input.ModControl)
	r.wantView(1, geom.Offset{})
	// The ;+ key of JIS layouts and the + of the keypad zoom in too.
	for _, k := range []input.KeyCode{input.KeySemicolon, input.KeyPlus} {
		key(k, input.ModControl)
		r.wantView(1.25, geom.Offset{X: -50, Y: -37.5})
		key(input.Key0, input.ModControl)
		r.wantView(1, geom.Offset{})
	}
}

func TestInteractiveViewer_DoubleTapZoom(t *testing.T) {
	r := newViewerRig(t, InteractiveViewer{DoubleTapZoom: true, Child: canvas(nil)}, false)
	r.pointer(core.PointerDown, 300, 200)
	r.pointer(core.PointerUp, 300, 200)
	r.pointer(core.PointerDown, 300, 200)
	r.pointer(core.PointerUp, 300, 200)
	r.settle()
	r.wantView(2, geom.Offset{X: -300, Y: -200})
	// A second, separate double click goes back.
	r.clock += 1000
	r.pointer(core.PointerDown, 300, 200)
	r.pointer(core.PointerUp, 300, 200)
	r.pointer(core.PointerDown, 300, 200)
	r.pointer(core.PointerUp, 300, 200)
	r.settle()
	r.wantView(1, geom.Offset{})
	// Off by default.
	q := newViewerRig(t, InteractiveViewer{Child: canvas(nil)}, false)
	wtest.Tap(q.tree, geom.Offset{X: 300, Y: 200})
	wtest.Tap(q.tree, geom.Offset{X: 300, Y: 200})
	q.settle()
	q.wantView(1, geom.Offset{})
}

func TestInteractiveViewer_Semantics(t *testing.T) {
	r := newViewerRig(t, InteractiveViewer{Label: "図面", Child: canvas(nil)}, false)
	root, _ := r.tree.SemanticsTree()
	n := wtest.ByLabel(t, root, "図面")
	if n.Role != semantics.RoleGroup || n.Value != "100%" || n.Actions&(semantics.ActionIncrease|semantics.ActionDecrease) == 0 {
		t.Fatalf("node %+v", *n)
	}
	if !r.tree.PerformSemanticsAction(n.ID, semantics.ActionIncrease, core.SemanticsArg{}) {
		t.Fatal("increase was refused")
	}
	r.settle()
	root, _ = r.tree.SemanticsTree()
	if n = wtest.ByLabel(t, root, "図面"); n.Value != "125%" {
		t.Errorf("value %q after increase, want 125%%", n.Value)
	}
}

func TestInteractiveViewer_ZeroValue(t *testing.T) {
	tree := wtest.TextTree(t, InteractiveViewer{})
	wtest.Ops(tree, wtest.Screen)
	wtest.Tap(tree, geom.Offset{X: 10, Y: 10})
	wtest.Wheel(tree, 1)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerPinch, Pos: geom.Offset{X: 10, Y: 10}, Pinch: 2})
	wtest.Ops(tree, wtest.Screen)

	var c ViewerController
	if c.Scale() != 1 || c.Offset() != (geom.Offset{}) || !c.Transform().IsIdentity() {
		t.Errorf("zero controller: %v %v", c.Scale(), c.Offset())
	}
	if ViewerBounded.String() != "ViewerBounded" || ViewerUnbounded.String() != "ViewerUnbounded" {
		t.Error("boundary names")
	}
}

func TestInteractiveViewer_ControllerChangeMovesTheView(t *testing.T) {
	var a, b ViewerController
	b.Set(2, geom.Offset{X: -50, Y: -50}, false)
	var state *wtest.SwapState
	use := &a
	tree := wtest.TextTree(t, wtest.Swap{State: &state, Build: func() core.Widget {
		return InteractiveViewer{Controller: use, Child: canvas(nil)}
	}})
	wtest.Ops(tree, wtest.Screen)
	use = &b
	state.SetState(func() {})
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerPinch, Pos: geom.Offset{}, Pinch: 1.5})
	if a.Scale() != 1 || b.Scale() != 3 {
		t.Errorf("the pinch reached the old controller: a %v, b %v", a.Scale(), b.Scale())
	}
}

func TestInteractiveViewer_BadLimitsPanic(t *testing.T) {
	wtest.ExpectPanic(t, "MinScale <= MaxScale", func() {
		tree := core.NewTree(InteractiveViewer{MinScale: 4, MaxScale: 2})
		defer tree.Dispose()
		wtest.Ops(tree, wtest.Screen)
	})
	var c ViewerController
	wtest.ExpectPanic(t, "positive scale", func() { c.Set(0, geom.Offset{}, false) })
	wtest.ExpectPanic(t, "positive factor", func() { c.ZoomBy(-1, geom.Offset{}, false) })
	wtest.ExpectPanic(t, "area", func() { c.ZoomToRect(geom.Rect{W: 10}, false) })
}

func TestVelocityTracker(t *testing.T) {
	var v velocityTracker
	start := uint32(1<<32 - 30)
	for i := range 6 {
		v.add(geom.Offset{X: float32(i) * 10}, start+uint32(i*10))
	}
	// The clock wraps between the samples: 10 px per 10 ms is 1000 px/s.
	got := v.velocity(start + 50)
	if !near(got.X, 1000) || got.Y != 0 {
		t.Errorf("velocity %v, want 1000 across the wrap", got)
	}
	if got := v.velocity(start + 150); got != (geom.Offset{}) {
		t.Errorf("stale samples gave %v", got)
	}
}
