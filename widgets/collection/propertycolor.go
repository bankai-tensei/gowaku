package collection

import (
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	overlayw "github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// colorPopoverPadding is the room around the picker in its card.
const colorPopoverPadding = theme.Space12

// colorProperty is the swatch of a PropertyColor with OnChange and no
// OnPress: a press opens a ColorPicker below it.
type colorProperty struct {
	core.StatefulBase
	color PropertyColor
}

type colorPropertyState struct {
	core.StateOf[colorProperty]
	entry *overlayw.Entry
	// opened is the color when the picker opened, which Escape goes back to.
	opened geom.Color
	// th and media are those of the swatch, which the popup takes over: it
	// lives in the overlay, outside them.
	th    *theme.Theme
	media basic.MediaQueryData
}

// CreateState implements core.StatefulWidget.
func (colorProperty) CreateState() core.State { return &colorPropertyState{} }

// Dispose implements core.State.
func (s *colorPropertyState) Dispose() { s.close(false) }

// open shows the picker below the swatch.
func (s *colorPropertyState) open() {
	ro := s.Context().RenderObject()
	if s.entry != nil || ro == nil || !s.Mounted() {
		return
	}
	overlay := overlayw.Of(s.Context())
	anchor := kit.OverlayRect(overlay.Context().RenderObject(), kit.GlobalRect(ro))
	s.opened = s.Widget().color.Value
	s.entry = overlay.InsertFrom(s.Context(), func() core.Widget { return s.popover(anchor) })
	kit.Rebuild(&s.StateBase)
}

// close takes the picker away. restore gives the keyboard back to the swatch.
func (s *colorPropertyState) close(restore bool) {
	if s.entry == nil {
		return
	}
	s.entry.Remove()
	s.entry = nil
	if restore {
		if f := kit.FirstFocusable(s.Context().RenderObject()); f != nil {
			core.RestoreFocus(f)
		}
	}
	if s.Mounted() {
		kit.Rebuild(&s.StateBase)
	}
}

// change reports a color of the picker.
func (s *colorPropertyState) change(c geom.Color) {
	if f := s.Widget().color.OnChange; f != nil {
		f(c)
	}
}

// revert goes back to the color the picker opened with, and closes it.
func (s *colorPropertyState) revert() {
	opened := s.opened
	changed := s.Widget().color.Value != opened
	s.close(true)
	if changed {
		s.change(opened)
	}
}

// Build implements core.State.
func (s *colorPropertyState) Build(ctx core.BuildContext) core.Widget {
	s.th, s.media = basic.ThemeOf(ctx), basic.MediaQueryOf(ctx)
	p := s.Widget().color
	return colorSwatch{Value: p.Value, OnPress: func() {
		if s.entry != nil {
			s.close(true)
			return
		}
		s.open()
	}}
}

// popover builds the card of the picker, below anchor, the swatch in the
// coordinates of the overlay.
func (s *colorPropertyState) popover(anchor geom.Rect) core.Widget {
	th := s.th
	l := basic.LookupStrings(s.Context())
	card := basic.Container{
		Color:   th.Color.Raised,
		Radius:  th.Radius.Group,
		Border:  paint.Stroke{Width: 1, Color: th.Color.Hair},
		Shadows: th.Color.Shadows(theme.LiftFloat),
		Padding: geom.All(colorPopoverPadding),
		Child:   field.ColorPicker{Value: s.opened, Autofocus: true, OnChange: s.change},
	}
	// The overlay may sit above the Localizations of the swatch.
	return basic.Theme{Data: th, Child: basic.MediaQuery{Data: s.media, Child: basic.CarryLocalizations(s.Context(), kit.ModalScope{Active: true, Child: interact.Focus{
		OnKey: func(e *core.KeyEvent) bool {
			switch e.Key {
			case input.KeyEscape:
				s.revert()
				return true
			case input.KeyEnter:
				s.close(true)
				return true
			}
			return false
		},
		Child: basic.Stack{Fit: layout.StackExpand, Children: []core.Widget{
			kit.Barrier{OnPress: func() { s.close(true) }},
			kit.Popup{Anchor: anchor, Side: layout.PopupBelow, Child: interact.Semantics{
				Role: semantics.RoleDialog, Label: l.ChooseColor,
				// A press on the card must not reach the barrier.
				Child: interact.GestureDetector{OnTapDown: func() {}, Child: card},
			}},
		}},
	}})}}
}
