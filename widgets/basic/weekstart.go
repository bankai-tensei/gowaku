package basic

import (
	"time"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/core"
)

// WeekStart returns the day a week starts on in the window of ctx, as a
// calendar or a week view lays out its days: see WeekStartOf. It rebuilds ctx
// when the locale changes.
func WeekStart(ctx core.BuildContext) time.Weekday {
	return WeekStartOf(LocaleOf(ctx))
}

// WeekStartOf returns the day a week starts on in locale: that of its region,
// as CLDR has it, or of the likely region of its language when it names none
// (US for en, JP for ja, DE for de). Sunday in the United States and Japan,
// Monday in most of Europe, Saturday in much of the Middle East.
func WeekStartOf(locale language.Tag) time.Weekday {
	region, _ := locale.Region()
	switch region.String() {
	case "AG", "AS", "BD", "BR", "BS", "BT", "BW", "BZ", "CA", "CN", "CO", "DM", "DO", "ET", "GT", "GU",
		"HK", "HN", "ID", "IL", "IN", "JM", "JP", "KE", "KH", "KR", "LA", "MH", "MM", "MO", "MT", "MX",
		"MZ", "NI", "NP", "PA", "PE", "PH", "PK", "PR", "PT", "PY", "SA", "SG", "SV", "TH", "TT", "TW",
		"UM", "US", "VE", "VI", "WS", "YE", "ZA", "ZW":
		return time.Sunday
	case "AE", "AF", "BH", "DJ", "DZ", "EG", "IQ", "IR", "JO", "KW", "LY", "OM", "QA", "SD", "SY":
		return time.Saturday
	case "MV":
		return time.Friday
	}
	return time.Monday
}
