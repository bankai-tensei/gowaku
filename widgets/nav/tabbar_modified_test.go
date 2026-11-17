package nav

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
)

// dotsOf counts the dots of modified tabs below ro.
func dotsOf(ro core.RenderObject) int {
	n := 0
	walkRender(ro, func(ro core.RenderObject) {
		if _, ok := ro.(*layout.RenderDecoratedBox); ok && ro.Size() == (geom.Size{W: tabDotSize, H: tabDotSize}) {
			n++
		}
	})
	return n
}

func TestTabBar_ModifiedShowsADotThatTurnsIntoClose(t *testing.T) {
	m := newTabModel("a.go", "b.go", "c.go")
	m.tabs[0].Modified = true
	m.tabs[1].Modified = true
	m.tabs[2].Permanent = true
	m.tabs[2].Modified = true
	h := newTabBarHarness(t, 800, false, m, nil)
	run := findRun(t, h.tree)
	// The selected tab and the unselected one show their dots, and so does
	// the permanent one, which has no close button.
	if got := dotsOf(run); got != 3 {
		t.Fatalf("got %d dots, want 3", got)
	}
	tabs := h.tabs(t, 0)
	for i, n := range tabs {
		if n.Value != "Modified" {
			t.Errorf("tab %d: value %q, want Modified", i, n.Value)
		}
	}
	// Over the unselected tab, its dot turns into the close button, which
	// closes.
	at := wtest.Center(tabs[1].Children[0].Rect)
	h.pointer(core.PointerMove, at, 0)
	h.settle(t)
	if got := dotsOf(run); got != 2 {
		t.Errorf("with the pointer over a modified tab, got %d dots, want 2", got)
	}
	h.pointer(core.PointerDown, at, input.ButtonPrimary)
	h.pointer(core.PointerUp, at, input.ButtonPrimary)
	if got := m.titles(); !slices.Equal(got, []string{"a.go", "c.go"}) {
		t.Errorf("the close button under the pointer must close, got %v", got)
	}
	// The dot of the selected tab closes too: it stands for its button.
	root, _ := h.tree.SemanticsTree()
	close := wtest.ByLabel(t, root, "Close a.go")
	h.tree.PerformSemanticsAction(close.ID, semantics.ActionTap, core.SemanticsArg{})
	h.render()
	if got := m.titles(); !slices.Equal(got, []string{"c.go"}) {
		t.Errorf("got %v", got)
	}
}

func TestTabBar_UnmodifiedHasNoValue(t *testing.T) {
	m := newTabModel("a", "b")
	h := newTabBarHarness(t, 800, false, m, nil)
	for i, n := range h.tabs(t, 0) {
		if n.Value != "" {
			t.Errorf("tab %d: value %q", i, n.Value)
		}
	}
	if got := dotsOf(findRun(t, h.tree)); got != 0 {
		t.Errorf("got %d dots", got)
	}
}
