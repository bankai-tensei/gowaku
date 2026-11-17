package editor

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

// openFindIn opens the find bar of the editor of v and types query into it.
func (h *splitHarness) openFindIn(v *View, query string) {
	h.focus(v)
	h.key(input.KeyF, input.ModControl)
	h.tree.DispatchText(query)
	h.frame()
}

func TestViews_FindBarsShowTheSharedFind(t *testing.T) {
	var ctl Controller
	var left, right View
	ctl.SetText("alpha beta\nbeta gamma\nalpha")
	h := newSplit(t, &ctl, &left, &right, Editor{})
	h.openFindIn(&left, "alpha")
	h.openFindIn(&right, "beta")
	ls := left.r.state
	if !ls.finding {
		t.Fatal("the left bar closed")
	}
	if ls.query != "beta" {
		t.Errorf("the left bar shows %q, the find is %q", ls.query, ctl.find.query)
	}
	// Replace All in the left bar replaces what it shows.
	ls.replacement = "X"
	ls.activate()
	ctl.ReplaceAll(ls.replacement)
	h.frame()
	if got, want := ctl.Text(), "alpha X\nX gamma\nalpha"; got != want {
		t.Errorf("text %q, want %q", got, want)
	}

	// An option and a bad pattern from one bar show in the other.
	rs := right.r.state
	rs.query, rs.options.Regexp = "(", true
	rs.search()
	h.frame()
	if !ls.options.Regexp || ls.query != "(" || !ls.badQuery {
		t.Errorf("left bar: query %q options %+v bad %v", ls.query, ls.options, ls.badQuery)
	}

	// Closing one bar ends the find; the other bar shows no query, so that it
	// never acts on one it does not show.
	rs.query, rs.options.Regexp = "alpha", false
	rs.search()
	h.frame()
	rs.closeFind()
	h.frame()
	if ls.query != "" || ls.badQuery {
		t.Errorf("after the right bar closed, the left shows %q (bad %v)", ls.query, ls.badQuery)
	}
	// The closed bar remembers its query for the next Ctrl+F.
	h.focus(&right)
	h.key(input.KeyF, input.ModControl)
	if rs.query != "alpha" || len(ctl.Matches()) != 2 || ls.query != "alpha" {
		t.Errorf("reopened with %q, %d matches, left shows %q", rs.query, len(ctl.Matches()), ls.query)
	}
}

// swapHost shows two editors of one controller side by side, and swaps their
// views when swapped is set. With keyed, the left editor has that key.
type swapHost struct {
	core.StatefulBase
	ctl     *Controller
	a, b    *View
	swapped *bool
	keyed   *int
	self    **swapHostState
}

type swapHostState struct{ core.StateBase }

func (swapHost) CreateState() core.State { return &swapHostState{} }

func (s *swapHostState) Build(core.BuildContext) core.Widget {
	w := s.Widget().(swapHost)
	*w.self = s
	l, r := w.a, w.b
	if *w.swapped {
		l, r = r, l
	}
	var left core.Widget = basic.Expanded{Child: Editor{Controller: w.ctl, View: l}}
	if w.keyed != nil {
		left = core.Keyed(*w.keyed, left)
	}
	return basic.Row{Cross: layout.CrossStretch, Children: []core.Widget{
		left,
		basic.Expanded{Child: Editor{Controller: w.ctl, View: r}},
	}}
}

func TestView_SwappingViewsKeepsBothPlaces(t *testing.T) {
	var ctl Controller
	var a, b View
	ctl.SetText(numbered(500))
	swapped := false
	var host *swapHostState
	tree := wtest.TextTree(t, swapHost{ctl: &ctl, a: &a, b: &b, swapped: &swapped, self: &host})
	size := geom.Size{W: 800, H: 300}
	testutil.RenderTree(tree, size)
	a.SetScroll(ScrollPosition{Line: 100, Offset: 3})
	b.SetScroll(ScrollPosition{Line: 300, Offset: 5})
	testutil.RenderTree(tree, size)
	host.SetState(func() { swapped = true })
	testutil.RenderTree(tree, size)
	if got := a.Scroll(); got != (ScrollPosition{Line: 100, Offset: 3}) {
		t.Errorf("a came to the right at %+v", got)
	}
	if got := b.Scroll(); got != (ScrollPosition{Line: 300, Offset: 5}) {
		t.Errorf("b came to the left at %+v", got)
	}
	if b.r == nil || b.r.first != 300 || a.r == nil || a.r.first != 100 {
		t.Error("the editors do not show the lines of their views")
	}
}

func TestView_MovingToANewKeyedEditorKeepsItsPlace(t *testing.T) {
	var ctl Controller
	var a, b View
	ctl.SetText(numbered(500))
	swapped := false
	key := 1
	var host *swapHostState
	tree := wtest.TextTree(t, swapHost{ctl: &ctl, a: &a, b: &b, swapped: &swapped, keyed: &key, self: &host})
	size := geom.Size{W: 800, H: 300}
	testutil.RenderTree(tree, size)
	a.SetScroll(ScrollPosition{Line: 200, Offset: 4})
	testutil.RenderTree(tree, size)
	// A new key makes a new editor for the left view.
	host.SetState(func() { key = 2 })
	testutil.RenderTree(tree, size)
	if got := a.Scroll(); got != (ScrollPosition{Line: 200, Offset: 4}) {
		t.Errorf("a in the new editor at %+v", got)
	}
}
