package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
)

// DecoratedBox paints a Decoration behind its child.
type DecoratedBox struct {
	Decoration layout.Decoration
	Child      core.Widget
}

// Key implements core.Widget.
func (DecoratedBox) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (d DecoratedBox) CreateRenderObject() core.RenderObject {
	return &layout.RenderDecoratedBox{Decoration: d.Decoration}
}

// UpdateRenderObject implements core.RenderWidget.
func (d DecoratedBox) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderDecoratedBox)
	if !r.Decoration.Equal(&d.Decoration) {
		r.Decoration = d.Decoration
		r.MarkNeedsPaint()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (d DecoratedBox) ChildWidget() core.Widget { return d.Child }

// Container combines the common ways of sizing, spacing and decorating a child.
// The zero value with no child is an empty box that takes no space.
//
// Without a child and without Width and Height, a Container is as small as its
// constraints allow: under loose constraints, as in a Center, a Column or a
// Stack, it is only as large as its Padding, and with none it takes no space
// and paints nothing, unlike a Container of Flutter, which fills the room it
// has. Give it a size, or put it
// where the constraints are tight (an Expanded, a PositionedFill, a SizedBox)
// to fill an area with a color.
//
// A Container with a Color, Gradient, Border or Shadows takes the pointer over
// its whole box; one that decorates nothing is hit only over its child (see
// Stack).
type Container struct {
	core.StatelessBase
	// Width and Height force a size. Zero leaves it to the child.
	Width, Height float32
	Padding       layout.Insets
	Color         geom.Color
	Gradient      *paint.Gradient
	// Border strokes the edge. The zero Stroke, of width 0, draws none. It is a
	// value, as in AnimatedContainer, so the two take the same literal.
	Border  paint.Stroke
	Radius  float32
	Shadows []paint.BoxShadow
	// Sink shrinks the box behind the child, as a pressed control does. See
	// layout.Decoration.
	Sink  float32
	Child core.Widget
}

// Build implements core.StatelessWidget.
func (c Container) Build(core.BuildContext) core.Widget {
	var border *paint.Stroke
	if c.Border.Width > 0 {
		border = &c.Border
	}
	return SizedBox{
		Width:  c.Width,
		Height: c.Height,
		Child: DecoratedBox{
			Decoration: layout.Decoration{
				Color: c.Color, Gradient: c.Gradient, Border: border, Radius: c.Radius, Shadows: c.Shadows, Sink: c.Sink,
			},
			Child: Padding{Insets: c.Padding, Child: c.Child},
		},
	}
}
