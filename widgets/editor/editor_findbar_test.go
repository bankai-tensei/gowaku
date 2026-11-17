package editor

import (
	"strconv"
	"strings"
	"testing"

	fieldw "github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

func TestTextEditor_FindBar(t *testing.T) {
	var ctl Controller
	ctl.SetText("cat dog\ncat bird\ndog")
	h := newEditor(t, &ctl, Editor{})
	ctl.SetSelection(TextRange{pos(0, 4), pos(0, 7)})
	h.Key(input.KeyF, input.ModControl)
	h.frame()
	state := h.view().state
	if !state.finding || state.query != "dog" {
		t.Fatalf("Ctrl+F must open the bar with the selection: %v %q", state.finding, state.query)
	}
	field, ok := h.tree.Focused().(*fieldw.RenderEditable)
	if !ok || field.Text() != "dog" {
		t.Fatalf("the query must have the keyboard, focus on %T", h.tree.Focused())
	}
	if len(ctl.Matches()) != 2 {
		t.Errorf("%d matches", len(ctl.Matches()))
	}
	// Typing replaces the selected query.
	h.typeText("cat")
	if state.query != "cat" || len(ctl.Matches()) != 2 {
		t.Errorf("query %q, %d matches", state.query, len(ctl.Matches()))
	}
	h.Key(input.KeyEnter, 0)
	if ctl.Selection().Ordered() != (TextRange{pos(1, 0), pos(1, 3)}) {
		t.Errorf("Enter selected %v", ctl.Selection())
	}
	if out := h.frame(); !strings.Contains(out, "2 found") && !strings.Contains(out, "glyphs=") {
		t.Error("the bar shows nothing")
	}
	h.Key(input.KeyF3, input.ModShift)
	if ctl.Selection().Ordered() != (TextRange{pos(0, 0), pos(0, 3)}) {
		t.Errorf("Shift+F3 selected %v", ctl.Selection())
	}
	h.Key(input.KeyEscape, 0)
	h.frame()
	if state.finding || len(ctl.Matches()) != 0 {
		t.Error("Escape must close the bar and end the search")
	}
	if h.tree.Focused() != h.view() {
		t.Errorf("the editor must get the keyboard back, not %T", h.tree.Focused())
	}
}

func TestTextEditor_ReplaceBar(t *testing.T) {
	var ctl Controller
	ctl.SetText("a-a-a")
	h := newEditor(t, &ctl, Editor{})
	h.Key(input.KeyH, input.ModControl)
	h.frame()
	state := h.view().state
	if !state.replacing {
		t.Fatal("Ctrl+H must open the replace row")
	}
	h.typeText("a")
	state.replacement = "b"
	h.Key(input.KeyEnter, 0)
	ctl.ReplaceMatch(state.replacement)
	h.frame()
	if got := ctl.Text(); got != "b-a-a" {
		t.Errorf("got %q", got)
	}
	if n := ctl.ReplaceAll("c"); n != 2 || ctl.Text() != "b-c-c" {
		t.Errorf("ReplaceAll %d: %q", n, ctl.Text())
	}
}

func TestTextEditor_ReadOnlyFindHasNoReplace(t *testing.T) {
	var ctl Controller
	ctl.SetText("text")
	h := newEditor(t, &ctl, Editor{ReadOnly: true})
	h.Key(input.KeyH, input.ModControl)
	h.frame()
	if s := h.view().state; !s.finding || s.replacing {
		t.Errorf("finding %v replacing %v", s.finding, s.replacing)
	}
}

func TestTextEditor_TypingRightAfterCtrlFGoesToTheQuery(t *testing.T) {
	var ctl Controller
	ctl.SetText("cat dog\ncat bird")
	h := newEditor(t, &ctl, Editor{})
	// No frame between the keys: the query has not got the keyboard yet.
	h.tree.DispatchKey(core.KeyEvent{Key: input.KeyF, Modifiers: input.ModControl})
	h.tree.DispatchText("ca")
	h.tree.DispatchText("t")
	h.frame()
	state := h.view().state
	if ctl.Text() != "cat dog\ncat bird" {
		t.Fatalf("the text typed after Ctrl+F went into the document: %q", ctl.Text())
	}
	if state.query != "cat" || len(ctl.Matches()) != 2 {
		t.Fatalf("query %q, %d matches", state.query, len(ctl.Matches()))
	}
	field, ok := h.tree.Focused().(*fieldw.RenderEditable)
	if !ok || field.Text() != "cat" {
		t.Fatalf("the query must have the keyboard, focus on %T", h.tree.Focused())
	}
	// The caret is after the text, so typing goes on instead of replacing it.
	h.typeText("s")
	if state.query != "cats" {
		t.Errorf("query %q, want cats", state.query)
	}
}

// barRect returns the bounds of the open find bar in the window.
func barRect(t *testing.T, h *editorHarness) geom.Rect {
	t.Helper()
	root, _ := h.tree.SemanticsTree()
	bars := wtest.RolesOf(root, semantics.RoleToolbar)
	if len(bars) != 1 {
		t.Fatalf("%d toolbars, want the find bar", len(bars))
	}
	return bars[0].Rect
}

// matchRect returns the bounds of the current match in the window.
func matchRect(h *editorHarness) geom.Rect {
	r := h.view()
	sel := h.ctl.Selection().Ordered()
	y := r.viewTop(sel.Start.Line)
	return core.LocalRectToGlobal(r, geom.Rect{X: r.textLeft, Y: y, W: 10, H: r.lineHeight})
}

func TestTextEditor_FindBarKeepsTheMatchInSight(t *testing.T) {
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "line " + strconv.Itoa(i)
	}
	for _, i := range []int{0, 1, 2, 60} {
		lines[i] = "needle " + strconv.Itoa(i)
	}
	for _, replace := range []bool{false, true} {
		var ctl Controller
		ctl.SetText(strings.Join(lines, "\n"))
		h := newEditor(t, &ctl, Editor{})
		key := input.KeyF
		if replace {
			key = input.KeyH
		}
		h.Key(key, input.ModControl)
		h.typeText("needle")
		bar := barRect(t, h)
		if replace && bar.H < 2*theme.ControlHeight {
			t.Fatalf("the bar has no replace row: %v", bar)
		}
		r := h.view()
		if viewTop := core.LocalRectToGlobal(r, geom.Rect{}).Y; viewTop+r.findRoom != bar.Y+bar.H {
			t.Errorf("replace %v: the view keeps %g free from %g, the bar reaches %g", replace, r.findRoom, viewTop, bar.Y+bar.H)
		}
		// The first lines come out from under the bar, and so does a match
		// that comes into view under it.
		for _, line := range []int{0, 1, 2, 60} {
			if line == 60 {
				// Line 60 shows right at the top, under the bar.
				r.state.Jump(float64(r.padTop()) + r.lineTop(60) - editorPadY)
				h.frame()
			}
			h.Key(input.KeyEnter, 0)
			if got := ctl.Selection().Ordered().Start.Line; got != line {
				t.Fatalf("replace %v: the current match is on line %d, want %d", replace, got, line)
			}
			if m := matchRect(h); m.Y < bar.Y+bar.H {
				t.Errorf("replace %v: the match on line %d at %v is under the bar %v", replace, line, m, bar)
			}
		}
		// Closing the bar gives the room back.
		h.Key(input.KeyEscape, 0)
		if r.findRoom != 0 || r.padTop() != editorPadY {
			t.Errorf("replace %v: the closed bar still takes %g", replace, r.findRoom)
		}
	}
}

func TestTextEditor_FindBarKeepsTheTextInPlaceScrolledDown(t *testing.T) {
	var ctl Controller
	ctl.SetText(strings.Repeat("line\n", 200))
	h := newEditor(t, &ctl, Editor{})
	r := h.view()
	r.state.Jump(float64(r.padTop()) + r.lineTop(100))
	h.frame()
	before := r.viewTop(100)
	r.state.openFind(false)
	h.frame()
	if r.findRoom == 0 {
		t.Fatal("the open bar takes no room")
	}
	if got := r.viewTop(100); got != before {
		t.Errorf("opening the bar moved line 100 from %g to %g", before, got)
	}
}
