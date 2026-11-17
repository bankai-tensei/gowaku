// Package com implements COM objects in Go without cgo, for the interfaces
// that Windows calls back into: the UI Automation providers today, the
// drag-and-drop and file dialog event interfaces later (ADR 0042 §5).
//
// The body of an object (a vtable pointer per interface, the reference count
// and a handle) lives in memory from CoTaskMemAlloc, outside the Go heap, so no
// Go pointer is ever handed to the OS. The Go value behind an object is found
// from the handle in a table, and leaves it when the count drops to zero.
// Every object of an interface shares one vtable, built once.
//
// It also builds VARIANT, BSTR and SAFEARRAY values, and the small thunks that
// let a syscall.NewCallback function receive double arguments.
//
// Like the rest of platform/win32, this package may use unsafe and syscall. It
// must not import anything of gowaku.
//
// The table of live objects and the vtable of IUnknown are package-level
// state, the one exception to CODING_STYLE §3.2 here: the OS calls back
// through function pointers shared by every object, so nothing narrower can
// find them. Both are guarded for use from any thread.
package com
