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
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

// This file holds the grids of months and of years of a Calendar, which its
// title opens, and the title itself.

// calendarView is what the grid of a calendar shows.
type calendarView uint8

const (
	viewDays calendarView = iota
	viewMonths
	viewYears
)

// String implements fmt.Stringer.
func (v calendarView) String() string {
	switch v {
	case viewMonths:
		return "months"
	case viewYears:
		return "years"
	}
	return "days"
}

// The measures of the grids of months and of years. They fill the room of the
// days, names included, so that the card around does not move as they turn.
const (
	// calendarPicks is the number of cells of a grid, in calendarPickRows of
	// calendarPickColumns.
	calendarPicks       = 12
	calendarPickColumns = 4
	calendarPickRows    = calendarPicks / calendarPickColumns
	calendarPickWidth   = calendarWidth / calendarPickColumns
	calendarPickHeight  = (calendarWeekdays + 6*calendarCell) / calendarPickRows
	// calendarPill is the size of the ground of a month or a year, as round
	// as that of a day.
	calendarPillWidth  = 48
	calendarPillHeight = calendarCell
	// calendarTitlePadding is the room left and right of the title in its
	// button, and calendarTitleGlyph the size of its chevron.
	calendarTitlePadding = theme.Space8
	calendarTitleGlyph   = 12
)

// decadeOf returns the first year of the decade of year y.
func decadeOf(y int) int { return floorDiv(y, 10) * 10 }

// titleOf returns the title of the view: the month and the year, the year,
// or the decade.
func (s *calendarState) titleOf(l *basic.Strings) string {
	switch s.view {
	case viewMonths:
		return l.Year(s.focus.Year())
	case viewYears:
		d := decadeOf(s.focus.Year())
		return l.Years(d, d+9)
	}
	return l.MonthYear(s.page.Month(), s.page.Year())
}

// header builds the title and the arrows of the view.
func (s *calendarState) header(th *theme.Theme, l *basic.Strings, shaper *text.Shaper, title string, enabled bool) core.Widget {
	w := s.Widget()
	var prev, next func()
	prevLabel, nextLabel := l.PreviousMonth, l.NextMonth
	switch s.view {
	case viewMonths:
		prevLabel, nextLabel = l.PreviousYear, l.NextYear
		y := s.focus.Year()
		if enabled && (w.Min.IsZero() || civil(w.Min).Year() < y) {
			prev = func() { s.shift(-12) }
		}
		if enabled && (w.Max.IsZero() || civil(w.Max).Year() > y) {
			next = func() { s.shift(12) }
		}
	case viewYears:
		prevLabel, nextLabel = l.PreviousYears, l.NextYears
		d := decadeOf(s.focus.Year())
		if enabled && (w.Min.IsZero() || civil(w.Min).Year() < d) {
			prev = func() { s.shift(-120) }
		}
		if enabled && (w.Max.IsZero() || civil(w.Max).Year() > d+9) {
			next = func() { s.shift(120) }
		}
	default:
		if enabled && (w.Min.IsZero() || s.page.After(civil(w.Min))) {
			prev = func() { s.turn(-1) }
		}
		if enabled && (w.Max.IsZero() || !addMonths(s.page, 1).After(civil(w.Max))) {
			next = func() { s.turn(1) }
		}
	}
	return basic.SizedBox{Width: calendarWidth, Child: basic.Row{Cross: layout.CrossCenter, Children: []core.Widget{
		basic.Expanded{Child: basic.Row{Size: layout.MainMax, Cross: layout.CrossCenter, Children: []core.Widget{
			interact.Semantics{
				Role: semantics.RoleHeading, Level: 2, Label: title, Live: semantics.LivePolite,
				Child: s.titleButton(th, shaper, title, enabled),
			},
		}}},
		calendarButton(th, prevLabel, s.prev, prev),
		calendarButton(th, nextLabel, s.next, next),
	}}}
}

// titleButton builds the title, a button that opens the months of the year
// or the years of the decade. The decade, the last view, is a plain title.
func (s *calendarState) titleButton(th *theme.Theme, shaper *text.Shaper, title string, enabled bool) core.Widget {
	c := &th.Color
	style := kit.Colored(th.Text.Headline, c.Ink)
	if !enabled {
		style.Color = c.Ink3
	}
	plain := face{shaper: shaper, content: title, style: style, start: true}
	if s.view == viewYears || !enabled {
		return basic.Container{Height: calendarNav, Padding: geom.Insets{Left: calendarTitlePadding, Right: calendarTitlePadding}, Child: basic.Center{
			Child: plain,
		}}
	}
	up := viewMonths
	if s.view == viewMonths {
		up = viewYears
	}
	return interact.Pressable{Label: title, OnPress: func() { s.setView(up) }, Build: func(ps interact.PressState) core.Widget {
		var ground geom.Color
		switch {
		case ps.Pressed:
			ground = c.FillStrong
		case ps.Hovered:
			ground = c.Fill
		}
		return interact.FocusRing{Visible: ps.FocusVisible, Radius: th.Radius.Control, Child: basic.Container{
			Height: calendarNav, Color: ground, Radius: th.Radius.Control,
			Padding: geom.Insets{Left: calendarTitlePadding, Right: calendarTitlePadding - theme.Space2},
			Child: basic.Row{Size: layout.MainMin, Cross: layout.CrossCenter, Spacing: theme.Space4, Children: []core.Widget{
				plain,
				basic.Icon{Path: s.title, Size: calendarTitleGlyph, Color: c.Ink2},
			}},
		}}
	}}
}

// setView shows view v, and gives the keyboard to its grid.
func (s *calendarState) setView(v calendarView) {
	s.SetState(func() {
		s.view, s.follow, s.hovered = v, true, -1
		s.page = time.Date(s.focus.Year(), s.focus.Month(), 1, 0, 0, 0, 0, time.UTC)
	})
}

// shift moves the day in focus by n months, and the year or the decade shown
// with it.
func (s *calendarState) shift(n int) {
	s.SetState(func() {
		s.focus = s.clamp(addMonths(s.focus, n))
		s.page = time.Date(s.focus.Year(), s.focus.Month(), 1, 0, 0, 0, 0, time.UTC)
	})
}

// firstPick returns the year of the first cell of the grid of years: the
// year before the decade, as a month shows the days around it.
func (s *calendarState) firstPick() int { return decadeOf(s.focus.Year()) - 1 }

// pickIndex returns the cell of the civil day d in the grid shown.
func (s *calendarState) pickIndex(d time.Time) int {
	if s.view == viewYears {
		return d.Year() - s.firstPick()
	}
	return int(d.Month()) - 1
}

// pickDay returns the day that cell i of the grid shown stands for: the day of
// the focus in that month or year, kept within Min and Max.
func (s *calendarState) pickDay(i int) time.Time {
	y, m := s.focus.Year(), time.Month(i+1)
	if s.view == viewYears {
		y, m = s.firstPick()+i, s.focus.Month()
	}
	return s.clamp(time.Date(y, m, min(s.focus.Day(), daysIn(y, m)), 0, 0, 0, 0, time.UTC))
}

// pickAllowed reports whether cell i holds a day between Min and Max.
func (s *calendarState) pickAllowed(i int) bool {
	w := s.Widget()
	if w.OnChange == nil {
		return false
	}
	var from, to time.Time
	if s.view == viewYears {
		y := s.firstPick() + i
		from, to = time.Date(y, time.January, 1, 0, 0, 0, 0, time.UTC), time.Date(y, time.December, 31, 0, 0, 0, 0, time.UTC)
	} else {
		y, m := s.focus.Year(), time.Month(i+1)
		from, to = time.Date(y, m, 1, 0, 0, 0, 0, time.UTC), time.Date(y, m, daysIn(y, m), 0, 0, 0, 0, time.UTC)
	}
	return (w.Min.IsZero() || !to.Before(civil(w.Min))) && (w.Max.IsZero() || !from.After(civil(w.Max)))
}

// pick chooses cell i: a year shows its months, a month its days.
func (s *calendarState) pick(i int) {
	if !s.pickAllowed(i) {
		return
	}
	d := s.pickDay(i)
	next := viewDays
	if s.view == viewYears {
		next = viewMonths
	}
	s.SetState(func() {
		s.focus = d
		s.page = time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC)
		s.view, s.follow, s.hovered = next, true, -1
	})
}

// movePick puts the focus on the day d, within Min and Max, in the grid
// shown, turning the year or the decade when d leaves it, and gives its cell
// the keyboard.
func (s *calendarState) movePick(d time.Time) {
	d = s.clamp(d)
	s.SetState(func() {
		s.focus = d
		s.page = time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC)
	})
	// The cells stay the same objects whatever the year or the decade, so
	// the one for d is there now; see move.
	if i := s.pickIndex(d); i >= 0 && i < calendarPicks && s.picks[i] != nil && s.picks[i].Attached() {
		s.picks[i].Enabled = true
		s.picks[i].RequestFocus()
	}
}

// pickKey handles the keys of the grids of months and of years.
func (s *calendarState) pickKey(e *core.KeyEvent) bool {
	if e.Modifiers&(input.ModControl|input.ModAlt|input.ModShift) != 0 {
		return false
	}
	// unit is the number of months a cell stands for.
	unit := 1
	if s.view == viewYears {
		unit = 12
	}
	col := s.pickIndex(s.focus) % calendarPickColumns
	var n int
	switch e.Key {
	case input.KeyLeft:
		n = -1
	case input.KeyRight:
		n = 1
	case input.KeyUp:
		n = -calendarPickColumns
	case input.KeyDown:
		n = calendarPickColumns
	case input.KeyHome:
		n = -col
	case input.KeyEnd:
		n = calendarPickColumns - 1 - col
	case input.KeyPageUp, input.KeyPageDown:
		// A year of months, or a decade of years.
		n = 12
		if s.view == viewYears {
			n = 10
		}
		if e.Key == input.KeyPageUp {
			n = -n
		}
	case input.KeyEnter, input.KeySpace:
		if !e.Repeat {
			s.pick(s.pickIndex(s.focus))
		}
		return true
	default:
		return false
	}
	s.movePick(addMonths(s.focus, n*unit))
	return true
}

// monthGrid builds the twelve months of the year of the focus.
func (s *calendarState) monthGrid(th *theme.Theme, l *basic.Strings, shaper *text.Shaper, title string) core.Widget {
	w := s.Widget()
	y := s.focus.Year()
	var chosen, today time.Time
	if !w.Value.IsZero() {
		chosen = civil(w.Value)
	}
	if !w.Today.IsZero() {
		today = civil(w.Today)
	}
	return s.pickGrid(title, func(i int) core.Widget {
		m := time.Month(i + 1)
		return s.pickCell(th, shaper, i, pickLook{
			content: l.ShortMonths[i], label: l.MonthYear(m, y),
			selected: !chosen.IsZero() && chosen.Year() == y && chosen.Month() == m,
			current:  !today.IsZero() && today.Year() == y && today.Month() == m,
			inside:   true,
		})
	})
}

// yearGrid builds the years of the decade of the focus, with the year before
// and the one after it.
func (s *calendarState) yearGrid(th *theme.Theme, l *basic.Strings, shaper *text.Shaper, title string) core.Widget {
	w := s.Widget()
	first := s.firstPick()
	decade := decadeOf(s.focus.Year())
	chosen, today := -1, -1
	if !w.Value.IsZero() {
		chosen = civil(w.Value).Year()
	}
	if !w.Today.IsZero() {
		today = civil(w.Today).Year()
	}
	return s.pickGrid(title, func(i int) core.Widget {
		y := first + i
		// The digits only: the cell is narrow, and the title names the unit.
		return s.pickCell(th, shaper, i, pickLook{
			content: kit.Itoa(y), label: l.Year(y),
			selected: y == chosen, current: y == today,
			inside: y >= decade && y <= decade+9,
		})
	})
}

// pickGrid lays out the cells that cell builds in rows, as a grid named
// title.
func (s *calendarState) pickGrid(title string, cell func(i int) core.Widget) core.Widget {
	var state semantics.State
	if s.Widget().OnChange == nil {
		state = semantics.Disabled
	}
	rows := make([]core.Widget, calendarPickRows)
	for r := range rows {
		cells := make([]core.Widget, calendarPickColumns)
		for c := range cells {
			cells[c] = cell(r*calendarPickColumns + c)
		}
		rows[r] = interact.Semantics{Role: semantics.RoleRow, Child: basic.Row{Size: layout.MainMin, Children: cells}}
	}
	return interact.Semantics{Role: semantics.RoleGrid, Label: title, State: state, Child: basic.Column{
		Size: layout.MainMin, Cross: layout.CrossStart, Children: rows,
	}}
}

// pickLook is what a cell of the grid of months or of years shows.
type pickLook struct {
	// content is what the cell shows, and label what a screen reader hears.
	content, label string
	// selected marks the month or year of Value, current that of Today, and
	// inside a year of the decade shown.
	selected, current, inside bool
}

// pickCell builds cell i of the grid of months or of years.
func (s *calendarState) pickCell(th *theme.Theme, shaper *text.Shaper, i int, look pickLook) core.Widget {
	c := &th.Color
	enabled := s.Widget().OnChange != nil
	allowed := s.pickAllowed(i)
	style := kit.Colored(th.Text.Body, c.Ink)
	var ground geom.Color
	switch {
	case look.selected && enabled:
		ground, style.Color = c.Accent, c.OnAccent
	case look.selected:
		ground, style.Color = c.FillStrong, c.Ink3
	case !allowed || !look.inside:
		style.Color = c.Ink3
	case look.current:
		style.Color = c.AccentInk
	}
	if look.current {
		style.Weight = th.Text.Headline.Weight
	}
	if !look.selected && allowed && s.hovered == i {
		ground = c.Fill
	}
	var sem semantics.State
	if look.selected {
		sem |= semantics.Selected
	}
	var tap func()
	cursor := input.CursorArrow
	if allowed {
		tap = func() { s.pick(i) }
		cursor = input.CursorHand
	} else {
		sem |= semantics.Disabled
	}
	here := i == s.pickIndex(s.focus)
	var ring geom.Color
	if here && s.focused && s.shown {
		ring = c.AccentInk.WithAlpha(kit.RingOpacity)
	}
	pill := kit.Ring{Radius: calendarPillHeight / 2, Color: ring, Child: face{
		shaper: shaper, content: look.content, style: style,
		width: calendarPillWidth, height: calendarPillHeight,
		ground: ground, radius: calendarPillHeight / 2,
		strike: !allowed && enabled && look.inside,
	}}
	return interact.Semantics{Role: semantics.RoleGridCell, Label: look.label, State: sem, OnTap: tap, Child: focusHandle{
		ref:           &s.picks[i],
		enabled:       here && enabled,
		autofocus:     here && (s.Widget().autofocus || s.follow),
		onKey:         s.pickKey,
		onFocusChange: s.cellFocus,
		onShowFocus:   s.showFocus,
		child: interact.MouseRegion{
			OnEnter: func() { s.hover(i, true) },
			OnExit:  func() { s.hover(i, false) },
			Cursor:  cursor,
			Child: interact.GestureDetector{OnTap: tap, Child: basic.SizedBox{
				Width: calendarPickWidth, Height: calendarPickHeight, Child: basic.Center{Child: pill},
			}},
		},
	}}
}
