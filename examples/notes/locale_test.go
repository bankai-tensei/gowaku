package main

import (
	"regexp"
	"testing"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// englishOS stands for the root that Run gives a window on an English OS: a
// MediaQuery in en-US and the Localizations of the App.
type englishOS struct {
	core.StatelessBase
	child core.Widget
}

func (e englishOS) Build(ctx core.BuildContext) core.Widget {
	md := basic.MediaQueryOf(ctx)
	md.Locale = language.AmericanEnglish
	return basic.MediaQuery{Data: md, Child: basic.Localizations{Child: e.child}}
}

// The notebook is a Japanese application: on an English OS the words the
// library supplies, such as the count line and the empty result of the
// palette, are Japanese as well.
func TestPalette_SpeaksJapaneseOnAnEnglishOS(t *testing.T) {
	if b, _ := appLocale.Base(); b.String() != "ja" {
		t.Fatalf("appLocale = %v, want Japanese", appLocale)
	}
	fs, st := newDemo(demoNow())
	sh := &localShell{}
	a := newApp(st, fs.disk(), sh, func(dir string) notebook { return memLibrary{fs: fs, dir: dir} })
	a.now = demoNow
	w := a.restore()
	h := &harness{t: t, a: a, sh: sh, fs: fs, st: st, w: w}
	h.tt = gowakutest.New(t, englishOS{child: windowView{model: w}}, gowakutest.Options{Width: windowWidth, Height: windowHeight})
	h.settle()

	ja := basic.JapaneseStrings()
	h.key(input.KeyK, ctrl)
	if _, ok := h.tt.Find("新規"); !ok {
		t.Fatalf("the palette did not open:\n%s", h.tt.Dump())
	}
	dump := h.tt.Dump()
	if !regexp.MustCompile(`1 / \d+ 件`).MatchString(dump) {
		t.Errorf("no Japanese count line:\n%s", dump)
	}
	if regexp.MustCompile(`\d+ of \d+`).MatchString(dump) {
		t.Errorf("an English count line:\n%s", dump)
	}
	h.typeText("qqqqzzzz")
	if _, ok := h.tt.Find(ja.NoResults); !ok {
		t.Errorf("no %q for a query without results:\n%s", ja.NoResults, h.tt.Dump())
	}
	if _, ok := h.tt.Find("No results"); ok {
		t.Error("the empty result is in English")
	}
}
