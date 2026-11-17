package wtest

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
)

// LayoutCount counts in *N the layouts of the subtree below it.
type LayoutCount struct {
	N     *int
	Child core.Widget
}

type renderLayoutCount struct {
	layout.RenderProxy
	n *int
}

// Layout counts the layout and passes it to the child.
func (r *renderLayoutCount) Layout(c layout.Constraints) geom.Size {
	*r.n++
	return r.RenderProxy.Layout(c)
}

// Key implements core.Widget.
func (LayoutCount) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderObjectWidget.
func (l LayoutCount) CreateRenderObject() core.RenderObject {
	return &renderLayoutCount{n: l.N}
}

// UpdateRenderObject implements core.RenderObjectWidget.
func (LayoutCount) UpdateRenderObject(core.RenderObject) {}

// ChildWidget implements core.SingleChildWidget.
func (l LayoutCount) ChildWidget() core.Widget { return l.Child }
