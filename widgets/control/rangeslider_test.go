package control

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

// The rig's slider is 218 wide at the top left, so the center of a thumb
// travels from 9 to 209, and the track's middle is at 11.
const (
	rangeLeft   = 9
	rangeLength = 200
	rangeMid    = 11
)

// rangeAt returns the position of the place f along the rig's slider.
func rangeAt(f float32) geom.Offset { return geom.Offset{X: rangeLeft + rangeLength*f, Y: rangeMid} }

// rangeRig is a range slider whose values live in the test, as an app keeps
// them.
type rangeRig struct {
	t          *testing.T
	tree       *core.Tree
	swap       func(core.Widget)
	low, high  float32
	changes    int
	slider     RangeSlider
	rebuilding bool
}

func newRangeRig(t *testing.T, s RangeSlider) *rangeRig {
	t.Helper()
	r := &rangeRig{t: t, low: s.Low, high: s.High, slider: s, rebuilding: true}
	r.tree, r.swap = wtest.SwapTree(t, r.build())
	r.frame()
	return r
}

func (r *rangeRig) build() core.Widget {
	s := r.slider
	s.Low, s.High = r.low, r.high
	if s.OnChange == nil {
		s.OnChange = func(low, high float32) {
			r.changes++
			r.low, r.high = low, high
			if r.rebuilding {
				r.swap(r.build())
			}
		}
	}
	return basic.Align{Alignment: layout.AlignTopLeft, Child: basic.SizedBox{Width: 218, Child: s}}
}

func (r *rangeRig) frame() { wtest.Ops(r.tree, wtest.Screen) }

func (r *rangeRig) pointer(kind core.PointerKind, at geom.Offset) {
	r.tree.DispatchPointer(core.PointerEvent{Kind: kind, Pos: at})
	r.frame()
}

func (r *rangeRig) drag(from, to geom.Offset) {
	r.pointer(core.PointerDown, from)
	r.pointer(core.PointerMove, to)
	r.pointer(core.PointerUp, to)
}

func (r *rangeRig) key(k input.KeyCode) {
	r.tree.DispatchKey(core.KeyEvent{Key: k})
	r.frame()
}

func (r *rangeRig) want(low, high float32) {
	r.t.Helper()
	if !near(r.low, low) || !near(r.high, high) {
		r.t.Errorf("range %v–%v, want %v–%v", r.low, r.high, low, high)
	}
}

func near(a, b float32) bool { return geom.Abs32(a-b) < 1e-4 }

// thumbs returns the semantics of the two thumbs.
func (r *rangeRig) thumbs() []*semantics.Node {
	var out []*semantics.Node
	root, _ := r.tree.SemanticsTree()
	root.Walk(func(n *semantics.Node) bool {
		if n.Role == semantics.RoleSlider {
			out = append(out, n)
		}
		return true
	})
	return out
}

func TestRangeSlider_DragsEachThumb(t *testing.T) {
	r := newRangeRig(t, RangeSlider{Low: 0.2, High: 0.8, Step: 0.1, Snap: true})
	r.drag(rangeAt(0.8), rangeAt(1).Add(geom.Offset{X: 50}))
	r.want(0.2, 1)
	r.drag(rangeAt(0.2), rangeAt(0.43))
	r.want(0.4, 1)
	if r.changes != 2 {
		t.Errorf("one change per drag that snaps once, got %d", r.changes)
	}
}

func TestRangeSlider_ThumbsDoNotCross(t *testing.T) {
	r := newRangeRig(t, RangeSlider{Low: 0.2, High: 0.6, MinGap: 0.1})
	r.drag(rangeAt(0.2), rangeAt(0.95))
	r.want(0.5, 0.6)
	r.drag(rangeAt(0.6), rangeAt(0))
	r.want(0.5, 0.6)
	// Without a gap the thumbs may meet, but not pass.
	r = newRangeRig(t, RangeSlider{Low: 0.2, High: 0.6})
	r.drag(rangeAt(0.6), rangeAt(0.05))
	r.want(0.2, 0.2)
}

// TestRangeSlider_StaysWithinTheLimitsNearAnEnd gives thumbs closer than
// MinGap near an end and drags each to that end: the values stay within Min
// and Max, and the other thumb stays where it is.
func TestRangeSlider_StaysWithinTheLimitsNearAnEnd(t *testing.T) {
	r := newRangeRig(t, RangeSlider{Low: 0.05, High: 0.1, MinGap: 0.2})
	r.drag(rangeAt(0.05), rangeAt(0).Add(geom.Offset{X: -50}))
	r.want(0, 0.1)
	r = newRangeRig(t, RangeSlider{Low: 0.9, High: 0.95, MinGap: 0.2})
	r.drag(rangeAt(0.95), rangeAt(1).Add(geom.Offset{X: 50}))
	r.want(0.9, 1)
	// The same with limits other than 0 and 1, and in the middle of a drag.
	r = newRangeRig(t, RangeSlider{Low: 11, High: 12, Min: 10, Max: 20, MinGap: 3})
	r.pointer(core.PointerDown, rangeAt(0.1))
	r.pointer(core.PointerMove, rangeAt(0))
	r.want(10, 12)
	r.pointer(core.PointerUp, rangeAt(0))
}

func TestRangeSlider_DraggingTheBarMovesTheRange(t *testing.T) {
	r := newRangeRig(t, RangeSlider{Low: 0.2, High: 0.4, Step: 0.1, Snap: true})
	r.pointer(core.PointerDown, rangeAt(0.3))
	if r.changes != 0 {
		t.Error("a press on the bar alone changes nothing")
	}
	r.pointer(core.PointerMove, rangeAt(0.5))
	r.want(0.4, 0.6)
	r.pointer(core.PointerMove, rangeAt(1).Add(geom.Offset{X: 100}))
	r.want(0.8, 1)
	r.pointer(core.PointerUp, rangeAt(1))
}

func TestRangeSlider_PressOutsideBringsTheNearerThumb(t *testing.T) {
	r := newRangeRig(t, RangeSlider{Low: 40, High: 60, Min: 0, Max: 100})
	r.pointer(core.PointerDown, rangeAt(0.05))
	r.want(5, 60)
	r.pointer(core.PointerUp, rangeAt(0.05))
	r.pointer(core.PointerDown, rangeAt(0.9))
	r.want(5, 90)
	r.pointer(core.PointerUp, rangeAt(0.9))
}

func TestRangeSlider_ThumbsAtOnePlaceFollowTheDrag(t *testing.T) {
	r := newRangeRig(t, RangeSlider{Low: 0.5, High: 0.5})
	r.drag(rangeAt(0.5), rangeAt(0.7))
	r.want(0.5, 0.7)
	r = newRangeRig(t, RangeSlider{Low: 0.5, High: 0.5})
	r.drag(rangeAt(0.5), rangeAt(0.3))
	r.want(0.3, 0.5)
}

func TestRangeSlider_KeysMoveTheFocusedThumb(t *testing.T) {
	r := newRangeRig(t, RangeSlider{Low: 20, High: 60, Min: 0, Max: 100, Step: 5, MinGap: 10})
	r.key(input.KeyTab)
	r.key(input.KeyRight)
	r.want(25, 60)
	r.key(input.KeyEnd)
	r.want(50, 60)
	r.key(input.KeyPageDown)
	r.want(0, 60)
	r.key(input.KeyUp)
	r.want(5, 60)
	r.key(input.KeyTab)
	r.key(input.KeyHome)
	r.want(5, 15)
	r.key(input.KeyLeft)
	r.want(5, 15)
	r.key(input.KeyPageUp)
	r.want(5, 65)
	r.key(input.KeyEnd)
	r.want(5, 100)
	// Tab goes round to the low thumb.
	r.key(input.KeyTab)
	r.key(input.KeyRight)
	r.want(10, 100)
}

func TestRangeSlider_PressFocusesTheThumb(t *testing.T) {
	r := newRangeRig(t, RangeSlider{Low: 0.2, High: 0.8, Step: 0.1})
	r.drag(rangeAt(0.8), rangeAt(0.8))
	r.key(input.KeyLeft)
	r.want(0.2, 0.7)
	r.drag(rangeAt(0.2), rangeAt(0.2))
	r.key(input.KeyRight)
	r.want(0.3, 0.7)
}

func TestRangeSlider_RTLMirrors(t *testing.T) {
	r := newRangeRig(t, RangeSlider{Low: 0.2, High: 0.6, Step: 0.1, RTL: true})
	// The low thumb is at the right.
	r.drag(rangeAt(0.8), rangeAt(1))
	r.want(0, 0.6)
	r.drag(rangeAt(0.4), rangeAt(0.3))
	r.want(0, 0.7)
	r.key(input.KeyTab)
	r.key(input.KeyLeft)
	r.want(0.1, 0.7)
	r.key(input.KeyRight)
	r.want(0, 0.7)
	r.key(input.KeyUp)
	r.want(0.1, 0.7)
	n := r.thumbs()
	if len(n) != 2 || n[0].Rect.X <= n[1].Rect.X {
		t.Errorf("the low thumb must be right of the high one: %+v", n)
	}
}

func TestRangeSlider_Semantics(t *testing.T) {
	r := newRangeRig(t, RangeSlider{Label: "価格", Low: 20, High: 60, Min: 0, Max: 100, Step: 5, MinGap: 10})
	var group *semantics.Node
	root, _ := r.tree.SemanticsTree()
	root.Walk(func(n *semantics.Node) bool {
		if n.Role == semantics.RoleGroup && n.Label == "価格" {
			group = n
		}
		return group == nil
	})
	if group == nil {
		t.Fatal("no group")
	}
	n := r.thumbs()
	if len(n) != 2 {
		t.Fatalf("got %d thumbs", len(n))
	}
	low, high := n[0], n[1]
	if low.Label != "Minimum" || high.Label != "Maximum" {
		t.Errorf("labels %q %q", low.Label, high.Label)
	}
	if low.Range != (semantics.Range{Min: 0, Max: 50, Value: 20, Step: 5}) {
		t.Errorf("low %+v", low.Range)
	}
	if high.Range != (semantics.Range{Min: 30, Max: 100, Value: 60, Step: 5}) {
		t.Errorf("high %+v", high.Range)
	}
	if low.Actions&(semantics.ActionIncrease|semantics.ActionDecrease|semantics.ActionSetValue) == 0 {
		t.Errorf("actions %v", low.Actions)
	}
	if !r.tree.PerformSemanticsAction(high.ID, semantics.ActionDecrease, core.SemanticsArg{}) {
		t.Fatal("decrease failed")
	}
	r.frame()
	r.want(20, 55)
	if !r.tree.PerformSemanticsAction(r.thumbs()[0].ID, semantics.ActionSetValue, core.SemanticsArg{Number: 90}) {
		t.Fatal("set value failed")
	}
	r.frame()
	r.want(45, 55)
}

func TestRangeSlider_Disabled(t *testing.T) {
	tree := wtest.TextTree(t, basic.Align{Child: basic.SizedBox{Width: 100, Child: RangeSlider{Low: 0.2, High: 0.7}}})
	if got := wtest.Ops(tree, wtest.Screen); len(got) == 0 {
		t.Error("a disabled slider must still show")
	}
	var thumbs int
	tree.Semantics().Walk(func(n *semantics.Node) bool {
		if n.Role == semantics.RoleSlider {
			thumbs++
			if n.State&semantics.Disabled == 0 {
				t.Errorf("thumb %+v is not disabled", n)
			}
		}
		return true
	})
	if thumbs != 2 {
		t.Errorf("got %d thumbs", thumbs)
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	if tree.Focused() != nil {
		t.Error("a disabled slider takes no focus")
	}
}

func TestRangeSlider_ZeroValueWorks(t *testing.T) {
	tree := wtest.TextTree(t, RangeSlider{})
	if got := wtest.Ops(tree, wtest.Screen); len(got) == 0 {
		t.Error("the zero value must show")
	}
}

func TestRangeSlider_LowAboveHighIsPutInOrder(t *testing.T) {
	r := newRangeRig(t, RangeSlider{Low: 0.8, High: 0.2, Step: 0.1})
	r.key(input.KeyTab)
	r.key(input.KeyRight)
	r.want(0.3, 0.8)
}

// A drag of a thumb moves it without building again, and a frame of that
// drag allocates nothing: the move as the render object gets it, the layout of
// the thumbs and the paint. (core.Tree.DispatchPointer itself copies each event
// it delivers to the heap, once per event and for every pointer target alike,
// so the move is handed to the render object here.)
func TestRangeSlider_DragDoesNotAllocate(t *testing.T) {
	r := newRangeRig(t, RangeSlider{Low: 0.2, High: 0.8})
	r.rebuilding = false
	now := time.Second
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: rangeAt(0.8)})
	// The press sinks the thumb on a spring; let it come to rest.
	wtest.Settled(t, r.tree, wtest.Screen, &now)
	ro := findRangeSlider(r.tree.RootRenderObject())
	if ro == nil {
		t.Fatal("no range slider")
	}
	var cv paint.Canvas
	ev := core.PointerEvent{Kind: core.PointerMove}
	x := float32(0)
	step := func() {
		x = 1 - x
		ev.Pos = rangeAt(0.5 + 0.3*x)
		ro.HandlePointer(&ev)
		r.tree.Build()
		r.tree.Layout(geom.Tight(wtest.Screen))
		cv.Reset()
		r.tree.Paint(&cv)
	}
	step()
	before := r.changes
	if allocs := testing.AllocsPerRun(100, step); allocs != 0 {
		t.Errorf("a frame of a drag allocates %v times", allocs)
	}
	if r.changes == before || !near(r.high, 0.5) && !near(r.high, 0.8) {
		t.Errorf("the drag must report its changes: %d, %v", r.changes-before, r.high)
	}
	if r.tree.NeedsBuild() {
		t.Error("a drag must not build the slider again")
	}
}

// BenchmarkRangeSlider_Drag is one frame of a drag of a thumb: the move as the
// render object gets it, the layout of the thumbs and the paint.
func BenchmarkRangeSlider_Drag(b *testing.B) {
	low, high := float32(0.2), float32(0.8)
	tree := wtest.TextTree(b, basic.Align{Alignment: layout.AlignTopLeft, Child: basic.SizedBox{Width: 218, Child: RangeSlider{
		Low: low, High: high, OnChange: func(l, h float32) { low, high = l, h },
	}}})
	wtest.Ops(tree, wtest.Screen)
	now := time.Second
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: rangeAt(0.8)})
	wtest.Settled(b, tree, wtest.Screen, &now)
	var cv paint.Canvas
	b.ReportAllocs()
	b.ResetTimer()
	ro := findRangeSlider(tree.RootRenderObject())
	ev := core.PointerEvent{Kind: core.PointerMove}
	for i := range b.N {
		ev.Pos = rangeAt(0.5 + 0.3*float32(i&1))
		ro.HandlePointer(&ev)
		tree.Layout(geom.Tight(wtest.Screen))
		cv.Reset()
		tree.Paint(&cv)
	}
}
