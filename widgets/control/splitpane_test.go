package control

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

// splitRig is a SplitPane in a 401 wide window, whose room is 400, with what
// it reported.
type splitRig struct {
	tree      *core.Tree
	positions []float32
	folds     []bool
}

func newSplitRig(t *testing.T, p SplitPane) *splitRig {
	t.Helper()
	r := &splitRig{}
	p.First = basic.Text{Content: "left"}
	p.Second = Button{Label: "right", OnPress: func() {}}
	p.OnChange = func(v float32) { r.positions = append(r.positions, v) }
	p.OnCollapse = func(v bool) { r.folds = append(r.folds, v) }
	r.tree = wtest.TextTree(t, p)
	r.render()
	return r
}

var splitScreen = geom.Size{W: 401, H: 300}

func (r *splitRig) render() { wtest.Ops(r.tree, splitScreen) }

func (r *splitRig) split() *renderSplit {
	var found *renderSplit
	var walk func(ro core.RenderObject)
	walk = func(ro core.RenderObject) {
		if s, ok := ro.(*renderSplit); ok && found == nil {
			found = s
		}
		for i := range ro.ChildCount() {
			walk(ro.ChildAt(i))
		}
	}
	walk(r.tree.RootRenderObject())
	return found
}

func (r *splitRig) first() float32 { return r.split().first }

func (r *splitRig) divider() *semantics.Node {
	r.render()
	return wtest.FindRole(r.tree.Semantics(), semantics.RoleSeparator)
}

func (r *splitRig) Key(k input.KeyCode) {
	r.render()
	r.tree.DispatchKey(core.KeyEvent{Key: k})
	r.render()
}

func TestSplitPane_LaysOutThePanes(t *testing.T) {
	cases := []struct {
		name string
		p    SplitPane
		want float32
	}{
		{"ゼロ値は真ん中", SplitPane{}, 200},
		{"割合", SplitPane{Position: 0.25}, 100},
		{"ピクセル", SplitPane{Position: 120, Pixels: true}, 120},
		{"Default", SplitPane{Default: 0.75}, 300},
		{"最小の幅で止まる", SplitPane{Position: 0.1, MinFirst: 80}, 80},
		{"後ろの最小の幅で止まる", SplitPane{Position: 0.9, MinSecond: 150}, 250},
		{"畳む", SplitPane{Collapsible: SplitCollapseFirst, Collapsed: true}, 0},
		{"後ろを畳む", SplitPane{Collapsible: SplitCollapseSecond, Collapsed: true}, 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newSplitRig(t, tc.p)
			s := r.split()
			if s.first != tc.want {
				t.Errorf("first got %v, want %v", s.first, tc.want)
			}
			if got := s.ChildAt(2).Offset().X; got != tc.want+1 {
				t.Errorf("second starts at %v", got)
			}
		})
	}
}

func TestSplitPane_Vertical(t *testing.T) {
	r := newSplitRig(t, SplitPane{Axis: layout.Vertical, Position: 0.5})
	s := r.split()
	// The room is 299 high.
	if s.room != 299 || s.first != 150 || s.ChildAt(2).Offset().Y != 151 {
		t.Errorf("room %v first %v second at %v", s.room, s.first, s.ChildAt(2).Offset())
	}
	r.Key(input.KeyTab)
	r.Key(input.KeyDown)
	if s.first != 160 {
		t.Errorf("Down moves a horizontal divider: %v", s.first)
	}
	r.Key(input.KeyRight)
	if s.first != 160 {
		t.Errorf("Right leaves it: %v", s.first)
	}
}

func TestSplitPane_Semantics(t *testing.T) {
	r := newSplitRig(t, SplitPane{Position: 0.25, Collapsible: SplitCollapseFirst})
	d := r.divider()
	if d == nil {
		t.Fatal("no separator")
	}
	if d.Label != "Pane divider" || d.State&semantics.Focusable == 0 || d.State&semantics.Expanded == 0 {
		t.Errorf("got %+v", d)
	}
	if d.Range.Min != 0 || d.Range.Max != 100 || d.Range.Value != 25 {
		t.Errorf("the value is the share of the first pane: %+v", d.Range)
	}
	const want = semantics.ActionIncrease | semantics.ActionDecrease | semantics.ActionSetValue |
		semantics.ActionExpand | semantics.ActionCollapse | semantics.ActionFocus
	if d.Actions&want != want {
		t.Errorf("actions %b", d.Actions)
	}
	if d.Rect.W != kit.SplitHit || d.Rect.X != 100-(kit.SplitHit-1)/2 {
		t.Errorf("the strip is wider than the line: %+v", d.Rect)
	}
	root, _ := r.tree.SemanticsTree()
	r.tree.PerformSemanticsAction(d.ID, semantics.ActionSetValue, core.SemanticsArg{Number: 50})
	r.render()
	if r.first() != 200 {
		t.Errorf("SetValue moves it: %v", r.first())
	}
	if d := r.divider(); d.Range.Value != 50 {
		t.Errorf("the value follows: %v", d.Range.Value)
	}
	_ = root
	d = r.divider()
	r.tree.SemanticsTree()
	r.tree.PerformSemanticsAction(d.ID, semantics.ActionCollapse, core.SemanticsArg{})
	r.render()
	if d := r.divider(); d.State&semantics.Collapsed == 0 || r.first() != 0 {
		t.Errorf("Collapse folds the first pane: %+v", d)
	}
	if findText(r.tree.Semantics(), "left") {
		t.Error("a folded pane leaves the tree")
	}
}

// findText reports whether a text node says s below n.
func findText(n *semantics.Node, s string) bool {
	found := false
	n.Walk(func(m *semantics.Node) bool {
		if m.Role == semantics.RoleText && (m.Label == s || m.Value == s) {
			found = true
		}
		return !found
	})
	return found
}

func TestSplitPane_Keyboard(t *testing.T) {
	r := newSplitRig(t, SplitPane{MinFirst: 60, MinSecond: 100, Collapsible: SplitCollapseFirst})
	r.Key(input.KeyTab)
	if d := r.divider(); d.State&semantics.Focused == 0 {
		t.Fatal("Tab stops at the divider")
	}
	steps := []struct {
		name string
		key  input.KeyCode
		want float32
	}{
		{"右", input.KeyRight, 210},
		{"左", input.KeyLeft, 200},
		{"End は最大", input.KeyEnd, 300},
		{"Home は最小", input.KeyHome, 60},
		{"Enter で畳む", input.KeyEnter, 0},
		{"Enter で戻る", input.KeyEnter, 60},
		{"左で最小の半分を下回っても畳まない", input.KeyLeft, 60},
	}
	for _, s := range steps {
		r.Key(s.key)
		if got := r.first(); got != s.want {
			t.Fatalf("%s: got %v, want %v", s.name, got, s.want)
		}
	}
	if len(r.folds) != 2 || !r.folds[0] || r.folds[1] {
		t.Errorf("folds %v", r.folds)
	}
	if got := r.positions[len(r.positions)-1]; got != 60.0/400 {
		t.Errorf("OnChange reports a fraction: %v", got)
	}
}

func TestSplitPane_DragAndDoubleClick(t *testing.T) {
	r := newSplitRig(t, SplitPane{Position: 100, Pixels: true, Default: 150, MinFirst: 40, Collapsible: SplitCollapseFirst})
	at := geom.Offset{X: 100, Y: 50}
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: at})
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at, Time: 1})
	for _, x := range []float32{130, 170} {
		r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: x, Y: 60}})
		r.render()
	}
	if r.first() != 170 {
		t.Errorf("the divider follows the pointer: %v", r.first())
	}
	// Past half the minimum, the pane folds; back above it, it comes back.
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 10, Y: 60}})
	r.render()
	if r.first() != 0 {
		t.Errorf("a drag past half the minimum folds: %v", r.first())
	}
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 30, Y: 60}})
	r.render()
	if r.first() != 40 {
		t.Errorf("and back unfolds at the minimum: %v", r.first())
	}
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: geom.Offset{X: 30, Y: 60}})
	r.render()
	if r.positions[len(r.positions)-1] != 40 {
		t.Errorf("OnChange reports pixels: %v", r.positions)
	}

	at = geom.Offset{X: 40, Y: 50}
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at, Time: 1000})
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: at, Time: 1010})
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at, Time: 1100})
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: at, Time: 1110})
	r.render()
	if r.first() != 150 {
		t.Errorf("a double click goes back to Default: %v", r.first())
	}
}

func TestSplitPane_CursorAndHover(t *testing.T) {
	r := newSplitRig(t, SplitPane{})
	var h *kit.RenderSplitHandle
	s := r.split()
	var walk func(ro core.RenderObject)
	walk = func(ro core.RenderObject) {
		if x, ok := ro.(*kit.RenderSplitHandle); ok {
			h = x
		}
		for i := range ro.ChildCount() {
			walk(ro.ChildAt(i))
		}
	}
	walk(s)
	if h == nil || h.MouseCursor() != input.CursorColResize {
		t.Fatal("the strip shows the resize cursor")
	}
	before := h.Line()
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 202, Y: 40}})
	var now time.Duration
	wtest.Settled(t, r.tree, splitScreen, &now)
	if h.Line() == before {
		t.Error("the line darkens under the pointer")
	}
}

func TestSplitPane_CallerReplacesThePosition(t *testing.T) {
	tree, swap := wtest.SwapTree(t, SplitPane{Position: 0.25})
	wtest.Ops(tree, splitScreen)
	swap(SplitPane{Position: 0.5})
	wtest.Ops(tree, splitScreen)
	r := &splitRig{tree: tree}
	if r.first() != 200 {
		t.Errorf("a new Position wins: %v", r.first())
	}
}

func TestSplitPane_UnboundedPanics(t *testing.T) {
	wtest.ExpectPanic(t, "gowaku: SplitPane has an unbounded width", func() {
		testutil.RenderTree(core.NewTree(basic.Row{Children: []core.Widget{SplitPane{}}}), splitScreen)
	})
}

func benchSplit(b *testing.B) *core.Tree {
	b.Helper()
	tree := wtest.TextTree(b, SplitPane{
		// Boxes rather than text, whose relayout at a new width is the text
		// package's to measure.
		First:  SplitPane{Axis: layout.Vertical, First: basic.Container{Color: geom.Hex(0xdddddd)}, Second: basic.Container{Color: geom.Hex(0xcccccc)}},
		Second: basic.Container{Color: geom.Hex(0xeeeeee)},
	})
	testutil.RenderTree(tree, wtest.Page)
	return tree
}

func BenchmarkLayout_SplitPane(b *testing.B) {
	tree := benchSplit(b)
	c1, c2 := geom.Tight(wtest.Page), geom.Tight(geom.Size{W: wtest.Page.W + 40, H: wtest.Page.H})
	b.ReportAllocs()
	flip := false
	for b.Loop() {
		flip = !flip
		if flip {
			tree.Layout(c2)
		} else {
			tree.Layout(c1)
		}
	}
}

func BenchmarkPaint_SplitPane(b *testing.B) {
	tree := benchSplit(b)
	var cv paint.Canvas
	cv.Reset()
	tree.Paint(&cv)
	b.ReportAllocs()
	for b.Loop() {
		cv.Reset()
		tree.Paint(&cv)
	}
}
