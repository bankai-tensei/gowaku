package wayland

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sys/unix"

	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/dbus"
	"github.com/bankai-tensei/gowaku/platform/internal/desktop"
	"github.com/bankai-tensei/gowaku/platform/internal/evqueue"
)

// The versions gowaku binds, at most. Each is the oldest that has everything
// gowaku uses, or the newest the bindings know where newer only adds events.
const (
	compositorVersion  = 6 // wl_surface.preferred_buffer_scale
	wmBaseVersion      = 6 // xdg_toplevel suspended state
	seatVersion        = 8 // wl_pointer.axis_value120
	outputVersion      = 2 // wl_output.scale
	dataManagerVersion = 3
)

// output is a wl_output and its scale.
type output struct {
	wlOutputNop
	proxy wlOutput
	name  uint32
	scale int32
	p     *Platform
}

// wlOutputScale implements wlOutputListener.
func (o *output) wlOutputScale(factor int32) {
	o.scale = factor
	for _, w := range o.p.windows {
		w.updateScale()
	}
}

// Platform is the Wayland implementation of platform.Platform.
type Platform struct {
	c        *conn
	registry wlRegistry
	// ext is c for the bindings outside the package, and globals every
	// global the registry offers, for them (extension_linux.go).
	ext     Conn
	globals []Global

	compositor        wlCompositor
	compositorVersion uint32
	shm               wlShm
	wmBase            xdgWmBase
	dataManager       wlDataDeviceManager
	decorations       zxdgDecorationManagerV1
	fractional        wpFractionalScaleManagerV1
	viewporter        wpViewporter
	cursorShapes      wpCursorShapeManagerV1
	textInputs        zwpTextInputManagerV3
	exporter          zxdgExporterV2
	outputs           []*output

	seat seat

	windows map[wlSurface]*Window
	lastID  platform.WindowID
	queue   evqueue.Queue

	theme *desktop.Theme
	// a11y is the AT-SPI bridge, and a11yEvents what it asked for (ADR 0043).
	a11y       *dbus.A11y
	a11yEvents []platform.Event
	// gestures is the pointer gestures of the compositor, for pinches, and
	// tablets its tablet manager, for pens.
	gestures zwpPointerGesturesV1
	tablets  zwpTabletManagerV2
	// iconManager gives toplevels their icons, where the compositor has
	// xdg-toplevel-icon-v1, or is nil.
	iconManager *iconManager
	// trayHost holds the tray icons and notifications, made on first use.
	trayHost *dbus.Trays
	// clientDecorations draws the title bar in the application even where the
	// compositor would, for trying it out: GOWAKU_WAYLAND_DECORATIONS=client.
	clientDecorations bool

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
	fds                 [2]unix.PollFd
}

var _ platform.Platform = (*Platform)(nil)

// New connects to the compositor of $WAYLAND_DISPLAY and locks the calling
// goroutine to its OS thread, which makes it the UI goroutine. Destroy releases
// the lock.
func New() (*Platform, error) {
	p := &Platform{windows: map[wlSurface]*Window{}, wakeRead: -1, wakeWrite: -1}
	runtime.LockOSThread()
	if err := p.init(); err != nil {
		p.release()
		runtime.UnlockOSThread()
		return nil, err
	}
	return p, nil
}

func (p *Platform) init() error {
	c, err := connect()
	if err != nil {
		return err
	}
	p.c = c
	p.ext = Conn{c}
	p.seat.p = p
	if p.seat.kb, err = newKeyboard(); err != nil {
		return err
	}
	p.registry = wlDisplay(c.display).getRegistry(c)
	c.listenWlRegistry(p.registry, p)
	// The first roundtrip announces the globals, the second what was bound
	// in answer: the capabilities of the seat and the scales of outputs.
	for range 2 {
		if err := c.roundtrip(); err != nil {
			return err
		}
	}
	// The seat and the manager may come in either order.
	p.seat.text.init(p)
	switch {
	case p.compositor == 0:
		return errors.New("wayland: the compositor has no wl_compositor")
	case p.wmBase == 0:
		return errors.New("wayland: the compositor has no xdg_wm_base")
	}
	p.clientDecorations = os.Getenv("GOWAKU_WAYLAND_DECORATIONS") == "client"

	var pipe [2]int
	if err := unix.Pipe2(pipe[:], unix.O_NONBLOCK|unix.O_CLOEXEC); err != nil {
		return fmt.Errorf("wayland: create wake pipe: %w", err)
	}
	p.wakeRead, p.wakeWrite = pipe[0], pipe[1]
	fd := int32(c.hot(c.lib.getFD, c.display))
	p.fds = [2]unix.PollFd{{Fd: fd, Events: unix.POLLIN}, {Fd: int32(p.wakeRead), Events: unix.POLLIN}}
	// After the pipe, which a change wakes.
	p.theme = desktop.WatchTheme(p.interrupt)
	p.a11y = dbus.NewA11y(p.interrupt)
	return nil
}

// wlRegistryGlobal implements wlRegistryListener.
func (p *Platform) wlRegistryGlobal(name uint32, iface string, version uint32) {
	c := p.c
	p.globals = append(p.globals, Global{name, iface, version})
	bind := func(in *wlInterface, newest uint32) proxy {
		return p.registry.bind(c, name, in, min(version, newest))
	}
	switch iface {
	case "wl_compositor":
		p.compositorVersion = min(version, compositorVersion)
		p.compositor = wlCompositor(bind(wlCompositorInterface, compositorVersion))
	case "wl_shm":
		p.shm = wlShm(bind(wlShmInterface, 1))
	case "xdg_wm_base":
		p.wmBase = xdgWmBase(bind(xdgWmBaseInterface, wmBaseVersion))
		c.listenXdgWmBase(p.wmBase, p)
	case "wl_seat":
		if p.seat.proxy == 0 {
			p.seat.version = min(version, seatVersion)
			p.seat.proxy = wlSeat(bind(wlSeatInterface, seatVersion))
			c.listenWlSeat(p.seat.proxy, &p.seat)
		}
	case "wl_output":
		if version < outputVersion {
			return
		}
		o := &output{proxy: wlOutput(bind(wlOutputInterface, outputVersion)), name: name, scale: 1, p: p}
		c.listenWlOutput(o.proxy, o)
		p.outputs = append(p.outputs, o)
	case "wl_data_device_manager":
		p.dataManager = wlDataDeviceManager(bind(wlDataDeviceManagerInterface, dataManagerVersion))
	case "zxdg_decoration_manager_v1":
		p.decorations = zxdgDecorationManagerV1(bind(zxdgDecorationManagerV1Interface, 1))
	case "wp_fractional_scale_manager_v1":
		p.fractional = wpFractionalScaleManagerV1(bind(wpFractionalScaleManagerV1Interface, 1))
	case "wp_viewporter":
		p.viewporter = wpViewporter(bind(wpViewporterInterface, 1))
	case "wp_cursor_shape_manager_v1":
		p.cursorShapes = wpCursorShapeManagerV1(bind(wpCursorShapeManagerV1Interface, 1))
	case "zxdg_exporter_v2":
		p.exporter = zxdgExporterV2(bind(zxdgExporterV2Interface, 1))
	case "zwp_text_input_manager_v3":
		p.textInputs = zwpTextInputManagerV3(bind(zwpTextInputManagerV3Interface, 1))
	case "zwp_tablet_manager_v2":
		p.tablets = zwpTabletManagerV2(bind(zwpTabletManagerV2Interface, 1))
		p.seat.startTablet()
	case "zwp_pointer_gestures_v1":
		p.gestures = zwpPointerGesturesV1(bind(zwpPointerGesturesV1Interface, 1))
		p.seat.startPinch()
	case "xdg_toplevel_icon_manager_v1":
		p.iconManager = &iconManager{proxy: xdgToplevelIconManagerV1(bind(xdgToplevelIconManagerV1Interface, 1))}
		c.listenXdgToplevelIconManagerV1(p.iconManager.proxy, p.iconManager)
	}
}

// wlRegistryGlobalRemove implements wlRegistryListener. Of what gowaku binds,
// only outputs come and go while it runs.
func (p *Platform) wlRegistryGlobalRemove(name uint32) {
	p.globals = slices.DeleteFunc(p.globals, func(g Global) bool { return g.Name == name })
	for i, o := range p.outputs {
		if o.name != name {
			continue
		}
		p.outputs = append(p.outputs[:i], p.outputs[i+1:]...)
		for _, w := range p.windows {
			w.leaveOutput(o.proxy)
		}
		p.c.destroy(proxy(o.proxy))
		return
	}
}

// xdgWmBasePing implements xdgWmBaseListener: the compositor checks that the
// application lives.
func (p *Platform) xdgWmBasePing(serial uint32) { p.wmBase.pong(p.c, serial) }

// outputScale returns the scale of an output, or 0 if it is unknown.
func (p *Platform) outputScale(o wlOutput) int32 {
	for _, out := range p.outputs {
		if out.proxy == o {
			return out.scale
		}
	}
	return 0
}

// Brightness implements platform.Platform.
func (p *Platform) Brightness() platform.Brightness { return p.theme.Brightness() }

// ReduceMotion implements platform.Platform: GNOME enable-animations, through
// the portal (ADR 0043 §6).
func (p *Platform) ReduceMotion() bool { return p.theme.ReduceMotion() }

// Locale implements platform.Platform: the locale of the messages, from the
// environment as gettext reads it. Nothing announces a change.
func (p *Platform) Locale() string { return desktop.Locale(os.Getenv) }

// SetModalHandler implements platform.Platform. Wayland has no modal loops: the
// compositor moves and resizes windows while the application goes on.
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
		p.seat.repeatDue(time.Now())
		if ev, ok := p.queue.Pop(); ok {
			return ev
		}
		c := p.c
		if int32(c.hot(c.lib.dispatchPending, c.display)) < 0 {
			panic("gowaku: " + c.err().Error())
		}
		if ev, ok := p.queue.Pop(); ok {
			return ev
		}
		wait := -1
		switch {
		case timeout == 0:
			wait = 0
		case timeout > 0:
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return platform.Event{}
			}
			// Round up so that a sub-millisecond rest does not spin.
			wait = int((remaining + time.Millisecond - 1) / time.Millisecond)
		}
		if due, ok := p.seat.nextRepeat(); ok {
			ms := max(int((time.Until(due)+time.Millisecond-1)/time.Millisecond), 0)
			if wait < 0 || ms < wait {
				wait = ms
			}
		}
		if !p.read(wait) && timeout == 0 {
			// Waiting may have dispatched what was already in the queue of
			// libwayland.
			ev, _ := p.queue.Pop()
			return ev
		}
	}
}

// takeAccessibility queues what the AT-SPI bridge has for the application: a
// redraw of every window once it connects, which builds the first trees, and
// the actions that readers asked for.
func (p *Platform) takeAccessibility() {
	if p.a11y.TakeStarted() {
		for _, w := range p.windows {
			p.queue.Push(platform.Event{Kind: platform.EventRedraw, Window: w.id})
		}
	}
	p.a11yEvents = p.a11y.TakeActions(p.a11yEvents[:0])
	for i := range p.a11yEvents {
		p.queue.Push(p.a11yEvents[i])
	}
}

// read waits for the connection or the wake pipe for up to ms milliseconds,
// forever if negative, and reads what arrived into the queue of libwayland. It
// reports whether anything did.
func (p *Platform) read(ms int) bool {
	c := p.c
	// Another reader would be a thread of the driver, which reads its own queue;
	// events of the default queue may have arrived in between.
	for int32(c.hot(c.lib.prepareRead, c.display)) != 0 {
		if int32(c.hot(c.lib.dispatchPending, c.display)) < 0 {
			panic("gowaku: " + c.err().Error())
		}
	}
	// A full socket leaves the rest for the next flush; EAGAIN is not an error.
	c.hot(c.lib.flush, c.display)
	p.fds[0].Revents, p.fds[1].Revents = 0, 0
	n, err := unix.Poll(p.fds[:], ms)
	if err != nil && !errors.Is(err, unix.EINTR) {
		c.hot(c.lib.cancelRead, c.display)
		panic("gowaku: poll: " + err.Error())
	}
	if n > 0 && p.fds[0].Revents != 0 {
		if int32(c.hot(c.lib.readEvents, c.display)) < 0 {
			panic("gowaku: " + c.err().Error())
		}
	} else {
		c.hot(c.lib.cancelRead, c.display)
	}
	if n > 0 && p.fds[1].Revents != 0 {
		var drain [16]byte
		// A short read leaves nothing that matters: the flag below is the state.
		_, _ = unix.Read(p.wakeRead, drain[:]) //nolint:errcheck // see above
		p.wakePending.Store(false)
		if p.woken.Swap(false) {
			p.queue.Push(platform.Event{Kind: platform.EventWake})
		}
	}
	return n > 0
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
	if p.c != nil {
		p.seat.destroy()
		c := p.c
		for _, o := range p.outputs {
			c.destroy(proxy(o.proxy))
		}
		for _, pr := range []proxy{
			proxy(p.compositor), proxy(p.shm), proxy(p.dataManager), proxy(p.registry),
		} {
			c.destroy(pr)
		}
		if p.wmBase != 0 {
			p.wmBase.destroy(c)
		}
		if p.decorations != 0 {
			p.decorations.destroy(c)
		}
		if p.fractional != 0 {
			p.fractional.destroy(c)
		}
		if p.viewporter != 0 {
			p.viewporter.destroy(c)
		}
		if p.cursorShapes != 0 {
			p.cursorShapes.destroy(c)
		}
		if p.textInputs != 0 {
			p.textInputs.destroy(c)
		}
		if p.exporter != 0 {
			p.exporter.destroy(c)
		}
		c.hot(c.lib.flush, c.display)
		c.close()
		p.c = nil
	}
	if p.seat.kb != nil {
		p.seat.kb.destroy()
		p.seat.kb = nil
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
	slog.Debug("wayland: disconnected")
}
