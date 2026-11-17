package field

import (
	"math"
	"strings"
	"time"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

const (
	// numberScrubPixels is how far a drag of the scrub handle goes for one
	// step.
	numberScrubPixels = 4
	// numberBig is how many steps Shift, PageUp and PageDown take at once,
	// and numberSmall the share of a step that Ctrl takes.
	numberBig   = 10
	numberSmall = 0.1
	// numberGripInset is the room after the scrub handle.
	numberGripInset = theme.Space6
)

// NumberField edits a number: typed in, stepped with the keys, the wheel or a
// drag of the handle at its end.
//
// The text follows the locale of the window: its decimal separator, and its
// group separators with Grouping (FormatDecimal). It accepts what people type
// besides, such as full-width digits from a Japanese IME and the separators of
// other locales where they cannot be mistaken. See
// docs/ADR/0093-number-range-token.md for the rules.
// The typed text becomes the value when Enter is pressed or the field is left;
// text that is no number turns the border red while it is typed, and goes back
// to the value then. A value outside Min and Max is brought within them, and
// rounded to Precision.
//
// Enter takes the typed text. Inside a FormItem it then also submits the Form
// around, as Enter in an Input does, once the parent has rebuilt with the new
// value, so that the FormItem checks that value and not the old one. Outside a
// FormItem, Enter only takes the text: it submits nothing. An Enter that an
// IME takes to confirm its conversion does neither.
//
// Up and Down step the value by Step, Shift with them by ten steps and Ctrl by
// a tenth of one; PageUp and PageDown step by ten. The wheel steps it while the
// field has the keyboard, and only then, so that a page scrolls past it. A drag
// of the handle steps it once every few pixels, to the right for up. Escape
// puts back the value over what was typed. Assistive technology sees a spin
// button with the range.
//
// The parent keeps Value; OnChange asks for a new one. Without OnChange the
// field is disabled.
type NumberField struct {
	core.StatefulBase
	Value float64
	// Min and Max limit the value when Max is greater than Min.
	Min, Max float64
	// Step is how far Up and Down move the value. Zero selects 1.
	Step float64
	// Precision is the number of digits after the decimal separator, which
	// the value is rounded to. Zero selects as many as Step has, and a
	// negative one whole numbers.
	Precision int
	// Grouping writes the group separators of the locale, as 1,234.5.
	// Without it the digits run together, as suits years and codes. Typed
	// group separators are accepted either way.
	Grouping bool
	// Label names the field above it and to a screen reader.
	Label string
	// HideLabel draws no caption for Label, which still names the field to a
	// screen reader.
	HideLabel bool
	// Placeholder shows while the field is empty.
	Placeholder string
	// ReadOnly shows the value and keeps the user from changing it, as on
	// Input: typing, the keys and the wheel that step, the handle and the
	// steps of a screen reader do nothing, while the text can be selected and
	// copied. Without OnChange the field is disabled instead.
	ReadOnly bool
	OnChange func(float64)
}

func (w NumberField) step() float64 {
	if w.Step > 0 {
		return w.Step
	}
	return 1
}

func (w NumberField) precision() int {
	switch {
	case w.Precision > 0:
		return w.Precision
	case w.Precision < 0:
		return 0
	}
	return decimalsOf(w.step())
}

// normalize rounds v to the precision and keeps it within the limits.
func (w NumberField) normalize(v float64) float64 {
	v = roundTo(v, w.precision())
	if w.Max > w.Min {
		v = min(max(v, w.Min), w.Max)
	}
	return v
}

type numberFieldState struct {
	core.StateOf[NumberField]
	ctrl InputController
	// sym are the separators of locale.
	sym    numberSymbols
	locale language.Tag
	symOK  bool
	// current is the value the field shows, and shown its text, which the
	// field was last given. draft is the text typed since, dirty says there
	// is one, and invalid that it is no number.
	current float64
	shown   string
	draft   string
	dirty   bool
	invalid bool
	focused bool
	// syncing is set while the field takes the text of the value, whose edit
	// is not typing.
	syncing bool
	// wheel and scrub hold what the wheel and the handle moved short of a
	// step.
	wheel, scrub float32
	// scrubX is where the pointer was at the last event of a drag of the
	// handle.
	scrubX float32
	grip   *paint.Path
	// later submits the Form one frame after an Enter that changed the
	// value, and wait counts its ticks.
	later *core.Ticker
	wait  int
}

// CreateState implements core.StatefulWidget.
func (NumberField) CreateState() core.State { return &numberFieldState{} }

// InitState implements core.State.
func (s *numberFieldState) InitState() {
	s.current = s.Widget().Value
	s.grip = scrubGlyph()
}

// DidUpdateWidget implements core.State. The parent's value wins, so that a
// value it turns down goes back. A new value also replaces the text being
// typed, which was meant for the old one; a build with the same value keeps
// it.
func (s *numberFieldState) DidUpdateWidget(old core.Widget) {
	v := s.Widget().Value
	s.current = v
	if v == old.(NumberField).Value || !s.dirty {
		return
	}
	s.dirty, s.invalid = false, false
	text := s.format(v)
	s.shown = text
	s.setText(text)
}

// format spells v as the field shows it.
func (s *numberFieldState) format(v float64) string {
	w := s.Widget()
	return formatNumber(w.normalize(v), w.precision(), s.sym, w.Grouping)
}

// composing reports whether an IME is converting text in the field.
func (s *numberFieldState) composing() bool {
	r := s.ctrl.field()
	return r != nil && r.Composition != nil
}

// show makes v the value the field shows, its text in place of what was typed.
func (s *numberFieldState) show(v float64) {
	s.current = v
	s.dirty, s.invalid = false, false
	text := s.format(v)
	s.shown = text
	s.setText(text)
	kit.Rebuild(&s.StateBase)
}

// setText puts text in the field in place of what was typed.
func (s *numberFieldState) setText(text string) {
	r := s.ctrl.field()
	if r == nil {
		return
	}
	if r.Content != text {
		s.syncing = true
		s.ctrl.Replace(0, len(r.Content), text)
		s.syncing = false
	}
	// The field holds the text it is about to be given as Value, so that a
	// parent that keeps another value replaces it.
	r.value = text
}

// emit shows v, kept within the limits, and asks the parent for it.
func (s *numberFieldState) emit(v float64) {
	w := s.Widget()
	if w.OnChange == nil || w.ReadOnly {
		return
	}
	v = w.normalize(v)
	changed := v != s.current
	s.show(v)
	if changed {
		w.OnChange(v)
	}
}

// base is the value a step starts from: the typed one if it is a number.
func (s *numberFieldState) base() float64 {
	if s.dirty {
		if v, ok := parseNumber(s.draft, s.sym); ok {
			return v
		}
	}
	return s.current
}

// stepBy moves the value by n steps.
func (s *numberFieldState) stepBy(n float64) {
	if n != 0 {
		s.emit(s.base() + n*s.Widget().step())
	}
}

// multiplier returns how many steps one press takes with mods.
func (s *numberFieldState) multiplier(mods input.Modifiers) float64 {
	switch {
	case mods&input.ModShift != 0:
		return numberBig
	case mods&input.ModControl != 0:
		// No finer than the precision shows.
		w := s.Widget()
		return math.Max(numberSmall, math.Pow10(-w.precision())/w.step())
	}
	return 1
}

// commit turns the typed text into the value, or puts the value back over
// text that is no number.
func (s *numberFieldState) commit() {
	if !s.dirty {
		return
	}
	v, ok := parseNumber(s.draft, s.sym)
	if !ok || s.Widget().OnChange == nil {
		s.show(s.current)
		return
	}
	s.emit(v)
}

// changed follows typing, which an IME's composition never is.
func (s *numberFieldState) changed(text string) {
	if s.syncing {
		return
	}
	s.draft, s.dirty = text, text != s.shown
	invalid := s.dirty && strings.TrimFunc(text, isBlank) != ""
	if invalid {
		_, ok := parseNumber(text, s.sym)
		invalid = !ok
	}
	if invalid != s.invalid {
		s.invalid = invalid
		kit.Rebuild(&s.StateBase)
	}
}

func isBlank(r rune) bool { return r == ' ' || r == '　' || r == '\t' }

// submit commits on Enter, unless an IME is converting: that Enter is the
// IME's. In a FormItem it then submits the Form around the field.
func (s *numberFieldState) submit(string) {
	if s.composing() {
		return
	}
	before := s.current
	s.commit()
	scope := fieldScopeOf(s.Context())
	if scope.form == nil {
		return
	}
	if s.current == before {
		scope.submit()
		return
	}
	// The FormItem holds the old value until the parent rebuilds with the
	// new one: the first tick comes before that build, the second after it.
	if s.later == nil {
		s.later = s.NewTicker(func(time.Duration) {
			s.wait++
			if s.wait < 2 {
				return
			}
			s.later.Stop()
			fieldScopeOf(s.Context()).submit()
		})
	}
	s.wait = 0
	s.later.Start()
}

func (s *numberFieldState) fieldFocus(focused bool) {
	s.focused = focused
	s.wheel = 0
	if !focused {
		s.commit()
	}
}

// key steps the value with the keys the field leaves.
func (s *numberFieldState) key(e *core.KeyEvent) bool {
	if s.Widget().ReadOnly || s.composing() || e.Modifiers&^(input.ModShift|input.ModControl) != 0 {
		return false
	}
	switch e.Key {
	case input.KeyUp:
		s.stepBy(s.multiplier(e.Modifiers))
	case input.KeyDown:
		s.stepBy(-s.multiplier(e.Modifiers))
	case input.KeyPageUp:
		s.stepBy(numberBig)
	case input.KeyPageDown:
		s.stepBy(-numberBig)
	case input.KeyEscape:
		if !s.dirty {
			return false
		}
		s.show(s.current)
	default:
		return false
	}
	return true
}

// scroll steps the value with the wheel while the field has the keyboard.
func (s *numberFieldState) scroll(e *core.PointerEvent) {
	if !s.focused || e.Scroll.Y == 0 || s.Widget().ReadOnly {
		return
	}
	e.Handled = true
	s.wheel += e.Scroll.Y
	n := float32(int(s.wheel))
	s.wheel -= n
	s.stepBy(float64(n))
}

// scrubStart commits what was typed and gives the field the keyboard, so that
// the keys and the wheel follow the drag.
func (s *numberFieldState) scrubStart() {
	s.scrub = 0
	s.commit()
	s.ctrl.Focus()
}

// scrubbed steps the value for a drag of dx pixels.
func (s *numberFieldState) scrubbed(dx float32, mods input.Modifiers) {
	s.scrub += dx
	n := float32(int(s.scrub / numberScrubPixels))
	s.scrub -= n * numberScrubPixels
	s.stepBy(float64(n) * s.multiplier(mods))
}

// Build implements core.State.
func (s *numberFieldState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	th := basic.ThemeOf(ctx)
	c := &th.Color
	if locale := basic.LocaleOf(ctx); !s.symOK || locale != s.locale {
		s.sym, s.locale, s.symOK = symbolsFrom(basic.FormatDecimal(ctx, numberSample, 1)), locale, true
	}
	if !s.dirty {
		s.shown = s.format(s.current)
	}
	label := w.Label
	if label == "" {
		label = w.Placeholder
	}
	step := w.step()
	r := semantics.Range{Min: w.Min, Max: s.current + step, Value: s.current, Step: step}
	if w.Max > w.Min {
		r.Max = w.Max
	}
	sem := interact.Semantics{Role: semantics.RoleStepper, Label: label, Value: s.shown, Range: r}
	if w.OnChange == nil {
		sem.State = semantics.Disabled
		d := numberDisabled{label: w.Label, text: s.shown}
		if w.HideLabel {
			d.label = ""
		}
		sem.Child = d
		return sem
	}
	if w.ReadOnly {
		sem.State |= semantics.ReadOnly
	} else {
		sem.OnIncrease = func() { s.stepBy(1) }
		sem.OnDecrease = func() { s.stepBy(-1) }
		sem.OnSetValue = func(_ string, v float64) { s.emit(v) }
	}

	scope := fieldScopeOf(ctx)
	if s.invalid && scope.err == "" {
		scope.err = basic.StringsOf(ctx).InvalidNumber
	}
	if s.invalid {
		sem.State |= semantics.Invalid
	}
	grip := c.Ink3
	field := Input{
		Value:       s.shown,
		Label:       w.Label,
		HideLabel:   w.HideLabel,
		Placeholder: w.Placeholder,
		ReadOnly:    w.ReadOnly,
		Controller:  &s.ctrl,
		OnChange:    s.changed,
		OnSubmit:    s.submit,
		Extras: Extras{OnFocus: s.fieldFocus, trailing: numberPointer{
			cursor: input.CursorColResize,
			onPointer: func(e *core.PointerEvent) {
				s.scrubPointer(e)
			},
			child: basic.Padding{Insets: geom.Insets{Right: numberGripInset}, Child: basic.Icon{
				Path: s.grip, Size: kit.GlyphSize, Color: grip,
			}},
		}},
	}
	if w.ReadOnly {
		// No handle: it would promise a drag that does nothing.
		field.Extras.trailing = nil
	}
	sem.Child = numberPointer{onPointer: s.scroll, child: interact.Focus{
		OnKey: s.key,
		Child: core.Inherited[fieldScope]{Value: scope, Child: field},
	}}
	return sem
}

// scrubPointer turns a drag of the handle into steps.
func (s *numberFieldState) scrubPointer(e *core.PointerEvent) {
	switch e.Kind {
	case core.PointerDown:
		if e.Button != input.ButtonPrimary {
			return
		}
		e.Handled = true
		s.scrubX = e.Pos.X
		s.scrubStart()
	case core.PointerMove:
		dx := e.Pos.X - s.scrubX
		s.scrubX = e.Pos.X
		s.scrubbed(dx, e.Modifiers)
	}
}

// scrubGlyph returns the two arrows of the scrub handle, on the 24 unit grid
// of Icon.
func scrubGlyph() *paint.Path {
	p := &paint.Path{}
	p.MoveTo(3, 12)
	p.LineTo(9.5, 6.5)
	p.LineTo(9.5, 17.5)
	p.Close()
	p.MoveTo(21, 12)
	p.LineTo(14.5, 6.5)
	p.LineTo(14.5, 17.5)
	p.Close()
	return p
}

// numberDisabled is a NumberField without OnChange: its text, faint, in the
// box of a field.
type numberDisabled struct {
	core.StatelessBase
	label, text string
}

// Build implements core.StatelessWidget.
func (d numberDisabled) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	c := &th.Color
	style := fieldStyle(th, kit.Colored(th.Text.Control, c.Ink3))
	box := basic.Container{
		Height: fieldHeight, Radius: th.Radius.Control, Color: c.Fill,
		Padding: geom.Symmetric(fieldPaddingX, 0),
		Child:   basic.Align{Alignment: layout.AlignCenterLeft, Child: basic.Text{Content: d.text, Style: style}},
	}
	if d.label == "" {
		return box
	}
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: fieldLabelGap, Children: []core.Widget{
		basic.Text{Content: d.label, Style: kit.Colored(th.Text.Caption, c.Ink3)},
		box,
	}}
}

// numberPointer hands the pointer events over its child to onPointer, which
// marks those it takes as handled, and sets the mouse cursor over it.
type numberPointer struct {
	onPointer func(e *core.PointerEvent)
	cursor    input.Cursor
	child     core.Widget
}

// Key implements core.Widget.
func (numberPointer) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (p numberPointer) CreateRenderObject() core.RenderObject {
	return &renderNumberPointer{onPointer: p.onPointer, cursor: p.cursor}
}

// UpdateRenderObject implements core.RenderWidget.
func (p numberPointer) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderNumberPointer)
	r.onPointer, r.cursor = p.onPointer, p.cursor
}

// ChildWidget implements core.SingleChildWidget.
func (p numberPointer) ChildWidget() core.Widget { return p.child }

type renderNumberPointer struct {
	layout.RenderProxy
	onPointer func(e *core.PointerEvent)
	cursor    input.Cursor
}

// HandlePointer implements core.PointerTarget.
func (r *renderNumberPointer) HandlePointer(e *core.PointerEvent) {
	if r.onPointer != nil {
		r.onPointer(e)
	}
}

// MouseCursor implements core.CursorSource.
func (r *renderNumberPointer) MouseCursor() input.Cursor { return r.cursor }
