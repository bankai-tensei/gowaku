package wayland

import (
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/dbus"
)

// CreateTray implements platform.Trays, through StatusNotifierItem (ADR 0051).
func (p *Platform) CreateTray(opts platform.TrayOptions, on func(platform.TrayEvent)) (platform.Tray, error) {
	return p.trays().CreateTray(opts, on)
}

// Notify implements platform.Trays, through org.freedesktop.Notifications.
func (p *Platform) Notify(title, body string) error { return p.trays().Notify(title, body) }

func (p *Platform) trays() *dbus.Trays {
	if p.trayHost == nil {
		p.trayHost = &dbus.Trays{}
	}
	return p.trayHost
}
