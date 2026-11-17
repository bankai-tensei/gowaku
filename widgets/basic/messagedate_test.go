package basic

import (
	"strings"
	"testing"
	"time"
)

func TestStrings_MessageDate(t *testing.T) {
	// Now is Wednesday, 2026-11-18, 14:30.
	now := time.Date(2026, time.November, 18, 14, 30, 0, 0, time.UTC)
	at := func(month time.Month, day, hour, minute int) time.Time {
		return time.Date(2026, month, day, hour, minute, 0, 0, time.UTC)
	}
	cases := []struct {
		name   string
		t      time.Time
		en, ja string
	}{
		{"this morning", at(time.November, 18, 9, 5), "9:05 AM", "9:05"},
		{"this afternoon", at(time.November, 18, 15, 4), "3:04 PM", "15:04"},
		{"midnight", at(time.November, 18, 0, 0), "12:00 AM", "0:00"},
		{"noon", at(time.November, 18, 12, 0), "12:00 PM", "12:00"},
		{"yesterday", at(time.November, 17, 23, 59), "Yesterday", "昨日"},
		{"two days ago", at(time.November, 16, 8, 0), "Monday", "月曜日"},
		{"six days ago", at(time.November, 12, 8, 0), "Thursday", "木曜日"},
		{"seven days ago", at(time.November, 11, 8, 0), "Nov 11", "11月11日"},
		{"this year", at(time.February, 3, 8, 0), "Feb 3", "2月3日"},
		{"last year", time.Date(2025, time.December, 31, 22, 0, 0, 0, time.UTC), "Dec 31, 2025", "2025/12/31"},
		{"tomorrow", at(time.November, 19, 8, 0), "Nov 19", "11月19日"},
	}
	for _, tc := range cases {
		if got := englishTable.MessageDate(tc.t, now); got != tc.en {
			t.Errorf("en %s: got %q, want %q", tc.name, got, tc.en)
		}
		got := japaneseTable.MessageDate(tc.t, now)
		if got != tc.ja {
			t.Errorf("ja %s: got %q, want %q", tc.name, got, tc.ja)
		}
		if strings.ContainsAny(got, " 　") {
			t.Errorf("ja %s: %q has a space", tc.name, got)
		}
	}
}

// TestStrings_MessageDateCountsDaysInTheZoneOfNow: 23:30 of the day before in
// Tokyo is the same day as now in UTC, and the day that counts is Tokyo's.
func TestStrings_MessageDateCountsDaysInTheZoneOfNow(t *testing.T) {
	tokyo := time.FixedZone("JST", 9*60*60)
	now := time.Date(2026, time.November, 18, 8, 0, 0, 0, tokyo)
	then := time.Date(2026, time.November, 17, 14, 30, 0, 0, time.UTC) // 23:30 in Tokyo
	if got := englishTable.MessageDate(then, now); got != "Yesterday" {
		t.Errorf("got %q, want Yesterday", got)
	}
	if got := englishTable.MessageDate(now.Add(-time.Hour).UTC(), now); got != "7:00 AM" {
		t.Errorf("got %q, want 7:00 AM in the zone of now", got)
	}
}

func TestStrings_TimeOfATableWithoutFormats(t *testing.T) {
	afternoon := time.Date(2026, time.November, 18, 15, 4, 0, 0, time.UTC)
	for _, tc := range []struct {
		s    Strings
		want string
	}{
		{Strings{AM: "AM", PM: "PM"}, "3:04 PM"},
		{Strings{Hour24: true}, "15:04"},
		{Strings{AM: "오전", PM: "오후", PeriodFirst: true}, "오후 3:04"},
	} {
		if got := tc.s.Time(afternoon); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}
