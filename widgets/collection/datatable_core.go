package collection

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// tableCol is a column of a table as the last build saw it, in the order the
// columns show.
type tableCol struct {
	// model is the index of the column in Table.Columns.
	model      int
	title      string
	width, min float32
	// left is the start of the column from the start of the first one.
	left                                     float32
	trailing, sortable, editable, filterable bool
	values                                   []string
	filter                                   TableFilter
	editor                                   func(TableEditor) core.Widget
	// kind is how the cells show; hasText says the column has a Text, and
	// onCheck is TableColumn.OnCheck.
	kind    CellKind
	hasText bool
	onCheck func(row int, checked bool)
}

// tableConfig is what a build of a Table passes to its core, other than
// the rows and columns.
type tableConfig struct {
	rows      int
	label     string
	rowHeight float32
	frozen    int
	mode      TableSelectMode
	sort      TableSort
	// key names row i by the key of Table.RowKey, or is nil without one.
	key func(i int) core.Key

	onSort        func(TableSort)
	onFilter      func(int, TableFilter)
	onCommit      func(TableEdit) bool
	onCancel      func(row, column int)
	onActivate    func(row int)
	onSelection   func()
	onColumns     func()
	onContextMenu func(row int, pos geom.Offset)
}

// tableZone is a part of the header under the pointer: the edge of a column
// that resizes it, or its filter button.
type tableZone struct {
	kind tableZoneKind
	// col is the column, in the order they show.
	col int
}

type tableZoneKind uint8

const (
	zoneNone tableZoneKind = iota
	zoneGrip
	zoneFilter
)

// tableDrag is a press on a table that is still down.
type tableDrag struct {
	kind tableDragKind
	// col is the column, in the order they show, of a header press, and model
	// its index in Table.Columns. model is what names the column for as long
	// as the press lasts: a build in between may show it elsewhere, or not at
	// all, and keepDrag brings col up to date.
	col, model int
	// start is where the press began, from the left of the table, and width
	// the width of the column then.
	start, width float32
	// x is where the pointer is now; moved says it went past the slop.
	x     float32
	moved bool
}

type tableDragKind uint8

const (
	tableDragNone tableDragKind = iota
	tableDragResize
	tableDragHeader
)

// tableCore is all of a Table that does not need the type of its rows:
// the geometry, the selection and the keys, the editor and the filter. The
// generic state embeds it, and hands it the text of the cells through src.
type tableCore struct {
	base *core.StateBase
	src  tableSource
	own  TableController
	ctrl *TableController

	// vert and horz scroll the rows and the columns.
	vert, horz basic.ScrollState
	ro         *renderTable

	cfg tableConfig
	// models are the columns in the order of Table.Columns, and cols in
	// the order they show; order maps the second to the first.
	models, cols []tableCol
	order        []int
	rowH         float32
	frozen       int
	// frozenW is the width of the frozen columns, and totalW that of all.
	frozenW, totalW float32

	th     *theme.Theme
	shaper *text.Shaper
	words  *basic.Strings
	media  basic.MediaQueryData

	// viewW and viewH are the size of the table at its last layout, and pool
	// the number of row objects the last build made for it.
	viewW, viewH float32
	pool         int

	hoverSlot, hoverHeader int
	hoverZone              tableZone
	drag                   tableDrag
	// refocus asks the next layout to give the keyboard to the cell in focus.
	refocus bool
	// menuPress is a press of the secondary button on menuRow that the
	// table keeps for OnContextMenu.
	menuPress bool
	menuRow   int

	// editing is true while the editor of the cell editRow, editCol (an
	// index of Table.Columns) is open, with editText in it. editSeq tells
	// one opening from the next.
	editing             bool
	editRow, editCol    int
	editText, editStart string
	editSeq             int

	filter tableFilterPopup

	// widgets says a column is of CellWidget. hosts are the states that build
	// the widgets of those cells, built the number of row objects of the last
	// build, and cellRows the row each widget cell was built for, by row
	// object and column. empty is what a widget cell without a widget holds.
	widgets  bool
	hosts    []*tableHostState
	built    int
	cellRows []int
	empty    core.Widget
}

func newTableCore() tableCore {
	return tableCore{hoverSlot: -1, hoverHeader: -1, filter: tableFilterPopup{col: -1}}
}

// init connects the core to its state. The two scroll states share the
// element of the table, which their tickers need: StateBase is only a pointer
// to it.
func (c *tableCore) init(base *core.StateBase, src tableSource) {
	c.base, c.src = base, src
	c.vert.StateBase = *base
	c.horz.StateBase = *base
	c.vert.InitState()
	c.horz.InitState()
	c.vert.OnMove = c.scrolled
	c.horz.OnMove = c.scrolled
}

// attach makes ctrl the controller of the table, or its own one for nil.
func (c *tableCore) attach(ctrl *TableController) {
	if ctrl == nil {
		ctrl = &c.own
	}
	if c.ctrl == ctrl {
		return
	}
	if c.ctrl != nil && c.ctrl.view == tableListener(c) {
		c.ctrl.view = nil
	}
	c.ctrl = ctrl
	ctrl.view = c
}

func (c *tableCore) dispose() {
	c.closeFilter(false)
	c.vert.Dispose()
	c.horz.Dispose()
	if c.ctrl != nil && c.ctrl.view == tableListener(c) {
		c.ctrl.view = nil
	}
}

// controllerChanged implements tableListener.
func (c *tableCore) controllerChanged() { c.changed() }

// changed rebuilds the table.
func (c *tableCore) changed() {
	if c.base.Mounted() {
		c.base.SetState(func() {})
	}
}

// selectionChanged rebuilds the table and tells the caller. The user chose
// among the rows on screen, so their keys are taken now, before the caller
// can change the rows.
func (c *tableCore) selectionChanged() {
	c.ctrl.remember(c.cfg.rows, c.cfg.key)
	c.changed()
	if c.cfg.onSelection != nil {
		c.cfg.onSelection()
	}
}

// scrolled follows a step of either scroll. The widget cells of row objects
// that now show other rows build again, before the layout.
func (c *tableCore) scrolled() {
	if c.ro != nil {
		c.ro.MarkNeedsLayout()
	}
	if len(c.hosts) > 0 {
		c.syncHosts()
	}
}

// build lays the columns out and returns the widgets of the table.
func (c *tableCore) build(ctx core.BuildContext, shaper *text.Shaper, cfg tableConfig) core.Widget {
	c.cfg = cfg
	c.th = basic.ThemeOf(ctx)
	c.shaper = shaper
	c.words = basic.StringsOf(ctx)
	c.media = basic.MediaQueryOf(ctx)
	c.rowH = cfg.rowHeight
	if c.rowH <= 0 {
		c.rowH = tableRowHeight
	}
	c.ctrl.follow(cfg.rows, cfg.key)
	c.ctrl.clip(cfg.rows)
	c.ctrl.remember(cfg.rows, cfg.key)
	c.order = c.ctrl.displayOrder(c.order, len(c.models))
	c.cols = c.cols[:0]
	c.frozenW = 0
	var left float32
	for j, m := range c.order {
		col := c.models[m]
		if w := c.ctrl.ColumnWidth(m); w > 0 {
			col.width = w
		}
		col.width = max(col.width, col.min)
		col.left = left
		left += col.width
		c.cols = append(c.cols, col)
		if j < cfg.frozen {
			c.frozenW = left
		}
	}
	c.frozen = min(max(cfg.frozen, 0), len(c.cols))
	c.totalW = left
	c.widgets = false
	for j := range c.cols {
		if c.cols[j].kind == CellWidget {
			c.widgets = true
		}
	}
	if c.widgets {
		if c.empty == nil {
			c.empty = basic.SizedBox{}
		}
		if c.viewH > 0 {
			// The layout would bring a scroll past fewer rows back, after the
			// widget cells built for the rows of the old offset.
			c.vert.Scroller.SetLimit(float64(cfg.rows)*float64(c.rowH) - float64(c.bodyH()))
		}
	}
	c.built = c.pool
	c.resetCellRows(c.pool)
	c.keepDrag()
	if c.editing && (c.editRow >= cfg.rows || c.displayOf(c.editCol) < 0) {
		c.editing = false
	}

	header := tableHeaderWidget{c: c, cells: make([]core.Widget, 0, 2*len(c.cols))}
	for j := range c.cols {
		header.cells = append(header.cells, tableCellWidget{c: c, col: j, header: true})
	}
	for j := range c.cols {
		header.cells = append(header.cells, tableZoneWidget{c: c, zone: tableZone{kind: zoneGrip, col: j}})
	}
	children := make([]core.Widget, 0, c.pool+2)
	children = append(children, header)
	for k := range c.pool {
		row := tableRowWidget{c: c, slot: k, cells: make([]core.Widget, len(c.cols))}
		for j := range c.cols {
			row.cells[j] = tableCellWidget{c: c, col: j, slot: k}
		}
		children = append(children, row)
	}
	if c.editing {
		// Each opening is a new editor, which takes the keyboard anew.
		children = append(children, core.Keyed(c.editSeq, focusFirst{onLeave: c.editorLeft, child: c.editor()}))
	}
	view := tableBody{c: c, children: children}
	inner := basic.ScrollFrame{Axis: layout.Horizontal, State: &c.horz, View: view}
	return basic.ScrollFrame{Axis: layout.Vertical, State: &c.vert, View: inner}
}

// keepDrag points a press that is still down on the header at its column
// again after a build laid the columns out, and ends the press if the column
// is gone. Without it the press would resize, move or sort whatever column
// took the place its own had, or index past the columns that are left.
func (c *tableCore) keepDrag() {
	if c.drag.kind == tableDragNone {
		return
	}
	j := c.displayOf(c.drag.model)
	if j < 0 {
		c.drag = tableDrag{}
		return
	}
	c.drag.col = j
}

// displayOf returns where the column of Table.Columns at model shows, or
// -1.
func (c *tableCore) displayOf(model int) int {
	for j, m := range c.order {
		if m == model {
			return j
		}
	}
	return -1
}

// bodyH returns the height of the rows in view.
func (c *tableCore) bodyH() float32 { return max(c.viewH-tableHeaderHeight, 0) }

// rowY returns the top of row r from the top of the table. The product is
// taken in float64 so that rows far down a million keep their place.
func (c *tableCore) rowY(r int) float32 {
	return tableHeaderHeight + float32(float64(r)*float64(c.rowH)-c.vert.Offset.Value())
}

// visibleRows returns the rows in view, first included and last not.
func (c *tableCore) visibleRows() (first, last int) {
	top := c.vert.Offset.Value()
	h := float64(c.rowH)
	first = int(top / h)
	last = max(min(int((top+float64(c.bodyH()))/h)+1, c.cfg.rows), 0)
	// The offset may lie past fewer rows until the next layout clamps it.
	return min(max(first, 0), last), last
}

// poolSize returns how many row objects the view needs: as many rows as can
// show at once, part of one at each end.
func (c *tableCore) poolSize() int {
	if c.bodyH() <= 0 {
		return 0
	}
	return int(c.bodyH()/c.rowH) + 2
}

// rowAt returns the row at y from the top of the table, or -1.
func (c *tableCore) rowAt(y float32) int {
	if y < tableHeaderHeight {
		return -1
	}
	r := int((float64(y-tableHeaderHeight) + c.vert.Offset.Value()) / float64(c.rowH))
	if r < 0 || r >= c.cfg.rows {
		return -1
	}
	return r
}

// colX returns the left of the column at j, in the order they show, from the
// left of the table.
func (c *tableCore) colX(j int) float32 {
	if j < c.frozen {
		return c.cols[j].left
	}
	return float32(float64(c.cols[j].left) - c.horz.Offset.Value())
}

// colAt returns the column at x from the left of the table, or -1.
func (c *tableCore) colAt(x float32) int {
	for j := range c.cols {
		if j == c.frozen && x < c.frozenW {
			return -1
		}
		l := c.colX(j)
		if x >= l && x < l+c.cols[j].width {
			return j
		}
	}
	return -1
}

// rowVisible reports whether row r has an object at the last layout.
func (c *tableCore) rowVisible(r int) bool {
	if r < 0 {
		return true
	}
	first, last := c.visibleRows()
	return r >= first && r < last && r-first < c.pool
}

// measured follows the size of the table after a layout.
func (c *tableCore) measured(w, h float32) {
	c.viewW, c.viewH = w, h
	c.vert.Metrics = layout.ScrollMetrics{View: float64(c.bodyH()), Content: float64(c.cfg.rows) * float64(c.rowH)}
	c.vert.Scroller.SetLimit(c.vert.Metrics.Content - c.vert.Metrics.View)
	c.horz.Metrics = layout.ScrollMetrics{View: float64(max(w-c.frozenW, 0)), Content: float64(c.totalW - c.frozenW)}
	c.horz.Scroller.SetLimit(c.horz.Metrics.Content - c.horz.Metrics.View)
	if need := c.poolSize(); need != c.pool {
		c.pool = need
		c.changed()
	}
}

// reveal scrolls the cell of row r and column j, in the order they show, into
// view. Row -1 is the header, which is always in view.
func (c *tableCore) reveal(r, j int) {
	if r >= 0 {
		top, rowH := float64(r)*float64(c.rowH), float64(c.rowH)
		at, view := c.vert.Offset.Value(), float64(c.bodyH())
		switch {
		case top < at:
			c.vert.Jump(top)
		case top+rowH > at+view:
			c.vert.Jump(max(top+rowH-view, 0))
		}
	}
	if j >= c.frozen && j < len(c.cols) {
		col := c.cols[j]
		at := c.horz.Offset.Value()
		view := float64(c.viewW - c.frozenW)
		left := float64(col.left - c.frozenW)
		switch {
		case left < at:
			c.horz.Jump(left)
		case left+float64(col.width) > at+view:
			c.horz.Jump(min(left+float64(col.width)-view, float64(max(c.totalW-c.viewW, 0))))
		}
	}
}

// focusCol returns the column in focus, in the order they show, or -1.
func (c *tableCore) focusCol() int {
	j := c.displayOf(c.ctrl.col)
	if j < 0 && len(c.cols) > 0 {
		return 0
	}
	return j
}

// focusRow returns the row in focus within the rows, -1 for the header.
func (c *tableCore) focusRow() int {
	return min(max(c.ctrl.row, -1), c.cfg.rows-1)
}
