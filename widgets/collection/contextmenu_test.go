package collection

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
)

// menuCall is one call of an OnContextMenu.
type menuCall[K comparable] struct {
	item K
	pos  geom.Offset
}

// secondaryClick presses and releases the secondary button at pos.
func secondaryClick(tree *core.Tree, pos geom.Offset, clock *uint32) {
	*clock += 1000
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: pos, Time: *clock})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos, Button: input.ButtonSecondary, Time: *clock})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos, Button: input.ButtonSecondary, Time: *clock + 10})
}

func (r *treeRig) secondary(label string) geom.Offset {
	pos := r.at(label, 150, treeRowHeight/2)
	secondaryClick(r.tree, pos, &r.clock)
	r.frame()
	return pos
}

func TestTree_SecondaryClickSelectsTheRowAndAsksForAMenu(t *testing.T) {
	var calls []menuCall[string]
	r := newTreeRig(t, true, func(v *Tree[string]) {
		v.OnContextMenu = func(node string, pos geom.Offset) { calls = append(calls, menuCall[string]{node, pos}) }
	})
	r.click("Music", 0)
	at := r.secondary("Pictures")
	r.wantSelection("Pictures")
	r.wantFocus("Pictures")
	if len(calls) != 1 || calls[0].item != "Pictures" || calls[0].pos != at {
		t.Fatalf("OnContextMenu got %v, want Pictures at %v", calls, at)
	}
	// A secondary click on one of several selected rows keeps them all.
	r.click("Documents", 0)
	r.click("Music", input.ModControl)
	r.secondary("Documents")
	r.wantSelection("Documents", "Music")
	r.wantFocus("Documents")
	if len(calls) != 2 || calls[1].item != "Documents" {
		t.Errorf("OnContextMenu got %v", calls)
	}
	// A secondary click on a row outside the selection selects it alone,
	// whatever is held.
	r.secondary("notes.txt")
	r.wantSelection("notes.txt")
}

func TestTree_ContextMenuKey(t *testing.T) {
	var calls []menuCall[string]
	r := newTreeRig(t, false, func(v *Tree[string]) {
		v.OnContextMenu = func(node string, pos geom.Offset) { calls = append(calls, menuCall[string]{node, pos}) }
	})
	r.click("Music", 0)
	r.Key(input.KeyDown, 0)
	r.Key(input.KeyContextMenu, 0)
	row := r.item("Pictures").Rect
	want := geom.Offset{X: row.X + treeLabelX(0, false), Y: row.Y + row.H}
	if len(calls) != 1 || calls[0].item != "Pictures" || calls[0].pos != want {
		t.Fatalf("the menu key got %v, want Pictures at %v", calls, want)
	}
	r.Key(input.KeyF10, input.ModShift)
	if len(calls) != 2 || calls[1].item != "Pictures" {
		t.Errorf("Shift+F10 got %v", calls)
	}
	// F10 alone is the menu bar's, not the tree's.
	r.Key(input.KeyF10, 0)
	if len(calls) != 2 {
		t.Errorf("F10 asked for a context menu: %v", calls)
	}
}

func TestTree_SecondaryClickWithoutOnContextMenuKeepsTheFocus(t *testing.T) {
	r := newTreeRig(t, false, nil)
	r.click("Music", 0)
	r.secondary("Pictures")
	r.wantSelection("Pictures")
	r.wantFocus("Pictures")
}

func TestTree_SecondaryClickSelectsUnderAContextMenu(t *testing.T) {
	data := sampleFolders()
	ctrl := &TreeController[string]{}
	ran := 0
	items := []overlay.MenuItem{{Action: interact.Action{Label: "Delete", OnInvoke: func() { ran++ }}}}
	tree := wtest.TextTree(t, overlay.Overlay{Child: basic.Align{Alignment: layout.AlignTopLeft, Child: basic.SizedBox{
		Width: treeScreen.W, Height: treeScreen.H,
		Child: overlay.ContextMenu{Items: items, Child: Tree[string]{
			Roots: data[""], ChildrenOf: func(k string) []string { return data[k] }, Label: "Files", Controller: ctrl,
		}},
	}}})
	wtest.Ops(tree, treeScreen)
	var row *semantics.Node
	for _, n := range wtest.RolesOf(tree.Semantics(), semantics.RoleTreeItem) {
		if n.Label == "Music" {
			row = n
		}
	}
	if row == nil {
		t.Fatal("no row Music")
	}
	var clock uint32
	secondaryClick(tree, geom.Offset{X: row.Rect.X + 100, Y: row.Rect.Y + treeRowHeight/2}, &clock)
	wtest.Ops(tree, treeScreen)
	if got := ctrl.Selection(); len(got) != 1 || got[0] != "Music" {
		t.Errorf("selection %v, want the row under the click", got)
	}
	if wtest.ByLabel(t, tree.Semantics(), "Delete") == nil {
		t.Error("the ContextMenu around the tree did not open")
	}
}

func TestGridView_SecondaryClickSelectsAndAsksForAMenu(t *testing.T) {
	var calls []menuCall[int]
	r := newTileRig(t, 20, func(g *GridView) {
		g.OnContextMenu = func(i int, pos geom.Offset) { calls = append(calls, menuCall[int]{i, pos}) }
	})
	r.click(1, 0)
	at := wtest.Center(r.cell(5).Rect)
	secondaryClick(r.tree, at, &r.clock)
	r.frame()
	if got := r.selection(); !slices.Equal(got, []int{5}) {
		t.Errorf("selection %v, want [5]", got)
	}
	r.wantFocus(5)
	if len(calls) != 1 || calls[0].item != 5 || calls[0].pos != at {
		t.Fatalf("OnContextMenu got %v, want 5 at %v", calls, at)
	}
	// On a selection of several, the selection stays.
	r.click(2, input.ModControl)
	secondaryClick(r.tree, wtest.Center(r.cell(2).Rect), &r.clock)
	r.frame()
	if got := r.selection(); !slices.Equal(got, []int{2, 5}) {
		t.Errorf("selection %v, want [2 5]", got)
	}
	r.wantFocus(2)
	r.Key(input.KeyContextMenu, 0)
	cell := r.cell(2).Rect
	if len(calls) != 3 || calls[2].item != 2 || calls[2].pos != (geom.Offset{X: cell.X, Y: cell.Y + cell.H}) {
		t.Errorf("the menu key got %v, want 2 below %v", calls, cell)
	}
}

func TestGridView_SecondaryClickWithoutOnContextMenuKeepsTheFocus(t *testing.T) {
	r := newTileRig(t, 20, nil)
	r.click(1, 0)
	secondaryClick(r.tree, wtest.Center(r.cell(6).Rect), &r.clock)
	r.frame()
	if got := r.selection(); !slices.Equal(got, []int{6}) {
		t.Errorf("selection %v, want [6]", got)
	}
	r.wantFocus(6)
}

func TestTable_SecondaryClickSelectsAndAsksForAMenu(t *testing.T) {
	ctrl := &TableController{}
	var calls []menuCall[int]
	tree := tableTree(t, Table[track]{Rows: tracks(100), Columns: trackColumns(), Controller: ctrl,
		OnContextMenu: func(row int, pos geom.Offset) { calls = append(calls, menuCall[int]{row, pos}) }})
	var now uint32
	clickAt(tree, rowPos(1), 0, &now)
	secondaryClick(tree, rowPos(4), &now)
	wtest.Ops(tree, tableScreen)
	if got := selection(ctrl); !sameRanges(got, []RowRange{{4, 5}}) {
		t.Errorf("selection %v, want row 4", got)
	}
	if got := wtest.FocusedLabel(tree); got != "Track 4" {
		t.Errorf("focus on %q", got)
	}
	if len(calls) != 1 || calls[0].item != 4 || calls[0].pos != rowPos(4) {
		t.Fatalf("OnContextMenu got %v, want 4 at %v", calls, rowPos(4))
	}
	// A secondary click in a selection of several keeps it and moves the
	// focus.
	clickAt(tree, rowPos(6), input.ModShift, &now)
	secondaryClick(tree, rowPos(5), &now)
	wtest.Ops(tree, tableScreen)
	if got := selection(ctrl); !sameRanges(got, []RowRange{{4, 7}}) {
		t.Errorf("selection %v, want rows 4 to 6", got)
	}
	if row, _ := ctrl.FocusedCell(); row != 5 {
		t.Errorf("focus on row %d, want 5", row)
	}
	tableKey(tree, input.KeyF10, input.ModShift)
	want := geom.Offset{X: 0, Y: tableHeaderHeight + 6*tableRowHeight}
	if len(calls) != 3 || calls[2].item != 5 || calls[2].pos != want {
		t.Errorf("Shift+F10 got %v, want 5 at %v", calls, want)
	}
}

func TestTable_SecondaryClickGoesOnToAContextMenu(t *testing.T) {
	ctrl := &TableController{}
	items := []overlay.MenuItem{{Action: interact.Action{Label: "Remove", OnInvoke: func() {}}}}
	tree := tableTree(t, overlay.ContextMenu{Items: items, Child: Table[track]{Rows: tracks(100), Columns: trackColumns(), Controller: ctrl}})
	var now uint32
	secondaryClick(tree, rowPos(3), &now)
	wtest.Ops(tree, tableScreen)
	if got := selection(ctrl); !sameRanges(got, []RowRange{{3, 4}}) {
		t.Errorf("selection %v, want row 3", got)
	}
	if wtest.ByLabel(t, tree.Semantics(), "Remove") == nil {
		t.Error("the ContextMenu around the table did not open")
	}
}

func TestTable_SecondaryClickWithoutAMenuKeepsTheFocus(t *testing.T) {
	ctrl := &TableController{}
	tree := tableTree(t, Table[track]{Rows: tracks(100), Columns: trackColumns(), Controller: ctrl})
	var now uint32
	clickAt(tree, rowPos(1), 0, &now)
	secondaryClick(tree, rowPos(2), &now)
	wtest.Ops(tree, tableScreen)
	if got := wtest.FocusedLabel(tree); got != "Track 2" {
		t.Errorf("focus on %q, want Track 2", got)
	}
}
