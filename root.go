package gowaku

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

// rootWidget is what Run puts around Home: the services every widget can count
// on, which change only when the window or the OS do.
type rootWidget struct {
	core.StatefulBase
	window *window
	home   Widget
}

type rootState struct {
	core.StateBase
	// body is what lies below the MediaQuery. It is held by pointer, so that a
	// rebuild that hands over the same one leaves it alone: a resize changes
	// the MediaQuery and rebuilds only what reads the size. See core.Widget.
	body *rootBody
}

// CreateState implements core.StatefulWidget.
func (rootWidget) CreateState() core.State { return &rootState{} }

// InitState implements core.State.
func (s *rootState) InitState() { s.Widget().(rootWidget).window.root = s }

// refresh passes on a change of the window or the theme, which rebuilds
// everything below.
func (s *rootState) refresh() {
	s.body = nil
	s.SetState(func() {})
}

// resized passes on a new size of the window, which rebuilds only the widgets
// that read it.
func (s *rootState) resized() { s.SetState(func() {}) }

// Build implements core.State.
func (s *rootState) Build(core.BuildContext) core.Widget {
	r := s.Widget().(rootWidget)
	w := r.window
	if s.body == nil {
		s.body = &rootBody{window: w, home: r.home}
	}
	return core.Inherited[*window]{Value: w, Child: core.Inherited[*text.Shaper]{Value: w.app.shaper, Child: basic.MediaQuery{
		Data: basic.MediaQueryData{
			Brightness: osBrightness(w.app.brightness), Scale: w.scale, ReduceMotion: w.app.reduceMotion,
			Locale: w.app.locale, Size: w.contentSize(),
		},
		Child: s.body,
	}}}
}

// rootBody is the part of the root below the MediaQuery: the strings, the
// theme, the overlay and the title bar around Home.
type rootBody struct {
	core.StatelessBase
	window *window
	home   Widget
}

// Build implements core.StatelessWidget.
func (b *rootBody) Build(core.BuildContext) core.Widget {
	w := b.window
	th := w.app.theme()
	// The overlay covers the title bar too, so that a MenuBar in it can open
	// its menus.
	body := b.home
	frame := frameState{window: w, title: w.title, focused: w.focused, maximized: w.maximized,
		fullscreen: w.fullscreen, scale: w.scale}
	if w.titleBarShown() {
		body = basic.Column{Cross: layout.CrossStretch, Children: []core.Widget{
			titleBar{frame: frame, theme: th},
			basic.Expanded{Child: body},
		}}
	}
	body = core.Inherited[frameState]{Value: frame, Child: overlay.Overlay{Child: body}}
	return basic.Localizations{Strings: w.app.strings, Child: basic.Theme{Data: th, Child: body}}
}
