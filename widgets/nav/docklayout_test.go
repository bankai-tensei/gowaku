package nav

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/semantics"
)

// dockProbe is the content of a panel in the tests: it counts how often its
// state is made, and has the keyboard focus.
type dockProbe struct {
	core.StatefulBase
	id string
	h  *dockHarness
}

type dockProbeState struct{ core.StateOf[dockProbe] }

// CreateState implements core.StatefulWidget.
func (dockProbe) CreateState() core.State { return &dockProbeState{} }

// InitState implements core.State.
func (s *dockProbeState) InitState() {
	w := s.Widget()
	w.h.inits[w.id]++
	w.h.states[w.id] = s
}

// Build implements core.State.
func (s *dockProbeState) Build(core.BuildContext) core.Widget {
	w := s.Widget()
	return interact.Focus{
		Enabled: true,
		OnFocusChange: func(f bool) {
			if f {
				w.h.focused = w.id
			}
		},
		Child: basic.Container{Color: wtest.Blue},
	}
}

// dockHarness shows a DockLayout of 1000×700 whose panels are probes. Every
// panel but files is closable.
type dockHarness struct {
	tree    *core.Tree
	swap    func(core.Widget)
	ctrl    *DockController
	def     DockArrangement
	ids     []string
	size    geom.Size
	inits   map[string]int
	states  map[string]*dockProbeState
	focused string
	// time is the clock of the pointer, in milliseconds.
	time    uint32
	changes []DockArrangement
	closed  []string
	// autoHide and motion set those of the layout; motion lets it move.
	autoHide, motion bool
	// italic are the panels whose titles are italic.
	italic map[string]bool
	// ask is OnCloseRequest.
	ask func(id string) bool
}

func newDockHarness(t testing.TB, def DockArrangement, ids ...string) *dockHarness {
	t.Helper()
	h := &dockHarness{
		ctrl: &DockController{}, def: def, ids: ids, size: geom.Size{W: 1000, H: 700},
		inits: map[string]int{}, states: map[string]*dockProbeState{},
	}
	h.tree, h.swap = wtest.SwapTree(t, h.widget())
	h.render()
	return h
}

func (h *dockHarness) widget() core.Widget {
	panels := make([]DockPanel, len(h.ids))
	for i, id := range h.ids {
		panels[i] = DockPanel{
			ID: id, Title: id, Closable: id != "files", Italic: h.italic[id],
			Build: func(core.BuildContext) core.Widget { return dockProbe{id: id, h: h} },
		}
	}
	return basic.MediaQuery{
		Data: basic.MediaQueryData{Scale: 1, ReduceMotion: !h.motion},
		Child: overlay.Overlay{Child: DockLayout{
			Panels: panels, Controller: h.ctrl, Default: h.def, Label: "パネル", AutoHide: h.autoHide,
			OnChange:       func(a DockArrangement) { h.changes = append(h.changes, a) },
			OnClose:        func(id string) { h.closed = append(h.closed, id) },
			OnCloseRequest: h.ask,
		}},
	}
}

func (h *dockHarness) setPanels(ids ...string) {
	h.ids = ids
	h.swap(h.widget())
	h.render()
}

func (h *dockHarness) render() { testutil.RenderTree(h.tree, h.size) }

func (h *dockHarness) shape() string { return shape(h.ctrl.root) }

func (h *dockHarness) pointer(kind core.PointerKind, at geom.Offset) {
	h.tree.DispatchPointer(core.PointerEvent{Kind: kind, Pos: at, Button: input.ButtonPrimary, Time: h.time})
	h.render()
}

func (h *dockHarness) key(k input.KeyCode, mods input.Modifiers) {
	h.tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: mods})
	h.render()
}

// press presses the tab of the panel id and drags it to at, without letting
// go.
func (h *dockHarness) press(t *testing.T, id string, at geom.Offset) {
	t.Helper()
	from := wtest.Center(h.tab(t, id).Rect)
	h.pointer(core.PointerMove, from)
	h.pointer(core.PointerDown, from)
	h.pointer(core.PointerMove, from.Add(geom.Offset{Y: 8}))
	h.pointer(core.PointerMove, at)
}

// dragTab drags the tab of the panel id to at and lets go.
func (h *dockHarness) dragTab(t *testing.T, id string, at geom.Offset) {
	t.Helper()
	h.press(t, id, at)
	h.pointer(core.PointerUp, at)
}

// nodes returns the semantics nodes with role.
func (h *dockHarness) nodes(role semantics.Role) []*semantics.Node {
	root, _ := h.tree.SemanticsTree()
	return wtest.RolesOf(root, role)
}

// tab returns the tab of the panel id.
func (h *dockHarness) tab(t *testing.T, id string) *semantics.Node {
	t.Helper()
	for _, n := range h.nodes(semantics.RoleTab) {
		if n.Label == id {
			return n
		}
	}
	t.Fatalf("no tab %q", id)
	return nil
}

func (h *dockHarness) dock(t testing.TB) *renderDock {
	t.Helper()
	var out *renderDock
	walkRender(h.tree.RootRenderObject(), func(ro core.RenderObject) {
		if r, ok := ro.(*renderDock); ok {
			out = r
		}
	})
	if out == nil {
		t.Fatal("no dock")
	}
	return out
}

func TestDockLayout_ZeroValueIsOneGroupAndBuildsWhatShows(t *testing.T) {
	h := newDockHarness(t, DockArrangement{}, "a", "b", "c")
	lists := h.nodes(semantics.RoleTabList)
	if len(lists) != 1 || len(lists[0].Children) != 3 || lists[0].Label != "パネル" {
		t.Fatalf("got %d lists", len(lists))
	}
	if len(h.nodes(semantics.RoleSeparator)) != 0 {
		t.Error("one group has no divider")
	}
	if !maps(h.inits, map[string]int{"a": 1}) {
		t.Errorf("only the panel shown is built, got %v", h.inits)
	}
	groups := h.nodes(semantics.RoleGroup)
	if len(groups) != 1 || groups[0].Label != "a" {
		t.Errorf("the panel shown is a group named by its title, got %d", len(groups))
	}
	wtest.Tap(h.tree, wtest.Center(h.tab(t, "b").Rect))
	h.render()
	wtest.Tap(h.tree, wtest.Center(h.tab(t, "a").Rect))
	h.render()
	if !maps(h.inits, map[string]int{"a": 1, "b": 1}) {
		t.Errorf("a panel keeps its state while another shows, got %v", h.inits)
	}
	if got := h.shape(); got != "[a* b c]" {
		t.Errorf("got %s", got)
	}
	if len(h.changes) != 2 {
		t.Errorf("selecting a tab is a change, got %d", len(h.changes))
	}
}

func maps(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestDockLayout_SinglePanel(t *testing.T) {
	h := newDockHarness(t, DockArrangement{}, "a")
	for _, at := range []geom.Offset{{X: 8, Y: 400}, {X: 900, Y: 400}, {X: 500, Y: 650}, {X: 500, Y: 400}} {
		h.press(t, "a", at)
		if h.dock(t).previewShow {
			t.Errorf("%v: a lone panel has nowhere to go", at)
		}
		h.pointer(core.PointerUp, at)
		if got := h.shape(); got != "[a*]" {
			t.Errorf("%v: got %s", at, got)
		}
	}
	if len(h.changes) != 0 {
		t.Errorf("got %d changes", len(h.changes))
	}
}

func TestDockLayout_DropZones(t *testing.T) {
	cases := []struct {
		name    string
		at      geom.Offset
		want    string
		preview geom.Rect
		share   float32
	}{
		{"群の左", geom.Offset{X: 100, Y: 400}, "H([b*] | [a c*])", geom.Rect{X: 2, Y: 2, W: 496, H: 696}, 0.5},
		{"群の右", geom.Offset{X: 900, Y: 400}, "H([a c*] | [b*])", geom.Rect{X: 502, Y: 2, W: 496, H: 696}, 0.5},
		{"群の上", geom.Offset{X: 500, Y: 120}, "V([b*] | [a c*])", geom.Rect{X: 2, Y: 2, W: 996, H: 346}, 0.5},
		{"群の下", geom.Offset{X: 500, Y: 650}, "V([a c*] | [b*])", geom.Rect{X: 2, Y: 352, W: 996, H: 346}, 0.5},
		{"全体の左", geom.Offset{X: 8, Y: 400}, "H([b*] | [a c*])", geom.Rect{X: 2, Y: 2, W: 246, H: 696}, 0.25},
		{"全体の右", geom.Offset{X: 994, Y: 400}, "H([a c*] | [b*])", geom.Rect{X: 752, Y: 2, W: 246, H: 696}, 0.75},
		{"全体の下", geom.Offset{X: 500, Y: 694}, "V([a c*] | [b*])", geom.Rect{X: 2, Y: 527, W: 996, H: 171}, 0.75},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newDockHarness(t, DockArrangement{}, "a", "b", "c")
			h.press(t, "b", tc.at)
			r := h.dock(t)
			if !r.previewShow || !wtest.NearRect(r.preview, tc.preview) {
				t.Errorf("got preview %v %v, want %v", r.previewShow, r.preview, tc.preview)
			}
			h.pointer(core.PointerUp, tc.at)
			if got := h.shape(); got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
			if r.previewShow {
				t.Error("the preview must go with the drop")
			}
			if w := h.ctrl.root.weights; len(w) != 2 || !near(w[0], tc.share) {
				t.Errorf("got weights %v", w)
			}
			if len(h.nodes(semantics.RoleTabList)) != 2 || len(h.nodes(semantics.RoleSeparator)) != 1 {
				t.Error("two groups have two strips and a divider")
			}
			if len(h.changes) != 2 {
				t.Errorf("the press selects b, and the drop moves it; got %d changes", len(h.changes))
			}
		})
	}
	t.Run("同じ群の中央は何もしない", func(t *testing.T) {
		h := newDockHarness(t, DockArrangement{}, "a", "b", "c")
		h.press(t, "b", geom.Offset{X: 500, Y: 400})
		if h.dock(t).previewShow {
			t.Error("no preview where nothing would change")
		}
		h.pointer(core.PointerUp, geom.Offset{X: 500, Y: 400})
		if got := h.shape(); got != "[a b* c]" {
			t.Errorf("got %s", got)
		}
	})
}

func TestDockLayout_DropOnAnotherGroup(t *testing.T) {
	t.Run("中身の中央", func(t *testing.T) {
		h := newDockHarness(t, twoGroups(), "files", "a", "b")
		h.press(t, "a", geom.Offset{X: 125, Y: 400})
		if r := h.dock(t); !r.previewShow || !wtest.NearRect(r.preview, geom.Rect{X: 2, Y: 2, W: 246, H: 696}) {
			t.Errorf("the preview covers the group, got %v", r.preview)
		}
		h.pointer(core.PointerUp, geom.Offset{X: 125, Y: 400})
		if got := h.shape(); got != "H([files a*] | [b*])" {
			t.Errorf("got %s", got)
		}
	})
	t.Run("帯の上", func(t *testing.T) {
		h := newDockHarness(t, twoGroups(), "files", "a", "b")
		h.press(t, "a", geom.Offset{X: 10, Y: 17})
		if h.dock(t).previewShow {
			t.Error("over a strip the strip shows where the tab goes")
		}
		h.pointer(core.PointerUp, geom.Offset{X: 10, Y: 17})
		if got := h.shape(); got != "H([a* files] | [b*])" {
			t.Errorf("got %s", got)
		}
		if len(h.changes) != 2 {
			t.Errorf("got %d changes", len(h.changes))
		}
	})
	t.Run("最後の 1 枚が出ると群が消える", func(t *testing.T) {
		h := newDockHarness(t, twoGroups(), "files", "a", "b")
		h.dragTab(t, "files", geom.Offset{X: 600, Y: 400})
		if got := h.shape(); got != "[a b files*]" {
			t.Errorf("got %s", got)
		}
		if len(h.nodes(semantics.RoleTabList)) != 1 || len(h.nodes(semantics.RoleSeparator)) != 0 {
			t.Error("the split must collapse")
		}
	})
}

func TestDockLayout_PanelKeepsStateAcrossGroups(t *testing.T) {
	h := newDockHarness(t, DockArrangement{}, "a", "b", "c")
	wtest.Tap(h.tree, wtest.Center(h.tab(t, "b").Rect))
	h.render()
	first := h.states["b"]
	h.dragTab(t, "b", geom.Offset{X: 900, Y: 400})
	if got := h.shape(); got != "H([a c*] | [b*])" {
		t.Fatalf("got %s", got)
	}
	h.ctrl.Move("b", "c", DockBottom)
	h.render()
	if got := h.shape(); got != "V([a c*] | [b*])" {
		t.Fatalf("got %s", got)
	}
	h.ctrl.Move("b", "a", DockCenter)
	h.render()
	if got := h.shape(); got != "[a c b*]" {
		t.Fatalf("got %s", got)
	}
	if h.states["b"] != first || h.inits["b"] != 1 || h.inits["a"] != 1 {
		t.Errorf("the panels must keep their state, got inits %v", h.inits)
	}
}

func TestDockLayout_Close(t *testing.T) {
	h := newDockHarness(t, twoGroups(), "files", "a", "b")
	root, _ := h.tree.SemanticsTree()
	var labels []string
	for _, b := range wtest.Buttons(root, nil) {
		labels = append(labels, b.Label)
	}
	if slices.Contains(labels, "Close files") || !slices.Contains(labels, "Close b") {
		t.Fatalf("only closable panels have a close button, got %v", labels)
	}
	h.tree.PerformSemanticsAction(wtest.ByLabel(t, root, "Close b").ID, semantics.ActionTap, core.SemanticsArg{})
	h.render()
	if got := h.shape(); got != "H([files*] | [a*])" {
		t.Errorf("got %s", got)
	}
	if !slices.Equal(h.closed, []string{"b"}) || len(h.changes) != 1 || !slices.Equal(h.changes[0].Closed, []string{"b"}) {
		t.Errorf("got closed %v, changes %d", h.closed, len(h.changes))
	}
	// Ctrl+W closes the panel of the current group, but not one that is not
	// closable.
	h.ctrl.current = h.ctrl.groupOf("files")
	h.key(input.KeyW, input.ModControl)
	if got := h.shape(); got != "H([files*] | [a*])" {
		t.Errorf("files is not closable, got %s", got)
	}
	h.ctrl.Activate("b")
	h.render()
	if got := h.shape(); got != "H([files b*] | [a*])" {
		t.Errorf("got %s", got)
	}
	if h.inits["b"] != 2 {
		t.Errorf("a closed panel starts afresh, got %v", h.inits)
	}
}

// divider returns the center of the first divider.
func (h *dockHarness) divider(t *testing.T) geom.Offset {
	t.Helper()
	seps := h.nodes(semantics.RoleSeparator)
	if len(seps) == 0 {
		t.Fatal("no divider")
	}
	return wtest.Center(seps[0].Rect)
}

func TestDockLayout_ResizeDivider(t *testing.T) {
	h := newDockHarness(t, twoGroups(), "files", "a", "b")
	files := h.ctrl.groupOf("files")
	if files.rect.W != 250 {
		t.Fatalf("got %v", files.rect.W)
	}
	at := h.divider(t)
	h.pointer(core.PointerMove, at)
	h.pointer(core.PointerDown, at)
	h.pointer(core.PointerMove, at.Add(geom.Offset{X: 100}))
	if geom.Abs32(files.rect.W-350) > 1 {
		t.Errorf("the divider follows the pointer, got %v", files.rect.W)
	}
	if len(h.changes) != 0 {
		t.Error("OnChange waits for the end of the drag")
	}
	h.pointer(core.PointerMove, at.Add(geom.Offset{X: -1000}))
	if files.rect.W != dockMinGroupWidth {
		t.Errorf("a group keeps its smallest width, got %v", files.rect.W)
	}
	h.pointer(core.PointerMove, at.Add(geom.Offset{X: 2000}))
	if e := h.ctrl.groupOf("a").rect.W; e != dockMinGroupWidth {
		t.Errorf("so does the other, got %v", e)
	}
	h.pointer(core.PointerUp, at.Add(geom.Offset{X: 2000}))
	if len(h.changes) != 1 {
		t.Errorf("got %d changes", len(h.changes))
	}
	// A double click shares the room evenly.
	at = h.divider(t)
	h.pointer(core.PointerDown, at)
	h.pointer(core.PointerUp, at)
	h.pointer(core.PointerDown, at)
	h.pointer(core.PointerUp, at)
	if geom.Abs32(files.rect.W-500) > 1 {
		t.Errorf("got %v", files.rect.W)
	}
	// The press gave the divider the keyboard.
	h.key(input.KeyRight, 0)
	if geom.Abs32(files.rect.W-500-dockKeyStep) > 1 {
		t.Errorf("an arrow moves the divider, got %v", files.rect.W)
	}
	h.key(input.KeyHome, 0)
	if files.rect.W != dockMinGroupWidth {
		t.Errorf("Home goes to the limit, got %v", files.rect.W)
	}
	sep := h.nodes(semantics.RoleSeparator)[0]
	if sep.Range.Max != 100 || sep.Range.Value <= 0 || sep.Range.Value >= 15 || sep.Label != "Pane divider" {
		t.Errorf("got %+v %q", sep.Range, sep.Label)
	}
	h.tree.PerformSemanticsAction(sep.ID, semantics.ActionSetValue, core.SemanticsArg{Number: 50})
	h.render()
	if geom.Abs32(files.rect.W-500) > 1 {
		t.Errorf("a screen reader sets the share, got %v", files.rect.W)
	}
}

func TestDockLayout_EscapeCancelsADrag(t *testing.T) {
	h := newDockHarness(t, DockArrangement{}, "a", "b", "c")
	at := geom.Offset{X: 900, Y: 400}
	h.press(t, "b", at)
	if !h.dock(t).previewShow {
		t.Fatal("no preview")
	}
	h.key(input.KeyEscape, 0)
	if h.dock(t).previewShow {
		t.Error("Escape must take the preview away")
	}
	h.pointer(core.PointerUp, at)
	if got := h.shape(); got != "[a b* c]" {
		t.Errorf("got %s", got)
	}
}

func TestDockLayout_PanelsChangeDuringADrag(t *testing.T) {
	t.Run("引いているパネルが消える", func(t *testing.T) {
		h := newDockHarness(t, DockArrangement{}, "a", "b", "c")
		at := geom.Offset{X: 900, Y: 400}
		h.press(t, "b", at)
		h.setPanels("a", "c")
		h.pointer(core.PointerMove, at.Add(geom.Offset{Y: 10}))
		h.pointer(core.PointerUp, at.Add(geom.Offset{Y: 10}))
		if got := h.shape(); got != "[a* c]" {
			t.Errorf("got %s", got)
		}
	})
	t.Run("群が消える", func(t *testing.T) {
		h := newDockHarness(t, twoGroups(), "files", "a", "b")
		at := geom.Offset{X: 600, Y: 400}
		h.press(t, "files", at)
		h.setPanels("a", "b")
		if h.dock(t).previewShow {
			t.Error("the preview must go with the strip it came from")
		}
		h.pointer(core.PointerUp, at)
		if got := h.shape(); got != "[a b*]" {
			t.Errorf("got %s", got)
		}
	})
	t.Run("パネルが増える", func(t *testing.T) {
		h := newDockHarness(t, DockArrangement{}, "a", "b", "c")
		at := geom.Offset{X: 900, Y: 400}
		h.press(t, "b", at)
		h.setPanels("a", "b", "c", "d")
		h.pointer(core.PointerMove, at.Add(geom.Offset{Y: 10}))
		h.pointer(core.PointerUp, at.Add(geom.Offset{Y: 10}))
		if got := h.shape(); got != "H([a c* d] | [b*])" {
			t.Errorf("got %s", got)
		}
	})
}

func TestDockLayout_Keyboard(t *testing.T) {
	h := newDockHarness(t, twoGroups(), "files", "a", "b")
	const ctrlAlt = input.ModControl | input.ModAlt
	h.key(input.KeyRight, ctrlAlt)
	if h.focused != "b" {
		t.Fatalf("Ctrl+Alt+→ moves the focus to the next group, got %q", h.focused)
	}
	h.key(input.KeyLeft, ctrlAlt)
	if h.focused != "files" {
		t.Fatalf("got %q", h.focused)
	}
	h.key(input.KeyLeft, ctrlAlt)
	if h.focused != "files" {
		t.Errorf("there is nothing further left, got %q", h.focused)
	}
	h.key(input.KeyRight, ctrlAlt)
	b := h.states["b"]
	h.key(input.KeyLeft, ctrlAlt|input.ModShift)
	if got := h.shape(); got != "H([files b*] | [a*])" {
		t.Fatalf("Ctrl+Alt+Shift+← moves the panel to the group on the left, got %s", got)
	}
	if h.states["b"] != b || h.focused != "b" || h.tree.Focused() == nil {
		t.Error("the panel keeps its state and the focus")
	}
	h.key(input.KeyUp, ctrlAlt|input.ModShift)
	if got := h.shape(); got != "H(V([b*] | [files*]) | [a*])" {
		t.Fatalf("with no group above, the panel docks on top of its group, got %s", got)
	}
	h.key(input.KeyEnter, ctrlAlt)
	if h.ctrl.maximized != h.ctrl.groupOf("b") || len(h.nodes(semantics.RoleTabList)) != 1 {
		t.Error("Ctrl+Alt+Enter maximizes the current group")
	}
	h.key(input.KeyEnter, ctrlAlt)
	if h.ctrl.maximized != nil || len(h.nodes(semantics.RoleTabList)) != 3 {
		t.Error("and brings the others back")
	}
	// The keys of a TabBar work for the group with the focus.
	h.key(input.KeyRight, ctrlAlt)
	if h.focused != "a" {
		t.Fatalf("got %q", h.focused)
	}
	h.ctrl.Move("files", "a", DockCenter)
	h.render()
	h.key(input.KeyTab, input.ModControl)
	if g := h.ctrl.groupOf("a"); g.active != "a" {
		t.Errorf("Ctrl+Tab selects the next tab of the current group, got %s", shape(g))
	}
	if len(h.changes) != 5 {
		t.Errorf("got %d changes", len(h.changes))
	}
}

func TestDockLayout_MaximizeByDoubleClick(t *testing.T) {
	h := newDockHarness(t, twoGroups(), "files", "a", "b")
	strip := geom.Offset{X: 950, Y: 17}
	double := func() {
		h.time += 2000
		h.pointer(core.PointerDown, strip)
		h.pointer(core.PointerUp, strip)
		h.pointer(core.PointerDown, strip)
		h.pointer(core.PointerUp, strip)
	}
	double()
	if h.ctrl.maximized != h.ctrl.groupOf("a") {
		t.Fatal("a double click on the strip maximizes its group")
	}
	if len(h.nodes(semantics.RoleTabList)) != 1 || len(h.nodes(semantics.RoleSeparator)) != 0 {
		t.Error("the other groups hide")
	}
	if r := h.ctrl.groupOf("a").rect; r != (geom.Rect{W: 1000, H: 700}) {
		t.Errorf("got %v", r)
	}
	if a := h.ctrl.Arrangement(); a.Maximized != "b" {
		t.Errorf("got %q", a.Maximized)
	}
	double()
	if h.ctrl.maximized != nil || len(h.nodes(semantics.RoleTabList)) != 2 {
		t.Error("another brings them back")
	}
}

func TestDockLayout_ControllerAndRestore(t *testing.T) {
	h := newDockHarness(t, twoGroups(), "files", "a", "b")
	saved := h.ctrl.Arrangement()
	h.ctrl.Move("files", "", DockBottom)
	h.render()
	if got := h.shape(); got != "V([a b*] | [files*])" {
		t.Fatalf("got %s", got)
	}
	h.ctrl.SetArrangement(saved)
	h.render()
	if got := h.shape(); got != "H([files*] | [a b*])" {
		t.Errorf("got %s", got)
	}
	h.ctrl.Reset()
	h.render()
	if got := h.shape(); got != "H([files*] | [a b*])" {
		t.Errorf("got %s", got)
	}
	if len(h.changes) != 0 {
		t.Error("the controller's changes are not the user's")
	}
	if len(h.nodes(semantics.RoleSeparator)) != 1 {
		t.Error("the view must follow")
	}
	// A layout of its own starts from Default.
	tree := wtest.TextTree(t, DockLayout{Default: twoGroups(), Panels: []DockPanel{{ID: "a"}, {ID: "files"}, {ID: "b"}}})
	testutil.RenderTree(tree, geom.Size{W: 800, H: 600})
	if n := len(wtest.RolesOf(must(tree.SemanticsTree()), semantics.RoleTabList)); n != 2 {
		t.Errorf("got %d groups", n)
	}
}

func must(n *semantics.Node, _ semantics.ID) *semantics.Node { return n }

func TestDockLayout_Contracts(t *testing.T) {
	render := func(w core.Widget) func() {
		return func() {
			tree := core.NewTree(w)
			defer tree.Dispose()
			testutil.RenderTree(tree, geom.Size{W: 400, H: 300})
		}
	}
	wtest.ExpectPanic(t, "two panels with the ID \"a\"", render(DockLayout{Panels: []DockPanel{{ID: "a"}, {ID: "a"}}}))
	wtest.ExpectPanic(t, "a panel without an ID", render(DockLayout{Panels: []DockPanel{{}}}))
	wtest.ExpectPanic(t, "DockLayout has an unbounded", render(basic.Column{Children: []core.Widget{DockLayout{}}}))
	// The zero value shows an empty group.
	tree := wtest.TextTree(t, DockLayout{})
	testutil.RenderTree(tree, geom.Size{W: 400, H: 300})
	if n := len(wtest.RolesOf(must(tree.SemanticsTree()), semantics.RoleTabList)); n != 1 {
		t.Errorf("got %d strips", n)
	}
}

func TestDockLayout_ControllerMovesBetweenLayouts(t *testing.T) {
	h := newDockHarness(t, twoGroups(), "files", "a", "b")
	other := &DockController{}
	h.ctrl, other = other, h.ctrl
	h.swap(h.widget())
	h.render()
	if other.view != nil || h.ctrl.view == nil {
		t.Error("the layout must leave the old controller")
	}
	if got := h.shape(); got != "H([files*] | [a b*])" {
		t.Errorf("the new controller starts from Default, got %s", got)
	}
}
