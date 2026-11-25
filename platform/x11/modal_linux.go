package x11

import (
	"unsafe"

	"github.com/bankai-tensei/gowaku/platform"
)

// The predefined atoms of a window that belongs to another.
const (
	XA_WINDOW           = 33
	XA_WM_TRANSIENT_FOR = 68
)

// own makes w belong to the owner in opts, before it is mapped: transient for
// it, a dialog to the window manager, and modal if asked (EWMH). The window
// manager places it over the owner and keeps it in front; the application
// keeps the input from the owner. See docs/ADR/0047-modal-windows.md.
func (w *Window) own(opts *platform.WindowOptions) {
	owner, ok := opts.Owner.(*Window)
	if !ok || owner == nil || owner.xid == 0 {
		return
	}
	a := &w.platform.atoms
	transient := [1]uintptr{uintptr(owner.xid)}
	w.setProperty(XA_WM_TRANSIENT_FOR, XA_WINDOW, 32, unsafe.Pointer(&transient[0]), 1)
	kind := [1]uintptr{uintptr(a.NET_WM_WINDOW_TYPE_DIALOG)}
	w.setProperty(a.NET_WM_WINDOW_TYPE, XA_ATOM, 32, unsafe.Pointer(&kind[0]), 1)
	// initialState marks it modal, with the other states.
}

// Activate implements platform.Window. It asks the window manager, as a pager
// would, which most window managers grant to the application of the window.
func (w *Window) Activate() {
	if w.xid == 0 {
		return
	}
	p := w.platform
	m := XClientMessageEvent{MessageType: p.atoms.NET_ACTIVE_WINDOW, Format: 32}
	m.Type, m.Display, m.Window = ClientMessage, p.display, w.xid
	// Source 1 is an application.
	m.Data[0] = 1
	call(p.x.XSendEvent, uintptr(p.display), uintptr(p.root), 0, SubstructureNotifyMask|SubstructureRedirectMask, ptr(&m))
	call(p.x.XFlush, uintptr(p.display))
}

// SetVisible implements platform.Window: XWithdrawWindow takes the window
// from the window manager, task bar and all, as ICCCM 4.1.4 says, and
// XMapWindow gives it back.
func (w *Window) SetVisible(visible bool) {
	if w.xid == 0 {
		return
	}
	p := w.platform
	if visible {
		call(p.x.XMapWindow, uintptr(p.display), uintptr(w.xid))
		w.Activate()
		return
	}
	call(p.x.XWithdrawWindow, uintptr(p.display), uintptr(w.xid), p.screen)
	call(p.x.XFlush, uintptr(p.display))
}
