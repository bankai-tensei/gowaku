package gowaku

import "github.com/bankai-tensei/gowaku/theme"

// SetTheme changes the look of every window of the application while it runs,
// as App.Theme and App.ThemeMode set it at the start: a settings page calls it
// to change the accent color or to choose light, dark or the mode of the OS.
//
//	gowaku.SetTheme(theme.FromAccent(accent), gowaku.ThemeDark)
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
// It must be called on the UI goroutine, during Run, outside a Build.
func SetTheme(set theme.Set, mode ThemeMode) {
	ap := currentApp.Load()
	if ap == nil {
		panic("gowaku: SetTheme called outside Run")
	}
	ap.setTheme(set, mode)
}

// CurrentTheme returns the theme of the application and its mode: those of
// App, or of the last SetTheme. It must be called on the UI goroutine, during
// Run.
func CurrentTheme() (theme.Set, ThemeMode) {
	ap := currentApp.Load()
	if ap == nil {
		panic("gowaku: CurrentTheme called outside Run")
	}
	return ap.themes, ap.mode
}

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
