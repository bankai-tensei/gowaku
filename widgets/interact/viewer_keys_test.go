package interact

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

// appKeys is a viewer under a Focus that counts the keys that reach it, as the
// shortcuts of an app would.
type appKeys struct {
	r    *viewerRig
	keys []input.KeyCode
}

func newAppKeys(t *testing.T, v InteractiveViewer) *appKeys {
	t.Helper()
	a := &appKeys{}
	v.Controller = &ViewerController{}
	a.r = &viewerRig{t: t, size: wtest.Screen, ctl: v.Controller}
	a.r.tree = wtest.TextTree(t, Focus{
		OnKey: func(e *core.KeyEvent) bool {
			a.keys = append(a.keys, e.Key)
			return true
		},
		Child: basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1, ReduceMotion: true}, Child: v},
	})
	a.r.frame()
	return a
}

func (a *appKeys) key(k input.KeyCode) {
	a.r.tree.DispatchKey(core.KeyEvent{Key: k})
	a.r.frame()
}

func TestInteractiveViewer_ArrowsGoToTheAppWithNothingToPan(t *testing.T) {
	// The child is laid out at the size of the viewer, so it fits at a scale of 1.
	a := newAppKeys(t, InteractiveViewer{Constrained: true, Child: basic.SizedBox{Width: 400, Height: 300}})
	wtest.Tap(a.r.tree, geom.Offset{X: 200, Y: 150})
	if a.r.tree.Focused() == nil {
		t.Fatal("a press did not focus the viewer")
	}
	a.key(input.KeyRight)
	a.key(input.KeyLeft)
	if len(a.keys) != 2 {
		t.Fatalf("the app got %v, want Right and Left", a.keys)
	}
	a.r.wantView(1, geom.Offset{})

	// Zoomed in, the arrows pan, up to the edge.
	a.r.ctl.Set(2, geom.Offset{}, false)
	a.r.frame()
	a.key(input.KeyRight)
	if len(a.keys) != 2 {
		t.Errorf("an arrow that panned reached the app: %v", a.keys)
	}
	a.r.wantView(2, geom.Offset{X: -viewerKeyStep})
	a.key(input.KeyUp)
	if len(a.keys) != 3 {
		t.Errorf("an arrow at the edge did not reach the app: %v", a.keys)
	}
}

func TestInteractiveViewer_KeysZoomLeavesTheArrows(t *testing.T) {
	a := newAppKeys(t, InteractiveViewer{Keys: ViewerKeysZoom, Child: canvas(nil)})
	wtest.Tap(a.r.tree, geom.Offset{X: 300, Y: 250})
	a.key(input.KeyRight)
	a.key(input.KeyDown)
	if len(a.keys) != 2 {
		t.Errorf("the app got %v, want both arrows", a.keys)
	}
	a.r.wantView(1, geom.Offset{})
	a.r.tree.DispatchText("+")
	a.r.frame()
	if !near(a.r.ctl.Scale(), viewerKeyZoom) {
		t.Errorf("+ zoomed to %v, want %v", a.r.ctl.Scale(), viewerKeyZoom)
	}
	a.r.tree.DispatchKey(core.KeyEvent{Key: input.Key0, Modifiers: input.ModControl})
	a.r.frame()
	if len(a.keys) != 2 || !near(a.r.ctl.Scale(), 1) {
		t.Errorf("Ctrl+0 reached the app or left the scale at %v", a.r.ctl.Scale())
	}
}

func TestInteractiveViewer_KeysNoneTakesNoFocus(t *testing.T) {
	a := newAppKeys(t, InteractiveViewer{Keys: ViewerKeysNone, Child: canvas(nil)})
	a.key(input.KeyTab)
	if f := a.r.tree.Focused(); f != nil {
		t.Errorf("Tab focused %T", f)
	}
	wtest.Tap(a.r.tree, geom.Offset{X: 300, Y: 250})
	if f := a.r.tree.Focused(); f != nil {
		t.Errorf("a press focused %T", f)
	}
	// The press still pans.
	a.r.drag(geom.Offset{X: 300, Y: 250}, geom.Offset{X: 280, Y: 250}, 2, 200)
	a.r.wantView(1, geom.Offset{X: -20})
}

func TestInteractiveViewer_KeysNonePassesThePressFocusOn(t *testing.T) {
	focused := false
	tree := wtest.A11yTree(t, Focus{
		Enabled:       true,
		OnFocusChange: func(f bool) { focused = f },
		Child: basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1, ReduceMotion: true}, Child: InteractiveViewer{
			Keys: ViewerKeysNone,
			Child: Listener{
				OnPointer: func(e *core.PointerEvent) { e.Handled = e.Kind == core.PointerDown },
				Child:     basic.SizedBox{Width: 1000, Height: 800, Child: basic.Container{Color: wtest.Blue}},
			},
		}},
	})
	wtest.Tap(tree, geom.Offset{X: 300, Y: 250})
	if !focused {
		t.Errorf("a press on the child of a viewer without keys did not focus the Focus around it; focused %T", tree.Focused())
	}
}

func TestInteractiveViewer_WheelZooms(t *testing.T) {
	r := newViewerRig(t, InteractiveViewer{WheelZooms: true, Child: canvas(nil)}, false)
	r.ctl.Set(1.5, geom.Offset{X: -30, Y: -12}, false)
	r.frame()
	at := geom.Offset{X: 170, Y: 90}
	before := r.ctl.ToChild(at)
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: at, Scroll: geom.Offset{Y: 1}})
	if want := float32(1.5 * 1.1); geom.Abs32(r.ctl.Scale()-want) > 1e-4 {
		t.Errorf("a notch zoomed to %v, want %v", r.ctl.Scale(), want)
	}
	if got := r.ctl.ToChild(at); !nearOffset(got, before) {
		t.Errorf("the point under the pointer moved from %v to %v", before, got)
	}
	// Ctrl with the wheel zooms as before.
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: at, Scroll: geom.Offset{Y: -1}, Modifiers: input.ModControl})
	if !near(r.ctl.Scale(), 1.5) {
		t.Errorf("Ctrl and a notch back zoomed to %v, want 1.5", r.ctl.Scale())
	}
	// Shift with the wheel pans sideways.
	off := r.ctl.Offset()
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: at, Scroll: geom.Offset{Y: -0.5}, Modifiers: input.ModShift})
	if !near(r.ctl.Scale(), 1.5) || !near(r.ctl.Offset().X, off.X-50) || !near(r.ctl.Offset().Y, off.Y) {
		t.Errorf("Shift and the wheel moved the view to %v %v, want a pan of 50 to the left", r.ctl.Scale(), r.ctl.Offset())
	}
}

func TestInteractiveViewer_ListenerInTheChild(t *testing.T) {
	for _, pass := range []bool{false, true} {
		var presses int
		child := basic.Stack{Children: []core.Widget{
			basic.SizedBox{Width: 1000, Height: 800},
			Listener{
				OnPointer: func(e *core.PointerEvent) {
					if e.Kind == core.PointerDown {
						presses++
						if pass {
							e.Handled = false
						}
					}
				},
				Child: basic.SizedBox{Width: 1000, Height: 800},
			},
		}}
		r := newViewerRig(t, InteractiveViewer{Child: child}, true)
		r.drag(geom.Offset{X: 300, Y: 250}, geom.Offset{X: 250, Y: 250}, 5, 200)
		if presses != 1 {
			t.Errorf("pass %v: the Listener heard %d presses, want 1", pass, presses)
		}
		want := geom.Offset{}
		if pass {
			want = geom.Offset{X: -50}
		}
		r.wantView(1, want)
	}
}
