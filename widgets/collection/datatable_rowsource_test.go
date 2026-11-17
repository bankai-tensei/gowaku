package collection

import (
	"runtime"
	"strconv"
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

// logLine is a row that the table reads through Row, as a viewer of a file
// reads the line at an index of its own.
type logLine struct {
	n     int
	level string
}

// logTable is a table of n rows that exist only as indexes: Row makes each
// one as it is asked for, and counts the calls.
func logTable(n int, calls *int) Table[logLine] {
	return Table[logLine]{
		Label:    "Log",
		RowCount: n,
		Row: func(i int) logLine {
			*calls++
			level := "INFO"
			if i%1000 == 0 {
				level = "ERROR"
			}
			return logLine{n: i, level: level}
		},
		Columns: []TableColumn[logLine]{
			{Title: "Line", Width: 120, Text: func(l logLine) string { return strconv.Itoa(l.n) }, Trailing: true},
			{Title: "Level", Width: 100, Text: func(l logLine) string { return l.level }},
		},
	}
}

func heapNow() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// TestTable_RowSourceHoldsNoRows shows a hundred million rows through Row: the
// table keeps no slice of them, asks Row only for rows it shows, and tells a
// screen reader the number of all of them.
func TestTable_RowSourceHoldsNoRows(t *testing.T) {
	const n = 100_000_000
	calls := 0
	before := heapNow()
	tree := tableTree(t, logTable(n, &calls))
	if grew := int64(heapNow()) - int64(before); grew > 16<<20 {
		t.Errorf("the table grew the heap by %d MB for rows it does not hold", grew>>20)
	}
	if calls == 0 || calls > 200 {
		t.Errorf("the first frame asked Row %d times", calls)
	}
	grid := wtest.FindRole(tree.Semantics(), semantics.RoleGrid)
	if grid == nil || grid.RowCount != n+1 {
		t.Fatalf("got grid %+v", grid)
	}
	// Ctrl+End reaches the last row, which Row names.
	r := tableOf(t, tree)
	tableKey(tree, input.KeyTab, 0)
	tableKey(tree, input.KeyEnd, input.ModControl)
	if got := r.c.focusRow(); got != n-1 {
		t.Fatalf("Ctrl+End moves to row %d, want %d", got, n-1)
	}
	grid = wtest.FindRole(tree.Semantics(), semantics.RoleGrid)
	rows := wtest.RolesOf(grid, semantics.RoleRow)
	last := rows[len(rows)-1]
	if want := strconv.Itoa(n-1) + " INFO"; last.Label != want {
		t.Errorf("the last row is named %q, want %q", last.Label, want)
	}
	// A frame that scrolls asks Row for the rows in view, not for all.
	calls = 0
	r.c.vert.Jump(float64(n/2) * tableRowHeight)
	wtest.Ops(tree, tableScreen)
	if calls == 0 || calls > 200 {
		t.Errorf("a scroll asked Row %d times", calls)
	}
}

// TestTable_RowSourceWinsOverRows gives both: Row is read, and Rows is not.
func TestTable_RowSourceWinsOverRows(t *testing.T) {
	calls := 0
	tb := logTable(5, &calls)
	tb.Rows = []logLine{{n: 99, level: "STALE"}}
	tree := tableTree(t, tb)
	grid := wtest.FindRole(tree.Semantics(), semantics.RoleGrid)
	if grid.RowCount != 6 {
		t.Fatalf("got %d rows with the header, want 6", grid.RowCount)
	}
	rows := wtest.RolesOf(grid, semantics.RoleRow)
	if got := rows[1].Label; got != "0 ERROR" {
		t.Errorf("the first row is named %q, want %q", got, "0 ERROR")
	}
}

// TestTable_RowSourceSelectionFollowsKeys sorts the rows that Row returns, by
// changing the order it reads them in, and the selection stays on its row.
func TestTable_RowSourceSelectionFollowsKeys(t *testing.T) {
	data := []string{"a", "b", "c", "d"}
	order := []int{0, 1, 2, 3}
	ctl := &TableController{}
	table := func() core.Widget {
		return Table[string]{
			RowCount:   len(order),
			Row:        func(i int) string { return data[order[i]] },
			RowKey:     func(s string) core.Key { return s },
			Controller: ctl,
			Columns:    []TableColumn[string]{{Title: "Name", Text: func(s string) string { return s }}},
		}
	}
	var state *wtest.SwapState
	tree := tableTree(t, wtest.Swap{State: &state, Build: table})
	ctl.Select(1) // "b"
	wtest.Ops(tree, tableScreen)
	order = []int{3, 2, 1, 0}
	state.SetState(func() {})
	wtest.Ops(tree, tableScreen)
	if !ctl.IsSelected(2) || ctl.IsSelected(1) {
		t.Errorf("the selection is %v, want row 2, where b went", ctl.Selection())
	}
}
