package desktop

import (
	"sync/atomic"

	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/dbus"
)

// Theme follows the light or dark mode of the desktop, and whether it
// animates. Its methods are safe from any goroutine.
type Theme struct {
	brightness atomic.Uint32
	changed    atomic.Bool
	// reduceMotion is the opposite of enable-animations, false when unknown
	// (ADR 0043 §6).
	reduceMotion    atomic.Bool
	settingsChanged atomic.Bool
}

// WatchTheme reads the mode of the desktop, from xdg-desktop-portal or else
// from gsettings, and follows the changes that the portal announces. It does
// the same for enable-animations, from the portal only. wake runs on another
// goroutine after each change, to make the event loop look.
func WatchTheme(wake func()) *Theme {
	t := &Theme{}
	b, ok := portalBrightness()
	if !ok {
		b = ReadBrightness()
	}
	t.brightness.Store(uint32(b))
	dbus.WatchColorScheme(func(s dbus.ColorScheme) {
		b := brightnessOfScheme(s)
		if platform.Brightness(t.brightness.Swap(uint32(b))) != b {
			t.changed.Store(true)
			wake()
		}
	})
	if enabled, ok := dbus.ReadEnableAnimations(); ok {
		t.reduceMotion.Store(!enabled)
	}
	dbus.WatchEnableAnimations(func(enabled bool) {
		if t.reduceMotion.Swap(!enabled) != !enabled {
			t.settingsChanged.Store(true)
			wake()
		}
	})
	return t
}

func portalBrightness() (platform.Brightness, bool) {
	s, ok := dbus.ReadColorScheme()
	if !ok || s == dbus.NoPreference {
		// No preference says nothing; the theme name of gsettings may.
		return 0, false
	}
	return brightnessOfScheme(s), true
}

func brightnessOfScheme(s dbus.ColorScheme) platform.Brightness {
	if s == dbus.PreferDark {
		return platform.BrightnessDark
	}
	return platform.BrightnessLight
}

// Brightness returns the current mode.
func (t *Theme) Brightness() platform.Brightness { return platform.Brightness(t.brightness.Load()) }

// TakeChange reports whether the mode changed since the last call.
func (t *Theme) TakeChange() bool { return t.changed.Swap(false) }

// ReduceMotion reports whether the desktop asks for fewer animations.
func (t *Theme) ReduceMotion() bool { return t.reduceMotion.Load() }

// TakeSettingsChange reports whether ReduceMotion changed since the last call.
func (t *Theme) TakeSettingsChange() bool { return t.settingsChanged.Swap(false) }
