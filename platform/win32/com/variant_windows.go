package com

import (
	"math"
	"unicode/utf16"
	"unsafe"
)

// VARTYPE values of wtypes.h.
const (
	VT_EMPTY   = 0
	VT_I4      = 3
	VT_R8      = 5
	VT_BSTR    = 8
	VT_BOOL    = 11
	VT_UNKNOWN = 13
	VT_ARRAY   = 0x2000

	VARIANT_TRUE  = -1
	VARIANT_FALSE = 0
)

var (
	procSysAllocStringLen = oleaut32.NewProc("SysAllocStringLen")
	procSysFreeString     = oleaut32.NewProc("SysFreeString")
	procSysStringLen      = oleaut32.NewProc("SysStringLen")
	procVariantClear      = oleaut32.NewProc("VariantClear")

	procSafeArrayCreateVector = oleaut32.NewProc("SafeArrayCreateVector")
	procSafeArrayAccessData   = oleaut32.NewProc("SafeArrayAccessData")
	procSafeArrayUnaccessData = oleaut32.NewProc("SafeArrayUnaccessData")
	procSafeArrayGetLBound    = oleaut32.NewProc("SafeArrayGetLBound")
	procSafeArrayGetUBound    = oleaut32.NewProc("SafeArrayGetUBound")
	procSafeArrayDestroy      = oleaut32.NewProc("SafeArrayDestroy")
)

// VARIANT is the OLE VARIANT: a type tag and a union big enough for two
// pointers (a BRECORD), which makes it 24 bytes on 64-bit Windows and 16 on
// 32-bit.
type VARIANT struct {
	VT  uint16
	_   [3]uint16
	val [2]uintptr
}

// The setters overwrite v without clearing what it held, as a callee fills an
// out parameter.

// SetEmpty makes v VT_EMPTY.
func (v *VARIANT) SetEmpty() { *v = VARIANT{} }

// SetI4 makes v the VT_I4 i.
func (v *VARIANT) SetI4(i int32) {
	*v = VARIANT{VT: VT_I4}
	*(*int32)(unsafe.Pointer(&v.val)) = i
}

// SetBool makes v the VT_BOOL b.
func (v *VARIANT) SetBool(b bool) {
	*v = VARIANT{VT: VT_BOOL}
	var x int16 = VARIANT_FALSE
	if b {
		x = VARIANT_TRUE
	}
	*(*int16)(unsafe.Pointer(&v.val)) = x
}

// SetR8 makes v the VT_R8 f.
func (v *VARIANT) SetR8(f float64) {
	*v = VARIANT{VT: VT_R8}
	*(*uint64)(unsafe.Pointer(&v.val)) = math.Float64bits(f)
}

// SetBSTR makes v a VT_BSTR of s. v owns the string: VariantClear frees it.
func (v *VARIANT) SetBSTR(s string) {
	*v = VARIANT{VT: VT_BSTR}
	*(**uint16)(unsafe.Pointer(&v.val)) = SysAllocString(s)
}

// SetUnknown makes v the VT_UNKNOWN u, taking over one reference of u.
func (v *VARIANT) SetUnknown(u *Unknown) {
	*v = VARIANT{VT: VT_UNKNOWN}
	*(**Unknown)(unsafe.Pointer(&v.val)) = u
}

// I4 returns the value of a VT_I4.
func (v *VARIANT) I4() int32 { return *(*int32)(unsafe.Pointer(&v.val)) }

// Bool returns the value of a VT_BOOL.
func (v *VARIANT) Bool() bool { return *(*int16)(unsafe.Pointer(&v.val)) != VARIANT_FALSE }

// R8 returns the value of a VT_R8.
func (v *VARIANT) R8() float64 { return math.Float64frombits(*(*uint64)(unsafe.Pointer(&v.val))) }

// String returns the text of a VT_BSTR, and "" for any other type.
func (v *VARIANT) String() string {
	if v.VT != VT_BSTR {
		return ""
	}
	return BSTRToString(*(**uint16)(unsafe.Pointer(&v.val)))
}

// Unknown returns the interface of a VT_UNKNOWN, still owned by v.
func (v *VARIANT) Unknown() *Unknown { return *(**Unknown)(unsafe.Pointer(&v.val)) }

// Clear frees what v holds and makes it VT_EMPTY.
func (v *VARIANT) Clear() {
	// v is only used during the call.
	procVariantClear.Call(uintptr(unsafe.Pointer(v)))
}

// SysAllocString returns s as a BSTR, which the receiver frees with
// SysFreeString. It never returns nil, even for "".
func SysAllocString(s string) *uint16 {
	u := utf16.Encode([]rune(s))
	var p uintptr
	if len(u) > 0 {
		p = uintptr(unsafe.Pointer(&u[0]))
	}
	// SysAllocStringLen copies u, which only needs to live through the call.
	r, _, _ := procSysAllocStringLen.Call(p, uintptr(len(u)))
	if r == 0 {
		panic("gowaku: SysAllocStringLen: out of memory")
	}
	return *(**uint16)(unsafe.Pointer(&r))
}

// SysFreeString frees a BSTR. It does nothing on nil.
func SysFreeString(b *uint16) {
	if b != nil {
		procSysFreeString.Call(uintptr(unsafe.Pointer(b)))
	}
}

// BSTRToString returns the text of a BSTR, which may hold NUL characters.
func BSTRToString(b *uint16) string {
	if b == nil {
		return ""
	}
	n, _, _ := procSysStringLen.Call(uintptr(unsafe.Pointer(b)))
	return string(utf16.Decode(unsafe.Slice(b, int(n))))
}

// SAFEARRAY is an OLE safe array, only ever handled through a pointer.
type SAFEARRAY struct{ _ [0]byte }

func newSafeArray(vt uint16, n int) (*SAFEARRAY, unsafe.Pointer) {
	r, _, _ := procSafeArrayCreateVector.Call(uintptr(vt), 0, uintptr(n))
	if r == 0 {
		panic("gowaku: SafeArrayCreateVector: out of memory")
	}
	a := *(**SAFEARRAY)(unsafe.Pointer(&r))
	if n == 0 {
		return a, nil
	}
	var data unsafe.Pointer
	procSafeArrayAccessData.Call(uintptr(unsafe.Pointer(a)), uintptr(unsafe.Pointer(&data)))
	return a, data
}

func unaccess(a *SAFEARRAY) {
	procSafeArrayUnaccessData.Call(uintptr(unsafe.Pointer(a)))
}

// NewSafeArrayI4 returns a VT_I4 array of v, which the receiver destroys.
func NewSafeArrayI4(v []int32) *SAFEARRAY {
	a, data := newSafeArray(VT_I4, len(v))
	if data != nil {
		copy(unsafe.Slice((*int32)(data), len(v)), v)
		unaccess(a)
	}
	return a
}

// NewSafeArrayR8 returns a VT_R8 array of v, which the receiver destroys.
func NewSafeArrayR8(v []float64) *SAFEARRAY {
	a, data := newSafeArray(VT_R8, len(v))
	if data != nil {
		copy(unsafe.Slice((*float64)(data), len(v)), v)
		unaccess(a)
	}
	return a
}

// NewSafeArrayUnknown returns a VT_UNKNOWN array of v, which the receiver
// destroys. The array takes over one reference of each element.
func NewSafeArrayUnknown(v []*Unknown) *SAFEARRAY {
	a, data := newSafeArray(VT_UNKNOWN, len(v))
	if data != nil {
		copy(unsafe.Slice((**Unknown)(data), len(v)), v)
		unaccess(a)
	}
	return a
}

// Len returns the number of elements of a one-dimensional array.
func (a *SAFEARRAY) Len() int {
	var lo, hi int32
	procSafeArrayGetLBound.Call(uintptr(unsafe.Pointer(a)), 1, uintptr(unsafe.Pointer(&lo)))
	procSafeArrayGetUBound.Call(uintptr(unsafe.Pointer(a)), 1, uintptr(unsafe.Pointer(&hi)))
	return int(hi - lo + 1)
}

// I4s returns a copy of the elements of a VT_I4 array.
func (a *SAFEARRAY) I4s() []int32 { return elements[int32](a) }

// R8s returns a copy of the elements of a VT_R8 array.
func (a *SAFEARRAY) R8s() []float64 { return elements[float64](a) }

// Unknowns returns the elements of a VT_UNKNOWN array, still owned by it.
func (a *SAFEARRAY) Unknowns() []*Unknown { return elements[*Unknown](a) }

func elements[T any](a *SAFEARRAY) []T {
	n := a.Len()
	if n <= 0 {
		return nil
	}
	var data unsafe.Pointer
	procSafeArrayAccessData.Call(uintptr(unsafe.Pointer(a)), uintptr(unsafe.Pointer(&data)))
	out := make([]T, n)
	copy(out, unsafe.Slice((*T)(data), n))
	unaccess(a)
	return out
}

// Destroy frees the array and releases what it holds. It does nothing on nil.
func (a *SAFEARRAY) Destroy() {
	if a != nil {
		procSafeArrayDestroy.Call(uintptr(unsafe.Pointer(a)))
	}
}
