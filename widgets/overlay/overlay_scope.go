package overlay

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// callerScope gives what an overlay shows for a widget the theme, the
// MediaQuery and the Localizations above that widget, at, and rebuilds when
// they change. The overlay of the window sits above any basic.Theme of the
// application, so without it a dialog would come up in the theme of the
// window. See docs/ADR/0067-runtime-theme.md.
//
// It always puts the same three widgets above child, taking the values of the
// overlay for what at lacks, so that the state below it survives a change.
type callerScope struct {
	core.StatelessBase
	at    core.BuildContext
	child core.Widget
}

// Build implements core.StatelessWidget.
func (c callerScope) Build(ctx core.BuildContext) core.Widget {
	th, ok := core.DependOnAt[*theme.Theme](ctx, c.at)
	if !ok {
		th = basic.ThemeOf(ctx)
	}
	// The data of a MediaQuery holds no Size, which reaches the entry from the
	// MediaQuery above the overlay, so that a resize does not rebuild it.
	md, ok := core.DependOnAt[basic.MediaQueryData](ctx, c.at)
	if !ok {
		md, ok = core.DependOn[basic.MediaQueryData](ctx)
	}
	if !ok {
		md = basic.MediaQueryData{Scale: 1}
	}
	l, ok := core.DependOnAt[*basic.Localized](ctx, c.at)
	if !ok {
		// Nil, without a Localizations above the overlay either, reads as none.
		l, _ = core.DependOn[*basic.Localized](ctx)
	}
	return core.Inherited[basic.MediaQueryData]{Value: md, Child: core.Inherited[*basic.Localized]{
		Value: l,
		Child: core.Inherited[*theme.Theme]{Value: th, Child: c.child},
	}}
}
