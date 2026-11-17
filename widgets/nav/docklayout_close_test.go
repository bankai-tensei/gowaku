package nav

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
)

// middle clicks the middle button at at.
func (h *dockHarness) middle(at geom.Offset) {
	h.time += 1000
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: at, Time: h.time})
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at, Button: input.ButtonMiddle, Time: h.time})
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: at, Button: input.ButtonMiddle, Time: h.time})
	h.render()
}

// tapLabel taps the node labeled label through assistive technology.
func (h *dockHarness) tapLabel(t *testing.T, label string) {
	t.Helper()
	root, _ := h.tree.SemanticsTree()
	h.tree.PerformSemanticsAction(wtest.ByLabel(t, root, label).ID, semantics.ActionTap, core.SemanticsArg{})
	h.render()
}

func TestDockLayout_CloseRequest(t *testing.T) {
	var asked []string
	allow := false
	setup := func(t *testing.T) *dockHarness {
		asked, allow = nil, false
		h := newDockHarness(t, twoGroups(), "files", "a", "b")
		h.ask = func(id string) bool {
			asked = append(asked, id)
			return allow
		}
		h.swap(h.widget())
		h.render()
		return h
	}
	held := func(t *testing.T, h *dockHarness, how string) {
		t.Helper()
		if !slices.Equal(asked, []string{"b"}) {
			t.Errorf("%s asks first, got %v", how, asked)
		}
		if got := h.shape(); got != "H([files*] | [a b*])" {
			t.Errorf("%s keeps a panel the request holds back, got %s", how, got)
		}
		if len(h.closed) != 0 || len(h.changes) != 0 || len(h.ctrl.closed) != 0 {
			t.Errorf("%s reports nothing, got %v and %d changes", how, h.closed, len(h.changes))
		}
		asked = nil
	}
	t.Run("閉じる前に聞く", func(t *testing.T) {
		h := setup(t)
		h.tapLabel(t, "Close b")
		held(t, h, "the close button")
		h.middle(wtest.Center(h.tab(t, "b").Rect))
		held(t, h, "the middle button")
		h.ctrl.current = h.ctrl.groupOf("b")
		h.key(input.KeyW, input.ModControl)
		held(t, h, "Ctrl+W")
		h.key(input.KeyF4, input.ModControl)
		held(t, h, "Ctrl+F4")
		if h.inits["b"] != 1 {
			t.Errorf("the panel keeps its state, got %v", h.inits)
		}

		// The app agrees later, from its dialog.
		h.ctrl.Close("b")
		h.render()
		if got := h.shape(); got != "H([files*] | [a*])" || len(asked) != 0 {
			t.Errorf("Close does not ask, got %s and %v", got, asked)
		}
		if len(h.closed) != 0 || len(h.changes) != 0 {
			t.Error("and reports nothing")
		}
	})
	t.Run("許す", func(t *testing.T) {
		h := setup(t)
		allow = true
		h.tapLabel(t, "Close b")
		if !slices.Equal(asked, []string{"b"}) || !slices.Equal(h.closed, []string{"b"}) || len(h.changes) != 1 {
			t.Errorf("got asked %v, closed %v, %d changes", asked, h.closed, len(h.changes))
		}
	})
	t.Run("閉じられないパネルには聞かない", func(t *testing.T) {
		h := setup(t)
		h.ctrl.current = h.ctrl.groupOf("files")
		h.key(input.KeyW, input.ModControl)
		if len(asked) != 0 {
			t.Errorf("got %v", asked)
		}
	})
	t.Run("フライアウト", func(t *testing.T) {
		h := setup(t)
		h.ctrl.Unpin("b", DockBottom)
		h.render()
		h.ctrl.Show("b")
		h.render()
		h.key(input.KeyW, input.ModControl)
		if !slices.Equal(asked, []string{"b"}) || h.ctrl.Flyout() != "b" || len(h.ctrl.hidden) != 1 {
			t.Fatalf("Ctrl+W in the flyout asks and keeps it, got %v %q", asked, h.ctrl.Flyout())
		}
		asked = nil
		h.tapLabel(t, "Close b")
		if !slices.Equal(asked, []string{"b"}) || len(h.ctrl.hidden) != 1 || len(h.closed) != 0 {
			t.Errorf("so does its close button, got %v", asked)
		}
	})
}
