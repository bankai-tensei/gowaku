package wayland

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"
)

// maxArgs is WL_CLOSURE_MAX_ARGS of libwayland: no message has more.
const maxArgs = 20

// marshalDestroy is WL_MARSHAL_FLAG_DESTROY.
const marshalDestroy = 1

// proxy is a struct wl_proxy* of libwayland-client.
type proxy uintptr

// wlMessage is struct wl_message.
type wlMessage struct {
	name, signature *byte
	types           **wlInterface
}

// wlInterface is struct wl_interface. The tables live in Go memory, which the
// collector does not move, and the package variables keep them alive.
type wlInterface struct {
	name        *byte
	version     int32
	methodCount int32
	methods     *wlMessage
	eventCount  int32
	_           int32
	events      *wlMessage
}

func (in *wlInterface) set(name string, version, methodCount int, methods *wlMessage, eventCount int, events *wlMessage) {
	*in = wlInterface{
		name: cstr(name), version: int32(version),
		methodCount: int32(methodCount), methods: methods,
		eventCount: int32(eventCount), events: events,
	}
}

// nameArg passes the name of in as the string argument of wl_registry.bind.
func (in *wlInterface) nameArg() uint64 { return uint64(uintptr(unsafe.Pointer(in.name))) }

// cstr returns a string literal that ends in NUL as a C string. String data is
// never freed or moved.
func cstr(s string) *byte {
	if !strings.HasSuffix(s, "\x00") {
		panic("gowaku: wayland: C string without NUL")
	}
	return unsafe.StringData(s)
}

// cptr converts the address of C memory, which the collector does not manage,
// back to a pointer. Going through memory keeps vet from taking it for a Go
// pointer that was hidden in a uintptr.
func cptr(p uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&p)) }

// toFixed converts to wl_fixed_t, a 24.8 fixed-point number.
func toFixed(f float64) int32 { return int32(math.Round(f * 256)) }

// wlArgs is the union wl_argument array of an event.
type wlArgs [maxArgs]uint64

func (a *wlArgs) int(i int) int32     { return int32(uint32(a[i])) }
func (a *wlArgs) uint(i int) uint32   { return uint32(a[i]) }
func (a *wlArgs) fixed(i int) float64 { return float64(int32(uint32(a[i]))) / 256 }
func (a *wlArgs) object(i int) proxy  { return proxy(a[i]) }
func (a *wlArgs) string(i int) string { return goString(uintptr(a[i])) }
func (a *wlArgs) array(i int) []byte  { return arrayBytes(uintptr(a[i])) }

// wlArray is struct wl_array.
type wlArray struct {
	size, alloc uintptr
	data        unsafe.Pointer
}

// arrayBytes views a wl_array. The bytes belong to libwayland and are only valid
// during the event.
func arrayBytes(p uintptr) []byte {
	if p == 0 {
		return nil
	}
	a := (*wlArray)(cptr(p))
	if a.size == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(a.data), a.size)
}

// goString copies a C string.
func goString(p uintptr) string {
	if p == 0 {
		return ""
	}
	b := cptr(p)
	n := 0
	for *(*byte)(unsafe.Add(b, n)) != 0 {
		n++
	}
	return string(unsafe.Slice((*byte)(b), n))
}

// wlLib holds every function of libwayland-client that gowaku calls.
type wlLib struct {
	connect, disconnect, getFD, dispatchPending, prepareRead uintptr
	readEvents, cancelRead, flush, roundtrip, getError       uintptr
	marshalFlags, marshalConstructor, proxyDestroy           uintptr
	addDispatcher, getVersion                                uintptr
}

// openClient opens libwayland-client once per process.
var openClient = sync.OnceValues(func() (uintptr, error) {
	lib, err := purego.Dlopen("libwayland-client.so.0", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return 0, fmt.Errorf("wayland: load libwayland-client.so.0: %w", err)
	}
	return lib, nil
})

func (l *wlLib) load() error {
	lib, err := openClient()
	if err != nil {
		return err
	}
	for _, s := range []struct {
		name string
		proc *uintptr
	}{
		{"wl_display_connect", &l.connect}, {"wl_display_disconnect", &l.disconnect},
		{"wl_display_get_fd", &l.getFD}, {"wl_display_dispatch_pending", &l.dispatchPending},
		{"wl_display_prepare_read", &l.prepareRead}, {"wl_display_read_events", &l.readEvents},
		{"wl_display_cancel_read", &l.cancelRead}, {"wl_display_flush", &l.flush},
		{"wl_display_roundtrip", &l.roundtrip}, {"wl_display_get_error", &l.getError},
		{"wl_proxy_marshal_array_constructor_versioned", &l.marshalConstructor},
		{"wl_proxy_destroy", &l.proxyDestroy}, {"wl_proxy_add_dispatcher", &l.addDispatcher},
		{"wl_proxy_get_version", &l.getVersion},
	} {
		if *s.proc, err = purego.Dlsym(lib, s.name); err != nil {
			return fmt.Errorf("wayland: find %s: %w", s.name, err)
		}
	}
	// Since 1.20. The older call needs a separate destroy, which is the same on
	// one thread.
	l.marshalFlags, _ = purego.Dlsym(lib, "wl_proxy_marshal_array_flags") //nolint:errcheck // optional
	return nil
}

// handler is where the events of a proxy go.
type handler struct {
	listener any
	dispatch func(listener any, opcode uint32, a *wlArgs)
}

// conn is a connection to the compositor. It belongs to the UI goroutine.
type conn struct {
	lib      wlLib
	display  uintptr
	id       uintptr
	handlers map[proxy]handler

	// args holds the arguments of the request being sent, and strings the C
	// strings among them, reused from request to request.
	args    [maxArgs]uint64
	strings [maxArgs][]byte
	arrays  [maxArgs]wlArray
	// call holds the arguments of hot.
	call [8]uintptr
}

// conns finds a conn from the dispatcher, which gets its id as the
// implementation of every proxy. Each conn belongs to its UI goroutine, and
// tests run two of them at once.
var (
	conns      sync.Map // uintptr → *conn
	lastConnID atomic.Uintptr
)

// dispatcher is the one callback through which every event arrives.
var dispatcher = sync.OnceValue(func() uintptr {
	// The callback stays for the life of the process, like all of purego's.
	return purego.NewCallback(func(impl, target, opcode, _, args uintptr) uintptr {
		v, ok := conns.Load(impl)
		if !ok {
			return 0
		}
		c := v.(*conn)
		h, ok := c.handlers[proxy(target)]
		if !ok {
			return 0
		}
		h.dispatch(h.listener, uint32(opcode), (*wlArgs)(cptr(args)))
		return 0
	})
})

// connect opens the display of $WAYLAND_DISPLAY.
func connect() (*conn, error) {
	c := &conn{handlers: map[proxy]handler{}}
	if err := c.lib.load(); err != nil {
		return nil, err
	}
	c.display = c.hot(c.lib.connect, 0)
	if c.display == 0 {
		return nil, errors.New("wayland: cannot connect to the compositor: is WAYLAND_DISPLAY set?")
	}
	c.id = lastConnID.Add(1)
	conns.Store(c.id, c)
	return c, nil
}

func (c *conn) close() {
	if c.display == 0 {
		return
	}
	c.hot(c.lib.disconnect, c.display)
	conns.Delete(c.id)
	c.display = 0
}

// hot calls a C function without letting its arguments escape.
func (c *conn) hot(fn uintptr, args ...uintptr) uintptr {
	n := copy(c.call[:], args)
	r, _, _ := purego.SyscallN(fn, c.call[:n]...)
	return r
}

// listen sends the events of p to l.
func (c *conn) listen(p proxy, l any, dispatch func(any, uint32, *wlArgs)) {
	if p == 0 {
		panic("gowaku: wayland: listen on a null proxy")
	}
	c.handlers[p] = handler{l, dispatch}
	if int32(c.hot(c.lib.addDispatcher, uintptr(p), dispatcher(), c.id, 0)) != 0 {
		panic("gowaku: wayland: proxy already has a listener")
	}
}

// version returns the version of the object of p.
func (c *conn) version(p proxy) uint32 { return uint32(c.hot(c.lib.getVersion, uintptr(p))) }

// marshal sends request opcode of p with c.args, and returns the
// new object if the request makes one.
func (c *conn) marshal(p proxy, opcode uint32, iface *wlInterface, version, flags uint32) proxy {
	args := uintptr(unsafe.Pointer(&c.args[0]))
	var r uintptr
	if c.lib.marshalFlags != 0 {
		r = c.hot(c.lib.marshalFlags, uintptr(p), uintptr(opcode), uintptr(unsafe.Pointer(iface)), uintptr(version), uintptr(flags), args)
	} else {
		r = c.hot(c.lib.marshalConstructor, uintptr(p), uintptr(opcode), args, uintptr(unsafe.Pointer(iface)), uintptr(version))
		if flags&marshalDestroy != 0 {
			c.hot(c.lib.proxyDestroy, uintptr(p))
		}
	}
	if flags&marshalDestroy != 0 {
		delete(c.handlers, p)
	}
	return proxy(r)
}

// destroy frees a proxy whose interface has no destructor request.
func (c *conn) destroy(p proxy) {
	if p == 0 {
		return
	}
	delete(c.handlers, p)
	c.hot(c.lib.proxyDestroy, uintptr(p))
}

// str stores s as the C string of argument i.
func (c *conn) str(i int, s string) uint64 {
	if strings.IndexByte(s, 0) >= 0 {
		panic("gowaku: wayland: string contains a NUL byte")
	}
	b := append(append(c.strings[i][:0], s...), 0)
	c.strings[i] = b
	return uint64(uintptr(unsafe.Pointer(&b[0])))
}

// array stores b as the wl_array of argument i. libwayland copies it.
func (c *conn) array(i int, b []byte) uint64 {
	a := &c.arrays[i]
	*a = wlArray{size: uintptr(len(b)), alloc: uintptr(len(b))}
	if len(b) > 0 {
		a.data = unsafe.Pointer(&b[0])
	}
	return uint64(uintptr(unsafe.Pointer(a)))
}

// roundtrip waits until the compositor has handled every request so far.
func (c *conn) roundtrip() error {
	if int32(c.hot(c.lib.roundtrip, c.display)) < 0 {
		return c.err()
	}
	return nil
}

// err reports the error of the connection, which is fatal for it.
func (c *conn) err() error {
	code := int32(c.hot(c.lib.getError, c.display))
	return fmt.Errorf("wayland: connection failed (errno %d)", code)
}
