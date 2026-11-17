package x11

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// Handles. All of them are C pointers or XIDs.
type (
	// Display is the Display* of a connection.
	Display uintptr
	// XID identifies a window or cursor on the server.
	XID uintptr
	// Atom is an interned name.
	Atom uintptr
)

// Event types, masks and other values of X.h.
const (
	KeyPress         = 2
	KeyRelease       = 3
	ButtonPress      = 4
	ButtonRelease    = 5
	MotionNotify     = 6
	EnterNotify      = 7
	LeaveNotify      = 8
	FocusIn          = 9
	FocusOut         = 10
	Expose           = 12
	DestroyNotify    = 17
	UnmapNotify      = 18
	MapNotify        = 19
	ConfigureNotify  = 22
	PropertyNotify   = 28
	SelectionClear   = 29
	SelectionRequest = 30
	SelectionNotify  = 31
	ClientMessage    = 33

	KeyPressMask             = 1 << 0
	KeyReleaseMask           = 1 << 1
	ButtonPressMask          = 1 << 2
	ButtonReleaseMask        = 1 << 3
	EnterWindowMask          = 1 << 4
	LeaveWindowMask          = 1 << 5
	PointerMotionMask        = 1 << 6
	ExposureMask             = 1 << 15
	StructureNotifyMask      = 1 << 17
	SubstructureNotifyMask   = 1 << 19
	SubstructureRedirectMask = 1 << 20
	FocusChangeMask          = 1 << 21
	PropertyChangeMask       = 1 << 22

	CWBackPixmap  = 1 << 0
	CWBorderPixel = 1 << 3
	CWBitGravity  = 1 << 4
	CWEventMask   = 1 << 11

	InputOutput      = 1
	CopyFromParent   = 0
	NorthWestGravity = 1

	NotifyNormal       = 0
	NotifyWhileGrabbed = 3
	NotifyPointer      = 5

	PropModeReplace = 0
	AnyPropertyType = 0
	XA_ATOM         = 4
	XA_CARDINAL     = 6
	XA_STRING       = 31
	XA_WM_NAME      = 39
	XA_WM_CLASS     = 67
	CurrentTime     = 0

	QueuedAfterFlush = 2

	// LC_CTYPE is from locale.h of glibc.
	LC_CTYPE = 0

	XIMPreeditNothing = 0x0008
	XIMStatusNothing  = 0x0400
	XBufferOverflow   = -1
	XLookupNone       = 1

	// The font cursors of cursorfont.h.
	XC_hand2             = 60
	XC_sb_h_double_arrow = 108
	XC_sb_v_double_arrow = 116
	XC_left_ptr          = 68
	XC_xterm             = 152

	XkbUseCoreKbd = 0x0100
)

// XEvent is the union of every event. The functions below read it as the
// structure of its type.
type XEvent struct {
	Type int32
	_    [47]int32
}

// XAnyEvent starts every event but XErrorEvent.
type XAnyEvent struct {
	Type      int32
	Serial    uintptr
	SendEvent int32
	Display   Display
	Window    XID
}

// XKeyEvent is also XButtonEvent and XMotionEvent, whose keycode is the button
// and the hint.
type XKeyEvent struct {
	XAnyEvent
	Root, Subwindow XID
	Time            uintptr
	X, Y            int32
	XRoot, YRoot    int32
	State           uint32
	Keycode         uint32
	SameScreen      int32
}

// XCrossingEvent is an EnterNotify or LeaveNotify.
type XCrossingEvent struct {
	XAnyEvent
	Root, Subwindow XID
	Time            uintptr
	X, Y            int32
	XRoot, YRoot    int32
	Mode            int32
	Detail          int32
	SameScreen      int32
	Focus           int32
	State           uint32
}

// XFocusChangeEvent is a FocusIn or FocusOut.
type XFocusChangeEvent struct {
	XAnyEvent
	Mode   int32
	Detail int32
}

// XConfigureEvent is a ConfigureNotify.
type XConfigureEvent struct {
	XAnyEvent
	Target           XID
	X, Y             int32
	Width, Height    int32
	BorderWidth      int32
	Above            XID
	OverrideRedirect int32
}

// XClientMessageEvent is a ClientMessage.
type XClientMessageEvent struct {
	XAnyEvent
	MessageType Atom
	Format      int32
	Data        [5]uintptr
}

// XPropertyEvent is a PropertyNotify.
type XPropertyEvent struct {
	XAnyEvent
	Atom  Atom
	Time  uintptr
	State int32
}

// XSelectionRequestEvent is a SelectionRequest. Its window is the owner.
type XSelectionRequestEvent struct {
	XAnyEvent
	Requestor XID
	Selection Atom
	Target    Atom
	Property  Atom
	Time      uintptr
}

// XSelectionEvent is a SelectionNotify. Its window is the requestor.
type XSelectionEvent struct {
	XAnyEvent
	Selection Atom
	Target    Atom
	Property  Atom
	Time      uintptr
}

// XSelectionClearEvent is a SelectionClear.
type XSelectionClearEvent struct {
	XAnyEvent
	Selection Atom
	Time      uintptr
}

// XSetWindowAttributes is what XCreateWindow takes.
type XSetWindowAttributes struct {
	BackgroundPixmap   uintptr
	BackgroundPixel    uintptr
	BorderPixmap       uintptr
	BorderPixel        uintptr
	BitGravity         int32
	WinGravity         int32
	BackingStore       int32
	BackingPlanes      uintptr
	BackingPixel       uintptr
	SaveUnder          int32
	EventMask          uintptr
	DoNotPropagateMask uintptr
	OverrideRedirect   int32
	Colormap           XID
	Cursor             XID
}

// as reads ev as the event structure T.
func as[T any](ev *XEvent) *T {
	// Every structure is at most as large as the union, and shares its alignment.
	return (*T)(unsafe.Pointer(ev))
}

// call runs a C function of Xlib. The caller's pointers converted to uintptr stay
// alive for the call, like those of syscall.SyscallN.
//
//go:uintptrescapes
func call(fn uintptr, args ...uintptr) uintptr {
	r, _, _ := purego.SyscallN(fn, args...)
	return r
}

// cstring returns s as a NUL-terminated C string.
func cstring(s string) *byte {
	b := make([]byte, len(s)+1)
	copy(b, s)
	return &b[0]
}

// ptr converts a pointer for call, and moves what it points to off the stack.
func ptr[T any](p *T) uintptr {
	kept.slots[kept.next%uint8(len(kept.slots))] = unsafe.Pointer(p)
	kept.next++
	return uintptr(unsafe.Pointer(p))
}

// kept holds the latest arguments of ptr. A uintptr hides a pointer from Go in two
// ways, and go:uintptrescapes helps only with a conversion written in the call
// itself, not one inside ptr. The stack moves as it grows, so that C would write
// to the old copy: storing the pointer here moves the object to the heap. And the
// collector may free an object that nothing refers to during the call, with what
// its fields point to: this keeps it until later calls have pushed it out, and no
// call has as many pointers as there are slots. It belongs to the UI goroutine, like
// every call of this package, and it is the one mutable variable here.
var kept struct {
	next  uint8
	slots [16]unsafe.Pointer
}

// xlib holds every function gowaku calls. Loading fills all of it or reports what
// is missing, so that a call never goes through a nil pointer.
type xlib struct {
	XInitThreads, XOpenDisplay, XCloseDisplay, XSetErrorHandler uintptr
	XDefaultScreen, XRootWindow, XConnectionNumber              uintptr
	XPending, XEventsQueued, XNextEvent, XFlush                 uintptr
	XCreateWindow, XDestroyWindow, XMapWindow, XSelectInput     uintptr
	XInternAtom, XChangeProperty, XGetWindowProperty, XFree     uintptr
	XSetWMProtocols, XFilterEvent, XSendEvent                   uintptr
	XLookupKeysym, XLookupString, Xutf8LookupString             uintptr
	XSetSelectionOwner, XConvertSelection                       uintptr
	XCreateFontCursor, XDefineCursor, XFreeCursor               uintptr
	XUngrabPointer, XGrabPointer                                uintptr
	XkbSetDetectableAutoRepeat                                  uintptr
	XSupportsLocale, XSetLocaleModifiers                        uintptr
	XOpenIM, XCloseIM, XCreateIC, XDestroyIC, XGetICValues      uintptr
	XSetICFocus, XUnsetICFocus, Xutf8ResetIC                    uintptr
	XTranslateCoordinates, XMoveWindow                          uintptr
	XMatchVisualInfo, XCreateColormap, XFreeColormap            uintptr
	XDeleteProperty                                             uintptr

	setlocale uintptr
}

type libraries struct{ x11, c uintptr }

// openLibraries opens the libraries once per process.
var openLibraries = sync.OnceValues(func() (libraries, error) {
	x, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return libraries{}, fmt.Errorf("x11: load libX11.so.6: %w", err)
	}
	c, err := purego.Dlopen("libc.so.6", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return libraries{}, fmt.Errorf("x11: load libc.so.6: %w", err)
	}
	return libraries{x, c}, nil
})

func (x *xlib) load() error {
	libs, err := openLibraries()
	if err != nil {
		return err
	}
	for _, s := range []struct {
		name string
		proc *uintptr
	}{
		{"XInitThreads", &x.XInitThreads}, {"XOpenDisplay", &x.XOpenDisplay}, {"XCloseDisplay", &x.XCloseDisplay},
		{"XSetErrorHandler", &x.XSetErrorHandler}, {"XDefaultScreen", &x.XDefaultScreen}, {"XRootWindow", &x.XRootWindow},
		{"XConnectionNumber", &x.XConnectionNumber}, {"XPending", &x.XPending}, {"XEventsQueued", &x.XEventsQueued},
		{"XNextEvent", &x.XNextEvent}, {"XFlush", &x.XFlush}, {"XCreateWindow", &x.XCreateWindow},
		{"XDestroyWindow", &x.XDestroyWindow}, {"XMapWindow", &x.XMapWindow}, {"XSelectInput", &x.XSelectInput},
		{"XInternAtom", &x.XInternAtom}, {"XChangeProperty", &x.XChangeProperty},
		{"XGetWindowProperty", &x.XGetWindowProperty}, {"XFree", &x.XFree}, {"XSetWMProtocols", &x.XSetWMProtocols},
		{"XFilterEvent", &x.XFilterEvent}, {"XSendEvent", &x.XSendEvent}, {"XLookupKeysym", &x.XLookupKeysym},
		{"XLookupString", &x.XLookupString}, {"Xutf8LookupString", &x.Xutf8LookupString},
		{"XSetSelectionOwner", &x.XSetSelectionOwner}, {"XConvertSelection", &x.XConvertSelection},
		{"XCreateFontCursor", &x.XCreateFontCursor}, {"XDefineCursor", &x.XDefineCursor}, {"XFreeCursor", &x.XFreeCursor},
		{"XUngrabPointer", &x.XUngrabPointer}, {"XGrabPointer", &x.XGrabPointer},
		{"XkbSetDetectableAutoRepeat", &x.XkbSetDetectableAutoRepeat}, {"XSupportsLocale", &x.XSupportsLocale},
		{"XSetLocaleModifiers", &x.XSetLocaleModifiers}, {"XOpenIM", &x.XOpenIM}, {"XCloseIM", &x.XCloseIM},
		{"XCreateIC", &x.XCreateIC}, {"XDestroyIC", &x.XDestroyIC}, {"XGetICValues", &x.XGetICValues},
		{"XSetICFocus", &x.XSetICFocus}, {"XUnsetICFocus", &x.XUnsetICFocus}, {"Xutf8ResetIC", &x.Xutf8ResetIC},
		{"XTranslateCoordinates", &x.XTranslateCoordinates}, {"XMoveWindow", &x.XMoveWindow},
		{"XMatchVisualInfo", &x.XMatchVisualInfo}, {"XCreateColormap", &x.XCreateColormap},
		{"XFreeColormap", &x.XFreeColormap}, {"XDeleteProperty", &x.XDeleteProperty},
	} {
		if *s.proc, err = purego.Dlsym(libs.x11, s.name); err != nil {
			return fmt.Errorf("x11: find %s: %w", s.name, err)
		}
	}
	if x.setlocale, err = purego.Dlsym(libs.c, "setlocale"); err != nil {
		return fmt.Errorf("x11: find setlocale: %w", err)
	}
	return nil
}
