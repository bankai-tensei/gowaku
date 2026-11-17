// Package x11 implements platform.Platform on Xlib without cgo. libX11 is opened
// with purego, and every function is resolved at startup.
//
// Under Wayland it runs through XWayland. Text input takes what an input method
// commits, through XIM; the input method draws its own composition. See
// docs/ADR/0014-x11-platform.md.
//
// This is one of the few packages allowed to use unsafe and purego.
//
// The variable kept is an exception to CODING_STYLE §3.2. See ptr.
package x11
