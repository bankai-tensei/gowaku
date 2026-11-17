package control

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// tabsHarness keeps the selection of Tabs and rebuilds them.
type tabsHarness struct {
	tree     *core.Tree
	selected int
	size     geom.Size
}

func newTabsHarness(t *testing.T, reduce bool, pages ...TabPage) *tabsHarness {
	t.Helper()
	h := &tabsHarness{size: geom.Size{W: 400, H: 300}}
	var swap func(core.Widget)
	var build func() core.Widget
	build = func() core.Widget {
		tabs := Tabs{Pages: pages, Selected: h.selected, Label: "設定", OnChange: func(i int) {
			h.selected = i
			swap(build())
		}}
		return basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1, ReduceMotion: reduce}, Child: tabs}
	}
	h.tree, swap = wtest.SwapTree(t, build())
	testutil.RenderTree(h.tree, h.size)
	return h
}

func (h *tabsHarness) Key(k input.KeyCode, mods input.Modifiers) {
	h.tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: mods})
	testutil.RenderTree(h.tree, h.size)
}

// plate returns the box of the plate, the first box that has the color.
func (h *tabsHarness) plate(t *testing.T) geom.Rect {
	t.Helper()
	fill := theme.Default().Light.Color.Fill
	for _, op := range wtest.Ops(h.tree, h.size) {
		if op.Kind == paint.OpRRect && op.Color == fill && op.RRect.Rect.H == theme.ControlHeight {
			return op.RRect.Rect
		}
	}
	t.Fatal("no plate")
	return geom.Rect{}
}

func threePages() []TabPage {
	return []TabPage{
		{Label: "General", Child: basic.Text{Content: "page one"}},
		{Label: "Appearance", Child: basic.Text{Content: "page two"}},
		{Label: "Advanced", Child: basic.Text{Content: "page three"}},
	}
}

// tabNodes returns the tab list and its tabs.
func tabNodes(t *testing.T, tree *core.Tree) (*semantics.Node, []*semantics.Node) {
	t.Helper()
	root, _ := tree.SemanticsTree()
	list := wtest.FindRole(root, semantics.RoleTabList)
	if list == nil {
		t.Fatal("no tab list")
	}
	return list, list.Children
}

func TestTabs_ClickSelectsAndShowsThePage(t *testing.T) {
	h := newTabsHarness(t, false, threePages()...)
	_, tabs := tabNodes(t, h.tree)
	if len(tabs) != 3 {
		t.Fatalf("got %d tabs", len(tabs))
	}
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: wtest.Center(tabs[2].Rect)})
	if h.selected != 2 {
		t.Fatalf("a press must select at once, got %d", h.selected)
	}
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: wtest.Center(tabs[2].Rect)})
	now := time.Second
	wtest.Settled(t, h.tree, h.size, &now)
	root, _ := h.tree.SemanticsTree()
	page := wtest.FindRole(root, semantics.RoleGroup)
	if page == nil || page.Label != "Advanced" {
		t.Fatalf("the page must be named after its tab, got %+v", page)
	}
	wtest.ByLabel(t, root, "page three")
	var old bool
	root.Walk(func(n *semantics.Node) bool {
		old = old || n.Label == "page one"
		return true
	})
	if old {
		t.Error("the old page must be gone")
	}
}

func TestTabs_PlateSlidesToTheTab(t *testing.T) {
	h := newTabsHarness(t, false, threePages()...)
	_, tabs := tabNodes(t, h.tree)
	start := h.plate(t)
	if start.X != tabs[0].Rect.X || start.W != tabs[0].Rect.W {
		t.Fatalf("got %+v, want the box of the first tab %+v", start, tabs[0].Rect)
	}
	wtest.Tap(h.tree, wtest.Center(tabs[2].Rect))
	testutil.RenderTree(h.tree, h.size)
	now := time.Second
	wtest.Advance(h.tree, h.size, &now, 40*time.Millisecond)
	mid := h.plate(t)
	wtest.Settled(t, h.tree, h.size, &now)
	end := h.plate(t)
	if !(start.X < mid.X && mid.X != end.X) {
		t.Errorf("got %v, %v, %v", start.X, mid.X, end.X)
	}
	if end.X != tabs[2].Rect.X || end.W != tabs[2].Rect.W {
		t.Errorf("got %+v, want %+v", end, tabs[2].Rect)
	}
}

func TestTabs_ReduceMotionJumps(t *testing.T) {
	h := newTabsHarness(t, true, threePages()...)
	_, tabs := tabNodes(t, h.tree)
	wtest.Tap(h.tree, wtest.Center(tabs[1].Rect))
	testutil.RenderTree(h.tree, h.size)
	if got := h.plate(t); got.X != tabs[1].Rect.X {
		t.Errorf("the plate must jump, got %+v", got)
	}
}

func TestTabs_ArrowsHomeAndEnd(t *testing.T) {
	h := newTabsHarness(t, false, threePages()...)
	h.Key(input.KeyTab, 0)
	_, focus := h.tree.SemanticsTree()
	if _, tabs := tabNodes(t, h.tree); focus != tabs[0].ID {
		t.Fatal("the reader must be on the selected tab")
	}
	cases := []struct {
		name string
		key  input.KeyCode
		want int
	}{
		{"右", input.KeyRight, 1},
		{"End", input.KeyEnd, 2},
		{"右は端で先頭へ回る", input.KeyRight, 0},
		{"左は端で末尾へ回る", input.KeyLeft, 2},
		{"Home", input.KeyHome, 0},
	}
	for _, tc := range cases {
		h.Key(tc.key, 0)
		if h.selected != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, h.selected, tc.want)
		}
	}
	h.Key(input.KeyTab, 0)
	if h.selected != 0 {
		t.Error("Tab must leave the strip, not change the tab")
	}
}

func TestTabs_CtrlTabFromThePage(t *testing.T) {
	pages := threePages()
	pages[0].Child = basic.Align{Child: Button{Label: "inside", OnPress: func() {}}}
	h := newTabsHarness(t, false, pages...)
	h.Key(input.KeyTab, 0)
	h.Key(input.KeyTab, 0)
	button := h.tree.Focused()
	root, focus := h.tree.SemanticsTree()
	if n := root.Find(focus); n == nil || n.Label != "inside" {
		t.Fatalf("the button must have the focus, got %+v", n)
	}
	h.Key(input.KeyTab, input.ModControl)
	if h.selected != 1 {
		t.Fatalf("Ctrl+Tab must go to the next tab, got %d", h.selected)
	}
	_, focus = h.tree.SemanticsTree()
	if _, tabs := tabNodes(t, h.tree); focus != tabs[1].ID || h.tree.Focused() == button {
		t.Error("the keyboard must come to the strip")
	}
	cases := []struct {
		name string
		key  input.KeyCode
		mods input.Modifiers
		want int
	}{
		{"Ctrl+Shift+Tab", input.KeyTab, input.ModControl | input.ModShift, 0},
		{"Ctrl+Shift+Tab は回る", input.KeyTab, input.ModControl | input.ModShift, 2},
		{"Ctrl+PageDown", input.KeyPageDown, input.ModControl, 0},
		{"Ctrl+PageUp", input.KeyPageUp, input.ModControl, 2},
	}
	for _, tc := range cases {
		h.Key(tc.key, tc.mods)
		if h.selected != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, h.selected, tc.want)
		}
	}
}

func TestTabs_Semantics(t *testing.T) {
	h := newTabsHarness(t, false, threePages()...)
	list, tabs := tabNodes(t, h.tree)
	if list.Label != "設定" || list.State&semantics.Focusable == 0 {
		t.Errorf("got %+v", list)
	}
	for i, n := range tabs {
		selected := n.State&semantics.Selected != 0
		if n.Role != semantics.RoleTab || selected != (i == 0) || n.Actions&semantics.ActionTap == 0 || len(n.Children) != 0 {
			t.Errorf("tab %d: got %+v", i, n)
		}
	}
	if !h.tree.PerformSemanticsAction(tabs[1].ID, semantics.ActionTap, core.SemanticsArg{}) || h.selected != 1 {
		t.Errorf("a reader must select, got %d", h.selected)
	}
}

func TestTabs_DisabledWithoutOnChange(t *testing.T) {
	tree := wtest.A11yTree(t, Tabs{Pages: threePages(), Selected: 1})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	if tree.Focused() != nil {
		t.Error("disabled tabs must not take the focus")
	}
	root, _ := tree.SemanticsTree()
	list := wtest.FindRole(root, semantics.RoleTabList)
	for _, n := range list.Children {
		if n.State&semantics.Disabled == 0 || n.Actions&semantics.ActionTap != 0 {
			t.Errorf("got %+v", n)
		}
	}
	wtest.ByLabel(t, root, "page two")
}

func TestTabs_ZeroValueWorks(t *testing.T) {
	tree := wtest.TextTree(t, Tabs{})
	testutil.RenderTree(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab, Modifiers: input.ModControl})
	tree = wtest.TextTree(t, Tabs{Pages: []TabPage{{Label: "only"}}, Selected: 5, OnChange: func(int) {}})
	testutil.RenderTree(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyRight})
}

func TestTabs_TabIsThirtyHigh(t *testing.T) {
	h := newTabsHarness(t, false, threePages()...)
	if got := h.plate(t).H; got != theme.ControlHeight {
		t.Errorf("got %v", got)
	}
}

// BenchmarkTabStrip_LayoutPaint lays out and paints a strip of eight tabs, with
// the plate between two of them.
func BenchmarkTabStrip_LayoutPaint(b *testing.B) {
	pages := make([]TabPage, 8)
	for i := range pages {
		pages[i] = TabPage{Label: "Tab"}
	}
	tree := wtest.TextTree(b, Tabs{Pages: pages, OnChange: func(int) {}})
	size := geom.Size{W: 800, H: 200}
	testutil.RenderTree(tree, size)
	var strip *renderTabStrip
	var find func(ro core.RenderObject)
	find = func(ro core.RenderObject) {
		if r, ok := ro.(*renderTabStrip); ok {
			strip = r
			return
		}
		for i := range ro.ChildCount() {
			find(ro.ChildAt(i))
		}
	}
	find(tree.RootRenderObject())
	strip.plate = 3.4
	c := geom.Constraints{MaxW: 800, MaxH: geom.Inf}
	var cv paint.Canvas
	b.ReportAllocs()
	for b.Loop() {
		cv.Reset()
		strip.Layout(c)
		strip.Paint(&cv, geom.Offset{})
	}
}
