// Package d3d11 implements gpu.Backend on Direct3D 11 without cgo. COM methods
// are called through their vtables with syscall.SyscallN.
//
// This is one of the few packages allowed to use unsafe and syscall.
//
// The package-level DLL, procedure and GUID values in abi_windows.go are never
// written after initialization, so they are constants in all but syntax rather
// than the mutable globals CODING_STYLE §3.2 forbids.
//
// The ABI types are exported only because they keep their header names. This
// package is not part of the public API of gowaku, which makes no promise about
// them. See docs/ADR/0016-public-api.md.
package d3d11
