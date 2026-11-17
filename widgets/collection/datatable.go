package collection

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

// The measures of a data table (DESIGN.md §4).
const (
	tableRowHeight    = 32
	tableHeaderHeight = 32
	// tablePadding is the room between the edge of a cell and its text.
	tablePadding     = theme.Space12
	tableColumnWidth = 120
	tableColumnMin   = 40
	// tableGrip is how far from the edge of a column a press resizes it.
	tableGrip = 4
	// tableDragSlop is how far a header moves before a press becomes a drag.
	tableDragSlop = 4
)

// SortOrder is the direction a column of a Table is sorted in.
//
// It is SortNone (the zero value), SortAscending or SortDescending.
type SortOrder uint8

// The orders. The zero value is no sort.
const (
	SortNone SortOrder = iota
	SortAscending
	SortDescending
)

// String returns the name of the order.
func (o SortOrder) String() string {
	switch o {
	case SortAscending:
		return "ascending"
	case SortDescending:
		return "descending"
	}
	return "none"
}

// semantics returns the order as the column header tells a screen reader.
func (o SortOrder) semantics() semantics.Sort {
	switch o {
	case SortAscending:
		return semantics.SortAscending
	case SortDescending:
		return semantics.SortDescending
	}
	return semantics.SortNone
}

// next returns the order a click on the header of a sorted column asks for:
// ascending, then descending, then none.
func (o SortOrder) next() SortOrder { return (o + 1) % 3 }

// TableSort is how the rows of a Table are sorted: by the column of
// Table.Columns at Column, in Order. The zero value is no sort.
type TableSort struct {
	Column int
	Order  SortOrder
}

// TableFilter is what the header of a column lets through. Text keeps the rows
// whose cell holds it, and Values, when not nil, the rows whose cell is one of
// them. The zero value lets every row through. The table only shows the
// filter: matching rows is the work of Table.OnFilter.
type TableFilter struct {
	Text   string
	Values []string
}

// Active reports whether f holds anything back.
func (f TableFilter) Active() bool { return f.Text != "" || f.Values != nil }

// TableEdit is an edit of a cell of a Table: Row is an index into
// Table.Rows, Column one into Table.Columns, and Text the new text.
type TableEdit struct {
	Row, Column int
	Text        string
}

// TableEditor is what TableColumn.Editor gets to build the editor of a cell.
// Text is the text of the cell when editing started. The editor calls Change
// as the user edits, so that Enter and Tab commit what it shows, and may call
// Commit and Cancel itself.
type TableEditor struct {
	Row, Column int
	Text        string
	Change      func(text string)
	Commit      func()
	Cancel      func()
}

// TableSelectMode says how many rows of a Table the user can select.
//
// It is TableSelectMultiple (the zero value), TableSelectSingle or
// TableSelectNone.
type TableSelectMode uint8

// The modes. The zero value selects any number of rows.
const (
	TableSelectMultiple TableSelectMode = iota
	TableSelectSingle
	TableSelectNone
)

// TableColumn is a column of a Table over rows of type T.
type TableColumn[T any] struct {
	Title string
	// Width is the width the column starts with; zero selects 120. MinWidth
	// is as narrow as dragging its edge makes it; zero selects 40.
	Width, MinWidth float32
	// Text returns what the cell of row shows. Nil leaves the cells empty.
	// The table calls it for the rows in view on every paint, so it must be
	// cheap: keep the strings ready rather than formatting them each time.
	Text func(row T) string
	// Trailing puts the text at the end of the cell, as for numbers.
	Trailing bool
	// Sortable makes a click on the header ask Table.OnSort to sort.
	Sortable bool
	// Editable lets the cells be edited, when Table.OnCommit is set.
	Editable bool
	// Editor, when not nil, builds the editor of a cell in place of the
	// Input that edits it by default.
	Editor func(e TableEditor) core.Widget
	// Filterable shows a button in the header that opens a filter, when
	// Table.OnFilter is set: a text field, and a list of FilterValues to
	// tick when there are any. Filter is the filter in force, which the
	// header shows.
	Filterable   bool
	FilterValues []string
	Filter       TableFilter

	// Kind is how the cells show their row: as plain text, the zero value,
	// as a badge, a dot, a bar or a check box that the table draws, or as a
	// widget of the caller's. Every kind but CellCheck and CellWidget also
	// shows Text, which is what a screen reader reads, what Editable edits and
	// what a filter matches. See CellKind.
	Kind CellKind
	// Value returns how full the bar of a CellProgress cell is, from 0 to 1.
	Value func(row T) float32
	// Checked returns whether the box of a CellCheck cell is ticked.
	Checked func(row T) bool
	// OnCheck, when not nil, lets the user tick and clear the box of a
	// CellCheck cell, with a click on the box or Space on the cell. It gets
	// the index of the row in Rows and the state asked for; the table goes on
	// showing Checked, so the caller changes the row. Without OnCheck the
	// boxes only show.
	OnCheck func(row int, checked bool)
	// Tone returns the color of the badge, dot, bar or box of row, as one of
	// the colors of the theme, so that it follows the theme. Nil, or
	// CellToneDefault, gives the default of the kind: neutral for a badge and
	// a dot, the accent for a bar and a box.
	Tone func(row T) CellTone
	// Color, when not nil, returns a color of the caller's in place of Tone.
	// A transparent color falls back to Tone.
	Color func(row T) geom.Color
	// Widget returns the widget that a CellWidget cell shows for row, such
	// as a control.Sparkline. Nil, or a nil widget, leaves the cell empty.
	//
	// The table calls Widget for the rows in view only. It calls it again
	// for every row in view whenever the table is built again, as when the
	// State that builds the table calls SetState, so a cell shows what the
	// row and the state of its parent say now; and for the rows that a
	// scroll brings into view, while a frame that brings no new row calls it
	// for none.
	//
	// The widget may be as wide as the column less 12 at each side and as
	// tall as the row less 2 above and below: 28 in a row of the default 32,
	// enough for a line of Body in a Container with 4 above and below. It
	// keeps the size it takes within that room, sits in the middle of the
	// height of the row, and at the start of the cell, or at its end for a
	// Trailing column. A widget is a value boxed into core.Widget, which
	// allocates: keep the widgets of the rows ready, as for Text, to scroll
	// without garbage.
	//
	// The table cannot read a widget, so Text stays the text of the cell: what
	// a screen reader reads for the cell and its row, what a filter should
	// match and what the caller copies. The cells are never edited. Sortable
	// and Filterable work as for any column, since the caller sorts and
	// filters: sort by a key of the row, not by the widget. See
	// docs/ADR/0100-table-widget-cells.md.
	Widget func(row T) core.Widget
}

// Table shows rows of type T in columns under a header, for any number
// of rows: it lays out, paints and describes only the rows in view, and never
// copies Rows. Rows that live elsewhere, as the lines of a file of millions,
// come through RowCount and Row without a slice of them. It scrolls both
// ways, the header stays at the top, and the first Frozen columns stay at the
// left. The header is 32 high, and every row RowHeight, 32 unless set. The
// text of a cell starts 12 inside its edge.
//
// The user resizes a column by dragging the edge of its header, and moves it
// by dragging the header. A click on the header of a Sortable column asks
// OnSort for the next order, ascending, descending and none in turn; the table
// shows the arrow of Sort and leaves the sorting to the caller, who passes the
// sorted rows back in Rows, or through Row; for TableSort with SortNone, pass
// them in their original order. Filters work the same way through OnFilter.
//
// Rows are selected as in a file list: a click selects one, Ctrl+click adds
// or removes one, Shift+click selects a range, Ctrl+A all, and the arrows with
// Shift extend the range. The selection lives in Controller, which the table
// keeps itself when Controller is nil. With RowKey it stays on the same rows
// when they are sorted, filtered or taken out; without it, it holds indexes,
// and clearing it when the rows change is the caller's work.
//
// The table is one stop of Tab. Inside, the arrows move a cell with the focus,
// Home and End go to the ends of the row, Ctrl+Home and Ctrl+End to the first
// and the last row, Page Up and Page Down by a page, and Space selects or
// clears the row. Up from the first row reaches the header, where Enter sorts
// and Alt+Down opens the filter. Enter or F2 edits an Editable cell, Enter
// commits, Escape cancels, and Tab commits and edits the next editable cell.
// Enter on a cell that is not editable calls OnActivate, as does a double
// click.
//
// A widget in a CellWidget cell gets the pointer before the table: a press
// that it takes, as on a button, neither selects the row nor activates it, and
// a press that it leaves, as on a sparkline, does. A widget that takes the
// keyboard is a stop of Tab of its own while its row is in view, after the
// cell in focus. Enter or F2 on its cell moves the keyboard into it, Escape
// back to the cell, and the arrows that it leaves move on to the next cell.
//
// A screen reader sees a grid named by Label, with a row for the header and
// one for each row in view, and the number of all the rows. The header of the
// column of Sort says its order as semantics.Sort, as aria-sort does. A
// CellWidget cell is a grid cell named by Text that holds the node of its
// widget.
type Table[T any] struct {
	core.StatefulBase
	// Rows holds the rows. For rows that live elsewhere, such as the lines of
	// a file of millions, leave it nil and give RowCount and Row instead.
	Rows []T
	// RowCount and Row, when Row is not nil, take the place of Rows: the
	// table has RowCount rows, and Row returns row i, for i from 0 to
	// RowCount-1. The table asks Row only for the rows it lays out, paints,
	// describes or keeps selected, several times a frame for a row in view,
	// so it should look the row up, not build it: an index into the data, a
	// cache of the rows near the view. An index i means row i of Row in every
	// callback, as it means Rows[i]. Rows is not read while Row is set.
	RowCount int
	Row      func(i int) T
	Columns  []TableColumn[T]
	// Label names the table to a screen reader.
	Label string
	// RowHeight is the height of a row; zero selects 32. Every row has the
	// same height, which is what lets the table find the rows in view of a
	// million at once.
	RowHeight float32
	// Frozen is how many columns stay at the left when the table scrolls
	// sideways.
	Frozen int
	Select TableSelectMode
	// Controller holds the selection, the focus and the columns as the user
	// arranged them. Nil keeps them in the table.
	Controller *TableController
	// RowKey, when not nil, names each row by a key that stays with it, such
	// as its ID or path, and makes the selection follow the rows: after a
	// sort, a filter or a deletion the same rows stay selected and the focus
	// stays on its row, wherever they now are. The keys must be comparable and
	// unique. The table calls RowKey for the selected rows on each build,
	// and for every row when they changed; returning a pointer or a small
	// integer keeps that free of garbage. Without RowKey the selection is
	// indexes, which stay where they are when the rows change: see
	// TableController.
	RowKey func(row T) core.Key
	// Sort is the order the rows are in, which the header shows.
	Sort TableSort
	// OnSort asks for the rows in another order.
	OnSort func(s TableSort)
	// OnFilter asks for the rows that f lets through in the column of
	// Columns at column.
	OnFilter func(column int, f TableFilter)
	// OnCommit receives an edit. It returns false to refuse it, which keeps
	// the editor open; true closes it. Without OnCommit no cell is editable.
	OnCommit func(e TableEdit) bool
	// OnCancel reports that the user left the editor of a cell with Escape.
	OnCancel func(row, column int)
	// OnActivate runs on a double click or Enter on a row that is not edited.
	OnActivate func(row int)
	// OnContextMenu asks for the context menu of row, an index of Rows, after
	// a click of the secondary button on it or the menu key or Shift+F10 on
	// the cell in focus, as Tree.OnContextMenu does: pos is from the top left
	// of the table, at the release of the button or the bottom left of the
	// cell, and the press has selected the row unless it was selected
	// already, and given its cell the focus. Open the menu with
	// overlay.ShowPopupMenu from the BuildContext of a basic.Builder around
	// the table, adding pos to its place, and look row up in the same Rows
	// slice that was passed.
	OnContextMenu func(row int, pos geom.Offset)
	// OnSelectionChange runs after the user changes the selection.
	OnSelectionChange func()
	// OnColumnsChange runs after the user resizes or moves a column, to save
	// the arrangement of Controller.
	OnColumnsChange func()
}

// tableSource gives the core of a table the text and the data of its cells,
// which need the type of the rows.
type tableSource interface {
	cellText(row, column int) string
	cellData(row, column int) tableCell
	cellWidget(row, column int) core.Widget
}

type dataTableState[T any] struct {
	core.StateOf[Table[T]]
	tableCore
	// rows, or row with count rows, are the rows of the last build.
	rows    []T
	row     func(i int) T
	count   int
	columns []TableColumn[T]
	// keyOf names a row by Table.RowKey, made once so that builds do not
	// make it again.
	keyOf func(i int) core.Key
}

// CreateState implements core.StatefulWidget.
func (Table[T]) CreateState() core.State {
	return &dataTableState[T]{tableCore: newTableCore()}
}

// InitState implements core.State.
func (s *dataTableState[T]) InitState() {
	s.keyOf = func(i int) core.Key { return s.Widget().RowKey(s.at(i)) }
	s.init(&s.StateBase, s)
	s.attach(s.Widget().Controller)
}

// DidUpdateWidget implements core.State.
func (s *dataTableState[T]) DidUpdateWidget(core.Widget) { s.attach(s.Widget().Controller) }

// Dispose implements core.State.
func (s *dataTableState[T]) Dispose() { s.dispose() }

// at returns row i, from Rows or from Row.
func (s *dataTableState[T]) at(i int) T {
	if s.row != nil {
		return s.row(i)
	}
	return s.rows[i]
}

func (s *dataTableState[T]) cellText(row, column int) string {
	if row < 0 || row >= s.count || column < 0 || column >= len(s.columns) {
		return ""
	}
	if f := s.columns[column].Text; f != nil {
		return f(s.at(row))
	}
	return ""
}

func (s *dataTableState[T]) cellData(row, column int) tableCell {
	var d tableCell
	if row < 0 || row >= s.count || column < 0 || column >= len(s.columns) {
		return d
	}
	col := &s.columns[column]
	v := s.at(row)
	if col.Value != nil {
		d.value = col.Value(v)
	}
	if col.Checked != nil {
		d.checked = col.Checked(v)
	}
	if col.Tone != nil {
		d.tone = col.Tone(v)
	}
	if col.Color != nil {
		d.color = col.Color(v)
	}
	return d
}

func (s *dataTableState[T]) cellWidget(row, column int) core.Widget {
	if row < 0 || row >= s.count || column < 0 || column >= len(s.columns) {
		return nil
	}
	if f := s.columns[column].Widget; f != nil {
		return f(s.at(row))
	}
	return nil
}

// Build implements core.State.
func (s *dataTableState[T]) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	s.rows, s.row, s.count, s.columns = w.Rows, w.Row, len(w.Rows), w.Columns
	if w.Row != nil {
		s.rows, s.count = nil, max(w.RowCount, 0)
	}
	s.models = s.models[:0]
	for i := range w.Columns {
		col := &w.Columns[i]
		width := col.Width
		if width <= 0 {
			width = tableColumnWidth
		}
		least := col.MinWidth
		if least <= 0 {
			least = tableColumnMin
		}
		s.models = append(s.models, tableCol{
			model: i, title: col.Title, width: width, min: least,
			trailing: col.Trailing, sortable: col.Sortable && w.OnSort != nil,
			editable: col.Editable && w.OnCommit != nil && col.Kind != CellCheck && col.Kind != CellWidget, editor: col.Editor,
			kind: col.Kind, hasText: col.Text != nil, onCheck: col.OnCheck,
			filterable: col.Filterable && w.OnFilter != nil, values: col.FilterValues, filter: col.Filter,
		})
	}
	shaper, ok := core.DependOn[*text.Shaper](ctx)
	if !ok {
		panic("gowaku: collection.Table needs a text.Shaper above it; wrap the root in core.Inherited[*text.Shaper]")
	}
	var key func(int) core.Key
	if w.RowKey != nil {
		key = s.keyOf
	}
	return s.build(ctx, shaper, tableConfig{
		rows: s.count, label: w.Label, rowHeight: w.RowHeight, frozen: w.Frozen, mode: w.Select, sort: w.Sort, key: key,
		onSort: w.OnSort, onFilter: w.OnFilter, onCommit: w.OnCommit, onCancel: w.OnCancel,
		onActivate: w.OnActivate, onSelection: w.OnSelectionChange, onColumns: w.OnColumnsChange,
		onContextMenu: w.OnContextMenu,
	})
}
