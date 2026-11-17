package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// Divider is a hairline between groups of content, in the theme's Hair. The zero
// value is a horizontal line across the width it is given, as between the rows
// of a list; Axis: layout.Vertical stands it up across the height, as between
// the buttons of a toolbar. Indent and EndIndent keep the line short of the
// start and the end, as a Group stops its lines short of the left edge.
//
// Where the length along the line is unbounded, as for a horizontal Divider in a
// Row, the line has no length: give it a width with SizedBox.
type Divider struct {
	core.StatelessBase
	Axis layout.Axis
	// Indent and EndIndent are the room left bare before and after the line:
	// at the left and right of a horizontal line, at the top and bottom of a
	// vertical one.
	Indent, EndIndent float32
	// Color replaces the theme's Hair. The zero value selects Hair.
	Color geom.Color
}

// Build implements core.StatelessWidget.
func (d Divider) Build(ctx core.BuildContext) core.Widget {
	color := d.Color
	if color == (geom.Color{}) {
		color = ThemeOf(ctx).Color.Hair
	}
	return dividerLine{axis: d.Axis, indent: d.Indent, endIndent: d.EndIndent, color: color}
}

// dividerLine is the widget of renderDivider.
type dividerLine struct {
	axis              layout.Axis
	indent, endIndent float32
	color             geom.Color
}

// Key implements core.Widget.
func (dividerLine) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (d dividerLine) CreateRenderObject() core.RenderObject {
	return &renderDivider{axis: d.axis, indent: d.indent, endIndent: d.endIndent, color: d.color}
}

// UpdateRenderObject implements core.RenderWidget.
func (d dividerLine) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderDivider)
	if r.axis != d.axis {
		r.axis = d.axis
		r.MarkNeedsLayout()
	}
	if r.indent != d.indent || r.endIndent != d.endIndent || r.color != d.color {
		r.indent, r.endIndent, r.color = d.indent, d.endIndent, d.color
		r.MarkNeedsPaint()
	}
}

// renderDivider takes the whole length it is allowed along its axis, and the
// width of the line across it.
type renderDivider struct {
	core.RenderBase
	axis              layout.Axis
	indent, endIndent float32
	color             geom.Color
}

// Layout implements core.RenderObject.
func (r *renderDivider) Layout(c geom.Constraints) geom.Size {
	if r.axis == layout.Vertical {
		return c.Constrain(geom.Size{W: kit.DividerThickness, H: kit.FiniteOr(c.MaxH, c.MinH)})
	}
	return c.Constrain(geom.Size{W: kit.FiniteOr(c.MaxW, c.MinW), H: kit.DividerThickness})
}

// Paint implements core.RenderObject.
func (r *renderDivider) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	line := geom.Rect{X: off.X + r.indent, Y: off.Y + (sz.H-kit.DividerThickness)/2, W: sz.W - r.indent - r.endIndent, H: kit.DividerThickness}
	if r.axis == layout.Vertical {
		line = geom.Rect{X: off.X + (sz.W-kit.DividerThickness)/2, Y: off.Y + r.indent, W: kit.DividerThickness, H: sz.H - r.indent - r.endIndent}
	}
	if line.W <= 0 || line.H <= 0 {
		return
	}
	cv.DrawRRect(geom.RRect{Rect: line}, paint.Paint{Color: r.color})
}

// Semantics implements core.RenderObject.
func (r *renderDivider) Semantics(sb *core.SemanticsBuilder) {
	sb.Open(semantics.Node{Role: semantics.RoleSeparator}, r.Size())
	sb.Close()
}
