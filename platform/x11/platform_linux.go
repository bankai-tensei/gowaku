package x11

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"

	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/dbus"
	"github.com/bankai-tensei/gowaku/platform/internal/clip"
	"github.com/bankai-tensei/gowaku/platform/internal/desktop"
	"github.com/bankai-tensei/gowaku/platform/internal/evqueue"
	"github.com/bankai-tensei/gowaku/platform/internal/frame"
)

// maxArgs is the most any function on the path of an event takes.
const maxArgs = 6

// atoms are the names gowaku needs from the server.
type atoms struct {
	WM_PROTOCOLS, WM_DELETE_WINDOW, NET_WM_PING      Atom
	NET_WM_NAME, NET_WM_PID, UTF8_STRING, GTK_THEME  Atom
	CLIPBOARD, TARGETS, TEXT, INCR, GOWAKU_SELECTION Atom
	RESOURCE_MANAGER                                 Atom
	NET_WM_STATE, NET_WM_STATE_MODAL                 Atom
	NET_WM_WINDOW_TYPE, NET_WM_WINDOW_TYPE_DIALOG    Atom
	NET_ACTIVE_WINDOW                                Atom
	NET_WM_MOVERESIZE, MOTIF_WM_HINTS                Atom
	NET_WM_STATE_MAXIMIZED_VERT                      Atom
	NET_WM_STATE_MAXIMIZED_HORZ                      Atom
	WM_CHANGE_STATE, GTK_SHOW_WINDOW_MENU            Atom
	NET_WM_ICON, NET_WM_STATE_ABOVE                  Atom
	NET_WM_STATE_FULLSCREEN                          Atom

	XdndAware, XdndEnter, XdndPosition, XdndStatus   Atom
	XdndLeave, XdndDrop, XdndFinished, XdndSelection Atom
	XdndTypeList, XdndActionCopy, XdndActionMove     Atom
	XdndActionLink                                   Atom
}

// Platform is the X11 implementation of platform.Platform.
type Platform struct {
	x       xlib
	display Display
	screen  uintptr
	root    XID
	// helper is a window that is never shown. It owns the clipboard, which must
	// outlive any one window.
	helper XID
	atoms  atoms

	windows map[XID]*Window
	lastID  platform.WindowID
	queue   evqueue.Queue

	scale float32
	theme *desktop.Theme
	// a11y is the AT-SPI bridge, and a11yEvents what it asked for (ADR 0043).
	a11y       *dbus.A11y
	a11yEvents []platform.Event
	// xi holds XInput 2, and xiOpcode its extension, or zero without 2.4.
	xi       xi
	xiOpcode int32
	xiMinor  int32
	// trayHost holds the tray icons and notifications, made on first use.
	trayHost *dbus.Trays
	cursors  [5]XID
	// edgeCursors are the cursors of the edges of custom frames, made on
	// first use.
	edgeCursors map[frame.Edge]XID
	im          uintptr

	// wakeRead and wakeWrite are the self-pipe that interrupts the wait, and
	// wakePending says that a byte is in it. woken says that Wake asked for
	// the interruption, not a watcher of the desktop, whose state the loop
	// reads by itself. wakeClosed, under wakeMu, keeps writes off the closed
	// pipe: the watchers outlive the Platform.
	wakeRead, wakeWrite int
	wakePending         atomic.Bool
	woken               atomic.Bool
	wakeMu              sync.Mutex
	wakeClosed          bool
	wakeByte            [1]byte

	fds [2]unix.PollFd

	// clipboard is what this application put on the clipboard while it owns
	// it, and offers its MIME types. sel is a read in progress, and transfers
	// the INCR transfers to other programs. atomCache holds the atoms of the
	// MIME types.
	clipboard      platform.ClipboardContent
	clipboardOwned bool
	offers         []clip.Offer
	sel            selection
	transfers      []*transfer
	atomCache      map[string]Atom
	// dragging is the drag this application started, and dragOffers its
	// data, which the helper serves as the XdndSelection.
	dragging   *dragState
	dragOffers []clip.Offer

	// event is a field so that passing its address to Xlib does not allocate.
	event XEvent
	// args holds the arguments of hot.
	args [maxArgs]uintptr
}

var _ platform.Platform = (*Platform)(nil)

// New connects to the X server of $DISPLAY and locks the calling goroutine to its
// OS thread, which makes it the UI goroutine. Destroy releases the lock.
func New() (*Platform, error) {
	p := &Platform{windows: make(map[XID]*Window), wakeRead: -1, wakeWrite: -1}
	if err := p.x.load(); err != nil {
		return nil, err
	}
	runtime.LockOSThread()
	if err := p.init(); err != nil {
		p.release()
		runtime.UnlockOSThread()
		return nil, err
	}
	return p, nil
}

func (p *Platform) init() error {
	x := &p.x
	// Vulkan drivers talk to the same connection from their own threads.
	call(x.XInitThreads)
	p.setupLocale()
	p.display = Display(call(x.XOpenDisplay, 0))
	if p.display == 0 {
		return errors.New("x11: cannot connect to the X server: is DISPLAY set?")
	}
	// The default handler ends the process for any error, and some are harmless:
	// a clipboard client can vanish between its request and the answer. The
	// callback stays for the life of the process, like all of purego's.
	call(x.XSetErrorHandler, purego.NewCallback(onXError))
	p.screen = call(x.XDefaultScreen, uintptr(p.display))
	p.root = XID(call(x.XRootWindow, uintptr(p.display), p.screen))
	p.internAtoms()

	var detectable int32
	call(x.XkbSetDetectableAutoRepeat, uintptr(p.display), 1, ptr(&detectable))

	var err error
	if p.helper, err = p.createWindow(1, 1, PropertyChangeMask); err != nil {
		return err
	}
	// Desktops announce a new scale through this property of the root window.
	call(x.XSelectInput, uintptr(p.display), uintptr(p.root), PropertyChangeMask)
	p.scale = scaleOf(p.readResources())
	for i, shape := range []uintptr{XC_left_ptr, XC_xterm, XC_hand2, XC_sb_h_double_arrow, XC_sb_v_double_arrow} {
		p.cursors[i] = XID(call(x.XCreateFontCursor, uintptr(p.display), shape))
	}
	p.openIM()

	var pipe [2]int
	if err := unix.Pipe2(pipe[:], unix.O_NONBLOCK|unix.O_CLOEXEC); err != nil {
		return fmt.Errorf("x11: create wake pipe: %w", err)
	}
	p.wakeRead, p.wakeWrite = pipe[0], pipe[1]
	connection := int32(call(x.XConnectionNumber, uintptr(p.display)))
	p.fds = [2]unix.PollFd{{Fd: connection, Events: unix.POLLIN}, {Fd: int32(p.wakeRead), Events: unix.POLLIN}}
	// After the pipe, which a change wakes.
	p.theme = desktop.WatchTheme(p.interrupt)
	p.a11y = dbus.NewA11y(p.interrupt)
	p.initXi()
	return nil
}

func onXError(_, event uintptr) uintptr {
	// The event lives for the call. Its error code is at offset 32 and the
	// request at 33 (abi_linux_test.go), and it is read as the bytes it is.
	raw := *(*[34]byte)(*(*unsafe.Pointer)(unsafe.Pointer(&event)))
	slog.Debug("x11: protocol error", "code", raw[32], "request", raw[33])
	return 0
}

func (p *Platform) internAtoms() {
	a := &p.atoms
	for _, s := range []struct {
		name string
		atom *Atom
	}{
		{"WM_PROTOCOLS", &a.WM_PROTOCOLS}, {"WM_DELETE_WINDOW", &a.WM_DELETE_WINDOW}, {"_NET_WM_PING", &a.NET_WM_PING},
		{"_NET_WM_NAME", &a.NET_WM_NAME}, {"_NET_WM_PID", &a.NET_WM_PID}, {"UTF8_STRING", &a.UTF8_STRING},
		{"_GTK_THEME_VARIANT", &a.GTK_THEME}, {"CLIPBOARD", &a.CLIPBOARD}, {"TARGETS", &a.TARGETS}, {"TEXT", &a.TEXT},
		{"INCR", &a.INCR}, {"GOWAKU_SELECTION", &a.GOWAKU_SELECTION}, {"RESOURCE_MANAGER", &a.RESOURCE_MANAGER},
		{"_NET_WM_STATE", &a.NET_WM_STATE}, {"_NET_WM_STATE_MODAL", &a.NET_WM_STATE_MODAL},
		{"_NET_WM_WINDOW_TYPE", &a.NET_WM_WINDOW_TYPE}, {"_NET_WM_WINDOW_TYPE_DIALOG", &a.NET_WM_WINDOW_TYPE_DIALOG},
		{"_NET_ACTIVE_WINDOW", &a.NET_ACTIVE_WINDOW},
		{"_NET_WM_MOVERESIZE", &a.NET_WM_MOVERESIZE}, {"_MOTIF_WM_HINTS", &a.MOTIF_WM_HINTS},
		{"_NET_WM_STATE_MAXIMIZED_VERT", &a.NET_WM_STATE_MAXIMIZED_VERT},
		{"_NET_WM_STATE_MAXIMIZED_HORZ", &a.NET_WM_STATE_MAXIMIZED_HORZ},
		{"WM_CHANGE_STATE", &a.WM_CHANGE_STATE}, {"_GTK_SHOW_WINDOW_MENU", &a.GTK_SHOW_WINDOW_MENU},
		{"_NET_WM_ICON", &a.NET_WM_ICON}, {"_NET_WM_STATE_ABOVE", &a.NET_WM_STATE_ABOVE},
		{"_NET_WM_STATE_FULLSCREEN", &a.NET_WM_STATE_FULLSCREEN},
		{"XdndAware", &a.XdndAware}, {"XdndEnter", &a.XdndEnter}, {"XdndPosition", &a.XdndPosition},
		{"XdndStatus", &a.XdndStatus}, {"XdndLeave", &a.XdndLeave}, {"XdndDrop", &a.XdndDrop},
		{"XdndFinished", &a.XdndFinished}, {"XdndSelection", &a.XdndSelection}, {"XdndTypeList", &a.XdndTypeList},
		{"XdndActionCopy", &a.XdndActionCopy}, {"XdndActionMove", &a.XdndActionMove},
		{"XdndActionLink", &a.XdndActionLink},
	} {
		*s.atom = Atom(call(p.x.XInternAtom, uintptr(p.display), ptr(cstring(s.name)), 0))
	}
}

// createWindow makes a window that is not shown yet.
func (p *Platform) createWindow(width, height int, events uintptr) (XID, error) {
	attrs := XSetWindowAttributes{BitGravity: NorthWestGravity, EventMask: events}
	// No background: the GPU covers the whole window, and a fill would flash.
	id := XID(call(p.x.XCreateWindow, uintptr(p.display), uintptr(p.root), 0, 0, uintptr(width), uintptr(height), 0,
		CopyFromParent, InputOutput, CopyFromParent, CWBackPixmap|CWBorderPixel|CWBitGravity|CWEventMask, ptr(&attrs)))
	if id == 0 {
		return 0, errors.New("x11: create window")
	}
	return id, nil
}

// readProperty returns the value of a property with its type. It reads up to a
// few megabytes, which is more than anything but an INCR transfer has.
func (p *Platform) readProperty(w XID, property Atom, remove bool) (value []byte, kind Atom) {
	const maxLongs = 1 << 20
	var format int32
	var items, after uintptr
	var data *byte
	del := uintptr(0)
	if remove {
		del = 1
	}
	r := call(p.x.XGetWindowProperty, uintptr(p.display), uintptr(w), uintptr(property), 0, maxLongs, del, AnyPropertyType,
		ptr(&kind), ptr(&format), ptr(&items), ptr(&after), ptr(&data))
	if int32(r) != 0 || data == nil {
		return nil, 0
	}
	defer call(p.x.XFree, uintptr(unsafe.Pointer(data)))
	// Format 32 properties hold longs, as Xlib documents, whatever their width.
	size := int(items) * int(format) / 8
	if format == 32 {
		size = int(items) * int(unsafe.Sizeof(uintptr(0)))
	}
	// data holds size bytes until XFree.
	return bytes.Clone(unsafe.Slice(data, size)), kind
}

func (p *Platform) readResources() string {
	value, _ := p.readProperty(p.root, p.atoms.RESOURCE_MANAGER, false)
	return string(value)
}

// Brightness implements platform.Platform.
func (p *Platform) Brightness() platform.Brightness { return p.theme.Brightness() }

// ReduceMotion implements platform.Platform: GNOME enable-animations, through
// the portal (ADR 0043 §6).
func (p *Platform) ReduceMotion() bool { return p.theme.ReduceMotion() }

// Locale implements platform.Platform: the locale of the messages, from the
// environment as gettext reads it. Nothing announces a change.
func (p *Platform) Locale() string { return desktop.Locale(os.Getenv) }

// SetModalHandler implements platform.Platform. X11 has no modal loops: the window
// manager moves and resizes windows while the application goes on.
func (p *Platform) SetModalHandler(func()) {}

// Wake implements platform.Platform. After Destroy it does nothing.
func (p *Platform) Wake() {
	// Before the byte, so that the reader that takes it sees the flag.
	p.woken.Store(true)
	p.interrupt()
}

// interrupt makes a blocked WaitEvent look again at the queue and at the
// watchers of the desktop, without an EventWake. It is safe from any
// goroutine, and does nothing after Destroy.
func (p *Platform) interrupt() {
	if p.wakePending.Swap(true) {
		return
	}
	p.wakeMu.Lock()
	defer p.wakeMu.Unlock()
	if p.wakeClosed {
		return
	}
	if _, err := unix.Write(p.wakeWrite, p.wakeByte[:]); err != nil {
		panic("gowaku: write wake pipe: " + err.Error())
	}
}

// WaitEvent implements platform.Platform.
func (p *Platform) WaitEvent(timeout time.Duration) platform.Event {
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	for {
		if p.theme.TakeChange() {
			p.queue.Push(platform.Event{Kind: platform.EventBrightness})
		}
		if p.theme.TakeSettingsChange() {
			p.queue.Push(platform.Event{Kind: platform.EventSettings})
		}
		p.takeAccessibility()
		if ev, ok := p.queue.Pop(); ok {
			return ev
		}
		if p.dispatchPending() {
			continue
		}
		if timeout == 0 && !p.pollOnce(0) {
			return platform.Event{}
		}
		if timeout == 0 {
			continue
		}
		wait := -1
		if timeout > 0 {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return platform.Event{}
			}
			// Round up so that a sub-millisecond rest does not spin.
			wait = int((remaining + time.Millisecond - 1) / time.Millisecond)
		}
		p.pollOnce(wait)
	}
}

// takeAccessibility queues what the AT-SPI bridge has for the application: a
// redraw of every window once it connects, which builds the first trees, and
// the actions that readers asked for.
func (p *Platform) takeAccessibility() {
	if p.a11y.TakeStarted() {
		for _, w := range p.windows {
			w.updateOrigin(nil)
			p.queue.Push(platform.Event{Kind: platform.EventRedraw, Window: w.id})
		}
	}
	p.a11yEvents = p.a11y.TakeActions(p.a11yEvents[:0])
	for i := range p.a11yEvents {
		p.queue.Push(p.a11yEvents[i])
	}
}

// dispatchPending handles what Xlib holds and reports whether there was any.
func (p *Platform) dispatchPending() bool {
	x := &p.x
	found := false
	// This also flushes what the application has asked of the server.
	for p.hot(x.XPending, uintptr(p.display)) != 0 {
		p.hot(x.XNextEvent, uintptr(p.display), ptr(&p.event))
		found = true
		// Input methods take the events they need here.
		if p.hot(x.XFilterEvent, ptr(&p.event), 0)&0xFFFFFFFF != 0 {
			continue
		}
		p.dispatch(&p.event)
	}
	return found
}

// pollOnce waits for the X connection or the wake pipe for up to ms milliseconds,
// forever if negative. It reports whether anything arrived.
func (p *Platform) pollOnce(ms int) bool {
	p.fds[0].Revents, p.fds[1].Revents = 0, 0
	n, err := unix.Poll(p.fds[:], ms)
	if err != nil {
		if errors.Is(err, unix.EINTR) {
			return false
		}
		panic("gowaku: poll: " + err.Error())
	}
	if n == 0 {
		return false
	}
	if p.fds[1].Revents != 0 {
		var drain [16]byte
		// A short read leaves nothing that matters: the flag below is the state.
		_, _ = unix.Read(p.wakeRead, drain[:]) //nolint:errcheck // see above
		p.wakePending.Store(false)
		if p.woken.Swap(false) {
			p.queue.Push(platform.Event{Kind: platform.EventWake})
		}
	}
	return true
}

// hot is call for what runs for every event. The arguments of call escape to the
// heap, because the C call needs them there. Copying them to a field instead
// keeps them on the stack of the caller. Pointers among them must lead to memory
// that outlives the call by itself, which means fields of the Platform or Window.
func (p *Platform) hot(fn uintptr, args ...uintptr) uintptr {
	n := copy(p.args[:], args)
	r, _, _ := purego.SyscallN(fn, p.args[:n]...)
	return r
}

// Destroy implements platform.Platform.
func (p *Platform) Destroy() {
	for _, w := range p.windows {
		w.Close()
	}
	p.a11y.Close()
	if p.trayHost != nil {
		p.trayHost.Close()
	}
	p.release()
	runtime.UnlockOSThread()
}

func (p *Platform) release() {
	x := &p.x
	if p.display != 0 {
		if p.im != 0 {
			call(x.XCloseIM, p.im)
		}
		for _, c := range p.cursors {
			if c != 0 {
				call(x.XFreeCursor, uintptr(p.display), uintptr(c))
			}
		}
		if p.helper != 0 {
			call(x.XDestroyWindow, uintptr(p.display), uintptr(p.helper))
		}
		call(x.XCloseDisplay, uintptr(p.display))
		p.display = 0
	}
	p.wakeMu.Lock()
	p.wakeClosed = true
	p.wakeMu.Unlock()
	for _, fd := range []int{p.wakeRead, p.wakeWrite} {
		if fd >= 0 {
			// Nothing depends on the result at the end of the process.
			_ = unix.Close(fd) //nolint:errcheck // see above
		}
	}
}

// dispatch turns one X event into platform events.
func (p *Platform) dispatch(ev *XEvent) {
	header := as[XAnyEvent](ev)
	switch ev.Type {
	case SelectionRequest:
		p.answerSelection(as[XSelectionRequestEvent](ev))
		return
	case SelectionClear:
		p.clipboardOwned, p.clipboard, p.offers = false, platform.ClipboardContent{}, nil
		return
	case SelectionNotify:
		p.takeSelection(as[XSelectionEvent](ev))
		return
	case GenericEvent:
		p.genericEvent(ev)
		return
	case ClientMessage:
		if header.Window == p.helper {
			p.dragMessage(as[XClientMessageEvent](ev))
			return
		}
	case PropertyNotify:
		e := as[XPropertyEvent](ev)
		switch {
		case header.Window == p.root && e.Atom == p.atoms.RESOURCE_MANAGER:
			p.rescale()
		case header.Window == p.helper:
			p.takePiece(e)
		case p.windows[header.Window] != nil && e.Atom == p.atoms.NET_WM_STATE:
			if w := p.windows[header.Window]; w.readState() {
				// Maximized shows on the title bar, and fullscreen hides it.
				p.queue.Push(w.resizeEvent())
			}
		default:
			p.sendPiece(e)
		}
		return
	}
	if w := p.windows[header.Window]; w != nil {
		w.dispatch(ev)
	}
}

// rescale picks up a new Xft.dpi.
func (p *Platform) rescale() {
	scale := scaleOf(p.readResources())
	if scale == p.scale {
		return
	}
	p.scale = scale
	for _, w := range p.windows {
		w.a11y.Moved()
		p.queue.Push(w.resizeEvent())
	}
}
