package nav

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// tabModel is the list a TabBar shows, which its callbacks change.
type tabModel struct {
	tabs     []TabItem
	selected int
	torn     []TabTearOff
	group    *TabGroup
	noTear   bool
	changed  func()
	// hold keeps the tabs that OnClose asks to close, as an app asking
	// whether to save does, and held lists them.
	hold bool
	held []int
}

func newTabModel(titles ...string) *tabModel {
	m := &tabModel{}
	for i, t := range titles {
		m.tabs = append(m.tabs, TabItem{Key: i + 1, Label: t})
	}
	return m
}

func (m *tabModel) titles() []string {
	out := make([]string, len(m.tabs))
	for i := range m.tabs {
		out[i] = m.tabs[i].Label
	}
	return out
}

// bar returns the TabBar of m. byKey finds the tabs other bars hand over.
func (m *tabModel) bar(byKey func(core.Key) TabItem) TabBar {
	b := TabBar{
		Tabs: slices.Clone(m.tabs), Selected: m.selected, Label: "文書", Group: m.group,
		OnSelect: func(i int) { m.selected = i; m.changed() },
		OnClose: func(i int) {
			if m.hold {
				m.held = append(m.held, i)
				return
			}
			m.tabs = slices.Delete(m.tabs, i, i+1)
			m.selected = min(m.selected, len(m.tabs)-1)
			m.changed()
		},
		OnMove: func(from, to int) {
			t := m.tabs[from]
			m.tabs = slices.Insert(slices.Delete(m.tabs, from, from+1), to, t)
			m.changed()
		},
		OnNew: func() {
			m.tabs = append(m.tabs, TabItem{Key: 100 + len(m.tabs), Label: "new"})
			m.selected = len(m.tabs) - 1
			m.changed()
		},
		OnReceive: func(k core.Key, i int) {
			m.tabs = slices.Insert(m.tabs, i, byKey(k))
			m.changed()
		},
		OnDetach: func(i int) {
			m.tabs = slices.Delete(m.tabs, i, i+1)
			m.selected = max(min(m.selected, len(m.tabs)-1), 0)
			m.changed()
		},
	}
	if !m.noTear {
		b.OnTearOff = func(e TabTearOff) { m.torn = append(m.torn, e) }
	}
	return b
}

// tabBarHarness shows one or two bars in one window.
type tabBarHarness struct {
	tree *core.Tree
	a, b *tabModel
	size geom.Size
	now  time.Duration
}

func newTabBarHarness(t testing.TB, width float32, reduce bool, a, b *tabModel) *tabBarHarness {
	t.Helper()
	h := &tabBarHarness{a: a, b: b, size: geom.Size{W: width, H: 300}, now: time.Second}
	byKey := func(k core.Key) TabItem {
		for _, m := range []*tabModel{a, b} {
			if m == nil {
				continue
			}
			for _, it := range m.tabs {
				if it.Key == k {
					return it
				}
			}
		}
		return TabItem{Key: k, Label: fmt.Sprint(k)}
	}
	var swap func(core.Widget)
	build := func() core.Widget {
		children := []core.Widget{a.bar(byKey)}
		if b != nil {
			children = append(children, basic.SizedBox{Height: 100}, b.bar(byKey))
		}
		return basic.MediaQuery{
			Data:  basic.MediaQueryData{Scale: 1, ReduceMotion: reduce},
			Child: overlay.Overlay{Child: basic.Column{Cross: layout.CrossStretch, Children: children}},
		}
	}
	a.changed = func() { swap(build()) }
	if b != nil {
		b.changed = a.changed
	}
	h.tree, swap = wtest.SwapTree(t, build())
	testutil.RenderTree(h.tree, h.size)
	return h
}

func (h *tabBarHarness) render() { testutil.RenderTree(h.tree, h.size) }

func (h *tabBarHarness) settle(t testing.TB) { wtest.Settled(t, h.tree, h.size, &h.now) }

func (h *tabBarHarness) Key(k input.KeyCode, mods input.Modifiers) {
	h.tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: mods})
	h.render()
}

func (h *tabBarHarness) pointer(kind core.PointerKind, at geom.Offset, b input.Button) {
	h.tree.DispatchPointer(core.PointerEvent{Kind: kind, Pos: at, Button: b})
	h.render()
}

// drag presses at from, moves through the points and lets go at the last.
func (h *tabBarHarness) drag(from geom.Offset, through ...geom.Offset) {
	h.pointer(core.PointerMove, from, 0)
	h.pointer(core.PointerDown, from, 0)
	for _, p := range through {
		h.pointer(core.PointerMove, p, 0)
	}
	h.pointer(core.PointerUp, through[len(through)-1], 0)
}

// lists returns the tab lists of the window, top first.
func (h *tabBarHarness) lists(t *testing.T) []*semantics.Node {
	t.Helper()
	root, _ := h.tree.SemanticsTree()
	var out []*semantics.Node
	root.Walk(func(n *semantics.Node) bool {
		if n.Role == semantics.RoleTabList {
			out = append(out, n)
		}
		return true
	})
	if len(out) == 0 {
		t.Fatal("no tab list")
	}
	return out
}

// tabs returns the tabs of list i.
func (h *tabBarHarness) tabs(t *testing.T, i int) []*semantics.Node {
	t.Helper()
	return h.lists(t)[i].Children
}

func TestTabBar_Semantics(t *testing.T) {
	m := newTabModel("README.md", "main.go", "go.mod")
	m.selected = 1
	h := newTabBarHarness(t, 800, false, m, nil)
	list := h.lists(t)[0]
	if list.Label != "文書" {
		t.Errorf("got label %q", list.Label)
	}
	tabs := list.Children
	if len(tabs) != 3 {
		t.Fatalf("got %d tabs", len(tabs))
	}
	for i, n := range tabs {
		if n.Role != semantics.RoleTab || n.Label != m.tabs[i].Label {
			t.Errorf("tab %d: got %v %q", i, n.Role, n.Label)
		}
		if got := n.State&semantics.Selected != 0; got != (i == 1) {
			t.Errorf("tab %d: selected %v", i, got)
		}
		if n.Actions&semantics.ActionTap == 0 {
			t.Errorf("tab %d must take a tap", i)
		}
	}
	root, _ := h.tree.SemanticsTree()
	closeMain := wtest.ByLabel(t, root, "Close main.go")
	if closeMain.Role != semantics.RoleButton {
		t.Errorf("got %v", closeMain.Role)
	}
	wtest.ByLabel(t, root, "New tab")
	h.tree.PerformSemanticsAction(tabs[2].ID, semantics.ActionTap, core.SemanticsArg{})
	if m.selected != 2 {
		t.Errorf("a tap must select, got %d", m.selected)
	}
	h.tree.PerformSemanticsAction(closeMain.ID, semantics.ActionTap, core.SemanticsArg{})
	if got := m.titles(); !slices.Equal(got, []string{"README.md", "go.mod"}) {
		t.Errorf("got %v", got)
	}
}

func TestTabBar_JapaneseCloseLabel(t *testing.T) {
	ja := basic.JapaneseStrings()
	if got := ja.CloseNamed("メモ"); got != "メモを閉じる" {
		t.Errorf("got %q", got)
	}
}

func TestTabBar_ClickSelectsAndMiddleClickCloses(t *testing.T) {
	m := newTabModel("a", "b", "c")
	h := newTabBarHarness(t, 800, false, m, nil)
	tabs := h.tabs(t, 0)
	h.pointer(core.PointerDown, wtest.Center(tabs[2].Rect), input.ButtonPrimary)
	if m.selected != 2 {
		t.Fatalf("a press must select at once, got %d", m.selected)
	}
	h.pointer(core.PointerUp, wtest.Center(tabs[2].Rect), input.ButtonPrimary)
	h.pointer(core.PointerDown, wtest.Center(tabs[0].Rect), input.ButtonMiddle)
	if len(m.tabs) != 3 {
		t.Fatal("the middle button closes on release")
	}
	h.pointer(core.PointerUp, wtest.Center(tabs[0].Rect), input.ButtonMiddle)
	if got := m.titles(); !slices.Equal(got, []string{"b", "c"}) {
		t.Errorf("got %v", got)
	}
	if m.selected != 1 {
		t.Errorf("a middle click must not select, got %d", m.selected)
	}
}

func TestTabBar_CloseButtonShowsOnSelectedAndHovered(t *testing.T) {
	m := newTabModel("a", "b", "c")
	h := newTabBarHarness(t, 800, false, m, nil)
	tabs := h.tabs(t, 0)
	closeOf := func(i int) geom.Rect {
		return tabs[i].Children[0].Rect
	}
	// The × of an unselected tab takes no press until the pointer is over it.
	tree := h.tree
	if hit := tree.HitTest(wtest.Center(closeOf(1))); len(hit.Path) == 0 {
		t.Fatal("nothing hit")
	}
	h.pointer(core.PointerMove, wtest.Center(closeOf(1)), 0)
	h.settle(t)
	h.pointer(core.PointerDown, wtest.Center(closeOf(1)), input.ButtonPrimary)
	h.pointer(core.PointerUp, wtest.Center(closeOf(1)), input.ButtonPrimary)
	if got := m.titles(); !slices.Equal(got, []string{"a", "c"}) {
		t.Errorf("the × under the pointer must close, got %v", got)
	}
	if m.selected != 0 {
		t.Errorf("closing must not select, got %d", m.selected)
	}
}

func TestTabBar_HiddenCloseTakesNoPress(t *testing.T) {
	m := newTabModel("a", "b", "c")
	h := newTabBarHarness(t, 800, false, m, nil)
	tabs := h.tabs(t, 0)
	at := wtest.Center(tabs[2].Children[0].Rect)
	// A touch arrives without a hover first.
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at})
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: at})
	h.render()
	if len(m.tabs) != 3 || m.selected != 2 {
		t.Errorf("the hidden × must not close; got %v selected %d", m.titles(), m.selected)
	}
}

func TestTabBar_Keyboard(t *testing.T) {
	m := newTabModel("a", "b", "c", "d")
	h := newTabBarHarness(t, 800, false, m, nil)
	h.Key(input.KeyTab, 0)
	_, focus := h.tree.SemanticsTree()
	if tabs := h.tabs(t, 0); focus != tabs[0].ID {
		t.Fatal("the reader must be on the selected tab")
	}
	cases := []struct {
		name  string
		key   input.KeyCode
		mods  input.Modifiers
		want  int
		order string
	}{
		{"右", input.KeyRight, 0, 1, "abcd"},
		{"End", input.KeyEnd, 0, 3, "abcd"},
		{"右は回る", input.KeyRight, 0, 0, "abcd"},
		{"左は回る", input.KeyLeft, 0, 3, "abcd"},
		{"Home", input.KeyHome, 0, 0, "abcd"},
		{"Ctrl+Shift+右で動かす", input.KeyRight, input.ModControl | input.ModShift, 1, "bacd"},
		{"Ctrl+Shift+右をもう一度", input.KeyRight, input.ModControl | input.ModShift, 2, "bcad"},
		{"Ctrl+Shift+左", input.KeyLeft, input.ModControl | input.ModShift, 1, "bacd"},
		{"Ctrl+Tab", input.KeyTab, input.ModControl, 2, "bacd"},
		{"Ctrl+Shift+Tab", input.KeyTab, input.ModControl | input.ModShift, 1, "bacd"},
		{"Ctrl+PageDown", input.KeyPageDown, input.ModControl, 2, "bacd"},
		{"Ctrl+PageUp", input.KeyPageUp, input.ModControl, 1, "bacd"},
		{"Ctrl+Shift+PageDown で動かす", input.KeyPageDown, input.ModControl | input.ModShift, 2, "bcad"},
		{"Ctrl+Shift+PageUp で戻す", input.KeyPageUp, input.ModControl | input.ModShift, 1, "bacd"},
		{"Ctrl+W で閉じる", input.KeyW, input.ModControl, 1, "bcd"},
		{"Ctrl+F4 で閉じる", input.KeyF4, input.ModControl, 1, "bd"},
	}
	for _, tc := range cases {
		h.Key(tc.key, tc.mods)
		order := ""
		for _, s := range m.titles() {
			order += s
		}
		if m.selected != tc.want || order != tc.order {
			t.Errorf("%s: got %d %s, want %d %s", tc.name, m.selected, order, tc.want, tc.order)
		}
	}
}

func TestTabBar_ShortcutsWorkFromElsewhere(t *testing.T) {
	m := newTabModel("a", "b")
	h := newTabBarHarness(t, 800, false, m, nil)
	if h.tree.Focused() != nil {
		t.Fatal("nothing must have the focus")
	}
	h.Key(input.KeyTab, input.ModControl)
	if m.selected != 1 {
		t.Errorf("Ctrl+Tab must work without the focus, got %d", m.selected)
	}
}

func TestTabBar_DragReordersWithSlidingNeighbors(t *testing.T) {
	m := newTabModel("a", "b", "c", "d")
	h := newTabBarHarness(t, 800, false, m, nil)
	tabs := h.tabs(t, 0)
	tw := tabs[1].Rect.X - tabs[0].Rect.X
	if tw != 192 {
		t.Fatalf("four tabs share 800 less the new tab button, got %v", tw)
	}
	from := wtest.Center(tabs[0].Rect)
	h.pointer(core.PointerMove, from, 0)
	h.pointer(core.PointerDown, from, 0)
	// Past the middle of the third tab.
	to := from.Add(geom.Offset{X: tw*2 + 10})
	h.pointer(core.PointerMove, from.Add(geom.Offset{X: 2}), 0)
	if s := h.tree.Focused(); s == nil {
		t.Fatal("the press must focus the strip")
	}
	h.pointer(core.PointerMove, to, 0)
	run := findRun(t, h.tree)
	if run.dragged != 0 || run.children[0].Offset().X != tw*2+10 {
		t.Fatalf("the tab must follow the pointer, got %d at %v", run.dragged, run.children[0].Offset().X)
	}
	wtest.Advance(h.tree, h.size, &h.now, 40*time.Millisecond)
	if x := run.children[1].Offset().X; x >= tw || x <= 0 {
		t.Errorf("b must be sliding to the left, got %v", x)
	}
	h.settle(t)
	if x := run.children[2].Offset().X; x != tw {
		t.Errorf("c must make room, got %v", x)
	}
	if x := run.children[3].Offset().X; x != tw*3 {
		t.Errorf("d must stay, got %v", x)
	}
	h.pointer(core.PointerUp, to, 0)
	if got := m.titles(); !slices.Equal(got, []string{"b", "c", "a", "d"}) {
		t.Errorf("got %v", got)
	}
	if m.selected != 2 {
		t.Errorf("the selection must follow the tab, got %d", m.selected)
	}
	h.settle(t)
	if x := run.children[2].Offset().X; x != tw*2 {
		t.Errorf("a must settle in its slot, got %v", x)
	}
}

func TestTabBar_ReduceMotionJumps(t *testing.T) {
	m := newTabModel("a", "b", "c")
	h := newTabBarHarness(t, 800, true, m, nil)
	tabs := h.tabs(t, 0)
	from := wtest.Center(tabs[0].Rect)
	h.pointer(core.PointerMove, from, 0)
	h.pointer(core.PointerDown, from, 0)
	h.pointer(core.PointerMove, from.Add(geom.Offset{X: tabMaxWidth + 20}), 0)
	run := findRun(t, h.tree)
	if x := run.children[1].Offset().X; x != 0 {
		t.Errorf("b must jump, got %v", x)
	}
}

func TestTabBar_SmallMoveIsATap(t *testing.T) {
	m := newTabModel("a", "b")
	h := newTabBarHarness(t, 800, false, m, nil)
	tabs := h.tabs(t, 0)
	p := wtest.Center(tabs[1].Rect)
	h.drag(p, p.Add(geom.Offset{X: 2, Y: 1}))
	if !slices.Equal(m.titles(), []string{"a", "b"}) || m.selected != 1 {
		t.Errorf("got %v %d", m.titles(), m.selected)
	}
}

func TestTabBar_EscapeCancelsADrag(t *testing.T) {
	m := newTabModel("a", "b", "c")
	h := newTabBarHarness(t, 800, false, m, nil)
	tabs := h.tabs(t, 0)
	p := wtest.Center(tabs[0].Rect)
	h.pointer(core.PointerMove, p, 0)
	h.pointer(core.PointerDown, p, 0)
	h.pointer(core.PointerMove, p.Add(geom.Offset{X: tabMaxWidth * 2}), 0)
	h.Key(input.KeyEscape, 0)
	h.pointer(core.PointerUp, p.Add(geom.Offset{X: tabMaxWidth * 2}), 0)
	if !slices.Equal(m.titles(), []string{"a", "b", "c"}) {
		t.Errorf("got %v", m.titles())
	}
}

func TestTabBar_OverflowScrollsAndListsAllTabs(t *testing.T) {
	titles := make([]string, 12)
	for i := range titles {
		titles[i] = fmt.Sprintf("file%02d.go", i)
	}
	m := newTabModel(titles...)
	h := newTabBarHarness(t, 600, false, m, nil)
	run := findRun(t, h.tree)
	if run.tabW != tabMinWidth {
		t.Fatalf("tabs must shrink to the minimum, got %v", run.tabW)
	}
	root, _ := h.tree.SemanticsTree()
	more := wtest.ByLabel(t, root, "All tabs")
	if more.Rect.X+more.Rect.W > 600 || run.Size().W+more.Rect.W > 600 {
		t.Errorf("the strip and the button must fit, got %v and %+v", run.Size().W, more.Rect)
	}
	h.pointer(core.PointerScroll, geom.Offset{X: 100, Y: 20}, 0)
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: geom.Offset{X: 100, Y: 20}, Scroll: geom.Offset{Y: -2}})
	h.render()
	if run.scroll != 2*tabScrollStep {
		t.Errorf("the wheel must scroll, got %v", run.scroll)
	}
	// Selecting the last tab brings it into view.
	h.Key(input.KeyTab, input.ModControl|input.ModShift)
	if m.selected != 11 || run.scroll != run.content-run.Size().W {
		t.Errorf("got %d scrolled %v of %v", m.selected, run.scroll, run.content-run.Size().W)
	}
	h.tree.PerformSemanticsAction(more.ID, semantics.ActionTap, core.SemanticsArg{})
	h.render()
	root, _ = h.tree.SemanticsTree()
	item := wtest.ByLabel(t, root, "file03.go")
	h.tree.PerformSemanticsAction(item.ID, semantics.ActionTap, core.SemanticsArg{})
	h.render()
	if m.selected != 3 {
		t.Errorf("the menu must select, got %d", m.selected)
	}
	if run.scroll > 3*tabMinWidth {
		t.Errorf("the tab must come into view, got %v", run.scroll)
	}
}

func TestTabBar_NoOverflowHidesTheListButton(t *testing.T) {
	m := newTabModel("a", "b")
	h := newTabBarHarness(t, 800, false, m, nil)
	root, _ := h.tree.SemanticsTree()
	var found bool
	root.Walk(func(n *semantics.Node) bool {
		found = found || n.Label == "All tabs"
		return true
	})
	if found {
		t.Error("the list button must hide while everything fits")
	}
}

func TestTabBar_TitleIsCutWithAnEllipsis(t *testing.T) {
	long := "とても長いファイルの名前で、タブに入りきらないもの.txt"
	m := newTabModel(long, "b", "c", "d", "e", "f", "g", "h")
	h := newTabBarHarness(t, 800, false, m, nil)
	var title *renderTabTitle
	walkRender(findRun(t, h.tree).children[0], func(ro core.RenderObject) {
		if r, ok := ro.(*renderTabTitle); ok {
			title = r
		}
	})
	if title == nil {
		t.Fatal("no title")
	}
	if title.shown == title.full {
		t.Fatal("the title must be cut")
	}
	if title.shown.Size().W > title.Size().W {
		t.Errorf("the cut title must fit, got %v in %v", title.shown.Size().W, title.Size().W)
	}
	if got := title.shown.Runs(); len(got) == 0 {
		t.Error("the cut title must show")
	}
}

func TestTabBar_TearOffOutsideEveryBar(t *testing.T) {
	m := newTabModel("a", "b", "c")
	h := newTabBarHarness(t, 800, false, m, nil)
	tabs := h.tabs(t, 0)
	p := wtest.Center(tabs[1].Rect)
	h.pointer(core.PointerMove, p, 0)
	h.pointer(core.PointerDown, p, 0)
	h.pointer(core.PointerMove, p.Add(geom.Offset{X: 3, Y: 30}), 0)
	run := findRun(t, h.tree)
	if run.hidden != -1 {
		t.Fatal("near the strip the tab stays on it")
	}
	away := p.Add(geom.Offset{X: 40, Y: 150})
	h.pointer(core.PointerMove, away, 0)
	if run.hidden != 1 {
		t.Fatalf("far from the strip the tab comes off, got %d", run.hidden)
	}
	h.settle(t)
	if x := run.children[2].Offset().X; x != tabMaxWidth {
		t.Errorf("c must close the gap, got %v", x)
	}
	h.pointer(core.PointerUp, away, 0)
	if len(m.torn) != 1 {
		t.Fatalf("got %d tear-offs", len(m.torn))
	}
	e := m.torn[0]
	if e.Index != 1 || e.Key != 2 || e.Alone || e.Placed {
		t.Errorf("got %+v", e)
	}
}

func TestTabBar_TearOffComesBackToTheStrip(t *testing.T) {
	m := newTabModel("a", "b", "c")
	h := newTabBarHarness(t, 800, false, m, nil)
	tabs := h.tabs(t, 0)
	p := wtest.Center(tabs[0].Rect)
	h.drag(p, p.Add(geom.Offset{Y: 150}), p.Add(geom.Offset{X: tabMaxWidth * 2, Y: 5}))
	if len(m.torn) != 0 {
		t.Fatal("back on the strip it is a move")
	}
	if got := m.titles(); !slices.Equal(got, []string{"b", "c", "a"}) {
		t.Errorf("got %v", got)
	}
}

func TestTabBar_NoTearOffWithoutCallback(t *testing.T) {
	m := newTabModel("a", "b")
	m.noTear = true
	h := newTabBarHarness(t, 800, false, m, nil)
	tabs := h.tabs(t, 0)
	p := wtest.Center(tabs[0].Rect)
	h.pointer(core.PointerMove, p, 0)
	h.pointer(core.PointerDown, p, 0)
	h.pointer(core.PointerMove, p.Add(geom.Offset{Y: 200}), 0)
	if run := findRun(t, h.tree); run.hidden != -1 {
		t.Error("without OnTearOff the tab stays on the strip")
	}
	h.pointer(core.PointerUp, p.Add(geom.Offset{Y: 200}), 0)
}

func TestTabBar_DropOnAnotherBarOfTheGroup(t *testing.T) {
	g := &TabGroup{}
	a, b := newTabModel("a1", "a2", "a3"), newTabModel("b1", "b2")
	for i := range b.tabs {
		b.tabs[i].Key = 10 + i
	}
	a.group, b.group = g, g
	h := newTabBarHarness(t, 800, false, a, b)
	aTabs, bTabs := h.tabs(t, 0), h.tabs(t, 1)
	p := wtest.Center(aTabs[1].Rect)
	// Over the gap between b1 and b2.
	over := geom.Offset{X: bTabs[1].Rect.X + 10, Y: wtest.Center(bTabs[1].Rect).Y}
	h.pointer(core.PointerMove, p, 0)
	h.pointer(core.PointerDown, p, 0)
	h.pointer(core.PointerMove, p.Add(geom.Offset{Y: 60}), 0)
	h.pointer(core.PointerMove, over, 0)
	if got := len(h.tabs(t, 1)); got != 2 {
		t.Errorf("the ghost must not be a tab to a reader, got %d", got)
	}
	var bRun *renderTabRun
	walkRender(h.tree.RootRenderObject(), func(ro core.RenderObject) {
		if r, ok := ro.(*renderTabRun); ok {
			bRun = r
		}
	})
	if len(bRun.children) != 3 {
		t.Fatalf("the other bar must show where the tab goes, got %d children", len(bRun.children))
	}
	h.settle(t)
	if x := bRun.children[1].Offset().X; x != 2*tabMaxWidth {
		t.Errorf("b2 must make room, got %v", x)
	}
	h.pointer(core.PointerUp, over, 0)
	if got := a.titles(); !slices.Equal(got, []string{"a1", "a3"}) {
		t.Errorf("source: got %v", got)
	}
	if got := b.titles(); !slices.Equal(got, []string{"b1", "a2", "b2"}) {
		t.Errorf("target: got %v", got)
	}
	if b.selected != 1 {
		t.Errorf("the dropped tab must be selected, got %d", b.selected)
	}
	if len(a.torn) != 0 {
		t.Error("a drop on a bar is no tear-off")
	}
	h.render()
	if len(bRun.children) != 3 || len(h.tabs(t, 1)) != 3 {
		t.Error("the ghost must go when the tab arrives")
	}
}

func TestTabBar_LeavingTheOtherBarTakesTheGhostAway(t *testing.T) {
	g := &TabGroup{}
	a, b := newTabModel("a1", "a2"), newTabModel("b1")
	b.tabs[0].Key = 10
	a.group, b.group = g, g
	h := newTabBarHarness(t, 800, false, a, b)
	aTabs, bTabs := h.tabs(t, 0), h.tabs(t, 1)
	p := wtest.Center(aTabs[0].Rect)
	h.pointer(core.PointerMove, p, 0)
	h.pointer(core.PointerDown, p, 0)
	h.pointer(core.PointerMove, wtest.Center(bTabs[0].Rect), 0)
	h.pointer(core.PointerMove, geom.Offset{X: 400, Y: 250}, 0)
	var bRun *renderTabRun
	walkRender(h.tree.RootRenderObject(), func(ro core.RenderObject) {
		if r, ok := ro.(*renderTabRun); ok {
			bRun = r
		}
	})
	if len(bRun.children) != 1 {
		t.Errorf("got %d children", len(bRun.children))
	}
	h.pointer(core.PointerUp, geom.Offset{X: 400, Y: 250}, 0)
	if len(a.torn) != 1 || len(b.tabs) != 1 {
		t.Errorf("got %d tear-offs, %d tabs", len(a.torn), len(b.tabs))
	}
}

// fakeScreen is the host of a window at a place of the screen.
type fakeScreen struct {
	testHost
	origin geom.Offset
	scale  float32
	size   geom.Size
	active *int
}

func (f fakeScreen) ScreenOrigin() (geom.Offset, float32, bool) { return f.origin, f.scale, true }
func (f fakeScreen) ShowsScreenPoint(p geom.Offset) bool {
	return geom.Rect{X: f.origin.X, Y: f.origin.Y, W: f.size.W * f.scale, H: f.size.H * f.scale}.Contains(p)
}
func (f fakeScreen) Activate() { *f.active++ }

// testHost is a Host that does nothing.
type testHost struct{}

func (testHost) SetIMEEnabled(bool)         {}
func (testHost) SetIMECursorRect(geom.Rect) {}
func (testHost) CancelComposition()         {}
func (testHost) SetCursor(input.Cursor)     {}
func (testHost) ClipboardText() string      { return "" }
func (testHost) SetClipboardText(string)    {}

func TestTabBar_DropInAnotherWindow(t *testing.T) {
	g := &TabGroup{}
	a, b := newTabModel("a1", "a2"), newTabModel("b1")
	b.tabs[0].Key = 10
	a.group, b.group = g, g
	byKey := func(k core.Key) TabItem { return TabItem{Key: k, Label: "moved"} }
	size := geom.Size{W: 600, H: 200}
	var activations int
	window := func(m *tabModel, origin geom.Offset) *core.Tree {
		var swap func(core.Widget)
		build := func() core.Widget {
			return basic.MediaQuery{Data: basic.MediaQueryData{Scale: 2}, Child: basic.Column{Cross: layout.CrossStretch, Children: []core.Widget{m.bar(byKey)}}}
		}
		m.changed = func() { swap(build()) }
		var tree *core.Tree
		tree, swap = wtest.SwapTree(t, build())
		tree.SetHost(fakeScreen{origin: origin, scale: 2, size: size, active: &activations})
		testutil.RenderTree(tree, size)
		return tree
	}
	ta := window(a, geom.Offset{X: 100, Y: 100})
	tb := window(b, geom.Offset{X: 1500, Y: 700})
	render := func() {
		testutil.RenderTree(ta, size)
		testutil.RenderTree(tb, size)
	}
	send := func(kind core.PointerKind, at geom.Offset) {
		ta.DispatchPointer(core.PointerEvent{Kind: kind, Pos: at})
		render()
	}
	p := geom.Offset{X: tabMaxWidth*1.5 + 0, Y: 20}
	send(core.PointerMove, p)
	send(core.PointerDown, p)
	send(core.PointerMove, p.Add(geom.Offset{Y: 100}))
	// The second window shows its bar at 1500,700 on the screen, at scale 2:
	// a point 300 logical pixels right of the first window's origin plus...
	target := geom.Offset{X: (1500-100)/2 + tabMaxWidth + 20, Y: (700-100)/2 + 20}
	send(core.PointerMove, target)
	send(core.PointerUp, target)
	if got := a.titles(); !slices.Equal(got, []string{"a1"}) {
		t.Errorf("source: got %v", got)
	}
	if got := b.titles(); !slices.Equal(got, []string{"b1", "moved"}) {
		t.Errorf("target: got %v", got)
	}
	if activations != 1 {
		t.Errorf("the window that takes the tab must come to the front, got %d", activations)
	}
	// Torn off past every window, the drop says where.
	send(core.PointerMove, geom.Offset{X: 30, Y: 20})
	send(core.PointerDown, geom.Offset{X: 30, Y: 20})
	send(core.PointerMove, geom.Offset{X: 30, Y: 400})
	send(core.PointerUp, geom.Offset{X: 30, Y: 400})
	if len(a.torn) != 1 {
		t.Fatalf("got %d", len(a.torn))
	}
	e := a.torn[0]
	if !e.Placed || !e.Alone || e.Screen != (geom.Offset{X: 160, Y: 900}) {
		t.Errorf("got %+v", e)
	}
	// The grab was 30, 20 into the tab, at the left edge of the window.
	if e.Origin != (geom.Offset{X: 100, Y: 860}) {
		t.Errorf("got origin %+v", e.Origin)
	}
}

func TestTabBar_DisposedTargetLeavesTheGroup(t *testing.T) {
	g := &TabGroup{}
	a := newTabModel("a")
	a.group = g
	a.changed = func() {}
	tree := core.NewTree(testutil.WithShaper(testutil.NewShaper(t), a.bar(nil)))
	testutil.RenderTree(tree, wtest.Screen)
	if len(g.bars) != 1 {
		t.Fatalf("got %d bars", len(g.bars))
	}
	tree.Dispose()
	if len(g.bars) != 0 {
		t.Errorf("got %d bars", len(g.bars))
	}
}

func TestTabBar_SelectedLooksRaised(t *testing.T) {
	m := newTabModel("a", "b")
	h := newTabBarHarness(t, 800, false, m, nil)
	c := theme.Default().Light.Color
	var card, line bool
	for _, op := range wtest.Ops(h.tree, h.size) {
		if op.Kind == paint.OpRRect && op.Color == c.Card && op.RRect.TopLeft == 8 && op.RRect.BottomLeft == 0 {
			card = true
		}
		if op.Kind == paint.OpRRect && op.Color == c.Hair && op.RRect.Rect.H == 1 && op.RRect.Rect.W == 800 {
			line = true
		}
	}
	if !card || !line {
		t.Errorf("card %v, line %v", card, line)
	}
}

func findRun(t testing.TB, tree *core.Tree) *renderTabRun {
	t.Helper()
	var run *renderTabRun
	walkRender(tree.RootRenderObject(), func(ro core.RenderObject) {
		if r, ok := ro.(*renderTabRun); ok && run == nil {
			run = r
		}
	})
	if run == nil {
		t.Fatal("no strip")
	}
	return run
}

func walkRender(ro core.RenderObject, fn func(core.RenderObject)) {
	if ro == nil {
		return
	}
	fn(ro)
	for i := range ro.ChildCount() {
		walkRender(ro.ChildAt(i), fn)
	}
}

func BenchmarkTabBar_LayoutPaint(b *testing.B) {
	m := newTabModel("README.md", "main.go", "go.mod", "app.go", "tabbar.go", "docs.md")
	h := newTabBarHarness(b, 900, false, m, nil)
	run := findRun(b, h.tree)
	var cv paint.Canvas
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		run.scroll = float32(i % 2)
		run.MarkNeedsLayout()
		h.tree.Layout(geom.Tight(h.size))
		cv.Reset()
		h.tree.Paint(&cv)
	}
}

func TestTabBar_OnCloseMayHoldTheTab(t *testing.T) {
	m := newTabModel("a", "b", "c")
	m.hold, m.selected = true, 1
	h := newTabBarHarness(t, 800, false, m, nil)
	tabs := h.tabs(t, 0)
	h.pointer(core.PointerDown, wtest.Center(tabs[0].Rect), input.ButtonMiddle)
	h.pointer(core.PointerUp, wtest.Center(tabs[0].Rect), input.ButtonMiddle)
	h.tree.DispatchKey(core.KeyEvent{Key: input.KeyW, Modifiers: input.ModControl})
	h.render()
	if !slices.Equal(m.held, []int{0, 1}) {
		t.Fatalf("got %v", m.held)
	}
	if got := len(h.tabs(t, 0)); got != 3 {
		t.Errorf("the bar takes nothing out itself, got %d tabs", got)
	}
	// The app agrees later.
	m.hold = false
	m.tabs = slices.Delete(m.tabs, 1, 2)
	m.changed()
	h.render()
	if got := len(h.tabs(t, 0)); got != 2 {
		t.Errorf("got %d tabs", got)
	}
}
