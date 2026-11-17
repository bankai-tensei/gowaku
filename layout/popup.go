package layout

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
)

// RenderShortcuts hands the key presses that nothing in focus used to
// OnShortcut. See core.ShortcutTarget.
type RenderShortcuts struct {
	RenderProxy
	// OnShortcut returns true when it used the press.
	OnShortcut func(e *core.KeyEvent) bool
}

// HandleShortcut implements core.ShortcutTarget.
func (r *RenderShortcuts) HandleShortcut(e *core.KeyEvent) bool {
	return r.OnShortcut != nil && r.OnShortcut(e)
}

// PopupSide says where a popup goes from its anchor.
type PopupSide uint8

// The popup sides.
const (
	// PopupBelow opens under the anchor, aligned to its left edge, as a menu
	// of a menu bar does, and above it when there is no room below.
	PopupBelow PopupSide = iota
	// PopupBeside opens to the right of the anchor, aligned to its top, as a
	// submenu does, and to the left when there is no room on the right.
	PopupBeside
)

// RenderPopup fills its parent and places its child next to Anchor, in the
// coordinates of the parent, keeping it inside. Only the child takes the
// pointer, so that what lies under the rest stays usable.
type RenderPopup struct {
	RenderProxy
	Anchor geom.Rect
	Side   PopupSide
	// Gap is the room left between the anchor and the child, and Inset the
	// room the child keeps from the edges of the parent.
	Gap, Inset float32
	// Shift moves the child along the side: up for PopupBeside, so that the
	// first item of a submenu lines up with the item that opened it.
	Shift float32
}

// Layout implements core.RenderObject.
func (r *RenderPopup) Layout(c Constraints) geom.Size {
	sz := c.Biggest()
	if r.child == nil {
		return sz
	}
	room := geom.Size{W: max(sz.W-2*r.Inset, 0), H: max(sz.H-2*r.Inset, 0)}
	csz := core.LayoutChild(r.child, geom.Constraints{MaxW: room.W, MaxH: room.H})
	r.child.SetOffset(r.place(sz, csz))
	return sz
}

// place returns the top left corner of a child of size csz in a parent of size
// sz.
func (r *RenderPopup) place(sz, csz geom.Size) geom.Offset {
	a := r.Anchor
	var x, y float32
	switch r.Side {
	case PopupBeside:
		x = a.X + a.W + r.Gap
		if x+csz.W > sz.W-r.Inset && a.X-r.Gap-csz.W >= r.Inset {
			x = a.X - r.Gap - csz.W
		}
		y = a.Y - r.Shift
	default:
		x = a.X
		y = a.Y + a.H + r.Gap
		if y+csz.H > sz.H-r.Inset && a.Y-r.Gap-csz.H >= r.Inset {
			y = a.Y - r.Gap - csz.H
		}
	}
	x = geom.Clamp32(x, r.Inset, max(sz.W-r.Inset-csz.W, r.Inset))
	y = geom.Clamp32(y, r.Inset, max(sz.H-r.Inset-csz.H, r.Inset))
	return geom.Offset{X: x, Y: y}
}

// HitTest implements core.RenderObject. The object takes the pointer only over
// its child.
func (r *RenderPopup) HitTest(pos geom.Offset, res *core.HitTestResult) bool {
	if r.child == nil || !r.child.HitTest(pos.Sub(r.child.Offset()), res) {
		return false
	}
	res.Path = append(res.Path, r)
	return true
}

// RenderBarrier takes every pointer press and wheel turn over it, from any
// button, and reports the presses: the way out of a menu by a click beside it.
type RenderBarrier struct {
	RenderProxy
	OnPress func()
}

// Layout implements core.RenderObject.
func (r *RenderBarrier) Layout(c Constraints) geom.Size {
	sz := c.Biggest()
	if r.child != nil {
		core.LayoutChild(r.child, geom.Tight(sz))
	}
	return sz
}

// HandlePointer implements core.PointerTarget.
func (r *RenderBarrier) HandlePointer(e *core.PointerEvent) {
	switch e.Kind {
	case core.PointerDown:
		e.Handled = true
		if r.OnPress != nil {
			r.OnPress()
		}
	case core.PointerScroll:
		e.Handled = true
	}
}
