// Package dbus is what gowaku asks of Linux desktops over D-Bus: the color
// scheme and enable-animations of xdg-desktop-portal, its file chooser, and
// the AT-SPI2 bridge that screen readers such as Orca read the windows
// through (ADR 0043). platform/x11 and platform/wayland share it. Without a
// session bus, a portal or an accessibility bus every function reports that it
// cannot, and the reason goes to slog.Debug.
//
// It uses godbus, which is pure Go. The bus code builds only on Linux; the
// mapping of semantics nodes to AT-SPI is plain Go, tested on every system.
package dbus
