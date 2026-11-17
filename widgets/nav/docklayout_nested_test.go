package nav

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/semantics"
)

// nestedHarness shows an outer DockLayout of 1000×700 with left and more in
// a group at the left and, at the right, the panel inner, which holds a
// DockLayout of x at its left and y and z at its right. Every panel but inner
// is a probe.
type nestedHarness struct {
	*dockHarness
	inner     *DockController
	showInner bool
	// innerChanges are the changes the user made to the inner layout.
	innerChanges int
}

func newNestedHarness(t *testing.T) *nestedHarness {
	t.Helper()
	h := &dockHarness{
		ctrl: &DockController{}, size: geom.Size{W: 1000, H: 700},
		inits: map[string]int{}, states: map[string]*dockProbeState{},
	}
	n := &nestedHarness{dockHarness: h, inner: &DockController{}, showInner: true}
	h.tree, h.swap = wtest.SwapTree(t, n.widget())
	h.render()
	return n
}

func (n *nestedHarness) probe(id string) DockPanel {
	h := n.dockHarness
	return DockPanel{ID: id, Title: id, Closable: true, Build: func(core.BuildContext) core.Widget { return dockProbe{id: id, h: h} }}
}

func (n *nestedHarness) widget() core.Widget {
	inner := DockLayout{
		Panels: []DockPanel{n.probe("x"), n.probe("y"), n.probe("z")}, Controller: n.inner, Label: "内側",
		Default: DockArrangement{Root: DockNode{Children: []DockNode{
			{Panels: []string{"x"}}, {Panels: []string{"y", "z"}, Active: "y"},
		}}},
		OnChange: func(DockArrangement) { n.innerChanges++ },
	}
	panels := []DockPanel{n.probe("left"), n.probe("more")}
	if n.showInner {
		panels = append(panels, DockPanel{ID: "inner", Title: "inner", Build: func(core.BuildContext) core.Widget { return inner }})
	}
	h := n.dockHarness
	return basic.MediaQuery{
		Data: basic.MediaQueryData{Scale: 1, ReduceMotion: true},
		Child: overlay.Overlay{Child: DockLayout{
			Panels: panels, Controller: h.ctrl, Label: "パネル",
			Default: DockArrangement{Root: DockNode{Children: []DockNode{
				{Panels: []string{"left", "more"}, Active: "left"}, {Panels: []string{"inner"}},
			}}},
			OnChange: func(a DockArrangement) { h.changes = append(h.changes, a) },
		}},
	}
}

func (n *nestedHarness) innerShape() string { return shape(n.inner.root) }

func TestDockLayout_NestedKeysActOnTheInnermost(t *testing.T) {
	n := newNestedHarness(t)
	const ctrlAlt = input.ModControl | input.ModAlt
	if got := n.innerShape(); got != "H([x*] | [y* z])" {
		t.Fatalf("got %s", got)
	}
	// From outside both, the outer layout takes the keys.
	n.key(input.KeyRight, ctrlAlt)
	if n.focused != "x" {
		t.Fatalf("the outer layout moves the focus into the inner one, got %q", n.focused)
	}
	n.key(input.KeyRight, ctrlAlt)
	if n.focused != "y" {
		t.Fatalf("within the inner layout, it takes the keys, got %q", n.focused)
	}
	if n.ctrl.currentGroup() != n.ctrl.groupOf("inner") {
		t.Error("the outer layout follows the focus into the group of inner")
	}
	// At the edge of the inner layout, the arrow goes on to the outer one,
	// which has nothing further right either.
	n.key(input.KeyRight, ctrlAlt)
	if n.focused != "y" || n.tree.Focused() == nil {
		t.Errorf("got %q", n.focused)
	}
	// Ctrl+Tab acts on the inner group alone.
	n.key(input.KeyTab, input.ModControl)
	if n.inner.groupOf("z").active != "z" || n.ctrl.groupOf("left").active != "left" {
		t.Errorf("got inner %s, outer %s", n.innerShape(), n.shape())
	}
	n.key(input.KeyTab, input.ModControl)
	n.key(input.KeyLeft, ctrlAlt)
	n.key(input.KeyLeft, ctrlAlt)
	if n.focused != "left" {
		t.Fatalf("past the left edge of the inner layout, the outer one moves, got %q", n.focused)
	}
	// With the focus in the outer layout, the inner one is left alone.
	n.key(input.KeyLeft, ctrlAlt)
	if n.focused != "left" || n.tree.Focused() == nil || n.inner.current != n.inner.groupOf("y") {
		t.Error("a key the outer layout has no use for does not reach the inner one")
	}
	n.key(input.KeyTab, input.ModControl)
	if n.ctrl.groupOf("left").active != "more" || n.inner.groupOf("y").active != "z" {
		t.Errorf("got outer %s, inner %s", n.shape(), n.innerShape())
	}
	// From outside both, the inner layout never takes a key.
	n.tree.RequestFocus(nil)
	n.render()
	n.key(input.KeyTab, input.ModControl)
	if n.ctrl.groupOf("left").active != "left" || n.inner.groupOf("z").active != "z" {
		t.Errorf("got outer %s, inner %s", n.shape(), n.innerShape())
	}
	n.ctrl.current = n.ctrl.groupOf("inner")
	n.key(input.KeyRight, ctrlAlt)
	if n.tree.Focused() != nil {
		t.Errorf("got the focus in %q", n.focused)
	}
}

func TestDockLayout_NestedMaximize(t *testing.T) {
	n := newNestedHarness(t)
	const ctrlAlt = input.ModControl | input.ModAlt
	n.key(input.KeyRight, ctrlAlt)
	n.key(input.KeyRight, ctrlAlt)
	n.key(input.KeyEnter, ctrlAlt)
	if n.inner.maximized != n.inner.groupOf("y") || n.ctrl.maximized != nil {
		t.Fatal("Ctrl+Alt+Enter maximizes within the inner layout")
	}
	if r := n.inner.groupOf("y").rect; r != (geom.Rect{W: 499, H: 666}) {
		t.Errorf("over the inner layout only, got %v", r)
	}
	n.key(input.KeyEnter, ctrlAlt)
	// An inner layout of one group has nothing to maximize: the outer one does.
	n.inner.Move("x", "y", DockCenter)
	n.render()
	n.key(input.KeyEnter, ctrlAlt)
	if n.inner.maximized != nil || n.ctrl.maximized != n.ctrl.groupOf("inner") {
		t.Errorf("got inner %v, outer %v", n.inner.maximized, n.ctrl.maximized)
	}
	// A double click on the inner strip maximizes there.
	n.ctrl.Maximize("")
	n.inner.Reset()
	n.render()
	// The empty end of the strip of x, past its one tab.
	strip := geom.Offset{X: 730, Y: 34 + 17}
	n.time += 2000
	for range 2 {
		n.pointer(core.PointerDown, strip)
		n.pointer(core.PointerUp, strip)
	}
	if n.inner.maximized != n.inner.groupOf("x") || n.ctrl.maximized != nil {
		t.Errorf("got inner %v, outer %v", n.inner.maximized, n.ctrl.maximized)
	}
}

func TestDockLayout_NestedDragsStayInTheirLayout(t *testing.T) {
	t.Run("内側のタブは外へ出ない", func(t *testing.T) {
		n := newNestedHarness(t)
		at := geom.Offset{X: 250, Y: 400}
		n.press(t, "y", at)
		for _, r := range n.docks() {
			if r.previewShow {
				t.Error("no layout shows a preview outside the inner one")
			}
		}
		n.pointer(core.PointerUp, at)
		if n.innerShape() != "H([x*] | [y* z])" || n.shape() != "H([left* more] | [inner*])" {
			t.Errorf("got inner %s, outer %s", n.innerShape(), n.shape())
		}
	})
	t.Run("内側の端は内側に", func(t *testing.T) {
		n := newNestedHarness(t)
		// The right edge of the inner layout is that of the outer one: the
		// drag of an inner tab docks at the edge of the inner layout.
		at := geom.Offset{X: 994, Y: 400}
		n.press(t, "y", at)
		inner, outer := n.docks()[1], n.docks()[0]
		if !inner.previewShow || outer.previewShow {
			t.Errorf("got inner %v, outer %v", inner.previewShow, outer.previewShow)
		}
		n.pointer(core.PointerUp, at)
		if got := n.innerShape(); got != "H([x*] | [z*] | [y*])" || n.innerChanges != 1 {
			t.Errorf("got %s, %d changes", got, n.innerChanges)
		}
		if n.shape() != "H([left* more] | [inner*])" || len(n.changes) != 0 {
			t.Error("the outer layout stays")
		}
	})
	t.Run("外側のタブは内側の上で外側の場所に", func(t *testing.T) {
		n := newNestedHarness(t)
		at := geom.Offset{X: 750, Y: 400}
		n.press(t, "more", at)
		outer := n.docks()[0]
		if !outer.previewShow || !wtest.NearRect(outer.preview, sideOf(n.ctrl.groupOf("inner").rect, DockCenter, 1)) {
			t.Errorf("over the inner layout the outer tab joins its group, got %v", outer.preview)
		}
		n.pointer(core.PointerUp, at)
		if got := n.shape(); got != "H([left*] | [inner more*])" {
			t.Errorf("got %s", got)
		}
	})
	t.Run("外側の端は外側に", func(t *testing.T) {
		n := newNestedHarness(t)
		n.dragTab(t, "more", geom.Offset{X: 994, Y: 400})
		if got := n.shape(); got != "H([left*] | [inner*] | [more*])" {
			t.Errorf("got %s", got)
		}
		if n.innerShape() != "H([x*] | [y* z])" {
			t.Error("the inner layout stays")
		}
	})
}

// docks returns the render objects of the layouts, the outer first.
func (n *nestedHarness) docks() []*renderDock {
	var out []*renderDock
	walkRender(n.tree.RootRenderObject(), func(ro core.RenderObject) {
		if r, ok := ro.(*renderDock); ok {
			out = append(out, r)
		}
	})
	return out
}

func TestDockLayout_NestedRemovedDuringADrag(t *testing.T) {
	n := newNestedHarness(t)
	at := geom.Offset{X: 994, Y: 400}
	n.press(t, "y", at)
	n.showInner = false
	n.swap(n.widget())
	n.render()
	n.pointer(core.PointerMove, at.Add(geom.Offset{Y: 10}))
	n.pointer(core.PointerUp, at.Add(geom.Offset{Y: 10}))
	if got := n.shape(); got != "[left* more]" {
		t.Errorf("got %s", got)
	}
	if n.dock(t).previewShow || n.inner.view != nil {
		t.Error("the inner layout leaves nothing behind")
	}
	n.showInner = true
	n.swap(n.widget())
	n.render()
	if got := n.innerShape(); got != "H([x*] | [z*] | [y*])" && got != "H([x*] | [y* z])" {
		t.Errorf("got %s", got)
	}
}

func TestDockLayout_NestedSaveAndRestoreApart(t *testing.T) {
	n := newNestedHarness(t)
	outer, err := json.Marshal(n.ctrl.Arrangement())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(outer), `"x"`) {
		t.Errorf("the outer arrangement does not hold the inner one, got %s", outer)
	}
	saved := n.inner.Arrangement()
	n.inner.Move("x", "y", DockCenter)
	n.ctrl.Move("more", "", DockBottom)
	n.render()
	n.inner.SetArrangement(saved)
	n.render()
	if n.innerShape() != "H([x*] | [y* z])" || n.shape() != "V(H([left*] | [inner*]) | [more*])" {
		t.Errorf("got inner %s, outer %s", n.innerShape(), n.shape())
	}
	// The inner layout keeps its own hidden panels and flyout, within its room.
	n.inner.Unpin("z", DockRight)
	n.render()
	n.inner.Show("z")
	n.render()
	in := n.docks()[1]
	if f, sz := in.fly, in.Size(); f.X+f.W != sz.W-dockStripThickness || f.Y != 0 || f.H != sz.H {
		t.Errorf("the flyout of the inner layout stays in it, got %v in %v", f, sz)
	}
	if n.ctrl.Flyout() != "" || len(n.ctrl.Arrangement().Hidden) != 0 {
		t.Error("the outer layout has none")
	}
	n.key(input.KeyEscape, 0)
	if n.inner.Flyout() != "" {
		t.Error("Escape hides it")
	}
}

func TestDockLayout_NestedZeroValues(t *testing.T) {
	tree := wtest.TextTree(t, DockLayout{Panels: []DockPanel{
		{ID: "a", Build: func(core.BuildContext) core.Widget { return DockLayout{} }},
		{ID: "b", Build: func(core.BuildContext) core.Widget {
			return DockLayout{Panels: []DockPanel{{ID: "a"}}}
		}},
	}})
	testutil.RenderTree(tree, geom.Size{W: 600, H: 400})
	if got := len(wtest.RolesOf(must(tree.SemanticsTree()), semantics.RoleTabList)); got != 2 {
		t.Errorf("an inner layout shows its group, got %d tab lists", got)
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab, Modifiers: input.ModControl})
	testutil.RenderTree(tree, geom.Size{W: 600, H: 400})
}
