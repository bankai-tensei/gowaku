package core

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
)

// DragPhase says where a drag over the window is.
type DragPhase uint8

// The drag phases.
const (
	// DragOver is the pointer moving over the window with data, the first
	// time included.
	DragOver DragPhase = iota
	// DragLeave is the drag leaving the window, or being canceled.
	DragLeave
	// DragDrop is the data being dropped.
	DragDrop
)

// DragEvent is a drag over the window, from this or another application.
type DragEvent struct {
	// Pos is the pointer in logical pixels. Targets see it relative to
	// themselves; DispatchDrag takes it relative to the window.
	Pos geom.Offset
	// Kinds is what the drag carries. Data holds it only for DragDrop: other
	// programs hand the data over only on a drop.
	Kinds input.DataKinds
	Data  input.DragData
	// Allowed is what the source lets the drop do.
	Allowed   input.DropEffect
	Modifiers input.Modifiers
}

// DropTarget is a RenderObject that takes drops. The deepest one under the
// pointer whose DragMove accepts the drag gets the drop. See
// docs/ADR/0050-drag-and-drop.md.
type DropTarget interface {
	RenderObject
	// DragMove reports what a drop here would do, DropNone to refuse it.
	DragMove(e *DragEvent) input.DropEffect
	// DragExit reports that the drag left the object, or ended elsewhere.
	DragExit()
	// Drop takes the data, and returns what it did.
	Drop(e *DragEvent) input.DropEffect
}

// DragHost is a Host that can start a drag to other places and programs. A
// Host need not implement it.
type DragHost interface {
	// StartDrag drags d from the press in progress, allowing the effects in
	// allowed, and calls done with what the drop did, DropNone if it was
	// canceled.
	StartDrag(d input.DragData, allowed input.DropEffect, done func(input.DropEffect))
}

// DispatchDrag delivers a drag over the window, and returns what a drop at its
// place does.
func (t *Tree) DispatchDrag(phase DragPhase, e DragEvent) input.DropEffect {
	if phase == DragLeave {
		t.setDropTarget(nil)
		return input.DropNone
	}
	target, effect := t.dropTargetAt(&e)
	t.setDropTarget(target)
	if phase == DragDrop {
		t.setDropTarget(nil)
		if target == nil {
			return input.DropNone
		}
		local := e
		local.Pos, _ = GlobalToLocal(target, e.Pos)
		return target.Drop(&local)
	}
	return effect
}

// dropTargetAt returns the deepest target at e.Pos that accepts e, with what
// it would do.
func (t *Tree) dropTargetAt(e *DragEvent) (DropTarget, input.DropEffect) {
	t.hit(e.Pos)
	for _, ro := range t.hits.Path {
		target, ok := ro.(DropTarget)
		if !ok {
			continue
		}
		local := *e
		pos, ok := GlobalToLocal(ro, e.Pos)
		if !ok {
			continue
		}
		local.Pos = pos
		if effect := target.DragMove(&local); effect != input.DropNone {
			return target, effect
		}
	}
	return nil, input.DropNone
}

func (t *Tree) setDropTarget(target DropTarget) {
	if t.dropTarget != nil && t.dropTarget != target && t.dropTarget.base().tree != nil {
		t.dropTarget.DragExit()
	}
	t.dropTarget = target
}
