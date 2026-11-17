package field

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/text/aozora"
	"github.com/bankai-tensei/gowaku/theme"
)

// controlled is a field with a controller that logs its callbacks.
type controlled struct {
	*fieldHarness
	c   *InputController
	log []string
}

func newControlled(t *testing.T, f Input) *controlled {
	t.Helper()
	h := &controlled{c: &InputController{}}
	f.Controller = h.c
	f.OnChange = func(s string) { h.log = append(h.log, "change "+s) }
	f.OnSelectionChange = func(a, c int) { h.log = append(h.log, fmt.Sprintf("select %d %d", a, c)) }
	h.fieldHarness = newField(t, f)
	h.log = nil
	return h
}

// expectLog checks the callbacks since the last check.
func (h *controlled) expectLog(want ...string) {
	h.t.Helper()
	if !slices.Equal(h.log, want) {
		h.t.Errorf("callbacks %q, want %q", h.log, want)
	}
	h.log = nil
}

func (h *controlled) expectControlled(anchor, caret int) {
	h.t.Helper()
	if a, c := h.c.Selection(); a != anchor || c != caret {
		h.t.Errorf("controller selection %d→%d, want %d→%d", a, c, anchor, caret)
	}
}

func TestTextFieldController_SelectionKeepsItsDirection(t *testing.T) {
	h := newControlled(t, Input{Value: "hello world"})
	h.Key(input.KeyEnd, 0)
	h.Key(input.KeyLeft, input.ModShift)
	h.Key(input.KeyLeft, input.ModShift)
	// Selected backwards: the caret comes first.
	h.expectControlled(11, 9)
	h.expectSelection(9, 11)
	h.expectLog("select 11 11", "select 11 10", "select 11 9")

	h.c.SetSelection(5, 0)
	h.frame()
	h.expectControlled(5, 0)
	h.expectSelection(0, 5)
	h.expectLog("select 5 0")
	if got := h.c.SelectedText(); got != "hello" {
		t.Errorf("SelectedText = %q", got)
	}
	// Shift+Right moves the caret, the end that came first.
	h.Key(input.KeyRight, input.ModShift)
	h.expectControlled(5, 1)
	// Setting the same selection again tells nobody.
	h.c.SetSelection(5, 1)
	h.expectLog("select 5 1")
}

func TestTextFieldController_SetSelectionKeepsToTheText(t *testing.T) {
	h := newControlled(t, Input{Value: "あい"})
	h.c.SetSelection(-4, 4) // 4 is inside い
	h.expectControlled(0, 3)
	h.c.SetSelection(2, 100)
	h.expectControlled(0, 6)
}

func TestTextFieldController_EditsTellTheTextFirst(t *testing.T) {
	h := newControlled(t, Input{})
	h.typeText("a")
	h.typeText("b")
	h.expectLog("change a", "select 1 1", "change ab", "select 2 2")
	h.Key(input.KeyZ, input.ModControl)
	h.expectLog("change ", "select 0 0")
	// A click moves the caret too.
	h.typeText("one two")
	h.log = nil
	f := h.field()
	pos := geom.Offset{X: fieldPaddingX + f.laidOut.CaretRect(3).X, Y: 12}
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos, Time: 1000})
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos, Time: 1000})
	h.expectLog("select 3 3")
}

func TestTextFieldController_CompositionMovesNothingUntilCommitted(t *testing.T) {
	h := newControlled(t, Input{Value: "ab"})
	h.c.SetSelection(1, 1)
	h.log = nil
	h.tree.DispatchComposition(&input.Composition{Text: "かな", Cursor: 6})
	h.frame()
	h.expectLog()
	h.expectControlled(1, 1)
	h.tree.DispatchText("仮名")
	h.frame()
	h.expectLog("change a仮名b", "select 7 7")
	// A selection set from code keeps what the IME was converting.
	h.tree.DispatchComposition(&input.Composition{Text: "x", Cursor: 1})
	h.c.SetSelection(0, 0)
	h.expect("a仮名xb")
	h.expectControlled(0, 0)
}

func TestTextFieldController_ReplaceIsOneUndoStep(t *testing.T) {
	h := newControlled(t, Input{Value: "cat and cat"})
	h.c.SetSelection(9, 11) // the second "at"
	h.log = nil
	// Before the selection: it moves with its text.
	h.c.Replace(0, 3, "tiger")
	h.expect("tiger and cat")
	h.expectControlled(11, 13)
	h.expectLog("change tiger and cat", "select 11 13")
	// After it: it stays.
	h.c.Replace(13, 13, "!")
	h.expectControlled(11, 13)
	// Over it: a caret after the new text.
	h.c.Replace(10, 13, "cow")
	h.expect("tiger and cow!")
	h.expectControlled(13, 13)
	h.c.Undo()
	h.expect("tiger and cat!")
	h.c.Undo()
	h.c.Undo()
	h.expect("cat and cat")
	h.expectControlled(9, 11)
	if h.c.CanUndo() || !h.c.CanRedo() {
		t.Error("the three replacements were not three steps")
	}
	h.c.Redo()
	h.expect("tiger and cat")
}

func TestTextFieldController_ReplaceDoesNotMergeWithTyping(t *testing.T) {
	h := newControlled(t, Input{})
	h.typeText("ab")
	h.c.Replace(2, 2, "c")
	h.typeText("d")
	h.c.Undo()
	h.expect("abc")
	h.c.Undo()
	h.expect("ab")
}

func TestTextFieldController_SingleLineReplaceHasNoBreaks(t *testing.T) {
	h := newControlled(t, Input{})
	h.c.Replace(0, 0, "a\nb")
	h.expect("a b")
}

func TestTextFieldController_ClipboardAndInsert(t *testing.T) {
	h := newControlled(t, Input{Value: "copy me"})
	h.c.SetSelection(0, 4)
	h.c.Copy()
	if h.host.Clipboard != "copy" {
		t.Errorf("clipboard %q", h.host.Clipboard)
	}
	h.c.Cut()
	h.expect(" me")
	h.c.SelectAll()
	h.expectControlled(0, 3)
	h.c.Paste()
	h.expect("copy")
	h.c.Insert("!")
	h.expect("copy!")
	h.c.Undo()
	h.expect("copy")
}

func TestTextFieldController_Focus(t *testing.T) {
	var a, b InputController
	tree := wtest.TextTree(t, basic.Column{Children: []core.Widget{
		Input{Controller: &a, Value: "first"},
		Input{Controller: &b, Value: "second"},
	}})
	size := geom.Size{W: 400, H: 300}
	testutil.RenderTree(tree, size)
	if a.Focused() || b.Focused() {
		t.Fatal("a field has the keyboard before anyone asked")
	}
	b.Focus()
	if !b.Focused() || a.Focused() {
		t.Error("Focus did not give the second field the keyboard")
	}
	a.Focus()
	if !a.Focused() || b.Focused() {
		t.Error("Focus did not move the keyboard to the first field")
	}
}

func TestTextFieldController_FocusWaitsForTheField(t *testing.T) {
	var c InputController
	c.SetSelection(2, 4)
	c.Focus()
	tree := wtest.TextTree(t, basic.Align{Child: Input{Controller: &c, Value: "abcdef"}})
	testutil.RenderTree(tree, geom.Size{W: 400, H: 300})
	if !c.Focused() {
		t.Fatal("the field did not take the keyboard once laid out")
	}
	if a, b := c.Selection(); a != 2 || b != 4 {
		t.Errorf("selection %d→%d, want the one set before the field 2→4", a, b)
	}
}

func TestTextFieldController_KeepsTheSelectionForTheNextField(t *testing.T) {
	var c InputController
	var moves int
	field := func(key int) core.Widget {
		return core.Keyed(key, Input{
			Controller: &c, Value: "the same text", OnSelectionChange: func(int, int) { moves++ },
		})
	}
	var st *wtest.HolderState
	tree := wtest.TextTree(t, basic.Align{Child: wtest.Holder{First: field(1), State: &st}})
	size := geom.Size{W: 400, H: 300}
	testutil.RenderTree(tree, size)
	c.SetSelection(4, 8)
	moves = 0
	// A new field for the same text, as for a tab that comes back.
	st.SetState(func() { st.Child = field(2) })
	testutil.RenderTree(tree, size)
	if a, b := c.Selection(); a != 4 || b != 8 {
		t.Errorf("selection %d→%d after a new field, want 4→8", a, b)
	}
	if got, ok := tree.HitTest(geom.Offset{X: 20, Y: 10}).Path[0].(*RenderEditable); !ok || got.Anchor != 4 || got.Caret != 8 {
		t.Error("the new field did not take the selection of the controller")
	}
	if moves != 0 {
		t.Errorf("OnSelectionChange ran %d times while the field was built", moves)
	}
	// Without a field the controller still answers.
	st.SetState(func() { st.Child = basic.SizedBox{} })
	testutil.RenderTree(tree, size)
	if a, b := c.Selection(); a != 4 || b != 8 || c.Text() != "the same text" || c.Focused() {
		t.Errorf("without a field: selection %d→%d, text %q", a, b, c.Text())
	}
	c.Replace(0, 3, "a") // does nothing
	if c.Text() != "the same text" {
		t.Errorf("Replace without a field changed the text to %q", c.Text())
	}
}

func TestTextFieldController_NewValueKeepsTheSelectionQuietly(t *testing.T) {
	var c InputController
	var moves int
	field := func(v string) core.Widget {
		return Input{Controller: &c, Value: v, OnSelectionChange: func(int, int) { moves++ }}
	}
	var st *wtest.HolderState
	tree := wtest.TextTree(t, basic.Align{Child: wtest.Holder{First: field("a long text"), State: &st}})
	size := geom.Size{W: 400, H: 300}
	testutil.RenderTree(tree, size)
	c.SetSelection(2, 6)
	moves = 0
	st.SetState(func() { st.Child = field("short") })
	testutil.RenderTree(tree, size)
	if a, b := c.Selection(); a != 2 || b != 5 || c.Text() != "short" {
		t.Errorf("selection %d→%d in %q, want 2→5 in the new text", a, b, c.Text())
	}
	if moves != 0 {
		t.Error("OnSelectionChange ran for a Value the parent set")
	}
	if c.CanUndo() {
		t.Error("the new Value kept the history")
	}
}

func TestTextFieldController_SelectionInsideRubyShowsTheNotation(t *testing.T) {
	h := newControlled(t, Input{Value: rubyValue, Ruby: aozora.Marks})
	h.expectShown("前漢字後")
	h.c.SetSelection(markBase, markBase)
	h.frame()
	h.expectShown(rubyValue)
	h.c.SetSelection(markEnd, markEnd)
	h.frame()
	h.expectShown("前漢字後")
	// Replace edits the text as written.
	h.c.Replace(markBase, markBase+len("漢字"), "感じ")
	h.frame()
	h.expect("前｜感じ《かんじ》後")
}

func TestTextFieldController_Vertical(t *testing.T) {
	h := newControlled(t, Input{Value: "ab\ncd", Multiline: true, Style: text.Style{Size: 10, Orientation: text.Vertical}})
	h.c.SetSelection(4, 4)
	h.frame()
	// Right goes back to the first column, at the same height.
	h.Key(input.KeyRight, 0)
	h.expectControlled(1, 1)
	h.Key(input.KeyDown, input.ModShift)
	h.expectControlled(1, 2)
	h.expectLog("select 4 4", "select 1 1", "select 1 2")
}

func TestTextField_BareHasNoBox(t *testing.T) {
	h := newField(t, Input{Value: "plain", Bare: true})
	f := h.field()
	if f.insets != (geom.Insets{}) {
		t.Errorf("insets %v, want none", f.insets)
	}
	th := theme.Default().Light
	line := th.Text.Control.Size * th.Text.Control.LineHeight
	if got := f.Size().H; got != line {
		t.Errorf("height %v, want one line %v", got, line)
	}
	// No ground, border or ring: only the text and the caret.
	for l := range strings.Lines(h.frame()) {
		if !strings.Contains(l, "glyphs=") && !strings.Contains(l, "w=1.5 ") {
			t.Errorf("a bare field paints %s", l)
		}
	}
	padded := newField(t, Input{Value: "plain", Bare: true, Padding: geom.All(12)})
	if got := padded.field().Size().H; got != line+24 {
		t.Errorf("padded height %v, want %v", got, line+24)
	}
	boxed := newField(t, Input{Value: "plain", Padding: geom.Symmetric(4, 2)})
	if got := boxed.field().insets; got != geom.Symmetric(4, 2) {
		t.Errorf("a boxed field with Padding has insets %v", got)
	}
}

func TestTextField_CaretStaysAtTheEndWhileConverting(t *testing.T) {
	h := newControlled(t, Input{Value: "ab"})
	h.c.SetSelection(1, 1)
	// Typing: the caret follows the IME, here at the end of the kana.
	h.tree.DispatchComposition(&input.Composition{Text: "へんかん", Cursor: 12})
	h.frame()
	typing := h.field().displayCaret()
	if typing != 1+12 {
		t.Fatalf("caret at %d while typing, want %d", typing, 1+12)
	}
	// Space: the IME reports the start of the clause it converts. The caret
	// must not jump there and back on commit.
	h.tree.DispatchComposition(&input.Composition{Text: "変換", Cursor: 0, Clauses: []input.Clause{{Start: 0, End: 6, Kind: input.ClauseTarget}}})
	h.frame()
	if got := h.field().displayCaret(); got != 1+6 {
		t.Errorf("caret at %d while converting, want %d, the end of the composition", got, 1+6)
	}
	h.tree.DispatchText("変換")
	h.frame()
	h.expectControlled(7, 7)
}
