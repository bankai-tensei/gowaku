package field

import (
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	controlw "github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/behavior"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	overlayw "github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

// The measures of a date field.
const (
	// datePickerWidth is the width of a date field that its parent leaves
	// free, room for the longest date of either locale and the icon.
	datePickerWidth = 160
	datePickerIcon  = 16
	// datePopoverPadding is the room around the calendar in its card.
	datePopoverPadding = theme.Space12
)

// DatePicker is a field that shows a date and opens a Calendar below it to
// choose another. The field shows Value as the locale writes a date
// (2026/11/09, Nov 9, 2026), or Placeholder while Value is zero.
//
// A click, Enter, Space, Alt+Down or F4 opens the calendar, in a card that
// turns above the field when there is no room below. Choosing a day closes it
// and calls OnChange; Escape or a click beside it closes it without a change,
// and the keyboard goes back to the field. ShowToday and ShowClear add buttons
// under the calendar that choose Today and the zero time.
//
// Today, InitialMonth, Min, Max, Selectable and Marks are those of Calendar:
// the field never reads the clock. The calendar opens on the month of Value,
// else InitialMonth, else Today; for a birth date, give an InitialMonth some
// thirty years back and Max of today. Its title opens the months of the year
// and then the years of a decade, to go far in a few presses. Without
// OnChange the field is disabled.
//
// In a FormItem the field turns red with the error, and the error of Required
// is Strings.RequiredChoice (選択してください, Choose one). The calendar taking
// the keyboard is not a blur of the field; closing it with Escape or a click
// beside it gives the keyboard back to the field.
//
// A screen reader sees a combo box named by Label, whose value is the date and
// which is expanded while the calendar is open. Label, if set, also shows
// above the field, as that of an Input, unless HideLabel is set.
type DatePicker struct {
	core.StatefulBase
	Value time.Time
	Today time.Time
	// InitialMonth is the month the calendar opens on while Value is zero.
	InitialMonth time.Time
	// Min and Max are the first and the last day that can be chosen. Zero
	// leaves that end open.
	Min, Max   time.Time
	Selectable func(day time.Time) bool
	// Marks puts dots under days of the calendar, as Calendar.Marks does.
	Marks func(day time.Time) []geom.Color
	Label string
	// HideLabel draws no caption for Label, which still names the field to a
	// screen reader.
	HideLabel bool
	// Placeholder shows while Value is zero.
	Placeholder string
	// ShowToday adds a button that chooses Today, and ShowClear one that
	// chooses the zero time.
	ShowToday, ShowClear bool
	OnChange             func(day time.Time)
}

type datePickerState struct {
	core.StateOf[DatePicker]
	behavior.Control
	field *layout.RenderFocus
	entry *overlayw.Entry
	icon  *paint.Path
	// th and media are those of the field at its last build, which the
	// calendar takes over: it lives in the overlay, outside them.
	th    *theme.Theme
	media basic.MediaQueryData
	// scope is that of the FormItem around, as the last build found it.
	scope fieldScope
}

// CreateState implements core.StatefulWidget.
func (DatePicker) CreateState() core.State { return &datePickerState{} }

// InitState implements core.State.
func (s *datePickerState) InitState() {
	s.Attach(&s.StateBase)
	s.icon = calendarPath()
}

// Dispose implements core.State.
func (s *datePickerState) Dispose() {
	s.close(false)
	s.Control.Dispose()
}

// open shows the calendar under the field.
func (s *datePickerState) open() {
	if s.entry != nil || s.Widget().OnChange == nil || s.field == nil || !s.field.Attached() {
		return
	}
	overlay := overlayw.Of(s.Context())
	anchor := kit.GlobalRect(s.field)
	if ro := overlay.Context().RenderObject(); ro != nil {
		o := core.LocalToGlobal(ro, geom.Offset{})
		anchor.X, anchor.Y = anchor.X-o.X, anchor.Y-o.Y
	}
	s.entry = overlay.InsertFrom(s.Context(), func() core.Widget { return s.popover(anchor) })
	kit.Rebuild(&s.StateBase)
}

// close takes the calendar away. restore gives the keyboard back to the field;
// otherwise the keyboard has left the field for good, which a FormItem around
// hears as a blur.
func (s *datePickerState) close(restore bool) {
	if s.entry == nil {
		return
	}
	s.entry.Remove()
	s.entry = nil
	if restore && s.field != nil && s.field.Attached() {
		core.RestoreFocus(s.field)
	} else {
		s.scope.focusChanged(false)
	}
	if s.Mounted() {
		kit.Rebuild(&s.StateBase)
	}
}

func (s *datePickerState) toggle() {
	if s.entry != nil {
		s.close(true)
	} else {
		s.open()
	}
}

// pick closes the calendar and reports d.
func (s *datePickerState) pick(d time.Time) {
	s.close(true)
	if f := s.Widget().OnChange; f != nil {
		f(d)
	}
}

// fieldFocus follows the keyboard of the field. The calendar taking it is not
// a blur for the FormItem around.
func (s *datePickerState) fieldFocus(focused bool) {
	s.Focus(focused)
	if focused || s.entry == nil {
		s.scope.focusChanged(focused)
	}
}

func (s *datePickerState) key(e *core.KeyEvent) bool {
	switch {
	case e.Key == input.KeyEnter || e.Key == input.KeySpace:
		if !e.Repeat && e.Modifiers == 0 {
			s.open()
		}
		return e.Modifiers == 0
	case e.Key == input.KeyDown && e.Modifiers == input.ModAlt, e.Key == input.KeyF4 && e.Modifiers == 0:
		s.open()
		return true
	}
	return false
}

// Build implements core.State.
func (s *datePickerState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	shaper, ok := core.DependOn[*text.Shaper](ctx)
	if !ok {
		panic("gowaku: DatePicker needs a text.Shaper above it; wrap the root in core.Inherited[*text.Shaper]")
	}
	th := basic.ThemeOf(ctx)
	s.th, s.media = th, basic.MediaQueryOf(ctx)
	s.scope = fieldScopeOf(ctx)
	kit.ReportField(ctx, kit.FieldChoice)
	l := basic.StringsOf(ctx)
	c := &th.Color
	enabled := w.OnChange != nil
	open := s.entry != nil

	shown, ink := w.Placeholder, c.Ink3
	var value string
	if !w.Value.IsZero() {
		value = l.Date(w.Value)
		shown, ink = value, c.Ink
	}
	ground, icon := c.Field, c.Ink2
	border := motion.LerpColor(motion.LerpColor(c.Hair, c.Ink3, s.Hot.Get()), c.AccentInk, s.Lit.Get())
	ring := c.AccentInk
	switch {
	case !enabled:
		ground, ink, icon, border = c.Fill, c.Ink3, c.Ink3, c.Hair
	case s.scope.err != "":
		border, ring = c.Danger, c.Danger
	case open:
		border = c.AccentInk
	}
	box := s.Ring(th.Radius.Control, ring, basic.Container{
		Height:  fieldHeight,
		Color:   ground,
		Radius:  th.Radius.Control,
		Border:  paint.Stroke{Width: 1, Color: border},
		Padding: geom.Insets{Left: fieldPaddingX, Right: fieldPaddingX - theme.Space2},
		Child: basic.Row{Cross: layout.CrossCenter, Spacing: theme.Space8, Children: []core.Widget{
			basic.Expanded{Child: face{shaper: shaper, content: shown, style: kit.Colored(th.Text.Body, ink), start: true}},
			basic.Icon{Path: s.icon, Size: datePickerIcon, Color: icon},
		}},
	})

	label := w.Label
	if label == "" {
		label = l.ChooseDate
	}
	sem := interact.Semantics{Role: semantics.RoleComboBox, Label: label, Value: value, Hint: s.scope.err, State: s.scope.state()}
	var body core.Widget
	if enabled {
		if open {
			sem.State |= semantics.Expanded
			sem.OnCollapse = func() { s.close(true) }
		} else {
			sem.State |= semantics.Collapsed
			sem.OnExpand = s.open
		}
		sem.OnTap = s.toggle
		body = focusHandle{
			ref:           &s.field,
			enabled:       true,
			onKey:         s.key,
			onFocusChange: s.fieldFocus,
			onShowFocus:   s.ShowFocus,
			child: interact.MouseRegion{
				OnEnter: func() { s.Hover(true) },
				OnExit:  func() { s.Hover(false) },
				Cursor:  input.CursorHand,
				Child:   interact.GestureDetector{OnTap: s.toggle, Child: box},
			},
		}
	} else {
		sem.State |= semantics.Disabled | semantics.Collapsed
		body = box
	}
	sem.Child = body
	if w.Label == "" || w.HideLabel {
		return fieldWidth{width: datePickerWidth, child: sem}
	}
	return fieldWidth{width: datePickerWidth, child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: fieldLabelGap, Children: []core.Widget{
		basic.Text{Content: w.Label, Style: kit.Colored(th.Text.Caption, c.Ink2)},
		sem,
	}}}
}

// popover builds the card of the calendar, below anchor, the field in the
// coordinates of the overlay.
func (s *datePickerState) popover(anchor geom.Rect) core.Widget {
	w := s.Widget()
	th := s.th
	l := basic.LookupStrings(s.Context())
	parts := []core.Widget{Calendar{
		Value: w.Value, Today: w.Today, InitialMonth: w.InitialMonth, Min: w.Min, Max: w.Max, Selectable: w.Selectable,
		Marks: w.Marks, OnChange: s.pick, autofocus: true,
	}}
	if w.ShowToday || w.ShowClear {
		var actions []core.Widget
		if w.ShowToday {
			var today func()
			if s.allowed(w.Today) {
				day := in(civil(w.Today), w.Today)
				today = func() { s.pick(day) }
			}
			actions = append(actions, controlw.Button{Label: l.Today, Style: controlw.ButtonQuiet, Size: controlw.ButtonSmall, OnPress: today})
		}
		actions = append(actions, basic.Expanded{Child: basic.SizedBox{}})
		if w.ShowClear {
			var clearIt func()
			if !w.Value.IsZero() {
				clearIt = func() { s.pick(time.Time{}) }
			}
			actions = append(actions, controlw.Button{Label: l.Clear, Style: controlw.ButtonQuiet, Size: controlw.ButtonSmall, OnPress: clearIt})
		}
		parts = append(parts, basic.SizedBox{Width: calendarWidth, Child: basic.Row{Cross: layout.CrossCenter, Children: actions}})
	}
	card := basic.Container{
		Color:   th.Color.Raised,
		Radius:  th.Radius.Group,
		Border:  paint.Stroke{Width: 1, Color: th.Color.Hair},
		Shadows: th.Color.Shadows(theme.LiftFloat),
		Padding: geom.All(datePopoverPadding),
		Child:   basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space8, Children: parts},
	}
	// The overlay may sit above the Localizations of the field.
	return basic.Theme{Data: th, Child: basic.MediaQuery{Data: s.media, Child: basic.CarryLocalizations(s.Context(), kit.ModalScope{Active: true, Child: interact.Focus{
		OnKey: func(e *core.KeyEvent) bool {
			if e.Key != input.KeyEscape {
				return false
			}
			s.close(true)
			return true
		},
		Child: basic.Stack{Fit: layout.StackExpand, Children: []core.Widget{
			kit.Barrier{OnPress: func() { s.close(true) }},
			kit.Popup{Anchor: anchor, Side: layout.PopupBelow, Child: interact.Semantics{
				Role: semantics.RoleDialog, Label: l.ChooseDate,
				// A press on the card must not reach the barrier.
				Child: interact.GestureDetector{OnTapDown: func() {}, Child: card},
			}},
		}},
	}})}}
}

// allowed reports whether the day of t can be chosen.
func (s *datePickerState) allowed(t time.Time) bool {
	w := s.Widget()
	if t.IsZero() {
		return false
	}
	d := civil(t)
	if !w.Min.IsZero() && d.Before(civil(w.Min)) || !w.Max.IsZero() && d.After(civil(w.Max)) {
		return false
	}
	return w.Selectable == nil || w.Selectable(t)
}
