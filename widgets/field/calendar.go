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
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

// The measures of a calendar.
const (
	calendarCell = 32
	// calendarWidth is the width of the seven columns.
	calendarWidth  = 7 * calendarCell
	calendarDays   = 42
	calendarNav    = 28
	calendarNavIco = 16
	// calendarWeekdays is the height of the row of the names of the days.
	calendarWeekdays = 24
	calendarTitleGap = theme.Space6
	// calendarMarks is the most dots a day shows, calendarDot their size and
	// calendarDotGap the room between them. calendarDotBottom is the distance
	// from the foot of the cell to the middle of the dots: low enough to clear
	// the number, high enough to stay inside the circle of the chosen day.
	calendarMarks     = 3
	calendarDot       = 4
	calendarDotGap    = 2
	calendarDotBottom = 6
)

// Calendar shows the days of a month in six weeks and lets the user choose
// one. A title with the month and the year sits above, with buttons for the
// month before and after, and the names of the days head the columns. The week
// starts on the day the locale of the window gives (MediaQueryData.Locale), as
// do the names.
//
// The title is a button that turns the days into the twelve months of the
// year, and the title of the months, the year, one that turns them into the
// years of a decade; the arrows then move by a year and by ten years. Choosing
// a year shows its months, and choosing a month its days, so a day decades
// away takes a few presses: a birth date in 1990 from 2026 is the title twice,
// ‹ three times, 1990, the month and the day. Months and years that hold no
// day between Min and Max cannot be chosen.
//
// Value is the chosen day, which shows in the accent, and Today the day to
// mark as today. The calendar never reads the clock: pass today from the
// application, so that tests and pictures stay the same. Only the year, month
// and day of the times count, in their own zones; OnChange gets midnight in
// the zone of Value, or of Today, or UTC.
//
// Min and Max, when not zero, bound the days that can be chosen, and
// Selectable can turn others away; those show crossed out. Without OnChange
// the calendar is disabled.
//
// Marks, when set, puts colored dots under days, such as the days that have
// events. A screen reader hears Strings.Marked after the date of such a day.
//
// The keys follow the date grid of ARIA: the arrows move by a day and a week,
// Page Up and Page Down by a month, with Shift by a year, Home and End to the
// ends of the week, and Enter or Space choose the day. The month turns as the
// focus leaves it. Tab reaches one day only, the one the arrows move. In the
// grids of months and years the arrows move by one and by a row of four, Page
// Up and Page Down by a year or ten, Home and End to the ends of the row, and
// Enter or Space choose; the keyboard follows into the grid that comes next. A
// screen reader hears the title as a button and the months and years as cells
// of a grid, named "January 1990" and "1990".
//
// The calendar opens on the month of Value, else InitialMonth, else Today,
// else Min. It follows Value when that changes to another day.
type Calendar struct {
	core.StatefulBase
	Value time.Time
	Today time.Time
	// InitialMonth is the month to open on while Value is zero, near the day
	// that is likely to be chosen: some thirty years back for a birth date.
	// Only its year and month count.
	InitialMonth time.Time
	// Min and Max are the first and the last day that can be chosen. Zero
	// leaves that end open.
	Min, Max time.Time
	// Selectable reports whether a day can be chosen. Nil allows every day
	// between Min and Max.
	Selectable func(day time.Time) bool
	// Marks returns the colors of the dots under a day, at midnight in the zone
	// OnChange uses; nil or empty draws none. The first three show, in order;
	// the rest are left out, so put the one that matters most first. Give
	// colors meant to be read on Canvas and Card, such as AccentInk, Danger
	// and Success of the theme. Today and the other days keep them; the dots
	// of the chosen day take the color of its number (OnAccent on the
	// accent), and those of a day outside the month or that cannot be chosen
	// take Ink3, as its number does.
	//
	// The calendar calls Marks when it builds, once for each of the 42 days
	// it shows, never when it paints, and does not keep the slice: a function
	// may hand out the same backing array each time.
	Marks    func(day time.Time) []geom.Color
	OnChange func(day time.Time)

	// autofocus gives the keyboard to the day at once, as in a DatePicker.
	autofocus bool
}

type calendarState struct {
	core.StateOf[Calendar]
	// page is the first day of the month shown, and focus the day that Tab
	// and the arrows reach, both civil. The grids of months and years show
	// the year and the decade of focus.
	page, focus time.Time
	view        calendarView
	// hovered is the cell under the pointer, or -1.
	hovered        int
	focused, shown bool
	cells          [calendarDays]*layout.RenderFocus
	// picks are the cells of the grid of months or of years.
	picks [calendarPicks]*layout.RenderFocus
	// follow gives the keyboard to the grid that a change of view builds, as
	// the cell that had it is gone.
	follow bool
	// first is the day the weeks start on, as the last build found it.
	first             time.Weekday
	prev, next, title *paint.Path
}

// CreateState implements core.StatefulWidget.
func (Calendar) CreateState() core.State { return &calendarState{hovered: -1} }

// InitState implements core.State.
func (s *calendarState) InitState() {
	s.prev, s.next, s.title = kit.ChevronPath(false), kit.ChevronPath(true), kit.ChevronGlyph()
	w := s.Widget()
	start := w.Value
	if start.IsZero() && !w.InitialMonth.IsZero() {
		start = time.Date(w.InitialMonth.Year(), w.InitialMonth.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	if start.IsZero() {
		start = w.Today
	}
	if start.IsZero() {
		start = w.Min
	}
	s.show(s.clamp(civil(start)))
}

// DidUpdateWidget implements core.State.
func (s *calendarState) DidUpdateWidget(old core.Widget) {
	w, was := s.Widget(), old.(Calendar)
	if !w.Value.IsZero() && (was.Value.IsZero() || civil(w.Value) != civil(was.Value)) {
		s.show(civil(w.Value))
	}
}

// show makes d the day in focus, and turns to its month.
func (s *calendarState) show(d time.Time) {
	s.focus = d
	s.page = time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// clamp keeps the civil day d between Min and Max.
func (s *calendarState) clamp(d time.Time) time.Time {
	w := s.Widget()
	if !w.Min.IsZero() && d.Before(civil(w.Min)) {
		d = civil(w.Min)
	}
	if !w.Max.IsZero() && d.After(civil(w.Max)) {
		d = civil(w.Max)
	}
	return d
}

// allowed reports whether the civil day d can be chosen.
func (s *calendarState) allowed(d time.Time) bool {
	w := s.Widget()
	if w.OnChange == nil || !d.Equal(s.clamp(d)) {
		return false
	}
	return w.Selectable == nil || w.Selectable(in(d, s.zone()))
}

// zone returns the time whose zone the days handed out are in.
func (s *calendarState) zone() time.Time {
	if w := s.Widget(); !w.Value.IsZero() {
		return w.Value
	}
	return s.Widget().Today
}

// firstDay returns the first day of the grid of the page: the start of the
// week of the first of the month.
func (s *calendarState) firstDay(first time.Weekday) time.Time {
	back := (int(s.page.Weekday()) - int(first) + 7) % 7
	return s.page.AddDate(0, 0, -back)
}

// cellOf returns the cell that shows the civil day d, or -1.
func (s *calendarState) cellOf(d time.Time, first time.Weekday) int {
	i := int(d.Sub(s.firstDay(first)).Hours()+0.5) / 24
	if i < 0 || i >= calendarDays {
		return -1
	}
	return i
}

// move puts the focus on the civil day d, within Min and Max, turning the
// month if d is outside the one shown, and gives that day the keyboard.
func (s *calendarState) move(d time.Time) {
	d = s.clamp(d)
	first := s.first
	s.SetState(func() {
		s.focus = d
		if d.Year() != s.page.Year() || d.Month() != s.page.Month() {
			s.page = time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC)
		}
	})
	// The cells stay the same objects whatever the month, so the one for d is
	// there now. It is enabled at once, as the build will do, because the
	// build first drops the focus of what cannot take it.
	if i := s.cellOf(d, first); i >= 0 && s.cells[i] != nil && s.cells[i].Attached() {
		s.cells[i].Enabled = true
		s.cells[i].RequestFocus()
	}
}

// turn shows the month n months away, and moves the day in focus with it.
func (s *calendarState) turn(n int) {
	s.SetState(func() {
		s.page = addMonths(s.page, n)
		s.focus = s.clamp(addMonths(s.focus, n))
	})
}

// choose picks the civil day d, if it can be chosen.
func (s *calendarState) choose(d time.Time) {
	if !s.allowed(d) {
		return
	}
	if !d.Equal(s.focus) {
		s.move(d)
	}
	s.Widget().OnChange(in(d, s.zone()))
}

func (s *calendarState) key(e *core.KeyEvent) bool {
	if e.Modifiers&(input.ModControl|input.ModAlt) != 0 {
		return false
	}
	shift := e.Modifiers&input.ModShift != 0
	first := s.first
	d := s.focus
	switch e.Key {
	case input.KeyLeft:
		d = d.AddDate(0, 0, -1)
	case input.KeyRight:
		d = d.AddDate(0, 0, 1)
	case input.KeyUp:
		d = d.AddDate(0, 0, -7)
	case input.KeyDown:
		d = d.AddDate(0, 0, 7)
	case input.KeyPageUp, input.KeyPageDown:
		n := 1
		if shift {
			n = 12
		}
		if e.Key == input.KeyPageUp {
			n = -n
		}
		d = addMonths(d, n)
	case input.KeyHome:
		d = d.AddDate(0, 0, -((int(d.Weekday()) - int(first) + 7) % 7))
	case input.KeyEnd:
		d = d.AddDate(0, 0, 6-(int(d.Weekday())-int(first)+7)%7)
	case input.KeyEnter, input.KeySpace:
		if !e.Repeat {
			s.choose(d)
		}
		return true
	default:
		return false
	}
	s.move(d)
	return true
}

func (s *calendarState) cellFocus(focused bool) {
	if s.focused != focused {
		s.SetState(func() { s.focused = focused })
	}
}

func (s *calendarState) showFocus(show bool) {
	if s.shown != show {
		s.SetState(func() { s.shown = show })
	}
}

func (s *calendarState) hover(i int, inside bool) {
	switch {
	case inside && s.hovered != i:
		s.SetState(func() { s.hovered = i })
	case !inside && s.hovered == i:
		s.SetState(func() { s.hovered = -1 })
	}
}

// Build implements core.State.
func (s *calendarState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	shaper, ok := core.DependOn[*text.Shaper](ctx)
	if !ok {
		panic("gowaku: Calendar needs a text.Shaper above it; wrap the root in core.Inherited[*text.Shaper]")
	}
	th := basic.ThemeOf(ctx)
	l := basic.StringsOf(ctx)
	first := basic.WeekStart(ctx)
	s.first = first
	enabled := w.OnChange != nil
	title := s.titleOf(l)
	var grid core.Widget
	switch s.view {
	case viewMonths:
		grid = s.monthGrid(th, l, shaper, title)
	case viewYears:
		grid = s.yearGrid(th, l, shaper, title)
	default:
		grid = s.dayGrid(th, l, shaper, title, first)
	}
	// The key makes a new grid of each view, whose cell in focus takes the
	// keyboard as it appears.
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: calendarTitleGap, Children: []core.Widget{
		s.header(th, l, shaper, title, enabled),
		core.Keyed(s.view, grid),
	}}
}

// dayGrid builds the days of the month shown, under the names of the days.
func (s *calendarState) dayGrid(th *theme.Theme, l *basic.Strings, shaper *text.Shaper, title string, first time.Weekday) core.Widget {
	w := s.Widget()
	enabled := w.OnChange != nil
	names := make([]core.Widget, 7)
	for i := range names {
		day := (int(first) + i) % 7
		names[i] = interact.Semantics{Role: semantics.RoleColumnHeader, Label: l.Weekdays[day], Child: face{
			shaper: shaper, content: l.NarrowWeekdays[day], style: kit.Colored(th.Text.Caption, th.Color.Ink2),
			width: calendarCell, height: calendarWeekdays,
		}}
	}
	rows := make([]core.Widget, 0, 7)
	rows = append(rows, interact.Semantics{Role: semantics.RoleRow, Child: basic.Row{Size: layout.MainMin, Children: names}})

	start := s.firstDay(first)
	today, value := time.Time{}, time.Time{}
	if !w.Today.IsZero() {
		today = civil(w.Today)
	}
	if !w.Value.IsZero() {
		value = civil(w.Value)
	}
	for r := range 6 {
		cells := make([]core.Widget, 7)
		for c := range cells {
			i := r*7 + c
			cells[c] = s.cell(th, l, shaper, i, start.AddDate(0, 0, i), today, value)
		}
		rows = append(rows, interact.Semantics{Role: semantics.RoleRow, Child: basic.Row{Size: layout.MainMin, Children: cells}})
	}
	var state semantics.State
	if !enabled {
		state = semantics.Disabled
	}
	return interact.Semantics{Role: semantics.RoleGrid, Label: title, State: state, Child: basic.Column{
		Size: layout.MainMin, Cross: layout.CrossStart, Children: rows,
	}}
}

// cell builds cell i, which shows the civil day d.
func (s *calendarState) cell(th *theme.Theme, l *basic.Strings, shaper *text.Shaper, i int, d, today, value time.Time) core.Widget {
	c := &th.Color
	enabled := s.Widget().OnChange != nil
	allowed := s.allowed(d)
	inMonth := d.Month() == s.page.Month()
	selected := d.Equal(value)
	style := kit.Colored(th.Text.Body, c.Ink)
	var ground geom.Color
	// muted is the ink that every dot takes in place of its own, or zero.
	var muted geom.Color
	switch {
	case selected && enabled:
		ground, style.Color = c.Accent, c.OnAccent
		muted = c.OnAccent
	case selected:
		ground, style.Color = c.FillStrong, c.Ink3
		muted = c.Ink3
	case !allowed || !inMonth:
		style.Color = c.Ink3
		muted = c.Ink3
	case d.Equal(today):
		style.Color = c.AccentInk
	}
	var dots [calendarMarks]geom.Color
	label := l.SpokenDate(d)
	if marks := s.Widget().Marks; marks != nil {
		n := 0
		for _, m := range marks(in(d, s.zone())) {
			if n == calendarMarks {
				break
			}
			if m.A <= 0 {
				continue
			}
			if muted.A > 0 {
				m = muted
			}
			dots[n] = m
			n++
		}
		if n > 0 {
			label += ", " + l.Marked
		}
	}
	if d.Equal(today) {
		style.Weight = th.Text.Headline.Weight
	}
	if !selected && allowed && s.hovered == i {
		ground = c.Fill
	}
	var sem semantics.State
	if selected {
		sem |= semantics.Selected
	}
	var tap func()
	cursor := input.CursorArrow
	if allowed {
		tap = func() { s.choose(d) }
		cursor = input.CursorHand
	} else {
		sem |= semantics.Disabled
	}
	here := d.Equal(s.focus)
	var ring geom.Color
	if here && s.focused && s.shown {
		ring = c.AccentInk.WithAlpha(kit.RingOpacity)
	}
	return interact.Semantics{Role: semantics.RoleGridCell, Label: label, State: sem, OnTap: tap, Child: focusHandle{
		ref:           &s.cells[i],
		enabled:       here && enabled,
		autofocus:     here && (s.Widget().autofocus || s.follow),
		onKey:         s.key,
		onFocusChange: s.cellFocus,
		onShowFocus:   s.showFocus,
		child: interact.MouseRegion{
			OnEnter: func() { s.hover(i, true) },
			OnExit:  func() { s.hover(i, false) },
			Cursor:  cursor,
			Child: interact.GestureDetector{OnTap: tap, Child: kit.Ring{Radius: calendarCell / 2, Color: ring, Child: face{
				shaper: shaper, content: kit.Itoa(d.Day()), style: style,
				width: calendarCell, height: calendarCell,
				ground: ground, radius: calendarCell / 2, strike: !allowed && enabled && inMonth,
				dots: dots,
			}}},
		},
	}}
}

// calendarButton is a button of the title of a calendar, which shows an icon
// and names itself to a screen reader with label.
func calendarButton(th *theme.Theme, label string, icon *paint.Path, onPress func()) core.Widget {
	return interact.Pressable{Label: label, OnPress: onPress, Build: func(ps interact.PressState) core.Widget {
		c := &th.Color
		ink := c.Ink2
		var ground geom.Color
		switch {
		case !ps.Enabled:
			ink = c.Ink3
		case ps.Pressed:
			ground = c.FillStrong
		case ps.Hovered:
			ground = c.Fill
		}
		return interact.FocusRing{Visible: ps.FocusVisible, Radius: th.Radius.Control, Child: basic.Container{
			Width: calendarNav, Height: calendarNav, Color: ground, Radius: th.Radius.Control,
			Child: basic.Center{Child: basic.Icon{Path: icon, Size: calendarNavIco, Color: ink}},
		}}
	}}
}
