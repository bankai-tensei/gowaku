package editor

import (
	"strconv"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/paint"
)

// editorHarness drives a Editor through a real tree.
type editorHarness struct {
	t    testing.TB
	tree *core.Tree
	host *wtest.FakeHost
	ctl  *Controller
	size geom.Size
	// clock is the time of the last press.
	clock uint32
}

func newEditor(t testing.TB, ctl *Controller, e Editor) *editorHarness {
	t.Helper()
	e.Controller = ctl
	h := &editorHarness{
		t: t, ctl: ctl, host: &wtest.FakeHost{}, size: geom.Size{W: 400, H: 300},
		tree: wtest.TextTree(t, e),
	}
	h.tree.SetHost(h.host)
	h.frame()
	h.view().RequestFocus()
	h.frame()
	return h
}

func (h *editorHarness) frame() string { return testutil.RenderTree(h.tree, h.size) }

func (h *editorHarness) view() *renderEditor { return h.ctl.primary.r }

func (h *editorHarness) typeText(s string) {
	h.tree.DispatchText(s)
	h.frame()
}

func (h *editorHarness) Key(k input.KeyCode, mods input.Modifiers) {
	h.tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: mods})
	h.frame()
}

func (h *editorHarness) expect(want string) {
	h.t.Helper()
	if got := h.ctl.Text(); got != want {
		h.t.Errorf("text %q, want %q", got, want)
	}
}

func (h *editorHarness) expectCaret(line, col int) {
	h.t.Helper()
	if got := h.ctl.Selection().End; got != (TextPosition{line, col}) {
		h.t.Errorf("caret %v, want %d:%d", got, line, col)
	}
}

// click presses at pos clicks times in a row, after a pause that ends any
// earlier row of clicks.
func (h *editorHarness) click(pos geom.Offset, clicks int, mods input.Modifiers) {
	h.clock += 5000
	for range clicks {
		h.clock += 10
		h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos, Button: input.ButtonPrimary, Modifiers: mods, Time: h.clock})
		h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos, Button: input.ButtonPrimary, Time: h.clock})
	}
	h.frame()
}

// numbered returns n lines "line 0" to "line n-1".
func numbered(n int) string {
	var sb strings.Builder
	for i := range n {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString("line ")
		sb.WriteString(strconv.Itoa(i))
	}
	return sb.String()
}

func TestTextEditor_ZeroValueTypes(t *testing.T) {
	tree := wtest.TextTree(t, Editor{})
	host := &wtest.FakeHost{}
	tree.SetHost(host)
	testutil.RenderTree(tree, geom.Size{W: 300, H: 200})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	tree.DispatchText("hi")
	tree.DispatchKey(core.KeyEvent{Key: input.KeyEnter})
	tree.DispatchText("there")
	testutil.RenderTree(tree, geom.Size{W: 300, H: 200})
	r, ok := tree.Focused().(*renderEditor)
	if !ok {
		t.Fatalf("focus on %T", tree.Focused())
	}
	if got := r.ctl.Text(); got != "hi\nthere" {
		t.Errorf("got %q", got)
	}
}

func TestTextEditor_TypingAcrossLines(t *testing.T) {
	var ctl Controller
	ctl.SetText("ab\ncd")
	changes := 0
	h := newEditor(t, &ctl, Editor{OnChange: func() { changes++ }})
	h.Key(input.KeyEnd, 0)
	h.Key(input.KeyRight, 0)
	h.expectCaret(1, 0)
	h.Key(input.KeyBackspace, 0)
	h.expect("abcd")
	h.expectCaret(0, 2)
	h.Key(input.KeyEnter, 0)
	h.typeText("X")
	h.expect("ab\nXcd")
	h.Key(input.KeyUp, 0)
	h.expectCaret(0, 1)
	h.Key(input.KeyEnd, input.ModControl)
	h.expectCaret(1, 3)
	h.Key(input.KeyHome, input.ModControl|input.ModShift)
	if got := ctl.SelectedText(); got != "ab\nXcd" {
		t.Errorf("selected %q", got)
	}
	h.Key(input.KeyDelete, 0)
	h.expect("")
	if changes != 4 {
		t.Errorf("%d changes, want 4", changes)
	}
}

// rowY returns a y in the middle of row k of the view.
func (h *editorHarness) rowY(k int) float32 {
	r := h.view()
	return editorPadY + (float32(k)+0.5)*r.lineHeight
}

func TestTextEditor_MouseSelects(t *testing.T) {
	var ctl Controller
	ctl.SetText("alpha beta\ngamma delta\nepsilon")
	h := newEditor(t, &ctl, Editor{})
	var x float32 = editorPadX + 2
	h.click(geom.Offset{X: x, Y: h.rowY(1)}, 1, 0)
	h.expectCaret(1, 0)
	h.click(geom.Offset{X: 300, Y: h.rowY(2)}, 1, input.ModShift)
	if got := ctl.SelectedText(); got != "gamma delta\nepsilon" {
		t.Errorf("shift-click selected %q", got)
	}
	h.click(geom.Offset{X: x + 2, Y: h.rowY(0)}, 2, 0)
	if got := ctl.SelectedText(); got != "alpha" {
		t.Errorf("double-click selected %q", got)
	}
	h.click(geom.Offset{X: x, Y: h.rowY(1)}, 3, 0)
	if got := ctl.SelectedText(); got != "gamma delta\n" {
		t.Errorf("triple-click selected %q", got)
	}
	// A drag from the start of line 0 to the end of line 2.
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: geom.Offset{X: x - 4, Y: h.rowY(0)}, Button: input.ButtonPrimary, Time: h.clock + 5000})
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 390, Y: h.rowY(2)}, Button: input.ButtonPrimary})
	h.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: geom.Offset{X: 390, Y: h.rowY(2)}, Button: input.ButtonPrimary})
	h.frame()
	if got := ctl.SelectedText(); got != ctl.Text() {
		t.Errorf("drag selected %q", got)
	}
}

func TestTextEditor_ClipboardKeepsLines(t *testing.T) {
	var ctl Controller
	ctl.SetText("one\ntwo\nthree")
	h := newEditor(t, &ctl, Editor{})
	ctl.SetSelection(TextRange{pos(0, 1), pos(1, 2)})
	h.Key(input.KeyX, input.ModControl)
	if h.host.Clipboard != "ne\ntw" {
		t.Errorf("cut %q", h.host.Clipboard)
	}
	h.expect("oo\nthree")
	h.Key(input.KeyEnd, input.ModControl)
	h.Key(input.KeyV, input.ModControl)
	h.expect("oo\nthreene\ntw")
	h.expectCaret(2, 2)
	h.Key(input.KeyZ, input.ModControl)
	h.Key(input.KeyZ, input.ModControl)
	h.expect("one\ntwo\nthree")
	h.Key(input.KeyA, input.ModControl)
	h.Key(input.KeyC, input.ModControl)
	if h.host.Clipboard != "one\ntwo\nthree" {
		t.Errorf("copy %q", h.host.Clipboard)
	}
}

func TestTextEditor_TabTypesOrMovesFocus(t *testing.T) {
	var ctl Controller
	h := newEditor(t, &ctl, Editor{})
	h.Key(input.KeyTab, 0)
	h.expect("\t")
	var other Controller
	h2 := newEditor(t, &other, Editor{TabMovesFocus: true})
	h2.Key(input.KeyTab, 0)
	if other.Text() != "" {
		t.Errorf("Tab typed %q", other.Text())
	}
}

func TestTextEditor_ReadOnly(t *testing.T) {
	var ctl Controller
	ctl.SetText("fixed")
	h := newEditor(t, &ctl, Editor{ReadOnly: true})
	if n := len(h.host.Ime); n == 0 || h.host.Ime[n-1] {
		t.Errorf("a read-only editor must keep the IME off: %v", h.host.Ime)
	}
	h.typeText("x")
	h.Key(input.KeyBackspace, 0)
	h.Key(input.KeyEnter, 0)
	h.Key(input.KeyV, input.ModControl)
	h.expect("fixed")
	h.Key(input.KeyRight, input.ModShift)
	h.Key(input.KeyC, input.ModControl)
	if h.host.Clipboard != "f" {
		t.Errorf("copy from a read-only editor gave %q", h.host.Clipboard)
	}
}

func TestTextEditor_PageDownScrolls(t *testing.T) {
	var ctl Controller
	ctl.SetText(numbered(200))
	h := newEditor(t, &ctl, Editor{})
	r := h.view()
	page := int(h.size.H/r.lineHeight) - 1
	h.Key(input.KeyPageDown, 0)
	h.expectCaret(page, 0)
	if r.top() <= 0 {
		t.Error("PageDown must scroll")
	}
	if c := ctl.Selection().End.Line; c < r.first || c > r.last {
		t.Errorf("the caret on line %d is out of view %d-%d", c, r.first, r.last)
	}
	h.Key(input.KeyPageUp, 0)
	h.expectCaret(0, 0)
}

func TestTextEditor_IMEComposition(t *testing.T) {
	var ctl Controller
	ctl.SetText("ab\ncd")
	h := newEditor(t, &ctl, Editor{})
	ctl.SetSelection(TextRange{pos(0, 1), pos(1, 1)})
	h.frame()
	rects := h.host.Rects
	h.tree.DispatchComposition(&input.Composition{Text: "にほん", Cursor: len("にほん"),
		Clauses: []input.Clause{{Start: 0, End: len("にほん"), Kind: input.ClauseTarget}}})
	h.frame()
	// The selection over two lines went first.
	h.expect("ad")
	r := h.view()
	if got := r.display(0); got != "aにほんd" {
		t.Errorf("display %q", got)
	}
	if h.host.Rects == rects {
		t.Error("the composition must move the candidate window")
	}
	h.tree.DispatchComposition(nil)
	h.typeText("日本")
	h.expect("a日本d")
	h.expectCaret(0, 1+len("日本"))
	// A commit on blur keeps the text.
	h.tree.DispatchComposition(&input.Composition{Text: "x", Cursor: 1})
	h.view().FocusChanged(false)
	h.frame()
	h.expect("a日本xd")
}

func TestTextEditor_WrapsLongLines(t *testing.T) {
	var ctl Controller
	long := strings.Repeat("word ", 60)
	ctl.SetText(long + "\nshort")
	h := newEditor(t, &ctl, Editor{Wrap: true})
	r := h.view()
	rows := int(r.rows.at(0))
	if rows < 3 {
		t.Fatalf("a line of %d bytes took %d rows", len(long), rows)
	}
	if r.rows.total() != rows+1 {
		t.Errorf("total rows %d, want %d", r.rows.total(), rows+1)
	}
	h.Key(input.KeyDown, 0)
	if c := ctl.Selection().End; c.Line != 0 || c.Column == 0 {
		t.Errorf("Down must move within the wrapped line, got %v", c)
	}
	for range rows {
		h.Key(input.KeyDown, 0)
	}
	if c := ctl.Selection().End; c.Line != 1 {
		t.Errorf("Down past the rows must reach line 1, got %v", c)
	}
	// A click on the second row lands inside the first line.
	h.click(geom.Offset{X: editorPadX + 1, Y: h.rowY(1)}, 1, 0)
	if c := ctl.Selection().End; c.Line != 0 || c.Column == 0 {
		t.Errorf("a click on row 1 gave %v", c)
	}
}

func TestTextEditor_LaysOutOnlyTheView(t *testing.T) {
	var ctl Controller
	ctl.SetText(numbered(100_000))
	calls := 0
	deco := func(_ int, _ string, dst []Decoration) []Decoration {
		calls++
		return dst
	}
	h := newEditor(t, &ctl, Editor{Decorations: deco, LineNumbers: true})
	r := h.view()
	if calls > 200 || len(r.cache.entries) > 200 {
		t.Errorf("the first frame decorated %d lines and caches %d", calls, len(r.cache.entries))
	}
	ctl.SetSelection(TextRange{pos(50_000, 2), pos(50_000, 2)})
	h.frame()
	if r.first > 50_000 || r.last < 50_000 {
		t.Fatalf("view %d-%d misses the caret", r.first, r.last)
	}
	if out := h.frame(); !strings.Contains(out, "glyphs=") {
		t.Error("nothing drawn")
	}
	calls = 0
	h.typeText("X")
	h.typeText("Y")
	if calls != 2 {
		t.Errorf("typing decorated %d lines, want 2", calls)
	}
	calls = 0
	h.Key(input.KeyEnter, 0)
	if calls != 2 {
		t.Errorf("a new line decorated %d lines, want 2", calls)
	}
	if ctl.Line(50_000) != "liXY" || ctl.Line(50_001) != "ne 50000" || ctl.LineCount() != 100_001 {
		t.Errorf("lines %q %q", ctl.Line(50_000), ctl.Line(50_001))
	}
	ctl.Redecorate()
	calls = 0
	h.frame()
	if calls == 0 || calls > 200 {
		t.Errorf("Redecorate decorated %d lines", calls)
	}
}

func TestTextEditor_ScrollKeepsTheTopLine(t *testing.T) {
	var ctl Controller
	var sb strings.Builder
	for i := range 2000 {
		if i > 0 {
			sb.WriteByte('\n')
		}
		// Lines whose rows the estimate gets wrong.
		sb.WriteString(strings.Repeat("ii ", (i%7)*20))
	}
	ctl.SetText(sb.String())
	h := newEditor(t, &ctl, Editor{Wrap: true})
	r := h.view()
	r.state.Jump(float64(r.rows.before(1000))*float64(r.lineHeight) + editorPadY)
	h.frame()
	first := r.first
	y := editorPadY + r.lineTop(first) - r.top()
	for range 3 {
		h.frame()
	}
	if r.first != first || editorPadY+r.lineTop(first)-r.top() != y {
		t.Errorf("the top line moved from %d at %g to %d at %g", first, y, r.first, editorPadY+r.lineTop(r.first)-r.top())
	}
	// Scrolling up a little brings lines above into view without a jump.
	r.state.Jump(r.top() - 3*float64(r.lineHeight))
	h.frame()
	if r.first >= first {
		t.Errorf("scrolling up showed line %d, was %d", r.first, first)
	}
}

func TestTextEditor_DecorationsAndGutter(t *testing.T) {
	var ctl Controller
	ctl.SetText("func main\nmisspeled word")
	red := geom.Color{R: 1, A: 1}
	deco := func(i int, _ string, dst []Decoration) []Decoration {
		if i == 0 {
			return append(dst, Decoration{Start: 0, End: 4, Color: red, Bold: true})
		}
		return append(dst, Decoration{Start: 0, End: 9, Underline: UnderlineSquiggle, UnderlineColor: red},
			Decoration{Start: 10, End: 14, Background: geom.Color{G: 1, A: 0.3}, Underline: UnderlineSolid})
	}
	var gutter []int
	h := newEditor(t, &ctl, Editor{
		Decorations: deco, LineNumbers: true, GutterWidth: 8,
		Gutter: func(_ *paint.Canvas, g GutterLine) { gutter = append(gutter, g.Line) },
	})
	r := h.view()
	colored := false
	for _, run := range r.cache.get(0).layout.Runs() {
		if run.Color == red {
			colored = true
		}
	}
	if !colored {
		t.Error("the color of a decoration must reach the glyphs")
	}
	if n := len(r.cache.get(1).squiggles); n != 1 {
		t.Errorf("%d squiggles", n)
	}
	gutter = gutter[:0]
	out := h.frame()
	if !strings.Contains(out, "path ") {
		t.Errorf("no squiggle drawn:\n%s", out)
	}
	if len(gutter) != 2 || gutter[0] != 0 || gutter[1] != 1 {
		t.Errorf("gutter lines %v", gutter)
	}
	if r.numbers == 0 || r.textLeft <= r.numbers+8 {
		t.Errorf("numbers %g, text at %g", r.numbers, r.textLeft)
	}
}
