package render

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
)

// thinLine is the length in physical pixels below which a line rounds down, so
// that a hairline of one logical pixel stays one physical pixel at 150 percent.
const thinLine = 2

// snapLength returns the physical length of a logical one: never zero for what has
// any length at all. The same length always gives the same result, wherever it
// lies, which is what keeps every hairline of a window alike.
func snapLength(logical, scale float32) float32 {
	v := logical * scale
	switch {
	case v <= 0:
		return 0
	case v < thinLine:
		return max(geom.Floor32(v), 1)
	}
	return geom.Floor32(v + 0.5)
}

// snap moves a rounded rectangle onto physical pixels, where its edges are crisp.
// Without this, a line of one logical pixel covers two or three physical rows
// with weights that depend on where the layout left it.
//
// The border always snaps: its width must not change when a press starts to scale
// the box. The box itself stays put while scaled, because a transition has to move
// by less than a pixel, and so does a circle, because a thumb that slides must not
// step and has no straight edge to sharpen.
func snap(in *gpu.Instance, op *paint.Op, scale float32) {
	in.BorderWidth = snapLength(in.BorderWidth, scale) / scale
	r := in.Rect
	if op.Zoom() != 1 || 2*op.RRect.TopLeft >= min(r.W, r.H) {
		return
	}
	in.Rect = geom.Rect{
		X: geom.Floor32(r.X*scale+0.5) / scale,
		Y: geom.Floor32(r.Y*scale+0.5) / scale,
		W: snapLength(r.W, scale) / scale,
		H: snapLength(r.H, scale) / scale,
	}
}
