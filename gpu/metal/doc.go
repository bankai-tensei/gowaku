// Package metal implements gpu.Backend with Metal on macOS, without cgo. It
// calls the Objective-C runtime through purego: objc.Send for setup, and
// objc_msgSend with integer arguments for what runs on every frame. The SDF
// shaders are MSL source that Metal compiles when the device is created
// (docs/ADR/0082-macos-cocoa-metal.md).
//
// The package builds on darwin/arm64 only. The hot path hands structs of more
// than 16 bytes, such as MTLViewport, to objc_msgSend by pointer, which is how
// the AAPCS64 passes them; amd64 copies them to the stack instead. See
// docs/ADR/0082-macos-cocoa-metal.md.
//
// It imports gpu, platform (for NativeHandle) and geom. It must not import
// anything above gpu.
package metal
