package golden

import (
	"strconv"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
)

// localeScenes returns the scenes of localization: the same small form in
// English and in German, a language the application adds with a table of its
// own. The bundled font has no Japanese glyphs; time-pickers-ja shows what
// Japanese writes in digits.
func localeScenes() []Scene {
	return []Scene{
		{Name: "locale-en", Width: 900, Height: 471, Scale: 1.5, Root: localeFormEnglish},
		{Name: "locale-de", Width: 900, Height: 471, Scale: 1.5, Root: localeFormGerman},
		{Name: "locale-de-dark", Width: 900, Height: 471, Scale: 1.5, Dark: true, Root: localeFormGerman},
		{Name: "locale-de-date-picker-open", Width: 450, Height: 600, Scale: 1.5, Root: localeDatePickerGerman, Act: openDatePicker},
	}
}

// germanStrings is a table an application could add for German. It leaves out
// the words of the widgets it does not use, which then read as English, and
// writes dates the German way.
func germanStrings() *basic.Strings {
	s := &basic.Strings{
		Language: language.German,
		Months: [12]string{"Januar", "Februar", "März", "April", "Mai", "Juni",
			"Juli", "August", "September", "Oktober", "November", "Dezember"},
		ShortMonths:    [12]string{"Jan.", "Feb.", "März", "Apr.", "Mai", "Juni", "Juli", "Aug.", "Sept.", "Okt.", "Nov.", "Dez."},
		Weekdays:       [7]string{"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"},
		ShortWeekdays:  [7]string{"So.", "Mo.", "Di.", "Mi.", "Do.", "Fr.", "Sa."},
		NarrowWeekdays: [7]string{"So", "Mo", "Di", "Mi", "Do", "Fr", "Sa"},
		Today:          "Heute", Clear: "Löschen", ChooseDate: "Datum auswählen",
		PreviousMonth: "Vorheriger Monat", NextMonth: "Nächster Monat",
		Hour24: true, Hour: "Stunde", Minute: "Minute", Second: "Sekunde",
		Close: "Schließen", Search: "Suchen", NoResults: "Keine Treffer", Required: "Pflichtfeld",
		Remove: "Entfernen",
	}
	s.FormatDate = func(t time.Time) string {
		return strconv.Itoa(t.Day()) + ". " + s.ShortMonths[t.Month()-1] + " " + strconv.Itoa(t.Year())
	}
	s.FormatSpokenDate = func(t time.Time) string {
		return s.Weekdays[t.Weekday()] + ", " + strconv.Itoa(t.Day()) + ". " + s.Months[t.Month()-1] + " " + strconv.Itoa(t.Year())
	}
	s.FormatRemoveNamed = func(label string) string { return label + " entfernen" }
	return s
}

// withStrings puts child in a window of locale, below a Localizations that
// knows extra.
func withStrings(locale language.Tag, extra []*basic.Strings, child core.Widget) core.Widget {
	return localized(locale, basic.Localizations{Strings: extra, Child: child})
}

func localeFormEnglish() core.Widget {
	return withStrings(language.AmericanEnglish, nil, localeForm())
}

func localeFormGerman() core.Widget {
	return withStrings(language.MustParse("de-DE"), []*basic.Strings{germanStrings()}, localeForm())
}

// localeForm shows what the locale changes: the month, the weekdays and the
// first day of a calendar, a date, the hour cycle, a grouped number and the
// placeholder of a search field.
func localeForm() core.Widget {
	return basic.Padding{Insets: geom.All(16), Child: basic.Row{
		Size: layout.MainMin, Cross: layout.CrossStart, Spacing: 16, Children: []core.Widget{
			pickerCard{child: field.Calendar{Value: pickerDate(11, 18), Today: pickerDate(11, 9), OnChange: func(time.Time) {}}},
			basic.SizedBox{Width: 260, Child: basic.Column{
				Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: 12, Children: []core.Widget{
					field.DatePicker{Value: pickerDate(11, 9), OnChange: func(time.Time) {}},
					field.TimePicker{Value: hm(21, 5, 0), OnChange: func(time.Duration) {}},
					basic.Align{Alignment: layout.AlignTopLeft, Child: control.NumberStepper{
						Value: 12345, Max: 99999, Step: 100, OnChange: func(int) {},
					}},
					field.Search{OnChange: func(string) {}},
				},
			}},
		},
	}}
}

// localeDatePickerGerman is a German date field whose calendar opens in the
// overlay, above the Localizations of the field.
func localeDatePickerGerman() core.Widget {
	return withStrings(language.MustParse("de-DE"), []*basic.Strings{germanStrings()}, basic.Padding{
		Insets: geom.All(16), Child: basic.Align{
			Alignment: layout.AlignTopLeft,
			Child: field.DatePicker{
				Value: pickerDate(11, 18), Today: pickerDate(11, 9),
				ShowToday: true, ShowClear: true, OnChange: func(time.Time) {},
			},
		},
	})
}
