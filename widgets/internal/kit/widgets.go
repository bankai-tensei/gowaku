package kit

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
)

// Checkmark is the widget of layout.RenderCheckmark.
type Checkmark struct {
	Color    geom.Color
	Progress float32
}

// Key implements core.Widget.
func (Checkmark) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (c Checkmark) CreateRenderObject() core.RenderObject {
	return &layout.RenderCheckmark{Color: c.Color, Progress: c.Progress}
}

// UpdateRenderObject implements core.RenderWidget.
func (c Checkmark) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderCheckmark)
	if r.Color != c.Color || r.Progress != c.Progress {
		r.Color, r.Progress = c.Color, c.Progress
		r.MarkNeedsPaint()
	}
}

// Ring is the widget of layout.RenderRing: a focus ring around Child.
type Ring struct {
	Radius float32
	Color  geom.Color
	Child  core.Widget
}

// Key implements core.Widget.
func (Ring) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (f Ring) CreateRenderObject() core.RenderObject {
	return &layout.RenderRing{Radius: f.Radius, Color: f.Color}
}

// UpdateRenderObject implements core.RenderWidget.
func (f Ring) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderRing)
	if r.Radius != f.Radius || r.Color != f.Color {
		r.Radius, r.Color = f.Radius, f.Color
		r.MarkNeedsPaint()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (f Ring) ChildWidget() core.Widget { return f.Child }

// ModalScope is the widget of layout.RenderModal.
type ModalScope struct {
	Active bool
	// BlockSemantics hides what is under the scope from assistive technology
	// while it is Active, as for a dialog or a sheet; see
	// layout.RenderModal.BlockSemantics.
	BlockSemantics bool
	Child          core.Widget
}

// Key implements core.Widget.
func (ModalScope) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (m ModalScope) CreateRenderObject() core.RenderObject {
	return &layout.RenderModal{Active: m.Active, BlockSemantics: m.BlockSemantics}
}

// UpdateRenderObject implements core.RenderWidget.
func (m ModalScope) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderModal)
	r.Active, r.BlockSemantics = m.Active, m.BlockSemantics
}

// ChildWidget implements core.SingleChildWidget.
func (m ModalScope) ChildWidget() core.Widget { return m.Child }

// Barrier is the widget of layout.RenderBarrier.
type Barrier struct {
	OnPress func()
}

// Key implements core.Widget.
func (Barrier) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (b Barrier) CreateRenderObject() core.RenderObject {
	return &layout.RenderBarrier{OnPress: b.OnPress}
}

// UpdateRenderObject implements core.RenderWidget.
func (b Barrier) UpdateRenderObject(ro core.RenderObject) {
	ro.(*layout.RenderBarrier).OnPress = b.OnPress
}

// Popup is the widget of layout.RenderPopup, with the gaps of a menu.
type Popup struct {
	Anchor geom.Rect
	Side   layout.PopupSide
	// Shift moves the popup along the anchor, as a submenu lines its first
	// item up with the item that opened it.
	Shift float32
	Child core.Widget
}

// Key implements core.Widget.
func (Popup) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (p Popup) CreateRenderObject() core.RenderObject {
	r := &layout.RenderPopup{}
	p.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (p Popup) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderPopup)
	if r.Anchor != p.Anchor || r.Side != p.Side || r.Shift != p.Shift || r.Gap != MenuGap || r.Inset != MenuEdge {
		r.Anchor, r.Side, r.Shift, r.Gap, r.Inset = p.Anchor, p.Side, p.Shift, MenuGap, MenuEdge
		r.MarkNeedsLayout()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (p Popup) ChildWidget() core.Widget { return p.Child }

// PopupLayer is an overlay entry whose content its owner rebuilds as it
// changes: Build runs on every build of the layer.
type PopupLayer struct {
	core.StatefulBase
	Build func() core.Widget
	// State receives the State of the layer, for rebuilding it.
	State **PopupLayerState
}

// PopupLayerState is the State of a PopupLayer.
type PopupLayerState struct {
	core.StateBase
}

// CreateState implements core.StatefulWidget.
func (PopupLayer) CreateState() core.State { return &PopupLayerState{} }

// InitState implements core.State.
func (s *PopupLayerState) InitState() { *s.Widget().(PopupLayer).State = s }

// Build implements core.State.
func (s *PopupLayerState) Build(core.BuildContext) core.Widget {
	return s.Widget().(PopupLayer).Build()
}

// TabListFocus is Focus that hands its render object to Ref, so that a strip
// of tabs can move the keyboard to itself.
type TabListFocus struct {
	Ref                        **layout.RenderFocus
	Enabled                    bool
	OnKey                      func(e *core.KeyEvent) bool
	OnFocusChange, OnShowFocus func(bool)
	Child                      core.Widget
}

// Key implements core.Widget.
func (TabListFocus) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (f TabListFocus) CreateRenderObject() core.RenderObject {
	r := &layout.RenderFocus{}
	f.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (f TabListFocus) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderFocus)
	r.Enabled, r.OnKey, r.OnFocusChange, r.OnShowFocus = f.Enabled, f.OnKey, f.OnFocusChange, f.OnShowFocus
	*f.Ref = r
}

// ChildWidget implements core.SingleChildWidget.
func (f TabListFocus) ChildWidget() core.Widget { return f.Child }
