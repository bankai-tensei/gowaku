package editor

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

// expectInView fails unless the editor of v shows line and the caret on it
// lies inside the view.
func expectInView(t *testing.T, v *View, line int) {
	t.Helper()
	r := v.r
	if r == nil {
		t.Fatal("no editor shows the view")
	}
	if got := v.Selection().End.Line; got != line {
		t.Errorf("the caret is on line %d, want %d", got, line)
	}
	if line < r.first || line > r.last {
		t.Errorf("lines %d to %d in view, want %d among them", r.first, r.last, line)
	}
	y := r.viewTop(line)
	if y < 0 || y+r.lineHeight > r.size.H {
		t.Errorf("the caret line is at %g of a view %g high", y, r.size.H)
	}
}

func TestView_SelectionBeforeShownScrollsOnFirstLayout(t *testing.T) {
	var ctl Controller
	var v View
	ctl.SetText(numbered(300))
	// Open a file and jump to line 34, before the editor is built.
	v.SetSelection(caretRange(pos(34, 2)))
	tree := wtest.TextTree(t, Editor{Controller: &ctl, View: &v})
	testutil.RenderTree(tree, geom.Size{W: 400, H: 300})
	expectInView(t, &v, 34)
	if got := v.Selection(); got != caretRange(pos(34, 2)) {
		t.Errorf("selection %v", got)
	}
}

func TestController_SelectionBeforeShownScrollsOnFirstLayout(t *testing.T) {
	var ctl Controller
	var v View
	ctl.SetText(numbered(300))
	ctl.SetSelection(caretRange(pos(120, 0)))
	tree := wtest.TextTree(t, Editor{Controller: &ctl, View: &v})
	testutil.RenderTree(tree, geom.Size{W: 400, H: 300})
	expectInView(t, &v, 120)
}

func TestView_SelectionWhileHiddenWinsOverTheKeptPlace(t *testing.T) {
	var docs [2]Controller
	var views [2]View
	docs[0].SetText(numbered(500))
	docs[1].SetText("short")
	pick := 0
	var host *tabHostState
	tree := wtest.TextTree(t, tabHost{pick: &pick, docs: &docs, views: &views, self: &host})
	size := geom.Size{W: 400, H: 300}
	testutil.RenderTree(tree, size)
	host.SetState(func() { pick = 1 })
	testutil.RenderTree(tree, size)
	// The first tab kept its place at the top; a jump made while it is
	// hidden shows when it comes back.
	views[0].SetSelection(caretRange(pos(400, 1)))
	host.SetState(func() { pick = 0 })
	testutil.RenderTree(tree, size)
	expectInView(t, &views[0], 400)

	// A SetScroll after the selection still wins over the caret.
	host.SetState(func() { pick = 1 })
	testutil.RenderTree(tree, size)
	views[0].SetSelection(caretRange(pos(10, 0)))
	views[0].SetScroll(ScrollPosition{Line: 200})
	host.SetState(func() { pick = 0 })
	testutil.RenderTree(tree, size)
	if r := views[0].r; r.first != 200 {
		t.Errorf("first line %d, want the 200 of SetScroll", r.first)
	}
}

func TestView_SelectionAfterTheTextLoadsScrolls(t *testing.T) {
	var ctl Controller
	var v View
	h := newEditor(t, &ctl, Editor{View: &v})
	// The file arrives after the editor is shown, and the caret goes to the
	// line the user asked for.
	ctl.SetText(numbered(300))
	v.SetSelection(caretRange(pos(34, 0)))
	h.frame()
	expectInView(t, &v, 34)
}

func TestView_FocusFromCode(t *testing.T) {
	var ctl Controller
	var v View
	ctl.SetText("hello")
	// Asked before the editor is built: it takes the keyboard when laid out.
	v.Focus()
	if v.Focused() {
		t.Error("a view that no editor shows reports the focus")
	}
	tree := wtest.TextTree(t, basicColumn(Editor{Controller: &ctl, View: &v}, Editor{Controller: &ctl}))
	tree.SetHost(&wtest.FakeHost{})
	size := geom.Size{W: 400, H: 600}
	testutil.RenderTree(tree, size)
	if !v.Focused() {
		t.Fatal("Focus before the build did not focus the editor")
	}
	// Move the focus to the other editor, then back from code.
	other := ctl.views[1].r
	other.RequestFocus()
	testutil.RenderTree(tree, size)
	if v.Focused() {
		t.Fatal("the other editor did not take the focus")
	}
	v.Focus()
	if !v.Focused() {
		t.Error("Focus on a shown editor did not focus it at once")
	}
	if ctl.primary != &v {
		t.Error("the focused view is not the primary view")
	}
}

// basicColumn stacks two editors, sharing the height.
func basicColumn(a, b Editor) core.Widget {
	return basic.Column{Cross: layout.CrossStretch, Children: []core.Widget{basic.Expanded{Child: a}, basic.Expanded{Child: b}}}
}
