package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
)

// Transform paints its child through an affine transform around Origin: turned,
// slanted or stretched. Layout ignores it, as in Flutter, so the child keeps its
// upright place and the picture overlaps its neighbors. Unlike Scale and
// Translate, the pointer, the IME and assistive technology follow the picture: a
// turned button is hit inside its turned box only, and a screen reader gets the
// upright box around it.
//
// Shapes keep their exact edges and text stays sharp; see
// docs/ADR/0044-drawing-extensions.md. For motions that only move or scale, Scale
// and Translate stay the cheaper choice.
//
//	Rotate(angle, Card{...})
type Transform struct {
	// Transform is applied around Origin. The zero value changes nothing.
	Transform geom.Affine
	// Origin is the point that stays in place, 0..1 on each axis (not -1..1 as
	// in Flutter). The zero value is the top left; layout.AlignCenter turns
	// around the middle.
	Origin layout.Alignment
	Child  core.Widget
}

// Key implements core.Widget.
func (Transform) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t Transform) CreateRenderObject() core.RenderObject {
	return &layout.RenderTransform{Transform: t.Transform, Origin: t.Origin}
}

// UpdateRenderObject implements core.RenderWidget.
func (t Transform) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderTransform)
	if r.Transform != t.Transform || r.Origin != t.Origin {
		r.Transform, r.Origin = t.Transform, t.Origin
		r.MarkNeedsPaint()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (t Transform) ChildWidget() core.Widget { return t.Child }

// Rotate returns a Transform that turns child by radians around its center.
// Positive angles turn clockwise on screen.
func Rotate(radians float32, child core.Widget) Transform {
	return Transform{Transform: geom.Rotation(radians), Origin: layout.AlignCenter, Child: child}
}

// Skew returns a Transform that slants child around its center: x moves by
// tan(x) times the height above the center, and y by tan(y) times the width. The
// angles are in radians.
func Skew(x, y float32, child core.Widget) Transform {
	return Transform{Transform: geom.Skewing(x, y), Origin: layout.AlignCenter, Child: child}
}
