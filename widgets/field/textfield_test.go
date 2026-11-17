package field

import (
	"strings"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
)

// fieldHarness drives a Input through a real tree.
type fieldHarness struct {
	t    testing.TB
	tree *core.Tree
	host *wtest.FakeHost
	size geom.Size
}

func newField(t testing.TB, f Input) *fieldHarness {
	t.Helper()
	h := &fieldHarness{t: t, tree: wtest.TextTree(t, basic.Align{Child: f}), host: &wtest.FakeHost{}, size: geom.Size{W: 400, H: 300}}
	h.tree.SetHost(h.host)
	h.frame()
	h.tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	h.frame()
	return h
}

func (h *fieldHarness) frame() string { return testutil.RenderTree(h.tree, h.size) }

func (h *fieldHarness) field() *RenderEditable { return h.tree.Focused().(*RenderEditable) }

func (h *fieldHarness) typeText(s string) {
	h.tree.DispatchText(s)
	h.frame()
}

func (h *fieldHarness) Key(k input.KeyCode, mods input.Modifiers) {
	h.tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: mods})
	h.frame()
}

func (h *fieldHarness) expect(want string) {
	h.t.Helper()
	if got := h.field().Text(); got != want {
		h.t.Errorf("got %q, want %q", got, want)
	}
}

func (h *fieldHarness) expectSelection(start, end int) {
	h.t.Helper()
	if a, b := h.field().Selection(); a != start || b != end {
		h.t.Errorf("selection %d-%d, want %d-%d", a, b, start, end)
	}
}

func TestTextField_TypingAndCaret(t *testing.T) {
	var changes []string
	h := newField(t, Input{OnChange: func(s string) { changes = append(changes, s) }})
	if len(h.host.Ime) != 1 || !h.host.Ime[0] {
		t.Errorf("focus must turn the IME on: %v", h.host.Ime)
	}
	h.typeText("ab")
	h.typeText("c")
	h.expect("abc")
	h.expectSelection(3, 3)
	h.Key(input.KeyLeft, 0)
	h.typeText("X")
	h.expect("abXc")
	if len(changes) != 3 || changes[2] != "abXc" {
		t.Errorf("got %v", changes)
	}
}

func TestTextField_MovesByGrapheme(t *testing.T) {
	h := newField(t, Input{Value: "éx"})
	h.Key(input.KeyEnd, 0)
	h.Key(input.KeyLeft, 0)
	h.expectSelection(3, 3)
	h.Key(input.KeyLeft, 0)
	h.expectSelection(0, 0)
	h.Key(input.KeyDelete, 0)
	h.expect("x")
}

func TestTextField_BackspaceRemovesAWholeCluster(t *testing.T) {
	h := newField(t, Input{Value: "aé"})
	h.Key(input.KeyEnd, 0)
	h.Key(input.KeyBackspace, 0)
	h.expect("a")
}

func TestTextField_ShiftExtendsAndArrowsCollapse(t *testing.T) {
	h := newField(t, Input{Value: "abcd"})
	h.Key(input.KeyRight, input.ModShift)
	h.Key(input.KeyRight, input.ModShift)
	h.expectSelection(0, 2)
	h.Key(input.KeyRight, 0)
	h.expectSelection(2, 2)
	h.Key(input.KeyLeft, input.ModShift)
	h.Key(input.KeyLeft, 0)
	h.expectSelection(1, 1)
}

func TestTextField_WordKeys(t *testing.T) {
	h := newField(t, Input{Value: "one two"})
	h.Key(input.KeyEnd, 0)
	h.Key(input.KeyLeft, input.ModControl)
	h.expectSelection(4, 4)
	h.Key(input.KeyBackspace, input.ModControl)
	h.expect("two")
	h.Key(input.KeyDelete, input.ModControl)
	h.expect("")
}

func TestTextField_Clipboard(t *testing.T) {
	h := newField(t, Input{Value: "hello"})
	h.Key(input.KeyA, input.ModControl)
	h.expectSelection(0, 5)
	h.Key(input.KeyC, input.ModControl)
	if h.host.Clipboard != "hello" {
		t.Errorf("got %q", h.host.Clipboard)
	}
	h.Key(input.KeyX, input.ModControl)
	h.expect("")
	h.Key(input.KeyV, input.ModControl)
	h.Key(input.KeyV, input.ModControl)
	h.expect("hellohello")
}

func TestTextField_SingleLineFlattensPaste(t *testing.T) {
	h := newField(t, Input{})
	h.host.Clipboard = "a\r\nb\nc"
	h.Key(input.KeyV, input.ModControl)
	h.expect("a b c")
}

func TestTextField_EnterSubmitsOrBreaksLines(t *testing.T) {
	var submitted string
	h := newField(t, Input{Value: "x", OnSubmit: func(s string) { submitted = s }})
	h.Key(input.KeyEnter, 0)
	if submitted != "x" {
		t.Errorf("got %q", submitted)
	}
	h.expect("x")

	m := newField(t, Input{Multiline: true})
	m.typeText("a")
	m.Key(input.KeyEnter, 0)
	m.typeText("b")
	m.expect("a\nb")
	if got := m.field().Size().H; got != 50 { // two lines of 20, and the 10 that a line leaves in a field of 30
		t.Errorf("height %v", got)
	}
}

func TestTextField_UpAndDownKeepTheColumn(t *testing.T) {
	h := newField(t, Input{Value: "abcdef\nx\nabcdef", Multiline: true})
	h.Key(input.KeyRight, 0)
	h.Key(input.KeyRight, 0)
	h.Key(input.KeyRight, 0)
	h.Key(input.KeyDown, 0)
	h.expectSelection(8, 8) // the end of the short line
	h.Key(input.KeyDown, 0)
	h.expectSelection(12, 12) // back in the third column
}

func TestTextField_ValueFromOutside(t *testing.T) {
	var state *wtest.HolderState
	tree := wtest.TextTree(t, wtest.Holder{First: Input{Value: "old"}, State: &state})
	size := geom.Size{W: 300, H: 100}
	testutil.RenderTree(tree, size)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	f := tree.Focused().(*RenderEditable)
	f.Caret, f.Anchor = 3, 3
	tree.DispatchText("!")
	testutil.RenderTree(tree, size)

	// The same Value again leaves the edit alone.
	state.SetState(func() { state.Child = Input{Value: "old"} })
	testutil.RenderTree(tree, size)
	if f.Text() != "old!" {
		t.Errorf("got %q", f.Text())
	}
	// A new Value replaces it, and the caret stays inside.
	state.SetState(func() { state.Child = Input{Value: "x"} })
	testutil.RenderTree(tree, size)
	if f.Text() != "x" || f.Caret != 1 {
		t.Errorf("got %q, caret %d", f.Text(), f.Caret)
	}
}

func TestTextField_CompositionIsNotText(t *testing.T) {
	var changes int
	h := newField(t, Input{OnChange: func(string) { changes++ }})
	h.typeText("ab")
	h.Key(input.KeyLeft, 0)
	h.tree.DispatchComposition(&input.Composition{Text: "かな", Cursor: 6})
	h.frame()
	f := h.field()
	h.expect("ab")
	if got := f.display(); got != "aかなb" {
		t.Errorf("display %q", got)
	}
	if got := f.displayCaret(); got != 7 {
		t.Errorf("caret %d", got)
	}
	if changes != 1 {
		t.Errorf("composition must not count as an edit: %d", changes)
	}
	// What the platform sends on commit.
	h.tree.DispatchComposition(nil)
	h.typeText("仮名")
	h.expect("a仮名b")
	if got := f.display(); got != "a仮名b" {
		t.Errorf("display %q", got)
	}
}

func TestTextField_CompositionReplacesSelection(t *testing.T) {
	h := newField(t, Input{Value: "abc"})
	h.Key(input.KeyA, input.ModControl)
	h.tree.DispatchComposition(&input.Composition{Text: "あ", Cursor: 3})
	h.frame()
	if got := h.field().display(); got != "あ" {
		t.Errorf("display %q", got)
	}
	h.tree.DispatchComposition(nil)
	h.frame()
	if got := h.field().display(); got != "abc" {
		t.Errorf("a canceled composition must bring the text back: %q", got)
	}
}

func TestTextField_ClausesAreMarked(t *testing.T) {
	h := newField(t, Input{})
	base := h.frame()
	h.tree.DispatchComposition(&input.Composition{
		Text:   "abcd",
		Cursor: 2,
		Clauses: []input.Clause{
			{Start: 0, End: 2, Kind: input.ClauseTarget},
			{Start: 2, End: 4, Kind: input.ClauseConverted},
		},
	})
	got := h.frame()
	testutil.Golden(t, "composition", got)
	if got == base {
		t.Error("the composition must show")
	}
}

func TestTextField_IMEFollowsTheTargetClause(t *testing.T) {
	h := newField(t, Input{})
	if h.host.Rects == 0 {
		t.Fatal("focus must place the IME")
	}
	home := h.host.Rect
	h.tree.DispatchComposition(&input.Composition{
		Text:   "abcd",
		Cursor: 4,
		Clauses: []input.Clause{
			{Start: 0, End: 2, Kind: input.ClauseConverted},
			{Start: 2, End: 4, Kind: input.ClauseTarget},
		},
	})
	h.frame()
	f := h.field()
	want := f.laidOut.CaretRect(2).X + fieldPaddingX
	if h.host.Rect.X != want {
		t.Errorf("got %v, want the start of the target at %v", h.host.Rect.X, want)
	}
	if h.host.Rect.Y != home.Y || h.host.Rect.H == 0 {
		t.Errorf("got %v", h.host.Rect)
	}
	count := h.host.Rects
	h.frame()
	if h.host.Rects != count {
		t.Error("an unchanged position must not reach the OS")
	}
}

func TestTextField_BlurKeepsTheComposition(t *testing.T) {
	h := newField(t, Input{})
	f := h.field()
	h.tree.DispatchComposition(&input.Composition{Text: "かな", Cursor: 6})
	h.frame()
	h.tree.RequestFocus(nil)
	h.frame()
	if f.Text() != "かな" {
		t.Errorf("got %q", f.Text())
	}
	if h.host.Cancels != 1 {
		t.Errorf("the IME must drop its copy: %d", h.host.Cancels)
	}
	if n := len(h.host.Ime); n < 2 || h.host.Ime[n-1] {
		t.Errorf("blur must turn the IME off: %v", h.host.Ime)
	}
}

func TestTextField_LongLineScrollsToTheCaret(t *testing.T) {
	h := newField(t, Input{})
	h.typeText("the quick brown fox jumps over the lazy dog and keeps running far past the end of the field")
	f := h.field()
	if f.scroll <= 0 {
		t.Fatal("the end must scroll into view")
	}
	caret := f.laidOut.CaretRect(f.Caret).X - f.scroll
	if caret < 0 || caret > f.innerLength(f.Size()) {
		t.Errorf("caret at %v, field %v", caret, f.Size().W)
	}
	h.Key(input.KeyHome, 0)
	if f.scroll != 0 {
		t.Errorf("Home must scroll back, got %v", f.scroll)
	}
}

func TestTextField_ClickPlacesCaretAndDoubleClickSelectsWord(t *testing.T) {
	h := newField(t, Input{Value: "one two"})
	f := h.field()
	x := fieldPaddingX + f.laidOut.CaretRect(5).X
	pos := geom.Offset{X: x, Y: 12}
	press := func(time uint32) {
		h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos, Time: time})
		h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos, Time: time})
		h.frame()
	}
	press(1000)
	h.expectSelection(5, 5)
	press(1100)
	h.expectSelection(4, 7)
	press(1200)
	h.expectSelection(0, 7)
}

func TestTextField_DragSelects(t *testing.T) {
	h := newField(t, Input{Value: "abcdef"})
	f := h.field()
	at := func(i int) geom.Offset { return geom.Offset{X: fieldPaddingX + f.laidOut.CaretRect(i).X, Y: 12} }
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at(1)})
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: at(4)})
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: at(4)})
	h.expectSelection(1, 4)
}

func TestTextField_PlaceholderShowsOnlyWhenEmpty(t *testing.T) {
	h := newField(t, Input{Placeholder: "name"})
	withPlaceholder := h.frame()
	h.typeText("a")
	h.Key(input.KeyBackspace, 0)
	if got := h.frame(); got != withPlaceholder {
		t.Errorf("the placeholder must come back:\n%s\n%s", got, withPlaceholder)
	}
}

// typeRunes types s one character at a time, as a keyboard does.
func (h *fieldHarness) typeRunes(s string) {
	for _, r := range s {
		h.typeText(string(r))
	}
}

func TestTextField_UndoTakesBackAWord(t *testing.T) {
	h := newField(t, Input{})
	h.typeRunes("one two")
	h.Key(input.KeyZ, input.ModControl)
	h.expect("one ")
	h.Key(input.KeyZ, input.ModControl)
	h.expect("")
	h.Key(input.KeyZ, input.ModControl) // nothing left
	h.expect("")
}

func TestTextField_RedoRepeats(t *testing.T) {
	for _, tt := range []struct {
		name string
		key  input.KeyCode
		mods input.Modifiers
	}{
		{"Ctrl+Y", input.KeyY, input.ModControl},
		{"Ctrl+Shift+Z", input.KeyZ, input.ModControl | input.ModShift},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newField(t, Input{})
			h.typeRunes("one two")
			h.Key(input.KeyZ, input.ModControl)
			h.Key(input.KeyZ, input.ModControl)
			h.Key(tt.key, tt.mods)
			h.expect("one ")
			h.Key(tt.key, tt.mods)
			h.expect("one two")
			h.expectSelection(7, 7)
			h.Key(tt.key, tt.mods) // nothing left
			h.expect("one two")
		})
	}
}

func TestTextField_NewEditClearsRedo(t *testing.T) {
	h := newField(t, Input{})
	h.typeRunes("ab")
	h.Key(input.KeyZ, input.ModControl)
	h.typeText("c")
	h.Key(input.KeyY, input.ModControl)
	h.expect("c")
}

func TestTextField_CaretMoveEndsARun(t *testing.T) {
	h := newField(t, Input{})
	h.typeRunes("ab")
	h.Key(input.KeyLeft, 0)
	h.typeRunes("X")
	h.expect("aXb")
	h.Key(input.KeyZ, input.ModControl)
	h.expect("ab")
	h.expectSelection(1, 1)
}

func TestTextField_UndoRestoresTheSelection(t *testing.T) {
	h := newField(t, Input{Value: "hello"})
	h.Key(input.KeyA, input.ModControl)
	h.typeText("x")
	h.expect("x")
	h.Key(input.KeyZ, input.ModControl)
	h.expect("hello")
	h.expectSelection(0, 5)
}

func TestTextField_BackspaceRunIsOneStep(t *testing.T) {
	h := newField(t, Input{Value: "abcd"})
	h.Key(input.KeyEnd, 0)
	h.Key(input.KeyBackspace, 0)
	h.Key(input.KeyBackspace, 0)
	h.Key(input.KeyBackspace, 0)
	h.expect("a")
	h.Key(input.KeyZ, input.ModControl)
	h.expect("abcd")
	h.expectSelection(4, 4)
}

func TestTextField_DeleteRunIsOneStep(t *testing.T) {
	h := newField(t, Input{Value: "abcd"})
	h.Key(input.KeyDelete, 0)
	h.Key(input.KeyDelete, 0)
	h.expect("cd")
	h.Key(input.KeyZ, input.ModControl)
	h.expect("abcd")
	h.expectSelection(0, 0)
}

func TestTextField_TypingAndDeletingAreSeparateSteps(t *testing.T) {
	h := newField(t, Input{})
	h.typeRunes("abc")
	h.Key(input.KeyBackspace, 0)
	h.Key(input.KeyZ, input.ModControl)
	h.expect("abc")
	h.Key(input.KeyZ, input.ModControl)
	h.expect("")
}

func TestTextField_PasteAndCutAreOneStepEach(t *testing.T) {
	h := newField(t, Input{})
	h.host.Clipboard = "paste"
	h.typeRunes("a")
	h.Key(input.KeyV, input.ModControl)
	h.Key(input.KeyV, input.ModControl)
	h.expect("apastepaste")
	h.Key(input.KeyZ, input.ModControl)
	h.expect("apaste")
	h.Key(input.KeyA, input.ModControl)
	h.Key(input.KeyX, input.ModControl)
	h.expect("")
	h.Key(input.KeyZ, input.ModControl)
	h.expect("apaste")
}

func TestTextField_IMECommitIsOneStep(t *testing.T) {
	h := newField(t, Input{})
	h.tree.DispatchComposition(&input.Composition{Text: "にほんご", Cursor: 12})
	h.frame()
	h.tree.DispatchComposition(nil)
	h.typeText("日本語")
	h.tree.DispatchComposition(&input.Composition{Text: "を", Cursor: 3})
	h.frame()
	h.tree.DispatchComposition(nil)
	h.typeText("入力")
	h.expect("日本語入力")
	h.Key(input.KeyZ, input.ModControl)
	h.expect("日本語")
	h.Key(input.KeyZ, input.ModControl)
	h.expect("")
}

func TestTextField_UndoReportsTheChange(t *testing.T) {
	var last string
	h := newField(t, Input{OnChange: func(s string) { last = s }})
	h.typeRunes("ab")
	h.Key(input.KeyZ, input.ModControl)
	if last != "" {
		t.Errorf("got %q", last)
	}
	h.Key(input.KeyY, input.ModControl)
	if last != "ab" {
		t.Errorf("got %q", last)
	}
}

func TestTextField_ValueFromOutsideForgetsTheHistory(t *testing.T) {
	var state *wtest.HolderState
	tree := wtest.TextTree(t, wtest.Holder{First: Input{Value: "old"}, State: &state})
	size := geom.Size{W: 300, H: 100}
	testutil.RenderTree(tree, size)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	tree.DispatchText("x")
	testutil.RenderTree(tree, size)
	state.SetState(func() { state.Child = Input{Value: "new"} })
	testutil.RenderTree(tree, size)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyZ, Modifiers: input.ModControl})
	if got := tree.Focused().(*RenderEditable).Text(); got != "new" {
		t.Errorf("got %q", got)
	}
}

func TestTextField_ControlledValueKeepsTheHistory(t *testing.T) {
	var state *wtest.HolderState
	var build func(string) core.Widget
	build = func(v string) core.Widget {
		return Input{Value: v, OnChange: func(s string) {
			state.SetState(func() { state.Child = build(s) })
		}}
	}
	tree := wtest.TextTree(t, wtest.Holder{First: build(""), State: &state})
	size := geom.Size{W: 300, H: 100}
	testutil.RenderTree(tree, size)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	for _, s := range []string{"a", "b"} {
		tree.DispatchText(s)
		testutil.RenderTree(tree, size)
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyZ, Modifiers: input.ModControl})
	testutil.RenderTree(tree, size)
	if got := tree.Focused().(*RenderEditable).Text(); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestHistory_DropsTheOldestStep(t *testing.T) {
	var h kit.History
	for i := range kit.MaxHistory + 10 {
		h.Record(kit.Edit{At: i, Inserted: "x"})
	}
	if len(h.Undo) != kit.MaxHistory {
		t.Fatalf("got %d steps", len(h.Undo))
	}
	if h.Undo[0].At != 10 {
		t.Errorf("oldest step is %d", h.Undo[0].At)
	}
}

func TestTextField_FullBoxFollowsTheCaret(t *testing.T) {
	tree := wtest.TextTree(t, basic.Align{Child: basic.SizedBox{Width: 200, Height: 60, Child: Input{Multiline: true}}})
	host := &wtest.FakeHost{}
	tree.SetHost(host)
	size := geom.Size{W: 400, H: 300}
	testutil.RenderTree(tree, size)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	f := tree.Focused().(*RenderEditable)
	tree.DispatchText("1\n2\n3\n4\n5\n6")
	testutil.RenderTree(tree, size)
	if f.Size().H != 60 {
		t.Fatalf("the box must keep its size, got %v", f.Size().H)
	}
	if f.scrollY <= 0 {
		t.Fatal("the last line must scroll into view")
	}
	caret := f.laidOut.CaretRect(f.Caret)
	if bottom := caret.Y + caret.H - f.scrollY; bottom > f.view+0.01 {
		t.Errorf("caret ends at %v, view %v", bottom, f.view)
	}
	before := f.scrollY
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: geom.Offset{X: 20, Y: 20}, Scroll: geom.Offset{Y: 1}})
	if f.scrollY >= before {
		t.Errorf("the wheel must scroll up: %v", f.scrollY)
	}
}

func TestTextField_Semantics(t *testing.T) {
	h := newField(t, Input{Value: "v", Placeholder: "p"})
	n := h.tree.Semantics().Children[0]
	if n.Role != semantics.RoleTextField || n.Value != "v" || n.Label != "p" {
		t.Errorf("got %+v", n)
	}
}

func TestTextField_PaintDoesNotAllocate(t *testing.T) {
	h := newField(t, Input{Value: "steady text"})
	h.Key(input.KeyA, input.ModControl)
	var cv paint.Canvas
	cv.Reset()
	h.tree.Paint(&cv)
	allocs := testing.AllocsPerRun(10, func() {
		cv.Reset()
		h.tree.Paint(&cv)
	})
	if allocs != 0 {
		t.Errorf("got %v allocs", allocs)
	}
}

func TestButton_TapAndKeys(t *testing.T) {
	presses := 0
	tree := wtest.TextTree(t, basic.Align{Child: control.Button{Label: "OK", OnPress: func() { presses++ }}})
	size := geom.Size{W: 300, H: 100}
	testutil.RenderTree(tree, size)
	pos := geom.Offset{X: 10, Y: 10}
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos})
	pressed := testutil.RenderTree(tree, size)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos})
	wtest.RunUntilIdle(t, tree, size, nil)
	released := testutil.RenderTree(tree, size)
	if presses != 1 {
		t.Errorf("got %d presses", presses)
	}
	if pressed == released {
		t.Error("a press must show")
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	tree.DispatchKey(core.KeyEvent{Key: input.KeySpace})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyEnter})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyEnter, Repeat: true})
	if presses != 3 {
		t.Errorf("got %d presses", presses)
	}
}

func TestButton_ReleaseElsewhereCancels(t *testing.T) {
	presses := 0
	tree := wtest.TextTree(t, basic.Align{Child: control.Button{Label: "OK", OnPress: func() { presses++ }}})
	size := geom.Size{W: 300, H: 100}
	idle := testutil.RenderTree(tree, size)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: geom.Offset{X: 10, Y: 10}})
	testutil.RenderTree(tree, size)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: geom.Offset{X: 290, Y: 90}})
	tree.PointerLeft()
	wtest.RunUntilIdle(t, tree, size, nil)
	if got := testutil.RenderTree(tree, size); got != idle {
		t.Errorf("the look must return:\n%s\n%s", got, idle)
	}
	if presses != 0 {
		t.Errorf("got %d presses", presses)
	}
}

func TestButton_DisabledIgnoresEverything(t *testing.T) {
	tree := wtest.TextTree(t, basic.Align{Child: control.Button{Label: "OK"}})
	testutil.RenderTree(tree, geom.Size{W: 300, H: 100})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: geom.Offset{X: 10, Y: 10}})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	if tree.Focused() != nil {
		t.Error("a disabled button must not take focus")
	}
}

func TestButton_Semantics(t *testing.T) {
	tree := wtest.TextTree(t, control.Button{Label: "OK", OnPress: func() {}})
	testutil.RenderTree(tree, geom.Size{W: 300, H: 100})
	n := tree.Semantics().Children[0]
	if n.Role != semantics.RoleButton || n.Label != "OK" {
		t.Errorf("got %+v", n)
	}
}

func TestCheckbox_Toggles(t *testing.T) {
	var state *wtest.HolderState
	value := false
	var build func() core.Widget
	build = func() core.Widget {
		return basic.Align{Child: control.Checkbox{Value: value, Label: "同意する", OnChange: func(v bool) {
			state.SetState(func() {
				value = v
				state.Child = build()
			})
		}}}
	}
	tree := wtest.TextTree(t, wtest.Holder{First: build(), State: &state})
	size := geom.Size{W: 300, H: 100}
	before := testutil.RenderTree(tree, size)
	pos := geom.Offset{X: 10, Y: 10}
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos})
	testutil.RenderTree(tree, size)
	wtest.RunUntilIdle(t, tree, size, nil)
	after := testutil.RenderTree(tree, size)
	if !value {
		t.Fatal("a click must check the box")
	}
	if before == after {
		t.Error("the check must show")
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	tree.DispatchKey(core.KeyEvent{Key: input.KeySpace})
	testutil.RenderTree(tree, size)
	if value {
		t.Error("Space must uncheck the box")
	}
	n := tree.Semantics().Children[0]
	if n.Role != semantics.RoleCheckbox || n.Value != "unchecked" {
		t.Errorf("got %+v", n)
	}
}

func TestFocus_RowOfControlsInOrder(t *testing.T) {
	tree := wtest.TextTree(t, basic.Row{Cross: layout.CrossStart, Children: []core.Widget{
		basic.SizedBox{Width: 100, Child: Input{}},
		control.Button{Label: "a", OnPress: func() {}},
	}})
	testutil.RenderTree(tree, geom.Size{W: 400, H: 100})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	if _, ok := tree.Focused().(*RenderEditable); !ok {
		t.Fatalf("got %T", tree.Focused())
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	if _, ok := tree.Focused().(*layout.RenderFocus); !ok {
		t.Fatalf("got %T", tree.Focused())
	}
}

func TestCaret_BlinksWhileSleeping(t *testing.T) {
	h := newField(t, Input{})
	caret := func() bool { return strings.Contains(h.frame(), "w=1.5 ") }
	if !caret() {
		t.Fatal("a new focus must show the caret")
	}
	h.tree.Tick(time.Second)
	for h.tree.Ticking() {
		h.tree.Tick(time.Second + 200*time.Millisecond)
		h.frame()
	}
	wake, ok := h.tree.NextWake()
	if !ok {
		t.Fatal("the caret must plan its next phase")
	}
	h.tree.Tick(wake - time.Millisecond)
	if !caret() {
		t.Error("nothing must happen before the wake")
	}
	h.tree.Tick(wake)
	if caret() {
		t.Error("the caret must go off")
	}
	h.typeText("a")
	if !caret() {
		t.Error("typing must bring the caret back")
	}
	h.tree.RequestFocus(nil)
	h.frame()
	h.tree.Tick(wake + time.Second)
	h.frame()
	for h.tree.Ticking() {
		wake += time.Second
		h.tree.Tick(wake)
		h.frame()
	}
	if _, ok := h.tree.NextWake(); ok {
		t.Error("without the focus nothing must stay planned")
	}
}
