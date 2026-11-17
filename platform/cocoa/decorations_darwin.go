//go:build darwin && arm64

package cocoa

import (
	"bytes"
	"image"
	"image/png"
	"math"

	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/platform/internal/icon"
)

// The icon, size limits, always on top, fullscreen and transparency of a
// window. macOS has no icon per window: the icon goes to the Dock, for the
// whole application. See docs/ADR/0111-window-decorations.md.

// The window levels and the style bit of a fullscreen window.
const (
	NSNormalWindowLevel                         = 0
	NSFloatingWindowLevel                       = 3
	NSWindowStyleMaskFullScreen                 = 1 << 14
	NSWindowCollectionBehaviorFullScreenPrimary = 1 << 7
	// dockIconSize is the largest icon the Dock is given, in pixels.
	dockIconSize = 512
)

// SetIcon implements platform.Window: the icon of the application in the Dock,
// which the last window to set one decides. Nil goes back to the icon of the
// bundle, or the generic one.
func (w *Window) SetIcon(images []*image.NRGBA) {
	w.p.libs.pool(func() {
		var img objc.ID
		if icon.Largest(images) > 0 {
			img = imageOf(icon.Fit(images, min(icon.Largest(images), dockIconSize)))
		}
		w.p.app.Send(sel("setApplicationIconImage:"), img)
	})
}

// imageOf makes an autoreleased NSImage of img, or returns 0.
func imageOf(img *image.NRGBA) objc.ID {
	if img == nil {
		return 0
	}
	var buf bytes.Buffer
	if png.Encode(&buf, img) != nil {
		return 0
	}
	return class("NSImage").Send(sel("alloc")).Send(sel("initWithData:"), nsData(buf.Bytes())).Send(sel("autorelease"))
}

// SetSizeLimits implements platform.Window: contentMinSize and contentMaxSize,
// in points, which are logical pixels.
func (w *Window) SetSizeLimits(minSize, maxSize geom.Size) {
	w.p.libs.pool(func() { w.applySizeLimits(minSize, maxSize) })
}

// applySizeLimits sets the limits. An axis without a maximum gets the largest
// size there is, as AppKit has by default.
func (w *Window) applySizeLimits(minSize, maxSize geom.Size) {
	most := func(v float32) float64 {
		if v <= 0 {
			return math.MaxFloat64
		}
		return float64(v)
	}
	w.window.Send(sel("setContentMinSize:"), NSSize{float64(minSize.W), float64(minSize.H)})
	w.window.Send(sel("setContentMaxSize:"), NSSize{most(maxSize.W), most(maxSize.H)})
}

// SetAlwaysOnTop implements platform.Window: the floating window level.
func (w *Window) SetAlwaysOnTop(on bool) {
	level := int64(NSNormalWindowLevel)
	if on {
		level = NSFloatingWindowLevel
	}
	w.p.libs.pool(func() { w.window.Send(sel("setLevel:"), level) })
}

// SetFullscreen implements platform.Window: toggleFullScreen, which moves the
// window to a space of its own with an animation. The delegate hears when it
// is done.
func (w *Window) SetFullscreen(on bool) {
	if w.closed || on == w.Fullscreen() {
		return
	}
	w.p.libs.pool(func() { w.window.Send(sel("toggleFullScreen:"), objc.ID(0)) })
}

// Fullscreen implements platform.Window.
func (w *Window) Fullscreen() bool {
	if w.closed {
		return false
	}
	full := false
	w.p.libs.pool(func() { full = objc.Send[uint64](w.window, sel("styleMask"))&NSWindowStyleMaskFullScreen != 0 })
	return full
}

// Transparent implements platform.Window. The window and its layer are not
// opaque, and the window server blends them with what is below.
func (w *Window) Transparent() bool { return w.transparent }

// makeTransparent clears the background of the window and makes its layer
// blend, so that the alpha of the frames shows the desktop.
func (w *Window) makeTransparent() {
	w.window.Send(sel("setOpaque:"), false)
	w.window.Send(sel("setBackgroundColor:"), class("NSColor").Send(sel("clearColor")))
	w.layer.Send(sel("setOpaque:"), false)
}
