package nav

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/semantics"
)

// newAutoHideHarness is a dockHarness whose layout has AutoHide.
func newAutoHideHarness(t *testing.T, def DockArrangement, ids ...string) *dockHarness {
	t.Helper()
	h := newDockHarness(t, def, ids...)
	h.autoHide = true
	h.swap(h.widget())
	h.render()
	return h
}

// hiddenShape writes the hidden panels as side:id, in order.
func hiddenShape(c *DockController) []string {
	var out []string
	for _, h := range c.hidden {
		out = append(out, h.side.String()+":"+h.id)
	}
	return out
}

// tap presses and releases at at.
func (h *dockHarness) tap(at geom.Offset) {
	h.time += 1000
	h.pointer(core.PointerMove, at)
	h.pointer(core.PointerDown, at)
	h.pointer(core.PointerUp, at)
}

// onStripTab reports whether the focus is on a tab of a strip of hidden
// panels.
func onStripTab(tree *core.Tree) bool {
	var found bool
	if f := tree.Focused(); f != nil {
		walkRender(f, func(ro core.RenderObject) {
			if _, ok := ro.(*renderDockStripTab); ok {
				found = true
			}
		})
	}
	return found
}

// flyoutPin returns the pin button of the flyout, the checked one.
func (h *dockHarness) flyoutPin(t *testing.T) *semantics.Node {
	t.Helper()
	root, _ := h.tree.SemanticsTree()
	for _, n := range wtest.RolesOf(root, semantics.RoleCheckbox) {
		if n.Label == "Auto-hide" && n.State&semantics.Checked != 0 {
			return n
		}
	}
	t.Fatal("no pin button in a flyout")
	return nil
}

// groupLabels returns the labels of the nodes with role.
func (h *dockHarness) labels(role semantics.Role) []string {
	var out []string
	for _, n := range h.nodes(role) {
		out = append(out, n.Label)
	}
	return out
}

func TestDockLayout_UnpinAndPinRestoreTheOrigin(t *testing.T) {
	h := newDockHarness(t, twoGroups(), "files", "a", "b")
	h.ctrl.Unpin("files", DockCenter)
	h.render()
	if got := h.shape(); got != "[a b*]" {
		t.Fatalf("got %s", got)
	}
	if got := hiddenShape(h.ctrl); !slices.Equal(got, []string{"left:files"}) {
		t.Fatalf("a group at the left hides at the left, got %v", got)
	}
	a := h.ctrl.Arrangement()
	if hd := a.Hidden[0]; hd.Size != 250 || !near(hd.Share, 0.25) || !slices.Equal(hd.Origin, []string{"files"}) {
		t.Errorf("got %+v", hd)
	}
	lists := h.nodes(semantics.RoleTabList)
	if len(lists) != 2 || lists[1].Label != "Hidden panels" || len(lists[1].Children) != 1 {
		t.Fatalf("the strip is a tab list of its own, got %v", h.labels(semantics.RoleTabList))
	}
	tab := h.tab(t, "files").Rect
	if tab.X < 0 || tab.X+tab.W > dockStripThickness || tab.H <= tab.W {
		t.Errorf("the tab stands on the left strip, turned, got %v", tab)
	}
	if r := h.ctrl.groupOf("a").rect; r.X != dockStripThickness || r.W != 1000-dockStripThickness {
		t.Errorf("the groups leave the strip its room, got %v", r)
	}
	h.ctrl.Pin("files")
	h.render()
	if got := h.shape(); got != "H([files*] | [a b*])" {
		t.Fatalf("with its group gone, the panel docks at its edge, got %s", got)
	}
	if w := h.ctrl.root.weights; !near(w[0], 0.25) {
		t.Errorf("at the share it had, got %v", w)
	}
	if h.inits["files"] != 1 {
		t.Errorf("the panel keeps its state, got %v", h.inits)
	}
	if len(h.ctrl.hidden) != 0 || len(h.nodes(semantics.RoleTabList)) != 2 {
		t.Error("the strip goes with the last hidden panel")
	}

	// A panel whose group is still there goes back into it, in its place.
	h.ctrl.Unpin("a", DockRight)
	h.render()
	if got := h.shape(); got != "H([files*] | [b*])" {
		t.Fatalf("got %s", got)
	}
	h.ctrl.Pin("a")
	h.render()
	if got := h.shape(); got != "H([files*] | [a* b])" {
		t.Errorf("got %s", got)
	}
	if len(h.changes) != 0 {
		t.Error("the controller's changes are not the user's")
	}
}

func TestDockLayout_UnpinGroupPinsBackTogether(t *testing.T) {
	h := newDockHarness(t, twoGroups(), "files", "a", "b")
	b := h.states["b"]
	h.ctrl.UnpinGroup("a", DockCenter)
	h.render()
	if got := hiddenShape(h.ctrl); !slices.Equal(got, []string{"right:a", "right:b"}) {
		t.Fatalf("got %v", got)
	}
	if got := h.shape(); got != "[files*]" {
		t.Fatalf("got %s", got)
	}
	h.ctrl.Pin("b")
	h.render()
	if got := h.shape(); got != "H([files*] | [a b*])" {
		t.Fatalf("the panels hidden together come back together, got %s", got)
	}
	if w := h.ctrl.root.weights; !near(w[1], dockMaxShare) {
		t.Errorf("a share of three quarters comes back at most %v, got %v", dockMaxShare, w)
	}
	if h.states["b"] != b || h.inits["b"] != 1 {
		t.Error("b keeps its state")
	}
	// Hiding a panel hidden already moves it to the other side.
	h.ctrl.Unpin("files", DockTop)
	h.ctrl.Unpin("files", DockBottom)
	h.render()
	if got := hiddenShape(h.ctrl); !slices.Equal(got, []string{"bottom:files"}) {
		t.Errorf("got %v", got)
	}
	if r := h.dock(t).strips[DockBottom]; r != (geom.Rect{Y: 700 - dockStripThickness, W: 1000, H: dockStripThickness}) {
		t.Errorf("got %v", r)
	}
}

func TestDockLayout_FlyoutOpensAndHides(t *testing.T) {
	setup := func(t *testing.T) *dockHarness {
		h := newDockHarness(t, twoGroups(), "files", "a", "b")
		h.ctrl.Unpin("files", DockCenter)
		h.render()
		h.tap(wtest.Center(h.tab(t, "files").Rect))
		if h.ctrl.Flyout() != "files" {
			t.Fatal("a press on the tab slides the panel out")
		}
		return h
	}
	t.Run("タブで開いて閉じる", func(t *testing.T) {
		h := setup(t)
		if h.focused != "files" {
			t.Errorf("the panel takes the keyboard, got %q", h.focused)
		}
		if r := h.dock(t).fly; r != (geom.Rect{X: dockStripThickness, W: 250, H: 700}) {
			t.Errorf("it slides out over the groups at its size, got %v", r)
		}
		if got := h.shape(); got != "[a b*]" {
			t.Errorf("the groups stay, got %s", got)
		}
		tab := h.tab(t, "files")
		if tab.State&semantics.Selected == 0 {
			t.Error("the tab of the panel out is selected")
		}
		if !slices.Contains(h.labels(semantics.RoleGroup), "files") {
			t.Error("the panel out is a group named by its title")
		}
		h.tap(wtest.Center(tab.Rect))
		if h.ctrl.Flyout() != "" || slices.Contains(h.labels(semantics.RoleGroup), "files") {
			t.Error("the tab again hides it")
		}
		if h.inits["files"] != 1 || len(h.changes) != 0 {
			t.Errorf("got inits %v and %d changes", h.inits, len(h.changes))
		}
	})
	t.Run("Escape", func(t *testing.T) {
		h := setup(t)
		h.key(input.KeyEscape, 0)
		if h.ctrl.Flyout() != "" {
			t.Fatal("Escape hides it")
		}
		if !onStripTab(h.tree) {
			t.Error("and gives the keyboard to its tab")
		}
		h.key(input.KeyEnter, 0)
		if h.ctrl.Flyout() != "files" || h.focused != "files" {
			t.Error("Enter on the tab slides it out again")
		}
	})
	t.Run("外を押す", func(t *testing.T) {
		h := setup(t)
		h.tap(geom.Offset{X: 100, Y: 400})
		if h.ctrl.Flyout() != "files" {
			t.Fatal("a press inside it, on nothing, keeps it out")
		}
		if h.focused != "files" || h.tree.Focused() == nil {
			t.Error("and keeps the keyboard in it")
		}
		h.tap(geom.Offset{X: 700, Y: 400})
		if h.ctrl.Flyout() != "" {
			t.Error("a press outside hides it")
		}
	})
	t.Run("フォーカスが出る", func(t *testing.T) {
		h := setup(t)
		r := h.dock(t)
		var other core.FocusTarget
		for i := range r.slots {
			if r.slots[i].kind == dockSlotPane && r.slots[i].panel == "b" {
				other = kit.FirstFocusable(r.children[i])
			}
		}
		h.tree.RequestFocus(other)
		h.render()
		if h.ctrl.Flyout() != "" || h.focused != "b" {
			t.Errorf("the focus going elsewhere hides it, got %q %q", h.ctrl.Flyout(), h.focused)
		}
		h.ctrl.Show("files")
		h.render()
		h.tree.RequestFocus(nil)
		h.render()
		if h.ctrl.Flyout() != "" {
			t.Error("so does the focus going nowhere")
		}
	})
	t.Run("別のタブ", func(t *testing.T) {
		h := setup(t)
		h.ctrl.Unpin("b", DockRight)
		h.render()
		if h.ctrl.Flyout() != "" {
			t.Fatal("hiding another panel hides the one out")
		}
		h.ctrl.Show("files")
		h.render()
		h.tap(wtest.Center(h.tab(t, "b").Rect))
		if h.ctrl.Flyout() != "b" || h.focused != "b" {
			t.Errorf("the tab of another slides it out instead, got %q %q", h.ctrl.Flyout(), h.focused)
		}
		if f := h.dock(t).fly; f.X+f.W != 1000-dockStripThickness {
			t.Errorf("b comes out on the right, got %v", f)
		}
	})
}

func TestDockLayout_FlyoutSlides(t *testing.T) {
	h := newDockHarness(t, twoGroups(), "files", "a", "b")
	h.motion = true
	h.swap(h.widget())
	h.render()
	h.ctrl.Unpin("files", DockCenter)
	h.render()
	h.ctrl.Show("files")
	h.render()
	if f := h.dock(t).fly; f.X+f.W > dockStripThickness+1 {
		t.Fatalf("it starts hidden, got %v", f)
	}
	now := time.Second
	wtest.Advance(h.tree, h.size, &now, 100*time.Millisecond)
	mid := h.dock(t).fly
	wtest.Settled(t, h.tree, h.size, &now)
	end := h.dock(t).fly
	if !(mid.X > dockStripThickness-250 && mid.X < end.X) || end.X != dockStripThickness {
		t.Errorf("it slides out, got %v then %v", mid, end)
	}
}

func TestDockLayout_FlyoutResizeIsRemembered(t *testing.T) {
	h := newDockHarness(t, twoGroups(), "files", "a", "b")
	h.ctrl.Unpin("files", DockCenter)
	h.render()
	h.ctrl.Show("files")
	h.render()
	seps := h.nodes(semantics.RoleSeparator)
	if len(seps) != 1 || seps[0].Label != "Pane divider" {
		t.Fatalf("got %d separators", len(seps))
	}
	if seps[0].Range.Max != 100 || !near(float32(seps[0].Range.Value), 250/9.72) {
		t.Errorf("got %+v", seps[0].Range)
	}
	at := wtest.Center(seps[0].Rect)
	if geom.Abs32(at.X-(dockStripThickness+250)) > 1 {
		t.Fatalf("the handle is on the inner edge, got %v", at)
	}
	h.pointer(core.PointerMove, at)
	h.pointer(core.PointerDown, at)
	h.pointer(core.PointerMove, at.Add(geom.Offset{X: 100}))
	r := h.dock(t)
	if r.fly.W != 350 {
		t.Errorf("the edge follows the pointer, got %v", r.fly.W)
	}
	if len(h.changes) != 0 {
		t.Error("OnChange waits for the end of the drag")
	}
	h.pointer(core.PointerMove, at.Add(geom.Offset{X: 2000}))
	if r.fly.W != 1000-dockStripThickness-dockFlyoutMargin {
		t.Errorf("it leaves some of the groups, got %v", r.fly.W)
	}
	h.pointer(core.PointerMove, at.Add(geom.Offset{X: -1000}))
	if r.fly.W != dockMinGroupWidth {
		t.Errorf("and keeps its smallest width, got %v", r.fly.W)
	}
	h.pointer(core.PointerMove, at.Add(geom.Offset{X: 50}))
	h.pointer(core.PointerUp, at.Add(geom.Offset{X: 50}))
	if len(h.changes) != 1 || h.changes[0].Hidden[0].Size != 300 {
		t.Fatalf("got %d changes", len(h.changes))
	}
	if h.ctrl.Flyout() != "files" {
		t.Error("the handle is part of the flyout")
	}
	h.ctrl.HideFlyout()
	h.render()
	h.ctrl.Show("files")
	h.render()
	if r.fly.W != 300 {
		t.Errorf("the size is remembered, got %v", r.fly.W)
	}
	// The keys move the edge, and a double click goes back to the size the
	// layout chooses.
	at = wtest.Center(h.nodes(semantics.RoleSeparator)[0].Rect)
	h.pointer(core.PointerDown, at)
	h.pointer(core.PointerUp, at)
	h.key(input.KeyRight, 0)
	if r.fly.W != 300+dockKeyStep {
		t.Errorf("got %v", r.fly.W)
	}
	h.time += 2000
	at = wtest.Center(h.nodes(semantics.RoleSeparator)[0].Rect)
	h.pointer(core.PointerDown, at)
	h.pointer(core.PointerUp, at)
	h.pointer(core.PointerDown, at)
	h.pointer(core.PointerUp, at)
	if want := geom.Floor32((1000 - dockStripThickness) * dockFlyoutShare); r.fly.W != want {
		t.Errorf("got %v, want %v", r.fly.W, want)
	}
}

func TestDockLayout_PanelsChangeWhileAFlyoutIsOut(t *testing.T) {
	h := newDockHarness(t, twoGroups(), "files", "a", "b")
	h.ctrl.Unpin("files", DockCenter)
	h.render()
	h.ctrl.Show("files")
	h.render()
	h.setPanels("files", "a", "b", "c")
	if h.ctrl.Flyout() != "files" || h.shape() != "[a b* c]" {
		t.Errorf("a new panel leaves it out, got %q %s", h.ctrl.Flyout(), h.shape())
	}
	h.setPanels("a", "b", "c")
	if h.ctrl.Flyout() != "" || len(h.ctrl.hidden) != 0 {
		t.Errorf("a panel that goes takes its flyout, got %q", h.ctrl.Flyout())
	}
	if len(h.nodes(semantics.RoleTabList)) != 1 {
		t.Error("and its strip")
	}
	h.setPanels("files", "a", "b", "c")
	h.ctrl.Activate("files")
	h.render()
	if got := h.shape(); got != "[a b c files*]" || h.inits["files"] != 2 {
		t.Errorf("it comes back as a new panel, got %s %v", got, h.inits)
	}
}

func TestDockLayout_AutoHideButtonsAndKeys(t *testing.T) {
	h := newAutoHideHarness(t, twoGroups(), "files", "a", "b")
	root, _ := h.tree.SemanticsTree()
	var pins []*semantics.Node
	for _, n := range wtest.RolesOf(root, semantics.RoleCheckbox) {
		if n.Label == "Auto-hide" {
			pins = append(pins, n)
		}
	}
	if len(pins) != 2 || pins[0].State&semantics.Checked != 0 {
		t.Fatalf("each strip has a pin button, unchecked, got %d", len(pins))
	}
	h.tree.PerformSemanticsAction(pins[1].ID, semantics.ActionTap, core.SemanticsArg{})
	h.render()
	if got := hiddenShape(h.ctrl); !slices.Equal(got, []string{"right:a", "right:b"}) {
		t.Fatalf("the pin button hides the whole group, got %v", got)
	}
	if !onStripTab(h.tree) || len(h.changes) != 1 {
		t.Error("the keyboard goes to the tab of the selected panel, and it is a change")
	}
	h.key(input.KeyEnter, 0)
	if h.ctrl.Flyout() != "b" || h.focused != "b" {
		t.Fatalf("got %q %q", h.ctrl.Flyout(), h.focused)
	}
	if pin := h.flyoutPin(t); pin.Value != "checked" {
		t.Errorf("the pin of the flyout is checked, got %+v", pin)
	}
	// Ctrl+Alt+PageUp docks it back with the one hidden along with it.
	const ctrlAlt = input.ModControl | input.ModAlt
	h.key(input.KeyPageUp, ctrlAlt)
	if got := h.shape(); got != "H([files*] | [a b*])" || h.focused != "b" {
		t.Fatalf("got %s, focus %q", got, h.focused)
	}
	// Ctrl+Alt+PageDown hides the selected panel alone, and with Shift the
	// whole group.
	h.key(input.KeyPageDown, ctrlAlt)
	if got := hiddenShape(h.ctrl); !slices.Equal(got, []string{"right:b"}) {
		t.Fatalf("got %v", got)
	}
	h.key(input.KeyEnter, 0)
	h.key(input.KeyPageDown, ctrlAlt)
	if h.ctrl.Flyout() != "" || !onStripTab(h.tree) {
		t.Error("in the flyout, Ctrl+Alt+PageDown puts it away")
	}
	h.key(input.KeyEnter, 0)
	h.tree.PerformSemanticsAction(h.flyoutPin(t).ID, semantics.ActionTap, core.SemanticsArg{})
	h.render()
	if got := h.shape(); got != "H([files*] | [a b*])" {
		t.Fatalf("the pin of the flyout docks it back, got %s", got)
	}
	h.ctrl.current = h.ctrl.groupOf("files")
	h.key(input.KeyPageDown, ctrlAlt|input.ModShift)
	if got := hiddenShape(h.ctrl); !slices.Equal(got, []string{"left:files"}) {
		t.Errorf("got %v", got)
	}
	if h.inits["b"] != 1 || h.inits["files"] != 1 {
		t.Errorf("got %v", h.inits)
	}
}

func TestDockLayout_AutoHideIsOff(t *testing.T) {
	h := newDockHarness(t, twoGroups(), "files", "a", "b")
	root, _ := h.tree.SemanticsTree()
	for _, n := range wtest.RolesOf(root, semantics.RoleCheckbox) {
		t.Errorf("no pin buttons without AutoHide, got %q", n.Label)
	}
	h.ctrl.current = h.ctrl.groupOf("a")
	h.key(input.KeyPageDown, input.ModControl|input.ModAlt)
	if len(h.ctrl.hidden) != 0 {
		t.Error("nor the key")
	}
	// The controller still hides, and the user still shows and pins back.
	h.ctrl.Unpin("a", DockBottom)
	h.render()
	h.tap(wtest.Center(h.tab(t, "a").Rect))
	root, _ = h.tree.SemanticsTree()
	h.tree.PerformSemanticsAction(wtest.ByLabel(t, root, "Auto-hide").ID, semantics.ActionTap, core.SemanticsArg{})
	h.render()
	if got := h.shape(); got != "H([files*] | [a* b])" {
		t.Errorf("got %s", got)
	}
}

func TestDockLayout_FlyoutCloseAndActivate(t *testing.T) {
	h := newDockHarness(t, twoGroups(), "files", "a", "b")
	h.ctrl.Unpin("a", DockBottom)
	h.render()
	h.ctrl.Activate("a")
	h.render()
	if h.ctrl.Flyout() != "a" || h.focused != "a" {
		t.Fatal("Activate slides a hidden panel out")
	}
	h.key(input.KeyW, input.ModControl)
	if !slices.Equal(h.closed, []string{"a"}) || len(h.ctrl.hidden) != 0 || h.ctrl.Flyout() != "" {
		t.Fatalf("Ctrl+W in the flyout closes its panel, got %v", h.closed)
	}
	if got := h.shape(); got != "H([files*] | [b*])" {
		t.Errorf("and nothing else, got %s", got)
	}
	h.ctrl.Activate("a")
	h.render()
	if got := h.shape(); got != "H([files a*] | [b*])" {
		t.Errorf("a closed panel opens in the current group, got %s", got)
	}
	// The close button of the header closes it too.
	h.ctrl.Unpin("a", DockTop)
	h.render()
	h.ctrl.Show("a")
	h.render()
	root, _ := h.tree.SemanticsTree()
	h.tree.PerformSemanticsAction(wtest.ByLabel(t, root, "Close a").ID, semantics.ActionTap, core.SemanticsArg{})
	h.render()
	if len(h.closed) != 2 || h.ctrl.Arrangement().Closed[0] != "a" {
		t.Errorf("got %v", h.closed)
	}
	// Close from the controller works on a hidden panel.
	h.ctrl.Unpin("b", DockTop)
	h.ctrl.Close("b")
	h.render()
	if len(h.ctrl.hidden) != 0 || !slices.Contains(h.ctrl.closed, "b") {
		t.Error("got hidden panels left")
	}
}

func TestDockLayout_DragToAndFromStrips(t *testing.T) {
	t.Run("帯へ落とす", func(t *testing.T) {
		h := newDockHarness(t, twoGroups(), "files", "a", "b")
		h.ctrl.Unpin("files", DockCenter)
		h.render()
		at := geom.Offset{X: 10, Y: 500}
		h.press(t, "a", at)
		if r := h.dock(t); !r.previewShow || r.preview.X > dockStripThickness {
			t.Errorf("the preview covers the strip, got %v %v", r.previewShow, r.preview)
		}
		h.pointer(core.PointerUp, at)
		if got := hiddenShape(h.ctrl); !slices.Equal(got, []string{"left:files", "left:a"}) {
			t.Errorf("got %v", got)
		}
		if len(h.changes) != 2 {
			t.Errorf("got %d changes", len(h.changes))
		}
	})
	t.Run("帯から群へ", func(t *testing.T) {
		h := newDockHarness(t, twoGroups(), "files", "a", "b")
		h.ctrl.Unpin("files", DockCenter)
		h.render()
		from := wtest.Center(h.tab(t, "files").Rect)
		at := geom.Offset{X: 600, Y: 400}
		h.pointer(core.PointerMove, from)
		h.pointer(core.PointerDown, from)
		h.pointer(core.PointerMove, from.Add(geom.Offset{X: 8}))
		h.pointer(core.PointerMove, at)
		if !h.dock(t).previewShow {
			t.Error("no preview")
		}
		h.pointer(core.PointerUp, at)
		if got := h.shape(); got != "[a b files*]" || len(h.ctrl.hidden) != 0 {
			t.Errorf("got %s", got)
		}
		if h.ctrl.Flyout() != "" || h.inits["files"] != 1 || len(h.changes) != 1 {
			t.Errorf("got %q %v %d", h.ctrl.Flyout(), h.inits, len(h.changes))
		}
	})
	t.Run("帯から端へ、Escape で取り消し", func(t *testing.T) {
		h := newDockHarness(t, twoGroups(), "files", "a", "b")
		h.ctrl.Unpin("files", DockCenter)
		h.render()
		from := wtest.Center(h.tab(t, "files").Rect)
		at := geom.Offset{X: 990, Y: 400}
		h.pointer(core.PointerMove, from)
		h.pointer(core.PointerDown, from)
		h.pointer(core.PointerMove, at)
		h.key(input.KeyEscape, 0)
		if h.dock(t).previewShow {
			t.Error("Escape takes the preview away")
		}
		h.pointer(core.PointerUp, at)
		if len(h.ctrl.hidden) != 1 || h.ctrl.Flyout() != "" {
			t.Fatal("and the drag")
		}
		h.pointer(core.PointerMove, from)
		h.pointer(core.PointerDown, from)
		h.pointer(core.PointerMove, at)
		h.pointer(core.PointerUp, at)
		if got := h.shape(); got != "H([a b*] | [files*])" {
			t.Errorf("got %s", got)
		}
	})
	t.Run("ドラッグ中にパネルが消える", func(t *testing.T) {
		h := newDockHarness(t, twoGroups(), "files", "a", "b")
		h.ctrl.Unpin("files", DockCenter)
		h.render()
		from := wtest.Center(h.tab(t, "files").Rect)
		at := geom.Offset{X: 990, Y: 400}
		h.pointer(core.PointerMove, from)
		h.pointer(core.PointerDown, from)
		h.pointer(core.PointerMove, at)
		h.setPanels("a", "b")
		if r := h.dock(t); r.previewShow || h.ctrl.view.stripDrag != "" {
			t.Error("the drag ends with its tab")
		}
		h.pointer(core.PointerUp, at)
		if got := h.shape(); got != "[a b*]" || len(h.ctrl.hidden) != 0 {
			t.Errorf("got %s", got)
		}
	})
	t.Run("帯の上で離すと開く", func(t *testing.T) {
		h := newDockHarness(t, twoGroups(), "files", "a", "b")
		h.ctrl.Unpin("files", DockCenter)
		h.render()
		from := wtest.Center(h.tab(t, "files").Rect)
		h.pointer(core.PointerMove, from)
		h.pointer(core.PointerDown, from)
		h.pointer(core.PointerMove, from.Add(geom.Offset{Y: 2}))
		h.pointer(core.PointerUp, from.Add(geom.Offset{Y: 2}))
		if h.ctrl.Flyout() != "files" {
			t.Error("a press that barely moves is a tap")
		}
	})
}

func TestDockLayout_UnpinFromAMaximizedGroup(t *testing.T) {
	h := newDockHarness(t, twoGroups(), "files", "a", "b")
	h.ctrl.Maximize("b")
	h.ctrl.Unpin("b", DockCenter)
	h.render()
	if h.ctrl.maximized == nil || h.ctrl.groupOf("a") != h.ctrl.maximized {
		t.Error("the group stays maximized with a panel left")
	}
	h.ctrl.Show("b")
	h.render()
	if h.ctrl.Flyout() != "b" {
		t.Error("a panel slides out over a maximized group")
	}
	h.ctrl.Pin("b")
	h.render()
	if h.ctrl.maximized != nil || h.shape() != "H([files*] | [a b*])" {
		t.Errorf("pinning brings the others back, got %s", h.shape())
	}
}

func TestDockController_AutoHideZeroValue(t *testing.T) {
	var c DockController
	c.Unpin("a", DockLeft)
	c.UnpinGroup("a", DockLeft)
	c.Pin("a")
	c.Show("a")
	c.HideFlyout()
	if c.Flyout() != "" || len(c.Arrangement().Hidden) != 0 {
		t.Error("a controller that never showed has nothing to hide")
	}
	// Everything hidden leaves an empty group, and the first pin fills it.
	d := controllerOf(t, DockArrangement{}, "a", "b")
	d.UnpinGroup("a", DockTop)
	if got := shape(d.root); got != "[]" {
		t.Fatalf("got %s", got)
	}
	d.Pin("a")
	if got := got(t, d); got != "[a* b]" {
		t.Errorf("got %s", got)
	}
	// Hidden panels that the layout no longer has, or twice, go.
	e := controllerOf(t, DockArrangement{Root: DockNode{Panels: []string{"a"}}, Hidden: []DockHidden{
		{ID: "gone", Side: DockLeft}, {ID: "b", Side: DockZone(9)}, {ID: "b", Side: DockRight}, {ID: "a", Side: DockTop},
	}}, "a", "b")
	if got := hiddenShape(e); !slices.Equal(got, []string{"left:b"}) || shape(e.root) != "[a*]" {
		t.Errorf("got %v %s", got, shape(e.root))
	}
	// A restored panel without an origin docks at its edge.
	e.Pin("b")
	if got := got(t, e); got != "H([b*] | [a*])" {
		t.Errorf("got %s", got)
	}
}

func TestDockArrangement_JSONHidden(t *testing.T) {
	c := controllerOf(t, twoGroups(), "files", "a", "b")
	c.root.place(geom.Rect{W: 1000, H: 700})
	c.Unpin("files", DockCenter)
	c.UnpinGroup("b", DockBottom)
	a := c.Arrangement()
	data, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"version":1,"hidden":{"bottom":[{"id":"a","size":700,"origin":["a","b"],"share":1},{"id":"b","size":700,"origin":["a","b"],"share":1}],"left":[{"id":"files","size":250,"origin":["files"],"share":0.25}]}}`
	if string(data) != want {
		t.Errorf("got %s", data)
	}
	var back DockArrangement
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	d := &DockController{}
	d.SetArrangement(back)
	d.sync([]string{"files", "a", "b"}, &DockArrangement{})
	if got := hiddenShape(d); !slices.Equal(got, []string{"left:files", "bottom:a", "bottom:b"}) {
		t.Errorf("got %v", got)
	}
	again, err := json.Marshal(d.Arrangement())
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != want {
		t.Errorf("a second round trip must not change the text, got %s", again)
	}
	d.Pin("a")
	if got := got(t, d); got != "[a* b]" {
		t.Errorf("got %s", got)
	}
	// A document of version 1 from before hidden panels, and one with a side
	// this version does not know.
	var old DockArrangement
	if err := json.Unmarshal([]byte(`{"version":1,"root":{"panels":["a","b"]}}`), &old); err != nil || old.Hidden != nil {
		t.Errorf("got %v %+v", err, old)
	}
	var odd DockArrangement
	if err := json.Unmarshal([]byte(`{"version":1,"hidden":{"middle":[{"id":"a"}],"right":[{"id":"b"}]}}`), &odd); err != nil {
		t.Fatal(err)
	}
	if len(odd.Hidden) != 1 || odd.Hidden[0].ID != "b" || odd.Hidden[0].Side != DockRight {
		t.Errorf("an unknown side is ignored, got %+v", odd.Hidden)
	}
	e := &DockController{}
	e.SetArrangement(odd)
	e.sync([]string{"a", "b"}, &DockArrangement{})
	if got := shape(e.root); got != "[a*]" || len(e.hidden) != 1 {
		t.Errorf("got %s", got)
	}
}

func TestDockLayout_FlyoutOfANewLayout(t *testing.T) {
	// A layout that starts from a Default with hidden panels shows the strip
	// at once, and builds a hidden panel only when it first slides out.
	def := twoGroups()
	def.Hidden = []DockHidden{{ID: "out", Side: DockBottom, Size: 120}}
	h := newDockHarness(t, def, "files", "a", "b", "out")
	if h.inits["out"] != 0 || len(h.nodes(semantics.RoleTabList)) != 3 {
		t.Fatalf("got %v", h.inits)
	}
	h.tap(wtest.Center(h.tab(t, "out").Rect))
	if f := h.dock(t).fly; f != (geom.Rect{Y: 700 - dockStripThickness - 120, W: 1000, H: 120}) || h.inits["out"] != 1 {
		t.Errorf("got %v %v", f, h.inits)
	}
	tree := wtest.TextTree(t, DockLayout{Default: def, Panels: []DockPanel{{ID: "out", Title: "Out"}}})
	testutil.RenderTree(tree, geom.Size{W: 400, H: 300})
}
