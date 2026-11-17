package editor

import (
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
)

// editorNode returns the node of the editor in the accessibility tree.
func editorNode(t *testing.T, tree *core.Tree) *semantics.Node {
	t.Helper()
	root, _ := tree.SemanticsTree()
	var found *semantics.Node
	root.Walk(func(n *semantics.Node) bool {
		if n.Role == semantics.RoleTextField {
			found = n
		}
		return found == nil
	})
	if found == nil {
		t.Fatal("no text field in the tree")
	}
	return found
}

func TestTextEditor_SemanticsOfAModestDocument(t *testing.T) {
	var ctl Controller
	ctl.SetText("ab\n日本語\ncd")
	h := newEditor(t, &ctl, Editor{Label: "Notes"})
	ctl.SetSelection(TextRange{pos(1, len("日")), pos(2, 1)})
	h.frame()
	n := editorNode(t, h.tree)
	if n.Label != "Notes" || n.Value != "ab\n日本語\ncd" || n.Text.Text != n.Value {
		t.Errorf("label %q value %q", n.Label, n.Value)
	}
	if n.State&semantics.Multiline == 0 || n.State&semantics.ReadOnly != 0 {
		t.Errorf("state %v", n.State)
	}
	if n.Actions&semantics.ActionSetValue == 0 {
		t.Error("an editable editor takes ActionSetValue")
	}
	// UTF-16: "ab\n" is 3 units, 日 one more; "日本語\n" is 4, c one more.
	if n.Text.SelectionStart != 4 || n.Text.SelectionEnd != 8 {
		t.Errorf("selection %d-%d, want 4-8", n.Text.SelectionStart, n.Text.SelectionEnd)
	}
	if len(n.Text.LineStarts) != 2 || n.Text.LineStarts[0] != 3 || n.Text.LineStarts[1] != 7 {
		t.Errorf("line starts %v", n.Text.LineStarts)
	}
	if len(n.Text.LineRects) != 3 || n.Text.LineRects[1].Y <= n.Text.LineRects[0].Y {
		t.Errorf("line rects %v", n.Text.LineRects)
	}
	h.tree.PerformSemanticsAction(n.ID, semantics.ActionSetSelection, core.SemanticsArg{SelectionStart: 0, SelectionEnd: 2})
	if ctl.SelectedText() != "ab" {
		t.Errorf("SetSelection selected %q", ctl.SelectedText())
	}
	h.tree.PerformSemanticsAction(n.ID, semantics.ActionSetValue, core.SemanticsArg{Text: "new\ntext"})
	h.expect("new\ntext")
}

func TestTextEditor_SemanticsOfALargeDocumentIsAWindow(t *testing.T) {
	var ctl Controller
	ctl.SetText(numbered(100_000))
	h := newEditor(t, &ctl, Editor{ReadOnly: true})
	ctl.SetSelection(TextRange{pos(60_000, 0), pos(60_000, 4)})
	h.frame()
	n := editorNode(t, h.tree)
	lines := strings.Count(n.Value, "\n") + 1
	if lines > editorA11yWindow || !strings.Contains(n.Value, "line 60000\n") {
		t.Fatalf("the window holds %d lines", lines)
	}
	if n.State&semantics.ReadOnly == 0 || n.Actions&semantics.ActionSetValue != 0 {
		t.Errorf("a read-only editor: state %v actions %v", n.State, n.Actions)
	}
	got := n.Value[n.Text.SelectionStart:n.Text.SelectionEnd]
	if got != "line" || !strings.HasPrefix(n.Value[n.Text.SelectionStart:], "line 60000") {
		t.Errorf("the selection reads %q", got)
	}
	// The window is kept while nothing changes.
	again := editorNode(t, h.tree)
	if again.Value != n.Value || &h.view().a11y.lineStarts[0] != &again.Text.LineStarts[0] {
		t.Error("the window must be reused between frames")
	}
	h.tree.PerformSemanticsAction(n.ID, semantics.ActionSetSelection, core.SemanticsArg{SelectionStart: 0, SelectionEnd: 0})
	if c := ctl.Selection().End; c.Line != h.view().a11y.first {
		t.Errorf("selection at the start of the window went to %v", c)
	}
	h.Key(input.KeyHome, input.ModControl)
	if n := editorNode(t, h.tree); !strings.HasPrefix(n.Value, "line 0\n") {
		t.Errorf("the window must follow the caret: %.20q", n.Value)
	}
}
