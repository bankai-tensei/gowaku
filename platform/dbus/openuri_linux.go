package dbus

import (
	"fmt"

	godbus "github.com/godbus/dbus/v5"
)

const openURIInterface = "org.freedesktop.portal.OpenURI"

// OpenURI asks the portal to open uri with the handler of its scheme. parent
// identifies the window that a chooser of applications stays in front of, as
// the portal wants it ("x11:XID" or "wayland:HANDLE"), or "". It returns once
// the portal has the request, without waiting for the application to start or
// for the user to choose one, and ErrUnavailable without a bus or a portal.
func OpenURI(parent, uri string) error {
	c := session()
	if c == nil {
		return ErrUnavailable
	}
	options := map[string]godbus.Variant{
		"handle_token": godbus.MakeVariant(fmt.Sprintf("gowaku%d", lastToken.Add(1))),
	}
	_, err := call(c, openURIInterface+".OpenURI", parent, uri, options)
	return err
}
