package control

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/behavior"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

const (
	// radioCircle is the size of the circle, that of a checkbox (DESIGN §5).
	radioCircle = 16
	// radioDot is the white dot inside a chosen circle.
	radioDot = 6
	// radioGap is the room between the circle and the label.
	radioGap = theme.Space8
	// radioRowGap and radioColumnGap part the choices of a group stacked
	// down and set side by side.
	radioRowGap    = theme.Space8
	radioColumnGap = theme.Space16
)

// Radio is one choice of several, shown as a circle with its Label to the right.
// A click anywhere on the row, or Space while it has the focus, runs OnSelect.
// Without OnSelect it is disabled.
//
// Most choices belong in a RadioGroup, which makes the choices one stop for Tab
// and moves between them with the arrow keys. Radio is for a layout that a
// group cannot give; each one is then its own stop for Tab, so put them under
// a Semantics with semantics.RoleRadioGroup that names the choice.
type Radio struct {
	core.StatefulBase
	// Selected fills the circle: this is the current choice.
	Selected bool
	Label    string
	// OnSelect asks for this choice. It is not called for the choice that is
	// already selected.
	OnSelect func()

	// grouped means a RadioGroup owns the focus and the keys, and cursor that
	// it shows its focus on this choice.
	grouped, cursor bool
}

type radioState struct {
	core.StateOf[Radio]
	behavior.Control
	choose motion.Implicit
	chosen *motion.Value[float32]
	// pick runs OnSelect of the current widget; made once, as the widget
	// changes under it.
	pick func()
}

// CreateState implements core.StatefulWidget.
func (Radio) CreateState() core.State { return &radioState{} }

// InitState implements core.State.
func (s *radioState) InitState() {
	s.Attach(&s.StateBase)
	s.choose.Attach(&s.StateBase, func() { kit.Rebuild(&s.StateBase) })
	s.chosen = motion.Track(&s.choose, kit.OnOff(s.Widget().Selected), motion.LerpFloat)
	s.pick = func() {
		if w := s.Widget(); w.OnSelect != nil && !w.Selected {
			w.OnSelect()
		}
	}
	s.follow()
}

// DidUpdateWidget implements core.State.
func (s *radioState) DidUpdateWidget(core.Widget) {
	s.chosen.Set(kit.OnOff(s.Widget().Selected))
	s.choose.Commit(kit.ToggleDuration, motion.Standard)
	s.follow()
}

// follow shows the focus of the group on this choice, which has no Focus of
// its own inside a group.
func (s *radioState) follow() {
	w := s.Widget()
	if !w.grouped || s.Focused == w.cursor {
		return
	}
	// Build comes next, so there is nothing to rebuild.
	s.Focused, s.Shown = w.cursor, w.cursor
	s.Lit.Set(kit.OnOff(w.cursor))
	s.Tint.Commit(kit.HoverDuration, motion.Linear)
}

// Dispose implements core.State.
func (s *radioState) Dispose() {
	s.Control.Dispose()
	s.choose.Dispose()
}

// Build implements core.State.
func (s *radioState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	th := basic.ThemeOf(ctx)
	c := &th.Color
	enabled := w.OnSelect != nil
	on := s.chosen.Get()

	fill, line, ink := c.Accent, c.Ink3, c.Ink
	if !enabled {
		fill, line, ink = c.Ink3, c.Hair, c.Ink3
	}
	sink := geom.Clamp32(s.Sink.Value(), 0, 1)
	line = motion.LerpColor(line, c.Ink2, s.Hot.Get())
	fill = theme.Over(fill, c.OnAccent, kit.HoverStrength*s.Hot.Get()+(kit.PressStrength-kit.HoverStrength)*sink)
	dot := radioDot * on
	circle := basic.Container{
		Width: radioCircle, Height: radioCircle, Radius: radioCircle / 2,
		Color: fill.WithAlpha(fill.A * on),
		Sink:  kit.ButtonSink * 2 * sink,
		Child: basic.Center{Child: basic.Container{Width: dot, Height: dot, Radius: dot / 2, Color: c.OnAccent}},
	}
	if on < 1 {
		circle.Border = paint.Stroke{Width: 1, Color: motion.LerpColor(line, fill, on)}
	}
	row := s.Ring(radioCircle/2, c.AccentInk, circle)
	if w.Label != "" {
		row = basic.Row{Size: layout.MainMin, Spacing: radioGap, Cross: layout.CrossCenter, Children: []core.Widget{
			row, basic.Text{Content: w.Label, Style: kit.Colored(th.Text.Body, ink)},
		}}
	}
	sem := interact.Semantics{Role: semantics.RoleRadio, Label: w.Label}
	if w.Selected {
		// The bridges read a chosen radio button from Selected: UIA as
		// SelectionItem, AT-SPI as checked.
		sem.State = semantics.Selected
	}
	if !w.grouped {
		return s.Wrap(sem, enabled, s.pick, row)
	}
	if !enabled {
		sem.State |= semantics.Disabled
		sem.Child = row
		return sem
	}
	// The group has the focus and the keys; the row takes the pointer.
	sem.OnTap = s.pick
	sem.Child = interact.MouseRegion{
		OnEnter: func() { s.Hover(true) },
		OnExit:  func() { s.Hover(false) },
		Child: interact.GestureDetector{
			OnTapDown:   s.Press,
			OnTapCancel: s.Release,
			OnTap: func() {
				s.Release()
				s.pick()
			},
			Child: row,
		},
	}
	return sem
}

// RadioOption is one choice of a RadioGroup.
type RadioOption[T comparable] struct {
	Value T
	Label string
	// Disabled grays the choice out and skips it with the arrow keys. It breaks
	// the rule of positive names (CODING_STYLE §4.1) so that the zero value,
	// which nearly every choice wants, is an enabled one.
	Disabled bool
}

// RadioGroup picks one of Options: the one whose Value equals Value is
// selected, and OnChange asks for another. Without OnChange the whole group is
// disabled. A Value that no option has selects none.
//
// The choices are one stop for Tab, which enters at the selected one. The arrow
// keys move to the previous or next enabled choice and select it, wrapping at
// the ends, and Space selects the choice with the focus when none is selected
// yet, as Windows and WAI-ARIA do.
type RadioGroup[T comparable] struct {
	core.StatefulBase
	Value   T
	Options []RadioOption[T]
	// Label names the choice to assistive technology.
	Label string
	// Horizontal sets the choices side by side instead of stacking them down.
	Horizontal bool
	OnChange   func(T)
}

type radioGroupState[T comparable] struct {
	core.StateOf[RadioGroup[T]]
	focused, shown bool
	// cursor is the choice with the focus, while the group has it.
	cursor int
}

// CreateState implements core.StatefulWidget.
func (RadioGroup[T]) CreateState() core.State { return &radioGroupState[T]{} }

// selected returns the index of the selected option, or -1.
func (w *RadioGroup[T]) selected() int {
	for i := range w.Options {
		if w.Options[i].Value == w.Value {
			return i
		}
	}
	return -1
}

func (w *RadioGroup[T]) enabled(i int) bool {
	return w.OnChange != nil && i >= 0 && i < len(w.Options) && !w.Options[i].Disabled
}

// entry returns the choice that Tab lands on: the selected one if it can be
// used, or else the first that can, or -1.
func (w *RadioGroup[T]) entry() int {
	if i := w.selected(); w.enabled(i) {
		return i
	}
	return w.step(-1, 1)
}

// step returns the next enabled choice from i in the direction d, wrapping, or
// -1 if there is none. From -1 forward it starts at the first.
func (w *RadioGroup[T]) step(i, d int) int {
	n := len(w.Options)
	if i < 0 && d < 0 {
		i = 0
	}
	for range n {
		i = ((i+d)%n + n) % n
		if w.enabled(i) {
			return i
		}
	}
	return -1
}

func (s *radioGroupState[T]) pick(i int) {
	w := s.Widget()
	if !w.enabled(i) {
		return
	}
	s.cursor = i
	kit.Rebuild(&s.StateBase)
	if v := w.Options[i].Value; v != w.Value {
		w.OnChange(v)
	}
}

func (s *radioGroupState[T]) key(e *core.KeyEvent) bool {
	if e.Modifiers&(input.ModControl|input.ModAlt) != 0 {
		return false
	}
	w := s.Widget()
	switch e.Key {
	case input.KeyUp, input.KeyLeft:
		s.pick(w.step(s.cursor, -1))
	case input.KeyDown, input.KeyRight:
		s.pick(w.step(s.cursor, 1))
	case input.KeySpace:
		if !e.Repeat {
			s.pick(s.cursor)
		}
	default:
		return false
	}
	return true
}

func (s *radioGroupState[T]) focus(focused bool) {
	if focused && !s.focused {
		w := s.Widget()
		s.cursor = w.entry()
	}
	s.focused = focused
	kit.Rebuild(&s.StateBase)
}

func (s *radioGroupState[T]) showFocus(show bool) {
	s.shown = show
	kit.Rebuild(&s.StateBase)
}

// Build implements core.State.
func (s *radioGroupState[T]) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	kit.ReportField(ctx, kit.FieldChoice)
	sel := w.selected()
	if !s.focused {
		s.cursor = sel
	}
	rows := make([]core.Widget, len(w.Options))
	usable := false
	for i := range w.Options {
		o := &w.Options[i]
		var pick func()
		if w.enabled(i) {
			usable = true
			pick = func() { s.pick(i) }
		}
		rows[i] = Radio{
			Selected: i == sel, Label: o.Label, OnSelect: pick,
			grouped: true, cursor: s.focused && s.shown && i == s.cursor,
		}
	}
	var body core.Widget = basic.Column{Size: layout.MainMin, Spacing: radioRowGap, Children: rows}
	if w.Horizontal {
		body = basic.Row{Size: layout.MainMin, Spacing: radioColumnGap, Cross: layout.CrossCenter, Children: rows}
	}
	sem := interact.Semantics{Role: semantics.RoleRadioGroup, Label: w.Label}
	if sel >= 0 {
		sem.Value = w.Options[sel].Label
	}
	if !usable {
		sem.State = semantics.Disabled
	}
	sem.Child = interact.Focus{
		Enabled:       usable,
		OnKey:         s.key,
		OnFocusChange: s.focus,
		OnShowFocus:   s.showFocus,
		Child:         body,
	}
	return sem
}
