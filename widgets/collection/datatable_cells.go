package collection

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// CellKind is how the cells of a TableColumn show their row. The table draws
// every kind itself, from the rows in view, so that a column of badges or bars
// scrolls as fast as one of text. See docs/ADR/0071-datatable-cell-kinds.md.
//
// It is CellText (the zero value), CellBadge, CellDot, CellProgress,
// CellCheck or CellWidget.
type CellKind uint8

// The kinds. The zero value is plain text.
const (
	// CellText shows Text.
	CellText CellKind = iota
	// CellBadge shows Text on a pale pill, both in the color of the row, as
	// for a status.
	CellBadge
	// CellDot shows a dot in the color of the row before Text.
	CellDot
	// CellProgress shows a bar filled to Value, and Text after it when the
	// column has one, as for a share or a battery. Trailing does not move
	// it.
	CellProgress
	// CellCheck shows a check box ticked by Checked, and Text after it when
	// the column has one. OnCheck makes the box something the user ticks.
	// The cells are never edited.
	CellCheck
	// CellWidget shows the widget that Widget builds for the row, as a
	// control.Sparkline. Text names the cell to a screen reader without
	// showing. The cells are never edited. See TableColumn.Widget.
	CellWidget
)

// String returns the name of the kind.
func (k CellKind) String() string {
	switch k {
	case CellBadge:
		return "badge"
	case CellDot:
		return "dot"
	case CellProgress:
		return "progress"
	case CellCheck:
		return "check"
	case CellWidget:
		return "widget"
	}
	return "text"
}

// CellTone is a color of the theme for a cell of a TableColumn.
//
// It is CellToneDefault (the zero value), CellToneNeutral, CellToneAccent,
// CellToneSuccess, CellToneWarning or CellToneDanger.
type CellTone uint8

// The tones. The zero value is the default of the kind of the column.
const (
	CellToneDefault CellTone = iota
	// CellToneNeutral is gray, for what needs no notice.
	CellToneNeutral
	// CellToneAccent is the accent of the theme.
	CellToneAccent
	// CellToneSuccess is theme.Colors.Success, for what went well.
	CellToneSuccess
	// CellToneWarning is theme.Colors.Warning, for what needs attention
	// before it goes wrong.
	CellToneWarning
	// CellToneDanger is theme.Colors.Danger, for what went wrong.
	CellToneDanger
)

// String returns the name of the tone.
func (t CellTone) String() string {
	switch t {
	case CellToneNeutral:
		return "neutral"
	case CellToneAccent:
		return "accent"
	case CellToneSuccess:
		return "success"
	case CellToneWarning:
		return "warning"
	case CellToneDanger:
		return "danger"
	}
	return "default"
}

// The measures of the cell kinds.
const (
	// tableBadgeHeight is the height of a badge, a pill, and tableBadgePad
	// the room between its ends and its text.
	tableBadgeHeight = 20
	tableBadgePad    = theme.Space8
	// tableTint is how strong the color of a badge is under its text.
	tableTint = 0.14
	// tableDot is the side of a dot.
	tableDot = 8
	// tableBarHeight is the thickness of a bar, as that of ProgressBar, and
	// tableBarLabel the room kept after it for the text of the column, so
	// that the bars of a column are all as long.
	tableBarHeight = 6
	tableBarLabel  = 44
	// tableCheckSlop is how far around a check box a press still ticks it.
	tableCheckSlop = theme.Space4
	// tableKindGap is the room between a dot, a bar or a box and its text.
	tableKindGap = theme.Space8
)

// tableCell is the data of a cell, other than its text, that its kind shows.
type tableCell struct {
	value   float32
	checked bool
	tone    CellTone
	color   geom.Color
}

// tableColors are the colors of a cell: ink for text and lines, fill for a
// solid shape, and tint for a pale ground.
type tableColors struct {
	ink, fill, tint geom.Color
}

// colorsOf returns the colors of a cell of the kind k.
func (r *renderTable) colorsOf(k CellKind, d tableCell) tableColors {
	col := &r.c.th.Color
	if d.color.A > 0 {
		return tableColors{ink: d.color, fill: d.color, tint: d.color.WithAlpha(d.color.A * tableTint)}
	}
	tone := d.tone
	if tone == CellToneDefault {
		tone = CellToneNeutral
		if k == CellProgress || k == CellCheck {
			tone = CellToneAccent
		}
	}
	var fill geom.Color
	switch tone {
	case CellToneAccent:
		return tableColors{ink: col.AccentInk, fill: col.Accent, tint: col.Accent.WithAlpha(col.Accent.A * tableTint)}
	case CellToneSuccess:
		fill = col.Success
	case CellToneWarning:
		fill = col.Warning
	case CellToneDanger:
		fill = col.Danger
	default:
		return tableColors{ink: col.Ink2, fill: col.Ink3, tint: col.FillStrong}
	}
	return tableColors{ink: fill, fill: fill, tint: fill.WithAlpha(fill.A * tableTint)}
}

// paintCell draws the cell of row i in the column col, of a kind other than
// text, in box.
func (r *renderTable) paintCell(cv *paint.Canvas, col *tableCol, i int, box geom.Rect) {
	c := r.c
	room := box.W - 2*tablePadding
	if room <= 0 {
		return
	}
	s := c.src.cellText(i, col.model)
	d := c.src.cellData(i, col.model)
	colors := r.colorsOf(col.kind, d)
	left, right := box.X+tablePadding, box.X+box.W-tablePadding
	switch col.kind {
	case CellBadge:
		if s == "" {
			return
		}
		l := r.layoutOf(s, styleBadge)
		w := min(l.Size().W+2*tableBadgePad, room)
		x := left
		if col.trailing {
			x = right - w
		}
		pill := geom.Rect{X: x, Y: box.Y + geom.Floor32((box.H-tableBadgeHeight)/2), W: w, H: tableBadgeHeight}
		cv.DrawRRect(geom.RRectAll(pill, tableBadgeHeight/2), paint.Paint{Color: colors.tint})
		r.paintLayout(cv, l, x+tableBadgePad, box, x+w-tableBadgePad, colors.ink)
	case CellDot:
		var l *text.Layout
		w := float32(tableDot)
		if s != "" {
			l = r.layoutOf(s, styleCell)
			w += tableKindGap + l.Size().W
		}
		x := left
		if col.trailing && w < room {
			x = right - w
		}
		dot := geom.Rect{X: x, Y: box.Y + geom.Floor32((box.H-tableDot)/2), W: tableDot, H: tableDot}
		cv.DrawRRect(geom.RRectAll(dot, tableDot/2), paint.Paint{Color: colors.fill})
		if l != nil {
			r.paintLayout(cv, l, x+tableDot+tableKindGap, box, right, c.th.Color.Ink)
		}
	case CellProgress:
		bar := room
		if col.hasText {
			bar = max(room-tableBarLabel-tableKindGap, 0)
		}
		track := geom.Rect{X: left, Y: box.Y + geom.Floor32((box.H-tableBarHeight)/2), W: bar, H: tableBarHeight}
		r.paintBar(cv, track, clampShare(d.value), colors.fill)
		if s != "" {
			l := r.layoutOf(s, styleCell)
			x := max(right-l.Size().W, left+bar+tableKindGap)
			r.paintLayout(cv, l, x, box, right, c.th.Color.Ink2)
		}
	case CellCheck:
		x := checkLeft(box, col.trailing)
		r.paintCheck(cv, geom.Rect{X: x, Y: box.Y + geom.Floor32((box.H-kit.CheckboxBox)/2), W: kit.CheckboxBox, H: kit.CheckboxBox}, d.checked, colors.fill)
		if s == "" {
			return
		}
		l := r.layoutOf(s, styleCell)
		if col.trailing {
			end := x - tableKindGap
			r.paintLayout(cv, l, max(end-l.Size().W, left), box, end, c.th.Color.Ink)
		} else {
			r.paintLayout(cv, l, x+kit.CheckboxBox+tableKindGap, box, right, c.th.Color.Ink)
		}
	}
}

// paintLayout draws l from x, in the middle of the height of box, cut at
// right when it is too long.
func (r *renderTable) paintLayout(cv *paint.Canvas, l *text.Layout, x float32, box geom.Rect, right float32, color geom.Color) {
	if right <= x {
		return
	}
	ts := l.Size()
	at := geom.Offset{X: x, Y: box.Y + geom.Floor32((box.H-ts.H)/2+0.5)}
	long := x+ts.W > right
	if long {
		cv.Save()
		cv.ClipRect(geom.Rect{X: x, Y: box.Y, W: right - x, H: box.H})
	}
	runs := l.Runs()
	for k := range runs {
		cv.DrawGlyphs(&runs[k], at, color)
	}
	if long {
		cv.Restore()
	}
}

// paintBar draws a bar in track filled to share, as layout.RenderProgressBar
// does.
func (r *renderTable) paintBar(cv *paint.Canvas, track geom.Rect, share float32, fill geom.Color) {
	if track.W <= 0 {
		return
	}
	radius := track.H / 2
	cv.DrawRRect(geom.RRectAll(track, radius), paint.Paint{Color: r.c.th.Color.FillStrong})
	done := track.W * share
	if done <= 0 {
		return
	}
	bar := track
	bar.W = max(done, track.H)
	if done < track.H {
		// A sliver narrower than the bar is tall stays a pill, not a square.
		bar.X -= track.H - done
		cv.Save()
		cv.ClipRect(track)
		cv.DrawRRect(geom.RRectAll(bar, radius), paint.Paint{Color: fill})
		cv.Restore()
		return
	}
	cv.DrawRRect(geom.RRectAll(bar, radius), paint.Paint{Color: fill})
}

// paintCheck draws a check box in box, as Checkbox draws one at rest.
func (r *renderTable) paintCheck(cv *paint.Canvas, box geom.Rect, checked bool, fill geom.Color) {
	col := &r.c.th.Color
	rr := geom.RRectAll(box, r.c.th.Radius.Check)
	if !checked {
		cv.DrawRRect(rr, paint.Paint{Stroke: &paint.Stroke{Width: 1, Color: col.Ink3}})
		return
	}
	cv.DrawRRect(rr, paint.Paint{Color: fill})
	cv.Save()
	cv.Translate(box.X, box.Y)
	cv.DrawPath(r.tick, paint.Paint{Stroke: &paint.Stroke{Width: 1.75, Color: col.OnAccent}})
	cv.Restore()
}

// tickPath returns the tick of a check box of checkboxBox, the shape of
// layout.RenderCheckmark when done.
func tickPath() *paint.Path {
	const s = kit.CheckboxBox
	p := &paint.Path{}
	p.MoveTo(0.24*s, 0.52*s)
	p.LineTo(0.42*s, 0.70*s)
	p.LineTo(0.76*s, 0.32*s)
	return p
}

// checkLeft returns the left of the check box of a cell in box: at the start
// of the cell, or at its end for a trailing column, whatever the text.
func checkLeft(box geom.Rect, trailing bool) float32 {
	if trailing {
		return box.X + box.W - tablePadding - kit.CheckboxBox
	}
	return box.X + tablePadding
}

// clampShare returns v between 0 and 1, and 0 for NaN.
func clampShare(v float32) float32 {
	if !(v > 0) {
		return 0
	}
	return min(v, 1)
}

// cellLabel returns what a screen reader reads for the cell of row in the
// column h: its text, the share of a bar without one, and the title of a
// check column with a ticked box and no text, so that the name of a row says
// it.
func (c *tableCore) cellLabel(row int, h *tableCol) string {
	s := c.src.cellText(row, h.model)
	if s != "" {
		return s
	}
	switch h.kind {
	case CellProgress:
		return kit.Percent(clampShare(c.src.cellData(row, h.model).value))
	case CellCheck:
		if c.src.cellData(row, h.model).checked {
			return h.title
		}
	}
	return ""
}

// onCheckBox reports whether pos, from the top left of the table, is on the
// check box of row and the column at j, or near enough to tick it.
func (c *tableCore) onCheckBox(row, j int, pos geom.Offset) bool {
	h := &c.cols[j]
	if h.kind != CellCheck || h.onCheck == nil {
		return false
	}
	x := checkLeft(geom.Rect{X: c.colX(j), W: h.width}, h.trailing)
	return pos.X >= x-tableCheckSlop && pos.X < x+kit.CheckboxBox+tableCheckSlop && row >= 0
}

// toggleCheck asks for the opposite of the box of row in the column at j, in
// the order they show, and puts the focus on the cell. The selection stays:
// the box is data, not a choice of rows.
func (c *tableCore) toggleCheck(row, j int) bool {
	if j < 0 || j >= len(c.cols) || row < 0 || row >= c.cfg.rows {
		return false
	}
	h := c.cols[j]
	if h.kind != CellCheck || h.onCheck == nil {
		return false
	}
	if c.editing && !c.commitEdit(true) {
		return false
	}
	c.ctrl.row, c.ctrl.col = row, h.model
	c.reveal(row, j)
	c.focusNow()
	c.changed()
	h.onCheck(row, !c.src.cellData(row, h.model).checked)
	return true
}

// tableCheckWidget is the widget of renderTableCheck.
type tableCheckWidget struct {
	c   *tableCore
	col int
}

// Key implements core.Widget.
func (tableCheckWidget) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (w tableCheckWidget) CreateRenderObject() core.RenderObject {
	return &renderTableCheck{c: w.c, col: w.col}
}

// UpdateRenderObject implements core.RenderWidget.
func (w tableCheckWidget) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderTableCheck)
	r.c, r.col = w.c, w.col
}

// renderTableCheck is the check box of a cell of a CellCheck column, in the
// column col, in the order they show. It draws nothing: it is the check box a
// screen reader finds in the cell, named by the title of the column, which it
// can toggle when the column has OnCheck.
type renderTableCheck struct {
	core.RenderBase
	c    *tableCore
	col  int
	cell *renderTableCell
}

// Layout implements core.RenderObject.
func (r *renderTableCheck) Layout(cs layout.Constraints) geom.Size {
	return cs.Constrain(geom.Size{W: cs.MaxW, H: cs.MaxH})
}

// Paint implements core.RenderObject. The table paints the box.
func (r *renderTableCheck) Paint(*paint.Canvas, geom.Offset) {}

// row returns the row of the box, or -1.
func (r *renderTableCheck) row() int {
	if r.cell == nil {
		return -1
	}
	return r.cell.dataRow()
}

// Semantics implements core.RenderObject.
func (r *renderTableCheck) Semantics(sb *core.SemanticsBuilder) {
	c := r.c
	row := r.row()
	if row < 0 || r.col >= len(c.cols) {
		return
	}
	h := &c.cols[r.col]
	node := semantics.Node{Role: semantics.RoleCheckbox, Label: h.title}
	if c.src.cellData(row, h.model).checked {
		node.State |= semantics.Checked
	}
	if h.onCheck != nil {
		node.Actions |= semantics.ActionTap
	} else {
		node.State |= semantics.ReadOnly
	}
	sb.Open(node, r.Size())
	sb.Close()
}

// PerformSemanticsAction implements core.SemanticsActor: a tap toggles the
// box.
func (r *renderTableCheck) PerformSemanticsAction(a semantics.Actions, _ core.SemanticsArg) bool {
	if a != semantics.ActionTap {
		return false
	}
	return r.c.toggleCheck(r.row(), r.col)
}
