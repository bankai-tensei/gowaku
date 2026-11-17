package nav

import (
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
)

// dockSlotKind says what a child of a DockLayout is.
type dockSlotKind uint8

const (
	// dockSlotPane is the content of a panel, below the strip of its group.
	dockSlotPane dockSlotKind = iota
	// dockSlotStrip is the tab strip of a group.
	dockSlotStrip
	// dockSlotDivider is the handle between two children of a split.
	dockSlotDivider
	// dockSlotSide is the strip of the hidden panels at the side index.
	dockSlotSide
	// dockSlotFlyout is the ground and header of the hidden panel that slid
	// out, dockSlotFlyoutPane the content of a hidden panel, and
	// dockSlotFlyoutHandle the handle on the inner edge of the one out.
	dockSlotFlyout
	dockSlotFlyoutPane
	dockSlotFlyoutHandle
	// dockSlotKeys takes the keys of the layout. It comes last, so that it
	// is asked before the strips while the focus is outside the layout, and
	// has no size.
	dockSlotKeys
)

// dockSlot says where a child of a DockLayout goes: in node, a group for a
// pane or a strip, or, for a divider, after child index of the split node; a
// strip of hidden panels goes along the side index.
type dockSlot struct {
	kind  dockSlotKind
	node  *dockNode
	index int
	// panel is the ID of the panel of a pane.
	panel string
}

// The keys of the children of a DockLayout, which keep their elements as the
// tree changes: a panel keeps its state when it moves to another group.
type (
	dockPanelKey        string
	dockGroupKey        int
	dockDividerKey      struct{ split, index int }
	dockKeysKey         struct{}
	dockSideKey         DockZone
	dockFlyoutKey       struct{}
	dockFlyoutHandleKey struct{}
)

// dockView is the widget of renderDock.
type dockView struct {
	ctrl *DockController
	// fly is the hidden panel drawn over the layout, out or sliding back in.
	fly    string
	ref    **renderDock
	slots  []dockSlot
	ground geom.Color
	canvas geom.Color
	fill   geom.Color
	edge   geom.Color
	slide  *motion.AnimationController
	// onOutside reports a press outside the panel that slid out, and
	// onEntryLeft the focus leaving a menu or a dialog opened from within the
	// layout.
	onOutside, onEntryLeft func()
	children               []core.Widget
}

// Key implements core.Widget.
func (dockView) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (v dockView) CreateRenderObject() core.RenderObject {
	r := &renderDock{}
	v.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (v dockView) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderDock)
	r.ctrl, r.flyID = v.ctrl, v.fly
	clear(r.slots)
	r.slots = append(r.slots[:0], v.slots...)
	r.ground, r.canvas, r.fill, r.stroke = v.ground, v.canvas, v.fill, paint.Stroke{Width: 1, Color: v.edge}
	r.slide, r.onOutside, r.onEntryLeft = v.slide, v.onOutside, v.onEntryLeft
	// The tree may have changed shape; a new layout is cheap next to the
	// build that brought it.
	r.MarkNeedsLayout()
	*v.ref = r
}

// ChildWidgets implements core.MultiChildWidget.
func (v dockView) ChildWidgets() []core.Widget { return v.children }

// renderDock lays out the groups of a DockLayout by the tree of its
// controller: each group a tab strip over the panes of its panels, and the
// dividers between the children of each split. It draws the ground of the
// groups below them, and the preview of a drop above them.
type renderDock struct {
	core.RenderBase
	ctrl *DockController
	// flyID is the hidden panel drawn over the layout, out or sliding back
	// in; the controller's flyout is the one out.
	flyID    string
	slots    []dockSlot
	children []core.RenderObject
	ground   geom.Color
	// canvas is the ground of the strips of a layout within another, which
	// would show the ground of a group of the outer one.
	canvas geom.Color
	fill   geom.Color
	stroke paint.Stroke
	// preview is where a dragged tab would go, in the layout, while showing.
	preview     geom.Rect
	previewShow bool
	slide       *motion.AnimationController
	onOutside   func()
	onEntryLeft func()
	// inner is the room of the groups, within the strips of hidden panels,
	// and strips where the strips are, by side, empty for none.
	inner  geom.Rect
	strips [DockBottom + 1]geom.Rect
	// fly is where the panel that slid out is, and flyHit the room it takes
	// the pointer in, with its handle; flyLen is its size, within flyLo and
	// flyHi, of flyTotal, the length of inner across its side.
	fly, flyHit                    geom.Rect
	flyLen, flyLo, flyHi, flyTotal float32
}

// SetChildren implements core.MultiChildRenderObject.
func (r *renderDock) SetChildren(children []core.RenderObject) { r.children = children }

// ChildCount implements core.RenderObject.
func (r *renderDock) ChildCount() int { return len(r.children) }

// ChildAt implements core.RenderObject.
func (r *renderDock) ChildAt(i int) core.RenderObject { return r.children[i] }

// setPreview shows where a dragged tab would go, or nothing when show is
// false.
func (r *renderDock) setPreview(rect geom.Rect, show bool) {
	if !show {
		rect = geom.Rect{}
	}
	if r.preview != rect || r.previewShow != show {
		r.preview, r.previewShow = rect, show
		r.MarkNeedsPaint()
	}
}

// Layout implements core.RenderObject.
func (r *renderDock) Layout(c geom.Constraints) geom.Size {
	if c.MaxW >= geom.Inf || c.MaxH >= geom.Inf {
		panic("gowaku: DockLayout has an unbounded width or height, as inside a Scroll, a Row or a Column; give it a size with a SizedBox or Expanded")
	}
	sz := geom.Size{W: c.MaxW, H: c.MaxH}
	r.placeStrips(sz)
	if r.ctrl != nil && r.ctrl.root != nil {
		r.ctrl.root.place(r.inner)
		if m := r.ctrl.maximized; m != nil {
			m.place(r.inner)
		}
	}
	r.placeFlyout()
	for i, child := range r.children {
		rect := r.slotRect(&r.slots[i])
		core.LayoutChild(child, geom.Tight(rect.Size()))
		child.SetOffset(rect.Origin())
	}
	if r.ctrl != nil && r.ctrl.focus.kind != dockFocusNone {
		r.takeFocus()
	}
	return sz
}

// slotRect returns where the child of s goes.
func (r *renderDock) slotRect(s *dockSlot) geom.Rect {
	switch s.kind {
	case dockSlotPane:
		return s.node.body()
	case dockSlotStrip:
		rect := s.node.rect
		rect.H = min(rect.H, tabBarHeight)
		return rect
	case dockSlotKeys:
		return geom.Rect{}
	case dockSlotSide:
		return r.strips[s.index]
	case dockSlotFlyout:
		return r.fly
	case dockSlotFlyoutPane:
		return r.paneRect(s.panel)
	case dockSlotFlyoutHandle:
		return r.handleRect()
	}
	split := s.node
	if s.index+1 >= len(split.children) {
		return geom.Rect{}
	}
	at := split.children[s.index].rect
	const back = (kit.SplitHit - dividerThickness) / 2
	if split.axis == layout.Vertical {
		return geom.Rect{X: split.rect.X, Y: at.Y + at.H - back, W: split.rect.W, H: kit.SplitHit}
	}
	return geom.Rect{X: at.X + at.W - back, Y: split.rect.Y, W: kit.SplitHit, H: split.rect.H}
}

// Paint implements core.RenderObject.
func (r *renderDock) Paint(cv *paint.Canvas, off geom.Offset) {
	if r.nested() {
		sz := r.Size()
		cv.DrawRRect(geom.RRect{Rect: geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H}}, paint.Paint{Color: r.canvas})
	}
	if r.ctrl != nil && r.ctrl.root != nil {
		if m := r.ctrl.maximized; m != nil {
			r.paintGround(cv, off, m)
		} else {
			r.paintGround(cv, off, r.ctrl.root)
		}
	}
	clipped := false
	for i, child := range r.children {
		if !dockPainted(child) {
			continue
		}
		if k := r.slots[i].kind; !clipped && k >= dockSlotFlyout && k <= dockSlotFlyoutHandle {
			// The panel that slid out stays within the room of the groups,
			// also on its way out.
			clipped = true
			cv.Save()
			cv.ClipRect(r.inner.Translate(off))
		}
		child.Paint(cv, off.Add(child.Offset()))
	}
	if clipped {
		cv.Restore()
	}
	if r.previewShow {
		rr := geom.RRectAll(r.preview.Translate(off), dockPreviewRadius)
		cv.DrawRRect(rr, paint.Paint{Color: r.fill, Stroke: &r.stroke})
	}
}

// dockPainted reports whether the child ro of a layout is drawn: a slot that
// slides away is, though the input passes it by.
func dockPainted(ro core.RenderObject) bool {
	if sl, ok := ro.(*renderDockSlot); ok {
		return !sl.hidden
	}
	h, ok := ro.(core.Hider)
	return !ok || !h.Hidden()
}

// EntryFocusLeft implements overlay.EntryFocusListener.
func (r *renderDock) EntryFocusLeft() {
	if r.onEntryLeft != nil {
		r.onEntryLeft()
	}
}

// nested reports whether the layout lies within another DockLayout.
func (r *renderDock) nested() bool {
	for ro := r.Parent(); ro != nil; ro = ro.Parent() {
		if _, ok := ro.(*renderDock); ok {
			return true
		}
	}
	return false
}

// paintGround draws the ground of the groups below n.
func (r *renderDock) paintGround(cv *paint.Canvas, off geom.Offset, n *dockNode) {
	if !n.isGroup() {
		for _, c := range n.children {
			r.paintGround(cv, off, c)
		}
		return
	}
	cv.DrawRRect(geom.RRect{Rect: n.body().Translate(off)}, paint.Paint{Color: r.ground})
}

// dockSlotView is the widget of renderDockSlot.
type dockSlotView struct {
	hidden bool
	// leaving draws the slot but leaves it out of the pointer, the keyboard
	// and assistive technology, as the panel that slides back in.
	leaving bool
	// keep says the focus here keeps the panel that slid out, catch that the
	// slot takes the presses nothing in it takes, and bleed that it does not
	// clip what it holds, as the shadow of a flyout.
	keep, catch, bleed bool
	onFocus            func(within bool)
	onDouble           func()
	child              core.Widget
}

// Key implements core.Widget.
func (dockSlotView) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (v dockSlotView) CreateRenderObject() core.RenderObject {
	r := &renderDockSlot{}
	v.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (v dockSlotView) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderDockSlot)
	if r.hidden != v.hidden {
		r.hidden = v.hidden
		r.MarkNeedsPaint()
	}
	r.leaving = v.leaving
	r.onFocus, r.onDouble = v.onFocus, v.onDouble
	r.keep, r.catch, r.bleed = v.keep, v.catch, v.bleed
}

// ChildWidget implements core.SingleChildWidget.
func (v dockSlotView) ChildWidget() core.Widget { return v.child }

// renderDockSlot holds a pane or a strip of a DockLayout. It clips what it
// holds, hides it while it is not shown, so that the pointer, Tab and
// assistive technology pass it by but it keeps its state, reports the focus
// coming in, and turns a double click that nothing below takes into onDouble.
type renderDockSlot struct {
	layout.RenderProxy
	hidden, leaving    bool
	keep, catch, bleed bool
	onFocus            func(within bool)
	onDouble           func()
}

// Hidden implements core.Hider.
// A slot that slides away is drawn, but hidden from the input.
func (r *renderDockSlot) Hidden() bool { return r.hidden || r.leaving }

// ChildClip implements core.Clipper.
func (r *renderDockSlot) ChildClip() (geom.Rect, bool) {
	return geom.RectFromSize(r.Size()), !r.bleed
}

// Paint implements core.RenderObject.
func (r *renderDockSlot) Paint(cv *paint.Canvas, off geom.Offset) {
	if r.hidden {
		return
	}
	if r.bleed {
		r.RenderProxy.Paint(cv, off)
		return
	}
	sz := r.Size()
	cv.Save()
	cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H})
	r.RenderProxy.Paint(cv, off)
	cv.Restore()
}

// FocusWithinChanged implements core.FocusWithinListener.
func (r *renderDockSlot) FocusWithinChanged(within bool) {
	if r.onFocus != nil {
		r.onFocus(within)
	}
}

// HandlePointer implements core.PointerTarget.
func (r *renderDockSlot) HandlePointer(e *core.PointerEvent) {
	if e.Kind == core.PointerDown && e.Button == input.ButtonPrimary && e.Clicks == 2 && r.onDouble != nil {
		e.Handled = true
		r.onDouble()
		return
	}
	if e.Kind == core.PointerDown && r.catch {
		// A press on nothing in the flyout must not take the focus out of it.
		e.Handled = true
	}
}
