package nav

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

// titlesOf returns the titles below ro, in order.
func titlesOf(ro core.RenderObject) []*renderTabTitle {
	var out []*renderTabTitle
	walkRender(ro, func(ro core.RenderObject) {
		if r, ok := ro.(*renderTabTitle); ok {
			out = append(out, r)
		}
	})
	return out
}

func TestTabBar_ItalicTitle(t *testing.T) {
	m := newTabModel("main.go", "main.go", "b")
	m.tabs[1].Italic = true
	h := newTabBarHarness(t, 800, false, m, nil)
	run := findRun(t, h.tree)
	titles := titlesOf(run)
	if len(titles) != 3 {
		t.Fatalf("got %d titles", len(titles))
	}
	upright, slanted := titles[0], titles[1]
	if upright.style.Italic || !slanted.style.Italic || titles[2].style.Italic {
		t.Fatal("only the italic tab has an italic title")
	}
	if upright.full.Size() != slanted.full.Size() {
		t.Errorf("an italic title measures as upright, got %v and %v", upright.full.Size(), slanted.full.Size())
	}
	ur, sr := upright.shown.Runs(), slanted.shown.Runs()
	if len(ur) == 0 || len(sr) == 0 || ur[0].Face == sr[0].Face {
		t.Error("the italic title draws with the slanted face")
	}
	if got := h.tabs(t, 0); got[1].Label != "main.go" {
		t.Errorf("a reader hears the title as it is, got %q", got[1].Label)
	}
	m.tabs[1].Italic = false
	m.changed()
	h.render()
	if titlesOf(run)[1].style.Italic {
		t.Error("the title stands up again")
	}
}

func TestTabBar_ItalicTitleIsCutWithAnEllipsis(t *testing.T) {
	long := "とても長いファイルの名前で、タブに入りきらないもの.txt"
	m := newTabModel(long, "b", "c", "d", "e", "f", "g", "h")
	m.tabs[0].Italic = true
	h := newTabBarHarness(t, 800, false, m, nil)
	title := titlesOf(findRun(t, h.tree).children[0])[0]
	if !title.style.Italic || title.shown == title.full {
		t.Fatal("the italic title must be cut")
	}
	if title.shown.Size().W > title.Size().W {
		t.Errorf("the cut title must fit, got %v in %v", title.shown.Size().W, title.Size().W)
	}
}

func TestTabBar_ItalicTabDraggedToAnotherBar(t *testing.T) {
	g := &TabGroup{}
	a, b := newTabModel("a1", "a2"), newTabModel("b1")
	a.tabs[1].Italic = true
	b.tabs[0].Key = 10
	a.group, b.group = g, g
	h := newTabBarHarness(t, 800, false, a, b)
	aTabs, bTabs := h.tabs(t, 0), h.tabs(t, 1)
	p := wtest.Center(aTabs[1].Rect)
	over := wtest.Center(bTabs[0].Rect)
	h.pointer(core.PointerMove, p, 0)
	h.pointer(core.PointerDown, p, 0)
	h.pointer(core.PointerMove, p.Add(geom.Offset{Y: 60}), 0)
	h.pointer(core.PointerMove, over, 0)
	var runs []*renderTabRun
	walkRender(h.tree.RootRenderObject(), func(ro core.RenderObject) {
		if r, ok := ro.(*renderTabRun); ok {
			runs = append(runs, r)
		}
	})
	bRun := runs[len(runs)-1]
	if len(bRun.children) != 2 {
		t.Fatalf("the other bar must show where the tab goes, got %d children", len(bRun.children))
	}
	if ghost := titlesOf(bRun.children[1]); len(ghost) != 1 || !ghost[0].style.Italic {
		t.Error("the tab that would drop is italic too")
	}
	h.pointer(core.PointerUp, over, 0)
	if i := slices.IndexFunc(b.tabs, func(it TabItem) bool { return it.Label == "a2" }); i < 0 || !b.tabs[i].Italic {
		t.Errorf("the item moves as it is, got %+v", b.tabs)
	}
}

func TestTabBar_ItalicTabInTheListOfAllTabs(t *testing.T) {
	titles := make([]string, 12)
	for i := range titles {
		titles[i] = "file.go"
	}
	titles[3] = "preview.md"
	m := newTabModel(titles...)
	m.tabs[3].Italic = true
	h := newTabBarHarness(t, 600, false, m, nil)
	root, _ := h.tree.SemanticsTree()
	more := wtest.ByLabel(t, root, "All tabs")
	h.tree.PerformSemanticsAction(more.ID, semantics.ActionTap, core.SemanticsArg{})
	h.render()
	root, _ = h.tree.SemanticsTree()
	item := wtest.ByLabel(t, root, "preview.md")
	h.tree.PerformSemanticsAction(item.ID, semantics.ActionTap, core.SemanticsArg{})
	h.render()
	if m.selected != 3 {
		t.Errorf("the menu lists it by its title, got %d", m.selected)
	}
}

func TestTabBar_ItalicLayoutPaintDoesNotAllocate(t *testing.T) {
	m := newTabModel("README.md", "main.go", "go.mod", "app.go", "tabbar.go", "docs.md")
	m.tabs[1].Italic = true
	m.tabs[4].Italic = true
	h := newTabBarHarness(t, 900, false, m, nil)
	run := findRun(t, h.tree)
	var cv paint.Canvas
	i := 0
	frame := func() {
		i++
		run.scroll = float32(i % 2)
		run.MarkNeedsLayout()
		h.tree.Layout(geom.Tight(h.size))
		cv.Reset()
		h.tree.Paint(&cv)
	}
	frame()
	if allocs := testing.AllocsPerRun(100, frame); allocs != 0 {
		t.Errorf("got %v allocs per frame", allocs)
	}
}

func TestDockLayout_ItalicPanel(t *testing.T) {
	h := newDockHarness(t, twoGroups(), "files", "a", "b")
	h.italic = map[string]bool{"b": true, "files": true}
	h.swap(h.widget())
	h.render()
	var slanted []string
	for _, r := range titlesOf(h.tree.RootRenderObject()) {
		if r.style.Italic {
			slanted = append(slanted, r.text)
		}
	}
	if !slices.Equal(slanted, []string{"files", "b"}) {
		t.Errorf("the tabs of italic panels are italic, got %v", slanted)
	}
	if h.tab(t, "b").Label != "b" {
		t.Error("a reader hears the title as it is")
	}
	h.ctrl.Unpin("files", DockLeft)
	h.render()
	if !slices.Contains(h.labels(semantics.RoleTab), "files") {
		t.Fatal("no strip tab")
	}
	h.ctrl.Show("files")
	h.render()
	if !slices.Contains(h.labels(semantics.RoleGroup), "files") {
		t.Error("the italic panel slides out")
	}
}
