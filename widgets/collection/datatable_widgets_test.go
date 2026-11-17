package collection

import (
	"math"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

// probeLog records where the probes of a table painted.
type probeLog struct{ paints []probePaint }

type probePaint struct {
	id   int
	at   geom.Offset
	size geom.Size
}

func (l *probeLog) reset() { l.paints = l.paints[:0] }

// ids returns the ids painted, in order.
func (l *probeLog) ids() []int {
	out := make([]int, 0, len(l.paints))
	for _, p := range l.paints {
		out = append(out, p.id)
	}
	return out
}

// cellProbe is the widget of a cell that shows the row with id: it is 10
// tall and as wide as it may be, and records where it paints.
type cellProbe struct {
	id  int
	log *probeLog
}

// Key implements core.Widget.
func (cellProbe) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (p cellProbe) CreateRenderObject() core.RenderObject {
	return &renderCellProbe{id: p.id, log: p.log}
}

// UpdateRenderObject implements core.RenderWidget.
func (p cellProbe) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderCellProbe)
	r.id, r.log = p.id, p.log
	r.MarkNeedsPaint()
}

type renderCellProbe struct {
	core.RenderBase
	id  int
	log *probeLog
}

func (r *renderCellProbe) Layout(cs layout.Constraints) geom.Size {
	return cs.Constrain(geom.Size{W: cs.MaxW, H: 10})
}

func (r *renderCellProbe) Paint(_ *paint.Canvas, off geom.Offset) {
	if r.log != nil {
		r.log.paints = append(r.log.paints, probePaint{id: r.id, at: off, size: r.Size()})
	}
}

func (r *renderCellProbe) Semantics(sb *core.SemanticsBuilder) {
	sb.Open(semantics.Node{Role: semantics.RoleImage, Label: "probe " + strconv.Itoa(r.id)}, r.Size())
	sb.Close()
}

// station is a row of the tables of widget cells. probe is its widget, made
// once as a caller keeps them ready.
type station struct {
	id    int
	name  string
	trend []float64
	probe core.Widget
	spark core.Widget
}

func stations(n int, log *probeLog) []station {
	out := make([]station, n)
	for i := range out {
		trend := make([]float64, 12)
		for k := range trend {
			trend[k] = math.Sin(float64(i+k) / 2)
		}
		out[i] = station{id: i, name: "Station " + strconv.Itoa(i), trend: trend, probe: cellProbe{id: i, log: log}}
		out[i].spark = control.Sparkline{Data: trend, Height: 20, Label: "Trend " + strconv.Itoa(i)}
	}
	return out
}

// stationColumns are Name (0 to 100, editable and sortable), Probe (100 to
// 220, a widget column) and Action (220 to 340, a button per row that adds
// the id of its row to pressed).
func stationColumns(pressed *[]int) []TableColumn[station] {
	return []TableColumn[station]{
		{Title: "Name", Width: 100, Text: func(s station) string { return s.name }, Editable: true, Sortable: true},
		{Title: "Probe", Width: 120, Kind: CellWidget, Sortable: true, Filterable: true,
			Text:   func(s station) string { return "trend " + strconv.Itoa(s.id) },
			Widget: func(s station) core.Widget { return s.probe }},
		{Title: "Action", Width: 120, Kind: CellWidget,
			Text: func(s station) string { return "action " + strconv.Itoa(s.id) },
			Widget: func(s station) core.Widget {
				return control.Button{Label: "Open " + strconv.Itoa(s.id), Size: control.ButtonSmall, OnPress: func() {
					*pressed = append(*pressed, s.id)
				}}
			}},
	}
}

// probeY returns where the probe of the row at index r paints, in a table
// scrolled by off: the probe is 10 tall in the middle of the row.
func probeY(r int, off float64) float32 {
	return tableHeaderHeight + float32(float64(r)*tableRowHeight-off) + (tableRowHeight-10)/2
}

// checkProbes fails unless the probes painted are those of the rows in view of
// rows, each once, in its row, where a row shows the station ids[row].
func checkProbes(t *testing.T, tree *core.Tree, log *probeLog, ids []int) {
	t.Helper()
	r := tableOf(t, tree)
	log.reset()
	wtest.Ops(tree, tableScreen)
	first, last := r.c.visibleRows()
	want := ids[first:last]
	if got := log.ids(); !slices.Equal(got, want) {
		t.Fatalf("painted %v, want %v (rows %d to %d)", got, want, first, last)
	}
	off := r.c.vert.Offset.Value()
	for k, p := range log.paints {
		row := first + k
		if p.at.Y != probeY(row, off) || p.at.X != r.c.colX(r.c.displayOf(1))+tablePadding {
			t.Errorf("row %d: the probe of %d paints at %v", row, p.id, p.at)
		}
	}
}

func identity(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func TestDataTable_WidgetCellsFollowTheScroll(t *testing.T) {
	log := &probeLog{}
	var pressed []int
	tree := tableTree(t, Table[station]{Rows: stations(1000, log), Columns: stationColumns(&pressed)})
	r := tableOf(t, tree)
	ids := identity(1000)
	checkProbes(t, tree, log, ids)
	// Fractions of a row, whole rows, a page and a jump far down: every row
	// object shows its row, never the widget of the row it showed before.
	for _, at := range []float64{5, 17, 32, 33, 100, 333, 320 + 7, 12_345, 31_000, 0} {
		r.c.vert.Jump(at)
		checkProbes(t, tree, log, ids)
	}
	// The wheel moves the offset over frames, with a glide.
	for range 3 {
		wtest.Wheel(tree, -2)
		now := time.Second
		for range 30 {
			wtest.Advance(tree, tableScreen, &now, 16*time.Millisecond)
			checkProbes(t, tree, log, ids)
		}
	}
}

func TestDataTable_WidgetCellsLayOut(t *testing.T) {
	log := &probeLog{}
	cols := stationColumns(new([]int))
	cols[1].Trailing = true
	tree := tableTree(t, Table[station]{Rows: stations(20, log), Columns: cols})
	wtest.Ops(tree, tableScreen)
	p := log.paints[0]
	// As wide as the column less the padding, at its end for Trailing.
	if p.size.W != 120-2*tablePadding || p.at.X != 100+tablePadding {
		t.Errorf("trailing probe %+v", p)
	}
	// A Small button keeps its size, at the start of its cell, in the middle
	// of the row.
	sem := tree.Semantics()
	open := wtest.ByLabel(t, sem, "Open 0")
	if open.Rect.X != 220+tablePadding || open.Rect.H != 24 || open.Rect.Y != tableHeaderHeight+(tableRowHeight-24)/2 {
		t.Errorf("button at %+v", open.Rect)
	}
	// A taller row gives the widget more room, less the inset.
	tree = tableTree(t, Table[station]{Rows: stations(20, log), Columns: cols, RowHeight: 48})
	log.reset()
	wtest.Ops(tree, tableScreen)
	if p := log.paints[0]; p.at.Y != tableHeaderHeight+(48-10)/2 {
		t.Errorf("probe in a tall row at %v", p.at)
	}
}

func TestDataTable_WidgetCellsSortAndFilter(t *testing.T) {
	log := &probeLog{}
	all := stations(300, log)
	rows := all
	var sorts []TableSort
	var filters []TableFilter
	var swap *wtest.SwapState
	sort := TableSort{}
	tree := tableTree(t, wtest.Swap{State: &swap, Build: func() core.Widget {
		return Table[station]{Rows: rows, Columns: stationColumns(new([]int)), Sort: sort,
			OnSort:   func(s TableSort) { sorts = append(sorts, s) },
			OnFilter: func(_ int, f TableFilter) { filters = append(filters, f) }}
	}})
	r := tableOf(t, tree)
	r.c.vert.Jump(40 * tableRowHeight)
	checkProbes(t, tree, log, identity(300))

	// A click on the header of the widget column asks for a sort; the caller
	// sorts by a key of the row, here the id backwards.
	var now uint32
	clickAt(tree, geom.Offset{X: 150, Y: 16}, 0, &now)
	if len(sorts) != 1 || sorts[0] != (TableSort{Column: 1, Order: SortAscending}) {
		t.Fatalf("sorts %v", sorts)
	}
	rows = slices.Clone(all)
	slices.Reverse(rows)
	sort = sorts[0]
	swap.SetState(func() {})
	ids := make([]int, len(rows))
	for i, s := range rows {
		ids[i] = s.id
	}
	checkProbes(t, tree, log, ids)

	// A filter keeps a few rows, fewer than fill the view: the scroll comes
	// back to the top and the probes show the rows that are left.
	rows = nil
	ids = ids[:0]
	for _, s := range all {
		if s.id%50 == 0 {
			rows = append(rows, s)
			ids = append(ids, s.id)
		}
	}
	swap.SetState(func() {})
	checkProbes(t, tree, log, ids)
	if r.c.vert.Offset.Value() != 0 {
		t.Errorf("offset %v with %d rows", r.c.vert.Offset.Value(), len(rows))
	}
	cells := wtest.RolesOf(tree.Semantics(), semantics.RoleGridCell)
	if len(cells) != 3*len(rows) {
		t.Errorf("%d cells for %d rows", len(cells), len(rows))
	}
	if len(filters) != 0 {
		t.Errorf("filters %v", filters)
	}
}

func TestDataTable_WidgetCellsRebuildWithFewerRows(t *testing.T) {
	log := &probeLog{}
	all := stations(1000, log)
	rows := all
	var swap *wtest.SwapState
	tree := tableTree(t, wtest.Swap{State: &swap, Build: func() core.Widget {
		return Table[station]{Rows: rows, Columns: stationColumns(new([]int))}
	}})
	r := tableOf(t, tree)
	r.c.vert.Jump(990 * tableRowHeight)
	checkProbes(t, tree, log, identity(1000))
	// Down to 25 rows while scrolled to the end of 1000: the very first
	// frame shows the last rows that are left, at their places.
	rows = all[:25]
	swap.SetState(func() {})
	log.reset()
	tree.Build()
	tree.Layout(geom.Tight(tableScreen))
	if tree.NeedsBuild() {
		t.Error("the widget cells built for the rows of the old offset")
	}
	checkProbes(t, tree, log, identity(25))
	if got, want := r.c.vert.Offset.Value(), float64(25*tableRowHeight-10*tableRowHeight); got != want {
		t.Errorf("offset %v, want %v", got, want)
	}
	// And to none.
	rows = nil
	swap.SetState(func() {})
	log.reset()
	wtest.Ops(tree, tableScreen)
	if len(log.paints) != 0 {
		t.Errorf("painted %v without rows", log.ids())
	}
}

func TestDataTable_WidgetCellsResizeAndMove(t *testing.T) {
	log := &probeLog{}
	ctrl := &TableController{}
	tree := tableTree(t, Table[station]{Rows: stations(50, log), Columns: stationColumns(new([]int)), Controller: ctrl})
	// The edge of Probe, from 220 to 300.
	drag(tree, geom.Offset{X: 221, Y: 16}, geom.Offset{X: 301, Y: 16})
	if got := ctrl.ColumnWidth(1); got != 200 {
		t.Fatalf("width %v", got)
	}
	log.reset()
	wtest.Ops(tree, tableScreen)
	if p := log.paints[0]; p.size.W != 200-2*tablePadding {
		t.Errorf("the probe is %v wide", p.size.W)
	}
	// Probe before Name: the widgets move with their column.
	ctrl.SetColumnOrder([]int{1, 0, 2})
	checkProbes(t, tree, log, identity(50))
	if p := log.paints[0]; p.at.X != tablePadding {
		t.Errorf("moved probe at %v", p.at)
	}
	cells := wtest.RolesOf(tree.Semantics(), semantics.RoleGridCell)
	if cells[0].Label != "trend 0" || cells[1].Label != "Station 0" {
		t.Errorf("cells %q %q", cells[0].Label, cells[1].Label)
	}
	if b := wtest.ByLabel(t, cells[0], "probe 0"); b.Role != semantics.RoleImage {
		t.Errorf("probe node %+v", b)
	}
}

func TestDataTable_WidgetCellsUnderFrozenColumns(t *testing.T) {
	log := &probeLog{}
	cols := stationColumns(new([]int))
	for i := range cols {
		cols[i].Width = 300
	}
	tree := tableTree(t, Table[station]{Rows: stations(30, log), Columns: cols, Frozen: 1})
	r := tableOf(t, tree)
	r.c.horz.Jump(250)
	log.reset()
	wtest.Ops(tree, tableScreen)
	// Probe scrolled to 50, under the frozen Name that ends at 300; the table
	// clips the columns that scroll to the right of the frozen ones.
	p := log.paints[0]
	if p.at.X != 50+tablePadding {
		t.Fatalf("probe at %v", p.at)
	}
	// A press on the frozen column over the probe hits the frozen cell.
	var now uint32
	clickAt(tree, geom.Offset{X: 100, Y: rowPos(1).Y}, 0, &now)
	if got := wtest.FocusedLabel(tree); got != "Station 1" {
		t.Errorf("focus on %q", got)
	}
}

func TestDataTable_WidgetCellsSelectAndActivate(t *testing.T) {
	log := &probeLog{}
	ctrl := &TableController{}
	activated := -1
	tree := tableTree(t, Table[station]{Rows: stations(50, log), Columns: stationColumns(new([]int)), Controller: ctrl,
		OnActivate: func(r int) { activated = r }})
	// The probe takes no press: a click on it selects its row like any cell.
	var now uint32
	at := geom.Offset{X: 160, Y: rowPos(3).Y}
	clickAt(tree, at, 0, &now)
	if !sameRanges(selection(ctrl), []RowRange{{3, 4}}) {
		t.Fatalf("selection %v", selection(ctrl))
	}
	if got := wtest.FocusedLabel(tree); got != "trend 3" {
		t.Errorf("focus on %q", got)
	}
	clickAt(tree, geom.Offset{X: 160, Y: rowPos(5).Y}, input.ModShift, &now)
	if !sameRanges(selection(ctrl), []RowRange{{3, 6}}) {
		t.Errorf("shift click %v", selection(ctrl))
	}
	// A double click on it activates the row.
	for range 2 {
		tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at, Time: 99_000})
		tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: at, Time: 99_000})
	}
	wtest.Ops(tree, tableScreen)
	if activated != 3 {
		t.Errorf("activated %d", activated)
	}
	// The selection paints under the widgets.
	col := tableOf(t, tree).c.th.Color
	ops := wtest.Ops(tree, tableScreen)
	ground := -1
	for k, op := range ops {
		if op.Kind == paint.OpRRect && op.Color == col.AccentTint && op.RRect.Rect.Y == tableHeaderHeight+3*tableRowHeight {
			ground = k
		}
	}
	if ground < 0 {
		t.Fatal("the selected row has no ground")
	}
}

func TestDataTable_ButtonInACell(t *testing.T) {
	var pressed []int
	ctrl := &TableController{}
	activated := -1
	tree := tableTree(t, Table[station]{Rows: stations(50, nil), Columns: stationColumns(&pressed), Controller: ctrl,
		OnActivate: func(r int) { activated = r }})
	var now uint32
	clickAt(tree, rowPos(1), 0, &now)
	if !sameRanges(selection(ctrl), []RowRange{{1, 2}}) {
		t.Fatalf("selection %v", selection(ctrl))
	}
	// The button takes its press: it runs, and the selection stays.
	open := wtest.ByLabel(t, tree.Semantics(), "Open 4")
	clickAt(tree, wtest.Center(open.Rect), 0, &now)
	if !slices.Equal(pressed, []int{4}) {
		t.Fatalf("pressed %v", pressed)
	}
	if !sameRanges(selection(ctrl), []RowRange{{1, 2}}) {
		t.Errorf("the button changed the selection to %v", selection(ctrl))
	}
	// It has the keyboard, and its cell is the cell in focus.
	if got := wtest.FocusedLabel(tree); got != "Open 4" {
		t.Errorf("focus on %q", got)
	}
	if row, col := ctrl.FocusedCell(); row != 4 || col != 2 {
		t.Errorf("focused cell %d, %d", row, col)
	}
	// Space presses it; a double click on it does not activate the row.
	tableKey(tree, input.KeySpace, 0)
	if !slices.Equal(pressed, []int{4, 4}) {
		t.Errorf("Space: pressed %v", pressed)
	}
	at := wtest.Center(open.Rect)
	for range 2 {
		tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at, Time: 99_000})
		tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: at, Time: 99_000})
	}
	wtest.Ops(tree, tableScreen)
	if activated != -1 {
		t.Errorf("activated %d", activated)
	}
	// Escape gives the keyboard back to the cell, and Enter into the button.
	tableKey(tree, input.KeyEscape, 0)
	if got := wtest.FocusedLabel(tree); got != "action 4" {
		t.Errorf("Escape: focus on %q", got)
	}
	tableKey(tree, input.KeyEnter, 0)
	if got := wtest.FocusedLabel(tree); got != "Open 4" {
		t.Errorf("Enter: focus on %q", got)
	}
	// An arrow that the button leaves goes on to the next cell.
	tableKey(tree, input.KeyLeft, 0)
	if got := wtest.FocusedLabel(tree); got != "trend 4" {
		t.Errorf("Left: focus on %q", got)
	}
	// Enter on a cell whose widget takes no keyboard activates the row.
	tableKey(tree, input.KeyEnter, 0)
	if activated != 4 {
		t.Errorf("Enter on the probe: activated %d", activated)
	}
	tableKey(tree, input.KeyRight, 0)
	tableKey(tree, input.KeyF2, 0)
	if got := wtest.FocusedLabel(tree); got != "Open 4" {
		t.Errorf("F2: focus on %q", got)
	}
	tableKey(tree, input.KeyDown, 0)
	if got := wtest.FocusedLabel(tree); got != "action 5" {
		t.Errorf("Down: focus on %q", got)
	}
}

func TestDataTable_ButtonLeavesWithItsRow(t *testing.T) {
	tree := tableTree(t, Table[station]{Rows: stations(200, nil), Columns: stationColumns(new([]int))})
	r := tableOf(t, tree)
	var now uint32
	open := wtest.ByLabel(t, tree.Semantics(), "Open 2")
	clickAt(tree, wtest.Center(open.Rect), 0, &now)
	if got := wtest.FocusedLabel(tree); got != "Open 2" {
		t.Fatalf("focus on %q", got)
	}
	// Scrolled away, the row object shows another row: the keyboard goes to
	// the table rather than stay on a button of another row.
	r.c.vert.Jump(50 * tableRowHeight)
	wtest.Ops(tree, tableScreen)
	if f := tree.Focused(); core.RenderObject(f) != core.RenderObject(r) {
		t.Errorf("focus on %T %q", f, wtest.FocusedLabel(tree))
	}
	// Back in view, the arrows find the cell again.
	tableKey(tree, input.KeyDown, 0)
	if got := wtest.FocusedLabel(tree); got != "action 3" {
		t.Errorf("Down: focus on %q", got)
	}
}

func TestDataTable_TabVisitsTheWidgetsOfTheRowsInOrder(t *testing.T) {
	tree := tableTree(t, Table[station]{Rows: stations(200, nil), Columns: stationColumns(new([]int))})
	r := tableOf(t, tree)
	// From row 8 the rows wrap around the row objects: row 12 has the first.
	r.c.vert.Jump(8 * tableRowHeight)
	wtest.Ops(tree, tableScreen)
	var now uint32
	clickAt(tree, geom.Offset{X: 50, Y: tableHeaderHeight + 3*tableRowHeight + 16}, 0, &now)
	if got := wtest.FocusedLabel(tree); got != "Station 11" {
		t.Fatalf("focus on %q", got)
	}
	for _, want := range []string{"Open 11", "Open 12", "Open 13"} {
		tableKey(tree, input.KeyTab, 0)
		if got := wtest.FocusedLabel(tree); got != want {
			t.Fatalf("Tab: focus on %q, want %q", got, want)
		}
	}
}

// TestDataTable_TabSkipsTheRowObjectsLeftOver filters many rows down to one:
// the row objects that show no row keep the buttons they built, hidden, and
// Tab must not stop on them.
func TestDataTable_TabSkipsTheRowObjectsLeftOver(t *testing.T) {
	all := stations(20, nil)
	rows := all
	var swap *wtest.SwapState
	tree := tableTree(t, wtest.Swap{State: &swap, Build: func() core.Widget {
		return Table[station]{Rows: rows, Columns: stationColumns(new([]int))}
	}})
	r := tableOf(t, tree)
	rows = all[:1]
	swap.SetState(func() {})
	wtest.Ops(tree, tableScreen)
	var now uint32
	clickAt(tree, geom.Offset{X: 50, Y: rowPos(0).Y}, 0, &now)
	if got := wtest.FocusedLabel(tree); got != "Station 0" {
		t.Fatalf("focus on %q", got)
	}
	var seen []string
	for range 6 {
		tableKey(tree, input.KeyTab, 0)
		f := tree.Focused()
		seen = append(seen, wtest.FocusedLabel(tree))
		for ro := core.RenderObject(f); ro != nil; ro = ro.Parent() {
			if row, ok := ro.(*renderTableRow); ok && row.r < 0 {
				t.Fatalf("Tab stopped on %T in a row object that shows no row; visited %q", f, seen)
			}
		}
		if f == core.FocusTarget(r.focusCell()) {
			break
		}
	}
}

func TestDataTable_EditingNextToWidgetCells(t *testing.T) {
	log := &probeLog{}
	var edits []TableEdit
	tree := tableTree(t, Table[station]{Rows: stations(30, log), Columns: stationColumns(new([]int)),
		OnCommit: func(e TableEdit) bool { edits = append(edits, e); return true }})
	r := tableOf(t, tree)
	var now uint32
	// The widget columns are never edited, even when Editable.
	clickAt(tree, geom.Offset{X: 160, Y: rowPos(2).Y}, 0, &now)
	tableKey(tree, input.KeyF2, 0)
	tree.DispatchText("x")
	wtest.Ops(tree, tableScreen)
	if r.c.editing {
		t.Fatal("a widget cell opened an editor")
	}
	if r.c.models[1].editable || r.c.models[2].editable {
		t.Error("a widget column is editable")
	}
	// Name, next to it, edits; the widgets go on painting meanwhile.
	tableKey(tree, input.KeyLeft, 0)
	tableKey(tree, input.KeyF2, 0)
	if !r.c.editing {
		t.Fatal("no editor")
	}
	log.reset()
	wtest.Ops(tree, tableScreen)
	if first, last := r.c.visibleRows(); len(log.paints) != last-first {
		t.Errorf("%d probes while editing, %d rows in view", len(log.paints), last-first)
	}
	tree.DispatchText("!")
	wtest.Ops(tree, tableScreen)
	// Tab skips the widget columns and goes to the Name of the next row.
	tableKey(tree, input.KeyTab, 0)
	if len(edits) != 1 || edits[0].Row != 2 || edits[0].Column != 0 || edits[0].Text != "Station 2!" {
		t.Fatalf("edits %v", edits)
	}
	if !r.c.editing || r.c.editRow != 3 || r.c.editCol != 0 {
		t.Errorf("editing %v at %d, %d", r.c.editing, r.c.editRow, r.c.editCol)
	}
	// A press on a button in another row commits the edit, as the editor
	// loses the keyboard.
	tree.DispatchText("?")
	wtest.Ops(tree, tableScreen)
	open := wtest.ByLabel(t, tree.Semantics(), "Open 6")
	clickAt(tree, wtest.Center(open.Rect), 0, &now)
	if r.c.editing || len(edits) != 2 || edits[1].Text != "Station 3?" {
		t.Errorf("editing %v, edits %v", r.c.editing, edits)
	}
}

func TestDataTable_WidgetCellSemantics(t *testing.T) {
	tree := tableTree(t, Table[station]{Label: "Stations", Rows: stations(100, nil), Columns: stationColumns(new([]int))})
	r := tableOf(t, tree)
	r.c.vert.Jump(20 * tableRowHeight)
	wtest.Ops(tree, tableScreen)
	grid := wtest.RolesOf(tree.Semantics(), semantics.RoleGrid)[0]
	if grid.ColumnCount != 3 || grid.RowCount != 101 {
		t.Errorf("grid %+v", grid)
	}
	rows := wtest.RolesOf(grid, semantics.RoleRow)
	row := rows[1]
	if row.Label != "Station 20 trend 20 action 20" || row.RowIndex != 22 {
		t.Errorf("row %q %d", row.Label, row.RowIndex)
	}
	cells := wtest.RolesOf(row, semantics.RoleGridCell)
	if len(cells) != 3 || cells[1].Label != "trend 20" || cells[1].ColumnIndex != 2 || cells[1].RowIndex != 22 {
		t.Fatalf("cells %+v", cells)
	}
	// The cell holds the node of its widget: the image of the sparkline, the
	// button.
	img := wtest.RolesOf(cells[1], semantics.RoleImage)
	if len(img) != 1 || img[0].Label != "probe 20" {
		t.Errorf("image %+v", img)
	}
	if b := wtest.ByLabel(t, cells[2], "Open 20"); b.Role != semantics.RoleButton || b.Actions&semantics.ActionTap == 0 {
		t.Errorf("button %+v", b)
	}
	// A screen reader's tap on the button presses it.
	var pressed []int
	tree = tableTree(t, Table[station]{Rows: stations(10, nil), Columns: stationColumns(&pressed)})
	root, _ := tree.SemanticsTree()
	b := wtest.ByLabel(t, root, "Open 3")
	tree.PerformSemanticsAction(b.ID, semantics.ActionTap, core.SemanticsArg{})
	if !slices.Equal(pressed, []int{3}) {
		t.Errorf("pressed %v", pressed)
	}
}

func TestDataTable_SparklineCells(t *testing.T) {
	rows := stations(500, nil)
	tree := tableTree(t, Table[station]{Rows: rows, Columns: []TableColumn[station]{
		{Title: "Name", Width: 100, Text: func(s station) string { return s.name }},
		{Title: "Trend", Width: 160, Kind: CellWidget, Widget: func(s station) core.Widget { return s.spark }},
	}})
	r := tableOf(t, tree)
	r.c.vert.Jump(100 * tableRowHeight)
	ops := wtest.Ops(tree, tableScreen)
	// Each row in view has its line, as a path inside its cell.
	lines := 0
	for _, op := range ops {
		if op.Kind == paint.OpPath && op.Origin.X >= 100 && op.Origin.X < 260 {
			lines++
		}
	}
	if lines < 10 {
		t.Errorf("%d lines", lines)
	}
	cells := wtest.RolesOf(tree.Semantics(), semantics.RoleGridCell)
	if img := wtest.RolesOf(cells[1], semantics.RoleImage); len(img) != 1 || img[0].Label != "Trend 100" {
		t.Errorf("image %+v", img)
	}
	// Without Text the cell has no name of its own; the image speaks.
	if cells[1].Label != "" {
		t.Errorf("cell %q", cells[1].Label)
	}
}

func TestDataTable_WidgetCellsZeroValue(t *testing.T) {
	// A widget column without Widget, or with one that returns nil, leaves
	// its cells empty.
	tree := tableTree(t, Table[int]{Rows: []int{1, 2, 3}, Columns: []TableColumn[int]{
		{Kind: CellWidget},
		{Kind: CellWidget, Widget: func(int) core.Widget { return nil }},
	}})
	wtest.Ops(tree, tableScreen)
	cells := wtest.RolesOf(tree.Semantics(), semantics.RoleGridCell)
	if len(cells) != 6 {
		t.Errorf("%d cells", len(cells))
	}
	var now uint32
	clickAt(tree, rowPos(1), 0, &now)
	tableKey(tree, input.KeyEnter, 0)
	tableKey(tree, input.KeyEscape, 0)
	if CellWidget.String() != "widget" {
		t.Errorf("String %q", CellWidget.String())
	}
}

// sparkTable is a table of 20k rows with a frozen name and two columns of
// sparklines, a line and bars, whose widgets are made once, as a caller keeps
// them ready.
func sparkTable() Table[int] {
	const n = 20_000
	series := make([][]float64, 64)
	for k := range series {
		series[k] = make([]float64, 24)
		for i := range series[k] {
			series[k][i] = math.Sin(float64(i*(k+1)) / 5)
		}
	}
	names := make([]string, 512)
	for i := range names {
		names[i] = "Item " + strconv.Itoa(i)
	}
	lines, bars := make([]core.Widget, n), make([]core.Widget, n)
	rows := make([]int, n)
	for i := range rows {
		rows[i] = i
		lines[i] = control.Sparkline{Data: series[i%len(series)], Fill: true, MarkLast: true}
		bars[i] = control.Sparkline{Kind: control.SparklineBars, Data: series[(i*7)%len(series)]}
	}
	return Table[int]{Rows: rows, Frozen: 1, Columns: []TableColumn[int]{
		{Title: "Name", Width: 140, Text: func(i int) string { return names[i%len(names)] }},
		{Title: "Trend", Width: 160, Kind: CellWidget, Widget: func(i int) core.Widget { return lines[i] }},
		{Title: "Change", Width: 140, Kind: CellWidget, Widget: func(i int) core.Widget { return bars[i] }},
		{Title: "Owner", Width: 140, Text: func(i int) string { return names[(i*7)%len(names)] }},
	}}
}

// probeTable is sparkTable with probes, whose widgets build nothing, in
// place of the sparklines.
func probeTable() Table[int] {
	const n = 20_000
	probes := make([]core.Widget, n)
	rows := make([]int, n)
	for i := range rows {
		rows[i] = i
		probes[i] = cellProbe{id: i}
	}
	return Table[int]{Rows: rows, Columns: []TableColumn[int]{
		{Title: "Name", Width: 140, Text: func(int) string { return "Item" }},
		{Title: "Probe", Width: 160, Kind: CellWidget, Widget: func(i int) core.Widget { return probes[i] }},
	}}
}

func TestDataTable_ScrollingWidgetCellsDoesNotAllocate(t *testing.T) {
	t.Run("sparklines, within a row", func(t *testing.T) {
		tree := tableTree(t, sparkTable())
		r := tableOf(t, tree)
		var cv paint.Canvas
		now := time.Second
		base := float64(10_000 * tableRowHeight)
		k := 0
		// A frame that brings no new row into view builds nothing.
		step := func() {
			k++
			r.c.vert.Jump(base + float64(k%tableRowHeight))
			now += 8 * time.Millisecond
			tableFrame(tree, &cv, now)
		}
		r.c.vert.Jump(base)
		for range 100 {
			step()
		}
		if allocs := testing.AllocsPerRun(100, step); allocs != 0 {
			t.Errorf("a scroll step allocates %v times", allocs)
		}
		if tree.NeedsBuild() {
			t.Error("the scroll left a build behind")
		}
	})
	t.Run("widgets that build nothing, across rows", func(t *testing.T) {
		// Rows come into view at every step: the table itself allocates
		// nothing to build their cells again.
		tree := tableTree(t, probeTable())
		r := tableOf(t, tree)
		var cv paint.Canvas
		now := time.Second
		base := float64(10_000 * tableRowHeight)
		k := 0
		step := func() {
			k++
			r.c.vert.Jump(base + float64(k*11%(64*tableRowHeight)))
			now += 8 * time.Millisecond
			tableFrame(tree, &cv, now)
		}
		r.c.vert.Jump(base)
		for range 400 {
			step()
		}
		if allocs := testing.AllocsPerRun(100, step); allocs != 0 {
			t.Errorf("a scroll step allocates %v times", allocs)
		}
	})
}

// BenchmarkDataTable_ScrollStep_Sparklines is BenchmarkDataTable_ScrollStep_1M
// over sparkTable: a third of a row at a time, so that a row comes into view
// every third frame and its two sparklines build. A Sparkline builds its
// render widget anew, which allocates; the frames between allocate nothing.
func BenchmarkDataTable_ScrollStep_Sparklines(b *testing.B) {
	tree := tableTree(b, sparkTable())
	r := tableOf(b, tree)
	var cv paint.Canvas
	now := time.Second
	base := float64(10_000 * tableRowHeight)
	r.c.vert.Jump(base)
	for i := range 200 {
		r.c.vert.Jump(base + float64(i*11%(64*tableRowHeight)))
		now += 8 * time.Millisecond
		tableFrame(tree, &cv, now)
	}
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		i++
		r.c.vert.Jump(base + float64(i*11%(64*tableRowHeight)))
		now += 8 * time.Millisecond
		tableFrame(tree, &cv, now)
	}
}

// BenchmarkDataTable_ScrollStep_SparklinesInRow is
// BenchmarkDataTable_ScrollStep_Sparklines with steps that stay within a row,
// so that no frame builds: it should allocate nothing.
func BenchmarkDataTable_ScrollStep_SparklinesInRow(b *testing.B) {
	tree := tableTree(b, sparkTable())
	r := tableOf(b, tree)
	var cv paint.Canvas
	now := time.Second
	base := float64(10_000 * tableRowHeight)
	r.c.vert.Jump(base)
	for i := range 100 {
		r.c.vert.Jump(base + float64(i%tableRowHeight))
		now += 8 * time.Millisecond
		tableFrame(tree, &cv, now)
	}
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		i++
		r.c.vert.Jump(base + float64(i%tableRowHeight))
		now += 8 * time.Millisecond
		tableFrame(tree, &cv, now)
	}
}
