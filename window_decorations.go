package gowaku

import (
	"image"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/theme"
)

// The icon, size limits, always on top, fullscreen and transparency of a
// window. See docs/ADR/0111-window-decorations.md.

// SetIcon replaces the icon of the window, which the title bar, the task bar
// and the window switcher show. images are the same picture in one or more
// sizes; see WindowOptions.Icon. Nil goes back to the default. On macOS it is
// the icon of the application in the Dock. It must be called on the UI
// goroutine.
func (h *WindowHandle) SetIcon(images []image.Image) {
	if w := h.window; h.app.windows[w.native.ID()] == w {
		w.native.SetIcon(nrgbas(images))
	}
}

// SetSizeLimits bounds the client area of the window, in logical pixels; see
// WindowOptions.MinSize. A window outside the new bounds is resized into
// them. It must be called on the UI goroutine.
func (h *WindowHandle) SetSizeLimits(minSize, maxSize geom.Size) {
	if w := h.window; h.app.windows[w.native.ID()] == w {
		w.native.SetSizeLimits(minSize, maxSize)
	}
}

// SetAlwaysOnTop keeps the window above the windows of other applications, or
// stops; see WindowOptions.AlwaysOnTop. It must be called on the UI goroutine.
func (h *WindowHandle) SetAlwaysOnTop(on bool) {
	if w := h.window; h.app.windows[w.native.ID()] == w {
		w.native.SetAlwaysOnTop(on)
	}
}

// SetFullscreen puts the window over the whole of its monitor, or back where it
// was; see WindowOptions.Fullscreen. The change takes effect when the window
// system has made it, which may be after an animation on macOS. It must be
// called on the UI goroutine.
func (h *WindowHandle) SetFullscreen(on bool) {
	if w := h.window; h.app.windows[w.native.ID()] == w {
		w.native.SetFullscreen(on)
	}
}

// Fullscreen reports whether the window is fullscreen, by the program or by the
// user through the window system, also during the first Build of the window.
// It must be called on the UI goroutine.
func (h *WindowHandle) Fullscreen() bool {
	return h.window.native.Fullscreen()
}

// Transparent reports whether the window shows the desktop through the alpha
// of what it draws: it was opened with WindowOptions.Transparent, and the
// system can, also during the first Build of the window. It must be called on
// the UI goroutine.
func (h *WindowHandle) Transparent() bool {
	return h.window.transparent
}

// windowBackground returns what fills a window whose options ask for
// background, and whether that follows the theme. A transparent window starts
// from nothing: the zero color is clear there.
func windowBackground(background geom.Color, th *theme.Theme, transparent bool) (color geom.Color, themed bool) {
	if background != (geom.Color{}) {
		return background, false
	}
	if transparent {
		return geom.Color{}, false
	}
	return th.Color.Canvas, true
}

// clearColor returns the color that a frame of w starts from. The swap chain of
// a transparent window takes premultiplied colors, as gowaku draws them; that
// of an opaque one ignores the alpha.
func (w *window) clearColor() geom.Color {
	if w.transparent {
		return premultiplied(w.background)
	}
	return w.background
}

// premultiplied returns c with its color multiplied by its alpha.
func premultiplied(c geom.Color) geom.Color {
	return geom.Color{R: c.R * c.A, G: c.G * c.A, B: c.B * c.A, A: c.A}
}

// nrgbas converts images for the platform.
func nrgbas(images []image.Image) []*image.NRGBA {
	if len(images) == 0 {
		return nil
	}
	out := make([]*image.NRGBA, 0, len(images))
	for _, img := range images {
		if n := nrgba(img); n != nil {
			out = append(out, n)
		}
	}
	return out
}
