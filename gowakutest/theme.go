package gowakutest

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// SetTheme implements the theme of gowaku.SetTheme: the tree takes set in
// mode, as the windows of Run do. ThemeSystem follows Options.Dark, which
// stands for the OS.
func (h *host) SetTheme(set theme.Set, mode theme.Mode) {
	tt := h.tt
	if set == tt.themes && mode == tt.mode {
		return
	}
	tt.themes, tt.mode = set, mode
	tt.theme = tt.themes.For(tt.brightness())
	if tt.themeRoot != nil {
		tt.themeRoot.SetState(func() {})
	}
}

// CurrentTheme implements the theme of gowaku.CurrentTheme.
func (h *host) CurrentTheme() (theme.Set, theme.Mode) { return h.tt.themes, h.tt.mode }

// brightness is the brightness the tree is drawn in.
func (tt *Tester) brightness() theme.Brightness {
	os := theme.BrightnessLight
	if tt.dark {
		os = theme.BrightnessDark
	}
	return tt.mode.For(os)
}

// themed puts the theme of the Tester above child, and rebuilds when
// gowaku.SetTheme changes it.
type themed struct {
	core.StatefulBase
	tt    *Tester
	child core.Widget
}

type themedState struct{ core.StateBase }

// CreateState implements core.StatefulWidget.
func (themed) CreateState() core.State { return &themedState{} }

// InitState implements core.State.
func (s *themedState) InitState() { s.Widget().(themed).tt.themeRoot = s }

// Build implements core.State.
func (s *themedState) Build(core.BuildContext) core.Widget {
	w := s.Widget().(themed)
	return basic.Theme{Data: w.tt.theme, Child: w.child}
}
