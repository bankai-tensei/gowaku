package nav

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

const (
	// backButtonHeight is the height of a BackButton, that of a regular
	// control, and backChevron the size of its arrow.
	backButtonHeight = 28
	backChevron      = 18
)

// BackButton goes back to the previous page of the nearest Navigator that has
// one, as Alt+Left does, for the top bar of a page. It is the way back for a
// screen reader, which hears it as a button named 戻る or Back. On the first
// page, with nowhere to go, it takes no room and shows nothing.
type BackButton struct {
	core.StatefulBase
	// Label is shown after the arrow, as the title of the previous page.
	// Empty shows the arrow alone, and a screen reader still hears 戻る or Back.
	Label string
}

type backButtonState struct {
	core.StateOf[BackButton]
	chevron *paint.Path
	scope   pageScope
	press   func()
}

// CreateState implements core.StatefulWidget.
func (BackButton) CreateState() core.State { return &backButtonState{} }

// InitState implements core.State.
func (s *backButtonState) InitState() {
	s.chevron = kit.ChevronPath(false)
	s.press = func() { s.scope.c.back() }
}

// Build implements core.State.
func (s *backButtonState) Build(ctx core.BuildContext) core.Widget {
	scope, ok := core.DependOn[pageScope](ctx)
	if !ok || !scope.back {
		return basic.SizedBox{}
	}
	s.scope = scope
	th := basic.ThemeOf(ctx)
	label := s.Widget().Label
	name := label
	if name == "" {
		name = basic.StringsOf(ctx).Back
	}
	return interact.Pressable{Label: name, OnPress: s.press, Build: func(ps interact.PressState) core.Widget {
		c := &th.Color
		var ground geom.Color
		switch {
		case ps.Pressed:
			ground = c.FillStrong
		case ps.Hovered:
			ground = c.Fill
		}
		// The button is as wide as what it shows, wherever it is put: a Center
		// here would take all the width a column gives it.
		children := []core.Widget{basic.Icon{Path: s.chevron, Size: backChevron, Color: c.AccentInk}}
		pad := geom.Symmetric(theme.Space4, 0)
		if label != "" {
			children = append(children, basic.Text{Content: label, Style: kit.Colored(th.Text.Control, c.AccentInk)})
			pad.Right = theme.Space8
		}
		return interact.FocusRing{Visible: ps.FocusVisible, Radius: th.Radius.Control, Child: basic.Container{
			Height: backButtonHeight, Color: ground, Radius: th.Radius.Control, Padding: pad,
			Child: basic.Row{Size: layout.MainMin, Cross: layout.CrossCenter, Spacing: theme.Space2, Children: children},
		}}
	}}
}
