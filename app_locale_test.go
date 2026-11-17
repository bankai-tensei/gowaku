package gowaku

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/platform"
)

// localeProbe records the locale and the Today word of each build.
type localeProbe struct {
	StatelessBase
	seen *[]string
}

func (p localeProbe) Build(ctx BuildContext) Widget {
	*p.seen = append(*p.seen, basic.LocaleOf(ctx).String()+" "+basic.StringsOf(ctx).Today)
	return basic.SizedBox{}
}

func TestRun_LocaleFollowsTheOS(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	var seen []string
	p, b = newFakes(t,
		func() {
			p.locale = "en-GB"
			// A change of something else must not rebuild.
			p.send(platform.Event{Kind: platform.EventSettings})
		},
		func() { p.send(platform.Event{Kind: platform.EventSettings}) },
		func() { p.send(platform.Event{Kind: platform.EventClose, Window: 1}) },
	)
	p.locale = "ja-JP"
	if err := run(p, b, testutil.NewShaper(t), App{Home: localeProbe{seen: &seen}}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen[0] != "ja-JP 今日" || seen[1] != "en-GB Today" {
		t.Errorf("got %q", seen)
	}
}

func TestRun_AppLocaleAndStrings(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	var seen []string
	p, b = newFakes(t,
		func() {
			p.locale = "ja-JP"
			p.send(platform.Event{Kind: platform.EventSettings})
		},
		func() { p.send(platform.Event{Kind: platform.EventClose, Window: 1}) },
	)
	p.locale = "en-US"
	german := &basic.Strings{Language: language.German, Today: "Heute"}
	app := App{Home: localeProbe{seen: &seen}, Locale: language.MustParse("de-AT"), Strings: []*basic.Strings{german}}
	if err := run(p, b, testutil.NewShaper(t), app); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || seen[0] != "de-AT Heute" {
		t.Errorf("got %q, want one build in de-AT whatever the OS says", seen)
	}
}

func TestParseLocale(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"ja-JP", "ja-JP"}, {"en", "en"}, {"", "und"}, {"not a tag", "und"},
	} {
		if got := parseLocale(tc.in).String(); got != tc.want {
			t.Errorf("parseLocale(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
