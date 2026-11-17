package interact

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
)

// DragTarget takes text or files dropped on its child, from this window, from
// another gowaku window, or from another program such as Explorer, Nautilus or
// a browser. See docs/ADR/0050-drag-and-drop.md.
//
//	DragTarget{Kinds: input.DataFiles, OnDrop: s.open, OnHover: s.light, Child: area}
//
// A drag inside the app goes through the system too, so OnHover says only
// whether the drag is over the child, not where. To move items between two
// lists, make each item a DragTarget whose drop means "insert before it", and
// put one more after the last item for "append". In a test, drop with the
// DropText and DropTextAt of gowakutest.Tester.
type DragTarget struct {
	// Kinds are what it takes. The zero value takes both text and files.
	Kinds input.DataKinds
	// Effect is what a drop does, DropCopy unless set. Where the source does
	// not allow it, another effect it allows is used.
	Effect input.DropEffect
	// OnHover reports a drag that it would take coming over the child and
	// leaving it, to light it up.
	OnHover func(over bool)
	// OnDrop takes the data, with the place of the drop relative to the
	// child. Without it nothing is taken.
	OnDrop func(d input.DragData, pos geom.Offset)
	Child  core.Widget
}

func (t DragTarget) kinds() input.DataKinds {
	if t.Kinds == 0 {
		return input.DataText | input.DataFiles
	}
	return t.Kinds
}

// Key implements core.Widget.
func (DragTarget) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (t DragTarget) CreateRenderObject() core.RenderObject {
	r := &layout.RenderDropTarget{}
	t.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (t DragTarget) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderDropTarget)
	r.Kinds, r.Effect, r.OnHover, r.OnDrop = t.kinds(), t.Effect, t.OnHover, t.OnDrop
}

// ChildWidget implements core.SingleChildWidget.
func (t DragTarget) ChildWidget() core.Widget { return t.Child }

// Draggable drags Data out of its child when it is pressed and the pointer
// moves a few pixels: to a DragTarget, another window, or another program. A
// press that does not move runs OnTap. Put it around what takes no press of
// its own, such as the row of a list: a button inside keeps its presses.
//
// The drag is the system's drag and drop, in the app as well: the system draws
// the image that follows the pointer, and the app draws no ghost of the item
// and does not learn where the pointer is until the drop. To move something
// inside one view while the pointer moves, use the drag callbacks of
// GestureDetector, or a ReorderableList for the rows of a list.
type Draggable struct {
	Data input.DragData
	// Allowed is what the drop may do, DropCopy unless set. A move is the
	// application's to carry out, in OnDragEnd.
	Allowed     input.DropEffect
	OnTap       func()
	OnDragStart func()
	// OnDragEnd reports what the drop did, DropNone if it was canceled or
	// refused.
	OnDragEnd func(effect input.DropEffect)
	Child     core.Widget
}

// Key implements core.Widget.
func (Draggable) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (d Draggable) CreateRenderObject() core.RenderObject {
	r := &layout.RenderDraggable{}
	d.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (d Draggable) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderDraggable)
	r.Data, r.Allowed, r.OnTap, r.OnDragStart, r.OnDragEnd = d.Data, d.Allowed, d.OnTap, d.OnDragStart, d.OnDragEnd
}

// ChildWidget implements core.SingleChildWidget.
func (d Draggable) ChildWidget() core.Widget { return d.Child }
