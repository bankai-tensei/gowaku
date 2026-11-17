package dbus

import (
	"net/url"
	"strings"
)

// pathOfURI returns the local path of a file URI. Others, such as those of a
// remote location that the portal could not mount, have none.
func pathOfURI(uri string) (string, bool) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" || (u.Host != "" && u.Host != "localhost") {
		return "", false
	}
	if !strings.HasPrefix(u.Path, "/") {
		return "", false
	}
	return u.Path, true
}

// ColorScheme is the color-scheme setting of xdg-desktop-portal.
type ColorScheme uint32

// The color schemes.
const (
	NoPreference ColorScheme = iota
	PreferDark
	PreferLight
)

// schemeOfName reads the color-scheme of gsettings: "prefer-dark",
// "prefer-light" or "default".
func schemeOfName(name string) ColorScheme {
	switch strings.Trim(name, "'\n ") {
	case "prefer-dark":
		return PreferDark
	case "prefer-light":
		return PreferLight
	}
	return NoPreference
}
