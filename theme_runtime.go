package gowaku

import "github.com/bankai-tensei/gowaku/theme"

// SetTheme changes the look of every window of the application while it runs,
// as App.Theme and App.ThemeMode set it at the start: a settings page calls it
// to change the accent color or to choose light, dark or the mode of the OS.
//
//	gowaku.SetTheme(ctx, theme.FromAccent(accent), gowaku.ThemeDark)
//
// Every widget that reads the theme rebuilds, and what the OS draws for the
// windows, their title bars and backgrounds, follows. With ThemeSystem the
// windows follow the OS again from then on. Dialogs, toasts and menus that
// are showing take the new theme too, unless they were opened under a
// basic.Theme of their own. basic.MediaQueryData.Brightness stays what the OS asks
// for; basic.ThemeOf(ctx).Brightness is the one in use.
//
// The theme belongs to the application, not to a window: windows opened later
// take it too. Give part of the tree, or a whole window, a look of its own with
// basic.Theme. See docs/ADR/0067-runtime-theme.md.
//
// The theme goes through the core.Host of the tree of ctx, as ShowTray and
// the clipboard functions do: in a window of Run that is the application, and
// under a gowakutest.Tester the Tester, whose tree takes the theme as the
// windows do. A host that keeps no theme, as outside both, ignores it.
//
// It must be called on the UI goroutine, outside a Build, as from a callback.
func SetTheme(ctx BuildContext, set theme.Set, mode ThemeMode) {
	if h, ok := ctx.Host().(themeHost); ok {
		h.SetTheme(set, mode)
	}
}

// CurrentTheme returns the theme of the application and its mode: those of
// App, or of the last SetTheme. Under a gowakutest.Tester they are those of
// its Options, or of the last SetTheme. The zero theme.Set stands for
// theme.Default, as in App, and a host that keeps no theme gives it with
// ThemeSystem. It must be called on the UI goroutine.
func CurrentTheme(ctx BuildContext) (theme.Set, ThemeMode) {
	if h, ok := ctx.Host().(themeHost); ok {
		return h.CurrentTheme()
	}
	return theme.Set{}, ThemeSystem
}

// themeHost is a core.Host that keeps the theme: the window of Run, and the
// Tester of gowakutest, which matches it by its methods without importing
// gowaku; gowakutest's TestSetTheme_TheTreeTakesIt fails if the two part.
type themeHost interface {
	SetTheme(set theme.Set, mode ThemeMode)
	CurrentTheme() (theme.Set, ThemeMode)
}

// SetTheme implements themeHost, for SetTheme.
func (h windowHost) SetTheme(set theme.Set, mode ThemeMode) { h.w.app.setTheme(set, mode) }

// CurrentTheme implements themeHost, for CurrentTheme.
func (h windowHost) CurrentTheme() (theme.Set, ThemeMode) { return h.w.app.themes, h.w.app.mode }

// setTheme puts set and mode in place and passes the change on to every
// window.
func (ap *app) setTheme(set theme.Set, mode ThemeMode) {
	if set == ap.themes && mode == ap.mode {
		return
	}
	ap.themes, ap.mode = set, mode
	// The OS may have changed while a fixed mode made the change moot to the
	// look; ThemeSystem starts from where it is now.
	ap.brightness = ap.platform.Brightness()
	for _, w := range ap.windows {
		w.applyTheme()
		w.root.refresh()
	}
}
