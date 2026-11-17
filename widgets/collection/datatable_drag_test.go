package collection

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/semantics"
)

// dragRig is a table whose widget the test replaces while a press is down.
type dragRig struct {
	tree   *core.Tree
	ctrl   *TableController
	cols   []TableColumn[track]
	sorted []TableSort
	saved  int
	swap   *wtest.SwapState
}

func newDragRig(t *testing.T) *dragRig {
	t.Helper()
	r := &dragRig{ctrl: &TableController{}, cols: trackColumns()}
	for i := range r.cols {
		r.cols[i].Sortable = true
	}
	r.tree = tableTree(t, wtest.Swap{State: &r.swap, Build: func() core.Widget {
		return Table[track]{Rows: tracks(10), Columns: r.cols, Controller: r.ctrl,
			OnSort:          func(s TableSort) { r.sorted = append(r.sorted, s) },
			OnColumnsChange: func() { r.saved++ }}
	}})
	return r
}

// rebuild shows the table again with cols.
func (r *dragRig) rebuild(cols []TableColumn[track]) {
	r.swap.SetState(func() { r.cols = cols })
	wtest.Ops(r.tree, tableScreen)
}

func (r *dragRig) pointer(kind core.PointerKind, x float32) {
	r.tree.DispatchPointer(core.PointerEvent{Kind: kind, Pos: geom.Offset{X: x, Y: 16}})
	wtest.Ops(r.tree, tableScreen)
}

func TestDataTable_ResizeOutlivesItsColumn(t *testing.T) {
	r := newDragRig(t)
	// The right edge of Plays, the third column.
	r.pointer(core.PointerDown, 460)
	r.pointer(core.PointerMove, 480)
	if got := r.ctrl.ColumnWidth(2); got != 120 {
		t.Fatalf("width %v before the rebuild", got)
	}
	r.rebuild(r.cols[:2])
	r.pointer(core.PointerMove, 500)
	r.pointer(core.PointerUp, 500)
	if got := []float32{r.ctrl.ColumnWidth(0), r.ctrl.ColumnWidth(1)}; got[0] != 0 || got[1] != 0 {
		t.Errorf("the drag of a column that is gone resized another: %v", got)
	}
	if r.saved != 0 {
		t.Errorf("a drag that lost its column saved the columns %d times", r.saved)
	}
}

func TestDataTable_ResizeFollowsItsColumnWhenTheOrderChanges(t *testing.T) {
	r := newDragRig(t)
	// The right edge of Title, the first column.
	r.pointer(core.PointerDown, 200)
	r.pointer(core.PointerMove, 220)
	r.ctrl.SetColumnOrder([]int{2, 0, 1})
	wtest.Ops(r.tree, tableScreen)
	r.pointer(core.PointerMove, 250)
	r.pointer(core.PointerUp, 250)
	if got := r.ctrl.ColumnWidth(0); got != 250 {
		t.Errorf("Title is %v wide, want 250", got)
	}
	if got := r.ctrl.ColumnWidth(2); got != 0 {
		t.Errorf("Plays, now in the place Title had, was resized to %v", got)
	}
	if r.saved != 1 {
		t.Errorf("saved %d times", r.saved)
	}
}

func TestDataTable_HeaderDragOutlivesItsColumn(t *testing.T) {
	r := newDragRig(t)
	// Plays, from the middle of its header to the left.
	r.pointer(core.PointerDown, 410)
	r.pointer(core.PointerMove, 300)
	// The rebuild paints the header that is under the pointer.
	r.rebuild(r.cols[:2])
	r.pointer(core.PointerMove, 100)
	r.pointer(core.PointerUp, 100)
	if got := r.ctrl.ColumnOrder(); got != nil {
		t.Errorf("the drag of a column that is gone moved another: %v", got)
	}
	heads := wtest.RolesOf(r.tree.Semantics(), semantics.RoleColumnHeader)
	if len(heads) != 2 || heads[0].Label != "Title" || heads[1].Label != "Artist" {
		t.Errorf("heads %+v", heads)
	}
}

func TestDataTable_HeaderPressOutlivesItsColumn(t *testing.T) {
	r := newDragRig(t)
	// A press on Plays that never moves would sort on release.
	r.pointer(core.PointerDown, 410)
	r.rebuild(r.cols[:2])
	r.pointer(core.PointerUp, 410)
	if len(r.sorted) != 0 {
		t.Errorf("the press on a column that is gone sorted %v", r.sorted)
	}
}

func TestDataTable_HeaderDragFollowsItsColumnWhenTheOrderChanges(t *testing.T) {
	r := newDragRig(t)
	// Title, from the middle of its header.
	r.pointer(core.PointerDown, 100)
	r.pointer(core.PointerMove, 150)
	// Plays, Title, Artist: Title now shows second.
	r.ctrl.SetColumnOrder([]int{2, 0, 1})
	wtest.Ops(r.tree, tableScreen)
	// Past the middle of Artist, the last column: Title lands at the end.
	r.pointer(core.PointerMove, 440)
	r.pointer(core.PointerUp, 440)
	if got := r.ctrl.ColumnOrder(); len(got) != 3 || got[0] != 2 || got[1] != 1 || got[2] != 0 {
		t.Errorf("order %v, want [2 1 0]", got)
	}
}

func TestDataTable_HeaderPressSortsItsColumnWhenTheOrderChanges(t *testing.T) {
	r := newDragRig(t)
	r.pointer(core.PointerDown, 100)
	r.ctrl.SetColumnOrder([]int{2, 0, 1})
	wtest.Ops(r.tree, tableScreen)
	r.pointer(core.PointerUp, 100)
	if len(r.sorted) != 1 || r.sorted[0].Column != 0 {
		t.Errorf("sorted %v, want the column the press began on", r.sorted)
	}
}
