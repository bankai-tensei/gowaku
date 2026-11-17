package x11

import (
	"log/slog"
	"unsafe"
)

// setupLocale makes Xlib see the locale of the user. XIM picks its input method
// by locale, and $XMODIFIERS says which one the desktop runs.
func (p *Platform) setupLocale() {
	x := &p.x
	call(x.setlocale, LC_CTYPE, ptr(cstring("")))
	if call(x.XSupportsLocale)&0xFFFFFFFF == 0 {
		// Xlib cannot do this locale, as with the plain C locale of a service, but
		// UTF-8 text does not depend on it.
		call(x.setlocale, LC_CTYPE, ptr(cstring("C.UTF-8")))
	}
	call(x.XSetLocaleModifiers, ptr(cstring("")))
}

// openIM connects to the input method. Without one, keys still produce their text.
func (p *Platform) openIM() {
	p.im = call(p.x.XOpenIM, uintptr(p.display), 0, 0, 0)
	if p.im == 0 {
		slog.Debug("x11: no input method")
	}
}

// createIC gives w an input context. Each string below is an XN name of Xlib.h.
// XCreateIC is variadic, which is fine here: the arguments are integers and
// pointers, for which the C calling convention stays the same.
func (w *Window) createIC() {
	p := w.platform
	if p.im == 0 {
		return
	}
	w.ic = call(p.x.XCreateIC, p.im,
		ptr(cstring("inputStyle")), XIMPreeditNothing|XIMStatusNothing,
		ptr(cstring("clientWindow")), uintptr(w.xid),
		ptr(cstring("focusWindow")), uintptr(w.xid),
		0)
	if w.ic == 0 {
		slog.Debug("x11: no input context")
		return
	}
	// The input method can ask for more events than a window selects.
	var extra uintptr
	if r := call(p.x.XGetICValues, w.ic, ptr(cstring("filterEvents")), ptr(&extra), 0); r == 0 && extra != 0 {
		call(p.x.XSelectInput, uintptr(p.display), uintptr(w.xid), windowEvents|extra)
	}
	call(p.x.XUnsetICFocus, w.ic)
}

// syncICFocus lets the input method see keys when the window has the keyboard
// and wants conversion.
func (w *Window) syncICFocus() {
	if w.ic == 0 {
		return
	}
	if w.focused && w.imeEnabled && !w.imeSecure {
		call(w.platform.x.XSetICFocus, w.ic)
	} else {
		call(w.platform.x.XUnsetICFocus, w.ic)
	}
}

// SetIMEEnabled implements platform.Window.
func (w *Window) SetIMEEnabled(enabled bool) {
	w.imeEnabled = enabled
	w.syncICFocus()
}

// SetSecureInput implements platform.SecureInputWindow. Without the focus of
// its input context the input method sees no keys, and Xlib looks them up
// itself. X11 has no secure event input short of grabbing the keyboard, which
// would take it from the window manager too, so this is all it does.
func (w *Window) SetSecureInput(secure bool) {
	if secure == w.imeSecure {
		return
	}
	if secure {
		w.CancelComposition()
	}
	w.imeSecure = secure
	w.syncICFocus()
}

// SetIMECursorRect implements platform.Window. With the style of
// docs/ADR/0014-x11-platform.md the input method places its own window, so there
// is nothing to tell it.
func (w *Window) SetIMECursorRect(_, _, _, _ int) {}

// SetIMEVertical implements platform.Window. XIM has no way to say it.
func (w *Window) SetIMEVertical(bool) {}

// CancelComposition implements platform.Window.
func (w *Window) CancelComposition() {
	if w.ic == 0 {
		return
	}
	// The result is what the input method held, which the application dropped.
	if s := call(w.platform.x.Xutf8ResetIC, w.ic); s != 0 {
		call(w.platform.x.XFree, s)
	}
}

// lookupText returns the UTF-8 text of a key press. The result is only valid
// until the next one.
func (w *Window) lookupText(ev *XEvent) []byte {
	p := w.platform
	if w.ic == 0 {
		n := p.hot(p.x.XLookupString, ptr(ev), ptr(&w.lookup[0]), uintptr(len(w.lookup)), ptr(&w.keysym), 0)
		return w.lookup[:int32(n)]
	}
	n := int32(p.hot(p.x.Xutf8LookupString, w.ic, ptr(ev), ptr(&w.lookup[0]), uintptr(len(w.lookup)), ptr(&w.keysym), ptr(&w.status)))
	if w.status == XBufferOverflow {
		// n is the size it takes. A commit this long is rare and may allocate.
		big := make([]byte, n)
		n = int32(call(p.x.Xutf8LookupString, w.ic, ptr(ev), uintptr(unsafe.Pointer(&big[0])), uintptr(len(big)), ptr(&w.keysym), ptr(&w.status)))
		return big[:n]
	}
	if w.status == XLookupNone {
		return nil
	}
	return w.lookup[:n]
}
