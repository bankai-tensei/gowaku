package collection

import (
	"slices"
	"strconv"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
)

// tileScreen is the window; the grid of a tileRig is 400 by 300 at its top
// left: with columns of at least 100 and tiles 80 high, four columns and
// three rows and a bit in view.
var tileScreen = geom.Size{W: 600, H: 400}

// tileRig drives a GridView of n tiles labeled "item i".
type tileRig struct {
	t         *testing.T
	tree      *core.Tree
	swap      func(core.Widget)
	ctrl      *TableController
	n         int
	width     float32
	extra     func(*GridView)
	built     map[int]bool
	activated []int
	changes   int
	clock     uint32
}

func newTileRig(t *testing.T, n int, extra func(*GridView)) *tileRig {
	t.Helper()
	r := &tileRig{t: t, ctrl: &TableController{}, n: n, width: 400, extra: extra, built: map[int]bool{}}
	r.tree, r.swap = wtest.SwapTree(t, r.widget())
	r.frame()
	return r
}

func (r *tileRig) widget() core.Widget {
	g := GridView{
		ItemCount: r.n, MinColumnWidth: 100, TileHeight: 80,
		Label: "Photos", Controller: r.ctrl,
		LabelOf: func(i int) string { return "item " + strconv.Itoa(i) },
		ItemBuilder: func(i int) core.Widget {
			r.built[i] = true
			return basic.Container{Color: wtest.Red}
		},
		OnActivate:        func(i int) { r.activated = append(r.activated, i) },
		OnSelectionChange: func() { r.changes++ },
	}
	if r.extra != nil {
		r.extra(&g)
	}
	return basic.Align{Alignment: layout.AlignTopLeft, Child: basic.SizedBox{Width: r.width, Height: 300, Child: g}}
}

func (r *tileRig) refresh() {
	r.swap(r.widget())
	r.frame()
}

func (r *tileRig) frame() { wtest.Ops(r.tree, tileScreen) }

func (r *tileRig) cells() []*semantics.Node {
	return wtest.RolesOf(r.tree.Semantics(), semantics.RoleGridCell)
}

func (r *tileRig) cell(i int) *semantics.Node {
	r.t.Helper()
	label := "item " + strconv.Itoa(i)
	for _, n := range r.cells() {
		if n.Label == label {
			return n
		}
	}
	r.t.Fatalf("no tile %q", label)
	return nil
}

func (r *tileRig) Key(k input.KeyCode, m input.Modifiers) {
	r.tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: m})
	r.frame()
}

func (r *tileRig) click(i int, m input.Modifiers) {
	r.t.Helper()
	pos := wtest.Center(r.cell(i).Rect)
	r.clock += 1000
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos, Modifiers: m, Time: r.clock})
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos, Modifiers: m, Time: r.clock + 1})
	r.frame()
}

func (r *tileRig) focus() int {
	r.t.Helper()
	l := wtest.FocusedLabel(r.tree)
	if l == "" {
		return -1
	}
	i, err := strconv.Atoi(l[len("item "):])
	if err != nil {
		r.t.Fatalf("focus on %q", l)
	}
	return i
}

func (r *tileRig) wantFocus(want int) {
	r.t.Helper()
	if got := r.focus(); got != want {
		r.t.Errorf("focus on %d, want %d", got, want)
	}
}

func (r *tileRig) selection() []int {
	var out []int
	r.ctrl.SelectedRows(func(i int) bool { out = append(out, i); return true })
	return out
}

func (r *tileRig) wantSelection(want ...int) {
	r.t.Helper()
	if got := r.selection(); !slices.Equal(got, want) {
		r.t.Errorf("selection %v, want %v", got, want)
	}
}

func (r *tileRig) body() *renderGridBody {
	r.t.Helper()
	return gridBodyOf(r.t, r.tree)
}

func gridBodyOf(t testing.TB, tree *core.Tree) *renderGridBody {
	t.Helper()
	for _, ro := range tree.HitTest(geom.Offset{X: 2, Y: 2}).Path {
		if b, ok := ro.(*renderGridBody); ok {
			return b
		}
	}
	t.Fatal("no grid body")
	return nil
}

// topItem returns the first item whose tile shows at the top of the view.
func (r *tileRig) topItem() int {
	b := r.body()
	g := &b.geo
	return g.rowAt(b.position.Value()) * g.cols
}

func TestGridView_ZeroValue(t *testing.T) {
	tree := wtest.TextTree(t, basic.SizedBox{Width: 200, Height: 100, Child: GridView{}})
	wtest.Ops(tree, tileScreen)
	if n := len(wtest.RolesOf(tree.Semantics(), semantics.RoleGridCell)); n != 0 {
		t.Errorf("got %d cells", n)
	}
	grids := wtest.RolesOf(tree.Semantics(), semantics.RoleGrid)
	if len(grids) != 1 || grids[0].RowCount != 0 {
		t.Errorf("grid %+v", grids)
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyDown})
	wtest.Ops(tree, tileScreen)
	// Without an ItemBuilder the tiles are empty, and still there.
	tree = wtest.TextTree(t, basic.SizedBox{Width: 200, Height: 100, Child: GridView{ItemCount: 3}})
	wtest.Ops(tree, tileScreen)
	if n := len(wtest.RolesOf(tree.Semantics(), semantics.RoleGridCell)); n != 3 {
		t.Errorf("got %d cells, want 3", n)
	}
}

func TestGridView_InColumnPanics(t *testing.T) {
	g := GridView{ItemCount: 3}
	wtest.ExpectPanic(t, "collection.GridView has an unbounded height", func() {
		wtest.Rects(t, basic.Column{Children: []core.Widget{g}}, wtest.Page)
	})
}

func TestGridView_BuildsOnlyWhatIsNear(t *testing.T) {
	r := newTileRig(t, 100000, nil)
	// Four rows show; three more on each side are built.
	if n, most := len(r.built), (4+cachedItems)*4; n > most {
		t.Errorf("built %d tiles, want at most %d", n, most)
	}
	if !r.built[0] || !r.built[15] {
		t.Error("the tiles in view must exist")
	}
	if n := len(r.cells()); n == 0 || n > 4*4 {
		t.Errorf("%d tiles speak, want those in view", n)
	}
}

func TestGridView_ColumnsFollowTheWidth(t *testing.T) {
	cases := []struct {
		name  string
		width float32
		extra func(*GridView)
		cols  int
		tile  geom.Rect
	}{
		{"四列", 400, nil, 4, geom.Rect{X: 100, Y: 0, W: 100, H: 80}},
		{"幅が狭いと二列", 250, nil, 2, geom.Rect{X: 125, Y: 0, W: 125, H: 80}},
		{"列数を固定", 400, func(g *GridView) { g.Columns = 5 }, 5, geom.Rect{X: 80, Y: 0, W: 80, H: 80}},
		{"縦横比", 400, func(g *GridView) { g.TileHeight, g.AspectRatio = 0, 2 }, 4, geom.Rect{X: 100, Y: 0, W: 100, H: 50}},
		{"既定は正方形", 400, func(g *GridView) { g.TileHeight = 0 }, 4, geom.Rect{X: 100, Y: 0, W: 100, H: 100}},
		{"間隔と余白", 400, func(g *GridView) {
			g.Spacing, g.RunSpacing, g.Padding = 10, 6, geom.Insets{Left: 5, Top: 7, Right: 5}
		}, 3, geom.Rect{X: 5 + 370.0/3 + 10, Y: 7, W: 370.0 / 3, H: 80}},
		{"右から左", 400, func(g *GridView) { g.Direction = text.DirectionRTL }, 4, geom.Rect{X: 200, Y: 0, W: 100, H: 80}},
		{"どこにも収まらなくても一列", 50, nil, 1, geom.Rect{X: 0, Y: 80, W: 50, H: 80}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &tileRig{t: t, ctrl: &TableController{}, n: 30, width: c.width, extra: c.extra, built: map[int]bool{}}
			r.tree, r.swap = wtest.SwapTree(t, r.widget())
			r.frame()
			if got := r.body().geo.cols; got != c.cols {
				t.Errorf("%d columns, want %d", got, c.cols)
			}
			if got := r.cell(1).Rect; !wtest.NearRect(got, c.tile) {
				t.Errorf("item 1 at %+v, want %+v", got, c.tile)
			}
			if c.cols > 1 {
				if row := r.cell(c.cols).Rect; row.Y <= r.cell(0).Rect.Y {
					t.Errorf("item %d must start the second row: %+v", c.cols, row)
				}
			}
		})
	}
}

func TestGridView_Semantics(t *testing.T) {
	r := newTileRig(t, 10, nil)
	r.ctrl.Select(5)
	r.frame()
	grids := wtest.RolesOf(r.tree.Semantics(), semantics.RoleGrid)
	if len(grids) != 1 {
		t.Fatalf("%d grids", len(grids))
	}
	g := grids[0]
	if g.Label != "Photos" || g.RowCount != 3 || g.ColumnCount != 4 || g.State&semantics.Multiselectable == 0 {
		t.Errorf("grid %+v", g)
	}
	rows := wtest.RolesOf(g, semantics.RoleRow)
	if len(rows) != 3 || rows[2].RowIndex != 3 || len(rows[2].Children) != 2 {
		t.Fatalf("rows %+v", rows)
	}
	c := r.cell(5)
	if c.RowIndex != 2 || c.ColumnIndex != 2 || c.State&semantics.Selected == 0 || c.Actions&semantics.ActionTap == 0 {
		t.Errorf("item 5 %+v", c)
	}
	focusable := 0
	for _, n := range r.cells() {
		if n.State&semantics.Focusable != 0 {
			focusable++
		}
	}
	if focusable != 1 {
		t.Errorf("one tile must take Tab, got %d", focusable)
	}
	single := newTileRig(t, 10, func(g *GridView) { g.Select = TableSelectSingle })
	if wtest.RolesOf(single.tree.Semantics(), semantics.RoleGrid)[0].State&semantics.Multiselectable != 0 {
		t.Error("a single selection grid is not multiselectable")
	}
	// A tap of a screen reader selects.
	root, _ := r.tree.SemanticsTree()
	id := wtest.ByLabel(t, root, "item 7").ID
	if !r.tree.PerformSemanticsAction(id, semantics.ActionTap, core.SemanticsArg{}) {
		t.Fatal("tap must be done")
	}
	r.frame()
	r.wantSelection(7)
}

func TestGridView_Keyboard(t *testing.T) {
	// 10 items in four columns: rows of 0-3, 4-7 and 8-9.
	r := newTileRig(t, 10, nil)
	r.Key(input.KeyTab, 0)
	r.wantFocus(0)
	steps := []struct {
		name  string
		key   input.KeyCode
		focus int
	}{
		{"左端の左は動かない", input.KeyLeft, 0},
		{"上端の上は動かない", input.KeyUp, 0},
		{"右", input.KeyRight, 1},
		{"下", input.KeyDown, 5},
		{"右端から次の行へ", input.KeyRight, 6},
		{"右", input.KeyRight, 7},
		{"行末の右は次の行頭", input.KeyRight, 8},
		{"行頭の左は前の行末", input.KeyLeft, 7},
		{"短い最後の行へは最後の項目", input.KeyDown, 9},
		{"最後の行の下は動かない", input.KeyDown, 9},
		{"最後の右は動かない", input.KeyRight, 9},
		{"上は同じ列", input.KeyUp, 5},
		{"Home", input.KeyHome, 0},
		{"End", input.KeyEnd, 9},
		{"PageUp", input.KeyPageUp, 1},
		{"PageDown", input.KeyPageDown, 9},
	}
	for _, s := range steps {
		r.Key(s.key, 0)
		if got := r.focus(); got != s.focus {
			t.Errorf("%s: focus on %d, want %d", s.name, got, s.focus)
		}
	}
	r.wantSelection(9)

	// Right to left, Left goes on to the next item.
	rtl := newTileRig(t, 10, func(g *GridView) { g.Direction = text.DirectionRTL })
	rtl.Key(input.KeyTab, 0)
	rtl.Key(input.KeyLeft, 0)
	rtl.wantFocus(1)
	rtl.Key(input.KeyRight, 0)
	rtl.wantFocus(0)
}

func TestGridView_PageKeysScroll(t *testing.T) {
	r := newTileRig(t, 1000, nil)
	r.Key(input.KeyTab, 0)
	r.Key(input.KeyPageDown, 0)
	// A view of 300 holds three whole rows; a page is two.
	r.wantFocus(8)
	for range 10 {
		r.Key(input.KeyPageDown, 0)
	}
	r.wantFocus(88)
	b := r.body()
	if top := b.geo.rowTop(22); top < b.position.Value() || top+80 > b.position.Value()+300 {
		t.Errorf("the tile in focus must show, offset %v", b.position.Value())
	}
	r.Key(input.KeyEnd, 0)
	r.wantFocus(999)
	if off, limit := b.position.Value(), b.geo.content(1000)-300; off != limit {
		t.Errorf("End must reach the end, offset %v of %v", off, limit)
	}
}

func TestGridView_Selection(t *testing.T) {
	r := newTileRig(t, 20, nil)
	r.click(2, 0)
	r.wantSelection(2)
	r.click(5, input.ModControl)
	r.wantSelection(2, 5)
	r.click(2, input.ModControl)
	r.wantSelection(5)
	// Ctrl+click moved the anchor to 2, as in a Table.
	r.click(9, input.ModShift)
	r.wantSelection(2, 3, 4, 5, 6, 7, 8, 9)
	r.wantFocus(9)
	r.click(5, 0)
	r.click(9, input.ModShift)
	r.wantSelection(5, 6, 7, 8, 9)
	// The arrows with Shift extend from the anchor, with Ctrl move alone.
	r.Key(input.KeyDown, input.ModShift)
	r.wantSelection(5, 6, 7, 8, 9, 10, 11, 12, 13)
	r.Key(input.KeyRight, input.ModControl)
	r.wantFocus(14)
	r.wantSelection(5, 6, 7, 8, 9, 10, 11, 12, 13)
	r.Key(input.KeySpace, 0)
	r.wantSelection(5, 6, 7, 8, 9, 10, 11, 12, 13, 14)
	r.Key(input.KeyA, input.ModControl)
	if got := r.ctrl.SelectedCount(); got != 20 {
		t.Errorf("Ctrl+A selected %d", got)
	}
	if r.changes == 0 {
		t.Error("OnSelectionChange must run")
	}

	// A press below the tiles clears the selection.
	few := newTileRig(t, 6, nil)
	few.click(1, 0)
	few.click(4, input.ModControl)
	few.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: geom.Offset{X: 200, Y: 250}})
	few.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: geom.Offset{X: 200, Y: 250}})
	few.frame()
	few.wantSelection()

	one := newTileRig(t, 20, func(g *GridView) { g.Select = TableSelectSingle })
	one.click(2, 0)
	one.click(5, input.ModControl)
	one.click(9, input.ModShift)
	one.wantSelection(9)
	one.Key(input.KeyA, input.ModControl)
	one.wantSelection(9)

	none := newTileRig(t, 20, func(g *GridView) { g.Select = TableSelectNone })
	none.click(2, 0)
	none.Key(input.KeyRight, 0)
	none.wantSelection()
	none.wantFocus(3)
}

func TestGridView_Activate(t *testing.T) {
	r := newTileRig(t, 20, nil)
	pos := wtest.Center(r.cell(6).Rect)
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos, Time: 50000})
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos, Time: 50010})
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos, Time: 50100})
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos, Time: 50110})
	r.frame()
	r.Key(input.KeyRight, 0)
	r.Key(input.KeyEnter, 0)
	if !slices.Equal(r.activated, []int{6, 7}) {
		t.Errorf("activated %v", r.activated)
	}
}

func TestGridView_ResizeKeepsTheTopItem(t *testing.T) {
	r := newTileRig(t, 1000, nil)
	// Row 10, items 40 to 43, at the top.
	r.body().host.Jump(10 * 80)
	r.frame()
	if got := r.topItem(); got != 40 {
		t.Fatalf("top item %d", got)
	}
	r.width = 200
	r.refresh()
	if got := r.body().geo.cols; got != 2 {
		t.Fatalf("%d columns", got)
	}
	if got := r.topItem(); got != 40 {
		t.Errorf("after two columns the top item is %d, want 40", got)
	}
	if off := r.body().position.Value(); off != 20*80 {
		t.Errorf("offset %v, want %v", off, 20*80)
	}
	// Half a row down, the half stays.
	r.body().host.Jump(20*80 + 40)
	r.frame()
	r.width = 500
	r.refresh()
	if got, off := r.topItem(), r.body().position.Value(); got != 40 || off != 8*80+40 {
		t.Errorf("top item %d at offset %v, want 40 at %v", got, off, 8*80+40)
	}
	// A tile in focus that shows stays in view.
	r.click(47, 0)
	r.width = 200
	r.refresh()
	b := r.body()
	if top := b.geo.rowTop(47 / 2); top < b.position.Value() || top+80 > b.position.Value()+300 {
		t.Errorf("the tile in focus left the view: row top %v, offset %v", top, b.position.Value())
	}
}

func TestGridView_ShrinkWhileScrolledAndSelected(t *testing.T) {
	r := newTileRig(t, 1000, nil)
	r.Key(input.KeyTab, 0)
	r.Key(input.KeyEnd, 0)
	r.Key(input.KeyLeft, input.ModShift)
	r.wantSelection(998, 999)
	r.n = 10
	r.refresh()
	r.frame()
	b := r.body()
	if off := b.position.Value(); off != 0 {
		t.Errorf("a grid shorter than its view rests at the top, offset %v", off)
	}
	if n := len(r.cells()); n != 10 {
		t.Errorf("%d tiles speak, want 10", n)
	}
	r.wantSelection()
	r.wantFocus(9)
	// The keyboard keeps working on the tiles left.
	r.Key(input.KeyUp, 0)
	r.wantFocus(5)
	r.n = 0
	r.refresh()
	r.Key(input.KeyDown, 0)
	if n := len(r.cells()); n != 0 {
		t.Errorf("%d tiles left", n)
	}
	r.n = 4
	r.refresh()
	r.ctrl.Select(3)
	r.frame()
	r.wantSelection(3)
}

func TestGridView_ControllerFromOutside(t *testing.T) {
	r := newTileRig(t, 100, nil)
	r.ctrl.SetSelection(RowRange{3, 6})
	r.frame()
	n := 0
	for _, c := range r.cells() {
		if c.State&semantics.Selected != 0 {
			n++
		}
	}
	if n != 3 {
		t.Errorf("%d tiles show selected, want 3", n)
	}
	r.ctrl.SelectAll()
	if got := r.ctrl.SelectedCount(); got != 100 {
		t.Errorf("SelectAll selected %d", got)
	}
	// Another grid takes the controller over.
	other := newTileRig(t, 5, nil)
	other.ctrl = r.ctrl
	other.refresh()
	if r.ctrl.view != tableListener(gridBodyOf(t, other.tree).host) {
		t.Error("the controller must follow the grid that shows it")
	}
}

func TestGridView_RepaintWhileScrollingDoesNotAllocate(t *testing.T) {
	r := newGridFrameRunner(t)
	first := r.state.first
	p0 := r.state.Offset.Value()
	moved := false
	for range 50 {
		r.frame()
		moved = moved || r.state.Offset.Value() != p0
	}
	if !moved {
		t.Fatal("the view must move")
	}
	allocs := testing.AllocsPerRun(1, func() {
		for range 1000 {
			r.frame()
		}
	})
	if allocs != 0 {
		t.Errorf("got %v allocs in 1000 frames", allocs)
	}
	if r.state.first != first {
		t.Error("the motion must stay within the rows that exist")
	}
}
