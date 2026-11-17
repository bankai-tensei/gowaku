package collection

import (
	"slices"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	overlayw "github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// The measures of the filter of a column.
const (
	tableFilterWidth = 240
	// tableFilterRows is how many values show before the list scrolls, and
	// tableFilterRow the height of one.
	tableFilterRows = 7
	tableFilterRow  = 28
)

// tableFilterPopup is the filter of a column, open over the table.
type tableFilterPopup struct {
	// col is the column of Table.Columns whose filter is open, or -1.
	col   int
	entry *overlayw.Entry
	layer *kit.PopupLayerState
	// text is the text of the field, and off holds the values that are not
	// ticked, by their index in FilterValues.
	text string
	off  []bool
	// restore had the keyboard when the filter opened.
	restore core.FocusTarget
}

// openFilter opens the filter of the column at j, in the order they show,
// below its header. A second call for the same column closes it.
func (c *tableCore) openFilter(j int) {
	if j < 0 || j >= len(c.cols) || !c.cols[j].filterable || c.ro == nil || !c.ro.Attached() {
		return
	}
	h := c.cols[j]
	if c.filter.entry != nil {
		same := c.filter.col == h.model
		c.closeFilter(same)
		if same {
			return
		}
	}
	f := &c.filter
	f.col, f.text = h.model, h.filter.Text
	f.off = f.off[:0]
	for _, v := range h.values {
		f.off = append(f.off, h.filter.Values != nil && !slices.Contains(h.filter.Values, v))
	}
	f.restore = c.ro.Focused()
	at := core.LocalToGlobal(c.ro, geom.Offset{X: c.colX(j)})
	overlay := overlayw.Of(c.base.Context())
	anchor := kit.OverlayRect(overlay.Context().RenderObject(), geom.Rect{X: at.X, Y: at.Y, W: h.width, H: tableHeaderHeight})
	f.entry = overlay.InsertFrom(c.base.Context(), func() core.Widget {
		return kit.PopupLayer{Build: func() core.Widget { return c.filterCard(anchor) }, State: &c.filter.layer}
	})
	c.repaint()
}

// closeFilter takes the filter away. restore gives the keyboard back to what
// had it.
func (c *tableCore) closeFilter(restore bool) {
	f := &c.filter
	if f.entry == nil {
		return
	}
	f.entry.Remove()
	f.entry, f.layer, f.col = nil, nil, -1
	switch {
	case !restore || f.restore == nil:
	case c.owns(f.restore):
		// The cell that had the keyboard may show another one by now. The
		// keyboard is still in the filter, until it leaves the window; the
		// next layout of the table takes it then.
		c.refocus = true
		c.relayout()
	default:
		core.RestoreFocus(f.restore)
	}
	f.restore = nil
	c.repaint()
}

// applyFilter hands the filter as it stands to OnFilter.
func (c *tableCore) applyFilter() {
	f := &c.filter
	if f.col < 0 || f.col >= len(c.models) || c.cfg.onFilter == nil {
		return
	}
	out := TableFilter{Text: f.text}
	if slices.Contains(f.off, true) {
		out.Values = []string{}
		for i, v := range c.models[f.col].values {
			if i < len(f.off) && !f.off[i] {
				out.Values = append(out.Values, v)
			}
		}
	}
	c.cfg.onFilter(f.col, out)
	if f.layer != nil {
		kit.Rebuild(&f.layer.StateBase)
	}
}

// filterCard builds the filter of the open column under anchor, a rectangle
// of the overlay.
func (c *tableCore) filterCard(anchor geom.Rect) core.Widget {
	f := &c.filter
	th, l := c.th, c.words
	if f.col < 0 || f.col >= len(c.models) {
		return basic.SizedBox{}
	}
	h := c.models[f.col]
	parts := []core.Widget{field.Input{
		Value: f.text, Placeholder: l.Filter,
		OnChange: func(s string) {
			f.text = s
			c.applyFilter()
		},
	}}
	if len(h.values) > 0 {
		parts = append(parts, c.filterValues(h.values))
	}
	parts = append(parts, basic.Row{Main: layout.MainEnd, Children: []core.Widget{control.Button{
		Label: l.Clear, Style: control.ButtonQuiet, Size: control.ButtonSmall,
		OnPress: func() {
			f.text = ""
			clear(f.off)
			c.applyFilter()
		},
	}}})
	card := basic.Container{
		Width:   tableFilterWidth,
		Color:   th.Color.Raised,
		Radius:  th.Radius.Group,
		Border:  paint.Stroke{Width: 1, Color: th.Color.Hair},
		Shadows: th.Color.Shadows(theme.LiftFloat),
		Padding: geom.All(theme.Space12),
		Child:   basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: theme.Space8, Children: parts},
	}
	return basic.Theme{Data: th, Child: basic.MediaQuery{Data: c.media, Child: kit.ModalScope{Active: true, Child: interact.Focus{
		OnKey: func(e *core.KeyEvent) bool {
			if e.Key != input.KeyEscape {
				return false
			}
			c.closeFilter(true)
			return true
		},
		Child: basic.Stack{Fit: layout.StackExpand, Children: []core.Widget{
			kit.Barrier{OnPress: func() { c.closeFilter(true) }},
			kit.Popup{Anchor: anchor, Side: layout.PopupBelow, Child: interact.Semantics{
				Role: semantics.RoleDialog, Label: l.Filter + " " + h.title,
				// A press on the card must not reach the barrier.
				Child: interact.GestureDetector{OnTapDown: func() {}, Child: focusFirst{child: card}},
			}},
		}},
	}}}}
}

// filterValues builds the list of values to tick, under one that ticks them
// all.
func (c *tableCore) filterValues(values []string) core.Widget {
	f := &c.filter
	all := !slices.Contains(f.off, true)
	rows := make([]core.Widget, 0, len(values)+1)
	rows = append(rows, basic.SizedBox{Height: tableFilterRow, Child: basic.Align{Alignment: layout.AlignCenterLeft, Child: control.Checkbox{
		Value: all, Label: c.words.AllValues,
		OnChange: func(on bool) {
			for i := range f.off {
				f.off[i] = !on
			}
			c.applyFilter()
		},
	}}})
	for i, v := range values {
		rows = append(rows, basic.SizedBox{Height: tableFilterRow, Child: basic.Align{Alignment: layout.AlignCenterLeft, Child: control.Checkbox{
			Value: i < len(f.off) && !f.off[i], Label: v,
			OnChange: func(on bool) {
				if i < len(f.off) {
					f.off[i] = !on
					c.applyFilter()
				}
			},
		}}})
	}
	list := basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Children: rows}
	if len(rows) <= tableFilterRows {
		return list
	}
	return basic.SizedBox{Height: tableFilterRows * tableFilterRow, Child: basic.Scroll{Child: list}}
}

// owns reports whether f is the table or one of its cells.
func (c *tableCore) owns(f core.FocusTarget) bool {
	if cell, ok := f.(*renderTableCell); ok {
		return cell.c == c
	}
	return c.ro != nil && core.FocusTarget(c.ro) == f
}
