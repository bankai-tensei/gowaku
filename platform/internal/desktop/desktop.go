// Package desktop reads settings of Linux desktops that do not depend on the
// window system: platform/x11 and platform/wayland share it.
package desktop

import (
	"os/exec"
	"strings"
	"time"

	"github.com/bankai-tensei/gowaku/platform"
)

// gsettingsTimeout bounds the wait at startup for a desktop that does not answer.
const gsettingsTimeout = time.Second

func gsettings(key string) string {
	cmd := exec.Command("gsettings", "get", "org.gnome.desktop.interface", key)
	timer := time.AfterFunc(gsettingsTimeout, func() {
		if cmd.Process != nil {
			// It may have finished by itself, which is fine too.
			_ = cmd.Process.Kill() //nolint:errcheck // see above
		}
	})
	defer timer.Stop()
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// ReadBrightness asks gsettings once, for desktops without the color scheme of
// xdg-desktop-portal.
func ReadBrightness() platform.Brightness {
	return BrightnessOf(gsettings("color-scheme"), gsettings("gtk-theme"))
}

// BrightnessOf reads the answers of gsettings, each still in its quotes. Older
// desktops have no color scheme and show it in the name of the theme.
func BrightnessOf(colorScheme, gtkTheme string) platform.Brightness {
	if strings.Contains(colorScheme, "prefer-dark") {
		return platform.BrightnessDark
	}
	if strings.Contains(colorScheme, "prefer-light") {
		return platform.BrightnessLight
	}
	if strings.Contains(strings.ToLower(gtkTheme), "-dark") {
		return platform.BrightnessDark
	}
	return platform.BrightnessLight
}
