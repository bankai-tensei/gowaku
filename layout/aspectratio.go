package layout

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
)

// RenderAspectRatio sizes itself to a width over height of Ratio, and gives
// its child exactly that size. See AspectRatioSize.
type RenderAspectRatio struct {
	RenderProxy
	Ratio float32
}

// Layout implements core.RenderObject.
func (r *RenderAspectRatio) Layout(c Constraints) geom.Size {
	sz := AspectRatioSize(c, r.Ratio)
	if r.child != nil {
		core.LayoutChild(r.child, geom.Tight(sz))
	}
	return sz
}

// AspectRatioSize returns the size within c whose width over height is ratio,
// or as close to it as c allows. It takes the most width that c allows and the
// height that goes with it, or, with an unbounded width, the most height; then
// it shrinks to fit the maximums and grows to meet the minimums, keeping the
// ratio as long as it can. Tight constraints leave the ratio no say. A ratio of
// zero is 1, a square.
//
// It panics if ratio is negative, or if both lengths of c are unbounded.
func AspectRatioSize(c Constraints, ratio float32) geom.Size {
	if ratio < 0 {
		panic("gowaku: AspectRatio needs a Ratio above zero")
	}
	if ratio == 0 {
		ratio = 1
	}
	if c.IsTight() {
		return c.Constrain(geom.Size{})
	}
	var w, h float32
	switch {
	case c.MaxW < geom.Inf:
		w = c.MaxW
		h = w / ratio
	case c.MaxH < geom.Inf:
		h = c.MaxH
		w = h * ratio
	default:
		panic("gowaku: AspectRatio has an unbounded width and height, as inside a Row in a Scroll; give it a width or a height with a SizedBox or an Expanded")
	}
	if w > c.MaxW {
		w, h = c.MaxW, c.MaxW/ratio
	}
	if h > c.MaxH {
		h, w = c.MaxH, c.MaxH*ratio
	}
	if w < c.MinW {
		w, h = c.MinW, c.MinW/ratio
	}
	if h < c.MinH {
		h, w = c.MinH, c.MinH*ratio
	}
	return c.Constrain(geom.Size{W: w, H: h})
}
