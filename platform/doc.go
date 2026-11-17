// Package platform defines the window system abstraction that gowaku draws on:
// Platform, Window, Event and NativeHandle. Implementations live in
// platform/win32, platform/x11, platform/wayland and platform/cocoa.
//
// platform imports geom and input only. core, layout, paint, text, theme, motion and
// widgets must not import it; only the root package and gpu do, and gpu uses
// nothing but NativeHandle.
package platform
