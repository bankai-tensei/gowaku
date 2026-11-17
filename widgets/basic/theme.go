package basic

import (
	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/theme"
)

// Theme gives the widgets below it their look. Run provides one for the window,
// which follows the OS; use this widget for a part that looks different.
type Theme struct {
	core.StatelessBase
	Data  *theme.Theme
	Child core.Widget
}

// Build implements core.StatelessWidget.
func (t Theme) Build(core.BuildContext) core.Widget {
	return core.Inherited[*theme.Theme]{Value: t.Data, Child: t.Child}
}

// ThemeOf returns the nearest theme above ctx, and rebuilds ctx when it changes.
// Without one it returns the light default. A theme is read-only.
func ThemeOf(ctx core.BuildContext) *theme.Theme {
	if th, ok := core.DependOn[*theme.Theme](ctx); ok {
		return th
	}
	return theme.Default().Light
}

// MediaQueryData describes the window around a widget.
type MediaQueryData struct {
	// Brightness is what the OS asks for, whatever the theme in use.
	Brightness theme.Brightness
	// Scale is physical pixels per logical pixel.
	Scale float32
	// ReduceMotion means the user asked the OS for less animation. The
	// standard widgets then jump instead of moving or resizing, and keep their
	// fades (ADR 0042).
	ReduceMotion bool
	// Locale is the language and region of the window. The standard widgets
	// take their words, the names of months and days and the first day of the
	// week from it, through Localizations (see Strings). Run sets it to
	// App.Locale, or else to the locale of the OS. The zero value reads as
	// English. A MediaQuery that changes only the locale overrides it for the
	// tree below.
	Locale language.Tag
	// Size is the content area of the window in logical pixels, without the
	// title bar that gowaku draws. Run sets it and keeps it up to date as the
	// window is resized, for a layout that changes with the window, such as a
	// sidebar that folds away when it is narrow:
	//
	//	if basic.WindowSizeOf(ctx).W < 700 { ... }
	//
	// For the room of a widget within the window, use LayoutBuilder.
	Size geom.Size
}

// MediaQuery tells the widgets below it about the window. Run provides one.
//
// It hands the Size on apart from the rest, so that a resize rebuilds only the
// widgets that read the size: those that call WindowSizeOf or MediaQueryOf. Widgets
// that need only one of the other fields read it with ReduceMotionOf or
// LocaleOf, and are not rebuilt by a resize.
type MediaQuery struct {
	core.StatelessBase
	Data  MediaQueryData
	Child core.Widget
}

// mediaSize is the Size of a MediaQuery, as the tree sees it.
type mediaSize geom.Size

// Build implements core.StatelessWidget.
func (m MediaQuery) Build(core.BuildContext) core.Widget {
	d := m.Data
	size := mediaSize(d.Size)
	d.Size = geom.Size{}
	return core.Inherited[MediaQueryData]{Value: d, Child: core.Inherited[mediaSize]{Value: size, Child: m.Child}}
}

// reduceMotion reports whether the window of a State asks for less motion.
func reduceMotion(s *core.StateBase) bool { return ReduceMotionOf(s.Context()) }

// MediaQueryOf returns what the nearest MediaQuery says, and rebuilds ctx when it
// changes, a resize included. Without one it returns a light window of scale 1
// and no size.
func MediaQueryOf(ctx core.BuildContext) MediaQueryData {
	d := mediaSettingsOf(ctx)
	d.Size = WindowSizeOf(ctx)
	return d
}

// WindowSizeOf returns the Size of the nearest MediaQuery, the content area of the
// window, and rebuilds ctx when it changes. Without a MediaQuery it is zero.
func WindowSizeOf(ctx core.BuildContext) geom.Size {
	s, _ := core.DependOn[mediaSize](ctx)
	return geom.Size(s)
}

// ReduceMotionOf returns the ReduceMotion of the nearest MediaQuery. It rebuilds
// ctx when a field other than Size changes, but not on a resize.
func ReduceMotionOf(ctx core.BuildContext) bool { return mediaSettingsOf(ctx).ReduceMotion }

// LocaleOf returns the Locale of the nearest MediaQuery. It rebuilds ctx when a
// field other than Size changes, but not on a resize.
func LocaleOf(ctx core.BuildContext) language.Tag { return mediaSettingsOf(ctx).Locale }

// mediaSettingsOf returns the nearest MediaQueryData without its Size, and
// rebuilds ctx when it changes.
func mediaSettingsOf(ctx core.BuildContext) MediaQueryData {
	if d, ok := core.DependOn[MediaQueryData](ctx); ok {
		return d
	}
	return MediaQueryData{Scale: 1}
}
