package field

import (
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

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

// The measures of a time field.
const (
	// timeSegmentPadding is the room beside the digits of a part, inside its
	// highlight, and timeSegmentHeight the height of the highlight.
	timeSegmentPadding = theme.Space4
	timeSegmentHeight  = 22
	timeFieldPadding   = theme.Space6
	day24              = 24 * time.Hour
)

// HourCycle says how a TimePicker counts the hours.
//
// It is HourCycleLocale (the zero value), HourCycle12 or HourCycle24.
type HourCycle uint8

// The hour cycles.
const (
	// HourCycleLocale follows the locale of the window: 0 to 23 in Japanese,
	// 1 to 12 with AM and PM in English.
	HourCycleLocale HourCycle = iota
	// HourCycle12 counts from 1 to 12, with a part for AM and PM.
	HourCycle12
	// HourCycle24 counts from 0 to 23.
	HourCycle24
)

// String returns the name of the cycle.
func (h HourCycle) String() string {
	switch h {
	case HourCycle12:
		return "12h"
	case HourCycle24:
		return "24h"
	}
	return "locale"
}

// timePart is a part of a time field.
type timePart uint8

const (
	partHour timePart = iota
	partMinute
	partSecond
	partPeriod
	partCount
)

// TimePicker is a field for a time of day, made of parts for the hour, the
// minute, the second if Seconds is set, and AM or PM on a twelve-hour clock,
// edited as in the time fields of Windows and macOS.
//
// Value is the time since midnight, from 0 to 24 hours; others are taken
// modulo a day. Each part takes the keyboard on its own: Up and Down step it,
// wrapping around (the hour carries over AM and PM, the minutes and seconds
// do not carry), Home and End go to its ends, and digits type it, moving to the
// next part once it is complete. Left, Right and Tab move between the parts;
// A and P choose AM and PM. MinuteStep, if above 1, is how far Up and Down move
// the minutes, to the next multiple of it; typed minutes stay as typed.
//
// HourCycle zero follows the locale of the window, which also places AM and PM
// before the hour in Japanese. Without OnChange the field is disabled.
//
// A screen reader sees a group named by Label, with a spin button for each
// part, which it names (時, Hour) and can step. Label, if set, also shows above
// the field, as that of an Input, unless HideLabel is set.
type TimePicker struct {
	core.StatefulBase
	Value     time.Duration
	HourCycle HourCycle
	// Seconds adds a part for the seconds.
	Seconds bool
	// MinuteStep is how far Up and Down move the minutes. Zero selects 1.
	MinuteStep int
	Label      string
	// HideLabel draws no caption for Label, which still names the field to a
	// screen reader.
	HideLabel bool
	OnChange  func(time.Duration)
}

type timePickerState struct {
	core.StateOf[TimePicker]
	tint     motion.Implicit
	lit, hot *motion.Value[float32]
	hovered  bool
	// focused is the part with the keyboard, or partCount for none.
	focused timePart
	refs    [partCount]*layout.RenderFocus
	// typing is the part whose first digit was typed, or partCount, and typed
	// that digit.
	typing timePart
	typed  int
	// order and parts are the parts in the order of the field, and twelve
	// whether the clock has twelve hours, as the last build found them.
	order  [partCount]timePart
	parts  int
	twelve bool
	// scope is that of the FormItem around, as the last build found it.
	scope fieldScope
}

// CreateState implements core.StatefulWidget.
func (TimePicker) CreateState() core.State {
	return &timePickerState{focused: partCount, typing: partCount}
}

// InitState implements core.State.
func (s *timePickerState) InitState() {
	s.tint.Attach(&s.StateBase, func() { kit.Rebuild(&s.StateBase) })
	s.lit = motion.Track(&s.tint, 0, motion.LerpFloat)
	s.hot = motion.Track(&s.tint, 0, motion.LerpFloat)
}

// Dispose implements core.State.
func (s *timePickerState) Dispose() { s.tint.Dispose() }

func (s *timePickerState) sync() {
	s.lit.Set(kit.OnOff(s.focused != partCount))
	s.hot.Set(kit.OnOff(s.hovered))
	s.tint.Commit(kit.HoverDuration, motion.Linear)
	kit.Rebuild(&s.StateBase)
}

func (s *timePickerState) hover(inside bool) {
	if s.hovered != inside {
		s.hovered = inside
		s.sync()
	}
}

// partFocus follows the keyboard of part p. A FormItem around hears every
// move, and takes the blur of one part back when the next one gets it.
func (s *timePickerState) partFocus(p timePart, focused bool) {
	switch {
	case focused:
		s.focused = p
	case s.focused == p:
		s.focused = partCount
	default:
		return
	}
	s.typing = partCount
	s.scope.focusChanged(focused)
	s.sync()
}

// value returns Value within a day.
func (s *timePickerState) value() time.Duration {
	v := s.Widget().Value % day24
	if v < 0 {
		v += day24
	}
	return v
}

func (s *timePickerState) change(v time.Duration) {
	v %= day24
	if v < 0 {
		v += day24
	}
	if f := s.Widget().OnChange; f != nil && v != s.value() {
		f(v)
	}
}

// limits returns the lowest and highest number of part p.
func (s *timePickerState) limits(p timePart) (lo, hi int) {
	switch p {
	case partHour:
		if s.twelve {
			return 1, 12
		}
		return 0, 23
	case partPeriod:
		return 0, 1
	}
	return 0, 59
}

// number returns the number that part p shows.
func (s *timePickerState) number(p timePart) int {
	v := s.value()
	h := int(v / time.Hour)
	switch p {
	case partHour:
		if s.twelve {
			if h%12 == 0 {
				return 12
			}
			return h % 12
		}
		return h
	case partMinute:
		return int(v/time.Minute) % 60
	case partSecond:
		return int(v/time.Second) % 60
	}
	return h / 12
}

// with returns the value with part p set to n, which is within its limits.
func (s *timePickerState) with(p timePart, n int) time.Duration {
	v := s.value()
	h, m, sec := int(v/time.Hour), int(v/time.Minute)%60, int(v/time.Second)%60
	frac := v % time.Second
	switch p {
	case partHour:
		if s.twelve {
			h = n%12 + h/12*12
		} else {
			h = n
		}
	case partMinute:
		m = n
	case partSecond:
		sec = n
	case partPeriod:
		h = h%12 + n*12
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second + frac
}

// step moves part p by dir, up or down.
func (s *timePickerState) step(p timePart, dir int) {
	n := s.number(p)
	switch p {
	case partHour:
		// The hour moves the time, so that it carries over noon and midnight.
		s.change(s.value() + time.Duration(dir)*time.Hour)
	case partMinute:
		st := max(s.Widget().MinuteStep, 1)
		var next int
		switch {
		case dir > 0:
			next = (n/st + 1) * st
		case n%st != 0:
			next = n / st * st
		default:
			next = n - st
		}
		s.change(s.with(p, (next%60+60)%60))
	case partSecond:
		s.change(s.with(p, ((n+dir)%60+60)%60))
	case partPeriod:
		s.change(s.with(p, 1-n))
	}
}

// typeDigit types digit d into part p.
func (s *timePickerState) typeDigit(p timePart, d int) {
	lo, hi := s.limits(p)
	if s.typing == p {
		s.typing = partCount
		if v := s.typed*10 + d; v >= lo && v <= hi {
			s.change(s.with(p, v))
			s.advance(p)
			return
		}
	}
	if d*10 > hi {
		if d >= lo {
			s.change(s.with(p, d))
		}
		s.advance(p)
		return
	}
	s.typing, s.typed = p, d
	if d >= lo {
		s.change(s.with(p, d))
	}
}

// index returns the place of part p in the field, or -1.
func (s *timePickerState) index(p timePart) int {
	for i := range s.parts {
		if s.order[i] == p {
			return i
		}
	}
	return -1
}

// moveTo gives the keyboard to the part at place i, if there is one.
func (s *timePickerState) moveTo(i int) {
	if i < 0 || i >= s.parts {
		return
	}
	if r := s.refs[s.order[i]]; r != nil && r.Attached() {
		r.RequestFocus()
	}
}

// advance moves the keyboard to the part after p, if any.
func (s *timePickerState) advance(p timePart) { s.moveTo(s.index(p) + 1) }

func (s *timePickerState) key(p timePart, e *core.KeyEvent) bool {
	if e.Modifiers&(input.ModControl|input.ModAlt) != 0 {
		return false
	}
	lo, hi := s.limits(p)
	switch k := e.Key; {
	case k == input.KeyUp || k == input.KeyDown:
		s.typing = partCount
		dir := 1
		if k == input.KeyDown {
			dir = -1
		}
		s.step(p, dir)
	case k == input.KeyHome || k == input.KeyEnd:
		s.typing = partCount
		n := lo
		if k == input.KeyEnd {
			n = hi
		}
		s.change(s.with(p, n))
	case k == input.KeyLeft:
		s.moveTo(s.index(p) - 1)
	case k == input.KeyRight:
		s.moveTo(s.index(p) + 1)
	case k == input.KeyBackspace || k == input.KeyDelete:
		s.typing = partCount
	case p == partPeriod && (k == input.KeyA || k == input.KeyP):
		n := 0
		if k == input.KeyP {
			n = 1
		}
		s.change(s.with(p, n))
		s.advance(p)
	case p != partPeriod && k >= input.Key0 && k <= input.Key9 && e.Modifiers == 0:
		if !e.Repeat || s.typing != p {
			s.typeDigit(p, int(k-input.Key0))
		}
	default:
		return false
	}
	return true
}

// spell returns what part p shows.
func (s *timePickerState) spell(l *basic.Strings, p timePart) string {
	n := s.number(p)
	switch {
	case p == partPeriod && n == 0:
		return l.AM
	case p == partPeriod:
		return l.PM
	case p == partHour && s.twelve:
		return kit.Itoa(n)
	}
	return kit.Pad2(n)
}

// Build implements core.State.
func (s *timePickerState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	shaper, ok := core.DependOn[*text.Shaper](ctx)
	if !ok {
		panic("gowaku: TimePicker needs a text.Shaper above it; wrap the root in core.Inherited[*text.Shaper]")
	}
	th := basic.ThemeOf(ctx)
	l := basic.StringsOf(ctx)
	c := &th.Color
	enabled := w.OnChange != nil
	s.scope = fieldScopeOf(ctx)
	kit.ReportField(ctx, kit.FieldChoice)
	s.twelve = w.HourCycle == HourCycle12 || w.HourCycle == HourCycleLocale && !l.Hour24
	s.parts = 0
	add := func(p timePart) {
		s.order[s.parts] = p
		s.parts++
	}
	if s.twelve && l.PeriodFirst {
		add(partPeriod)
	}
	add(partHour)
	add(partMinute)
	if w.Seconds {
		add(partSecond)
	}
	if s.twelve && !l.PeriodFirst {
		add(partPeriod)
	}
	if !enabled {
		s.focused = partCount
	}

	style := kit.Colored(th.Text.Body, c.Ink)
	if !enabled {
		style.Color = c.Ink3
	}
	digits := kit.TextWidth(shaper, "00", style) + 2*timeSegmentPadding
	periodWidth := max(kit.TextWidth(shaper, l.AM, style), kit.TextWidth(shaper, l.PM, style)) + 2*timeSegmentPadding
	children := make([]core.Widget, 0, 2*s.parts)
	for i := range s.parts {
		p := s.order[i]
		if i > 0 {
			if p == partPeriod || s.order[i-1] == partPeriod {
				children = append(children, basic.SizedBox{Width: theme.Space2})
			} else {
				children = append(children, face{shaper: shaper, content: ":", style: style})
			}
		}
		width := digits
		if p == partPeriod {
			width = periodWidth
		}
		children = append(children, s.part(l, shaper, th, p, style, width, enabled))
	}

	ground := c.Field
	border := motion.LerpColor(motion.LerpColor(c.Hair, c.Ink3, s.hot.Get()), c.AccentInk, s.lit.Get())
	ring := c.AccentInk
	switch {
	case !enabled:
		ground, border = c.Fill, c.Hair
	case s.scope.err != "":
		border, ring = c.Danger, c.Danger
	}
	var box core.Widget = kit.Ring{Radius: th.Radius.Control, Color: ring.WithAlpha(kit.RingOpacity * s.lit.Get()), Child: basic.Container{
		Height:  fieldHeight,
		Color:   ground,
		Radius:  th.Radius.Control,
		Border:  paint.Stroke{Width: 1, Color: border},
		Padding: geom.Insets{Left: timeFieldPadding, Right: timeFieldPadding},
		Child:   basic.Row{Size: layout.MainMin, Cross: layout.CrossCenter, Children: children},
	}}
	if enabled {
		box = interact.MouseRegion{
			OnEnter: func() { s.hover(true) },
			OnExit:  func() { s.hover(false) },
			Child: interact.GestureDetector{
				// A click beside the parts goes to the first one.
				OnTap: func() {
					if s.focused == partCount {
						s.moveTo(0)
					}
				},
				Child: box,
			},
		}
	}
	state := s.scope.state()
	if !enabled {
		state |= semantics.Disabled
	}
	value := ""
	for i := range s.parts {
		switch {
		case i == 0:
		case s.order[i] == partPeriod || s.order[i-1] == partPeriod:
			value += " "
		default:
			value += ":"
		}
		value += s.spell(l, s.order[i])
	}
	sem := interact.Semantics{Role: semantics.RoleGroup, Label: w.Label, Value: value, Hint: s.scope.err, State: state, Child: basic.Row{
		Size: layout.MainMin, Children: []core.Widget{box},
	}}
	if w.Label == "" || w.HideLabel {
		return sem
	}
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: fieldLabelGap, Children: []core.Widget{
		basic.Text{Content: w.Label, Style: kit.Colored(th.Text.Caption, c.Ink2)},
		sem,
	}}
}

// part builds part p of the field.
func (s *timePickerState) part(l *basic.Strings, shaper *text.Shaper, th *theme.Theme, p timePart,
	style text.Style, width float32, enabled bool,
) core.Widget {
	lo, hi := s.limits(p)
	n := s.number(p)
	label := [partCount]string{l.Hour, l.Minute, l.Second, l.DayPeriod}[p]
	spelled := s.spell(l, p)
	r := semantics.Range{Min: float64(lo), Max: float64(hi), Value: float64(n), Step: 1}
	if p == partMinute {
		r.Step = float64(max(s.Widget().MinuteStep, 1))
	}
	sem := interact.Semantics{Role: semantics.RoleStepper, Label: label, Value: spelled, Range: r}
	var ground geom.Color
	if s.focused == p && enabled {
		ground, style.Color = th.Color.Accent, th.Color.OnAccent
	}
	look := face{
		shaper: shaper, content: spelled, style: style, width: width, height: timeSegmentHeight,
		ground: ground, radius: th.Radius.Check,
	}
	if !enabled {
		sem.State = semantics.Disabled
		sem.Child = look
		return sem
	}
	sem.OnIncrease = func() { s.step(p, 1) }
	sem.OnDecrease = func() { s.step(p, -1) }
	sem.OnSetValue = func(_ string, v float64) {
		if n := int(v); n >= lo && n <= hi {
			s.change(s.with(p, n))
		}
	}
	sem.Child = focusHandle{
		ref:           &s.refs[p],
		enabled:       true,
		onKey:         func(e *core.KeyEvent) bool { return s.key(p, e) },
		onFocusChange: func(focused bool) { s.partFocus(p, focused) },
		// The press must be handled for the part to take the focus.
		child: interact.GestureDetector{OnTapDown: func() {}, Child: look},
	}
	return sem
}
