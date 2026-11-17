package linuxwin

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/wayland"
	"github.com/bankai-tensei/gowaku/platform/x11"
)

// Open connects to Wayland if $WAYLAND_DISPLAY is set, and to X11 otherwise.
// GOWAKU_BACKEND=x11 or GOWAKU_BACKEND=wayland forces one. Without a forced
// choice, a failed Wayland falls back to X11, as XWayland usually runs.
func Open() (platform.Platform, error) {
	switch choice := os.Getenv("GOWAKU_BACKEND"); choice {
	case "x11":
		return x11.New()
	case "wayland":
		return wayland.New()
	case "":
	default:
		return nil, fmt.Errorf("gowaku: GOWAKU_BACKEND=%q is neither x11 nor wayland", choice)
	}
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return x11.New()
	}
	p, err := wayland.New()
	if err == nil {
		return p, nil
	}
	if os.Getenv("DISPLAY") == "" {
		return nil, err
	}
	slog.Debug("gowaku: Wayland failed, using X11", "err", err)
	return x11.New()
}
