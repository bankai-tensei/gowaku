package collection

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/theme"
)

// paintHeader draws the header, and the column being dragged over it.
func (r *renderTable) paintHeader(cv *paint.Canvas, off geom.Offset) {
	c := r.c
	col := &c.th.Color
	w := r.Size().W
	band := geom.Rect{X: off.X, Y: off.Y, W: w, H: tableHeaderHeight}
	cv.DrawRRect(geom.RRectAll(band, 0), paint.Paint{Color: col.Card})
	if c.frozen < len(c.cols) {
		cv.Save()
		cv.ClipRect(geom.Rect{X: off.X + c.frozenW, Y: off.Y, W: max(w-c.frozenW, 0), H: tableHeaderHeight})
		for j := c.frozen; j < len(c.cols); j++ {
			r.paintHead(cv, off, j)
		}
		cv.Restore()
	}
	for j := range c.frozen {
		r.paintHead(cv, off, j)
	}
	cv.DrawRRect(geom.RRectAll(geom.Rect{X: off.X, Y: off.Y + tableHeaderHeight - 1, W: w, H: 1}, 0), paint.Paint{Color: col.Hair})
	if c.focusRow() < 0 && r.ringShown() {
		r.paintRing(cv, off, -1, c.focusCol())
	}
	grip := -1
	switch {
	case c.drag.kind == tableDragResize:
		grip = c.drag.col
	case c.drag.kind == tableDragNone && c.hoverZone.kind == zoneGrip:
		grip = c.hoverZone.col
	}
	if grip >= 0 && grip < len(c.cols) {
		x := off.X + c.colX(grip) + c.cols[grip].width
		cv.DrawRRect(geom.RRectAll(geom.Rect{X: x - 1, Y: off.Y + theme.Space4, W: 2, H: tableHeaderHeight - 2*theme.Space4}, 1),
			paint.Paint{Color: col.AccentInk})
	}
	if c.drag.kind == tableDragHeader && c.drag.moved {
		r.paintDraggedHead(cv, off)
	}
}

// paintHead draws the header of column j.
func (r *renderTable) paintHead(cv *paint.Canvas, off geom.Offset, j int) {
	c := r.c
	color := &c.th.Color
	h := &c.cols[j]
	x := off.X + c.colX(j)
	w := r.Size().W
	if x >= off.X+w || x+h.width <= off.X {
		return
	}
	if c.drag.kind == tableDragHeader && c.drag.moved && c.drag.col == j {
		// The column is under the pointer; its place stays empty.
		cv.DrawRRect(geom.RRectAll(geom.Rect{X: x, Y: off.Y, W: h.width, H: tableHeaderHeight}, 0), paint.Paint{Color: color.Fill})
		return
	}
	box := geom.Rect{X: x, Y: off.Y, W: h.width, H: tableHeaderHeight}
	r.paintHeadContent(cv, box, j, color.Ink2)
	if j < len(c.cols)-1 {
		sep := geom.Rect{X: x + h.width - 1, Y: off.Y + (tableHeaderHeight-tableSeparator)/2, W: 1, H: tableSeparator}
		cv.DrawRRect(geom.RRectAll(sep, 0), paint.Paint{Color: color.Hair})
	}
}

// paintHeadContent draws the title, the sort arrow and the filter button of
// column j in box.
func (r *renderTable) paintHeadContent(cv *paint.Canvas, box geom.Rect, j int, ink geom.Color) {
	c := r.c
	color := &c.th.Color
	h := &c.cols[j]
	end := box.W
	if r.filterShown(j) {
		end -= tableFilterInset + tableFilterButton
		btn := geom.Rect{X: box.X + end, Y: box.Y + (tableHeaderHeight-tableFilterButton)/2, W: tableFilterButton, H: tableFilterButton}
		if c.hoverZone == (tableZone{kind: zoneFilter, col: j}) || c.filter.col == h.model {
			cv.DrawRRect(geom.RRectAll(btn, c.th.Radius.Check), paint.Paint{Color: color.Fill})
		}
		glyph := color.Ink2
		if h.filter.Active() {
			glyph = color.AccentInk
		}
		cv.Save()
		cv.Translate(btn.X+(tableFilterButton-tableFilterGlyph)/2, btn.Y+(tableFilterButton-tableFilterGlyph)/2)
		cv.DrawPath(r.funnel, paint.Paint{Color: glyph})
		cv.Restore()
		// The title may run up to the button, short of the padding.
		end += tablePadding - theme.Space4
	}
	var arrow *paint.Path
	switch {
	case c.cfg.sort.Column != h.model:
	case c.cfg.sort.Order == SortAscending:
		arrow = r.sortUp
	case c.cfg.sort.Order == SortDescending:
		arrow = r.sortDown
	}
	room := end
	if arrow != nil {
		room -= sortChevronWidth + tableSortGap
	}
	if h.title != "" && room > 2*tablePadding {
		r.paintText(cv, h.title, styleHead, geom.Rect{X: box.X, Y: box.Y, W: room, H: box.H}, h.trailing, ink)
	}
	if arrow != nil {
		tw := float32(0)
		if h.title != "" {
			tw = min(r.layoutOf(h.title, styleHead).Size().W, max(room-2*tablePadding, 0))
		}
		ax := box.X + tablePadding + tw + tableSortGap
		if h.trailing {
			ax = box.X + room - tablePadding + tableSortGap
		}
		cv.Save()
		cv.Translate(ax, box.Y+geom.Floor32((tableHeaderHeight-sortChevronHeight)/2))
		cv.DrawPath(arrow, paint.Paint{Color: ink})
		cv.Restore()
	}
}

// filterShown reports whether the header of column j shows its filter button:
// when the filter holds something back, is open, or the pointer or the
// keyboard is on the header.
func (r *renderTable) filterShown(j int) bool {
	c := r.c
	h := &c.cols[j]
	if !h.filterable {
		return false
	}
	return h.filter.Active() || c.filter.col == h.model || c.hoverHeader == j || c.hoverZone.col == j && c.hoverZone.kind != zoneNone ||
		c.focusRow() < 0 && c.focusCol() == j && r.ringShown()
}

// paintDraggedHead draws the header being moved under the pointer, and a line
// where it would land.
func (r *renderTable) paintDraggedHead(cv *paint.Canvas, off geom.Offset) {
	c := r.c
	color := &c.th.Color
	j := c.drag.col
	h := &c.cols[j]
	at := c.dropIndex()
	lineX := float32(float64(c.totalW) - c.horz.Offset.Value())
	if at < len(c.cols) {
		lineX = c.colX(at)
	}
	cv.DrawRRect(geom.RRectAll(geom.Rect{X: off.X + lineX - 1, Y: off.Y, W: 2, H: r.Size().H}, 1), paint.Paint{Color: color.Accent})
	box := geom.Rect{X: off.X + c.colX(j) + c.drag.x - c.drag.start, Y: off.Y + 2, W: h.width, H: tableHeaderHeight - 4}
	rr := geom.RRectAll(box, c.th.Radius.Control)
	for _, s := range color.Shadows(theme.LiftFloat) {
		cv.DrawShadow(rr, s.Blur, s.Color, s.Offset)
	}
	cv.DrawRRect(rr, paint.Paint{Color: color.Raised, Stroke: &paint.Stroke{Width: 1, Color: color.Hair}})
	r.paintHeadContent(cv, geom.Rect{X: box.X, Y: off.Y, W: box.W, H: tableHeaderHeight}, j, color.Ink)
}

// sortChevronWidth and sortChevronHeight are the size of the sort arrow.
const (
	sortChevronWidth  = 8
	sortChevronHeight = 5
)

// sortChevron returns the arrow of a sorted column, pointing up for
// ascending.
func sortChevron(up bool) *paint.Path {
	pts := [...]geom.Offset{{X: 0, Y: 1.1}, {X: 1.1, Y: 0}, {X: 4, Y: 2.9}, {X: 6.9, Y: 0}, {X: 8, Y: 1.1}, {X: 4, Y: 5}}
	p := &paint.Path{}
	for i, pt := range pts {
		if up {
			pt.Y = sortChevronHeight - pt.Y
		}
		if i == 0 {
			p.MoveTo(pt.X, pt.Y)
		} else {
			p.LineTo(pt.X, pt.Y)
		}
	}
	p.Close()
	return p
}

// funnelPath returns the filter glyph, a funnel in a square of
// tableFilterGlyph.
func funnelPath() *paint.Path {
	p := &paint.Path{}
	p.MoveTo(0.5, 1.5)
	p.LineTo(11.5, 1.5)
	p.LineTo(7.2, 6.6)
	p.LineTo(7.2, 10.4)
	p.LineTo(4.8, 11.6)
	p.LineTo(4.8, 6.6)
	p.Close()
	return p
}
