package interact

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// PressState is what a Pressable knows about the user at the moment it builds.
// It is plain data for the builder, not a core.State. The zero value is a
// disabled area that nothing touches.
type PressState struct {
	// Hovered means the pointer is over the area.
	Hovered bool
	// Pressed means the primary button went down on the area and has not come
	// up yet. It stays true while the pointer is dragged outside.
	Pressed bool
	// Focused means the area has the keyboard, which Tab or a click gives it.
	Focused bool
	// FocusVisible means the area has the keyboard and the focus should show:
	// the keyboard gave it, not the pointer. Draw the focus ring with this one.
	FocusVisible bool
	// Enabled means the area reacts: OnPress is set. A disabled area is never
	// hovered, pressed or focused.
	Enabled bool
}

// Pressable is an area that runs OnPress when clicked, or when Space or Enter is
// pressed while it has the focus, and that rebuilds its content as the pointer
// and the keyboard come and go. It draws nothing of its own: Build returns the
// look for each PressState, so a custom button, a tile or a row of a list gets
// hover and press feedback without a State of its own.
//
//	Pressable{Label: "Play", OnPress: play, Build: func(s PressState) core.Widget {
//		var ground geom.Color
//		if s.Hovered || s.FocusVisible {
//			ground = th.Color.Fill
//		}
//		return Container{Color: ground, Radius: th.Radius.Control, Child: Icon{Path: playIcon}}
//	}}
//
// The flags change at once; for a fade, hand them to an AnimatedContainer.
//
// While enabled it shows the hand cursor, takes part in Tab focus and takes the
// focus when pressed. Without OnPress it is disabled: the arrow cursor, no focus,
// and every flag of the PressState false. Assistive technology sees Role, which
// defaults to semantics.RoleButton, with Label, Value and State, and can press it.
//
// Build wins over Child; with neither, the area is empty. Show the focus in
// Build when the content is not a Button already, or keyboard users cannot see
// where they are: FocusRing draws the ring that the controls use.
//
// A Pressable inside another, such as a play button on a tile, is a separate
// stop for Tab. When the outer one shows the inner one only on hover or focus,
// keep it shown while the inner one has the focus with OnFocusChange, or leave
// the inner one out of Tab with SkipTab when the outer one already does the
// same thing.
type Pressable struct {
	core.StatefulBase
	// Label names the area to assistive technology and to gowakutest.Find.
	Label string
	// Role is what the area is. Zero selects semantics.RoleButton.
	Role semantics.Role
	// Value is what assistive technology reads after Label.
	Value string
	// State is what assistive technology knows of the area, such as
	// semantics.Selected for the chosen one of a group. Disabled is added
	// without OnPress.
	State semantics.State
	// OnPress runs on a click or a key press. Nil disables the area.
	OnPress func()
	// Build returns the content for the current PressState.
	Build func(s PressState) core.Widget
	// Child is the content when Build is nil. It does not change with the
	// PressState.
	Child core.Widget
	// OnFocusChange reports gaining and losing the keyboard, for a parent that
	// changes with it.
	OnFocusChange func(focused bool)
	// SkipTab leaves the area out of Tab, so only the pointer reaches it. It
	// never takes the focus then.
	SkipTab bool
}

type pressableState struct {
	core.StateBase
	hovered, pressed, focused, shown bool

	// The callbacks are made once, so that Build makes no closures.
	enter, exit, down, cancel, tap, press func()
	key                                   func(e *core.KeyEvent) bool
	focus, show                           func(focused bool)
}

// CreateState implements core.StatefulWidget.
func (Pressable) CreateState() core.State { return &pressableState{} }

// InitState implements core.State.
func (s *pressableState) InitState() {
	s.enter = func() { s.set(&s.hovered, true) }
	s.exit = func() { s.set(&s.hovered, false) }
	s.down = func() { s.set(&s.pressed, true) }
	s.cancel = func() { s.set(&s.pressed, false) }
	s.tap = func() {
		s.set(&s.pressed, false)
		s.activate()
	}
	s.key = func(e *core.KeyEvent) bool {
		if !kit.Activates(e) {
			return false
		}
		if !e.Repeat {
			s.activate()
		}
		return true
	}
	s.press = s.activate
	s.show = func(show bool) { s.set(&s.shown, show) }
	s.focus = func(focused bool) {
		s.set(&s.focused, focused)
		if f := s.Widget().(Pressable).OnFocusChange; f != nil {
			f(focused)
		}
	}
}

// DidUpdateWidget implements core.State.
func (s *pressableState) DidUpdateWidget(core.Widget) {
	if s.Widget().(Pressable).OnPress == nil {
		// A disabled RenderGesture ignores the release, so a press that was
		// under way when OnPress went away would never end.
		s.pressed = false
	}
}

// Build implements core.State.
func (s *pressableState) Build(core.BuildContext) core.Widget {
	p := s.Widget().(Pressable)
	enabled := p.OnPress != nil
	state := PressState{Enabled: enabled}
	var cursor input.Cursor
	var down, cancel, tap, press func()
	var key func(e *core.KeyEvent) bool
	semState := p.State
	if enabled {
		state.Hovered, state.Pressed, state.Focused = s.hovered, s.pressed, s.focused
		state.FocusVisible = s.focused && s.shown
		cursor = input.CursorHand
		down, cancel, tap, key, press = s.down, s.cancel, s.tap, s.key, s.press
	} else {
		semState |= semantics.Disabled
	}
	child := p.Child
	if p.Build != nil {
		child = p.Build(state)
	}
	role := p.Role
	if role == semantics.RoleNone {
		role = semantics.RoleButton
	}
	// The wrappers stay in place while disabled, with nothing to do, so that
	// enabling and disabling keeps the State of the content. The region keeps
	// tracking the pointer, so that an area enabled under the pointer is
	// hovered at once.
	focusable := enabled && !p.SkipTab
	if !focusable {
		key = nil
	}
	return Semantics{Role: role, Label: p.Label, Value: p.Value, State: semState, OnTap: press, Child: Focus{
		Enabled:       focusable,
		OnKey:         key,
		OnFocusChange: s.focus,
		OnShowFocus:   s.show,
		Child: MouseRegion{
			OnEnter: s.enter,
			OnExit:  s.exit,
			Cursor:  cursor,
			Child:   GestureDetector{OnTapDown: down, OnTapCancel: cancel, OnTap: tap, Child: child},
		},
	}}
}

// activate runs the OnPress of the current widget, which may have changed
// since the callbacks were made.
func (s *pressableState) activate() {
	if f := s.Widget().(Pressable).OnPress; f != nil {
		f()
	}
}

// set changes a flag and rebuilds when it really changed.
func (s *pressableState) set(flag *bool, v bool) {
	if *flag != v {
		*flag = v
		kit.Rebuild(&s.StateBase)
	}
}
