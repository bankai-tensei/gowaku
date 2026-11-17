package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
)

// Opacity paints its child faded as one picture, from 0 to 1, as opacity does in
// CSS: where shapes of the child overlap, the lower ones do not show through.
// Between 0 and 1 the child draws into a picture of its own on the GPU each frame
// it is drawn (paint.Canvas.PushGroup), unless it draws a single shape, which
// fades alone.
//
// At 0 the child is still laid out but counts as absent: it is not hit by the
// pointer, which reaches what is below, it is not in the semantics tree, and
// Tab skips it. A hover overlay therefore keeps its MouseRegion outside the
// Opacity, where the pointer still finds it:
//
//	MouseRegion{OnEnter: show, OnExit: hide, Child: Stack{Children: []core.Widget{
//		cover,
//		AnimatedOpacity{Opacity: hint, Child: playButton},
//	}}}
type Opacity struct {
	Opacity float32
	Child   core.Widget
}

// Key implements core.Widget.
func (Opacity) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (o Opacity) CreateRenderObject() core.RenderObject {
	return &layout.RenderOpacity{Opacity: o.Opacity}
}

// UpdateRenderObject implements core.RenderWidget.
func (o Opacity) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderOpacity)
	if r.Opacity != o.Opacity {
		r.Opacity = o.Opacity
		r.MarkNeedsPaint()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (o Opacity) ChildWidget() core.Widget { return o.Child }

// Scale paints its child enlarged around Origin, for transitions. Layout and the
// pointer still see the unscaled child.
type Scale struct {
	Scale float32
	// Origin is the point that stays in place, 0..1 on each axis (not -1..1 as
	// in Flutter). The zero value is the top left; layout.AlignCenter scales
	// around the middle.
	Origin layout.Alignment
	Child  core.Widget
}

// Key implements core.Widget.
func (Scale) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (s Scale) CreateRenderObject() core.RenderObject {
	return &layout.RenderScale{Scale: s.Scale, Origin: s.Origin}
}

// UpdateRenderObject implements core.RenderWidget.
func (s Scale) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderScale)
	if r.Scale != s.Scale || r.Origin != s.Origin {
		r.Scale, r.Origin = s.Scale, s.Origin
		r.MarkNeedsPaint()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (s Scale) ChildWidget() core.Widget { return s.Child }

// IgnorePointer lets the pointer pass through its child to what is below. In a
// Stack it goes inside the Positioned or PositionedFill, not around it: the
// Stack places only its direct children by their positions.
type IgnorePointer struct {
	Child core.Widget
}

// Key implements core.Widget.
func (IgnorePointer) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (IgnorePointer) CreateRenderObject() core.RenderObject { return &layout.RenderIgnorePointer{} }

// UpdateRenderObject implements core.RenderWidget.
func (IgnorePointer) UpdateRenderObject(core.RenderObject) {}

// ChildWidget implements core.SingleChildWidget.
func (i IgnorePointer) ChildWidget() core.Widget { return i.Child }
