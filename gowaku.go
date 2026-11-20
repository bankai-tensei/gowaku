package gowaku

import (
	"errors"
	"image"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// ErrUnsupported reports that gowaku cannot run on this operating system yet.
var ErrUnsupported = errors.New("gowaku: this platform is not supported yet")

// ThemeMode says how an application treats the light and dark modes of the OS.
type ThemeMode = theme.Mode

// The theme modes.
const (
	// ThemeSystem follows the OS, also while running.
	ThemeSystem = theme.ModeSystem
	// ThemeLight stays light.
	ThemeLight = theme.ModeLight
	// ThemeDark stays dark.
	ThemeDark = theme.ModeDark
)

// App describes an application.
type App struct {
	Title string
	// Width and Height are the first window's client area in logical pixels.
	// Zero selects a default.
	Width, Height float32
	// Theme is the look of the application. The zero value is the default, and
	// theme.FromAccent makes one from a color.
	Theme theme.Set
	// ThemeMode picks the light or dark half of Theme.
	ThemeMode ThemeMode
	// Background fills the window behind Home. The zero value is the canvas of
	// the theme.
	Background geom.Color
	// OnClose runs when the first window closes. See WindowOptions.OnClose.
	OnClose func()
	// OnCloseRequest decides whether the first window closes when the user
	// asks. See WindowOptions.OnCloseRequest.
	OnCloseRequest func() bool
	// OnDrawError reports what the first window could not draw. See
	// WindowOptions.OnDrawError.
	OnDrawError func(error)
	// CustomFrame takes the title bar of the OS off the first window. See
	// WindowOptions.CustomFrame.
	CustomFrame bool
	// Icon, MinSize, MaxSize, AlwaysOnTop, Fullscreen and Transparent are
	// those of the first window. See the fields of WindowOptions.
	Icon             []image.Image
	MinSize, MaxSize geom.Size
	AlwaysOnTop      bool
	Fullscreen       bool
	Transparent      bool
	// AppID names the application to the Linux desktop, which matches it with
	// its desktop file by it: the app_id of Wayland and the class of WM_CLASS
	// on X11, such as "com.example.Editor". It should be the name of the
	// desktop file without ".desktop", where the desktop takes the icon of
	// the application from on Wayland. Empty is "gowaku". Windows and macOS
	// ignore it.
	AppID string
	// Locale is the language and region the standard widgets speak in every
	// window, which basic.MediaQueryData.Locale carries. The zero value
	// follows the locale of the OS, also while running where the OS announces
	// a change.
	Locale language.Tag
	// Strings are the tables of words the application adds to the bundled
	// English and Japanese ones, or puts in their place. See basic.Strings
	// and basic.Localizations.
	Strings []*basic.Strings
	Home    Widget
}
