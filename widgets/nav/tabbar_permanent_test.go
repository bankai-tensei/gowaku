package nav

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
)

func TestTabBar_PermanentTabDoesNotClose(t *testing.T) {
	m := newTabModel("a", "b")
	m.tabs[0].Permanent = true
	h := newTabBarHarness(t, 800, false, m, nil)
	root, _ := h.tree.SemanticsTree()
	var labels []string
	for _, b := range wtest.RolesOf(root, semantics.RoleButton) {
		labels = append(labels, b.Label)
	}
	if slices.Contains(labels, "Close a") || !slices.Contains(labels, "Close b") {
		t.Errorf("a permanent tab has no close button, got %v", labels)
	}
	at := wtest.Center(h.tabs(t, 0)[0].Rect)
	h.pointer(core.PointerDown, at, input.ButtonMiddle)
	h.pointer(core.PointerUp, at, input.ButtonMiddle)
	h.Key(input.KeyW, input.ModControl)
	if got := m.titles(); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("neither the middle button nor Ctrl+W close it, got %v", got)
	}
	h.Key(input.KeyRight, input.ModControl|input.ModShift)
	if got := m.titles(); !slices.Equal(got, []string{"b", "a"}) {
		t.Errorf("it still moves, got %v", got)
	}
}
