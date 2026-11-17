package field

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// The geometry of a Input in terms of its lines. In horizontal text the
// lines run along x and stack down y. In vertical text they run down y and stack
// leftwards from the right edge, so that the first line is always at cross 0.

// vertical reports whether the field sets its text in columns.
func (r *RenderEditable) vertical() bool { return r.field.Style.Orientation == text.Vertical }

// fieldInsets returns the room around the text of a field whose lines are line
// high: a single line sits in the middle of a control.
func fieldInsets(vertical bool, line float32) geom.Insets {
	across := max((fieldHeight-line)/2, 0)
	if vertical {
		return geom.Symmetric(across, fieldPaddingX)
	}
	return geom.Symmetric(fieldPaddingX, across)
}

// innerLength returns the length of a line in a field of size sz.
func (r *RenderEditable) innerLength(sz geom.Size) float32 {
	p := r.insets
	if r.vertical() {
		return max(sz.H-p.Top-p.Bottom, 0)
	}
	return max(sz.W-p.Left-p.Right, 0)
}

// innerCross returns the extent of a field of size sz across its lines.
func (r *RenderEditable) innerCross(sz geom.Size) float32 {
	p := r.insets
	if r.vertical() {
		return max(sz.W-p.Left-p.Right, 0)
	}
	return max(sz.H-p.Top-p.Bottom, 0)
}

// textCross returns the extent of the text across its lines.
func (r *RenderEditable) textCross() float32 {
	if r.vertical() {
		return r.laidOut.Size().W
	}
	return r.laidOut.Size().H
}

// textLength returns the length of the longest line.
func (r *RenderEditable) textLength() float32 {
	if r.vertical() {
		return r.laidOut.Size().H
	}
	return r.laidOut.Size().W
}

// crossRange returns where rect, in layout coordinates, lies across the lines.
func (r *RenderEditable) crossRange(rect geom.Rect) (start, end float32) {
	if r.vertical() {
		w := r.laidOut.Size().W
		return w - rect.X - rect.W, w - rect.X
	}
	return rect.Y, rect.Y + rect.H
}

// caretBar turns the caret of the layout, which has no extent along the line,
// into the bar to draw.
func (r *RenderEditable) caretBar(c geom.Rect) geom.Rect {
	if r.vertical() {
		c.H = kit.CaretWidth
	} else {
		c.W = kit.CaretWidth
	}
	return c
}

// logicalKey turns an arrow key into the one that does the same in horizontal
// text. In vertical text Down goes on along the line and Left on to the next
// line.
func (r *RenderEditable) logicalKey(key input.KeyCode) input.KeyCode {
	if !r.vertical() {
		return key
	}
	switch key {
	case input.KeyUp:
		return input.KeyLeft
	case input.KeyDown:
		return input.KeyRight
	case input.KeyLeft:
		return input.KeyDown
	case input.KeyRight:
		return input.KeyUp
	}
	return key
}

// underline returns the mark under a clause that covers rect: below it in
// horizontal text, right of it in vertical text (JLReq 3.3, as side lines are
// set). A gap at each end shows where one clause ends.
func (r *RenderEditable) underline(rect geom.Rect, thickness float32) geom.Rect {
	if r.vertical() {
		return geom.Rect{X: rect.X + rect.W - thickness - 1, Y: rect.Y + 1, W: thickness, H: max(rect.H-2, 1)}
	}
	return geom.Rect{X: rect.X + 1, Y: rect.Y + rect.H - thickness - 1, W: max(rect.W-2, 1), H: thickness}
}
