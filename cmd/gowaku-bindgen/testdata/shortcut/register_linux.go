package shortcut

import (
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/wayland"
)

// Shortcut is a global shortcut of the compositor.
type Shortcut struct {
	exampleShortcutV1Nop
	c       *wayland.Conn
	proxy   exampleShortcutV1
	pressed func()
}

// exampleShortcutV1Pressed implements exampleShortcutV1Listener. It runs inside
// Platform.WaitEvent, on the UI goroutine.
func (s *Shortcut) exampleShortcutV1Pressed(_, _ uint32) { s.pressed() }

// Register asks the compositor of p for the shortcut name while w is open, and
// calls pressed whenever it is pressed. It returns false when the compositor
// does not offer example_shortcut_manager_v1.
func Register(p *wayland.Platform, w platform.Window, name string, pressed func()) (*Shortcut, bool) {
	h, ok := w.NativeHandle().(platform.WaylandHandle)
	if !ok {
		return nil, false
	}
	for _, g := range p.Globals() {
		if g.Interface != "example_shortcut_manager_v1" {
			continue
		}
		c := p.Conn()
		m := exampleShortcutManagerV1(p.Bind(g, exampleShortcutManagerV1Interface, exampleShortcutManagerV1Version))
		s := &Shortcut{c: c, pressed: pressed}
		s.proxy = m.register(c, wayland.Proxy(h.Surface), name, "")
		listenExampleShortcutV1(c, s.proxy, s)
		m.destroy(c)
		return s, true
	}
	return nil, false
}

// Close gives the shortcut up.
func (s *Shortcut) Close() { s.proxy.destroy(s.c) }
