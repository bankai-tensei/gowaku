package field

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

// cursorHost is a FakeHost that remembers the cursor.
type cursorHost struct {
	wtest.FakeHost
	cursor input.Cursor
}

func (h *cursorHost) SetCursor(c input.Cursor) { h.cursor = c }

// areaHarness drives a SelectionArea through a real tree.
type areaHarness struct {
	t     *testing.T
	tree  *core.Tree
	host  *cursorHost
	swap  func(core.Widget)
	size  geom.Size
	clock uint32
}

func newArea(t *testing.T, w core.Widget) *areaHarness {
	t.Helper()
	tree, swap := wtest.SwapTree(t, w)
	h := &areaHarness{t: t, tree: tree, host: &cursorHost{}, swap: swap, size: wtest.Screen, clock: 1000}
	tree.SetHost(h.host)
	h.frame()
	return h
}

// column is a SelectionArea over texts, at the top left of the window.
func column(children ...core.Widget) core.Widget {
	return basic.Align{Alignment: layout.AlignTopLeft, Child: SelectionArea{Child: basic.Column{
		Size: layout.MainMin, Cross: layout.CrossStart, Spacing: 8, Children: children,
	}}}
}

func (h *areaHarness) frame() { testutil.RenderTree(h.tree, h.size) }

// paragraphs returns the texts of the window in the order they are painted.
func (h *areaHarness) paragraphs() []*basic.RenderParagraph {
	var out []*basic.RenderParagraph
	var walk func(ro core.RenderObject)
	walk = func(ro core.RenderObject) {
		if p, ok := ro.(*basic.RenderParagraph); ok {
			out = append(out, p)
		}
		for i := range ro.ChildCount() {
			walk(ro.ChildAt(i))
		}
	}
	walk(h.tree.RootRenderObject())
	return out
}

// at returns the middle of the caret at off of p, in the window.
func at(p *basic.RenderParagraph, off int) geom.Offset {
	c := p.TextLayout().CaretRect(off)
	return core.LocalToGlobal(p, geom.Offset{X: c.X + c.W/2, Y: c.Y + c.H/2})
}

func (h *areaHarness) pointer(kind core.PointerKind, pos geom.Offset, button input.Button, mods input.Modifiers) {
	h.tree.DispatchPointer(core.PointerEvent{Kind: kind, Pos: pos, Button: button, Modifiers: mods, Time: h.clock})
}

// click presses and releases at pos; the clock moves on so far that clicks do
// not count as a double click unless quick is set.
func (h *areaHarness) click(pos geom.Offset, mods input.Modifiers, quick bool) {
	if quick {
		h.clock += 100
	} else {
		h.clock += 2000
	}
	h.pointer(core.PointerMove, pos, input.ButtonPrimary, 0)
	h.pointer(core.PointerDown, pos, input.ButtonPrimary, mods)
	h.pointer(core.PointerUp, pos, input.ButtonPrimary, mods)
	h.frame()
}

func (h *areaHarness) drag(from, to geom.Offset) {
	h.clock += 2000
	h.pointer(core.PointerDown, from, input.ButtonPrimary, 0)
	h.pointer(core.PointerMove, to, input.ButtonPrimary, 0)
	h.pointer(core.PointerUp, to, input.ButtonPrimary, 0)
	h.frame()
}

func (h *areaHarness) key(k input.KeyCode, mods input.Modifiers) {
	h.tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: mods})
	h.frame()
}

// copied presses Ctrl+C and returns what reached the clipboard.
func (h *areaHarness) copied() string {
	h.host.Clipboard = ""
	h.key(input.KeyC, input.ModControl)
	return h.host.Clipboard
}

func (h *areaHarness) wantCopy(want string) {
	h.t.Helper()
	if got := h.copied(); got != want {
		h.t.Errorf("copied %q, want %q", got, want)
	}
}

func wantSel(t *testing.T, p *basic.RenderParagraph, start, end int) {
	t.Helper()
	if a, b := p.Selection(); a != start || b != end {
		t.Errorf("%q selects %d-%d, want %d-%d", p.Text(), a, b, start, end)
	}
}

// selectionRects returns the rectangles painted in the color of the
// selection.
func (h *areaHarness) selectionRects() []geom.Rect {
	var out []geom.Rect
	for _, op := range wtest.Rrects(h.tree, h.size) {
		if geom.Abs32(op.Color.A-selectionOpacity) < 1e-3 {
			out = append(out, op.RRect.Rect)
		}
	}
	return out
}

func TestSelectionArea_DragAcrossParagraphs(t *testing.T) {
	h := newArea(t, column(
		basic.Text{Content: "alpha beta"},
		basic.Text{Content: "gamma"},
		basic.Text{Content: "delta epsilon"},
	))
	ps := h.paragraphs()
	h.drag(at(ps[0], 6), at(ps[2], 5))
	wantSel(t, ps[0], 6, 10)
	wantSel(t, ps[1], 0, 5)
	wantSel(t, ps[2], 0, 5)
	h.wantCopy("beta\ngamma\ndelta")
	if len(h.selectionRects()) != 3 {
		t.Errorf("painted %d selection rectangles, want one per paragraph", len(h.selectionRects()))
	}

	// Backwards gives the same text.
	h.drag(at(ps[2], 5), at(ps[0], 6))
	h.wantCopy("beta\ngamma\ndelta")

	// Below the last text, the drag reaches its end; above the first, its
	// start.
	last := ps[2]
	below := core.LocalToGlobal(last, geom.Offset{X: 2, Y: last.Size().H + 20})
	h.drag(at(ps[1], 2), below)
	h.wantCopy("mma\ndelta epsilon")
	h.drag(at(ps[1], 2), geom.Offset{X: 300, Y: -5})
	h.wantCopy("alpha beta\nga")
}

func TestSelectionArea_WordLineAndShiftClick(t *testing.T) {
	h := newArea(t, column(
		basic.Text{Content: "one two\nthree four"},
		basic.Text{Content: "five six"},
	))
	ps := h.paragraphs()
	pos := at(ps[0], 10)
	h.click(pos, 0, false)
	h.click(pos, 0, true)
	h.wantCopy("three")
	h.click(pos, 0, false)
	h.click(pos, 0, true)
	h.click(pos, 0, true)
	h.wantCopy("three four")

	h.click(at(ps[0], 4), 0, false)
	h.click(at(ps[1], 4), input.ModShift, false)
	h.wantCopy("two\nthree four\nfive")
	// Shift+click again moves the free end and keeps the anchor.
	h.click(at(ps[0], 2), input.ModShift, false)
	h.wantCopy("e ")
}

func TestSelectionArea_SelectAllAndEscape(t *testing.T) {
	h := newArea(t, column(basic.Text{Content: "first"}, basic.Text{Content: "second"}))
	ps := h.paragraphs()
	h.click(at(ps[0], 1), 0, false)
	h.key(input.KeyA, input.ModControl)
	h.wantCopy("first\nsecond")
	h.key(input.KeyEscape, 0)
	if got := h.copied(); got != "" {
		t.Errorf("Escape must collapse the selection, copied %q", got)
	}
	// The anchor stays, so Shift+Left extends from the free end.
	h.key(input.KeyLeft, input.ModShift)
	h.wantCopy("d")
}

func TestSelectionArea_Keyboard(t *testing.T) {
	h := newArea(t, column(
		basic.Text{Content: "abc def"},
		basic.Text{Content: "ghi"},
	))
	ps := h.paragraphs()
	// Without an anchor the keys do nothing.
	h.key(input.KeyRight, input.ModShift)
	if h.copied() != "" {
		t.Fatal("Shift+arrow needs an anchor")
	}
	h.click(at(ps[0], 4), 0, false)
	h.key(input.KeyRight, input.ModShift)
	h.wantCopy("d")
	h.key(input.KeyEnd, input.ModShift)
	h.wantCopy("def")
	// At the end of a paragraph, the next character is the break.
	h.key(input.KeyRight, input.ModShift)
	h.wantCopy("def\n")
	h.key(input.KeyRight, input.ModShift)
	h.wantCopy("def\ng")
	h.key(input.KeyEnd, input.ModShift|input.ModControl)
	h.wantCopy("def\nghi")
	h.key(input.KeyHome, input.ModShift|input.ModControl)
	h.wantCopy("abc ")
	h.key(input.KeyRight, input.ModShift|input.ModControl)
	h.wantCopy("")
	// Down goes to the next paragraph; plain arrows are not ours.
	h.click(at(ps[0], 1), 0, false)
	h.key(input.KeyDown, input.ModShift)
	h.wantCopy("bc def\ng")
	if h.tree.Focused() == nil {
		t.Fatal("the area must keep the keyboard")
	}
	h.key(input.KeyLeft, 0)
	h.wantCopy("bc def\ng")
}

func TestSelectionArea_RubyCopiesTheBase(t *testing.T) {
	h := newArea(t, column(basic.RichText{Spans: []text.Span{
		{Text: "東京", Ruby: "とうきょう"},
		{Text: "の空"},
	}}))
	ps := h.paragraphs()
	if ps[0].Text() != "東京の空" {
		t.Fatalf("text %q", ps[0].Text())
	}
	h.click(at(ps[0], 0), 0, false)
	h.key(input.KeyA, input.ModControl)
	h.wantCopy("東京の空")
	// A drag that starts inside the group takes the whole group: the caret
	// cannot enter it.
	l := ps[0].TextLayout()
	group := l.CaretRect(len("東京"))
	inside := core.LocalToGlobal(ps[0], geom.Offset{X: group.X * 0.3, Y: group.Y + group.H/2})
	h.drag(inside, at(ps[0], len("東京の空")))
	if got := h.copied(); got != "東京の空" && got != "の空" {
		t.Errorf("copied %q, which splits the ruby group", got)
	}
}

func TestSelectionArea_Vertical(t *testing.T) {
	vertical := text.Style{Orientation: text.Vertical, Size: 16}
	h := newArea(t, column(basic.SizedBox{Height: 200, Child: basic.Text{Content: "縦書きの文章", Style: vertical}}))
	ps := h.paragraphs()
	h.drag(at(ps[0], 0), at(ps[0], len("縦書き")))
	h.wantCopy("縦書き")
	rects := h.selectionRects()
	if len(rects) != 1 {
		t.Fatalf("got %d rectangles", len(rects))
	}
	want := ps[0].TextLayout().SelectionRects(0, len("縦書き"), nil)[0].Translate(core.LocalToGlobal(ps[0], geom.Offset{}))
	if !wtest.NearRect(rects[0], want) || rects[0].H <= rects[0].W {
		t.Errorf("painted %v, want the column piece %v", rects[0], want)
	}
	// In vertical text Shift+Down goes on along the line.
	h.key(input.KeyDown, input.ModShift)
	h.wantCopy("縦書きの")
}

func TestSelectionArea_BidiRectangles(t *testing.T) {
	h := newArea(t, column(basic.Text{Content: "abc אבג def"}))
	ps := h.paragraphs()
	start, end := 2, len("abc אב")
	h.drag(at(ps[0], start), at(ps[0], end))
	got := h.selectionRects()
	want := ps[0].TextLayout().SelectionRects(start, end, nil)
	if len(got) != len(want) || len(want) < 2 {
		t.Fatalf("painted %v, want %v (a run reordered into pieces)", got, want)
	}
	origin := core.LocalToGlobal(ps[0], geom.Offset{})
	for i := range want {
		if !wtest.NearRect(got[i], want[i].Translate(origin)) {
			t.Errorf("rect %d = %v, want %v", i, got[i], want[i].Translate(origin))
		}
	}
	h.wantCopy("c אב")
}

func TestSelectionArea_RebuildWhileSelecting(t *testing.T) {
	build := func(second string) core.Widget {
		return column(basic.Text{Content: "keep this"}, basic.Text{Content: second})
	}
	h := newArea(t, build("and that"))
	ps := h.paragraphs()
	h.pointer(core.PointerDown, at(ps[0], 5), input.ButtonPrimary, 0)
	h.pointer(core.PointerMove, at(ps[1], 3), input.ButtonPrimary, 0)
	h.swap(build("and that"))
	h.frame()
	ps2 := h.paragraphs()
	if ps2[0] != ps[0] || ps2[1] != ps[1] {
		t.Fatal("the same widgets must keep their render objects")
	}
	h.pointer(core.PointerMove, at(ps[1], 8), input.ButtonPrimary, 0)
	h.pointer(core.PointerUp, at(ps[1], 8), input.ButtonPrimary, 0)
	h.frame()
	h.wantCopy("this\nand that")
}

func TestSelectionArea_TextChangesUnderTheSelection(t *testing.T) {
	h := newArea(t, column(basic.Text{Content: "hello world"}))
	ps := h.paragraphs()
	h.drag(at(ps[0], 6), at(ps[0], 11))
	h.wantCopy("world")
	// The offsets stay, kept inside the new text and on the start of a
	// character: 11 falls inside の and goes back to its start.
	h.swap(column(basic.Text{Content: "日本語のテキスト"}))
	h.frame()
	h.wantCopy("語")
	h.swap(column(basic.Text{Content: "hi"}))
	h.frame()
	h.wantCopy("")
	if a, b := h.paragraphs()[0].Selection(); b > len("日本語のテキスト") || a > b {
		t.Errorf("selection %d-%d", a, b)
	}
}

func TestSelectionArea_RemovedMidDrag(t *testing.T) {
	h := newArea(t, column(basic.Text{Content: "going away"}))
	ps := h.paragraphs()
	h.pointer(core.PointerDown, at(ps[0], 0), input.ButtonPrimary, 0)
	h.pointer(core.PointerMove, at(ps[0], 5), input.ButtonPrimary, 0)
	h.swap(basic.Text{Content: "plain"})
	h.frame()
	h.pointer(core.PointerMove, geom.Offset{X: 50, Y: 5}, input.ButtonPrimary, 0)
	h.pointer(core.PointerUp, geom.Offset{X: 50, Y: 5}, input.ButtonPrimary, 0)
	h.frame()
	for _, p := range h.paragraphs() {
		wantSel(t, p, 0, 0)
	}
	if h.copied() != "" {
		t.Error("nothing is selectable now")
	}
}

func TestSelectionArea_ZeroValueAndEmptyText(t *testing.T) {
	h := newArea(t, SelectionArea{})
	h.click(geom.Offset{X: 5, Y: 5}, 0, false)
	h.key(input.KeyA, input.ModControl)
	if h.copied() != "" || h.tree.Focused() != nil {
		t.Error("an empty area selects nothing and takes no focus")
	}
	h = newArea(t, column(basic.Text{}))
	h.click(geom.Offset{X: 1, Y: 5}, 0, false)
	h.click(geom.Offset{X: 1, Y: 5}, 0, true)
	h.key(input.KeyA, input.ModControl)
	h.key(input.KeyRight, input.ModShift)
	if got := h.copied(); got != "" {
		t.Errorf("copied %q from an empty text", got)
	}
}

func TestSelectionArea_ControlsInsideKeepWorking(t *testing.T) {
	pressed := 0
	h := newArea(t, column(
		basic.Text{Content: "before"},
		control.Button{Label: "push", OnPress: func() { pressed++ }},
		Input{Value: "field"},
		basic.Text{Content: "after"},
	))
	var label *basic.RenderParagraph
	for _, p := range h.paragraphs() {
		if p.Text() == "push" {
			label = p
		}
	}
	if label == nil {
		t.Fatal("no button label")
	}
	h.click(at(label, 2), 0, false)
	if pressed != 1 {
		t.Errorf("the button ran %d times", pressed)
	}
	if h.copied() != "" {
		t.Error("a press on the button must not select")
	}
	if h.host.cursor == input.CursorIBeam {
		t.Error("over the button the cursor is not the I-beam")
	}
	// The text of controls is not part of the selection.
	ps := h.paragraphs()
	h.click(at(ps[0], 1), 0, false)
	h.key(input.KeyA, input.ModControl)
	h.wantCopy("before\nafter")
	if h.host.cursor != input.CursorIBeam {
		t.Errorf("over text the cursor is %v", h.host.cursor)
	}
}

func TestSelectionArea_FocusPolicy(t *testing.T) {
	h := newArea(t, column(basic.Text{Content: "keep me"}))
	ps := h.paragraphs()
	h.drag(at(ps[0], 0), at(ps[0], 4))
	// The window going to the background keeps the selection.
	h.tree.SetWindowFocused(false)
	h.tree.SetWindowFocused(true)
	h.wantCopy("keep")
	// A press outside the area drops it.
	h.click(geom.Offset{X: 390, Y: 290}, 0, false)
	wantSel(t, ps[0], 0, 0)
	h.wantCopy("")
}

func TestSelectionArea_NestedAreaKeepsItsText(t *testing.T) {
	h := newArea(t, column(
		basic.Text{Content: "outer"},
		SelectionArea{Child: basic.Text{Content: "inner"}},
	))
	ps := h.paragraphs()
	h.click(at(ps[0], 1), 0, false)
	h.key(input.KeyA, input.ModControl)
	h.wantCopy("outer")
	h.click(at(ps[1], 1), 0, false)
	h.key(input.KeyA, input.ModControl)
	h.wantCopy("inner")
	wantSel(t, ps[0], 0, 0)
}

func TestSelectionArea_ContextMenu(t *testing.T) {
	h := newArea(t, overlay.Overlay{Child: column(basic.Text{Content: "menu text"})})
	ps := h.paragraphs()
	h.drag(at(ps[0], 0), at(ps[0], 4))
	pos := at(ps[0], 2)
	h.pointer(core.PointerDown, pos, input.ButtonSecondary, 0)
	h.pointer(core.PointerUp, pos, input.ButtonSecondary, 0)
	h.frame()
	wantSel(t, ps[0], 0, 4)
	root, _ := h.tree.SemanticsTree()
	n := wtest.ByLabel(t, root, "Copy")
	h.host.Clipboard = ""
	h.click(wtest.Center(n.Rect), 0, false)
	if h.host.Clipboard != "menu" {
		t.Errorf("Copy put %q on the clipboard", h.host.Clipboard)
	}
	wantSel(t, ps[0], 0, 4)

	// Select all, from the menu opened with the keyboard.
	h.key(input.KeyF10, input.ModShift)
	root, _ = h.tree.SemanticsTree()
	n = wtest.ByLabel(t, root, "Select all")
	h.click(wtest.Center(n.Rect), 0, false)
	h.wantCopy("menu text")
}

func TestSelectionArea_PaintDoesNotAllocate(t *testing.T) {
	h := newArea(t, column(basic.Text{Content: "one line"}, basic.Text{Content: "two\nlines"}))
	ps := h.paragraphs()
	h.drag(at(ps[0], 2), at(ps[1], 5))
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
