package basic

import (
	"testing"
	"time"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
)

func TestStrings_BundledTablesAreComplete(t *testing.T) {
	// English leaves its formats nil: the methods write English.
	if missing := testutil.EmptyFields(&englishTable, false); len(missing) > 0 {
		t.Errorf("en leaves out %v", missing)
	}
	// Japanese must not fall back to the English order.
	if missing := testutil.EmptyFields(&japaneseTable, true); len(missing) > 0 {
		t.Errorf("ja leaves out %v", missing)
	}
}

func TestStrings_EmptyTableFallsBackToEnglish(t *testing.T) {
	c := newTableChoice([]*Strings{{Language: language.German, Today: "Heute"}})
	de := c.match(language.German)
	if missing := testutil.EmptyFields(de, false); len(missing) > 0 {
		t.Errorf("fillFrom misses %v", missing)
	}
	if de.Today != "Heute" || de.Close != "Close" || de.Months[10] != "November" {
		t.Errorf("got %q %q %q", de.Today, de.Close, de.Months[10])
	}
	// A nil format writes English order with the words of the table.
	if got := de.MonthYear(time.November, 2026); got != "November 2026" {
		t.Errorf("MonthYear = %q", got)
	}
}

func TestLocalizations_Matching(t *testing.T) {
	german := &Strings{Language: language.German, Today: "Heute"}
	british := &Strings{Language: language.BritishEnglish, Today: "Today (GB)"}
	ownJapanese := &Strings{Language: language.Japanese, Today: "本日"}
	c := newTableChoice([]*Strings{german, british, nil})
	for _, tc := range []struct{ locale, want string }{
		{"ja-JP", "今日"},
		{"ja", "今日"},
		{"en-US", "Today"},
		{"en-GB", "Today (GB)"},
		{"en-AU", "Today (GB)"},
		{"de-AT", "Heute"},
		{"de-CH", "Heute"},
		{"pt-BR", "Today"},
		{"zh-Hans-CN", "Today"},
		{"und", "Today"},
	} {
		if got := c.match(language.MustParse(tc.locale)).Today; got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.locale, got, tc.want)
		}
	}
	if got := newTableChoice([]*Strings{ownJapanese}).match(language.MustParse("ja-JP")).Today; got != "本日" {
		t.Errorf("a table of the same language must replace the bundled one, got %q", got)
	}
}

func TestStrings_Formats(t *testing.T) {
	d := time.Date(2026, time.November, 9, 0, 0, 0, 0, time.UTC)
	en, ja := &englishTable, &japaneseTable
	for _, tc := range []struct{ got, want string }{
		{en.MonthYear(time.November, 2026), "November 2026"},
		{ja.MonthYear(time.November, 2026), "2026年11月"},
		{en.Date(d), "Nov 9, 2026"},
		{ja.Date(d), "2026/11/09"},
		{en.SpokenDate(d), "Monday, November 9, 2026"},
		{ja.SpokenDate(d), "2026年11月9日 月曜日"},
		{en.MatchCount(3, 12), "3 of 12"},
		{en.MatchCount(0, 12), "12 found"},
		{ja.MatchCount(3, 12), "3 / 12 件"},
		{ja.MatchCount(0, 12), "12 件"},
		{en.RemoveNamed("Draft"), "Remove Draft"},
		{ja.RemoveNamed("下書き"), "下書きを削除"},
		{en.CloseNamed("Notes"), "Close Notes"},
		{ja.CloseNamed("メモ"), "メモを閉じる"},
	} {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
}

// ctxProbe hands out the BuildContext it is built in.
type ctxProbe struct {
	core.StatelessBase
	ctx *core.BuildContext
}

func (p ctxProbe) Build(ctx core.BuildContext) core.Widget {
	*p.ctx = ctx
	return SizedBox{}
}

// localizedCtx mounts a probe below a MediaQuery of outer, a Localizations
// with extra, and a MediaQuery of inner unless it is the root locale.
func localizedCtx(t testing.TB, outer, inner language.Tag, extra ...*Strings) core.BuildContext {
	t.Helper()
	var ctx core.BuildContext
	probe := core.Widget(ctxProbe{ctx: &ctx})
	if inner != language.Und {
		probe = MediaQuery{Data: MediaQueryData{Scale: 1, Locale: inner}, Child: probe}
	}
	tree := core.NewTree(MediaQuery{Data: MediaQueryData{Scale: 1, Locale: outer}, Child: Localizations{
		Strings: extra, Child: probe,
	}})
	t.Cleanup(tree.Dispose)
	tree.Build()
	tree.Layout(geom.Tight(geom.Size{W: 10, H: 10}))
	return ctx
}

func TestStringsOf_LocalizationsAndOverrides(t *testing.T) {
	french := &Strings{Language: language.French, Today: "Aujourd’hui"}
	for _, tc := range []struct {
		name         string
		outer, inner string
		want         string
	}{
		{"Localizations の言語", "fr-FR", "und", "Aujourd’hui"},
		{"下の MediaQuery が別の言語", "en-US", "fr-CA", "Aujourd’hui"},
		{"下の MediaQuery が同梱の言語", "fr-FR", "ja-JP", "今日"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := localizedCtx(t, language.MustParse(tc.outer), language.MustParse(tc.inner), french)
			if got := StringsOf(ctx).Today; got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatInt_Locales(t *testing.T) {
	for _, tc := range []struct {
		locale string
		want   string
		dec    string
	}{
		{"en-US", "1,234,567", "1,234.50"},
		{"ja-JP", "1,234,567", "1,234.50"},
		{"de-DE", "1.234.567", "1.234,50"},
		// x/text carries a CLDR from before the narrow no-break space.
		{"fr-FR", "1 234 567", "1 234,50"},
		{"und", "1,234,567", "1,234.50"},
	} {
		t.Run(tc.locale, func(t *testing.T) {
			tag := language.MustParse(tc.locale)
			// Through a Localizations, and without one.
			for _, ctx := range []core.BuildContext{localizedCtx(t, tag, language.Und), bareCtx(t, tag)} {
				if got := FormatInt(ctx, 1234567); got != tc.want {
					t.Errorf("FormatInt = %q, want %q", got, tc.want)
				}
				if got := FormatDecimal(ctx, 1234.5, 2); got != tc.dec {
					t.Errorf("FormatDecimal = %q, want %q", got, tc.dec)
				}
			}
		})
	}
}

// bareCtx mounts a probe below a MediaQuery of locale alone.
func bareCtx(t testing.TB, locale language.Tag) core.BuildContext {
	t.Helper()
	var ctx core.BuildContext
	tree := core.NewTree(MediaQuery{Data: MediaQueryData{Scale: 1, Locale: locale}, Child: ctxProbe{ctx: &ctx}})
	t.Cleanup(tree.Dispose)
	tree.Build()
	return ctx
}

func TestStringsOf_DoesNotAllocate(t *testing.T) {
	ctx := localizedCtx(t, language.Japanese, language.Und)
	bare := bareCtx(t, language.Japanese)
	if n := testing.AllocsPerRun(100, func() { _ = StringsOf(ctx); _ = StringsOf(bare) }); n != 0 {
		t.Errorf("StringsOf allocates %v times", n)
	}
}

func BenchmarkStringsOf_Localizations(b *testing.B) {
	ctx := localizedCtx(b, language.MustParse("ja-JP"), language.Und)
	b.ReportAllocs()
	for b.Loop() {
		_ = StringsOf(ctx)
	}
}
