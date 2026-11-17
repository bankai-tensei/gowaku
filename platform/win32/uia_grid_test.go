package win32

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/semantics"
)

// virtualGrid is a table of 1001 rows and 2 columns that has nodes for the
// header and rows 500 and 501 only, as a DataTable scrolled to the middle
// does, with row 501 selected.
func virtualGrid() *semantics.Node {
	cell := func(id semantics.ID, row, col int, label string) *semantics.Node {
		return &semantics.Node{ID: id, Role: semantics.RoleGridCell, Label: label, RowIndex: row, ColumnIndex: col}
	}
	return &semantics.Node{Children: []*semantics.Node{{
		ID: 1, Role: semantics.RoleGrid, State: semantics.Multiselectable, RowCount: 1001, ColumnCount: 2,
		Children: []*semantics.Node{
			{ID: 2, Role: semantics.RoleRow, RowIndex: 1, Children: []*semantics.Node{
				{ID: 3, Role: semantics.RoleColumnHeader, Label: "Name", RowIndex: 1, ColumnIndex: 1},
				{ID: 4, Role: semantics.RoleColumnHeader, Label: "Size", RowIndex: 1, ColumnIndex: 2},
			}},
			{ID: 5, Role: semantics.RoleRow, RowIndex: 501, Actions: semantics.ActionTap, Children: []*semantics.Node{
				cell(6, 501, 1, "a"), cell(7, 501, 2, "1 KB"),
			}},
			{ID: 8, Role: semantics.RoleRow, RowIndex: 502, Actions: semantics.ActionTap, State: semantics.Selected,
				Children: []*semantics.Node{cell(9, 502, 1, "b"), cell(10, 502, 2, "2 KB")}},
		},
	}}}
}

// plainGrid is a grid that states no places, as a Calendar: its size and
// places come from the rows and cells it holds.
func plainGrid() *semantics.Node {
	day := func(id semantics.ID) *semantics.Node {
		return &semantics.Node{ID: id, Role: semantics.RoleGridCell, Actions: semantics.ActionTap}
	}
	return &semantics.Node{Children: []*semantics.Node{{
		ID: 1, Role: semantics.RoleGrid, Children: []*semantics.Node{
			{ID: 2, Role: semantics.RoleRow, Children: []*semantics.Node{
				{ID: 3, Role: semantics.RoleColumnHeader}, {ID: 4, Role: semantics.RoleColumnHeader}, {ID: 5, Role: semantics.RoleColumnHeader},
			}},
			{ID: 6, Role: semantics.RoleRow, Children: []*semantics.Node{day(7), day(8), day(9)}},
			{ID: 10, Role: semantics.RoleRow, Children: []*semantics.Node{day(11), {ID: 12, Role: semantics.RoleGridCell, State: semantics.Selected}}},
		},
	}}}
}

func TestSnapshot_GridOfAVirtualTable(t *testing.T) {
	s := newSnapshot(virtualGrid())
	if r, c := s.gridSize(1); r != 1001 || c != 2 {
		t.Errorf("size %d × %d", r, c)
	}
	if r, c, ok := s.gridPosition(10); r != 501 || c != 1 || !ok {
		t.Errorf("place of a cell %d, %d, %v", r, c, ok)
	}
	if r, c, _ := s.gridPosition(4); r != 0 || c != 1 {
		t.Errorf("place of a header %d, %d", r, c)
	}
	if id, ok := s.gridItem(1, 500, 0); id != 6 || !ok {
		t.Errorf("item at 500, 0 = %d, %v", id, ok)
	}
	if _, ok := s.gridItem(1, 3, 0); ok {
		t.Error("a row out of view has no item")
	}
	if got := s.columnHeaders(1, -1); !slices.Equal(got, []semantics.ID{3, 4}) {
		t.Errorf("headers %v", got)
	}
	if got := s.columnHeaders(1, 1); !slices.Equal(got, []semantics.ID{4}) {
		t.Errorf("header of the second column %v", got)
	}
	if id, ok := s.gridOf(9); id != 1 || !ok {
		t.Errorf("grid of a cell %d, %v", id, ok)
	}
	if got := s.selection(1); !slices.Equal(got, []semantics.ID{8}) {
		t.Errorf("selection %v", got)
	}
	// A row and a cell both belong to the grid, however deep.
	for _, id := range []semantics.ID{5, 9} {
		if c, ok := s.selectionContainer(id); c != 1 || !ok {
			t.Errorf("container of %d = %d, %v", id, c, ok)
		}
	}
}

func TestSnapshot_GridThatStatesNoPlaces(t *testing.T) {
	s := newSnapshot(plainGrid())
	if r, c := s.gridSize(1); r != 3 || c != 3 {
		t.Errorf("size %d × %d", r, c)
	}
	if r, c, ok := s.gridPosition(12); r != 2 || c != 1 || !ok {
		t.Errorf("place %d, %d, %v", r, c, ok)
	}
	if id, ok := s.gridItem(1, 1, 2); id != 9 || !ok {
		t.Errorf("item %d, %v", id, ok)
	}
	if got := s.columnHeaders(1, 2); !slices.Equal(got, []semantics.ID{5}) {
		t.Errorf("header %v", got)
	}
	if got := s.selection(1); !slices.Equal(got, []semantics.ID{12}) {
		t.Errorf("selection %v", got)
	}
	if _, _, ok := s.gridPosition(1); ok {
		t.Error("the grid itself has no place")
	}
}
