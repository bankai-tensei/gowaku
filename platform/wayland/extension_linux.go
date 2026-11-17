package wayland

import (
	"strconv"
	"strings"
)

// This file is the runtime of the bindings that cmd/gowaku-bindgen writes
// outside this package, for protocols that gowaku does not bind itself. They
// share the connection, the registry and the one dispatcher of the Platform,
// so their events arrive in WaitEvent like those of gowaku. Everything here
// belongs to the UI goroutine. See docs/ADR/0077-wayland-bindgen.md.

// Proxy is a struct wl_proxy* of libwayland-client: an object of the
// connection. Generated bindings define their types on it. The wl_surface of a
// window is Proxy(h.Surface) for the platform.WaylandHandle h of its
// NativeHandle.
type Proxy uintptr

// MarshalDestroy is the flag of Conn.Marshal for a destructor request, after
// which the proxy no longer exists.
const MarshalDestroy uint32 = marshalDestroy

// Message is a request or an event of an Interface.
type Message struct {
	// Name is the name of the message, and Signature its signature in the
	// letters of libwayland: the version it arrived in, then a letter per
	// argument, with ? before those that can be null.
	Name, Signature string
	// Types holds the interface of each letter of Signature, nil where there
	// is none or it is not generated: libwayland needs the interface only to
	// make the proxy of a new_id.
	Types []*Interface
}

// Interface is the struct wl_interface of an interface that generated
// bindings declare. libwayland keeps its address, so an Interface must not be
// copied or moved after Set: generated bindings keep theirs in a package
// array.
type Interface struct {
	in wlInterface
}

// Set fills in. It allocates, and generated bindings call it from init.
func (in *Interface) Set(name string, version int, requests, events []Message) {
	in.in = wlInterface{
		name: cstrCopy(name), version: int32(version),
		methodCount: int32(len(requests)), methods: messages(requests),
		eventCount: int32(len(events)), events: messages(events),
	}
}

// messages converts ms to the layout of libwayland. The Go memory stays alive
// and in place as long as the Interface that points to it.
func messages(ms []Message) *wlMessage {
	if len(ms) == 0 {
		return nil
	}
	out := make([]wlMessage, len(ms))
	for i := range ms {
		m := &ms[i]
		out[i].name = cstrCopy(m.Name)
		out[i].signature = cstrCopy(m.Signature)
		if n := signatureLetters(m.Signature); len(m.Types) != n {
			panic("gowaku: wayland: message " + m.Name + " has " + strconv.Itoa(len(m.Types)) + " types for " + strconv.Itoa(n) + " arguments")
		}
		if len(m.Types) == 0 {
			continue
		}
		types := make([]*wlInterface, len(m.Types))
		for j, t := range m.Types {
			if t != nil {
				types[j] = &t.in
			}
		}
		out[i].types = &types[0]
	}
	return &out[0]
}

// signatureLetters counts the arguments of a signature of libwayland.
func signatureLetters(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if c := s[i]; c != '?' && (c < '0' || c > '9') {
			n++
		}
	}
	return n
}

// cstrCopy returns s as a C string in Go memory of its own.
func cstrCopy(s string) *byte {
	if strings.IndexByte(s, 0) >= 0 {
		panic("gowaku: wayland: name contains a NUL byte")
	}
	b := append([]byte(s), 0)
	return &b[0]
}

// Conn is the connection of a Platform, through which generated bindings send
// requests and receive events.
type Conn struct {
	c *conn
}

// SetInt stores v as argument i of the next request.
func (c *Conn) SetInt(i int, v int32) { c.c.args[i] = uint64(uint32(v)) }

// SetUint stores v as argument i of the next request.
func (c *Conn) SetUint(i int, v uint32) { c.c.args[i] = uint64(v) }

// SetFixed stores v as the wl_fixed_t argument i of the next request.
func (c *Conn) SetFixed(i int, v float64) { c.c.args[i] = uint64(uint32(toFixed(v))) }

// SetString stores s as argument i of the next request. s must not contain
// NUL.
func (c *Conn) SetString(i int, s string) { c.c.args[i] = c.c.str(i, s) }

// SetArray stores b as the wl_array argument i of the next request.
// libwayland copies it.
func (c *Conn) SetArray(i int, b []byte) { c.c.args[i] = c.c.array(i, b) }

// SetObject stores o as argument i of the next request; 0 is null.
func (c *Conn) SetObject(i int, o Proxy) { c.c.args[i] = uint64(o) }

// SetNewID marks argument i of the next request as the new object, which
// Marshal returns.
func (c *Conn) SetNewID(i int) { c.c.args[i] = 0 }

// SetInterface stores the name of in as argument i of the next request, as
// wl_registry.bind takes it.
func (c *Conn) SetInterface(i int, in *Interface) { c.c.args[i] = in.in.nameArg() }

// Marshal sends request opcode of p with the arguments set before, and
// returns the new object if the request makes one, of interface iface and the
// given version. flags is 0 or MarshalDestroy.
func (c *Conn) Marshal(p Proxy, opcode uint32, iface *Interface, version, flags uint32) Proxy {
	var in *wlInterface
	if iface != nil {
		in = &iface.in
	}
	return Proxy(c.c.marshal(proxy(p), opcode, in, version, flags))
}

// Version returns the version that the object of p was bound or made with.
func (c *Conn) Version(p Proxy) uint32 { return c.c.version(proxy(p)) }

// Destroy frees a proxy whose interface has no destructor request.
func (c *Conn) Destroy(p Proxy) { c.c.destroy(proxy(p)) }

// Roundtrip waits until the compositor has handled every request so far, and
// dispatches the events that arrive meanwhile.
func (c *Conn) Roundtrip() error { return c.c.roundtrip() }

// Args are the arguments of an event, valid only while it is dispatched.
type Args wlArgs

// Int returns argument i, an int or an fd.
func (a *Args) Int(i int) int32 { return (*wlArgs)(a).int(i) }

// Uint returns argument i.
func (a *Args) Uint(i int) uint32 { return (*wlArgs)(a).uint(i) }

// Fixed returns the wl_fixed_t argument i.
func (a *Args) Fixed(i int) float64 { return (*wlArgs)(a).fixed(i) }

// Object returns the object argument i; 0 is null.
func (a *Args) Object(i int) Proxy { return Proxy((*wlArgs)(a).object(i)) }

// String returns a copy of argument i.
func (a *Args) String(i int) string { return (*wlArgs)(a).string(i) }

// Array returns the wl_array argument i. The bytes belong to libwayland and
// are valid only during the event.
func (a *Args) Array(i int) []byte { return (*wlArgs)(a).array(i) }

// listener is a generated listener with its dispatch function.
type listener[L any] struct {
	l        L
	dispatch func(L, uint32, *Args)
}

func dispatchListener[L any](h any, opcode uint32, a *wlArgs) {
	x := h.(listener[L])
	x.dispatch(x.l, opcode, (*Args)(a))
}

// Listen sends the events of p to dispatch with l. A proxy has one listener;
// a second panics.
func Listen[L any](c *Conn, p Proxy, l L, dispatch func(l L, opcode uint32, a *Args)) {
	c.c.listen(proxy(p), listener[L]{l, dispatch}, dispatchListener[L])
}

// Global is a global object that the compositor announced on the registry.
type Global struct {
	// Name identifies the global in Bind, Interface is the name of its
	// interface and Version the newest version the compositor offers.
	Name      uint32
	Interface string
	Version   uint32
}

// Conn returns the connection of p, for generated bindings.
func (p *Platform) Conn() *Conn { return &p.ext }

// Globals returns the globals that the compositor offers now. Those that
// come later, such as an output plugged in, appear on the next call.
func (p *Platform) Globals() []Global { return append([]Global(nil), p.globals...) }

// Bind binds global g as interface in, at version or g.Version if that is
// older.
func (p *Platform) Bind(g Global, in *Interface, version uint32) Proxy {
	if in.in.name == nil {
		panic("gowaku: wayland: Bind with an Interface that was never Set")
	}
	return Proxy(p.registry.bind(p.c, g.Name, &in.in, min(version, g.Version)))
}
