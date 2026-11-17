package paint

import "github.com/bankai-tensei/gowaku/geom"

// TextLayout is text set into glyphs that DrawText can draw. A *text.Layout is
// one; paint does not import text, so it names what it needs here.
type TextLayout interface {
	// Size is the box of the text.
	Size() geom.Size
	// Baseline is the distance from the top of the box to the baseline of the
	// first line.
	Baseline() float32
	// Runs are the glyphs, relative to the top left corner of the box.
	Runs() []GlyphRun
}

// TextAnchor says which point of a text DrawText puts at its position. The
// zero value is the top left corner of the text's box.
type TextAnchor struct {
	// X is 0 at the left edge of the text, 0.5 in its middle and 1 at its
	// right edge.
	X float32
	// Y is 0 at the top of the box, 0.5 in its middle and 1 at its bottom.
	Y float32
	// Baseline puts the baseline of the first line at the position instead,
	// and leaves Y out: a label of a chart tick or a ruler then sits on the
	// line it labels, whatever its font.
	Baseline bool
}

// DrawText draws t so that its anchor point is at pos, with color for the
// glyphs that have no color of their own. It records the runs of t and keeps
// pointers to them, so t must stay unchanged until the next Reset, as with
// DrawGlyphs. It draws glyphs only, not the backgrounds of spans.
//
// DrawText does not allocate, which makes it fit for a layout.Painter: lay the
// text out where a BuildContext is, in Build, and hand the Layout to the
// painter. text.Labels keeps the layouts of labels that the painter picks
// only while it paints, such as the numbers of a ruler that depend on its
// size. X measures the box of t, so lay a label out at an infinite length
// (geom.Inf); one laid out at a finite length with AlignCenter or AlignEnd
// is as wide as that length.
func (c *Canvas) DrawText(t TextLayout, pos geom.Offset, anchor TextAnchor, color geom.Color) {
	sz := t.Size()
	off := geom.Offset{X: pos.X - sz.W*anchor.X, Y: pos.Y - sz.H*anchor.Y}
	if anchor.Baseline {
		off.Y = pos.Y - t.Baseline()
	}
	runs := t.Runs()
	for i := range runs {
		c.DrawGlyphs(&runs[i], off, color)
	}
}
