package overlay

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/theme"
)

// scopeProbe records the theme and the words of each build.
type scopeProbe struct {
	core.StatelessBase
	themes  *[]*theme.Theme
	strings *[]*basic.Strings
}

// Build implements core.StatelessWidget.
func (p scopeProbe) Build(ctx core.BuildContext) core.Widget {
	*p.themes = append(*p.themes, basic.ThemeOf(ctx))
	if p.strings != nil {
		*p.strings = append(*p.strings, basic.StringsOf(ctx))
	}
	return basic.SizedBox{Width: 10, Height: 10}
}

// swappableTheme is a basic.Theme whose data a test can change.
func swappableTheme(th *theme.Theme, state **wtest.HolderState, child core.Widget) core.Widget {
	return wtest.Holder{First: basic.Theme{Data: th, Child: child}, State: state}
}

func swapTheme(s *wtest.HolderState, th *theme.Theme, child core.Widget) {
	s.SetState(func() { s.Child = basic.Theme{Data: th, Child: child} })
}

func TestShowDialog_TakesTheThemeOfTheCaller(t *testing.T) {
	window := theme.Default().Light
	part := theme.FromAccent(geom.Hex(0xc0392b)).Dark
	var ctx core.BuildContext
	var nested *wtest.HolderState
	caller := wtest.Capture{Got: &ctx, Child: basic.SizedBox{}}
	// The Overlay is above the Theme and the MediaQuery of the part, as the
	// one of Run is above those of the application.
	tree := wtest.TextTree(t, basic.Theme{Data: window, Child: basic.Localizations{Child: Overlay{Child: swappableTheme(part, &nested,
		basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1, Locale: language.Japanese}, Child: caller}),
	}}})
	wtest.Ops(tree, wtest.Screen)

	var themes []*theme.Theme
	var words []*basic.Strings
	ShowDialog(ctx, func(func()) core.Widget { return scopeProbe{themes: &themes, strings: &words} })
	wtest.Ops(tree, wtest.Screen)
	if len(themes) == 0 || themes[len(themes)-1] != part {
		t.Fatalf("the dialog must take the theme of the part, got %v", themes)
	}
	if words[len(words)-1].Language != language.Japanese {
		t.Errorf("the dialog must speak the language of the part, got %v", words[len(words)-1].Language)
	}

	// The dialog follows the theme of the part while it shows.
	again := theme.FromAccent(geom.Hex(0x2e86c1)).Light
	swapTheme(nested, again, basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1, Locale: language.Japanese}, Child: caller})
	wtest.Ops(tree, wtest.Screen)
	if got := themes[len(themes)-1]; got != again {
		t.Errorf("the dialog must follow the theme of the part, got %p want %p", got, again)
	}
}

func TestShowDialog_KeepsTheThemeOfACallerThatLeft(t *testing.T) {
	part := theme.FromAccent(geom.Hex(0xc0392b)).Dark
	var ctx core.BuildContext
	var top, body *wtest.HolderState
	tree := wtest.TextTree(t, swappableTheme(theme.Default().Light, &top, Overlay{Child: wtest.Holder{
		First: basic.Theme{Data: part, Child: wtest.Capture{Got: &ctx, Child: basic.SizedBox{}}},
		State: &body,
	}}))
	wtest.Ops(tree, wtest.Screen)
	var themes []*theme.Theme
	ShowDialog(ctx, func(func()) core.Widget { return scopeProbe{themes: &themes} })
	wtest.Ops(tree, wtest.Screen)

	// The part goes, as a row that showed a toast and was deleted does; then
	// the window changes its theme, which rebuilds the overlay.
	body.SetState(func() { body.Child = basic.SizedBox{} })
	wtest.Ops(tree, wtest.Screen)
	swapTheme(top, theme.Default().Dark, Overlay{Child: wtest.Holder{First: basic.SizedBox{}, State: &body}})
	wtest.Ops(tree, wtest.Screen)
	if got := themes[len(themes)-1]; got != part {
		t.Errorf("the dialog must keep the theme of the part that opened it, got %p want %p", got, part)
	}
}

func TestShowDialog_WithoutAThemeOfItsOwnFollowsTheWindow(t *testing.T) {
	var ctx core.BuildContext
	var top, body *wtest.HolderState
	tree := wtest.TextTree(t, swappableTheme(theme.Default().Light, &top, Overlay{Child: wtest.Holder{
		First: wtest.Capture{Got: &ctx, Child: basic.SizedBox{}},
		State: &body,
	}}))
	wtest.Ops(tree, wtest.Screen)
	var themes []*theme.Theme
	ShowDialog(ctx, func(func()) core.Widget { return scopeProbe{themes: &themes} })
	wtest.Ops(tree, wtest.Screen)
	// Even once the caller has gone, the theme above it is still there.
	body.SetState(func() { body.Child = basic.SizedBox{} })
	wtest.Ops(tree, wtest.Screen)
	dark := theme.Default().Dark
	swapTheme(top, dark, Overlay{Child: wtest.Holder{First: basic.SizedBox{}, State: &body}})
	wtest.Ops(tree, wtest.Screen)
	if got := themes[len(themes)-1]; got != dark {
		t.Errorf("the dialog must follow the theme of the window, got %p want %p", got, dark)
	}
}

func TestShowToast_EachToastTakesTheThemeOfItsCaller(t *testing.T) {
	window := theme.Default().Light
	part := theme.FromAccent(geom.Hex(0xc0392b)).Dark
	var plain, inPart core.BuildContext
	var o *Host
	tree := wtest.TextTree(t, basic.Theme{Data: window, Child: Overlay{Child: wtest.Capture{Got: new(core.BuildContext), Child: basic.Column{
		Children: []core.Widget{
			wtest.Capture{Got: &plain, Child: basic.SizedBox{Height: 10}},
			basic.Theme{Data: part, Child: wtest.Capture{Got: &inPart, Child: basic.SizedBox{Height: 10}}},
		},
	}}}})
	wtest.Ops(tree, wtest.Screen)
	o = Of(plain)
	ShowToast(plain, Toast{Message: "window"})
	ShowToast(inPart, Toast{Message: "part"})
	now := time.Second
	wtest.Advance(tree, wtest.Screen, &now, 50*time.Millisecond)

	items := o.toasts.items
	if len(items) != 2 {
		t.Fatalf("got %d toasts", len(items))
	}
	for i, want := range []*theme.Theme{window, part} {
		got, _ := core.Lookup[*theme.Theme](items[i].state.Context())
		if got != want {
			t.Errorf("toast %q: got theme %p, want %p", items[i].toast.Message, got, want)
		}
	}
}

func TestInsertFrom_LeavesInsertAsItWas(t *testing.T) {
	window := theme.Default().Light
	var ctx core.BuildContext
	tree := wtest.TextTree(t, basic.Theme{Data: window, Child: Overlay{Child: basic.Theme{
		Data:  theme.Default().Dark,
		Child: wtest.Capture{Got: &ctx, Child: basic.SizedBox{}},
	}}})
	wtest.Ops(tree, wtest.Screen)
	var themes []*theme.Theme
	Of(ctx).Insert(func() core.Widget { return scopeProbe{themes: &themes} })
	wtest.Ops(tree, wtest.Screen)
	if got := themes[len(themes)-1]; got != window {
		t.Errorf("an entry of Insert takes the theme of the overlay, got %p", got)
	}
}
