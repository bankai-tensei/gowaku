package dbus

import (
	"log/slog"

	godbus "github.com/godbus/dbus/v5"
)

// enableAnimationsKey is the setting of GNOME that ReduceMotion follows (ADR
// 0043 §6). KDE has nothing the portal shows, which reads as no reduction.
const enableAnimationsKey = "enable-animations"

// ReadEnableAnimations asks the portal whether the desktop animates. ok is
// false without a portal that knows the setting.
func ReadEnableAnimations() (enabled, ok bool) {
	c := session()
	if c == nil {
		return false, false
	}
	v, ok := readSetting(c, gnomeInterface, enableAnimationsKey)
	if !ok {
		return false, false
	}
	return boolOf(v)
}

// boolOf unwraps a boolean setting, which Read wraps twice.
func boolOf(v godbus.Variant) (bool, bool) {
	for range 2 {
		if inner, ok := v.Value().(godbus.Variant); ok {
			v = inner
		}
	}
	b, ok := v.Value().(bool)
	return b, ok
}

// WatchEnableAnimations calls fn with each new value of the setting, on a
// goroutine of its own. It reports false if no change can come, without a bus.
func WatchEnableAnimations(fn func(enabled bool)) bool {
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
		slog.Debug("dbus: watch enable-animations", "err", err)
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
			if namespace != gnomeInterface || key != enableAnimationsKey {
				continue
			}
			if enabled, ok := boolOf(value); ok {
				fn(enabled)
			}
		}
	}()
	return true
}
