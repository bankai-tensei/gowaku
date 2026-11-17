package basic

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
)

// Stack paints its children on top of each other, the last one on top.
//
// A press goes to the last child hit at its place. The layout widgets (Align,
// Center, Padding, Row, Column, Stack, SizedBox, Positioned, PositionedFill and
// the like) are hit only where one of their children is, so a layer such as
// PositionedFill{Child: Align{Alignment: layout.AlignBottomCenter, Child:
// button}} takes the pointer over the button only, and the controls under the
// rest of it still get their presses. A Container with a Color, Gradient,
// Border or Shadows, text, images and the widgets that take the pointer
// (interact.GestureDetector, interact.Pressable, interact.Listener) are hit
// over their whole box. IgnorePointer lets the pointer through its child; put
// it inside a Positioned or PositionedFill, which must be the direct children
// of the Stack.
type Stack struct {
	// Alignment places the children that are not Positioned. It runs 0..1, not
	// -1..1 as in Flutter; the zero value is the top left.
	Alignment layout.Alignment
	Fit       layout.StackFit
	Children  []core.Widget
}

// Key implements core.Widget.
func (Stack) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (s Stack) CreateRenderObject() core.RenderObject {
	return &layout.RenderStack{Alignment: s.Alignment, Fit: s.Fit}
}

// UpdateRenderObject implements core.RenderWidget.
func (s Stack) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderStack)
	if r.Alignment != s.Alignment || r.Fit != s.Fit {
		r.Alignment, r.Fit = s.Alignment, s.Fit
		r.MarkNeedsLayout()
	}
}

// ChildWidgets implements core.MultiChildWidget.
func (s Stack) ChildWidgets() []core.Widget { return s.Children }

// Positioned places a child of a Stack at a fixed spot. The zero value is the
// top left corner.
//
// Left and Right together do not stretch the child, nor do Top and Bottom:
// Right is read only with AnchorRight, and Bottom only with AnchorBottom. To
// fill the Stack less some insets use PositionedFill; for a fixed size set
// Width and Height; for a size that follows the Stack use a LayoutBuilder.
type Positioned struct {
	Left, Top, Right, Bottom float32
	// AnchorRight and AnchorBottom measure from the far edges, using Right and
	// Bottom instead of Left and Top.
	AnchorRight, AnchorBottom bool
	// Width and Height force the size of the child. Zero leaves it free.
	Width, Height float32
	Child         core.Widget
}

// Key implements core.Widget.
func (Positioned) Key() core.Key { return nil }

func (p Positioned) apply(r *layout.RenderPositioned) {
	r.Left, r.Top, r.Right, r.Bottom = p.Left, p.Top, p.Right, p.Bottom
	r.AnchorRight, r.AnchorBottom = p.AnchorRight, p.AnchorBottom
	r.Width, r.Height = p.Width, p.Height
}

// CreateRenderObject implements core.RenderWidget.
func (p Positioned) CreateRenderObject() core.RenderObject {
	r := &layout.RenderPositioned{}
	p.apply(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (p Positioned) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderPositioned)
	if r.Left != p.Left || r.Top != p.Top || r.Right != p.Right || r.Bottom != p.Bottom ||
		r.AnchorRight != p.AnchorRight || r.AnchorBottom != p.AnchorBottom ||
		r.Width != p.Width || r.Height != p.Height {
		p.apply(r)
		// The offset is decided by the stack, which reads these fields.
		r.Parent().MarkNeedsLayout()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (p Positioned) ChildWidget() core.Widget { return p.Child }

// PositionedFill stretches a child of a Stack over the whole Stack, less
// Insets. The child is made exactly that size and placed at the top left inset.
// The zero value fills the Stack edge to edge. Like Positioned, it does not
// count toward the size of the Stack, which its other children decide (or Fit).
type PositionedFill struct {
	Insets layout.Insets
	Child  core.Widget
}

// Key implements core.Widget.
func (PositionedFill) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (p PositionedFill) CreateRenderObject() core.RenderObject {
	return &layout.RenderPositionedFill{Insets: p.Insets}
}

// UpdateRenderObject implements core.RenderWidget.
func (p PositionedFill) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderPositionedFill)
	if r.Insets != p.Insets {
		r.Insets = p.Insets
		// The size and offset are decided by the stack, which reads Insets.
		r.Parent().MarkNeedsLayout()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (p PositionedFill) ChildWidget() core.Widget { return p.Child }
