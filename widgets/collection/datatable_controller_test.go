package collection

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
)

func TestDataTableController_Ranges(t *testing.T) {
	steps := []struct {
		name string
		do   func(c *TableController)
		want []RowRange
	}{
		{"一つ選ぶ", func(c *TableController) { c.Select(4) }, []RowRange{{4, 5}}},
		{"接する範囲はつながる", func(c *TableController) { c.AddRange(RowRange{5, 8}) }, []RowRange{{4, 8}}},
		{"離れた範囲は別", func(c *TableController) { c.AddRange(RowRange{10, 12}) }, []RowRange{{4, 8}, {10, 12}}},
		{"前に足す", func(c *TableController) { c.AddRange(RowRange{0, 2}) }, []RowRange{{0, 2}, {4, 8}, {10, 12}}},
		{"またがる範囲はまとまる", func(c *TableController) { c.AddRange(RowRange{1, 11}) }, []RowRange{{0, 12}}},
		{"中を外すと割れる", func(c *TableController) { c.RemoveRange(RowRange{3, 5}) }, []RowRange{{0, 3}, {5, 12}}},
		{"端を外す", func(c *TableController) { c.RemoveRange(RowRange{10, 20}) }, []RowRange{{0, 3}, {5, 10}}},
		{"切り替えで外す", func(c *TableController) { c.Toggle(0) }, []RowRange{{1, 3}, {5, 10}}},
		{"切り替えで足す", func(c *TableController) { c.Toggle(4) }, []RowRange{{1, 3}, {4, 10}}},
		{"空の範囲は何もしない", func(c *TableController) { c.AddRange(RowRange{7, 7}) }, []RowRange{{1, 3}, {4, 10}}},
		{"まとめて置き換える", func(c *TableController) { c.SetSelection(RowRange{8, 9}, RowRange{2, 3}, RowRange{3, 4}) },
			[]RowRange{{2, 4}, {8, 9}}},
		{"すべて", func(c *TableController) { c.SelectAll() }, []RowRange{{0, 50}}},
		{"消す", func(c *TableController) { c.ClearSelection() }, nil},
	}
	c := &TableController{rows: 50}
	for _, st := range steps {
		st.do(c)
		if got := c.Selection(); !sameRanges(got, st.want) {
			t.Fatalf("%s: got %v, want %v", st.name, got, st.want)
		}
	}
}

func TestDataTableController_Queries(t *testing.T) {
	c := &TableController{}
	c.SetSelection(RowRange{2, 4}, RowRange{10, 11})
	for row, want := range map[int]bool{1: false, 2: true, 3: true, 4: false, 9: false, 10: true, 11: false} {
		if got := c.IsSelected(row); got != want {
			t.Errorf("IsSelected(%d) = %v", row, got)
		}
	}
	if n := c.SelectedCount(); n != 3 {
		t.Errorf("count %d", n)
	}
	var rows []int
	c.SelectedRows(func(r int) bool {
		rows = append(rows, r)
		return len(rows) < 2
	})
	if !slices.Equal(rows, []int{2, 3}) {
		t.Errorf("rows %v", rows)
	}
	c.clip(3)
	if got := c.Selection(); !sameRanges(got, []RowRange{{2, 3}}) {
		t.Errorf("after the rows shrank %v", got)
	}
}

func TestDataTableController_ColumnOrder(t *testing.T) {
	c := &TableController{}
	if got := c.displayOrder(nil, 3); !slices.Equal(got, []int{0, 1, 2}) {
		t.Errorf("default %v", got)
	}
	c.SetColumnOrder([]int{2, 0, 1})
	if got := c.displayOrder(nil, 3); !slices.Equal(got, []int{2, 0, 1}) {
		t.Errorf("set %v", got)
	}
	// An order for other columns, or one that repeats, is ignored.
	for _, bad := range [][]int{{0, 1}, {0, 0, 1}, {0, 1, 3}} {
		c.SetColumnOrder(bad)
		if got := c.displayOrder(nil, 3); !slices.Equal(got, []int{0, 1, 2}) {
			t.Errorf("%v gives %v", bad, got)
		}
	}
	c.SetColumnOrder(nil)
	if c.ColumnOrder() != nil {
		t.Error("nil restores the own order")
	}
	c.SetColumnWidth(2, 150)
	if c.ColumnWidth(2) != 150 || c.ColumnWidth(0) != 0 || c.ColumnWidth(9) != 0 {
		t.Error("widths")
	}
}

func TestDataTable_ControllerChangesRepaint(t *testing.T) {
	ctrl := &TableController{}
	tree := tableTree(t, Table[track]{Rows: tracks(20), Columns: trackColumns(), Controller: ctrl})
	ctrl.Select(3)
	wtest.Ops(tree, tableScreen)
	row := wtest.ByLabel(t, tree.Semantics(), "Track 3")
	if row.State&semantics.Selected == 0 {
		t.Error("a selection set from outside shows")
	}
	ctrl.SetColumnOrder([]int{2, 1, 0})
	wtest.Ops(tree, tableScreen)
	if heads := wtest.RolesOf(tree.Semantics(), semantics.RoleColumnHeader); heads[0].Label != "Plays" {
		t.Errorf("first head %q", heads[0].Label)
	}
}

func TestDataTable_TypingStartsAnEdit(t *testing.T) {
	var edits []TableEdit
	tree := tableTree(t, Table[track]{Rows: tracks(10), Columns: trackColumns(),
		OnCommit: func(e TableEdit) bool { edits = append(edits, e); return true }})
	tableKey(tree, input.KeyTab, 0)
	tree.DispatchText(" ")
	wtest.Ops(tree, tableScreen)
	if wtest.FindRole(tree.Semantics(), semantics.RoleTextField) != nil {
		t.Fatal("a space starts an edit")
	}
	tree.DispatchText("Z")
	wtest.Ops(tree, tableScreen)
	tree.DispatchText("u")
	tableKey(tree, input.KeyEnter, 0)
	if len(edits) != 1 || edits[0].Text != "Zu" {
		t.Errorf("edits %v", edits)
	}
	// Artist is not editable: typing does nothing.
	tableKey(tree, input.KeyRight, 0)
	tree.DispatchText("Q")
	wtest.Ops(tree, tableScreen)
	if wtest.FindRole(tree.Semantics(), semantics.RoleTextField) != nil {
		t.Error("a cell that is not editable started an edit")
	}
}

func TestDataTable_SingleAndNoSelection(t *testing.T) {
	single := &TableController{}
	tree := tableTree(t, Table[track]{Rows: tracks(20), Columns: trackColumns(), Controller: single, Select: TableSelectSingle})
	tableKey(tree, input.KeyTab, 0)
	tableKey(tree, input.KeyDown, input.ModShift)
	tableKey(tree, input.KeyDown, input.ModShift)
	if got := selection(single); !sameRanges(got, []RowRange{{2, 3}}) {
		t.Errorf("single: %v", got)
	}
	tableKey(tree, input.KeyA, input.ModControl)
	if got := selection(single); !sameRanges(got, []RowRange{{2, 3}}) {
		t.Errorf("single after Ctrl+A: %v", got)
	}

	none := &TableController{}
	tree = tableTree(t, Table[track]{Rows: tracks(20), Columns: trackColumns(), Controller: none, Select: TableSelectNone})
	tableKey(tree, input.KeyTab, 0)
	tableKey(tree, input.KeyDown, 0)
	tableKey(tree, input.KeySpace, 0)
	if len(none.Selection()) != 0 || wtest.FocusedLabel(tree) != "Track 1" {
		t.Errorf("none: %v, focus %q", none.Selection(), wtest.FocusedLabel(tree))
	}
	if row := wtest.FindRole(tree.Semantics(), semantics.RoleRow); row.Actions&semantics.ActionTap != 0 {
		t.Error("rows that cannot be selected are not tapped")
	}
}
