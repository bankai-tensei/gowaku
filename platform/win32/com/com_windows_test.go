package com

import (
	"math"
	"sync"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	iidTest      = windows.GUID{Data1: 0x6A1B2C3D, Data2: 1, Data3: 2, Data4: [8]byte{1, 2, 3, 4, 5, 6, 7, 8}}
	iidTestOther = windows.GUID{Data1: 0x6A1B2C3E, Data2: 1, Data3: 2, Data4: [8]byte{1, 2, 3, 4, 5, 6, 7, 8}}
	iidAbsent    = windows.GUID{Data1: 0x6A1B2C3F}
)

type counter struct{ n uintptr }

// testInterfaces builds two interfaces of one method each, once: callbacks
// are never freed.
var testInterfaces = sync.OnceValues(func() (*Interface, *Interface) {
	get := syscall.NewCallback(func(this *Unknown, add uintptr) uintptr {
		c, ok := Get[*counter](this)
		if !ok {
			return E_FAIL
		}
		c.n += add
		return c.n
	})
	return NewInterface(iidTest, get), NewInterface(iidTestOther, get)
})

func refs(u *Unknown) int32 { return u.body.refs }

func TestObject_AddRefReleaseFreesAtZero(t *testing.T) {
	a, _ := testInterfaces()
	before := liveObjects()
	u := New(&counter{}, a)
	if refs(u) != 1 || liveObjects() != before+1 {
		t.Fatalf("new object: refs %d, live %d", refs(u), liveObjects())
	}
	u.AddRef()
	u.AddRef()
	if refs(u) != 3 {
		t.Errorf("after two AddRef: refs %d, want 3", refs(u))
	}
	if got := u.Call(3, 5); got != 5 {
		t.Errorf("method returned %d, want 5", got)
	}
	u.Release()
	u.Release()
	if liveObjects() != before+1 {
		t.Error("object freed while referenced")
	}
	u.Release()
	if liveObjects() != before {
		t.Errorf("object not freed at zero: live %d, want %d", liveObjects(), before)
	}
}

func TestObject_QueryInterface(t *testing.T) {
	a, b := testInterfaces()
	c := &counter{}
	u := New(c, a, b)
	defer u.Release()

	other := u.Query(b)
	if other == nil {
		t.Fatal("QueryInterface for the second interface failed")
	}
	if other == u {
		t.Error("the second interface must be another pointer")
	}
	if !Owns(other, b) || Owns(other, a) || !Owns(u, a) {
		t.Error("Owns does not tell the interfaces apart")
	}
	if refs(u) != 2 {
		t.Errorf("refs after QueryInterface %d, want 2", refs(u))
	}
	// Both interfaces lead to the same Go value.
	other.Call(3, 2)
	u.Call(3, 3)
	if c.n != 5 {
		t.Errorf("shared state %d, want 5", c.n)
	}
	unk, hr := other.QueryInterface(&IID_IUnknown)
	if hr != S_OK || unk != u {
		t.Errorf("IUnknown from the second interface = %p (%#x), want the first %p", unk, hr, u)
	}
	unk.Release()
	if p, hr := u.QueryInterface(&iidAbsent); p != nil || hr != E_NOINTERFACE {
		t.Errorf("absent interface: %p, %#x", p, hr)
	}
	other.Release()
	if refs(u) != 1 {
		t.Errorf("refs after the releases %d, want 1", refs(u))
	}
}

func TestVariant_RoundTrip(t *testing.T) {
	var v VARIANT
	if unsafe.Sizeof(v) != 24 {
		t.Errorf("sizeof VARIANT = %d, want 24", unsafe.Sizeof(v))
	}
	v.SetI4(-7)
	if v.VT != VT_I4 || v.I4() != -7 {
		t.Errorf("I4: %d %d", v.VT, v.I4())
	}
	v.SetBool(true)
	if v.VT != VT_BOOL || !v.Bool() {
		t.Error("Bool true")
	}
	v.SetBool(false)
	if v.Bool() {
		t.Error("Bool false")
	}
	v.SetR8(math.Pi)
	if v.VT != VT_R8 || v.R8() != math.Pi {
		t.Errorf("R8: %v", v.R8())
	}
	for _, s := range []string{"", "Save", "こんにちは 😀", "a\x00b"} {
		v.SetBSTR(s)
		if v.VT != VT_BSTR || v.String() != s {
			t.Errorf("BSTR %q: got %q", s, v.String())
		}
		v.Clear()
		if v.VT != VT_EMPTY {
			t.Errorf("Clear left type %d", v.VT)
		}
	}
	a, _ := testInterfaces()
	u := New(&counter{}, a)
	u.AddRef()
	v.SetUnknown(u)
	if v.Unknown() != u {
		t.Error("Unknown")
	}
	v.Clear()
	if refs(u) != 1 {
		t.Errorf("VariantClear did not release: refs %d", refs(u))
	}
	u.Release()
}

func TestSafeArray(t *testing.T) {
	i4 := NewSafeArrayI4([]int32{3, 1, -2})
	if got := i4.I4s(); len(got) != 3 || got[0] != 3 || got[2] != -2 {
		t.Errorf("I4s = %v", got)
	}
	i4.Destroy()
	r8 := NewSafeArrayR8([]float64{1.5, 2.5})
	if got := r8.R8s(); len(got) != 2 || got[1] != 2.5 {
		t.Errorf("R8s = %v", got)
	}
	r8.Destroy()
	empty := NewSafeArrayUnknown(nil)
	if empty.Len() != 0 {
		t.Errorf("empty Len = %d", empty.Len())
	}
	empty.Destroy()

	a, _ := testInterfaces()
	before := liveObjects()
	u := New(&counter{}, a)
	arr := NewSafeArrayUnknown([]*Unknown{u})
	if got := arr.Unknowns(); len(got) != 1 || got[0] != u {
		t.Errorf("Unknowns = %v", got)
	}
	arr.Destroy()
	if liveObjects() != before {
		t.Error("destroying the array did not release its element")
	}
}

// TestFloatThunk calls the thunk the way a C++ caller would: with the doubles
// in floating-point registers and nothing in the integer ones. syscall cannot
// load a floating-point register, so a float caller moves the arguments there
// and clears the integer registers before it jumps to the thunk. The callback
// sees the doubles only if the thunk moved them back.
func TestFloatThunk(t *testing.T) {
	type call struct {
		this, x, y, rest uintptr
	}
	var got call
	one := syscall.NewCallback(func(this, x, rest uintptr) uintptr {
		got = call{this: this, x: x, rest: rest}
		return 42
	})
	two := syscall.NewCallback(func(this, x, y, rest uintptr) uintptr {
		got = call{this, x, y, rest}
		return 43
	})

	thunk, err := NewFloatThunk(one, 1)
	if err != nil {
		t.Fatal(err)
	}
	stub := callerStub(t, thunk, 1)
	r, _, _ := syscall.SyscallN(stub, 7, uintptr(math.Float64bits(-2.75)), 99)
	if r != 42 || got.this != 7 || math.Float64frombits(uint64(got.x)) != -2.75 || got.rest != 99 {
		t.Errorf("one double: returned %d, got %+v (x = %v)", r, got, math.Float64frombits(uint64(got.x)))
	}

	thunk, err = NewFloatThunk(two, 2)
	if err != nil {
		t.Fatal(err)
	}
	stub = callerStub(t, thunk, 2)
	r, _, _ = syscall.SyscallN(stub, 8, uintptr(math.Float64bits(1e300)), uintptr(math.Float64bits(0.5)), 77)
	if r != 43 || got.this != 8 || math.Float64frombits(uint64(got.x)) != 1e300 ||
		math.Float64frombits(uint64(got.y)) != 0.5 || got.rest != 77 {
		t.Errorf("two doubles: returned %d, got %+v", r, got)
	}
}

func callerStub(t *testing.T, thunk uintptr, floats int) uintptr {
	t.Helper()
	stub, err := NewFloatCaller(thunk, floats)
	if err != nil {
		t.Fatal(err)
	}
	return stub
}
