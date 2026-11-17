package wayland

import (
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/dbus"
)

// exportRoundtrips bounds the wait for the handle of xdg-foreign.
const exportRoundtrips = 4

// exported is a window exported through xdg-foreign, so that another client,
// the portal, can put its dialog in front of it.
type exported struct {
	proxy  zxdgExportedV2
	handle string
}

// zxdgExportedV2Handle implements zxdgExportedV2Listener.
func (e *exported) zxdgExportedV2Handle(handle string) { e.handle = handle }

// ShowFileDialog implements platform.Window, through xdg-desktop-portal.
func (w *Window) ShowFileDialog(opts platform.FileDialogOptions, done func([]string, error)) {
	parent := ""
	if h := w.exportHandle(); h != "" {
		parent = "wayland:" + h
	}
	go func() { done(dbus.ShowFileDialog(parent, opts)) }()
}

// exportHandle returns the handle of xdg-foreign for w, or "" if the
// compositor has none. The dialog then floats on its own.
func (w *Window) exportHandle() string {
	p := w.p
	if p.exporter == 0 || w.surface == 0 {
		return ""
	}
	if w.export == nil {
		w.export = &exported{proxy: p.exporter.exportToplevel(p.c, w.surface)}
		p.c.listenZxdgExportedV2(w.export.proxy, w.export)
	}
	for range exportRoundtrips {
		if w.export.handle != "" {
			break
		}
		if err := p.c.roundtrip(); err != nil {
			return ""
		}
	}
	return w.export.handle
}
