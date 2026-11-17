package golden

import (
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/field"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
)

// pickerScenes returns the scenes of the picker widgets. They use a
// fixed day, never the clock. The bundled font has no Japanese glyphs, so the
// Japanese scene shows what is written in digits there: the 24 hour clock and
// the numeric date.
func pickerScenes() []Scene {
	return []Scene{
		{Name: "calendar", Width: 828, Height: 471, Scale: 1.5, Root: calendars},
		{Name: "calendar-dark", Width: 828, Height: 471, Scale: 1.5, Dark: true, Root: calendars},
		{Name: "date-picker-open", Width: 450, Height: 600, Scale: 1.5, Root: datePickerPage, Act: openDatePicker},
		{Name: "date-picker-open-dark", Width: 450, Height: 600, Scale: 1.5, Dark: true, Root: datePickerPage, Act: openDatePicker},
		{Name: "time-pickers-ja", Width: 450, Height: 255, Scale: 1.5, Root: timePickersJapanese},
		{Name: "time-pickers-12h", Width: 450, Height: 255, Scale: 1.5, Root: timePickersEnglish, Act: focusMinute},
		{Name: "time-pickers-12h-dark", Width: 450, Height: 255, Scale: 1.5, Dark: true, Root: timePickersEnglish, Act: focusMinute},
	}
}

// pickerDate returns a day of 2026; the scenes take November 9 as today.
func pickerDate(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC) }

// localized puts child in a window of the locale tag.
func localized(tag language.Tag, child core.Widget) core.Widget {
	return basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1.5, Locale: tag}, Child: child}
}

// pickerCard is the card a calendar sits on in an application.
type pickerCard struct {
	core.StatelessBase
	child core.Widget
}

// Build implements core.StatelessWidget.
func (p pickerCard) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Container{
		Color:   th.Color.Card,
		Radius:  th.Radius.Group,
		Padding: geom.All(12),
		Child:   p.child,
	}
}

// calendars shows a calendar with a chosen day and today, and one that starts
// its weeks on Monday and allows only some days.
func calendars() core.Widget {
	nothing := func(time.Time) {}
	weekdays := func(d time.Time) bool { return d.Weekday() != time.Saturday && d.Weekday() != time.Sunday }
	return basic.Padding{Insets: geom.All(16), Child: basic.Row{
		Size: layout.MainMin, Cross: layout.CrossStart, Spacing: 16, Children: []core.Widget{
			localized(language.English, pickerCard{child: field.Calendar{
				Value: pickerDate(11, 18), Today: pickerDate(11, 9), OnChange: nothing,
			}}),
			localized(language.BritishEnglish, pickerCard{child: field.Calendar{
				Value: pickerDate(11, 9), Today: pickerDate(11, 9), Min: pickerDate(11, 4), Max: pickerDate(11, 25),
				Selectable: weekdays, OnChange: nothing,
			}}),
		},
	}}
}

// datePickerPage is a date field at the top left of a window.
func datePickerPage() core.Widget {
	return localized(language.English, basic.Padding{Insets: geom.All(16), Child: basic.Align{
		Alignment: layout.AlignTopLeft,
		Child: field.DatePicker{
			Label: "Due date", Value: pickerDate(11, 18), Today: pickerDate(11, 9),
			ShowToday: true, ShowClear: true, OnChange: func(time.Time) {},
		},
	}})
}

// openDatePicker clicks the date field.
func openDatePicker(tree *core.Tree, _ core.BuildContext) {
	clickRole(tree, semantics.RoleComboBox, "")
}

// clickRole clicks the first node of role r, with label unless it is empty.
func clickRole(tree *core.Tree, r semantics.Role, label string) {
	var at geom.Rect
	tree.Semantics().Walk(func(n *semantics.Node) bool {
		if n.Role == r && (label == "" || n.Label == label) {
			at = n.Rect
			return false
		}
		return true
	})
	pos := geom.Offset{X: at.X + at.W/2, Y: at.Y + at.H/2}
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: pos})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos})
}

func hm(h, m, s int) time.Duration {
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(s)*time.Second
}

// timePickersJapanese shows the time fields of a Japanese window, which count
// 24 hours, and a date field, which writes the date in digits.
func timePickersJapanese() core.Widget {
	nothing := func(time.Duration) {}
	return localized(language.Japanese, basic.Padding{Insets: geom.All(16), Child: basic.Column{
		Size: layout.MainMin, Cross: layout.CrossStart, Spacing: 12, Children: []core.Widget{
			field.TimePicker{Value: hm(9, 30, 0), OnChange: nothing},
			field.TimePicker{Value: hm(21, 5, 42), Seconds: true, OnChange: nothing},
			field.DatePicker{Value: pickerDate(11, 9), OnChange: func(time.Time) {}},
		},
	}})
}

// timePickersEnglish shows the time fields of an English window, which count
// 12 hours, one of them disabled.
func timePickersEnglish() core.Widget {
	nothing := func(time.Duration) {}
	return localized(language.English, basic.Padding{Insets: geom.All(16), Child: basic.Column{
		Size: layout.MainMin, Cross: layout.CrossStart, Spacing: 12, Children: []core.Widget{
			field.TimePicker{Label: "Start", Value: hm(21, 30, 0), MinuteStep: 15, OnChange: nothing},
			basic.Row{Size: layout.MainMin, Spacing: 12, Children: []core.Widget{
				field.TimePicker{Value: hm(0, 5, 42), Seconds: true, OnChange: nothing},
				field.TimePicker{Value: hm(12, 0, 0)},
			}},
		},
	}})
}

// focusMinute clicks the minutes of the first time field.
func focusMinute(tree *core.Tree, _ core.BuildContext) {
	clickRole(tree, semantics.RoleStepper, "Minute")
}
