package markdown

import (
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/field"
)

// clipHost is a Host that keeps the clipboard.
type clipHost struct{ clip string }

func (*clipHost) SetIMEEnabled(bool)          {}
func (*clipHost) SetIMECursorRect(geom.Rect)  {}
func (*clipHost) CancelComposition()          {}
func (*clipHost) SetCursor(input.Cursor)      {}
func (h *clipHost) ClipboardText() string     { return h.clip }
func (h *clipHost) SetClipboardText(s string) { h.clip = s }

// selectable is a document of every block, inside a SelectionArea in a
// Scroll.
const selectable = "# 見出し\n\n" +
	"段落の *強調* と [リンク](https://example.com) と `code` です。\n" +
	"続き\n\n" +
	"- one\n- two\n  - nested\n\n" +
	"1. first\n2. second\n\n" +
	"```go\nfunc main() {\n\tprintln(1)\n}\n```\n\n" +
	"> quoted\n\n" +
	"| A | B |\n|---|:-:|\n| 1 | 2 |\n| 3 |\n\n" +
	"- [x] task\n\n" +
	"Loose:\n\n- a\n\n- b\n"

// wantAll is what Select all and Copy give for selectable.
const wantAll = "見出し\n\n" +
	"段落の 強調 と リンク と code です。続き\n\n" +
	"one\ntwo\nnested\n\n" +
	"first\nsecond\n\n" +
	"func main() {\n    println(1)\n}\n\n" +
	"quoted\n\n" +
	"A\tB\n1\t2\n3\t\n\n" +
	"task\n\n" +
	"Loose:\n\na\n\nb"

type selRig struct {
	t    *testing.T
	tt   *gowakutest.Tester
	host *clipHost
	time uint32
}

func newSelRig(t *testing.T, w core.Widget, opts gowakutest.Options) *selRig {
	t.Helper()
	tt := gowakutest.New(t, w, opts)
	h := &clipHost{}
	tt.Tree().SetHost(h)
	return &selRig{t: t, tt: tt, host: h, time: 1000}
}

func (r *selRig) pointer(kind core.PointerKind, pos geom.Offset, mods input.Modifiers) {
	r.tt.Tree().DispatchPointer(core.PointerEvent{
		Kind: kind, Pos: pos, Button: input.ButtonPrimary, Modifiers: mods, Time: r.time,
	})
}

func (r *selRig) drag(from, to geom.Offset) {
	r.time += 2000
	r.pointer(core.PointerDown, from, 0)
	r.pointer(core.PointerMove, to, 0)
	r.pointer(core.PointerUp, to, 0)
	r.tt.Pump()
}

func (r *selRig) copied() string {
	r.host.clip = ""
	r.tt.Key(input.KeyC, input.ModControl)
	return r.host.clip
}

// caret returns the middle of the caret at off of p, in the window.
func caret(p *renderRich, off int) geom.Offset {
	c := p.laidOut.CaretRect(off).Translate(p.TextOrigin())
	return core.LocalToGlobal(p, geom.Offset{X: c.X + c.W/2, Y: c.Y + c.H/2})
}

func selectableDoc() core.Widget {
	return field.SelectionArea{Child: basic.Scroll{Child: Markdown{Source: selectable}}}
}

func TestSelection_SelectAllCopiesEveryBlock(t *testing.T) {
	r := newSelRig(t, selectableDoc(), gowakutest.Options{Height: 1600})
	p := findRich(r.tt, "続き")
	r.drag(caret(p, 0), caret(p, 3))
	r.tt.Key(input.KeyA, input.ModControl)
	if got := r.copied(); got != wantAll {
		t.Errorf("copied\n%q\nwant\n%q", got, wantAll)
	}
}

func TestSelection_DragAcrossHeadingListAndCode(t *testing.T) {
	r := newSelRig(t, selectableDoc(), gowakutest.Options{Height: 1600})
	head := findRich(r.tt, "見出し")
	code := findRich(r.tt, "println")
	r.drag(caret(head, len("見")), caret(code, len("func ma")))
	want := "出し\n\n" +
		"段落の 強調 と リンク と code です。続き\n\n" +
		"one\ntwo\nnested\n\n" +
		"first\nsecond\n\n" +
		"func ma"
	if got := r.copied(); got != want {
		t.Errorf("copied\n%q\nwant\n%q", got, want)
	}
	// The highlight is on the texts in between, also the link.
	link := findRich(r.tt, "リンク")
	if a, b := link.Selection(); a != 0 || b != len(link.Text()) {
		t.Errorf("the paragraph with the link has %d..%d selected, want all %d", a, b, len(link.Text()))
	}
	// The numbers of the ordered list are no part of it.
	if n := findRich(r.tt, "1."); n == nil || n.TextLayout() != nil {
		t.Error("the number of a list item can be selected")
	} else if a, b := n.Selection(); a != b {
		t.Errorf("the number of a list item is selected %d..%d", a, b)
	}
}

func TestSelection_TableCellsAndShiftArrows(t *testing.T) {
	r := newSelRig(t, field.SelectionArea{Child: basic.Scroll{Child: Markdown{
		Source: "| 左 | 右 |\n|:--|--:|\n| a | bb |\n| c | d |\n",
	}}}, gowakutest.Options{})
	a := findRich(r.tt, "a")
	d := findRich(r.tt, "d")
	r.drag(caret(a, 0), caret(d, 1))
	if got := r.copied(); got != "a\tbb\nc\td" {
		t.Errorf("copied %q", got)
	}
	// Right-aligned cells take the pointer where their text is drawn.
	if x := d.TextOrigin().X + d.laidOut.CaretRect(0).X; x <= 0 {
		t.Fatalf("a right-aligned cell has its text at %v", x)
	}
	// Shift+Left moves the free end back to the start of the cell, and then
	// to the end of the previous one.
	r.tt.Key(input.KeyLeft, input.ModShift)
	if got := r.copied(); got != "a\tbb\nc\t" {
		t.Errorf("after Shift+Left copied %q", got)
	}
	r.tt.Key(input.KeyLeft, input.ModShift)
	if got := r.copied(); got != "a\tbb\nc" {
		t.Errorf("after Shift+Left twice copied %q", got)
	}
}

func TestSelection_LinksStillWork(t *testing.T) {
	var tapped string
	r := newSelRig(t, field.SelectionArea{Child: Markdown{
		Source:    "before [link](/x) after",
		OnLinkTap: func(u string) { tapped = u },
	}}, gowakutest.Options{})
	r.tt.Tap("link")
	if tapped != "/x" {
		t.Errorf("a tap on a link inside a SelectionArea followed %q", tapped)
	}
	p := findRich(r.tt, "before")
	r.drag(caret(p, 0), caret(p, len(p.Text())))
	if got := r.copied(); got != "before link after" {
		t.Errorf("copied %q", got)
	}
}

func TestSelection_PaintDoesNotAllocate(t *testing.T) {
	r := newSelRig(t, selectableDoc(), gowakutest.Options{Height: 1600})
	var cv paint.Canvas
	measure := func() float64 {
		cv.Reset()
		r.tt.Tree().Paint(&cv)
		return testing.AllocsPerRun(10, func() {
			cv.Reset()
			r.tt.Tree().Paint(&cv)
		})
	}
	if n := measure(); n != 0 {
		t.Errorf("paint without a selection: %v allocs", n)
	}
	head := findRich(r.tt, "見出し")
	code := findRich(r.tt, "println")
	r.drag(caret(head, 0), caret(code, 4))
	if n := measure(); n != 0 {
		t.Errorf("paint with a selection: %v allocs", n)
	}
	if !strings.HasPrefix(r.copied(), "見出し") {
		t.Error("nothing was selected")
	}
}

func TestSelection_OutsideAnAreaHasNoState(t *testing.T) {
	tt := gowakutest.New(t, Markdown{Source: selectable}, gowakutest.Options{Height: 1600})
	var visit func(ro core.RenderObject)
	visit = func(ro core.RenderObject) {
		if p, ok := ro.(*renderRich); ok && p.selection != nil {
			t.Errorf("%q has a selection outside a SelectionArea", p.Text())
		}
		for i := range ro.ChildCount() {
			visit(ro.ChildAt(i))
		}
	}
	visit(tt.Tree().RootRenderObject())
}
