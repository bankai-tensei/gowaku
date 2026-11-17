package com

import (
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// HRESULT values of winerror.h.
const (
	S_OK          = 0
	S_FALSE       = 1
	E_NOTIMPL     = 0x80004001
	E_NOINTERFACE = 0x80004002
	E_POINTER     = 0x80004003
	E_FAIL        = 0x80004005
	E_OUTOFMEMORY = 0x8007000E
	E_INVALIDARG  = 0x80070057
)

// Vtable slots of IUnknown.
const (
	iUnknown_QueryInterface = 0 // IUnknown::QueryInterface
	iUnknown_AddRef         = 1 // IUnknown::AddRef
	iUnknown_Release        = 2 // IUnknown::Release
)

// IID_IUnknown is the interface every COM object has.
var IID_IUnknown = windows.GUID{Data1: 0x00000000, Data2: 0x0000, Data3: 0x0000,
	Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}

var (
	ole32    = windows.NewLazySystemDLL("ole32.dll")
	oleaut32 = windows.NewLazySystemDLL("oleaut32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procCoTaskMemAlloc = ole32.NewProc("CoTaskMemAlloc")
)

// Failed reports whether hr is an error.
func Failed(hr uintptr) bool { return int32(hr) < 0 }

// Unknown is what any COM interface pointer points to: first of all, its
// vtable. For an object made by New it is one slot of the object body, which
// also leads to the rest of the object.
//
// A *Unknown that came from the OS may point to an object of any layout; only
// its first word, the vtable, can be read. Owns tells the two apart.
type Unknown struct {
	vtbl uintptr
	body *body
}

// body is the head of an object made by New, in CoTaskMemAlloc memory. The
// slots, one Unknown per interface, follow it.
type body struct {
	refs   int32
	_      int32
	handle uintptr
	slots  uintptr
}

func (b *body) slot(i int) *Unknown {
	// The slots follow the head inside the same allocation.
	return (*Unknown)(unsafe.Add(unsafe.Pointer(b), unsafe.Sizeof(body{})+uintptr(i)*unsafe.Sizeof(Unknown{})))
}

// Call calls method slot of the interface, counted from the start of IUnknown,
// with u as the this pointer. It returns what the method returns, an HRESULT
// for most.
//
// Pointers converted to uintptr in args stay valid during the call, even if
// the method calls back into Go and the stack moves: the directive below
// puts what they point to on the heap, as for LazyProc.Call.
//
//go:uintptrescapes
func (u *Unknown) Call(slot int, args ...uintptr) uintptr {
	vtbl := unsafe.Slice(*(**uintptr)(unsafe.Pointer(u)), slot+1)
	all := make([]uintptr, 0, len(args)+1)
	all = append(all, uintptr(unsafe.Pointer(u)))
	all = append(all, args...)
	r, _, _ := syscall.SyscallN(vtbl[slot], all...)
	return r
}

// AddRef adds a reference.
func (u *Unknown) AddRef() { u.Call(iUnknown_AddRef) }

// Release drops a reference. It does nothing on nil.
func (u *Unknown) Release() {
	if u != nil {
		u.Call(iUnknown_Release)
	}
}

// QueryInterface returns the interface iid of the object with a reference
// added, or nil and the failure.
func (u *Unknown) QueryInterface(iid *windows.GUID) (*Unknown, uintptr) {
	var out *Unknown
	// iid and out are read and written during the call only.
	hr := u.Call(iUnknown_QueryInterface, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out)))
	return out, hr
}

// Query is QueryInterface for an interface implemented in Go. It returns nil
// if the object does not have it.
func (u *Unknown) Query(iface *Interface) *Unknown {
	out, _ := u.QueryInterface(&iface.IID)
	return out
}

// Addr returns u as the integer a syscall takes.
func (u *Unknown) Addr() uintptr { return uintptr(unsafe.Pointer(u)) }

// Owns reports whether u is the interface iface of an object made by New.
// Only then may Get look behind it.
func Owns(u *Unknown, iface *Interface) bool {
	return u != nil && u.vtbl == iface.vtbl
}

// Interface is a COM interface implemented in Go: its IID and the vtable that
// all its objects share.
type Interface struct {
	IID  windows.GUID
	vtbl uintptr
}

// NewInterface builds the vtable of an interface. The methods are the
// addresses of the functions after the three of IUnknown, in the order of the
// header, as syscall.NewCallback or NewFloatThunk return them. Each takes the
// this pointer, a *Unknown, first; Get finds the Go value from it.
//
// The vtable is never freed, so call this once per interface and process.
func NewInterface(iid windows.GUID, methods ...uintptr) *Interface {
	unknown := unknownMethods()
	n := 3 + len(methods)
	mem := coTaskMemAlloc(uintptr(n) * unsafe.Sizeof(uintptr(0)))
	vtbl := unsafe.Slice((*uintptr)(mem), n)
	copy(vtbl, unknown[:])
	copy(vtbl[3:], methods)
	return &Interface{IID: iid, vtbl: uintptr(mem)}
}

// New makes an object with the given interfaces, behind which stands v. It
// returns the first interface with one reference, which the caller owns. When
// the last reference goes, v leaves the table and the memory is freed.
func New(v any, ifaces ...*Interface) *Unknown {
	if len(ifaces) == 0 {
		panic("gowaku: com.New needs at least one interface")
	}
	size := unsafe.Sizeof(body{}) + uintptr(len(ifaces))*unsafe.Sizeof(Unknown{})
	b := (*body)(coTaskMemAlloc(size))
	b.refs = 1
	b.slots = uintptr(len(ifaces))
	for i, iface := range ifaces {
		s := b.slot(i)
		s.vtbl = iface.vtbl
		s.body = b
	}

	objects.mu.Lock()
	objects.next++
	b.handle = objects.next
	if objects.m == nil {
		objects.m = make(map[uintptr]entry)
	}
	objects.m[b.handle] = entry{v: v, ifaces: ifaces}
	objects.mu.Unlock()
	return b.slot(0)
}

// Get returns the Go value behind u, an interface pointer of an object made by
// New, if it is a T. u must be one of those objects (see Owns): the this
// pointer that a method of an Interface receives always is.
func Get[T any](u *Unknown) (T, bool) {
	objects.mu.Lock()
	e, ok := objects.m[u.body.handle]
	objects.mu.Unlock()
	if !ok {
		var zero T
		return zero, false
	}
	v, ok := e.v.(T)
	return v, ok
}

// entry is what the table knows of an object.
type entry struct {
	v      any
	ifaces []*Interface
}

// objects is the table of live objects, by handle.
var objects struct {
	mu   sync.Mutex
	next uintptr
	m    map[uintptr]entry
}

// liveObjects returns the number of objects alive, for tests.
func liveObjects() int {
	objects.mu.Lock()
	defer objects.mu.Unlock()
	return len(objects.m)
}

var unknownOnce = sync.OnceValue(func() [3]uintptr {
	return [3]uintptr{
		syscall.NewCallback(queryInterface),
		syscall.NewCallback(addRef),
		syscall.NewCallback(release),
	}
})

func unknownMethods() [3]uintptr { return unknownOnce() }

// queryInterface is IUnknown::QueryInterface of every object made by New.
func queryInterface(this *Unknown, riid *windows.GUID, ppv **Unknown) uintptr {
	if ppv == nil {
		return E_POINTER
	}
	*ppv = nil
	if riid == nil {
		return E_POINTER
	}
	b := this.body
	if *riid == IID_IUnknown {
		*ppv = b.slot(0)
	} else {
		objects.mu.Lock()
		e := objects.m[b.handle]
		objects.mu.Unlock()
		for i, iface := range e.ifaces {
			if iface.IID == *riid {
				*ppv = b.slot(i)
				break
			}
		}
	}
	if *ppv == nil {
		return E_NOINTERFACE
	}
	atomic.AddInt32(&b.refs, 1)
	return S_OK
}

// addRef is IUnknown::AddRef of every object made by New.
func addRef(this *Unknown) uintptr {
	return uintptr(uint32(atomic.AddInt32(&this.body.refs, 1)))
}

// release is IUnknown::Release of every object made by New.
func release(this *Unknown) uintptr {
	b := this.body
	n := atomic.AddInt32(&b.refs, -1)
	switch {
	case n == 0:
		objects.mu.Lock()
		delete(objects.m, b.handle)
		objects.mu.Unlock()
		windows.CoTaskMemFree(unsafe.Pointer(b))
	case n < 0:
		panic("gowaku: COM object released more often than referenced")
	}
	return uintptr(uint32(n))
}

// coTaskMemAlloc returns size bytes of zeroed memory outside the Go heap.
func coTaskMemAlloc(size uintptr) unsafe.Pointer {
	r, _, _ := procCoTaskMemAlloc.Call(size)
	if r == 0 {
		panic("gowaku: CoTaskMemAlloc: out of memory")
	}
	// Reading through &r turns the address into a pointer without a
	// uintptr-to-pointer conversion; the memory is not the Go heap's.
	p := *(*unsafe.Pointer)(unsafe.Pointer(&r))
	clear(unsafe.Slice((*byte)(p), size))
	return p
}
