package gowakutest

import (
	"slices"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
)

// host is the window of a Tester: it does nothing but keep a clipboard of its
// own, which the copy and paste of the widgets, through core.Host, and those
// of gowaku.WriteClipboard and gowaku.ReadClipboard, through
// platform.Clipboard, reach, and record the trays and notifications of
// gowaku.ShowTray and gowaku.Notify, through platform.Trays.
type host struct {
	tt            *Tester
	clip          platform.ClipboardContent
	trays         []*Tray
	notifications []Notification
	dialogs       []*FileDialog
	// noNotifications is the refusal of SetNotificationsAllowed, and
	// answers the done functions of RequestNotifications that wait for the
	// next Pump.
	noNotifications bool
	answers         []func(error)
}

var (
	_ core.Host          = (*host)(nil)
	_ platform.Clipboard = (*host)(nil)
)

// SetIMEEnabled implements core.Host.
func (*host) SetIMEEnabled(bool) {}

// SetIMECursorRect implements core.Host.
func (*host) SetIMECursorRect(geom.Rect) {}

// CancelComposition implements core.Host.
func (*host) CancelComposition() {}

// SetCursor implements core.Host.
func (*host) SetCursor(input.Cursor) {}

// ClipboardText implements core.Host.
func (h *host) ClipboardText() string { return h.clip.Text }

// SetClipboardText implements core.Host. Like a copy in a window, it replaces
// every kind the clipboard held.
func (h *host) SetClipboardText(s string) { h.clip = platform.ClipboardContent{Text: s} }

// ClipboardKinds implements platform.Clipboard.
func (h *host) ClipboardKinds() platform.ClipboardKinds { return h.clip.Kinds() }

// ReadClipboard implements platform.Clipboard.
func (h *host) ReadClipboard(kinds platform.ClipboardKinds) platform.ClipboardContent {
	var c platform.ClipboardContent
	if kinds&platform.ClipboardText != 0 {
		c.Text = h.clip.Text
	}
	if kinds&platform.ClipboardHTML != 0 {
		c.HTML = h.clip.HTML
	}
	if kinds&platform.ClipboardImage != 0 {
		c.Image = h.clip.Image
	}
	if kinds&platform.ClipboardFiles != 0 {
		c.Files = slices.Clone(h.clip.Files)
	}
	return c
}

// WriteClipboard implements platform.Clipboard.
func (h *host) WriteClipboard(c platform.ClipboardContent) {
	c.Files = slices.Clone(c.Files)
	h.clip = c
}

// Clipboard returns the text on the clipboard of the Tester, which starts
// empty: what a copy of a field.Input, or another copy of the tree, put there.
// The clipboard is the Tester's own, never that of the OS, so tests do not
// see each other's copies or the user's.
func (tt *Tester) Clipboard() string { return tt.host.clip.Text }

// SetClipboard puts s on the clipboard of the Tester, in place of all it
// held, as another program copying text: a paste then pastes s.
func (tt *Tester) SetClipboard(s string) { tt.host.SetClipboardText(s) }

// ClipboardContent returns every kind on the clipboard of the Tester, for
// what gowaku.WriteClipboard put there beyond the text: HTML, an image, files.
func (tt *Tester) ClipboardContent() core.ClipboardContent {
	return core.ClipboardContent(tt.host.ReadClipboard(platform.ClipboardText | platform.ClipboardHTML | platform.ClipboardImage | platform.ClipboardFiles))
}

// SetClipboardContent puts c on the clipboard of the Tester, in place of all
// it held, as another program copying an image or files, for what reads it
// with gowaku.ReadClipboard.
func (tt *Tester) SetClipboardContent(c core.ClipboardContent) {
	tt.host.WriteClipboard(platform.ClipboardContent(c))
}
