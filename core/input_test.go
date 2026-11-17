package core

import (
	"time"

	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
)

// probe is a leaf or a box around one child that records what reaches it.
type probe struct {
	StatelessBase
	name   string
	log    *[]string
	handle bool
	focus  bool
	order  int
	cursor input.Cursor
	size   geom.Size
	child  Widget
}

func (p probe) CreateRenderObject() RenderObject {
	return &probeObject{w: p}
}

func (p probe) UpdateRenderObject(ro RenderObject) { ro.(*probeObject).w = p }

func (p probe) ChildWidget() Widget { return p.child }

type probeObject struct {
	RenderBase
	w     probe
	child RenderObject
}

func (r *probeObject) SetChild(c RenderObject) { r.child = c }

func (r *probeObject) ChildCount() int {
	if r.child == nil {
		return 0
	}
	return 1
}

func (r *probeObject) ChildAt(int) RenderObject { return r.child }

func (r *probeObject) Layout(c geom.Constraints) geom.Size {
	if r.child != nil {
		LayoutChild(r.child, geom.Loose(r.w.size))
		r.child.SetOffset(geom.Offset{X: 10, Y: 10})
	}
	return c.Constrain(r.w.size)
}

func (r *probeObject) Paint(*paint.Canvas, geom.Offset) {}

func (r *probeObject) note(s string) { *r.w.log = append(*r.w.log, r.w.name+":"+s) }

func (r *probeObject) HandlePointer(e *PointerEvent) {
	names := map[PointerKind]string{PointerDown: "down", PointerMove: "move", PointerUp: "up", PointerScroll: "scroll"}
	r.note(names[e.Kind])
	e.Handled = r.w.handle
}

func (r *probeObject) HandleHover(inside bool) {
	if inside {
		r.note("enter")
	} else {
		r.note("exit")
	}
}

func (r *probeObject) MouseCursor() input.Cursor { return r.w.cursor }

func (r *probeObject) CanFocus() bool { return r.w.focus }

func (r *probeObject) FocusOrder() int { return r.w.order }

func (r *probeObject) HandleKey(*KeyEvent) bool {
	r.note("key")
	return false
}

func (r *probeObject) HandleText(s string) { r.note("text " + s) }

func (r *probeObject) HandleComposition(*input.Composition) { r.note("composition") }

func (r *probeObject) FocusChanged(focused bool) {
	if focused {
		r.note("focus")
	} else {
		r.note("blur")
	}
}

type recordingHost struct {
	noHost
	cursor input.Cursor
	ime    []bool
}

func (h *recordingHost) SetCursor(c input.Cursor) { h.cursor = c }

func (h *recordingHost) SetIMEEnabled(enabled bool) { h.ime = append(h.ime, enabled) }

func inputTree(t *testing.T, root Widget) *Tree {
	t.Helper()
	tree := NewTree(root)
	t.Cleanup(tree.Dispose)
	tree.Build()
	tree.Layout(geom.Tight(geom.Size{W: 100, H: 100}))
	return tree
}

func nested(log *[]string, outerHandles, innerHandles bool) Widget {
	return probe{
		name: "outer", log: log, handle: outerHandles, size: geom.Size{W: 100, H: 100},
		child: probe{name: "inner", log: log, handle: innerHandles, size: geom.Size{W: 20, H: 20}},
	}
}

func joined(log []string) string {
	s := ""
	for i, l := range log {
		if i > 0 {
			s += " "
		}
		s += l
	}
	return s
}

func at(x, y float32) geom.Offset { return geom.Offset{X: x, Y: y} }

func TestPointer_DeepestHandlerTakesThePress(t *testing.T) {
	var log []string
	tree := inputTree(t, nested(&log, true, true))
	tree.DispatchPointer(PointerEvent{Kind: PointerDown, Pos: at(15, 15)})
	tree.DispatchPointer(PointerEvent{Kind: PointerMove, Pos: at(90, 90)})
	tree.DispatchPointer(PointerEvent{Kind: PointerUp, Pos: at(90, 90)})
	// The move also reports the hover, which is not part of the press.
	if got, want := joined(log), "inner:down outer:enter inner:move inner:up"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestPointer_UnhandledPressGoesToTheParent(t *testing.T) {
	var log []string
	tree := inputTree(t, nested(&log, true, false))
	tree.DispatchPointer(PointerEvent{Kind: PointerDown, Pos: at(15, 15)})
	if got := joined(log); got != "inner:down outer:down" {
		t.Errorf("got %s", got)
	}
}

func TestPointer_PositionIsLocal(t *testing.T) {
	var pos geom.Offset
	inner := localProbe{got: &pos}
	tree := inputTree(t, probe{log: &[]string{}, size: geom.Size{W: 100, H: 100}, child: inner})
	tree.DispatchPointer(PointerEvent{Kind: PointerDown, Pos: at(15, 17)})
	if pos != at(5, 7) {
		t.Errorf("got %v", pos)
	}
}

type localProbe struct {
	StatelessBase
	got *geom.Offset
}

func (l localProbe) CreateRenderObject() RenderObject { return &localObject{got: l.got} }

func (localProbe) UpdateRenderObject(RenderObject) {}

type localObject struct {
	RenderBase
	got *geom.Offset
}

func (r *localObject) Layout(c geom.Constraints) geom.Size {
	return c.Constrain(geom.Size{W: 20, H: 20})
}

func (r *localObject) Paint(*paint.Canvas, geom.Offset) {}

func (r *localObject) HandlePointer(e *PointerEvent) { *r.got = e.Pos }

func TestPointer_ClickCount(t *testing.T) {
	var clicks []int
	tree := inputTree(t, clickProbe{got: &clicks})
	press := func(pos geom.Offset, time uint32) {
		tree.DispatchPointer(PointerEvent{Kind: PointerDown, Pos: pos, Time: time})
		tree.DispatchPointer(PointerEvent{Kind: PointerUp, Pos: pos, Time: time})
	}
	press(at(5, 5), 1000)
	press(at(6, 5), 1200)
	press(at(6, 5), 1400)
	press(at(6, 5), 2000)   // too late
	press(at(50, 50), 2100) // too far
	want := []int{1, 2, 3, 1, 1}
	if len(clicks) != len(want) {
		t.Fatalf("got %v", clicks)
	}
	for i := range want {
		if clicks[i] != want[i] {
			t.Fatalf("got %v, want %v", clicks, want)
		}
	}
}

type clickProbe struct {
	StatelessBase
	got *[]int
}

func (c clickProbe) CreateRenderObject() RenderObject { return &clickObject{got: c.got} }

func (clickProbe) UpdateRenderObject(RenderObject) {}

type clickObject struct {
	RenderBase
	got *[]int
}

func (r *clickObject) Layout(c geom.Constraints) geom.Size {
	return c.Constrain(geom.Size{W: 100, H: 100})
}

func (r *clickObject) Paint(*paint.Canvas, geom.Offset) {}

func (r *clickObject) HandlePointer(e *PointerEvent) {
	if e.Kind == PointerDown {
		*r.got = append(*r.got, e.Clicks)
		e.Handled = true
	}
}

func TestHover_EnterExitAndLeave(t *testing.T) {
	var log []string
	tree := inputTree(t, nested(&log, false, false))
	tree.DispatchPointer(PointerEvent{Kind: PointerMove, Pos: at(50, 50)})
	if got := joined(log); got != "outer:enter" {
		t.Errorf("got %s", got)
	}
	log = nil
	tree.DispatchPointer(PointerEvent{Kind: PointerMove, Pos: at(15, 15)})
	if got := joined(log); got != "inner:enter" {
		t.Errorf("got %s", got)
	}
	log = nil
	tree.PointerLeft()
	if got := joined(log); got != "inner:exit outer:exit" {
		t.Errorf("got %s", got)
	}
}

func TestCursor_DeepestSourceWins(t *testing.T) {
	var log []string
	tree := NewTree(probe{
		name: "outer", log: &log, size: geom.Size{W: 100, H: 100}, cursor: input.CursorHand,
		child: probe{name: "inner", log: &log, size: geom.Size{W: 20, H: 20}, cursor: input.CursorIBeam},
	})
	t.Cleanup(tree.Dispose)
	host := &recordingHost{}
	tree.SetHost(host)
	tree.Build()
	tree.Layout(geom.Tight(geom.Size{W: 100, H: 100}))
	tree.DispatchPointer(PointerEvent{Kind: PointerMove, Pos: at(15, 15)})
	if host.cursor != input.CursorIBeam {
		t.Errorf("got %d", host.cursor)
	}
	tree.DispatchPointer(PointerEvent{Kind: PointerMove, Pos: at(50, 50)})
	if host.cursor != input.CursorHand {
		t.Errorf("got %d", host.cursor)
	}
}

// focusRow is two focusable leaves, one unfocusable, and a third focusable one.
func focusRow(log *[]string) Widget {
	leaf := func(name string, focus bool, child Widget) probe {
		return probe{name: name, log: log, focus: focus, size: geom.Size{W: 100, H: 100}, child: child}
	}
	return leaf("a", true, leaf("b", false, leaf("c", true, nil)))
}

func TestFocus_TabCycles(t *testing.T) {
	var log []string
	tree := inputTree(t, focusRow(&log))
	tab := KeyEvent{Key: input.KeyTab}
	tree.DispatchKey(tab)
	tree.DispatchKey(tab)
	tree.DispatchKey(tab)
	if got := joined(log); got != "a:focus a:key a:blur c:focus c:key b:key a:key c:blur a:focus" {
		t.Errorf("got %s", got)
	}
}

func TestFocus_ShiftTabGoesBack(t *testing.T) {
	var log []string
	tree := inputTree(t, focusRow(&log))
	tree.DispatchKey(KeyEvent{Key: input.KeyTab, Modifiers: input.ModShift})
	if got := joined(log); got != "c:focus" {
		t.Errorf("got %s", got)
	}
}

func TestFocus_TextGoesToTheFocused(t *testing.T) {
	var log []string
	tree := inputTree(t, focusRow(&log))
	tree.DispatchText("lost")
	tree.DispatchKey(KeyEvent{Key: input.KeyTab})
	log = nil
	tree.DispatchText("あ")
	tree.DispatchComposition(&input.Composition{})
	if got := joined(log); got != "a:text あ a:composition" {
		t.Errorf("got %s", got)
	}
}

func TestFocus_PressOnNothingClears(t *testing.T) {
	var log []string
	tree := inputTree(t, focusRow(&log))
	tree.DispatchKey(KeyEvent{Key: input.KeyTab})
	tree.DispatchPointer(PointerEvent{Kind: PointerDown, Pos: at(50, 50)})
	if tree.Focused() != nil {
		t.Error("focus must be gone")
	}
}

func TestFocus_PressSkipsTargetsThatCannotFocus(t *testing.T) {
	var log []string
	tree := inputTree(t, probe{
		name: "outer", log: &log, focus: true, size: geom.Size{W: 100, H: 100},
		child: probe{
			name: "middle", log: &log, size: geom.Size{W: 50, H: 50},
			child: probe{name: "inner", log: &log, handle: true, size: geom.Size{W: 20, H: 20}},
		},
	})
	tree.DispatchPointer(PointerEvent{Kind: PointerDown, Pos: at(25, 25)})
	if got, want := joined(log), "inner:down outer:focus"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestFocus_WindowBlurKeepsTheTarget(t *testing.T) {
	var log []string
	tree := inputTree(t, focusRow(&log))
	tree.DispatchKey(KeyEvent{Key: input.KeyTab})
	focused := tree.Focused()
	log = nil
	tree.SetWindowFocused(false)
	tree.SetWindowFocused(true)
	if got := joined(log); got != "a:blur a:focus" {
		t.Errorf("got %s", got)
	}
	if tree.Focused() != focused {
		t.Error("the target must stay")
	}
}

// switcher replaces its child through SetState.
type switcher struct {
	StatefulBase
	first Widget
	state **switcherState
}

type switcherState struct {
	StateBase
	child Widget
}

func (s switcher) CreateState() State {
	*s.state = &switcherState{child: s.first}
	return *s.state
}

func (s *switcherState) Build(BuildContext) Widget { return s.child }

func TestFocus_RemovedTargetDropsFocus(t *testing.T) {
	var log []string
	var state *switcherState
	tree := NewTree(switcher{first: probe{name: "a", log: &log, focus: true, size: geom.Size{W: 10, H: 10}}, state: &state})
	t.Cleanup(tree.Dispose)
	host := &recordingHost{}
	tree.SetHost(host)
	tree.Build()
	tree.Layout(geom.Tight(geom.Size{W: 100, H: 100}))
	tree.DispatchKey(KeyEvent{Key: input.KeyTab})
	log = nil
	state.SetState(func() { state.child = localProbe{got: &geom.Offset{}} })
	tree.Build()
	if tree.Focused() != nil {
		t.Error("focus must be gone")
	}
	if len(log) != 0 {
		t.Errorf("a removed object must hear nothing, got %v", log)
	}
	if len(host.ime) != 1 || host.ime[0] {
		t.Errorf("the IME must go off, got %v", host.ime)
	}
}

func TestPointer_RemovedTargetGetsNothing(t *testing.T) {
	var log []string
	var state *switcherState
	tree := NewTree(switcher{first: probe{name: "a", log: &log, handle: true, size: geom.Size{W: 100, H: 100}}, state: &state})
	t.Cleanup(tree.Dispose)
	tree.Build()
	tree.Layout(geom.Tight(geom.Size{W: 100, H: 100}))
	tree.DispatchPointer(PointerEvent{Kind: PointerDown, Pos: at(5, 5)})
	state.SetState(func() { state.child = localProbe{got: &geom.Offset{}} })
	tree.Build()
	log = nil
	tree.DispatchPointer(PointerEvent{Kind: PointerUp, Pos: at(5, 5)})
	if len(log) != 0 {
		t.Errorf("got %v", log)
	}
}

func TestHost_DefaultDoesNothing(t *testing.T) {
	var b RenderBase
	b.Host().SetCursor(input.CursorHand)
	if b.Host().ClipboardText() != "" {
		t.Error("want no clipboard")
	}
}

func TestTicker_SleepingWakesTheTreeOnlyOnTime(t *testing.T) {
	h := newHarness(leaf{})
	tree := h.tree
	defer tree.Dispose()
	calls := 0
	k := h.state.NewTicker(func(time.Duration) { calls++ })
	k.Start()
	k.SleepUntil(time.Second)
	if tree.Ticking() {
		t.Error("a sleeping ticker must not ask for frames")
	}
	if wake, ok := tree.NextWake(); !ok || wake != time.Second {
		t.Errorf("got %v, %v", wake, ok)
	}
	if tree.Due(time.Second - 1) {
		t.Error("not due yet")
	}
	tree.Tick(time.Second - 1)
	if calls != 0 {
		t.Errorf("got %d calls", calls)
	}
	tree.Tick(time.Second)
	if calls != 1 || !tree.Ticking() {
		t.Errorf("got %d calls", calls)
	}
	k.Stop()
}

func TestTicker_PeriodicKeepsToItsGrid(t *testing.T) {
	h := newHarness(leaf{})
	tree := h.tree
	defer tree.Dispose()
	var calls []time.Duration
	k := h.state.NewPeriodicTicker(time.Second, func(now time.Duration) { calls = append(calls, now) })
	k.Start()
	if !tree.Ticking() {
		t.Fatal("the first call must come on the next frame")
	}
	tree.Tick(300 * time.Millisecond)
	if tree.Ticking() {
		t.Error("a periodic ticker must sleep between calls")
	}
	if wake, ok := tree.NextWake(); !ok || wake != 1300*time.Millisecond {
		t.Errorf("wake %v, %v", wake, ok)
	}
	// A late frame keeps the grid.
	tree.Tick(1350 * time.Millisecond)
	if wake, _ := tree.NextWake(); wake != 2300*time.Millisecond {
		t.Errorf("wake %v after a late frame", wake)
	}
	// A stall skips what it missed instead of catching up.
	tree.Tick(5700 * time.Millisecond)
	if wake, _ := tree.NextWake(); wake != 6300*time.Millisecond {
		t.Errorf("wake %v after a stall", wake)
	}
	if len(calls) != 3 {
		t.Errorf("got %d calls", len(calls))
	}
	// Start begins a new grid.
	k.Start()
	tree.Tick(6000 * time.Millisecond)
	if wake, _ := tree.NextWake(); wake != 7000*time.Millisecond {
		t.Errorf("wake %v after Start", wake)
	}
}

func TestTicker_PeriodicNeedsAPeriod(t *testing.T) {
	h := newHarness(leaf{})
	defer h.tree.Dispose()
	defer func() {
		if recover() == nil {
			t.Error("want a panic")
		}
	}()
	h.state.NewPeriodicTicker(0, func(time.Duration) {})
}

func TestTicker_StopsWhenItsStateLeaves(t *testing.T) {
	var k *Ticker
	child := stateful{id: "a", build: func(s *testState, _ BuildContext) Widget {
		if k == nil {
			k = s.NewTicker(func(time.Duration) {})
			k.Start()
		}
		return leaf{w: 1, h: 1}
	}}
	h := newHarness(child)
	defer h.tree.Dispose()
	h.tree.Build()
	if !k.Running() {
		t.Fatal("the ticker should run")
	}
	h.set(leaf{w: 1, h: 1})
	if k.Running() || h.tree.Ticking() {
		t.Error("the ticker must stop with its State")
	}
}
