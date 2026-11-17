package markdown

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// What a copy puts between the texts of a document (SelectionBreak): a blank
// line between blocks, a line break between the items of a tight list and
// between the rows of a table, and a tab between the cells of a row. See
// docs/ADR/0105-markdown-selection.md.
const (
	blockBreak = "\n\n"
	lineBreak  = "\n"
	cellBreak  = "\t"
)

var _ basic.SelectableText = (*renderRich)(nil)

// richSelection is the highlight a field.SelectionArea puts on a renderRich,
// as basic.RenderParagraph has it: a paragraph outside an area never gets
// one, so all it costs there is a nil check in Paint.
type richSelection struct {
	start, end int
	color      geom.Color
	// rects is reused from one paint to the next.
	rects []geom.Rect
}

// TextLayout implements basic.SelectableText. It is nil before the first
// layout and for text that is not to be selected, which keeps it out of a
// selection.
func (r *renderRich) TextLayout() *text.Layout {
	if r.noSelect {
		return nil
	}
	return r.laidOut
}

// TextOrigin implements basic.SelectableText: the text moves right as Align
// asks.
func (r *renderRich) TextOrigin() geom.Offset { return geom.Offset{X: r.shift} }

// SelectionBreak implements basic.SelectableText.
func (r *renderRich) SelectionBreak() string {
	if r.brk == "" {
		return lineBreak
	}
	return r.brk
}

// SetSelection implements basic.SelectableText.
func (r *renderRich) SetSelection(start, end int, color geom.Color) {
	if start > end {
		start, end = end, start
	}
	s := r.selection
	if s == nil {
		if start == end {
			return
		}
		s = &richSelection{}
		r.selection = s
	}
	if s.start == start && s.end == end && (s.color == color || start == end) {
		return
	}
	s.start, s.end, s.color = start, end, color
	r.MarkNeedsPaint()
}

// ClearSelection implements basic.SelectableText.
func (r *renderRich) ClearSelection() {
	if s := r.selection; s != nil && s.start != s.end {
		s.start, s.end = 0, 0
		r.MarkNeedsPaint()
	}
}

// Selection implements basic.SelectableText.
func (r *renderRich) Selection() (start, end int) {
	if s := r.selection; s != nil {
		return s.start, s.end
	}
	return 0, 0
}

// paintSelection fills the selected range at off, which already holds the
// shift of the text, over the grounds of code and under the glyphs. It does
// not allocate once rects has grown to the number of lines.
func (r *renderRich) paintSelection(cv *paint.Canvas, off geom.Offset) {
	s := r.selection
	if s.start >= s.end || r.laidOut == nil {
		return
	}
	end := min(s.end, len(r.shaped.Text()))
	s.rects = r.laidOut.SelectionRects(min(s.start, end), end, s.rects[:0])
	for i := range s.rects {
		cv.DrawRRect(geom.RRectAll(s.rects[i].Translate(off), 0), paint.Paint{Color: s.color})
	}
}
