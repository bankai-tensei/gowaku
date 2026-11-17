package field_test

import (
	"testing"
	"time"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	fieldw "github.com/bankai-tensei/gowaku/widgets/field"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
)

// birthDate is the date field of a registration form, which keeps its value.
type birthDate struct {
	core.StatefulBase
	initial time.Time
	got     *time.Time
}

type birthDateState struct {
	core.StateBase
	v time.Time
}

func (birthDate) CreateState() core.State { return &birthDateState{} }

func (s *birthDateState) Build(core.BuildContext) core.Widget {
	b := s.Widget().(birthDate)
	today := time.Date(2026, time.November, 16, 0, 0, 0, 0, time.UTC)
	return fieldw.DatePicker{
		Label: "Birth date", Value: s.v, Today: today, Max: today, InitialMonth: b.initial,
		OnChange: func(d time.Time) {
			*b.got = d
			s.SetState(func() { s.v = d })
		},
	}
}

// spoken is the label of a day of the calendar in English.
func spoken(d time.Time) string {
	return d.Weekday().String() + ", " + d.Month().String() + " " + itoa(d.Day()) + ", " + itoa(d.Year())
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

// A birth date decades back takes a few taps: the title twice, back three
// decades, the year, the month and the day.
func TestDatePicker_ReachesABirthDateInAFewTaps(t *testing.T) {
	var got time.Time
	tt := gowakutest.New(t, birthDate{got: &got}, pressableFonts)
	tt.Tap("Birth date")
	tt.Tap("November 2026")
	if _, ok := tt.Find("January 2026"); !ok {
		t.Fatalf("the title must open the months of 2026:\n%s", tt.Dump())
	}
	tt.Tap("2026")
	if _, ok := tt.Find("2020–2029"); !ok {
		t.Fatalf("the year must open the decade:\n%s", tt.Dump())
	}
	if n, _ := tt.Find("2027"); n == nil || n.State&semantics.Disabled == 0 {
		t.Errorf("a year after Max must be disabled, got %+v", n)
	}
	for range 3 {
		tt.Tap("Previous years")
	}
	tt.Tap("1990")
	tt.Tap("May 1990")
	want := time.Date(1990, time.May, 15, 0, 0, 0, 0, time.UTC)
	tt.Tap(spoken(want))
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if n, _ := tt.Find("Birth date"); n == nil || n.Value != "May 15, 1990" {
		t.Errorf("the field must show the date, got %+v", n)
	}
	if w := tt.Warnings(); len(w) > 0 {
		t.Errorf("warnings: %v", w)
	}
}

// InitialMonth opens the calendar near the date to choose.
func TestDatePicker_OpensOnTheInitialMonth(t *testing.T) {
	var got time.Time
	tt := gowakutest.New(t, birthDate{initial: time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC), got: &got}, pressableFonts)
	tt.Tap("Birth date")
	if _, ok := tt.Find("January 1990"); !ok {
		t.Fatalf("the calendar must open on January 1990:\n%s", tt.Dump())
	}
	want := time.Date(1990, time.January, 20, 0, 0, 0, 0, time.UTC)
	tt.Tap(spoken(want))
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// The grids of months and years answer the keyboard as the days do, and the
// keyboard follows into the grid that comes next.
func TestCalendar_MonthsAndYearsByKeyboard(t *testing.T) {
	var got time.Time
	tt := gowakutest.New(t, birthDate{got: &got}, pressableFonts)
	tt.Tap("Birth date")
	focused := func() *gowakutest.Node {
		root, id := tt.Tree().SemanticsTree()
		return root.Find(id)
	}
	// Shift+Tab from the day goes to the arrow back, then to the title: the
	// arrow forward is off, as Max is today.
	for range 2 {
		tt.Key(input.KeyTab, input.ModShift)
	}
	if n := focused(); n == nil || n.Label != "November 2026" {
		t.Fatalf("focus on %+v, want the title", n)
	}
	tt.Key(input.KeyEnter, 0)
	if n := focused(); n == nil || n.Role != semantics.RoleGridCell || n.Label != "November 2026" {
		t.Fatalf("focus on %+v, want the month", n)
	}
	tt.Key(input.KeyPageUp, 0)
	tt.Key(input.KeyUp, 0)
	if n := focused(); n == nil || n.Label != "July 2025" {
		t.Fatalf("focus on %+v, want July 2025", n)
	}
	tt.Key(input.KeyEnter, 0)
	if n := focused(); n == nil || n.Role != semantics.RoleGridCell || n.Label != spoken(time.Date(2025, time.July, 16, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("focus on %+v, want July 16, 2025", n)
	}
	tt.Key(input.KeyEnter, 0)
	if want := time.Date(2025, time.July, 16, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// localized puts w in a window of the locale tag.
func localized(tag string, w core.Widget) core.Widget {
	return basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1, Locale: language.MustParse(tag)}, Child: w}
}

// The years of a calendar speak the locale.
func TestCalendar_YearsInJapanese(t *testing.T) {
	tt := gowakutest.New(t, localized("ja", fieldw.Calendar{
		Value: time.Date(1990, time.May, 15, 0, 0, 0, 0, time.UTC), OnChange: func(time.Time) {},
	}), pressableFonts)
	tt.Tap("1990年5月")
	tt.Tap("1990年")
	if _, ok := tt.Find("1990年–1999年"); !ok {
		t.Fatalf("the decade must be titled in Japanese:\n%s", tt.Dump())
	}
	if n, _ := tt.Find("1990年"); n == nil || n.Role != semantics.RoleGridCell || n.State&semantics.Selected == 0 {
		t.Errorf("the year of the value must be a selected cell, got %+v", n)
	}
}

// The captions of the fields of a ColorPicker read in light and in dark.
func TestColorPicker_CaptionsMeetContrast(t *testing.T) {
	for _, dark := range []bool{false, true} {
		opts := pressableFonts
		opts.Dark = dark
		tt := gowakutest.New(t, fieldw.ColorPicker{
			Value: geom.Hex(0x2f5fd0), Numeric: true, OnChange: func(geom.Color) {},
		}, opts)
		if w := tt.Warnings(); len(w) > 0 {
			t.Errorf("dark %v: %v", dark, w)
		}
	}
}
