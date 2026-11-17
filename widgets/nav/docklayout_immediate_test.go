package nav

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/semantics"
)

// addPanel gives the layout the panel id at the next build, without building,
// as an app does in the same handler as a call to its controller.
func (h *dockHarness) addPanel(id string) {
	h.ids = append(slices.Clone(h.ids), id)
	h.swap(h.widget())
}

func TestDockController_MoveRightAfterAdd(t *testing.T) {
	h := newDockHarness(t, twoGroups(), "files", "a", "b")
	h.addPanel("c")
	h.ctrl.Move("c", "files", DockRight)
	h.render()
	if got := h.shape(); got != "H([files*] | [c*] | [a b*])" {
		t.Errorf("a panel moves in the handler that adds it, got %s", got)
	}
	if h.inits["c"] != 1 {
		t.Errorf("and shows, got %v", h.inits)
	}

	// Into a group, and next to a panel added along with it.
	h.addPanel("d")
	h.addPanel("e")
	h.ctrl.Move("d", "a", DockCenter)
	h.ctrl.Move("e", "d", DockBottom)
	h.render()
	if got := h.shape(); got != "H([files*] | [c*] | V([a b d*] | [e*]))" {
		t.Errorf("got %s", got)
	}

	// A panel that the layout never gets leaves at the next build.
	h.ctrl.Move("ghost", "files", DockLeft)
	h.render()
	if got := h.shape(); got != "H([files*] | [c*] | V([a b d*] | [e*]))" {
		t.Errorf("a panel the layout does not have goes, got %s", got)
	}
}

func TestDockController_OtherEditsRightAfterAdd(t *testing.T) {
	t.Run("Activate", func(t *testing.T) {
		h := newDockHarness(t, twoGroups(), "files", "a", "b")
		h.ctrl.Activate("a")
		h.render()
		h.addPanel("c")
		h.ctrl.Activate("c")
		h.render()
		if got := h.shape(); got != "H([files*] | [a b c*])" {
			t.Errorf("a new panel shows in the current group, got %s", got)
		}
	})
	t.Run("Unpin", func(t *testing.T) {
		h := newDockHarness(t, twoGroups(), "files", "a", "b")
		h.addPanel("c")
		h.ctrl.Unpin("c", DockRight)
		h.render()
		if got := hiddenShape(h.ctrl); !slices.Equal(got, []string{"right:c"}) || h.shape() != "H([files*] | [a b*])" {
			t.Errorf("a new panel hides at once, got %v %s", got, h.shape())
		}
		h.ctrl.Show("c")
		h.render()
		if h.ctrl.Flyout() != "c" || h.inits["c"] != 1 {
			t.Errorf("and slides out, got %q %v", h.ctrl.Flyout(), h.inits)
		}
	})
	t.Run("Close", func(t *testing.T) {
		h := newDockHarness(t, twoGroups(), "files", "a", "b")
		h.addPanel("c")
		h.ctrl.Close("c")
		h.render()
		if got := h.shape(); got != "H([files*] | [a b*])" || !slices.Equal(h.ctrl.closed, []string{"c"}) {
			t.Errorf("a new panel can start closed, got %s %v", got, h.ctrl.closed)
		}
	})
	t.Run("Reset の後", func(t *testing.T) {
		h := newDockHarness(t, twoGroups(), "files", "a", "b")
		h.ctrl.Reset()
		h.ctrl.Move("a", "files", DockBottom)
		h.render()
		if got := h.shape(); got != "H(V([files*] | [a*]) | [b*])" {
			t.Errorf("a move right after Reset works, got %s", got)
		}
	})
}

func TestDockController_SetArrangementKeepsTheFlyout(t *testing.T) {
	h := newDockHarness(t, twoGroups(), "files", "a", "b")
	h.ctrl.Unpin("files", DockLeft)
	h.render()
	h.ctrl.Show("files")
	h.render()
	if h.ctrl.Flyout() != "files" || h.focused != "files" {
		t.Fatalf("setup: got %q %q", h.ctrl.Flyout(), h.focused)
	}
	strip := h.ctrl.groupOf("a").id

	// The app adds a tab in the same handler, by arrangement.
	a := h.ctrl.Arrangement()
	a.Root.Panels = append(a.Root.Panels, "c")
	a.Root.Active = "c"
	h.addPanel("c")
	h.ctrl.SetArrangement(a)
	h.render()
	if h.ctrl.Flyout() != "files" {
		t.Errorf("the panel out stays out, got %q", h.ctrl.Flyout())
	}
	if !slices.Contains(h.labels(semantics.RoleGroup), "files") || h.focused != "files" || h.tree.Focused() == nil {
		t.Errorf("shown and with the keyboard, got %v %q", h.labels(semantics.RoleGroup), h.focused)
	}
	if got := h.shape(); got != "[a b c*]" {
		t.Errorf("got %s", got)
	}
	if h.inits["files"] != 1 {
		t.Errorf("it keeps its state, got %v", h.inits)
	}
	if g := h.ctrl.groupOf("a"); g == nil || g.id != strip {
		t.Error("a group that stays keeps its strip")
	}

	// Moved to another edge, or docked, it follows the arrangement.
	a = h.ctrl.Arrangement()
	a.Hidden[0].Side = DockRight
	h.ctrl.SetArrangement(a)
	h.render()
	if h.ctrl.Flyout() != "" || !slices.Equal(hiddenShape(h.ctrl), []string{"right:files"}) {
		t.Errorf("a panel hidden at another edge goes in, got %q %v", h.ctrl.Flyout(), hiddenShape(h.ctrl))
	}
}

func TestDockController_SetArrangementKeepsWhatShows(t *testing.T) {
	h := newDockHarness(t, twoGroups(), "files", "a", "b")
	h.ctrl.Activate("a")
	h.render()
	files, editors := h.ctrl.groupOf("files").id, h.ctrl.groupOf("a").id
	a := h.ctrl.Arrangement()
	a.Root.Children[1].Panels = append(a.Root.Children[1].Panels, "c")
	a.Root.Children[1].Active = "c"
	h.addPanel("c")
	h.ctrl.SetArrangement(a)
	h.render()
	if got := h.shape(); got != "H([files*] | [a b c*])" {
		t.Errorf("got %s", got)
	}
	if !slices.Contains(h.labels(semantics.RoleGroup), "files") || h.inits["files"] != 1 {
		t.Errorf("the explorer stays shown, got %v %v", h.labels(semantics.RoleGroup), h.inits)
	}
	if h.ctrl.groupOf("files").id != files || h.ctrl.groupOf("a").id != editors {
		t.Error("the groups that stay keep their strips")
	}
	if h.ctrl.currentGroup() != h.ctrl.groupOf("a") {
		t.Error("and the current group stays")
	}
}
