package basic

import (
	"time"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// Strings holds the words the standard widgets show and say for one language:
// the names of months and days, the labels of buttons and the names screen
// readers speak. English and Japanese come with gowaku; an application adds a
// language, or changes the words of one, by giving its own Strings to
// App.Strings or to a Localizations widget. Start from EnglishStrings to keep
// what you do not change.
//
// A table may leave words out. Empty strings, and arrays whose first element
// is empty, read as the English words; nil Format functions write the English
// word order with the words of the table. The bool fields have no fallback:
// their zero value is the English behavior.
//
// The week of a calendar does not come from the table: it starts on the day
// CLDR gives for the region of the locale, or the likely region of its
// language; see WeekStart. Numbers come from the locale too; see FormatInt.
//
// A table must not change once it is given to a Localizations; give a new one
// instead.
type Strings struct {
	// Language is the language, and optionally the script and region, the table
	// is for. Localizations matches the locale of the window against it, so
	// "de" serves de-DE and de-AT, and "en-GB" wins over the bundled English
	// for en-GB.
	Language language.Tag

	// Calendar and DatePicker

	// Months are the names of the months from January, and ShortMonths their
	// short forms.
	Months, ShortMonths [12]string
	// Weekdays are the names of the days from Sunday in full (Sunday, 日曜日),
	// which a screen reader says for the columns of a calendar. ShortWeekdays
	// are their short forms (Sun, 日), as ShortDate writes them, and
	// NarrowWeekdays the narrowest that still tell the days apart (Su, 日),
	// the column heads of a calendar.
	Weekdays, ShortWeekdays, NarrowWeekdays [7]string
	// Today and Clear are the buttons of a date field, and ChooseDate names
	// the calendar it opens. PreviousMonth and NextMonth are the arrows of a
	// calendar. Yesterday is the word MessageDate writes for the day before.
	Today, Yesterday, Clear, ChooseDate string
	PreviousMonth, NextMonth            string
	// ChooseMonth and ChooseYear name the grids of months and of years that
	// the title of a calendar opens. PreviousYear and NextYear are the arrows
	// over the months, PreviousYears and NextYears those over the years,
	// which turn by ten.
	ChooseMonth, ChooseYear  string
	PreviousYear, NextYear   string
	PreviousYears, NextYears string
	// Marked follows the date of a day that has marks (Calendar.Marks) for a
	// screen reader: "Monday, November 9, 2026, has events".
	Marked string

	// TimePicker

	// AM and PM mark the halves of a day on a twelve-hour clock.
	AM, PM string
	// Hour24 says the locale writes the hour from 0 to 23. A TimePicker of
	// HourCycleLocale follows it.
	Hour24 bool
	// PeriodFirst puts AM and PM before the hour, as Japanese does (午後 3:00).
	PeriodFirst bool
	// Hour, Minute, Second and DayPeriod name the parts of a time field to a
	// screen reader.
	Hour, Minute, Second, DayPeriod string

	// Shared

	// Close, Search and NoResults serve several widgets. Required is the error
	// of a form field left empty, RequiredChoice that of a choice left
	// unmade, such as a dropdown or a date, and RequiredCheck that of a box
	// left unchecked. Remove names the remove button of a chip (see
	// RemoveNamed), and More the button of a toolbar that opens the items
	// that do not fit.
	Close, Search, NoResults, Required string
	RequiredChoice, RequiredCheck      string
	// Minimize, Maximize and Restore name the buttons of the title bar that
	// gowaku draws, next to Close.
	Minimize, Maximize, Restore string
	Remove, More                string

	// SplitPane, PropertyGrid, Breadcrumb

	// PaneDivider names the divider of a SplitPane, and FilterProperties the
	// search field of a PropertyGrid.
	PaneDivider, FilterProperties string
	// Breadcrumb names a breadcrumb trail, CurrentLocation describes its last
	// segment, and HiddenPath names the … button of the segments that do not
	// fit.
	Breadcrumb, CurrentLocation, HiddenPath string
	// Steps names the list of the steps of a nav.Steps, and StepCompleted
	// and CurrentStep describe a step that is done and the one under way.
	Steps, StepCompleted, CurrentStep string

	// Tree

	// Loading is the row a tree shows under a node whose children are still
	// coming.
	Loading string

	// TabBar

	// NewTab names the button after the tabs that adds one, and AllTabs the
	// one that lists them when they do not fit.
	NewTab, AllTabs string
	// Modified is the value a tab of a TabBar gives assistive technology
	// while what it shows has changes that are not saved.
	Modified string

	// Editor

	// Find names the find bar and its query, ReplaceWith the field of the
	// replacement, and the rest its buttons. BadPattern says a regular
	// expression does not compile, and Searching that a search of a large
	// text is still under way.
	Find, ReplaceWith, Replace, ReplaceAll  string
	MatchCase, WholeWord, RegularExpression string
	PreviousMatch, NextMatch, BadPattern    string
	Searching                               string

	// Table

	// Filter names the filter button of a column header, and the field of
	// its filter. AllValues ticks every value of the filter. How a column is
	// sorted needs no words: the header says it as semantics.Sort, which the
	// screen reader puts in its own words.
	Filter, AllValues string

	// Formats. Each one, when set, replaces the English pattern of the method
	// of the same name without the prefix.

	// FormatMonthYear spells a month of a year: "November 2026".
	FormatMonthYear func(m time.Month, year int) string
	// FormatYear spells a year: "2026".
	FormatYear func(year int) string
	// FormatDate spells a date in a date field: "Nov 9, 2026".
	FormatDate func(t time.Time) string
	// FormatShortDate spells a date without its year, with its weekday:
	// "Mon, Nov 9".
	FormatShortDate func(t time.Time) string
	// FormatMonthDay spells a day of a month without the year: "Nov 9".
	FormatMonthDay func(t time.Time) string
	// FormatSpokenDate spells a date in full with its weekday, for a screen
	// reader: "Monday, November 9, 2026".
	FormatSpokenDate func(t time.Time) string
	// FormatTime spells a time of day: "3:04 PM", or "15:04" where the
	// locale counts the hours to 23 (Hour24).
	FormatTime func(t time.Time) string
	// FormatMatchCount says which match of a search is selected, k from 1, of
	// n: "3 of 12"; k of 0 gives the count alone: "12 found".
	FormatMatchCount func(k, n int) string
	// FormatRemoveNamed names the button that removes the item label:
	// "Remove Draft".
	FormatRemoveNamed func(label string) string
	// FormatCloseNamed names the button that closes the item title:
	// "Close Notes".
	FormatCloseNamed func(title string) string
	// FormatMovedTo tells a screen reader that the item label of a
	// ReorderableList moved to place k, from 1, of n: "Moved Milk to
	// position 2 of 5". An empty label leaves the name out.
	FormatMovedTo func(label string, k, n int) string

	// Navigator
	// Back names the button that goes back to the previous page, and
	// PageNotFound heads the page of a route name that nothing matches.
	Back, PageNotFound string

	// NumberField, RangeSlider

	// InvalidNumber is the error of a number field whose text is no number.
	// RangeMinimum and RangeMaximum name the low and the high thumb of a
	// range slider.
	InvalidNumber, RangeMinimum, RangeMaximum string

	// SelectionArea

	// Copy and SelectAll are the items of the menu of a selection area.
	// Copied is what a Copy button says for a moment after it copied.
	Copy, Copied, SelectAll string

	// ColorPicker

	// ChooseColor names a color picker and the popup that holds one.
	// ColorPlane names its square of saturation and brightness, and Hue,
	// Saturation, Brightness and Opacity its strips and what they set.
	// HexCode names its field of the hex code, Red, Green and Blue its
	// number fields, and OriginalColor the half of its preview that goes back
	// to the color it started with.
	ChooseColor, ColorPlane                  string
	Hue, Saturation, Brightness, Opacity     string
	HexCode, Red, Green, Blue, OriginalColor string

	// Sparkline

	// Minimum, Maximum and Latest head the numbers of the summary a screen
	// reader hears for a sparkline without a label of its own.
	Minimum, Maximum, Latest string

	// InfoBar, StatusBar, CommandPalette

	// Information, Success, Warning and Error name the severity of an
	// InfoBar to a screen reader, before its title and message.
	Information, Success, Warning, Error string
	// StatusBar names a status bar.
	StatusBar string
	// CommandPalette names the command palette and the action that opens it,
	// and SearchCommands is the placeholder of its search field.
	CommandPalette, SearchCommands string

	// DockLayout

	// AutoHide names the pin button of a DockLayout, checked while the panel
	// is hidden at an edge, and HiddenPanels the strips of the hidden panels.
	AutoHide, HiddenPanels string
}

// EnglishStrings returns a copy of the bundled English table.
func EnglishStrings() Strings { return englishTable }

// JapaneseStrings returns a copy of the bundled Japanese table.
func JapaneseStrings() Strings { return japaneseTable }

// MonthYear spells a month of a year, as a calendar shows it above its days.
func (s *Strings) MonthYear(m time.Month, year int) string {
	if s.FormatMonthYear != nil {
		return s.FormatMonthYear(m, year)
	}
	return s.Months[m-1] + " " + kit.Itoa(year)
}

// Year spells a year, as a calendar shows it above its months and in its grid
// of years.
func (s *Strings) Year(year int) string {
	if s.FormatYear != nil {
		return s.FormatYear(year)
	}
	return kit.Itoa(year)
}

// Years spells the years from first to last, as a calendar shows them above
// its grid of years: "2020–2029".
func (s *Strings) Years(first, last int) string {
	return s.Year(first) + "–" + s.Year(last)
}

// Date spells a whole date, as a date field shows it.
func (s *Strings) Date(t time.Time) string {
	if s.FormatDate != nil {
		return s.FormatDate(t)
	}
	return s.ShortMonths[t.Month()-1] + " " + kit.Itoa(t.Day()) + ", " + kit.Itoa(t.Year())
}

// ShortDate spells a date of this year or the next few days, with its short
// weekday and without the year, as the head of a day in an agenda or a
// forecast: "Mon, Nov 16" in English, 11月16日（月） in Japanese.
func (s *Strings) ShortDate(t time.Time) string {
	if s.FormatShortDate != nil {
		return s.FormatShortDate(t)
	}
	return s.ShortWeekdays[t.Weekday()] + ", " + s.MonthDay(t)
}

// MonthDay spells a day of a month without the year or the weekday: "Nov 16"
// in English, 11月16日 in Japanese.
func (s *Strings) MonthDay(t time.Time) string {
	if s.FormatMonthDay != nil {
		return s.FormatMonthDay(t)
	}
	return s.ShortMonths[t.Month()-1] + " " + kit.Itoa(t.Day())
}

// SpokenDate spells a date in full with its weekday, as a screen reader says a
// day of a calendar.
func (s *Strings) SpokenDate(t time.Time) string {
	if s.FormatSpokenDate != nil {
		return s.FormatSpokenDate(t)
	}
	return s.Weekdays[t.Weekday()] + ", " + s.Months[t.Month()-1] + " " + kit.Itoa(t.Day()) + ", " + kit.Itoa(t.Year())
}

// Time spells the time of day of t, to the minute: "3:04 PM" in English,
// 15:04 in Japanese. A table without FormatTime writes the hour from 0 to 23
// if it is Hour24, and else from 1 to 12 with AM or PM, before it if it is
// PeriodFirst.
func (s *Strings) Time(t time.Time) string {
	if s.FormatTime != nil {
		return s.FormatTime(t)
	}
	if s.Hour24 {
		return kit.Itoa(t.Hour()) + ":" + kit.Pad2(t.Minute())
	}
	hour, period := t.Hour()%12, s.AM
	if t.Hour() >= 12 {
		period = s.PM
	}
	if hour == 0 {
		hour = 12
	}
	clock := kit.Itoa(hour) + ":" + kit.Pad2(t.Minute())
	if s.PeriodFirst {
		return period + " " + clock
	}
	return clock + " " + period
}

// MessageDate spells when t happened as a list of messages or files shows it,
// seen from now: the time for a moment of today ("3:04 PM", 15:04), the word
// Yesterday for the day before ("Yesterday", 昨日), the weekday for one of
// the five days before that ("Monday", 月曜日), the day of the
// month for an older one of this year ("Nov 9", 11月9日), and the whole date
// for one of another year ("Nov 9, 2025", 2025/11/09). A time after today
// is written as a date. Days are counted in the zone of now.
//
// now comes from the caller, usually time.Now(), so that a test can fix it.
func (s *Strings) MessageDate(t, now time.Time) string {
	t = t.In(now.Location())
	day := func(x time.Time) time.Time { return time.Date(x.Year(), x.Month(), x.Day(), 0, 0, 0, 0, time.UTC) }
	today, then := day(now), day(t)
	switch days := int(today.Sub(then).Hours() / 24); {
	case days == 0:
		return s.Time(t)
	case days == 1:
		return s.Yesterday
	case days > 1 && days < 7:
		return s.Weekdays[t.Weekday()]
	case t.Year() == now.Year():
		return s.MonthDay(t)
	}
	return s.Date(t)
}

// MatchCount says which match of a search is selected, k from 1, of n; k of 0
// gives the count alone.
func (s *Strings) MatchCount(k, n int) string {
	switch {
	case s.FormatMatchCount != nil:
		return s.FormatMatchCount(k, n)
	case k > 0:
		return kit.Itoa(k) + " of " + kit.Itoa(n)
	}
	return kit.Itoa(n) + " found"
}

// RemoveNamed names the button that removes the item label, such as the
// remove button of a chip.
func (s *Strings) RemoveNamed(label string) string {
	if s.FormatRemoveNamed != nil {
		return s.FormatRemoveNamed(label)
	}
	return s.Remove + " " + label
}

// CloseNamed names the button that closes the item title, such as the close
// button of a tab.
func (s *Strings) CloseNamed(title string) string {
	if s.FormatCloseNamed != nil {
		return s.FormatCloseNamed(title)
	}
	return s.Close + " " + title
}

// MovedTo tells that the item label moved to place k, from 1, of n, as a
// ReorderableList announces a move. An empty label leaves the name out.
func (s *Strings) MovedTo(label string, k, n int) string {
	if s.FormatMovedTo != nil {
		return s.FormatMovedTo(label, k, n)
	}
	if label == "" {
		return "Moved to position " + kit.Itoa(k) + " of " + kit.Itoa(n)
	}
	return "Moved " + label + " to position " + kit.Itoa(k) + " of " + kit.Itoa(n)
}

// fillFrom gives every word s leaves out the word of e. Formats stay as they
// are: a nil one already writes the English order with the words of s.
func (s *Strings) fillFrom(e *Strings) {
	months := func(dst *[12]string, src [12]string) {
		if dst[0] == "" {
			*dst = src
		}
	}
	days := func(dst *[7]string, src [7]string) {
		if dst[0] == "" {
			*dst = src
		}
	}
	word := func(dst *string, src string) {
		if *dst == "" {
			*dst = src
		}
	}
	months(&s.Months, e.Months)
	months(&s.ShortMonths, e.ShortMonths)
	days(&s.Weekdays, e.Weekdays)
	days(&s.ShortWeekdays, e.ShortWeekdays)
	days(&s.NarrowWeekdays, e.NarrowWeekdays)
	for _, p := range [...]struct {
		dst *string
		src string
	}{
		{&s.Today, e.Today}, {&s.Yesterday, e.Yesterday}, {&s.Clear, e.Clear}, {&s.ChooseDate, e.ChooseDate},
		{&s.PreviousMonth, e.PreviousMonth}, {&s.NextMonth, e.NextMonth},
		{&s.ChooseMonth, e.ChooseMonth}, {&s.ChooseYear, e.ChooseYear},
		{&s.PreviousYear, e.PreviousYear}, {&s.NextYear, e.NextYear},
		{&s.PreviousYears, e.PreviousYears}, {&s.NextYears, e.NextYears},
		{&s.AM, e.AM}, {&s.PM, e.PM},
		{&s.Hour, e.Hour}, {&s.Minute, e.Minute}, {&s.Second, e.Second}, {&s.DayPeriod, e.DayPeriod},
		{&s.Close, e.Close}, {&s.Search, e.Search}, {&s.NoResults, e.NoResults}, {&s.Required, e.Required},
		{&s.RequiredChoice, e.RequiredChoice}, {&s.RequiredCheck, e.RequiredCheck},
		{&s.Minimize, e.Minimize}, {&s.Maximize, e.Maximize}, {&s.Restore, e.Restore},
		{&s.Remove, e.Remove}, {&s.More, e.More},
		{&s.PaneDivider, e.PaneDivider}, {&s.FilterProperties, e.FilterProperties},
		{&s.Breadcrumb, e.Breadcrumb}, {&s.CurrentLocation, e.CurrentLocation}, {&s.HiddenPath, e.HiddenPath},
		{&s.Steps, e.Steps}, {&s.StepCompleted, e.StepCompleted}, {&s.CurrentStep, e.CurrentStep},
		{&s.Loading, e.Loading},
		{&s.NewTab, e.NewTab}, {&s.AllTabs, e.AllTabs}, {&s.Modified, e.Modified},
		{&s.Find, e.Find}, {&s.ReplaceWith, e.ReplaceWith}, {&s.Replace, e.Replace}, {&s.ReplaceAll, e.ReplaceAll},
		{&s.MatchCase, e.MatchCase}, {&s.WholeWord, e.WholeWord}, {&s.RegularExpression, e.RegularExpression},
		{&s.PreviousMatch, e.PreviousMatch}, {&s.NextMatch, e.NextMatch}, {&s.BadPattern, e.BadPattern},
		{&s.Searching, e.Searching},
		{&s.Filter, e.Filter}, {&s.AllValues, e.AllValues},
		{&s.Back, e.Back}, {&s.PageNotFound, e.PageNotFound},
		{&s.Marked, e.Marked},
		{&s.InvalidNumber, e.InvalidNumber}, {&s.RangeMinimum, e.RangeMinimum}, {&s.RangeMaximum, e.RangeMaximum},
		{&s.Copy, e.Copy}, {&s.Copied, e.Copied}, {&s.SelectAll, e.SelectAll},
		{&s.ChooseColor, e.ChooseColor}, {&s.ColorPlane, e.ColorPlane},
		{&s.Hue, e.Hue}, {&s.Saturation, e.Saturation}, {&s.Brightness, e.Brightness}, {&s.Opacity, e.Opacity},
		{&s.HexCode, e.HexCode}, {&s.Red, e.Red}, {&s.Green, e.Green}, {&s.Blue, e.Blue},
		{&s.OriginalColor, e.OriginalColor},
		{&s.Minimum, e.Minimum}, {&s.Maximum, e.Maximum}, {&s.Latest, e.Latest},
		{&s.Information, e.Information}, {&s.Success, e.Success}, {&s.Warning, e.Warning}, {&s.Error, e.Error},
		{&s.StatusBar, e.StatusBar}, {&s.CommandPalette, e.CommandPalette}, {&s.SearchCommands, e.SearchCommands},
		{&s.AutoHide, e.AutoHide}, {&s.HiddenPanels, e.HiddenPanels},
	} {
		word(p.dst, p.src)
	}
}
