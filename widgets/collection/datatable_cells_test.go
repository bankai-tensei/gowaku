package collection

import (
	"strconv"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

// device is a row of the tables of cell kinds.
type device struct {
	name, status string
	ok           bool
	battery      float32
	battLabel    string
	synced       bool
}

func devices(n int) []device {
	out := make([]device, n)
	for i := range out {
		out[i] = device{
			name: "Device " + strconv.Itoa(i), ok: i%2 == 0, battery: float32(i%5) / 4,
			battLabel: strconv.Itoa(i%5*25) + "%", synced: i%3 == 0,
		}
		out[i].status = "Online"
		if !out[i].ok {
			out[i].status = "Offline"
		}
	}
	return out
}

// pink is a color of the caller's, which no token of the theme has.
var pink = geom.Hex(0xe0409a)

// deviceColumns are a column of each kind: name, status badge, dot, bar and
// check box, 100 wide each.
func deviceColumns(onCheck func(int, bool)) []TableColumn[device] {
	tone := func(d device) CellTone {
		if d.ok {
			return CellToneSuccess
		}
		return CellToneDanger
	}
	return []TableColumn[device]{
		{Title: "Name", Width: 100, Text: func(d device) string { return d.name }},
		{Title: "Status", Width: 100, Kind: CellBadge, Text: func(d device) string { return d.status }, Tone: tone},
		{Title: "Link", Width: 100, Kind: CellDot, Text: func(d device) string { return d.status },
			Color: func(device) geom.Color { return pink }},
		{Title: "Battery", Width: 100, Kind: CellProgress, Value: func(d device) float32 { return d.battery }},
		{Title: "Synced", Width: 100, Kind: CellCheck, Checked: func(d device) bool { return d.synced }, OnCheck: onCheck,
			Editable: true},
	}
}

// rrectsIn returns the filled rounded rectangles of ops inside box.
func rrectsIn(list []paint.Op, box geom.Rect) []paint.Op {
	var out []paint.Op
	for _, op := range list {
		r := op.RRect.Rect
		if op.Kind == paint.OpRRect && r.X >= box.X && r.Y >= box.Y && r.X+r.W <= box.X+box.W && r.Y+r.H <= box.Y+box.H {
			out = append(out, op)
		}
	}
	return out
}

// cellBox returns the box of the cell of row r and column j of the device
// table, while it is at the top.
func cellBox(r, j int) geom.Rect {
	return geom.Rect{X: float32(j) * 100, Y: tableHeaderHeight + float32(r)*tableRowHeight, W: 100, H: tableRowHeight}
}

func TestDataTable_CellKindsPaint(t *testing.T) {
	tree := tableTree(t, Table[device]{Rows: devices(10), Columns: deviceColumns(nil), Select: TableSelectNone})
	col := tableOf(t, tree).c.th.Color
	list := wtest.Ops(tree, tableScreen)

	t.Run("badge", func(t *testing.T) {
		for r, want := range []geom.Color{col.Success, col.Danger} {
			got := rrectsIn(list, cellBox(r, 1))
			if len(got) != 1 {
				t.Fatalf("row %d: %d shapes", r, len(got))
			}
			pill := got[0]
			if pill.Color != want.WithAlpha(want.A*tableTint) || pill.RRect.Rect.H != tableBadgeHeight || pill.RRect.TopLeft != tableBadgeHeight/2 {
				t.Errorf("row %d: pill %+v", r, pill)
			}
			if pill.RRect.Rect.X != 100+tablePadding || pill.RRect.Rect.W <= 2*tableBadgePad {
				t.Errorf("row %d: the pill sits at %+v", r, pill.RRect.Rect)
			}
		}
	})
	t.Run("dot", func(t *testing.T) {
		got := rrectsIn(list, cellBox(0, 2))
		if len(got) != 1 || got[0].Color != pink || got[0].RRect.Rect.W != tableDot || got[0].RRect.Rect.X != 200+tablePadding {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("progress", func(t *testing.T) {
		room := float32(100 - 2*tablePadding)
		// Row 2 is half full: the track and the bar over half of it.
		got := rrectsIn(list, cellBox(2, 3))
		if len(got) != 2 || got[0].Color != col.FillStrong || got[0].RRect.Rect.W != room || got[0].RRect.Rect.H != tableBarHeight {
			t.Fatalf("got %+v", got)
		}
		if got[1].Color != col.Accent || got[1].RRect.Rect.W != room/2 {
			t.Errorf("bar %+v", got[1])
		}
		if got := rrectsIn(list, cellBox(0, 3)); len(got) != 1 {
			t.Errorf("an empty bar draws %d shapes", len(got))
		}
	})
	t.Run("check", func(t *testing.T) {
		on := rrectsIn(list, cellBox(0, 4))
		if len(on) != 1 || on[0].Color != col.Accent || on[0].RRect.Rect.W != kit.CheckboxBox || on[0].RRect.Rect.X != 400+tablePadding {
			t.Fatalf("ticked %+v", on)
		}
		off := rrectsIn(list, cellBox(1, 4))
		if len(off) != 1 || off[0].Color.A != 0 || off[0].BorderWidth != 1 || off[0].BorderColor != col.Ink3 {
			t.Fatalf("clear %+v", off)
		}
		ticks := 0
		for _, op := range list {
			if op.Kind == paint.OpPath && op.Origin.X == 400+tablePadding && op.Origin.Y > cellBox(0, 4).Y && op.Origin.Y < cellBox(0, 4).Y+tableRowHeight {
				ticks++
			}
		}
		if ticks != 1 {
			t.Errorf("%d ticks in the ticked box", ticks)
		}
	})
}

func TestDataTable_CellKindsDefaultColors(t *testing.T) {
	cols := []TableColumn[device]{
		{Title: "Status", Width: 100, Kind: CellBadge, Text: func(d device) string { return d.status }},
		{Title: "Link", Width: 100, Kind: CellDot},
		{Title: "Battery", Width: 100, Kind: CellProgress, Value: func(device) float32 { return 1 },
			Tone: func(device) CellTone { return CellToneDefault }},
		{Title: "Synced", Width: 100, Kind: CellCheck, Checked: func(device) bool { return true },
			Color: func(device) geom.Color { return geom.Color{} }},
	}
	tree := tableTree(t, Table[device]{Rows: devices(3), Columns: cols, Select: TableSelectNone})
	col := tableOf(t, tree).c.th.Color
	list := wtest.Ops(tree, tableScreen)
	want := []geom.Color{col.FillStrong, col.Ink3, col.Accent, col.Accent}
	for j, w := range want {
		got := rrectsIn(list, cellBox(0, j))
		if len(got) == 0 || got[len(got)-1].Color != w {
			t.Errorf("column %d: got %+v, want the color %v", j, got, w)
		}
	}
}

func TestDataTable_CellKindsLongTextStaysInTheCell(t *testing.T) {
	long := "A status far too long for the cell it is in"
	cols := []TableColumn[device]{
		{Title: "Status", Width: 100, Kind: CellBadge, Text: func(device) string { return long }, Trailing: true},
		{Title: "Link", Width: 100, Kind: CellDot, Text: func(device) string { return long }},
		{Title: "Battery", Width: 100, Kind: CellProgress, Text: func(device) string { return long }},
		{Title: "Synced", Width: 100, Kind: CellCheck, Text: func(device) string { return long }, Trailing: true},
	}
	tree := tableTree(t, Table[device]{Rows: devices(3), Columns: cols, Select: TableSelectNone})
	list := wtest.Ops(tree, tableScreen)
	for j := range cols {
		box := cellBox(0, j)
		for _, op := range rrectsIn(list, box) {
			r := op.RRect.Rect
			if r.X < box.X+tablePadding || r.X+r.W > box.X+box.W-tablePadding {
				t.Errorf("column %d: a shape leaves the padding: %+v", j, r)
			}
		}
	}
}

func TestDataTable_CheckBoxToggles(t *testing.T) {
	ctrl := &TableController{}
	type check struct {
		row     int
		checked bool
	}
	var got []check
	rows := devices(10)
	var activated int
	tree := tableTree(t, Table[device]{Rows: rows, Controller: ctrl, OnCommit: func(TableEdit) bool { return true },
		OnActivate: func(int) { activated++ },
		Columns:    deviceColumns(func(row int, checked bool) { got = append(got, check{row, checked}) })})
	var now uint32
	clickAt(tree, rowPos(5), 0, &now)
	box := cellBox(3, 4)
	clickAt(tree, geom.Offset{X: box.X + tablePadding + kit.CheckboxBox/2, Y: box.Y + box.H/2}, 0, &now)
	if len(got) != 1 || got[0] != (check{3, !rows[3].synced}) {
		t.Fatalf("checks %v", got)
	}
	if s := selection(ctrl); !sameRanges(s, []RowRange{{5, 6}}) {
		t.Errorf("a tick changed the selection to %v", s)
	}
	if r, c := ctrl.FocusedCell(); r != 3 || c != 4 {
		t.Errorf("focus on %d, %d", r, c)
	}

	// Away from the box, a press selects as anywhere else.
	clickAt(tree, geom.Offset{X: box.X + 80, Y: box.Y + box.H/2}, 0, &now)
	if len(got) != 1 {
		t.Errorf("a press beside the box ticks it: %v", got)
	}
	if s := selection(ctrl); !sameRanges(s, []RowRange{{3, 4}}) {
		t.Errorf("selection %v", s)
	}

	// Space ticks the box in focus, and Shift+Space still selects.
	tableKey(tree, input.KeySpace, 0)
	if len(got) != 2 || got[1] != (check{3, !rows[3].synced}) {
		t.Fatalf("checks after Space %v", got)
	}
	tableKey(tree, input.KeyDown, input.ModControl)
	tableKey(tree, input.KeySpace, input.ModShift)
	if len(got) != 2 {
		t.Errorf("Shift+Space ticks: %v", got)
	}
	if s := selection(ctrl); !sameRanges(s, []RowRange{{3, 5}}) {
		t.Errorf("selection after Shift+Space %v", s)
	}

	// A check column is never edited, even when Editable: Enter activates.
	tableKey(tree, input.KeyEnter, 0)
	if tableOf(t, tree).c.editing || activated != 1 {
		t.Errorf("editing %v, activated %d", tableOf(t, tree).c.editing, activated)
	}
}

func TestDataTable_CheckBoxWithoutOnCheckOnlyShows(t *testing.T) {
	ctrl := &TableController{}
	tree := tableTree(t, Table[device]{Rows: devices(10), Controller: ctrl, Columns: deviceColumns(nil)})
	var now uint32
	box := cellBox(3, 4)
	clickAt(tree, geom.Offset{X: box.X + tablePadding + kit.CheckboxBox/2, Y: box.Y + box.H/2}, 0, &now)
	if s := selection(ctrl); !sameRanges(s, []RowRange{{3, 4}}) {
		t.Errorf("a press on a box that only shows selects: %v", s)
	}
	n := byRoleLabel(t, tree.Semantics(), semantics.RoleCheckbox, "Synced")
	if n.State&semantics.ReadOnly == 0 || n.Actions&semantics.ActionTap != 0 {
		t.Errorf("box %+v", n)
	}
}

// byRoleLabel returns the first node of role r labeled label.
func byRoleLabel(t *testing.T, root *semantics.Node, r semantics.Role, label string) *semantics.Node {
	t.Helper()
	var found *semantics.Node
	root.Walk(func(n *semantics.Node) bool {
		if n.Role == r && n.Label == label {
			found = n
		}
		return found == nil
	})
	if found == nil {
		t.Fatalf("no %v %q", r, label)
	}
	return found
}

func TestDataTable_CellKindsSemantics(t *testing.T) {
	var got []int
	rows := devices(10)
	tree := tableTree(t, Table[device]{Rows: rows, Columns: deviceColumns(func(row int, _ bool) { got = append(got, row) })})
	grid := wtest.FindRole(tree.Semantics(), semantics.RoleGrid)
	all := wtest.RolesOf(grid, semantics.RoleRow)
	// Row 0: synced, battery empty. Row 2: half full, not synced.
	cells := wtest.RolesOf(all[1], semantics.RoleGridCell)
	if len(cells) != 5 || cells[1].Label != "Online" || cells[2].Label != "Online" {
		t.Fatalf("cells %+v", cells)
	}
	if cells[3].Label != "0%" || cells[3].Range.Max != 100 || cells[3].Range.Value != 0 {
		t.Errorf("bar %+v", cells[3])
	}
	if half := wtest.RolesOf(all[3], semantics.RoleGridCell)[3]; half.Label != "50%" || half.Range.Value != 50 {
		t.Errorf("half bar %+v", half)
	}
	box := wtest.FindRole(cells[4], semantics.RoleCheckbox)
	if box == nil || box.Label != "Synced" || box.State&semantics.Checked == 0 || box.Actions&semantics.ActionTap == 0 {
		t.Fatalf("box %+v", box)
	}
	if box.Rect.X != 400+tablePadding || box.Rect.W != kit.CheckboxBox {
		t.Errorf("the box is at %+v", box.Rect)
	}
	if cells[4].Label != "Synced" {
		t.Errorf("a ticked cell without text reads %q", cells[4].Label)
	}
	if want := "Device 0 Online Online 0% Synced"; all[1].Label != want {
		t.Errorf("row %q, want %q", all[1].Label, want)
	}
	if clear := wtest.RolesOf(all[2], semantics.RoleGridCell)[4]; clear.Label != "" {
		t.Errorf("a clear cell reads %q", clear.Label)
	}
	tree.PerformSemanticsAction(nodeOf(tree, semantics.RoleCheckbox, "Synced"), semantics.ActionTap, core.SemanticsArg{})
	if len(got) != 1 || got[0] != 0 {
		t.Errorf("a tap on the box gives %v", got)
	}
}

// cellKindRows is a table of a million rows with a column of each kind.
func cellKindRows() Table[int] {
	names := make([]string, 512)
	for i := range names {
		names[i] = "Item " + strconv.Itoa(i)
	}
	statuses := []string{"Online", "Offline", "Charging", "Idle"}
	rows := make([]int, 1_000_000)
	for i := range rows {
		rows[i] = i
	}
	tone := func(i int) CellTone { return CellTone(i%4 + 1) }
	return Table[int]{Rows: rows, Frozen: 1, Columns: []TableColumn[int]{
		{Title: "Name", Width: 160, Text: func(i int) string { return names[i%len(names)] }},
		{Title: "Status", Width: 120, Kind: CellBadge, Text: func(i int) string { return statuses[i%4] }, Tone: tone},
		{Title: "Link", Width: 120, Kind: CellDot, Text: func(i int) string { return statuses[i%4] }, Tone: tone},
		{Title: "Battery", Width: 160, Kind: CellProgress, Value: func(i int) float32 { return float32(i%101) / 100 },
			Text: func(i int) string { return names[i%101] }},
		{Title: "Synced", Width: 100, Kind: CellCheck, Checked: func(i int) bool { return i%3 == 0 }, OnCheck: func(int, bool) {}},
	}}
}

func TestDataTable_ScrollingCellKindsDoesNotAllocate(t *testing.T) {
	tree := tableTree(t, cellKindRows())
	r := tableOf(t, tree)
	var cv paint.Canvas
	now := time.Second
	step := func() {
		at := r.c.vert.Offset.Value() + 11
		if at > 16_000_000+64*tableRowHeight {
			at = 16_000_000
		}
		r.c.vert.Jump(at)
		now += 8 * time.Millisecond
		tableFrame(tree, &cv, now)
	}
	r.c.vert.Jump(16_000_000)
	for range 400 {
		step()
	}
	if allocs := testing.AllocsPerRun(100, step); allocs != 0 {
		t.Errorf("a scroll step allocates %v times", allocs)
	}
}

// BenchmarkDataTable_ScrollStep_CellKinds_1M is BenchmarkDataTable_ScrollStep_1M
// over a column of each kind of cell.
func BenchmarkDataTable_ScrollStep_CellKinds_1M(b *testing.B) {
	tree := tableTree(b, cellKindRows())
	r := tableOf(b, tree)
	var cv paint.Canvas
	now := time.Second
	base := float64(16_000_000)
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

func TestCellKind_String(t *testing.T) {
	for k, want := range map[CellKind]string{CellText: "text", CellBadge: "badge", CellDot: "dot", CellProgress: "progress", CellCheck: "check"} {
		if got := k.String(); got != want {
			t.Errorf("%d: %q", k, got)
		}
	}
	for c, want := range map[CellTone]string{CellToneDefault: "default", CellToneNeutral: "neutral", CellToneAccent: "accent",
		CellToneSuccess: "success", CellToneWarning: "warning", CellToneDanger: "danger"} {
		if got := c.String(); got != want {
			t.Errorf("%d: %q", c, got)
		}
	}
}
