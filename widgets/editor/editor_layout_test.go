package editor

import (
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
)

func TestTextEditor_UnboundedRoomTakesADefaultSize(t *testing.T) {
	var ctl Controller
	ctl.SetText("a\nb")
	tree := wtest.TextTree(t, basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Children: []core.Widget{Editor{Controller: &ctl}}})
	testutil.RenderTree(tree, geom.Size{W: 600, H: 800})
	if sz := ctl.primary.r.Size(); sz.W != 600 || sz.H <= 0 || sz.H >= 800 {
		t.Errorf("size %v", sz)
	}
}

func TestTextEditor_ScrollsSidewaysToTheCaret(t *testing.T) {
	var ctl Controller
	ctl.SetText(strings.Repeat("long ", 200) + "\nx")
	h := newEditor(t, &ctl, Editor{})
	h.Key(input.KeyEnd, 0)
	r := h.view()
	if r.hscroll() <= 0 {
		t.Fatal("End on a long line must scroll the text left")
	}
	caret := r.cache.get(0).layout.CaretRect(ctl.Selection().End.Column)
	if x := r.textLeft + caret.X - r.hscroll(); x < r.gutter || x > h.size.W {
		t.Errorf("the caret is at %g, out of view", x)
	}
	h.Key(input.KeyHome, 0)
	if r.hscroll() != 0 {
		t.Errorf("Home left the text scrolled by %g", r.hscroll())
	}
}

func TestTextEditor_FindBarCountFollowsEdits(t *testing.T) {
	var ctl Controller
	ctl.SetText("aa")
	h := newEditor(t, &ctl, Editor{})
	h.Key(input.KeyF, input.ModControl)
	h.typeText("a")
	if n := len(ctl.Matches()); n != 2 {
		t.Fatalf("%d matches", n)
	}
	h.view().RequestFocus()
	h.Key(input.KeyEnd, 0)
	h.typeText("a")
	if n := len(ctl.Matches()); n != 3 {
		t.Errorf("%d matches after typing", n)
	}
	if !strings.Contains(h.frame(), "glyphs=") {
		t.Error("nothing drawn")
	}
}
