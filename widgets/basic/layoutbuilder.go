package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// LayoutBuilder builds its content with the constraints its parent gives it,
// so that the content can depend on the room it has: two columns when wide,
// one when narrow, or an image as large as the space. For a choice about the
// whole window, such as folding a sidebar away, WindowSizeOf is simpler.
//
//	basic.LayoutBuilder{Content: func(ctx core.BuildContext, c layout.Constraints) core.Widget {
//		if c.MaxW < 600 {
//			return narrow
//		}
//		return wide
//	}}
//
// The content is built again when the constraints change, when the parent
// rebuilds the LayoutBuilder, and when something it reads from ctx changes,
// such as the theme. A frame whose layout leaves the constraints as they were
// builds nothing. The LayoutBuilder is as large as its content; a nil Content
// builds nothing, and the LayoutBuilder is then as small as allowed.
//
// The constraints are known only once the layout reaches the LayoutBuilder, so
// the content is built then: the layout reports them, and the frame builds and
// lays out again before it paints, as a collection.List does when it learns
// its height. Each LayoutBuilder nested in the content of another adds such a
// round, and a frame runs only a few before painting, so deep nesting shows the
// inner content a frame late. See docs/ADR/0130-layout-builder-aspect-ratio-viewer-keys.md.
type LayoutBuilder struct {
	core.StatefulBase
	Content func(ctx core.BuildContext, c layout.Constraints) core.Widget
}

// CreateState implements core.StatefulWidget.
func (LayoutBuilder) CreateState() core.State { return &layoutBuilderState{} }

type layoutBuilderState struct {
	core.StateOf[LayoutBuilder]
	// c is what the last layout gave, which known says has happened.
	c     layout.Constraints
	known bool
}

// Build implements core.State.
func (s *layoutBuilderState) Build(ctx core.BuildContext) core.Widget {
	var child core.Widget
	if f := s.Widget().Content; f != nil && s.known {
		child = f(ctx, s.c)
	}
	return layoutBuilderBox{s: s, child: child}
}

// laidOut takes the constraints of a layout, and builds the content again
// when they changed.
func (s *layoutBuilderState) laidOut(c layout.Constraints) {
	if s.known && s.c == c {
		return
	}
	s.c, s.known = c, true
	kit.Rebuild(&s.StateBase)
}

// layoutBuilderBox is the render widget of a LayoutBuilder.
type layoutBuilderBox struct {
	s     *layoutBuilderState
	child core.Widget
}

// Key implements core.Widget.
func (layoutBuilderBox) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (b layoutBuilderBox) CreateRenderObject() core.RenderObject {
	return &renderLayoutBuilder{s: b.s}
}

// UpdateRenderObject implements core.RenderWidget.
func (b layoutBuilderBox) UpdateRenderObject(ro core.RenderObject) {
	ro.(*renderLayoutBuilder).s = b.s
}

// ChildWidget implements core.SingleChildWidget.
func (b layoutBuilderBox) ChildWidget() core.Widget { return b.child }

// renderLayoutBuilder hands the constraints of each layout to its state. The
// child it lays out then was built for the constraints before; if they
// changed, the state builds it again and it is laid out again before the
// frame paints.
type renderLayoutBuilder struct {
	layout.RenderProxy
	s *layoutBuilderState
}

// HitTestSelf implements core.SelfHitTester. The builder is hit only over its
// child, as the layout objects are.
func (*renderLayoutBuilder) HitTestSelf(geom.Offset) bool { return false }

// Layout implements core.RenderObject.
func (r *renderLayoutBuilder) Layout(c layout.Constraints) geom.Size {
	r.s.laidOut(c)
	return r.RenderProxy.Layout(c)
}
