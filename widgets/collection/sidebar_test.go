package collection

import (
	"fmt"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

// sidebarPage is a Sidebar first in a Row, as its doc says, before a page.
func sidebarPage(s Sidebar) core.Widget {
	return basic.Row{Cross: layout.CrossStretch, Children: []core.Widget{s, basic.Expanded{Child: basic.Container{}}}}
}

func sidebarItems(n int) []core.Widget {
	items := make([]core.Widget, n)
	for i := range items {
		items[i] = SidebarItem{Label: fmt.Sprintf("Item %d", i), OnPress: func() {}}
	}
	return items
}

func TestSidebar_WidthIncludesTheHairline(t *testing.T) {
	th := theme.Default().Light
	for _, c := range []struct {
		width, want float32
	}{{0, sidebarWidth}, {260, 260}} {
		tree := wtest.TextTree(t, sidebarPage(Sidebar{Width: c.width, Children: sidebarItems(2)}))
		var ground, hair geom.Rect
		for _, op := range wtest.Rrects(tree, wtest.Screen) {
			switch op.Color {
			case th.Color.Sidebar:
				ground = op.RRect.Rect
			case th.Color.Hair:
				if op.RRect.Rect.W == 1 {
					hair = op.RRect.Rect
				}
			}
		}
		if ground.W != c.want-1 || ground.H != wtest.Screen.H || hair.X != c.want-1 {
			t.Errorf("Width %v: ground %v, hairline %v; want %v in all, full height", c.width, ground, hair, c.want)
		}
	}
}

func TestSidebar_FillsAPaneThatFixesItsWidth(t *testing.T) {
	th := theme.Default().Light
	tree := wtest.TextTree(t, basic.Row{Cross: layout.CrossStretch, Children: []core.Widget{
		basic.SizedBox{Width: 300, Child: Sidebar{Children: sidebarItems(1)}},
	}})
	found := false
	for _, op := range wtest.Rrects(tree, wtest.Screen) {
		if op.Color == th.Color.Sidebar {
			found = true
			if op.RRect.Rect.W != 299 {
				t.Errorf("the ground is %v wide in a pane of 300", op.RRect.Rect.W)
			}
		}
	}
	if !found {
		t.Error("no ground drawn")
	}
}

// rectOf returns where the node that says label is.
func rectOf(t *testing.T, tree *core.Tree, label string) geom.Rect {
	t.Helper()
	root, _ := tree.SemanticsTree()
	return wtest.ByLabel(t, root, label).Rect
}

func TestSidebar_RowsScrollBetweenHeaderAndFooter(t *testing.T) {
	tree := core.NewTree(testutil.WithShaper(testutil.NewShaper(t), sidebarPage(Sidebar{
		Title:    "Library",
		Header:   control.Button{Label: "Find", OnPress: func() {}},
		Children: sidebarItems(30),
		Footer:   control.Button{Label: "Add", OnPress: func() {}},
	})))
	t.Cleanup(tree.Dispose)
	size := wtest.Screen
	testutil.RenderTree(tree, size)
	header, footer, first := rectOf(t, tree, "Find"), rectOf(t, tree, "Add"), rectOf(t, tree, "Item 0")
	if footer.Y+footer.H != size.H-sidebarInset {
		t.Errorf("the footer is at %v; it must sit at the bottom", footer)
	}
	if first.Y < header.Y+header.H {
		t.Errorf("the first row %v is not under the header %v", first, header)
	}

	at := wtest.Center(first)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: at})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: at, Scroll: geom.Offset{Y: -3}})
	var now time.Duration
	wtest.Settled(t, tree, size, &now)
	if moved := rectOf(t, tree, "Item 0"); moved.Y >= first.Y {
		t.Errorf("the rows did not scroll: Item 0 at %v, then %v", first, moved)
	}
	if got := rectOf(t, tree, "Find"); got != header {
		t.Errorf("the header moved from %v to %v", header, got)
	}
	if got := rectOf(t, tree, "Add"); got != footer {
		t.Errorf("the footer moved from %v to %v", footer, got)
	}
}

func TestSidebar_ListHoldsOnlyTheRows(t *testing.T) {
	tree := wtest.A11yTree(t, sidebarPage(Sidebar{
		Title:    "Library",
		Header:   control.Button{Label: "Find", OnPress: func() {}},
		Children: sidebarItems(3),
		Footer:   control.Button{Label: "Add", OnPress: func() {}},
	}))
	root, _ := tree.SemanticsTree()
	list := wtest.ByLabel(t, root, "Library")
	if list.Role != semantics.RoleList || countLabel(root, "Library") != 1 {
		t.Fatalf("the title must name the list once: %v, %d times", list.Role, countLabel(root, "Library"))
	}
	if n := len(wtest.RolesOf(list, semantics.RoleListItem)); n != 3 {
		t.Errorf("the list has %d items, want 3", n)
	}
	if countLabel(list, "Find") != 0 || countLabel(list, "Add") != 0 {
		t.Error("the header and the footer are not rows of the list")
	}
}

func TestSidebarItem_CountIsTheValueOfTheRow(t *testing.T) {
	tree := wtest.A11yTree(t, sidebarPage(Sidebar{Title: "Mail", Children: []core.Widget{
		SidebarItem{Label: "Inbox", Count: 128, OnPress: func() {}},
		SidebarItem{Label: "Sent", OnPress: func() {}},
	}}))
	root, _ := tree.SemanticsTree()
	inbox := wtest.ByLabel(t, root, "Inbox")
	if inbox.Role != semantics.RoleListItem || inbox.Value != "128" {
		t.Errorf("Inbox: %v %q, want a list item of value 128", inbox.Role, inbox.Value)
	}
	said := 0
	root.Walk(func(n *semantics.Node) bool {
		if n.Label == "99+" || n.Value == "99+" || n.Label == "128" {
			said++
		}
		return true
	})
	if said != 0 || len(inbox.Children) != 0 {
		t.Errorf("the pill must not be read again: %d nodes, %d children", said, len(inbox.Children))
	}
	if sent := wtest.ByLabel(t, root, "Sent"); sent.Value != "" {
		t.Errorf("a row without a count has no value, got %q", sent.Value)
	}
	th := theme.Default().Light
	pills := 0
	for _, op := range wtest.Rrects(tree, wtest.Screen) {
		if op.Color == th.Color.Fill && op.RRect.Rect.H == 18 {
			pills++
		}
	}
	if pills != 1 {
		t.Errorf("got %d quiet pills, want 1", pills)
	}
}

func TestSidebarItem_DetailIsASecondLine(t *testing.T) {
	const preview = "See you at the station at seven, and bring the tickets we bought last week"
	tree := wtest.A11yTree(t, sidebarPage(Sidebar{Title: "Chats", Children: []core.Widget{
		SidebarItem{Label: "Aoi", Detail: preview, Count: 3, OnPress: func() {}},
		SidebarItem{Label: "Ren", OnPress: func() {}},
	}}))
	root, _ := tree.SemanticsTree()
	aoi := wtest.ByLabel(t, root, "Aoi")
	if aoi.Role != semantics.RoleListItem || aoi.Hint != preview || aoi.Value != "3" {
		t.Errorf("Aoi: %v hint %q value %q, want the whole detail as the description", aoi.Role, aoi.Hint, aoi.Value)
	}
	said := 0
	root.Walk(func(n *semantics.Node) bool {
		if n != aoi && (n.Label == preview || n.Value == preview || n.Hint == preview) {
			said++
		}
		return true
	})
	if said != 0 || len(aoi.Children) != 0 {
		t.Errorf("the detail must be read once: %d more nodes, %d children", said, len(aoi.Children))
	}
	ren := wtest.ByLabel(t, root, "Ren")
	if ren.Hint != "" {
		t.Errorf("a row without a detail has no description, got %q", ren.Hint)
	}
	if aoi.Rect.H != sidebarDetailRow || ren.Rect.H != theme.ControlHeight {
		t.Errorf("rows %g and %g high, want %d with a detail and %d without", aoi.Rect.H, ren.Rect.H, sidebarDetailRow, theme.ControlHeight)
	}
	// The detail is one line in Ink2, cut with an ellipsis.
	th := theme.Default().Light
	var found bool
	for _, ro := range tree.HitTest(geom.Offset{X: aoi.Rect.X + aoi.Rect.W/2, Y: aoi.Rect.Y + aoi.Rect.H - 12}).Path {
		if p, ok := ro.(*basic.RenderParagraph); ok && p.Text() == preview {
			found = true
			l := p.TextLayout()
			if l.LineCount() != 1 || !l.Truncated() || p.Style().Color != th.Color.Ink2 || p.Style().Size != th.Text.Caption.Size {
				t.Errorf("detail: %d lines, truncated %v, style %+v", l.LineCount(), l.Truncated(), p.Style())
			}
		}
	}
	if !found {
		t.Error("no detail under the label")
	}
}
