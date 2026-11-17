package interact

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
)

// Semantics describes its child to assistive technology, and does what a
// screen reader asks with the callbacks it is given. The standard controls
// describe themselves; use it for what you build from smaller parts:
//
//	Semantics{Role: semantics.RoleHeading, Level: 1, Label: "Inbox", Child: title}
//
// The Focusable and Focused states, and the Focus action, are set by core for
// whatever takes the focus directly inside.
type Semantics struct {
	Role  semantics.Role
	Label string
	Value string
	// Hint says what using the node does, when the label does not.
	Hint  string
	State semantics.State
	// Live announces changes of the label or value. See semantics.Live.
	Live semantics.Live
	// Range is the number of a slider, stepper or progress indicator.
	Range semantics.Range
	// Level is the level of a heading, from 1.
	Level int
	// PosInSet and SetSize place an item among its siblings, both from 1:
	// the second of three steps is 2 of 3. Zero leaves them to the platform.
	PosInSet, SetSize int
	// Shortcut is the key that does the same from anywhere in the window, as
	// a menu shows it: "Ctrl+S".
	Shortcut string

	// OnTap and the others do the actions of a screen reader. A nil one leaves
	// its action out. They do nothing while State has Disabled.
	OnTap      func()
	OnIncrease func()
	OnDecrease func()
	// OnSetValue replaces the value: text for a field, number for a range.
	OnSetValue func(text string, number float64)
	OnExpand   func()
	OnCollapse func()

	Child core.Widget
}

// Key implements core.Widget.
func (Semantics) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (s Semantics) CreateRenderObject() core.RenderObject {
	r := &layout.RenderSemantics{}
	s.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget. Semantics draw nothing, so
// no layout or paint follows.
func (s Semantics) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*layout.RenderSemantics)
	r.Node = semantics.Node{
		Role: s.Role, Label: s.Label, Value: s.Value, Hint: s.Hint,
		State: s.State, Live: s.Live, Range: s.Range, Level: s.Level,
		Shortcut: s.Shortcut, PosInSet: s.PosInSet, SetSize: s.SetSize,
	}
	r.OnTap, r.OnIncrease, r.OnDecrease = s.OnTap, s.OnIncrease, s.OnDecrease
	r.OnSetValue, r.OnExpand, r.OnCollapse = s.OnSetValue, s.OnExpand, s.OnCollapse
}

// ChildWidget implements core.SingleChildWidget.
func (s Semantics) ChildWidget() core.Widget { return s.Child }

// ExcludeSemantics hides its child from assistive technology, as aria-hidden
// does on the web, while it stays on screen and under the pointer. Use it for
// what only decorates and says nothing a screen reader should read: the
// initial in an avatar next to the name, a glyph drawn as text, an ornament.
//
//	ExcludeSemantics{Child: basic.Text{Content: "S", Style: th.Text.Headline}}
//
// Do not put controls inside: they still take the focus with Tab and the
// pointer, but a screen reader cannot see them. basic.Icon and basic.Image
// without a Label are left out on their own.
type ExcludeSemantics struct {
	Child core.Widget
}

// Key implements core.Widget.
func (ExcludeSemantics) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (ExcludeSemantics) CreateRenderObject() core.RenderObject {
	return &layout.RenderExcludeSemantics{}
}

// UpdateRenderObject implements core.RenderWidget.
func (ExcludeSemantics) UpdateRenderObject(core.RenderObject) {}

// ChildWidget implements core.SingleChildWidget.
func (e ExcludeSemantics) ChildWidget() core.Widget { return e.Child }

// FocusTraversalOrder sets where the controls below it come in the order of
// Tab, as tabindex does in HTML: those with an Order above zero come first,
// lowest first, and the rest follow in the order they are painted. The nearest
// one above a control wins.
//
//	FocusTraversalOrder{Order: 1, Child: searchField}
type FocusTraversalOrder struct {
	Order int
	Child core.Widget
}

// Key implements core.Widget.
func (FocusTraversalOrder) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (f FocusTraversalOrder) CreateRenderObject() core.RenderObject {
	return &layout.RenderFocusOrder{Order: f.Order}
}

// UpdateRenderObject implements core.RenderWidget.
func (f FocusTraversalOrder) UpdateRenderObject(ro core.RenderObject) {
	ro.(*layout.RenderFocusOrder).Order = f.Order
}

// ChildWidget implements core.SingleChildWidget.
func (f FocusTraversalOrder) ChildWidget() core.Widget { return f.Child }
