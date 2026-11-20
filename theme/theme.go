package theme

import (
	"strconv"
	"sync"

	"github.com/bankai-tensei/gowaku/geom"
)

// defaultAccent is the indigo that a theme has unless told otherwise.
const defaultAccent = 0x2f5fd0

// defaults holds the result of Default. See the package comment.
var defaults = sync.OnceValue(func() Set { return FromAccent(geom.Hex(defaultAccent)) })

// Brightness tells light themes from dark ones.
type Brightness uint8

// The brightnesses.
const (
	BrightnessLight Brightness = iota
	BrightnessDark
)

// String returns the name of b.
func (b Brightness) String() string {
	switch b {
	case BrightnessLight:
		return "light"
	case BrightnessDark:
		return "dark"
	}
	return "Brightness(" + strconv.Itoa(int(b)) + ")"
}

// Mode says how an application treats the light and dark modes of the OS: it
// follows them, or stays light or dark.
type Mode uint8

// The modes.
const (
	// ModeSystem follows the OS, also while running.
	ModeSystem Mode = iota
	// ModeLight stays light.
	ModeLight
	// ModeDark stays dark.
	ModeDark
)

// For returns the brightness that m asks for, where the OS asks for os.
func (m Mode) For(os Brightness) Brightness {
	switch m {
	case ModeLight:
		return BrightnessLight
	case ModeDark:
		return BrightnessDark
	}
	return os
}

// Theme is the full set of tokens for one brightness.
type Theme struct {
	Brightness Brightness
	Color      Colors
	Text       TextTheme
	Radius     Radii

	// extensions holds the tokens of the application, at most one value of
	// each dynamic type. See WithExtensions.
	extensions []any
}

// New returns the theme of an accent color.
func New(accent geom.Color, b Brightness) *Theme {
	return &Theme{Brightness: b, Color: NewColors(accent, b), Text: NewTextTheme(), Radius: NewRadii()}
}

// Set is a light and dark pair for following the OS. Either may be nil.
type Set struct {
	Light, Dark *Theme
}

// FromAccent returns both themes of an accent color.
func FromAccent(accent geom.Color) Set {
	return Set{Light: New(accent, BrightnessLight), Dark: New(accent, BrightnessDark)}
}

// Default returns the themes that apply when nothing else is asked for. It
// computes them once and returns the same Themes to every caller, so they must be
// treated as read-only. It is safe to call from any goroutine.
func Default() Set { return defaults() }

// For returns the theme of the given brightness. It falls back to the other
// theme of s, and to Default when s is empty.
func (s Set) For(b Brightness) *Theme {
	first, second := s.Light, s.Dark
	if b == BrightnessDark {
		first, second = s.Dark, s.Light
	}
	switch {
	case first != nil:
		return first
	case second != nil:
		return second
	}
	return Default().For(b)
}
