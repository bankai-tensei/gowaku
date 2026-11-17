// Package cocoa implements platform.Platform with AppKit on macOS, without
// cgo. It calls the Objective-C runtime through purego/objc, and makes the
// classes it needs (the content view, which is an NSTextInputClient, and the
// delegates) at run time with objc.RegisterClass. See
// docs/ADR/0082-macos-cocoa-metal.md.
//
// The content view of each window hosts a CAMetalLayer, which
// platform.CocoaHandle hands to gpu/metal.
//
// AppKit runs only on the main thread, so the package locks the main
// goroutine to it when the program starts, and the UI goroutine must be the
// main goroutine. It builds on darwin/arm64, as gpu/metal does. See
// docs/ADR/0082-macos-cocoa-metal.md.
//
// It imports platform, input, semantics and geom, like the other platforms.
package cocoa
