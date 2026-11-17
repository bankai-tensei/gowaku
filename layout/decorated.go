package layout

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// Decoration is the look of a box. The zero value draws nothing. Gradient
// coordinates are relative to the box.
type Decoration struct {
	Color    geom.Color
	Gradient *paint.Gradient
	Border   *paint.Stroke
	Radius   float32
	Shadows  []paint.BoxShadow
	// Sink shrinks the box around its center, from 0 for none, while the child
	// stays where it is. A pressed control sinks this way: scaling the label too
	// would make its glyphs jump between pixel sizes.
	Sink float32
}

// Equal reports whether d and o look the same.
func (d *Decoration) Equal(o *Decoration) bool {
	if d.Color != o.Color || d.Radius != o.Radius || d.Sink != o.Sink || len(d.Shadows) != len(o.Shadows) {
		return false
	}
	for i := range d.Shadows {
		if d.Shadows[i] != o.Shadows[i] {
			return false
		}
	}
	return sameValue(d.Gradient, o.Gradient) && sameValue(d.Border, o.Border)
}

func sameValue[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// RenderDecoratedBox paints a Decoration behind its child.
type RenderDecoratedBox struct {
	RenderProxy
	Decoration Decoration
}

// Paint implements core.RenderObject.
func (r *RenderDecoratedBox) Paint(cv *paint.Canvas, off geom.Offset) {
	d := &r.Decoration
	sz := r.Size()
	cv.Save()
	cv.Translate(off.X, off.Y)
	if d.Sink != 0 {
		cv.Translate(sz.W*d.Sink/2, sz.H*d.Sink/2)
		cv.Scale(1 - d.Sink)
	}
	rect := geom.RectFromSize(sz)
	for i := range d.Shadows {
		s := &d.Shadows[i]
		grown := geom.Rect{X: -s.Spread, Y: -s.Spread, W: sz.W + 2*s.Spread, H: sz.H + 2*s.Spread}
		cv.DrawShadow(geom.RRectAll(grown, d.Radius+s.Spread), s.Blur, s.Color, s.Offset)
	}
	cv.DrawRRect(geom.RRectAll(rect, d.Radius), paint.Paint{Color: d.Color, Gradient: d.Gradient, Stroke: d.Border})
	cv.Restore()
	r.RenderProxy.Paint(cv, off)
}
