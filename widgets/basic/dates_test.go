package basic

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/text/language"
)

func TestStrings_ShortDateAndMonthDay(t *testing.T) {
	// 2026-11-16 is a Monday.
	day := time.Date(2026, time.November, 16, 9, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name                       string
		s                          *Strings
		short, monthDay            string
		full, shortName, narrowDay string
	}{
		{"en", &englishTable, "Mon, Nov 16", "Nov 16", "Monday", "Mon", "Mo"},
		{"ja", &japaneseTable, "11月16日（月）", "11月16日", "月曜日", "月", "月"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.s.ShortDate(day); got != tc.short {
				t.Errorf("ShortDate = %q, want %q", got, tc.short)
			}
			if got := tc.s.MonthDay(day); got != tc.monthDay {
				t.Errorf("MonthDay = %q, want %q", got, tc.monthDay)
			}
			wd := day.Weekday()
			if tc.s.Weekdays[wd] != tc.full || tc.s.ShortWeekdays[wd] != tc.shortName || tc.s.NarrowWeekdays[wd] != tc.narrowDay {
				t.Errorf("Monday is %q, %q, %q; want %q, %q, %q", tc.s.Weekdays[wd], tc.s.ShortWeekdays[wd],
					tc.s.NarrowWeekdays[wd], tc.full, tc.shortName, tc.narrowDay)
			}
		})
	}
}

// TestStrings_JapaneseDatesHaveNoSpaces: the typesetting puts the room between
// the digits and the kanji (JLReq), so the strings carry none.
func TestStrings_JapaneseDatesHaveNoSpaces(t *testing.T) {
	for d := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC); d.Year() == 2026; d = d.AddDate(0, 0, 13) {
		for _, s := range []string{japaneseTable.ShortDate(d), japaneseTable.MonthDay(d)} {
			if strings.ContainsAny(s, " 　") {
				t.Errorf("%v: %q has a space", d.Format(time.DateOnly), s)
			}
		}
	}
}

// TestStrings_ShortDateOfATableWithoutFormats: a table that gives only words
// writes the English order with them.
func TestStrings_ShortDateOfATableWithoutFormats(t *testing.T) {
	s := Strings{Language: language.French}
	s.ShortMonths[10] = "nov."
	s.ShortWeekdays[1] = "lun."
	day := time.Date(2026, time.November, 16, 0, 0, 0, 0, time.UTC)
	if got, want := s.ShortDate(day), "lun., nov. 16"; got != want {
		t.Errorf("ShortDate = %q, want %q", got, want)
	}
}

func TestWeekStartOf_FromRegionOrLikelyRegion(t *testing.T) {
	for _, tc := range []struct {
		locale string
		want   time.Weekday
	}{
		{"en", time.Sunday}, {"en-US", time.Sunday}, {"en-GB", time.Monday},
		{"ja", time.Sunday}, {"de", time.Monday}, {"fr-CA", time.Sunday},
		{"ar-EG", time.Saturday}, {"und", time.Sunday},
	} {
		if got := WeekStartOf(language.MustParse(tc.locale)); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.locale, got, tc.want)
		}
	}
}
