package editor

import (
	"strings"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

// splitHarness shows one controller in two editors side by side, each 400
// wide.
type splitHarness struct {
	t           testing.TB
	tree        *core.Tree
	host        *wtest.FakeHost
	ctl         *Controller
	left, right *View
	size        geom.Size
	now         time.Duration
}

func newSplit(t testing.TB, ctl *Controller, left, right *View, e Editor) *splitHarness {
	t.Helper()
	l, r := e, e
	l.Controller, l.View, l.Label = ctl, left, "left"
	r.Controller, r.View, r.Label = ctl, right, "right"
	row := basic.Row{Cross: layout.CrossStretch, Children: []core.Widget{basic.Expanded{Child: l}, basic.Expanded{Child: r}}}
	h := &splitHarness{t: t, ctl: ctl, left: left, right: right, host: &wtest.FakeHost{}, size: geom.Size{W: 800, H: 300}, tree: wtest.TextTree(t, row)}
	h.tree.SetHost(h.host)
	h.frame()
	return h
}

func (h *splitHarness) frame() string { return testutil.RenderTree(h.tree, h.size) }

// tick runs the tickers a frame later and draws the frame.
func (h *splitHarness) tick() {
	h.now += 16 * time.Millisecond
	h.tree.Tick(h.now)
	h.frame()
}

// settle ticks until no motion is left, within two seconds.
func (h *splitHarness) settle() {
	for range 120 {
		h.tick()
	}
}

func (h *splitHarness) focus(v *View) {
	v.r.RequestFocus()
	h.frame()
}

func (h *splitHarness) key(k input.KeyCode, mods input.Modifiers) {
	h.tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: mods})
	h.frame()
}

// wheel turns the wheel over the middle of the left or right editor.
func (h *splitHarness) wheel(right bool, scroll geom.Offset, mods input.Modifiers) {
	at := geom.Offset{X: 200, Y: 150}
	if right {
		at.X += 400
	}
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: at})
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: at, Scroll: scroll, Modifiers: mods})
	h.frame()
}

func caretRange(p TextPosition) TextRange { return TextRange{Start: p, End: p} }

func TestViews_ShareTheTextButNotTheSelection(t *testing.T) {
	var ctl Controller
	var left, right View
	ctl.SetText("one\ntwo\nthree")
	left.SetSelection(caretRange(pos(0, 3)))
	right.SetSelection(caretRange(pos(2, 5)))
	h := newSplit(t, &ctl, &left, &right, Editor{})

	h.focus(&right)
	h.tree.DispatchText("!")
	h.frame()
	if got := ctl.Text(); got != "one\ntwo\nthree!" {
		t.Fatalf("text %q", got)
	}
	if got := right.Selection(); got != caretRange(pos(2, 6)) {
		t.Errorf("right selection %v", got)
	}
	if got := left.Selection(); got != caretRange(pos(0, 3)) {
		t.Errorf("left selection %v, want it left alone", got)
	}
	if ctl.Selection() != right.Selection() {
		t.Errorf("the controller selects %v, want the right view, which the user typed in", ctl.Selection())
	}

	// An edit in the left view moves the right selection with its text.
	h.focus(&left)
	h.key(input.KeyEnter, 0)
	if got := ctl.Text(); got != "one\n\ntwo\nthree!" {
		t.Fatalf("text %q", got)
	}
	if got := right.Selection(); got != caretRange(pos(3, 6)) {
		t.Errorf("right selection %v after a line came above it", got)
	}
	if got := left.Selection(); got != caretRange(pos(1, 0)) {
		t.Errorf("left selection %v", got)
	}

	// The controller acts on the left view now; the history is shared.
	ctl.SetSelection(caretRange(pos(0, 1)))
	if got := left.Selection(); got != caretRange(pos(0, 1)) {
		t.Errorf("SetSelection went elsewhere: left %v", got)
	}
	ctl.Undo()
	if got := ctl.Text(); got != "one\ntwo\nthree!" {
		t.Errorf("undo left %q", got)
	}
	if got := right.Selection(); got != caretRange(pos(2, 6)) {
		t.Errorf("right selection %v after undo", got)
	}

	// A selection that an edit swallows ends after the new text.
	right.SetSelection(TextRange{Start: pos(1, 1), End: pos(1, 2)})
	h.frame()
	ctl.Replace(TextRange{Start: pos(1, 0), End: pos(1, 3)}, "TWO!")
	if got := right.Selection(); got != caretRange(pos(1, 4)) {
		t.Errorf("right selection %v inside a replaced range", got)
	}
}

func TestViews_EachViewKeepsItsAccessibleSelection(t *testing.T) {
	var ctl Controller
	var left, right View
	ctl.SetText("abc\ndef")
	left.SetSelection(caretRange(pos(0, 1)))
	right.SetSelection(caretRange(pos(1, 2)))
	h := newSplit(t, &ctl, &left, &right, Editor{})
	root, _ := h.tree.SemanticsTree()
	want := map[string]int{"left": 1, "right": 6}
	found := 0
	root.Walk(func(n *semantics.Node) bool {
		if n.Role == semantics.RoleTextField && n.Text != nil {
			found++
			if n.Text.SelectionEnd != want[n.Label] {
				t.Errorf("%s: caret at %d, want %d", n.Label, n.Text.SelectionEnd, want[n.Label])
			}
		}
		return true
	})
	if found != 2 {
		t.Fatalf("%d text fields", found)
	}
	// A screen reader that selects in the right editor selects there.
	var id semantics.ID
	root.Walk(func(n *semantics.Node) bool {
		if n.Label == "right" {
			id = n.ID
		}
		return id == 0
	})
	h.tree.PerformSemanticsAction(id, semantics.ActionSetSelection, core.SemanticsArg{SelectionStart: 0, SelectionEnd: 3})
	if got := right.Selection(); got != (TextRange{Start: pos(0, 0), End: pos(0, 3)}) {
		t.Errorf("right selection %v", got)
	}
	if got := left.Selection(); got != caretRange(pos(0, 1)) {
		t.Errorf("left selection %v", got)
	}
}

func TestViews_AnEditAboveKeepsTheOtherViewOnItsText(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		var ctl Controller
		var left, right View
		ctl.SetText(numbered(300))
		right.SetScroll(ScrollPosition{Line: 100, Offset: 3})
		h := newSplit(t, &ctl, &left, &right, Editor{Wrap: wrap})
		if got := right.Scroll(); got.Line != 100 || got.Offset != 3 {
			t.Fatalf("wrap %v: right scrolled to %+v", wrap, got)
		}
		top := right.r.top()
		h.focus(&left)
		h.tree.DispatchText("x\ny\nz\n")
		h.frame()
		got := right.Scroll()
		if got.Line != 103 || got.Offset != 3 || ctl.Line(got.Line) != "line 100" {
			t.Errorf("wrap %v: right scrolled to %+v (%q)", wrap, got, ctl.Line(got.Line))
		}
		if d := right.r.top() - top; d != 3*float64(right.r.lineHeight) {
			t.Errorf("wrap %v: the content moved by %g, want 3 lines", wrap, d)
		}
		if l := left.Scroll(); l.Line != 0 {
			t.Errorf("wrap %v: left moved to %+v", wrap, l)
		}
		// Lines that go away above the view take it back.
		ctl.Replace(TextRange{Start: pos(0, 0), End: pos(3, 0)}, "")
		h.frame()
		if got := right.Scroll(); got.Line != 100 || got.Offset != 3 {
			t.Errorf("wrap %v: right at %+v after the lines went", wrap, got)
		}
	}
}

func TestView_ScrollRoundTrips(t *testing.T) {
	long := strings.Repeat("a long line of words that wraps ", 6)
	for _, wrap := range []bool{false, true} {
		var ctl Controller
		var v View
		lines := make([]string, 400)
		for i := range lines {
			lines[i] = long
		}
		ctl.SetText(strings.Join(lines, "\n"))
		told := 0
		v.OnScroll = func(ScrollPosition) { told++ }
		h := newEditor(t, &ctl, Editor{View: &v, Wrap: wrap})
		want := ScrollPosition{Line: 120, Offset: 5}
		if !wrap {
			want.X = 40
		}
		v.SetScroll(want)
		h.frame()
		if got := v.Scroll(); got != want {
			t.Errorf("wrap %v: scrolled to %+v, want %+v", wrap, got, want)
		}
		if r := h.view(); r.first != 120 {
			t.Errorf("wrap %v: the first line in view is %d", wrap, r.first)
		}
		h.tree.Tick(time.Second)
		h.frame()
		if told != 0 {
			t.Errorf("wrap %v: OnScroll ran %d times for SetScroll", wrap, told)
		}
		// The very top is {0, 0}.
		v.SetScroll(ScrollPosition{})
		h.frame()
		if got := v.Scroll(); got != (ScrollPosition{}) || h.view().top() != 0 {
			t.Errorf("wrap %v: top at %+v, offset %g", wrap, got, h.view().top())
		}
		// Past the end clamps.
		v.SetScroll(ScrollPosition{Line: 10_000})
		h.frame()
		r := h.view()
		if got := r.top(); got != r.state.Metrics.Content-r.state.Metrics.View {
			t.Errorf("wrap %v: offset %g past the end", wrap, got)
		}
	}
}

func TestView_OnScrollFollowsTheUserAndTheCaret(t *testing.T) {
	var ctl Controller
	var v View
	ctl.SetText(numbered(500))
	var told []ScrollPosition
	v.OnScroll = func(p ScrollPosition) { told = append(told, p) }
	h := newEditor(t, &ctl, Editor{View: &v})
	// A touchpad moves the view at once and tells it at once.
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: geom.Offset{X: 100, Y: 100}, Scroll: geom.Offset{Y: -2.5}})
	if len(told) != 1 || told[0] != v.Scroll() || told[0].Line == 0 {
		t.Fatalf("told %+v, scrolled to %+v", told, v.Scroll())
	}
	h.frame()
	if len(told) != 1 {
		t.Errorf("a layout told again: %+v", told)
	}
	// Following the caret is told on the next frame.
	told = told[:0]
	ctl.SetSelection(caretRange(pos(300, 0)))
	h.frame()
	if len(told) != 0 {
		t.Errorf("told during the layout: %+v", told)
	}
	h.tree.Tick(time.Second)
	h.frame()
	if len(told) != 1 || told[0] != v.Scroll() || v.Scroll().Line < 280 {
		t.Errorf("told %+v, scrolled to %+v", told, v.Scroll())
	}
	// So is a resize that changes the first line: at the end of the
	// document, a taller view shows lines above.
	ctl.SetSelection(caretRange(pos(499, 0)))
	h.frame()
	h.tree.Tick(2 * time.Second)
	h.frame()
	told = told[:0]
	before := v.Scroll()
	h.size.H = 600
	h.frame()
	h.tree.Tick(3 * time.Second)
	h.frame()
	if len(told) != 1 || told[0] != v.Scroll() || v.Scroll().Line >= before.Line {
		t.Errorf("after a resize told %+v, scrolled from %+v to %+v", told, before, v.Scroll())
	}
	// A resize that keeps the first line tells nothing.
	told = told[:0]
	h.size.H = 500
	h.frame()
	h.tree.Tick(4 * time.Second)
	h.frame()
	v.SetScroll(ScrollPosition{Line: 100})
	h.frame()
	told = told[:0]
	h.size.H = 400
	h.frame()
	h.tree.Tick(5 * time.Second)
	h.frame()
	if len(told) != 0 {
		t.Errorf("a resize below the first line told %+v", told)
	}
}

func TestViews_SyncedScrollingDoesNotLoop(t *testing.T) {
	var a, b Controller
	var left, right View
	a.SetText(numbered(500))
	b.SetText(numbered(400))
	leftTold, rightTold := 0, 0
	left.OnScroll = func(p ScrollPosition) { leftTold++; right.SetScroll(p) }
	right.OnScroll = func(p ScrollPosition) { rightTold++; left.SetScroll(p) }
	row := basic.Row{Cross: layout.CrossStretch, Children: []core.Widget{
		basic.Expanded{Child: Editor{Controller: &a, View: &left}},
		basic.Expanded{Child: Editor{Controller: &b, View: &right}},
	}}
	h := &splitHarness{t: t, host: &wtest.FakeHost{}, size: geom.Size{W: 800, H: 300}, tree: wtest.TextTree(t, row)}
	h.tree.SetHost(h.host)
	h.frame()

	h.wheel(false, geom.Offset{Y: -3}, 0)
	h.settle()
	if left.Scroll().Line == 0 || right.Scroll() != left.Scroll() {
		t.Errorf("left at %+v, right at %+v", left.Scroll(), right.Scroll())
	}
	if rightTold != 0 || leftTold == 0 {
		t.Errorf("left told %d times, right %d", leftTold, rightTold)
	}
	// The other way, past the end of the shorter document: the right view
	// stops at its end and the left one follows to the same line, without
	// either telling the other again.
	leftTold, rightTold = 0, 0
	h.wheel(true, geom.Offset{Y: -2000}, 0)
	h.settle()
	if rightTold == 0 || leftTold != 0 {
		t.Errorf("left told %d times, right %d", leftTold, rightTold)
	}
	if left.Scroll() != right.Scroll() {
		t.Errorf("left at %+v, right at %+v", left.Scroll(), right.Scroll())
	}
}

func TestView_RestoresItsPlaceAcrossTabs(t *testing.T) {
	var docs [2]Controller
	var views [2]View
	docs[0].SetText(numbered(500))
	docs[1].SetText("short")
	pick := 0
	var host *tabHostState
	tree := wtest.TextTree(t, tabHost{pick: &pick, docs: &docs, views: &views, self: &host})
	size := geom.Size{W: 400, H: 300}
	testutil.RenderTree(tree, size)
	views[0].SetScroll(ScrollPosition{Line: 250, Offset: 2})
	views[0].SetSelection(caretRange(pos(260, 3)))
	testutil.RenderTree(tree, size)
	host.SetState(func() { pick = 1 })
	testutil.RenderTree(tree, size)
	if got := views[1].Scroll(); got != (ScrollPosition{}) {
		t.Errorf("the second tab starts at %+v", got)
	}
	host.SetState(func() { pick = 0 })
	testutil.RenderTree(tree, size)
	if got := views[0].Scroll(); got != (ScrollPosition{Line: 250, Offset: 2}) {
		t.Errorf("the first tab came back at %+v", got)
	}
	if got := views[0].Selection(); got != caretRange(pos(260, 3)) {
		t.Errorf("the first tab came back with %v selected", got)
	}
	if views[0].r == nil || views[0].r.first != 250 {
		t.Error("the editor does not show line 250 first")
	}
}

type tabHost struct {
	core.StatefulBase
	pick  *int
	docs  *[2]Controller
	views *[2]View
	self  **tabHostState
}

type tabHostState struct{ core.StateBase }

func (tabHost) CreateState() core.State { return &tabHostState{} }

func (s *tabHostState) Build(core.BuildContext) core.Widget {
	w := s.Widget().(tabHost)
	*w.self = s
	return Editor{Controller: &w.docs[*w.pick], View: &w.views[*w.pick]}
}

func TestView_ShownTwicePanics(t *testing.T) {
	var ctl Controller
	var v View
	defer func() {
		if r := recover(); r == nil || !strings.HasPrefix(r.(string), "gowaku: ") {
			t.Errorf("recovered %v", r)
		}
	}()
	row := basic.Row{Cross: layout.CrossStretch, Children: []core.Widget{
		basic.Expanded{Child: Editor{Controller: &ctl, View: &v}},
		basic.Expanded{Child: Editor{Controller: &ctl, View: &v}},
	}}
	testutil.RenderTree(wtest.TextTree(t, row), geom.Size{W: 800, H: 300})
}

func TestViews_FindShowsInBothAndSelectsInItsOwn(t *testing.T) {
	var ctl Controller
	var left, right View
	ctl.SetText("alpha beta\nbeta gamma")
	h := newSplit(t, &ctl, &left, &right, Editor{})
	h.focus(&right)
	h.key(input.KeyF, input.ModControl)
	h.tree.DispatchText("beta")
	h.frame()
	h.key(input.KeyF3, 0)
	if got := right.Selection().Ordered(); got != (TextRange{Start: pos(1, 0), End: pos(1, 4)}) && got != (TextRange{Start: pos(0, 6), End: pos(0, 10)}) {
		t.Errorf("right selection %v, want a match", got)
	}
	if !left.Selection().Empty() {
		t.Errorf("left selection %v, want it left alone", left.Selection())
	}
	// Both editors paint the matches.
	var cv paint.Canvas
	for _, v := range []*View{&left, &right} {
		cv.Reset()
		v.r.Paint(&cv, geom.Offset{})
		fills := 0
		for _, op := range cv.DisplayList().Ops {
			if op.Kind == paint.OpRRect && op.Color == v.r.look.match {
				fills++
			}
		}
		if fills != 2 {
			t.Errorf("%s editor paints %d matches, want 2", v.r.editor.Label, fills)
		}
	}
}

func TestEditor_ScrollsSidewaysWithTheWheelAndTheBar(t *testing.T) {
	var ctl Controller
	var v View
	ctl.SetText(strings.Repeat("wide ", 300) + "\nshort")
	h := newEditor(t, &ctl, Editor{View: &v})
	r := h.view()
	m := r.state.horz.Metrics
	if m.Content <= m.View || m.View <= 0 {
		t.Fatalf("sideways metrics %+v", m)
	}
	at := geom.Offset{X: 200, Y: 100}
	// Shift turns a notch of the wheel sideways; it glides.
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: at, Scroll: geom.Offset{Y: -1}, Modifiers: input.ModShift})
	for i := range 60 {
		h.tree.Tick(time.Duration(i) * 16 * time.Millisecond)
		h.frame()
	}
	if x := r.hscroll(); x <= 0 || v.Scroll().X != x || v.Scroll().Line != 0 {
		t.Errorf("shift and the wheel scrolled to %g, view at %+v", x, v.Scroll())
	}
	// A touchpad moves at once.
	before := r.hscroll()
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: at, Scroll: geom.Offset{X: 0.5}})
	if r.hscroll() <= before {
		t.Errorf("the touchpad moved from %g to %g", before, r.hscroll())
	}
	// The bar drags the text to its end.
	h.frame()
	r.state.horz.Jump(m.Content - m.View)
	h.frame()
	if got := v.Scroll().X; got != float32(m.Content-m.View) {
		t.Errorf("the bar scrolled to %g, want %g", got, m.Content-m.View)
	}
	// The bar shows while the text moves, along the bottom.
	frame := h.frame()
	if !strings.Contains(frame, "radii=") {
		t.Error("no scroll bar drawn")
	}
}

func TestEditor_WrapHasNothingToScrollSideways(t *testing.T) {
	var ctl Controller
	var v View
	ctl.SetText(strings.Repeat("wide ", 300))
	h := newEditor(t, &ctl, Editor{View: &v, Wrap: true})
	r := h.view()
	if m := r.state.horz.Metrics; m.Content != m.View {
		t.Errorf("sideways metrics %+v with Wrap", m)
	}
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: geom.Offset{X: 100, Y: 100}, Scroll: geom.Offset{X: 0.5}})
	h.frame()
	v.SetScroll(ScrollPosition{X: 100})
	h.frame()
	if r.hscroll() != 0 || v.Scroll().X != 0 {
		t.Errorf("scrolled sideways by %g with Wrap", r.hscroll())
	}
}

func TestDecoration_ItalicSlantsWithoutMovingTheCaret(t *testing.T) {
	const line = "hello world"
	shape := func(italic bool) *lineEntry {
		var ctl Controller
		ctl.SetText(line)
		h := newEditor(t, &ctl, Editor{Decorations: func(_ int, _ string, dst []Decoration) []Decoration {
			return append(dst, Decoration{Start: 0, End: 5, Italic: italic})
		}})
		return h.view().cache.get(0)
	}
	plain, slanted := shape(false), shape(true)
	for col := 0; col <= len(line); col++ {
		if a, b := plain.layout.CaretRect(col), slanted.layout.CaretRect(col); a != b {
			t.Errorf("caret at %d: %v upright, %v italic", col, a, b)
		}
	}
	pr, sr := plain.layout.Runs(), slanted.layout.Runs()
	if len(sr) < 2 || sr[0].Face == pr[0].Face {
		t.Errorf("the italic run uses face %v, the upright one %v", sr[0].Face, pr[0].Face)
	}
	if sr[len(sr)-1].Face != pr[len(pr)-1].Face {
		t.Error("the text after the decoration is italic too")
	}
}

// The views of one document paint without allocating, scrolled down and
// sideways, with one following the other.
func TestViews_ScrollAndPaintAllocateNothing(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		var ctl Controller
		var left, right View
		lines := make([]string, 20_000)
		for i := range lines {
			lines[i] = "line " + strings.Repeat("of some width ", 1+i%7)
		}
		ctl.SetText(strings.Join(lines, "\n"))
		left.OnScroll = func(p ScrollPosition) { right.SetScroll(p) }
		h := newSplit(t, &ctl, &left, &right, Editor{LineNumbers: true, Wrap: wrap, HighlightCurrentLine: true})
		if _, err := ctl.Find("width", FindOptions{}); err != nil {
			t.Fatal(err)
		}
		r := left.r
		r.state.Jump(5_000 * float64(r.lineHeight))
		if !wrap {
			r.state.horz.Jump(60)
		}
		h.frame()
		base := r.top()
		c := geom.Tight(h.size)
		var cv paint.Canvas
		step := 0
		run := func() {
			r.state.Jump(base + float64(step%8)*7)
			step++
			h.tree.Layout(c)
			cv.Reset()
			h.tree.Paint(&cv)
		}
		for range 8 {
			run()
		}
		if allocs := testing.AllocsPerRun(20, run); allocs != 0 {
			t.Errorf("wrap %v: %v allocations per scroll and paint", wrap, allocs)
		}
		if right.Scroll().Line != left.Scroll().Line {
			t.Errorf("wrap %v: right at %+v, left at %+v", wrap, right.Scroll(), left.Scroll())
		}
	}
}
