package nav

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

func TestCrumbFold(t *testing.T) {
	// Segments of 50 with separators of 16, and a … button of 40 with its own.
	seg := []float32{50, 50, 50, 50, 50}
	sep := []float32{0, 16, 16, 16, 16}
	cases := []struct {
		name     string
		limit    float32
		from, to int
	}{
		{"全部入る", 314, 0, 0},
		{"1 つ畳む", 313, 1, 2},
		{"1 つ畳む下限", 304, 1, 2},
		{"2 つ畳む", 303, 1, 3},
		{"2 つ畳む下限", 238, 1, 3},
		{"最初と最後だけ", 237, 1, 4},
		{"最初と最後だけ下限", 172, 1, 4},
		{"最初も畳む", 171, 0, 4},
		{"最後しか入らなくても最後は残す", 20, 0, 4},
	}
	for _, tc := range cases {
		from, to := crumbFold(seg, sep, 56, tc.limit)
		if from != tc.from || to != tc.to {
			t.Errorf("%s: got [%d, %d), want [%d, %d)", tc.name, from, to, tc.from, tc.to)
		}
	}
	if from, to := crumbFold(seg[:1], sep[:1], 56, 10); from != to {
		t.Error("a single segment never folds")
	}
}

// crumbRig is a Breadcrumb of five places in a window width wide, with a
// button after it, and the places it opened.
type crumbRig struct {
	tree   *core.Tree
	opened []int
	size   geom.Size
}

func newCrumbRig(t *testing.T, width float32) *crumbRig {
	t.Helper()
	r := &crumbRig{size: geom.Size{W: width, H: 200}}
	items := []BreadcrumbItem{{Label: "Home", Icon: kit.CrossPath(24, 12, 2)}, {Label: "Documents"}, {Label: "Projects"}, {Label: "gowaku"}, {Label: "widgets"}}
	r.tree = wtest.TextTree(t, overlay.Overlay{Child: basic.Column{Cross: layout.CrossStretch, Children: []core.Widget{
		Breadcrumb{Items: items, OnSelect: func(i int) { r.opened = append(r.opened, i) }},
		control.Button{Label: "after", OnPress: func() {}},
	}}})
	wtest.Ops(r.tree, r.size)
	return r
}

func (r *crumbRig) sem() *semantics.Node {
	wtest.Ops(r.tree, r.size)
	return r.tree.Semantics()
}

// shown returns the links, buttons and texts of the trail in order.
func (r *crumbRig) shown() []string {
	var out []string
	wtest.FindRole(r.sem(), semantics.RoleGroup).Walk(func(n *semantics.Node) bool {
		switch n.Role {
		case semantics.RoleLink, semantics.RoleButton, semantics.RoleText:
			out = append(out, n.Label)
		}
		return true
	})
	return out
}

func TestBreadcrumb_Semantics(t *testing.T) {
	r := newCrumbRig(t, 600)
	group := wtest.FindRole(r.sem(), semantics.RoleGroup)
	if group == nil || group.Label != "Breadcrumb" {
		t.Fatalf("got %+v", group)
	}
	if got, want := r.shown(), []string{"Home", "Documents", "Projects", "gowaku", "widgets"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	link := wtest.FindRole(group, semantics.RoleLink)
	if link.Label != "Home" || link.State&semantics.Focusable == 0 || link.Actions&semantics.ActionTap == 0 {
		t.Errorf("a link: %+v", link)
	}
	var current *semantics.Node
	group.Walk(func(n *semantics.Node) bool {
		if n.Role == semantics.RoleText && n.Label == "widgets" {
			current = n
		}
		return true
	})
	if current == nil || current.Hint != "Current location" || current.State&semantics.Focusable != 0 {
		t.Errorf("the current place is text: %+v", current)
	}
}

func TestBreadcrumb_FoldsTheMiddle(t *testing.T) {
	r := newCrumbRig(t, 300)
	got := r.shown()
	if len(got) < 3 || got[0] != "Home" || got[1] != "Hidden locations" || got[len(got)-1] != "widgets" {
		t.Fatalf("got %q", got)
	}
	if slices.Contains(got, "Documents") {
		t.Errorf("the first after Home folds first: %q", got)
	}
	// The menu of the … button opens the folded places.
	wtest.Tap(r.tree, wtest.Center(wtest.ByLabel(t, r.sem(), "Hidden locations").Rect))
	wtest.Ops(r.tree, r.size)
	item := wtest.ByLabel(t, r.sem(), "Documents")
	if item.Role != semantics.RoleMenuItem {
		t.Fatalf("got %+v", item)
	}
	wtest.Tap(r.tree, wtest.Center(item.Rect))
	if !slices.Equal(r.opened, []int{1}) {
		t.Errorf("opened %v", r.opened)
	}
}

func TestBreadcrumb_TooNarrowKeepsTheLast(t *testing.T) {
	r := newCrumbRig(t, 120)
	got := r.shown()
	if len(got) != 2 || got[0] != "Hidden locations" || got[1] != "widgets" {
		t.Errorf("got %q", got)
	}
}

func TestBreadcrumb_Keyboard(t *testing.T) {
	r := newCrumbRig(t, 600)
	focused := func() string {
		var out string
		r.sem().Walk(func(n *semantics.Node) bool {
			if n.State&semantics.Focused != 0 {
				out = n.Label
			}
			return out == ""
		})
		return out
	}
	var order []string
	for range 5 {
		wtest.Ops(r.tree, r.size)
		r.tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
		order = append(order, focused())
	}
	if want := []string{"Home", "Documents", "Projects", "gowaku", "after"}; !slices.Equal(order, want) {
		t.Errorf("every link is a stop of Tab, the current place none: %q", order)
	}
	press(r.tree, input.KeyTab, input.ModShift)
	wtest.Ops(r.tree, r.size)
	press(r.tree, input.KeySpace, 0)
	press(r.tree, input.KeyEnter, 0)
	if !slices.Equal(r.opened, []int{3}) {
		t.Errorf("Enter opens a link, Space does not: %v", r.opened)
	}
}

func TestBreadcrumb_ZeroValue(t *testing.T) {
	tree := wtest.TextTree(t, Breadcrumb{})
	wtest.Ops(tree, wtest.Screen)
	tree = wtest.TextTree(t, Breadcrumb{Items: []BreadcrumbItem{{Label: "Only"}}})
	wtest.Ops(tree, wtest.Screen)
	if wtest.FindRole(tree.Semantics(), semantics.RoleLink) != nil {
		t.Error("a single segment is the current place")
	}
}

func benchCrumb(b *testing.B) *core.Tree {
	b.Helper()
	items := []BreadcrumbItem{{Label: "Home"}, {Label: "Documents"}, {Label: "Projects"}, {Label: "gowaku"}, {Label: "widgets"}}
	tree := wtest.TextTree(b, overlay.Overlay{Child: Breadcrumb{Items: items, OnSelect: func(int) {}}})
	wtest.Ops(tree, geom.Size{W: 300, H: 100})
	return tree
}

func BenchmarkLayout_Breadcrumb(b *testing.B) {
	tree := benchCrumb(b)
	// Both widths fold the same segments, as a window resized by a little.
	c1, c2 := geom.Tight(geom.Size{W: 300, H: 100}), geom.Tight(geom.Size{W: 302, H: 100})
	b.ReportAllocs()
	flip := false
	for b.Loop() {
		flip = !flip
		if flip {
			tree.Layout(c2)
		} else {
			tree.Layout(c1)
		}
	}
}

func BenchmarkPaint_Breadcrumb(b *testing.B) {
	tree := benchCrumb(b)
	var cv paint.Canvas
	cv.Reset()
	tree.Paint(&cv)
	b.ReportAllocs()
	for b.Loop() {
		cv.Reset()
		tree.Paint(&cv)
	}
}

// press sends a key and renders a frame at the size of the picker tests.
func press(tree *core.Tree, k input.KeyCode, m input.Modifiers) {
	tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: m})
	wtest.Ops(tree, geom.Size{W: 600, H: 500})
}
