package basic

import "github.com/bankai-tensei/gowaku/core"

// RepaintBoundary keeps the picture of Child apart from what surrounds it. A
// change inside draws only Child again, and a change outside reuses its pixels,
// at the price of the memory that they take. Place one around what changes on
// its own or stays while its surroundings change. List does so for its rows.
type RepaintBoundary struct {
	Child core.Widget
}

// Key implements core.Widget.
func (RepaintBoundary) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (RepaintBoundary) CreateRenderObject() core.RenderObject {
	return core.NewRenderRepaintBoundary()
}

// UpdateRenderObject implements core.RenderWidget.
func (RepaintBoundary) UpdateRenderObject(core.RenderObject) {}

// ChildWidget implements core.SingleChildWidget.
func (r RepaintBoundary) ChildWidget() core.Widget { return r.Child }
