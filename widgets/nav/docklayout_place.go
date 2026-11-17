package nav

import (
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
)

// Where the strips of hidden panels and the panel that slid out go, the focus
// they ask for, and the presses outside the flyout.

// placeStrips puts the strips of the hidden panels along their sides of a
// layout of size sz, the left and right ones the whole height, and leaves the
// room within them to the groups.
func (r *renderDock) placeStrips(sz geom.Size) {
	r.strips = [DockBottom + 1]geom.Rect{}
	r.inner = geom.Rect{W: sz.W, H: sz.H}
	if r.ctrl == nil {
		return
	}
	var has [DockBottom + 1]bool
	for i := range r.ctrl.hidden {
		has[r.ctrl.hidden[i].side] = true
	}
	const t = dockStripThickness
	in := &r.inner
	if has[DockLeft] {
		r.strips[DockLeft] = geom.Rect{W: t, H: sz.H}
		in.X += t
		in.W -= t
	}
	if has[DockRight] {
		r.strips[DockRight] = geom.Rect{X: sz.W - t, W: t, H: sz.H}
		in.W -= t
	}
	in.W = max(in.W, 0)
	if has[DockTop] {
		r.strips[DockTop] = geom.Rect{X: in.X, W: in.W, H: t}
		in.Y += t
		in.H -= t
	}
	if has[DockBottom] {
		r.strips[DockBottom] = geom.Rect{X: in.X, Y: sz.H - t, W: in.W, H: t}
		in.H -= t
	}
	in.H = max(in.H, 0)
}

// placeFlyout finds where the panel that slid out is, part of the way out as
// the slide goes, and the bounds of its size.
func (r *renderDock) placeFlyout() {
	r.fly, r.flyHit = geom.Rect{}, geom.Rect{}
	r.flyLen, r.flyLo, r.flyHi, r.flyTotal = 0, 0, 0, 0
	if r.ctrl == nil {
		return
	}
	h := r.ctrl.hiddenEntry(r.flyID)
	if h == nil {
		return
	}
	progress := float32(1)
	if r.slide != nil {
		progress = geom.Clamp32(r.slide.Value(), 0, 1)
	}
	r.flyLen, r.flyLo, r.flyHi, r.flyTotal = r.flyoutSize(h)
	r.fly = r.flyoutRect(h.side, r.flyLen, progress)
	const half = kit.SplitHit / 2
	hit := r.fly
	switch h.side {
	case DockLeft:
		hit.W += half
	case DockRight:
		hit.X -= half
		hit.W += half
	case DockTop:
		hit.H += half
	case DockBottom:
		hit.Y -= half
		hit.H += half
	}
	r.flyHit = hit
}

// flyoutSize returns the size of the hidden panel h when it is out, the least
// and most it may be, and the length of the room of the groups across its
// side.
func (r *renderDock) flyoutSize(h *dockHidden) (length, lo, hi, total float32) {
	total, lo = r.inner.W, dockMinGroupWidth
	if h.side == DockTop || h.side == DockBottom {
		total, lo = r.inner.H, dockMinGroupHeight
	}
	lo = min(lo, total)
	hi = max(total-dockFlyoutMargin, lo)
	length = h.size
	if length <= 0 {
		length = geom.Floor32(total * dockFlyoutShare)
	}
	return geom.Clamp32(length, lo, hi), lo, hi, total
}

// flyoutRect returns where a panel of length that comes out at side is, when
// progress of the way out, from 0 to 1.
func (r *renderDock) flyoutRect(side DockZone, length, progress float32) geom.Rect {
	in := r.inner
	out := length * progress
	switch side {
	case DockRight:
		return geom.Rect{X: in.X + in.W - out, Y: in.Y, W: length, H: in.H}
	case DockTop:
		return geom.Rect{X: in.X, Y: in.Y - length + out, W: in.W, H: length}
	case DockBottom:
		return geom.Rect{X: in.X, Y: in.Y + in.H - out, W: in.W, H: length}
	}
	return geom.Rect{X: in.X - length + out, Y: in.Y, W: length, H: in.H}
}

// paneRect returns where the content of the hidden panel id goes: below the
// header of the flyout, for the one out, and for the others where they would
// come out, so that they are not laid out again when they do.
func (r *renderDock) paneRect(id string) geom.Rect {
	rect := r.fly
	if id != r.flyID {
		h := r.ctrl.hiddenEntry(id)
		if h == nil {
			return geom.Rect{}
		}
		length, _, _, _ := r.flyoutSize(h)
		rect = r.flyoutRect(h.side, length, 1)
	}
	top := min(float32(tabBarHeight), rect.H)
	return geom.Rect{X: rect.X, Y: rect.Y + top, W: rect.W, H: rect.H - top}
}

// handleRect returns where the handle on the inner edge of the panel that
// slid out goes.
func (r *renderDock) handleRect() geom.Rect {
	h := r.ctrl.hiddenEntry(r.flyID)
	if h == nil {
		return geom.Rect{}
	}
	f := r.fly
	const back = (kit.SplitHit - dividerThickness) / 2
	switch h.side {
	case DockRight:
		return geom.Rect{X: f.X - back - dividerThickness, Y: f.Y, W: kit.SplitHit, H: f.H}
	case DockTop:
		return geom.Rect{X: f.X, Y: f.Y + f.H - back - dividerThickness, W: f.W, H: kit.SplitHit}
	case DockBottom:
		return geom.Rect{X: f.X, Y: f.Y - back - dividerThickness, W: f.W, H: kit.SplitHit}
	}
	return geom.Rect{X: f.X + f.W - back - dividerThickness, Y: f.Y, W: kit.SplitHit, H: f.H}
}

// takeFocus gives the keyboard where the controller asked for it.
func (r *renderDock) takeFocus() {
	f := r.ctrl.focus
	r.ctrl.focus = dockFocus{}
	var target core.FocusTarget
	switch f.kind {
	case dockFocusFlyout:
		target = r.flyoutFocus()
	case dockFocusStripTab:
		for i := range r.slots {
			if r.slots[i].kind != dockSlotSide {
				continue
			}
			if tab := stripTabOf(r.children[i], f.id); tab != nil {
				target = focusAbove(tab, r.children[i])
			}
		}
	case dockFocusPanel:
		if g := r.ctrl.groupOf(f.id); g != nil {
			target = r.groupFocus(g)
		}
	}
	if target != nil {
		core.RestoreFocus(target)
	}
}

// flyoutFocus returns the first control of the panel that slid out, or of its
// header.
func (r *renderDock) flyoutFocus() core.FocusTarget {
	for _, kind := range [...]dockSlotKind{dockSlotFlyoutPane, dockSlotFlyout} {
		for i := range r.slots {
			if sl := &r.slots[i]; sl.kind == kind && sl.panel == r.ctrl.flyout {
				if f := kit.FirstFocusable(r.children[i]); f != nil {
					return f
				}
			}
		}
	}
	return nil
}

// groupFocus returns the first control of the panel g shows, or its strip.
func (r *renderDock) groupFocus(g *dockNode) core.FocusTarget {
	for _, kind := range [...]dockSlotKind{dockSlotPane, dockSlotStrip} {
		for i := range r.slots {
			sl := &r.slots[i]
			if sl.kind != kind || sl.node != g || (kind == dockSlotPane && sl.panel != g.active) {
				continue
			}
			if f := kit.FirstFocusable(r.children[i]); f != nil {
				return f
			}
		}
	}
	return nil
}

// focusAbove returns the object that takes the focus at or above ro, up to
// top, or nil.
func focusAbove(ro, top core.RenderObject) core.FocusTarget {
	for ; ro != nil; ro = ro.Parent() {
		if f, ok := ro.(core.FocusTarget); ok && f.CanFocus() {
			return f
		}
		if ro == top {
			break
		}
	}
	return nil
}

// stripTabOf returns the tab of the hidden panel id below ro, or nil.
func stripTabOf(ro core.RenderObject, id string) core.RenderObject {
	if t, ok := ro.(*renderDockStripTab); ok && t.id == id {
		return t
	}
	for i := range ro.ChildCount() {
		if t := stripTabOf(ro.ChildAt(i), id); t != nil {
			return t
		}
	}
	return nil
}

// stripAt returns the side of the strip of hidden panels at p, and its room.
func (r *renderDock) stripAt(p geom.Offset) (DockZone, geom.Rect, bool) {
	for side := DockLeft; side <= DockBottom; side++ {
		if rect := r.strips[side]; rect.W > 0 && rect.Contains(p) {
			return side, rect, true
		}
	}
	return DockCenter, geom.Rect{}, false
}

// looking reports whether the layout watches the presses for one outside the
// panel that slid out.
func (r *renderDock) looking() bool {
	return r.onOutside != nil && r.ctrl != nil && r.ctrl.flyout != "" && r.ctrl.flyout == r.flyID && r.flyHit.W > 0
}

// HitTest implements core.RenderObject. While a panel is out, the layout goes
// on the path before its children, to see the presses outside it that they
// take; it never takes one.
func (r *renderDock) HitTest(pos geom.Offset, res *core.HitTestResult) bool {
	n := len(res.Path)
	if !r.RenderBase.HitTest(pos, res) {
		return false
	}
	if r.looking() {
		last := len(res.Path) - 1
		copy(res.Path[n+1:], res.Path[n:last])
		res.Path[n] = r
	}
	return true
}

// HandlePointer implements core.PointerTarget. It only looks.
func (r *renderDock) HandlePointer(e *core.PointerEvent) {
	if e.Kind != core.PointerDown || !r.looking() || r.flyHit.Contains(e.Pos) {
		return
	}
	if _, _, onStrip := r.stripAt(e.Pos); !onStrip {
		r.onOutside()
	}
}
