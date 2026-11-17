package collection

import (
	"slices"
	"strconv"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/input"
)

// keyedTable drives a Table of tracks whose rows the test changes.
type keyedTable struct {
	tree  *core.Tree
	state *wtest.SwapState
	ctrl  *TableController
	rows  []track
	now   uint32
}

func newKeyedTable(t *testing.T, n int, keyed bool) *keyedTable {
	t.Helper()
	k := &keyedTable{ctrl: &TableController{}, rows: tracks(n)}
	var key func(track) core.Key
	if keyed {
		key = func(r track) core.Key { return r.title }
	}
	k.tree = tableTree(t, wtest.Swap{State: &k.state, Build: func() core.Widget {
		return Table[track]{Rows: k.rows, Columns: trackColumns(), Controller: k.ctrl, RowKey: key}
	}})
	return k
}

// rebuild shows the rows as they now are.
func (k *keyedTable) rebuild() {
	k.state.SetState(func() {})
	wtest.Ops(k.tree, tableScreen)
}

func (k *keyedTable) selected() []string {
	var out []string
	k.ctrl.SelectedRows(func(i int) bool {
		out = append(out, k.rows[i].title)
		return true
	})
	return out
}

func TestTable_SelectionFollowsTheRowKey(t *testing.T) {
	k := newKeyedTable(t, 10, true)
	clickAt(k.tree, rowPos(3), 0, &k.now)
	clickAt(k.tree, rowPos(4), input.ModControl, &k.now)

	// Sort in place, the other way round.
	slices.Reverse(k.rows)
	k.rebuild()
	if got := selection(k.ctrl); !sameRanges(got, []RowRange{{5, 7}}) {
		t.Errorf("after the sort: %v, want rows 5 and 6", got)
	}
	if got := k.selected(); !slices.Equal(got, []string{"Track 4", "Track 3"}) {
		t.Errorf("after the sort: %v", got)
	}
	if got := wtest.FocusedLabel(k.tree); got != "Track 4" {
		t.Errorf("after the sort the focus is on %q, want Track 4", got)
	}

	// Take out a row above the selection, into a new slice.
	k.rows = slices.Delete(slices.Clone(k.rows), 0, 1)
	k.rebuild()
	if got := k.selected(); !slices.Equal(got, []string{"Track 4", "Track 3"}) {
		t.Errorf("after the deletion: %v", got)
	}
	if got := wtest.FocusedLabel(k.tree); got != "Track 4" {
		t.Errorf("after the deletion the focus is on %q, want Track 4", got)
	}

	// Take out a selected row: it leaves the selection.
	i := slices.IndexFunc(k.rows, func(r track) bool { return r.title == "Track 3" })
	k.rows = slices.Delete(slices.Clone(k.rows), i, i+1)
	k.rebuild()
	if got := k.selected(); !slices.Equal(got, []string{"Track 4"}) {
		t.Errorf("after deleting a selected row: %v", got)
	}

	// A selection from code names the rows of the next build.
	k.rows = tracks(10)
	k.ctrl.Select(2)
	k.rebuild()
	if got := k.selected(); !slices.Equal(got, []string{"Track 2"}) {
		t.Errorf("a selection from code: %v", got)
	}
}

func TestTable_SelectAllStaysAll(t *testing.T) {
	k := newKeyedTable(t, 10, true)
	clickAt(k.tree, rowPos(0), 0, &k.now)
	tableKey(k.tree, input.KeyA, input.ModControl)
	k.rows = k.rows[:7]
	k.rebuild()
	if got := selection(k.ctrl); !sameRanges(got, []RowRange{{0, 7}}) {
		t.Errorf("all after a deletion: %v", got)
	}
}

func TestTable_WithoutRowKeyTheSelectionIsIndexes(t *testing.T) {
	k := newKeyedTable(t, 10, false)
	clickAt(k.tree, rowPos(3), 0, &k.now)
	slices.Reverse(k.rows)
	k.rebuild()
	if got := k.selected(); !slices.Equal(got, []string{"Track 6"}) {
		t.Errorf("without keys the index stays: %v", got)
	}
}

func TestGridView_SelectionFollowsTheItemKey(t *testing.T) {
	ids := []int{10, 11, 12, 13, 14, 15}
	r := newTileRig(t, len(ids), nil)
	r.extra = func(g *GridView) {
		g.ItemCount = len(ids)
		g.ItemKey = func(i int) core.Key { return ids[i] }
		g.LabelOf = func(i int) string { return "item " + strconv.Itoa(ids[i]-10) }
	}
	r.refresh()
	r.click(2, 0)
	r.wantFocus(2)
	ids = []int{15, 14, 13, 12, 11, 10}
	r.refresh()
	r.wantSelection(3)
	r.wantFocus(2)
	ids = []int{15, 13, 12, 11, 10}
	r.refresh()
	r.wantSelection(2)
	r.wantFocus(2)
}
