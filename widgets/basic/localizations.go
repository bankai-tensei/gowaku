package basic

import (
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/number"

	"github.com/bankai-tensei/gowaku/core"
)

// Localizations gives the widgets below it the Strings of their locale, the
// Locale of the nearest MediaQuery. It chooses among the bundled English and
// Japanese tables and Strings with the matcher of x/text/language, so ja-JP
// gets Japanese and en-GB English; a locale that matches no table gets
// English. A table of Strings for the same language as a bundled one, or as
// an earlier one, takes its place.
//
// Run puts one at the root of every window with App.Strings, so an
// application needs its own only to give a part of the tree other tables. A
// Localizations below another replaces it: give it every table it needs.
//
// The choice is made once per locale, so StringsOf costs a lookup. A
// MediaQuery below that names another locale still gets its words from these
// tables, at the cost of a match each time.
type Localizations struct {
	core.StatefulBase
	// Strings are the tables the application adds. See Strings for the words
	// a table may leave out. Give a new slice to change them; the tables are
	// compared by pointer.
	Strings []*Strings
	Child   core.Widget
}

// Localized is what a Localizations gives the tree: the locale, its table and
// a printer for its numbers, and the tables to choose from for a MediaQuery
// below that names another locale. An overlay hands it on to what it shows
// with core.Inherited, as CarryLocalizations does.
type Localized struct {
	locale  language.Tag
	strings *Strings
	printer *message.Printer
	choice  *tableChoice
}

// tableChoice is the tables of a Localizations, every word filled in, and the
// matcher over their languages.
type tableChoice struct {
	tables  []*Strings
	matcher language.Matcher
}

// stringsFor returns the table of locale.
func (l *Localized) stringsFor(locale language.Tag) *Strings {
	if locale == l.locale {
		return l.strings
	}
	return l.choice.match(locale)
}

type localizationsState struct {
	core.StateOf[Localizations]
	// given are the Strings that choice was made from, and current what the
	// tree sees.
	given   []*Strings
	choice  *tableChoice
	current *Localized
}

// CreateState implements core.StatefulWidget.
func (Localizations) CreateState() core.State { return &localizationsState{} }

// Build implements core.State.
func (s *localizationsState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	locale := LocaleOf(ctx)
	fresh := s.choice == nil || !sameTables(s.given, w.Strings)
	if fresh {
		s.given = w.Strings
		s.choice = newTableChoice(w.Strings)
	}
	if fresh || s.current.locale != locale {
		s.current = &Localized{locale: locale, strings: s.choice.match(locale), printer: printerFor(locale), choice: s.choice}
	}
	return core.Inherited[*Localized]{Value: s.current, Child: w.Child}
}

func sameTables(a, b []*Strings) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// newTableChoice puts extra after the bundled tables, each in place of a table
// of the same language, and fills in the words they leave out.
func newTableChoice(extra []*Strings) *tableChoice {
	tables := []*Strings{&englishTable, &japaneseTable}
	for _, t := range extra {
		if t == nil {
			continue
		}
		filled := *t
		filled.fillFrom(&englishTable)
		if i := indexOfLanguage(tables, t.Language); i >= 0 {
			tables[i] = &filled
		} else {
			tables = append(tables, &filled)
		}
	}
	tags := make([]language.Tag, len(tables))
	for i, t := range tables {
		tags[i] = t.Language
	}
	return &tableChoice{tables: tables, matcher: language.NewMatcher(tags)}
}

// match returns the table of locale. English comes first, so it is what a
// locale without a match gets.
func (c *tableChoice) match(locale language.Tag) *Strings {
	_, i, confidence := c.matcher.Match(locale)
	if confidence == language.No {
		i = 0
	}
	return c.tables[i]
}

// indexOfLanguage returns the index of the table of tag, or -1.
func indexOfLanguage(tables []*Strings, tag language.Tag) int {
	for i, t := range tables {
		if t.Language == tag {
			return i
		}
	}
	return -1
}

// printerFor returns a printer of the numbers of locale; the root locale
// prints as English.
func printerFor(locale language.Tag) *message.Printer {
	if locale == language.Und {
		locale = language.English
	}
	return message.NewPrinter(locale)
}

// StringsOf returns the words for the locale of ctx, and rebuilds ctx when they
// change. Without a Localizations above, it reads the bundled tables only.
func StringsOf(ctx core.BuildContext) *Strings {
	locale := LocaleOf(ctx)
	if l, ok := core.DependOn[*Localized](ctx); ok && l != nil {
		return l.stringsFor(locale)
	}
	return stringsFor(locale)
}

// LookupStrings is StringsOf without the dependency, for callbacks, which run
// outside a Build.
func LookupStrings(ctx core.BuildContext) *Strings {
	md, _ := core.Lookup[MediaQueryData](ctx)
	if l, ok := core.Lookup[*Localized](ctx); ok && l != nil {
		return l.stringsFor(md.Locale)
	}
	return stringsFor(md.Locale)
}

// CarryLocalizations gives child, which goes into an overlay, the
// Localizations above ctx: the overlay may sit above it.
func CarryLocalizations(ctx core.BuildContext, child core.Widget) core.Widget {
	if l, ok := core.Lookup[*Localized](ctx); ok && l != nil {
		return core.Inherited[*Localized]{Value: l, Child: child}
	}
	return child
}

// stringsFor chooses a bundled table without a matcher, so that a tree without
// a Localizations does not allocate.
func stringsFor(tag language.Tag) *Strings {
	ja, _ := language.Japanese.Base()
	if base, _ := tag.Base(); base == ja {
		return &japaneseTable
	}
	return &englishTable
}

// FormatInt writes v with the digits, signs and grouping of the locale of ctx,
// as golang.org/x/text/number has them from CLDR: 1,234,567 in English,
// 1.234.567 in German. It rebuilds ctx when the locale changes.
func FormatInt(ctx core.BuildContext, v int) string {
	return printerOf(ctx).Sprint(number.Decimal(v))
}

// FormatDecimal writes v as FormatInt does, with exactly digits digits after
// the decimal separator, rounded half to even.
func FormatDecimal(ctx core.BuildContext, v float64, digits int) string {
	return printerOf(ctx).Sprint(number.Decimal(v, number.Scale(digits)))
}

// printerOf returns the printer of the Localizations above ctx, or a new one
// when there is none or a MediaQuery below it names another locale.
func printerOf(ctx core.BuildContext) *message.Printer {
	locale := LocaleOf(ctx)
	if l, ok := core.DependOn[*Localized](ctx); ok && l != nil && l.locale == locale {
		return l.printer
	}
	return printerFor(locale)
}
