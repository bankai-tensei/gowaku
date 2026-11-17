package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
)

// CustomLayout arranges its children with a layout.Delegate.
type CustomLayout struct {
	Delegate layout.Delegate
	Children []core.Widget
}

// Key implements core.Widget.
func (CustomLayout) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (l CustomLayout) CreateRenderObject() core.RenderObject {
	return &layout.RenderCustomLayout{Delegate: l.Delegate}
}

// UpdateRenderObject implements core.RenderWidget. A delegate can hide any
// state, and need not be comparable, so every update lays out again.
func (l CustomLayout) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderCustomLayout)
	r.Delegate = l.Delegate
	r.MarkNeedsLayout()
}

// ChildWidgets implements core.MultiChildWidget.
func (l CustomLayout) ChildWidgets() []core.Widget { return l.Children }
