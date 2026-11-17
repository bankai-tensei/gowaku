package collection

import (
	"strconv"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
)

// tableScreen holds a table of ten rows in view below its header.
var tableScreen = geom.Size{W: 600, H: 400}

const tableTestHeight = tableHeaderHeight + 10*tableRowHeight

type track struct {
	title, artist, plays string
}

func tracks(n int) []track {
	out := make([]track, n)
	for i := range out {
		out[i] = track{title: "Track " + strconv.Itoa(i), artist: "Artist " + strconv.Itoa(i%7), plays: strconv.Itoa(i * 3)}
	}
	return out
}

func trackColumns() []TableColumn[track] {
	return []TableColumn[track]{
		{Title: "Title", Width: 200, Text: func(t track) string { return t.title }, Sortable: true, Editable: true},
		{Title: "Artist", Width: 160, Text: func(t track) string { return t.artist }, Filterable: true,
			FilterValues: []string{"Artist 0", "Artist 1", "Artist 2"}},
		{Title: "Plays", Width: 100, Text: func(t track) string { return t.plays }, Trailing: true, Editable: true},
	}
}

// tableTree renders w at the top left of a window with an overlay.
func tableTree(t testing.TB, w core.Widget) *core.Tree {
	t.Helper()
	tree := wtest.TextTree(t, overlay.Overlay{Child: basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1}, Child: basic.Align{
		Alignment: layout.AlignTopLeft, Child: basic.SizedBox{Width: tableScreen.W, Height: tableTestHeight, Child: w},
	}}})
	wtest.Ops(tree, tableScreen)
	return tree
}

func tableKey(tree *core.Tree, k input.KeyCode, m input.Modifiers) {
	tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: m})
	wtest.Ops(tree, tableScreen)
}

// clickAt presses and releases the pointer at pos, a second apart from the
// last click so that clicks do not pair up.
func clickAt(tree *core.Tree, pos geom.Offset, m input.Modifiers, now *uint32) {
	*now += 1000
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos, Modifiers: m, Time: *now})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos, Modifiers: m, Time: *now})
	wtest.Ops(tree, tableScreen)
}

// rowPos returns a point in the cell of row r and the first column, while
// the table is at the top.
func rowPos(r int) geom.Offset {
	return geom.Offset{X: 50, Y: tableHeaderHeight + float32(r)*tableRowHeight + tableRowHeight/2}
}

func selection(c *TableController) []RowRange { return append([]RowRange(nil), c.Selection()...) }

func sameRanges(a, b []RowRange) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDataTable_Semantics(t *testing.T) {
	tree := tableTree(t, Table[track]{Label: "Library", Rows: tracks(1000), Columns: trackColumns(),
		Sort: TableSort{Column: 0, Order: SortAscending}, OnSort: func(TableSort) {}, OnFilter: func(int, TableFilter) {}})
	root := tree.Semantics()
	grid := wtest.FindRole(root, semantics.RoleGrid)
	if grid == nil || grid.Label != "Library" || grid.RowCount != 1001 || grid.ColumnCount != 3 {
		t.Fatalf("got grid %+v", grid)
	}
	if grid.State&semantics.Multiselectable == 0 {
		t.Error("a table selects many rows by default")
	}
	rows := wtest.RolesOf(grid, semantics.RoleRow)
	// The header and the rows in view, part of the eleventh included.
	if len(rows) != 12 || rows[0].RowIndex != 1 || rows[1].RowIndex != 2 {
		t.Fatalf("got %d rows, first %+v", len(rows), rows[0])
	}
	heads := wtest.RolesOf(rows[0], semantics.RoleColumnHeader)
	if len(heads) != 3 || heads[0].Label != "Title" || heads[0].Sort != semantics.SortAscending || heads[0].Actions&semantics.ActionTap == 0 {
		t.Fatalf("got heads %+v", heads)
	}
	if heads[0].Hint != "" || heads[1].Sort != semantics.SortNone {
		t.Errorf("the order is said once, by the sorted header: %+v, %+v", heads[0], heads[1])
	}
	if heads[1].Actions&semantics.ActionTap != 0 {
		t.Error("a column that does not sort cannot be tapped")
	}
	if b := wtest.ByLabel(t, heads[1], "Filter"); b.Role != semantics.RoleButton {
		t.Errorf("filter button %+v", b)
	}
	cells := wtest.RolesOf(rows[3], semantics.RoleGridCell)
	if len(cells) != 3 || cells[0].Label != "Track 2" || cells[2].Label != "6" || cells[1].RowIndex != 4 || cells[1].ColumnIndex != 2 {
		t.Fatalf("got cells %+v", cells)
	}
	if want := cells[0].Label + " " + cells[1].Label + " " + cells[2].Label; rows[3].Label != want {
		t.Errorf("a row must be named by its cells: %q, want %q", rows[3].Label, want)
	}
	if rows[0].Label != "" {
		t.Errorf("the header row is named %q", rows[0].Label)
	}
	if cells[0].Rect.Y != tableHeaderHeight+2*tableRowHeight || cells[1].Rect.X != 200 {
		t.Errorf("cell boxes %v %v", cells[0].Rect, cells[1].Rect)
	}
}

func TestDataTable_OnlyRowsInViewHaveNodes(t *testing.T) {
	tree := tableTree(t, Table[track]{Rows: tracks(1_000_000), Columns: trackColumns(), Select: TableSelectSingle})
	root := tree.Semantics()
	grid := wtest.FindRole(root, semantics.RoleGrid)
	if grid.RowCount != 1_000_001 || grid.State&semantics.Multiselectable != 0 {
		t.Fatalf("got %+v", grid)
	}
	if n := len(wtest.RolesOf(grid, semantics.RoleGridCell)); n > 12*3 {
		t.Errorf("%d cells have nodes", n)
	}
}

func TestDataTable_KeysMoveTheFocusAndTheSelection(t *testing.T) {
	ctrl := &TableController{}
	tree := tableTree(t, Table[track]{Rows: tracks(100), Columns: trackColumns(), Controller: ctrl})
	tableKey(tree, input.KeyTab, 0)
	if got := wtest.FocusedLabel(tree); got != "Track 0" {
		t.Fatalf("Tab: focus on %q", got)
	}
	steps := []struct {
		name  string
		key   input.KeyCode
		mods  input.Modifiers
		focus string
		sel   []RowRange
	}{
		{"下", input.KeyDown, 0, "Track 1", []RowRange{{1, 2}}},
		{"Shift+下", input.KeyDown, input.ModShift, "Track 2", []RowRange{{1, 3}}},
		{"右", input.KeyRight, 0, "Artist 2", []RowRange{{1, 3}}},
		{"Ctrl+下は選択を変えない", input.KeyDown, input.ModControl, "Artist 3", []RowRange{{1, 3}}},
		{"Space で足す", input.KeySpace, 0, "Artist 3", []RowRange{{1, 4}}},
		{"Space で外す", input.KeySpace, 0, "Artist 3", []RowRange{{1, 3}}},
		{"End", input.KeyEnd, 0, "9", []RowRange{{1, 3}}},
		{"Home", input.KeyHome, 0, "Track 3", []RowRange{{1, 3}}},
		{"PageDown", input.KeyPageDown, 0, "Track 12", []RowRange{{12, 13}}},
		{"Ctrl+End", input.KeyEnd, input.ModControl, "297", []RowRange{{99, 100}}},
		{"Ctrl+Home", input.KeyHome, input.ModControl, "Track 0", []RowRange{{0, 1}}},
		{"上で見出しへ", input.KeyUp, 0, "Title", []RowRange{{0, 1}}},
		{"Ctrl+A", input.KeyA, input.ModControl, "Title", []RowRange{{0, 100}}},
	}
	for _, st := range steps {
		tableKey(tree, st.key, st.mods)
		if got := wtest.FocusedLabel(tree); got != st.focus {
			t.Errorf("%s: focus on %q, want %q", st.name, got, st.focus)
		}
		if got := selection(ctrl); !sameRanges(got, st.sel) {
			t.Errorf("%s: selection %v, want %v", st.name, got, st.sel)
		}
	}
}

func TestDataTable_FocusFollowsTheWheel(t *testing.T) {
	tree := tableTree(t, Table[track]{Rows: tracks(1000), Columns: trackColumns()})
	tableKey(tree, input.KeyTab, 0)
	tableKey(tree, input.KeyDown, 0)
	wtest.Wheel(tree, -10)
	wtest.RunUntilIdle(t, tree, tableScreen, nil)
	wtest.Ops(tree, tableScreen)
	// The cell went out of view: the table holds the keyboard, and the next
	// arrow brings the cell back.
	_, focus := tree.SemanticsTree()
	if n := tree.Semantics().Find(focus); n == nil || n.Role != semantics.RoleGrid {
		t.Fatalf("focus on %+v", n)
	}
	tableKey(tree, input.KeyDown, 0)
	if got := wtest.FocusedLabel(tree); got != "Track 2" {
		t.Errorf("focus on %q", got)
	}
}

func TestDataTable_ClicksSelect(t *testing.T) {
	ctrl := &TableController{}
	changes := 0
	tree := tableTree(t, Table[track]{Rows: tracks(100), Columns: trackColumns(), Controller: ctrl,
		OnSelectionChange: func() { changes++ }})
	var now uint32
	steps := []struct {
		name string
		row  int
		mods input.Modifiers
		sel  []RowRange
	}{
		{"クリック", 3, 0, []RowRange{{3, 4}}},
		{"Ctrl+クリックで足す", 5, input.ModControl, []RowRange{{3, 4}, {5, 6}}},
		{"Shift+クリックで範囲", 8, input.ModShift, []RowRange{{5, 9}}},
		{"Ctrl+Shift+クリックで範囲を足す", 1, input.ModControl | input.ModShift, []RowRange{{1, 9}}},
		{"Ctrl+クリックで外す", 4, input.ModControl, []RowRange{{1, 4}, {5, 9}}},
	}
	for _, st := range steps {
		clickAt(tree, rowPos(st.row), st.mods, &now)
		if got := selection(ctrl); !sameRanges(got, st.sel) {
			t.Errorf("%s: selection %v, want %v", st.name, got, st.sel)
		}
	}
	if changes != len(steps) {
		t.Errorf("got %d changes", changes)
	}
	if got := wtest.FocusedLabel(tree); got != "Track 4" {
		t.Errorf("focus on %q", got)
	}
}

func TestDataTable_DoubleClickActivates(t *testing.T) {
	activated := -1
	tree := tableTree(t, Table[track]{Rows: tracks(100), Columns: trackColumns(), OnActivate: func(r int) { activated = r }})
	at := geom.Offset{X: 250, Y: rowPos(2).Y}
	for range 2 {
		tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at, Time: 10})
		tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: at, Time: 10})
	}
	wtest.Ops(tree, tableScreen)
	if activated != 2 {
		t.Errorf("activated %d", activated)
	}
}

func TestDataTable_HeaderClickSorts(t *testing.T) {
	cases := []struct {
		name string
		sort TableSort
		want TableSort
	}{
		{"並べていない列は昇順", TableSort{}, TableSort{Column: 0, Order: SortAscending}},
		{"昇順の次は降順", TableSort{Column: 0, Order: SortAscending}, TableSort{Column: 0, Order: SortDescending}},
		{"降順の次は元に戻す", TableSort{Column: 0, Order: SortDescending}, TableSort{Column: 0, Order: SortNone}},
		{"ほかの列で並べていても昇順", TableSort{Column: 2, Order: SortDescending}, TableSort{Column: 0, Order: SortAscending}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []TableSort
			tree := tableTree(t, Table[track]{Rows: tracks(10), Columns: trackColumns(), Sort: tc.sort,
				OnSort: func(s TableSort) { got = append(got, s) }})
			var now uint32
			clickAt(tree, geom.Offset{X: 60, Y: 16}, 0, &now)
			// A column that does not sort ignores the click.
			clickAt(tree, geom.Offset{X: 240, Y: 16}, 0, &now)
			if len(got) != 1 || got[0] != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func drag(tree *core.Tree, from, to geom.Offset) {
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: from})
	mid := geom.Offset{X: (from.X + to.X) / 2, Y: (from.Y + to.Y) / 2}
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: mid})
	wtest.Ops(tree, tableScreen)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: to})
	wtest.Ops(tree, tableScreen)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: to})
	wtest.Ops(tree, tableScreen)
}

func TestDataTable_DraggingAnEdgeResizes(t *testing.T) {
	ctrl := &TableController{}
	saved := 0
	tree := tableTree(t, Table[track]{Rows: tracks(10), Columns: trackColumns(), Controller: ctrl,
		OnColumnsChange: func() { saved++ }})
	drag(tree, geom.Offset{X: 201, Y: 16}, geom.Offset{X: 251, Y: 16})
	if got := ctrl.ColumnWidth(0); got != 250 {
		t.Errorf("width %v", got)
	}
	cells := wtest.RolesOf(tree.Semantics(), semantics.RoleGridCell)
	if cells[1].Rect.X != 250 {
		t.Errorf("the next column starts at %v", cells[1].Rect.X)
	}
	drag(tree, geom.Offset{X: 250, Y: 16}, geom.Offset{X: 0, Y: 16})
	if got := ctrl.ColumnWidth(0); got != tableColumnMin {
		t.Errorf("narrowed to %v", got)
	}
	if saved != 2 {
		t.Errorf("saved %d times", saved)
	}
}

func TestDataTable_DraggingAHeaderMovesTheColumn(t *testing.T) {
	ctrl := &TableController{}
	sorted := 0
	tree := tableTree(t, Table[track]{Rows: tracks(10), Columns: trackColumns(), Controller: ctrl,
		OnSort: func(TableSort) { sorted++ }})
	// Title, from the middle of its header to past the middle of Plays.
	drag(tree, geom.Offset{X: 100, Y: 16}, geom.Offset{X: 440, Y: 16})
	if got := ctrl.ColumnOrder(); len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 0 {
		t.Fatalf("order %v", got)
	}
	if sorted != 0 {
		t.Error("a drag does not sort")
	}
	heads := wtest.RolesOf(tree.Semantics(), semantics.RoleColumnHeader)
	if heads[0].Label != "Artist" || heads[2].Label != "Title" || heads[2].Rect.X != 260 {
		t.Errorf("heads %q %q at %v", heads[0].Label, heads[2].Label, heads[2].Rect)
	}
}

func TestDataTable_EditCommitsCancelsAndTabs(t *testing.T) {
	var edits []TableEdit
	canceled := 0
	refuse := false
	tree := tableTree(t, Table[track]{Rows: tracks(10), Columns: trackColumns(),
		OnCommit: func(e TableEdit) bool {
			edits = append(edits, e)
			return !refuse
		},
		OnCancel: func(int, int) { canceled++ },
	})
	tableKey(tree, input.KeyTab, 0)
	tableKey(tree, input.KeyF2, 0)
	field := wtest.FindRole(tree.Semantics(), semantics.RoleTextField)
	if field == nil || field.Value != "Track 0" || field.State&semantics.Focused == 0 {
		t.Fatalf("editor %+v", field)
	}
	tree.DispatchText("!")
	tableKey(tree, input.KeyEnter, 0)
	if len(edits) != 1 || edits[0] != (TableEdit{Row: 0, Column: 0, Text: "Track 0!"}) {
		t.Fatalf("edits %v", edits)
	}
	if wtest.FindRole(tree.Semantics(), semantics.RoleTextField) != nil || wtest.FocusedLabel(tree) != "Track 0" {
		t.Fatalf("the editor stays, focus on %q", wtest.FocusedLabel(tree))
	}

	tableKey(tree, input.KeyEnter, 0)
	tree.DispatchText("x")
	tableKey(tree, input.KeyEscape, 0)
	if canceled != 1 || len(edits) != 1 {
		t.Errorf("canceled %d, edits %v", canceled, edits)
	}

	// Tab skips Artist, which is not editable, to Plays.
	tableKey(tree, input.KeyEnter, 0)
	tableKey(tree, input.KeyTab, 0)
	field = wtest.FindRole(tree.Semantics(), semantics.RoleTextField)
	if field == nil || field.Value != "0" || len(edits) != 2 {
		t.Fatalf("after Tab %+v, edits %v", field, edits)
	}
	// And then to the title of the next row.
	tableKey(tree, input.KeyTab, 0)
	if field = wtest.FindRole(tree.Semantics(), semantics.RoleTextField); field == nil || field.Value != "Track 1" {
		t.Fatalf("after the second Tab %+v", field)
	}
	refuse = true
	tree.DispatchText("?")
	tableKey(tree, input.KeyEnter, 0)
	if wtest.FindRole(tree.Semantics(), semantics.RoleTextField) == nil {
		t.Error("a refused edit keeps the editor")
	}
}

func TestDataTable_CustomEditor(t *testing.T) {
	cols := trackColumns()
	var got TableEditor
	cols[0].Editor = func(e TableEditor) core.Widget {
		got = e
		return control.Button{Label: "Pick", OnPress: func() {
			e.Change("Picked")
			e.Commit()
		}}
	}
	var edits []TableEdit
	tree := tableTree(t, Table[track]{Rows: tracks(10), Columns: cols,
		OnCommit: func(e TableEdit) bool { edits = append(edits, e); return true }})
	tableKey(tree, input.KeyTab, 0)
	tableKey(tree, input.KeyDown, 0)
	tableKey(tree, input.KeyEnter, 0)
	if got.Row != 1 || got.Text != "Track 1" || wtest.FocusedLabel(tree) != "Pick" {
		t.Fatalf("editor %+v, focus %q", got, wtest.FocusedLabel(tree))
	}
	tableKey(tree, input.KeySpace, 0)
	if len(edits) != 1 || edits[0].Text != "Picked" || edits[0].Row != 1 {
		t.Errorf("edits %v", edits)
	}
}

func TestDataTable_FrozenColumnsStay(t *testing.T) {
	cols := trackColumns()
	for i := range cols {
		cols[i].Width = 300
	}
	tree := tableTree(t, Table[track]{Rows: tracks(10), Columns: cols, Frozen: 1})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: geom.Offset{X: 400, Y: 100}, Scroll: geom.Offset{X: 3}})
	wtest.RunUntilIdle(t, tree, tableScreen, nil)
	wtest.Ops(tree, tableScreen)
	heads := wtest.RolesOf(tree.Semantics(), semantics.RoleColumnHeader)
	if heads[0].Rect.X != 0 {
		t.Errorf("the frozen column moved to %v", heads[0].Rect.X)
	}
	// Three columns of 300 in 600: the others scroll by up to 300.
	if heads[1].Rect.X != 0 || heads[2].Rect.X != 300 {
		t.Errorf("the others at %v and %v", heads[1].Rect.X, heads[2].Rect.X)
	}
	// A click on the frozen column hits it, not the one scrolled under it.
	var now uint32
	clickAt(tree, geom.Offset{X: 100, Y: rowPos(1).Y}, 0, &now)
	if got := wtest.FocusedLabel(tree); got != "Track 1" {
		t.Errorf("focus on %q", got)
	}
}

func TestDataTable_FilterOpensAndReports(t *testing.T) {
	var got []TableFilter
	tree := tableTree(t, Table[track]{Rows: tracks(10), Columns: trackColumns(),
		OnFilter: func(col int, f TableFilter) {
			if col != 1 {
				t.Errorf("column %d", col)
			}
			got = append(got, f)
		}})
	tableKey(tree, input.KeyTab, 0)
	tableKey(tree, input.KeyRight, 0)
	tableKey(tree, input.KeyDown, input.ModAlt)
	root := tree.Semantics()
	if d := wtest.FindRole(root, semantics.RoleDialog); d == nil || d.Label != "Filter Artist" {
		t.Fatalf("dialog %+v", d)
	}
	if n := root.Find(func() semantics.ID { _, f := tree.SemanticsTree(); return f }()); n == nil || n.Role != semantics.RoleTextField {
		t.Fatalf("focus on %+v", n)
	}
	tree.DispatchText("7")
	wtest.Ops(tree, tableScreen)
	if len(got) != 1 || got[0].Text != "7" || got[0].Values != nil {
		t.Fatalf("filters %v", got)
	}
	tree.PerformSemanticsAction(nodeOf(tree, semantics.RoleCheckbox, "Artist 1"), semantics.ActionTap, core.SemanticsArg{})
	wtest.Ops(tree, tableScreen)
	if f := got[len(got)-1]; len(f.Values) != 2 || f.Values[0] != "Artist 0" || f.Values[1] != "Artist 2" {
		t.Errorf("values %v", f.Values)
	}
	tableKey(tree, input.KeyEscape, 0)
	if wtest.FindRole(tree.Semantics(), semantics.RoleDialog) != nil {
		t.Error("Escape leaves the filter open")
	}
	if got := wtest.FocusedLabel(tree); got != "Artist 0" {
		t.Errorf("focus back on %q", got)
	}
}

// nodeOfLabel returns the first node labeled label, and nodeOf the first of
// role r labeled label.
func nodeOfLabel(tree *core.Tree, label string) semantics.ID {
	return nodeOf(tree, semantics.RoleNone, label)
}

func nodeOf(tree *core.Tree, r semantics.Role, label string) semantics.ID {
	root, _ := tree.SemanticsTree()
	var id semantics.ID
	root.Walk(func(n *semantics.Node) bool {
		if n.Label == label && (r == semantics.RoleNone || n.Role == r) {
			id = n.ID
		}
		return id == 0
	})
	return id
}

func TestDataTable_ScreenReaderTapsSelectAndSort(t *testing.T) {
	ctrl := &TableController{}
	var sorts []TableSort
	tree := tableTree(t, Table[track]{Rows: tracks(10), Columns: trackColumns(), Controller: ctrl,
		OnSort: func(s TableSort) { sorts = append(sorts, s) }})
	tree.PerformSemanticsAction(nodeOfLabel(tree, "Track 4"), semantics.ActionTap, core.SemanticsArg{})
	wtest.Ops(tree, tableScreen)
	if got := selection(ctrl); !sameRanges(got, []RowRange{{4, 5}}) {
		t.Errorf("selection %v", got)
	}
	tree.PerformSemanticsAction(nodeOfLabel(tree, "Title"), semantics.ActionTap, core.SemanticsArg{})
	if len(sorts) != 1 || sorts[0].Order != SortAscending {
		t.Errorf("sorts %v", sorts)
	}
}

func TestDataTable_ZeroValue(t *testing.T) {
	tree := tableTree(t, Table[track]{})
	if grid := wtest.FindRole(tree.Semantics(), semantics.RoleGrid); grid == nil || grid.RowCount != 1 {
		t.Fatalf("got %+v", grid)
	}
	tableKey(tree, input.KeyTab, 0)
	tableKey(tree, input.KeyDown, 0)
}
