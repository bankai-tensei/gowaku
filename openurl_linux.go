package gowaku

import (
	"fmt"
	"os/exec"

	"github.com/bankai-tensei/gowaku/platform/dbus"
)

// urlOpeners asks the portal first, which also works inside Flatpak and Snap,
// and xdg-open after it.
func urlOpeners() openers {
	return openers{func(u string) error { return dbus.OpenURI("", u) }, xdgOpen}
}

// xdgOpen runs xdg-open with u as its one argument, not through a shell, and
// does not wait for it: with no desktop to hand u to, xdg-open runs the
// browser itself until it closes. checkURL has made sure that u starts with a
// scheme, not with a dash that xdg-open would take for an option.
func xdgOpen(u string) error {
	cmd := exec.Command("xdg-open", u)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("gowaku: xdg-open: %w", err)
	}
	go cmd.Wait() //nolint:errcheck // nothing waits for the outcome; Wait only reaps the process
	return nil
}
