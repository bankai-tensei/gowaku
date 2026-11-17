package basic

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/theme"
)

type brandTokens struct{ Highlight geom.Color }

// themeSwap holds a Theme whose data the test replaces.
type themeSwap struct {
	core.StatefulBase
	first *theme.Theme
	state **themeSwapState
	child core.Widget
}

type themeSwapState struct {
	core.StateBase
	data *theme.Theme
}

func (themeSwap) CreateState() core.State { return &themeSwapState{} }

func (s *themeSwapState) InitState() {
	w := s.Widget().(themeSwap)
	s.data, *w.state = w.first, s
}

func (s *themeSwapState) Build(core.BuildContext) core.Widget {
	return Theme{Data: s.data, Child: s.Widget().(themeSwap).child}
}

func TestThemeExtension_ReadInBuildFollowsTheTheme(t *testing.T) {
	light := theme.WithExtension(theme.Default().Light, brandTokens{Highlight: geom.Hex(0xffd60a)})
	dark := theme.WithExtension(theme.Default().Dark, brandTokens{Highlight: geom.Hex(0x806b05)})
	var swap *themeSwapState
	var seen []geom.Color
	tree := wtest.A11yTree(t, themeSwap{first: light, state: &swap, child: Builder{Content: func(ctx core.BuildContext) core.Widget {
		seen = append(seen, theme.ExtensionOr(ThemeOf(ctx), brandTokens{}).Highlight)
		return SizedBox{}
	}}})
	swap.SetState(func() { swap.data = dark })
	tree.Build()
	if len(seen) != 2 || seen[0] != geom.Hex(0xffd60a) || seen[1] != geom.Hex(0x806b05) {
		t.Errorf("Build saw %v", seen)
	}
}
