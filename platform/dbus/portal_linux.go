package dbus

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"

	godbus "github.com/godbus/dbus/v5"
)

const (
	portalName = "org.freedesktop.portal.Desktop"
	portalPath = "/org/freedesktop/portal/desktop"

	settingsInterface = "org.freedesktop.portal.Settings"
	appearance        = "org.freedesktop.appearance"
	colorSchemeKey    = "color-scheme"
	// gnomeInterface is where older backends, such as xdg-desktop-portal-gtk,
	// keep the setting, as the string of gsettings.
	gnomeInterface = "org.gnome.desktop.interface"
)

// ErrUnavailable means that the session has no bus or no portal for the request.
var ErrUnavailable = errors.New("dbus: no session bus or portal")

// session connects to the session bus once. It returns nil without one.
var session = sync.OnceValue(func() *godbus.Conn {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" && os.Getenv("XDG_RUNTIME_DIR") == "" {
		slog.Debug("dbus: no session bus address")
		return nil
	}
	c, err := godbus.ConnectSessionBus()
	if err != nil {
		slog.Debug("dbus: connect session bus", "err", err)
		return nil
	}
	return c
})

// ReadColorScheme asks the portal for the color scheme of the desktop. ok is
// false without a portal that knows it, as on desktops older than 2022.
func ReadColorScheme() (scheme ColorScheme, ok bool) {
	c := session()
	if c == nil {
		return 0, false
	}
	for _, namespace := range []string{appearance, gnomeInterface} {
		if v, ok := readSetting(c, namespace, colorSchemeKey); ok {
			if s, ok := schemeOf(v); ok {
				return s, true
			}
		}
	}
	return 0, false
}

func readSetting(c *godbus.Conn, namespace, key string) (godbus.Variant, bool) {
	var v godbus.Variant
	obj := c.Object(portalName, portalPath)
	// ReadOne is from version 2 of the interface, and Read is deprecated but
	// everywhere.
	if err := obj.Call(settingsInterface+".ReadOne", 0, namespace, key).Store(&v); err != nil {
		if err := obj.Call(settingsInterface+".Read", 0, namespace, key).Store(&v); err != nil {
			slog.Debug("dbus: read setting", "namespace", namespace, "err", err)
			return v, false
		}
	}
	return v, true
}

// schemeOf unwraps the value of the setting, which Read wraps twice. It is a
// number in org.freedesktop.appearance, and the string of gsettings in
// org.gnome.desktop.interface.
func schemeOf(v godbus.Variant) (ColorScheme, bool) {
	for range 2 {
		if inner, ok := v.Value().(godbus.Variant); ok {
			v = inner
		}
	}
	switch value := v.Value().(type) {
	case uint32:
		return ColorScheme(value), true
	case string:
		return schemeOfName(value), true
	}
	return 0, false
}

// WatchColorScheme calls fn with each new color scheme, on a goroutine of its
// own. It reports false if no change can come, without a bus.
func WatchColorScheme(fn func(ColorScheme)) bool {
	c := session()
	if c == nil {
		return false
	}
	err := c.AddMatchSignal(
		godbus.WithMatchObjectPath(portalPath),
		godbus.WithMatchInterface(settingsInterface),
		godbus.WithMatchMember("SettingChanged"),
	)
	if err != nil {
		slog.Debug("dbus: watch color scheme", "err", err)
		return false
	}
	signals := make(chan *godbus.Signal, 8)
	c.Signal(signals)
	go func() {
		for s := range signals {
			if s.Name != settingsInterface+".SettingChanged" || len(s.Body) != 3 {
				continue
			}
			namespace, _ := s.Body[0].(string)
			key, _ := s.Body[1].(string)
			value, _ := s.Body[2].(godbus.Variant)
			if (namespace != appearance && namespace != gnomeInterface) || key != colorSchemeKey {
				continue
			}
			if scheme, ok := schemeOf(value); ok {
				fn(scheme)
			}
		}
	}()
	return true
}

// call calls a method of the portal, turning the absence of a portal into
// ErrUnavailable.
func call(c *godbus.Conn, method string, args ...any) (*godbus.Call, error) {
	r := c.Object(portalName, portalPath).Call(method, 0, args...)
	if r.Err != nil {
		var e godbus.Error
		if errors.As(r.Err, &e) && (e.Name == "org.freedesktop.DBus.Error.ServiceUnknown" ||
			e.Name == "org.freedesktop.DBus.Error.UnknownMethod" || e.Name == "org.freedesktop.DBus.Error.UnknownInterface") {
			slog.Debug("dbus: no portal", "method", method, "err", r.Err)
			return nil, ErrUnavailable
		}
		return nil, fmt.Errorf("dbus: %s: %w", method, r.Err)
	}
	return r, nil
}
