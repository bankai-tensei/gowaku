// Package win32 implements platform.Platform on Win32 without cgo.
//
// This is one of the few packages allowed to use unsafe and syscall.
//
// The package-level DLL and procedure values in api_windows.go are resolved
// once and never written after that, so they are constants in all but syntax
// rather than the mutable globals CODING_STYLE §3.2 forbids. The same goes for
// uiaSetup in uia_windows.go, which loads UI Automation and builds the vtables
// of its providers once per process: the vtables are shared by every window.
// Platform state lives in the Platform struct, and the window procedure is a
// method value of it.
package win32
