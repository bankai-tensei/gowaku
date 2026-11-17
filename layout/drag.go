package layout

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
)

// dragThreshold is how far the pointer moves with the button down before a
// press becomes a drag, in logical pixels.
const dragThreshold = 4

// RenderDropTarget takes drops of the kinds it accepts. See core.DropTarget.
type RenderDropTarget struct {
	RenderProxy
	// Kinds are those it accepts, and Effect what it does with them; it
	// falls back to another effect that the source allows.
	Kinds  input.DataKinds
	Effect input.DropEffect
	// OnHover reports the drag coming over and leaving the object.
	OnHover func(over bool)
	// OnDrop takes the data, with the place of the drop.
	OnDrop func(d input.DragData, pos geom.Offset)

	over bool
}

// accept returns what a drop of e does here.
func (r *RenderDropTarget) accept(e *core.DragEvent) input.DropEffect {
	if r.OnDrop == nil || e.Kinds&r.Kinds == 0 {
		return input.DropNone
	}
	want := r.Effect
	if want == input.DropNone {
		want = input.DropCopy
	}
	if effect := e.Allowed.Pick(want); effect != input.DropNone {
		return effect
	}
	return e.Allowed.Pick(input.DropCopy | input.DropMove | input.DropLink)
}

// DragMove implements core.DropTarget.
func (r *RenderDropTarget) DragMove(e *core.DragEvent) input.DropEffect {
	effect := r.accept(e)
	r.hover(effect != input.DropNone)
	return effect
}

// DragExit implements core.DropTarget.
func (r *RenderDropTarget) DragExit() { r.hover(false) }

// Drop implements core.DropTarget.
func (r *RenderDropTarget) Drop(e *core.DragEvent) input.DropEffect {
	r.hover(false)
	effect := r.accept(e)
	if effect != input.DropNone {
		r.OnDrop(e.Data, e.Pos)
	}
	return effect
}

func (r *RenderDropTarget) hover(over bool) {
	if r.over != over {
		r.over = over
		if r.OnHover != nil {
			r.OnHover(over)
		}
	}
}

// RenderDraggable starts a drag of Data when its child is pressed and the
// pointer moves a few pixels. A press that does not move is a tap.
type RenderDraggable struct {
	RenderProxy
	Data    input.DragData
	Allowed input.DropEffect
	// OnTap runs for a press that does not become a drag.
	OnTap func()
	// OnDragStart runs when the drag starts, and OnDragEnd when it ends, with
	// what the drop did.
	OnDragStart func()
	OnDragEnd   func(effect input.DropEffect)

	pressed  bool
	dragging bool
	start    geom.Offset
}

// HandlePointer implements core.PointerTarget.
func (r *RenderDraggable) HandlePointer(e *core.PointerEvent) {
	if e.Button != input.ButtonPrimary || r.Data.Kinds() == 0 {
		return
	}
	switch e.Kind {
	case core.PointerDown:
		if r.dragging {
			return
		}
		e.Handled = true
		r.pressed, r.start = true, e.Pos
	case core.PointerMove:
		d := e.Pos.Sub(r.start)
		if !r.pressed || d.X*d.X+d.Y*d.Y < dragThreshold*dragThreshold {
			return
		}
		r.pressed = false
		r.startDrag()
	case core.PointerUp:
		if r.pressed {
			r.pressed = false
			if r.OnTap != nil {
				r.OnTap()
			}
		}
	case core.PointerCancel:
		r.pressed = false
	}
}

func (r *RenderDraggable) startDrag() {
	host, ok := r.Host().(core.DragHost)
	if !ok {
		return
	}
	allowed := r.Allowed
	if allowed == input.DropNone {
		allowed = input.DropCopy
	}
	r.dragging = true
	if r.OnDragStart != nil {
		r.OnDragStart()
	}
	host.StartDrag(r.Data, allowed, func(effect input.DropEffect) {
		r.dragging = false
		if r.OnDragEnd != nil {
			r.OnDragEnd(effect)
		}
	})
}
