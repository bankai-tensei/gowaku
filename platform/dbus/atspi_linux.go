package dbus

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	godbus "github.com/godbus/dbus/v5"

	"github.com/bankai-tensei/gowaku/platform"
)

const (
	a11yBusName       = "org.a11y.Bus"
	a11yBusPath       = "/org/a11y/bus"
	a11yStatus        = "org.a11y.Status"
	registryName      = "org.a11y.atspi.Registry"
	socketInterface   = "org.a11y.atspi.Socket"
	propertiesIface   = "org.freedesktop.DBus.Properties"
	introspectIface   = "org.freedesktop.DBus.Introspectable"
	eventObject       = "org.a11y.atspi.Event.Object."
	eventWindow       = "org.a11y.atspi.Event.Window."
	errUnknownObject  = "org.freedesktop.DBus.Error.UnknownObject"
	errInvalidArgs    = "org.freedesktop.DBus.Error.InvalidArgs"
	errUnknownProp    = "org.freedesktop.DBus.Error.UnknownProperty"
	errPropReadOnly   = "org.freedesktop.DBus.Error.PropertyReadOnly"
	atspiVersion      = "2.1"
	gowakuVersion     = "0.12"
	forceA11yVariable = "GOWAKU_A11Y"
)

// ref is a reference to an accessible object: the bus name of its
// application and its path.
type ref struct {
	Name string
	Path godbus.ObjectPath
}

// A11y is the AT-SPI application of a platform: one connection to the
// accessibility bus, whose root holds the windows as frames (ADR 0043 §2).
//
// NewA11y connects in the background, and only when assistive technology is
// on: org.a11y.Status says so, or GOWAKU_A11Y=1 forces it. NO_AT_BRIDGE=1
// keeps it off. Without a session bus or an accessibility bus it stays off,
// and the reason goes to slog.Debug.
//
// AT-SPI asks about the windows on goroutines of godbus. They read the trees of
// the last Update under the mutex; the actions they ask for wait in a list
// that the event loop takes with TakeActions. All other methods, and those of
// A11yWindow, belong to the UI goroutine unless they say otherwise.
type A11y struct {
	wake    func()
	appName string
	locale  string
	done    chan struct{}

	listening atomic.Bool
	// started means that the bridge connected and the event loop has not
	// heard of it yet.
	started atomic.Bool
	// hasActions means that actions wait.
	hasActions atomic.Bool

	mu sync.Mutex
	// conn is the accessibility bus, nil until registered; busName is its
	// unique name, set before registration starts.
	conn    *godbus.Conn
	busName string
	desktop ref
	windows []*A11yWindow
	// lastWindow numbers the windows, which never share a number.
	lastWindow uint16
	appID      int32
	actions    []platform.Event
	closed     bool
}

// NewA11y starts the bridge. wake runs on another goroutine when actions
// wait or the bridge connected, to make the event loop look.
func NewA11y(wake func()) *A11y {
	a := &A11y{
		wake:    wake,
		appName: filepath.Base(os.Args[0]),
		locale:  localeName(),
		done:    make(chan struct{}),
	}
	go a.start()
	return a
}

// localeName returns the locale of messages, as setlocale(LC_MESSAGES) would.
func localeName() string {
	for _, v := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if s := os.Getenv(v); s != "" {
			return s
		}
	}
	return "C"
}

func (a *A11y) start() {
	if os.Getenv("NO_AT_BRIDGE") == "1" {
		slog.Debug("dbus: accessibility off: NO_AT_BRIDGE=1")
		return
	}
	c := session()
	if c == nil {
		slog.Debug("dbus: accessibility off: no session bus")
		return
	}
	if os.Getenv(forceA11yVariable) == "1" || a11yEnabled(c) {
		a.connect(c)
		return
	}
	// Wait for a screen reader to start.
	if err := c.AddMatchSignal(
		godbus.WithMatchObjectPath(a11yBusPath),
		godbus.WithMatchInterface(propertiesIface),
		godbus.WithMatchMember("PropertiesChanged"),
	); err != nil {
		slog.Debug("dbus: accessibility off: watch status", "err", err)
		return
	}
	signals := make(chan *godbus.Signal, 8)
	c.Signal(signals)
	defer c.RemoveSignal(signals)
	// It may have turned on before the match.
	if a11yEnabled(c) {
		a.connect(c)
		return
	}
	slog.Debug("dbus: accessibility waits for assistive technology")
	for {
		select {
		case <-a.done:
			return
		case s := <-signals:
			if s.Path == a11yBusPath && s.Name == propertiesIface+".PropertiesChanged" && statusTurnedOn(s.Body) {
				a.connect(c)
				return
			}
		}
	}
}

// a11yEnabled asks org.a11y.Status whether assistive technology is on.
func a11yEnabled(c *godbus.Conn) bool {
	obj := c.Object(a11yBusName, a11yBusPath)
	for _, p := range []string{"IsEnabled", "ScreenReaderEnabled"} {
		v, err := obj.GetProperty(a11yStatus + "." + p)
		if err != nil {
			slog.Debug("dbus: read accessibility status", "property", p, "err", err)
			return false
		}
		if on, _ := v.Value().(bool); on {
			return true
		}
	}
	return false
}

// statusTurnedOn reads PropertiesChanged of org.a11y.Status.
func statusTurnedOn(body []any) bool {
	if len(body) < 2 {
		return false
	}
	if iface, _ := body[0].(string); iface != a11yStatus {
		return false
	}
	changed, _ := body[1].(map[string]godbus.Variant)
	for _, p := range []string{"IsEnabled", "ScreenReaderEnabled"} {
		if on, _ := changed[p].Value().(bool); on {
			return true
		}
	}
	return false
}

// connect opens the accessibility bus and registers the application.
func (a *A11y) connect(session *godbus.Conn) {
	var address string
	if err := session.Object(a11yBusName, a11yBusPath).Call(a11yBusName+".GetAddress", 0).Store(&address); err != nil {
		slog.Debug("dbus: accessibility off: no accessibility bus", "err", err)
		return
	}
	conn, err := godbus.Connect(address, godbus.WithHandler(handler{a}))
	if err != nil {
		slog.Debug("dbus: accessibility off: connect", "address", address, "err", err)
		return
	}
	names := conn.Names()
	if len(names) == 0 {
		slog.Debug("dbus: accessibility off: no unique name")
		_ = conn.Close() //nolint:errcheck // nothing was sent on it
		return
	}
	a.mu.Lock()
	a.busName = names[0]
	a.mu.Unlock()
	var desktop ref
	err = conn.Object(registryName, rootPath).
		Call(socketInterface+".Embed", 0, ref{names[0], rootPath}).Store(&desktop)
	if err != nil {
		slog.Debug("dbus: accessibility off: register with the registry", "err", err)
		_ = conn.Close() //nolint:errcheck // nothing depends on it
		return
	}
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		_ = conn.Close() //nolint:errcheck // the platform is gone
		return
	}
	a.conn, a.desktop = conn, desktop
	a.mu.Unlock()
	slog.Debug("dbus: accessibility on", "address", address, "name", names[0])
	a.listening.Store(true)
	a.started.Store(true)
	a.wake()
}

// TakeStarted reports, once, that the bridge connected. The event loop then
// asks every window for a redraw, which builds the first trees.
func (a *A11y) TakeStarted() bool { return a.started.Swap(false) }

// TakeActions appends the actions that readers asked for to dst, and forgets
// them. It does not allocate when none wait or dst has room.
func (a *A11y) TakeActions(dst []platform.Event) []platform.Event {
	if !a.hasActions.Load() {
		return dst
	}
	a.mu.Lock()
	dst = append(dst, a.actions...)
	clear(a.actions)
	a.actions = a.actions[:0]
	a.hasActions.Store(false)
	a.mu.Unlock()
	return dst
}

// post queues an action for the event loop. It is safe from any goroutine.
func (a *A11y) post(ev platform.Event) {
	ev.Kind = platform.EventAccessibilityAction
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return
	}
	a.actions = append(a.actions, ev)
	a.hasActions.Store(true)
	a.mu.Unlock()
	a.wake()
}

// Close leaves the accessibility bus.
func (a *A11y) Close() {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return
	}
	a.closed = true
	conn, name := a.conn, a.busName
	a.conn = nil
	a.windows = nil
	a.mu.Unlock()
	close(a.done)
	a.listening.Store(false)
	if conn == nil {
		return
	}
	// No reply is awaited: a registry that hangs must not hold up the exit.
	conn.Object(registryName, rootPath).Go(socketInterface+".Unembed", godbus.FlagNoReplyExpected, nil, ref{name, rootPath})
	_ = conn.Close() //nolint:errcheck // the process is done with it
}

// ref returns the reference of an object of the application.
func (a *A11y) refLocked(path string) ref { return ref{a.busName, godbus.ObjectPath(path)} }

// emit sends an event of AT-SPI. A failure means that the bus is gone, and
// there is nobody to tell.
func emit(conn *godbus.Conn, path, name, detail string, detail1, detail2 int32, data any) {
	if data == nil {
		data = int32(0)
	}
	_ = conn.Emit(godbus.ObjectPath(path), name, detail, detail1, detail2, //nolint:errcheck // see above
		godbus.MakeVariant(data), map[string]godbus.Variant{})
}

// eventName returns the full name of an Object event.
func eventName(member string) string { return eventObject + member }

// windowEventName returns the full name of a Window event.
func windowEventName(member string) string { return eventWindow + member }
