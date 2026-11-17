package basic

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// paragraphSelection is the highlight a selection area puts on a
// RenderParagraph. A paragraph outside any area never gets one, so all it
// costs there is a nil check in Paint. See docs/ADR/0089-selection-area.md.
type paragraphSelection struct {
	start, end int
	color      geom.Color
	// rects is reused from one paint to the next.
	rects []geom.Rect
}

// SetSelection highlights the bytes of the text from start to end with color,
// beneath the glyphs, line by line as the text is set: across columns in
// vertical text, and in pieces where right-to-left runs reorder a line. It is
// the part of field.SelectionArea that only the paragraph can do. Offsets
// inside a cluster, such as a ruby group, move to its start, and offsets past
// the end of the text stop there. An empty range shows nothing.
func (r *RenderParagraph) SetSelection(start, end int, color geom.Color) {
	if start > end {
		start, end = end, start
	}
	s := r.selection
	if s == nil {
		if start == end {
			return
		}
		s = &paragraphSelection{}
		r.selection = s
	}
	if s.start == start && s.end == end && (s.color == color || start == end) {
		return
	}
	s.start, s.end, s.color = start, end, color
	r.MarkNeedsPaint()
}

// ClearSelection removes the highlight of SetSelection.
func (r *RenderParagraph) ClearSelection() {
	if s := r.selection; s != nil && s.start != s.end {
		s.start, s.end = 0, 0
		r.MarkNeedsPaint()
	}
}

// Selection returns the range SetSelection highlights, in order, or 0, 0.
func (r *RenderParagraph) Selection() (start, end int) {
	if s := r.selection; s != nil {
		return s.start, s.end
	}
	return 0, 0
}

// paintSelection fills the selected range under the glyphs at off. It does
// not allocate once rects has grown to the number of lines.
func (r *RenderParagraph) paintSelection(cv *paint.Canvas, off geom.Offset) {
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
