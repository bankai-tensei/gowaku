package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
)

// Flex lays out its children in a line along Axis. Row and Column are the
// usual way to write it.
type Flex struct {
	Axis     layout.Axis
	Main     layout.MainAxisAlignment
	Cross    layout.CrossAxisAlignment
	Size     layout.MainAxisSize
	Spacing  float32
	Children []core.Widget
}

// Key implements core.Widget.
func (Flex) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (f Flex) CreateRenderObject() core.RenderObject {
	return &layout.RenderFlex{Axis: f.Axis, Main: f.Main, Cross: f.Cross, MainSize: f.Size, Spacing: f.Spacing}
}

// UpdateRenderObject implements core.RenderWidget.
func (f Flex) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderFlex)
	if r.Axis != f.Axis || r.Main != f.Main || r.Cross != f.Cross || r.MainSize != f.Size || r.Spacing != f.Spacing {
		r.Axis, r.Main, r.Cross, r.MainSize, r.Spacing = f.Axis, f.Main, f.Cross, f.Size, f.Spacing
		r.MarkNeedsLayout()
	}
}

// ChildWidgets implements core.MultiChildWidget.
func (f Flex) ChildWidgets() []core.Widget { return f.Children }

// Row lays out its children from left to right. It takes all the width it is
// allowed, as in Flutter, so that Expanded children have something to share;
// Size: MainMin shrinks it to its children. Across, the children line up at
// the top, since the zero value of Cross is layout.CrossStart, unlike Flutter,
// which centers them: set Cross to layout.CrossCenter for a row of an icon
// and a label, or layout.CrossStretch to make them as tall as the row. See
// Column.
type Row struct {
	core.StatelessBase
	Main     layout.MainAxisAlignment
	Cross    layout.CrossAxisAlignment
	Size     layout.MainAxisSize
	Spacing  float32
	Children []core.Widget
}

// Build implements core.StatelessWidget.
func (r Row) Build(core.BuildContext) core.Widget {
	return Flex{Axis: layout.Horizontal, Main: r.Main, Cross: r.Cross, Size: r.Size, Spacing: r.Spacing, Children: r.Children}
}

// Column lays out its children from top to bottom. It takes all the height it
// is allowed, as in Flutter, so that Expanded children have something to share.
// That also holds inside a Center, which then has nothing to center: the
// children sit at the top. Set Size to MainMin, or write VStack, for a column
// as tall as its children. Across, the children line up at the left (the
// start), since the zero value of Cross is layout.CrossStart, unlike Flutter,
// which centers them: set Cross to layout.CrossCenter to center them, or
// layout.CrossStretch to make them as wide as the column.
type Column struct {
	core.StatelessBase
	Main     layout.MainAxisAlignment
	Cross    layout.CrossAxisAlignment
	Size     layout.MainAxisSize
	Spacing  float32
	Children []core.Widget
}

// Build implements core.StatelessWidget.
func (c Column) Build(core.BuildContext) core.Widget {
	return Flex{Axis: layout.Vertical, Main: c.Main, Cross: c.Cross, Size: c.Size, Spacing: c.Spacing, Children: c.Children}
}

// Flexible lets a child of a Row or Column share the free space. The child may
// stay smaller than its share. Flex is its weight, where zero counts as one.
type Flexible struct {
	Flex  int
	Fit   layout.FlexFit
	Child core.Widget
}

// Key implements core.Widget.
func (Flexible) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (f Flexible) CreateRenderObject() core.RenderObject {
	return &layout.RenderFlexible{Flex: f.Flex, Fit: f.Fit}
}

// UpdateRenderObject implements core.RenderWidget.
func (f Flexible) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderFlexible)
	if r.Flex != f.Flex || r.Fit != f.Fit {
		r.Flex, r.Fit = f.Flex, f.Fit
		// The share is decided by the parent, which reads these fields.
		r.Parent().MarkNeedsLayout()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (f Flexible) ChildWidget() core.Widget { return f.Child }

// Expanded is a Flexible whose child must fill its share.
type Expanded struct {
	core.StatelessBase
	Flex  int
	Child core.Widget
}

// Build implements core.StatelessWidget.
func (e Expanded) Build(core.BuildContext) core.Widget {
	return Flexible{Flex: e.Flex, Fit: layout.FitTight, Child: e.Child}
}
