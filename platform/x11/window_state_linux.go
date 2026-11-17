package x11

import (
	"image"
	"strings"
	"unsafe"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/internal/icon"
)

// The icon, size limits, always on top, fullscreen and transparency of a
// window, through ICCCM and EWMH. See docs/ADR/0111-window-decorations.md.

// The values of visuals, colormaps and size hints.
const (
	TrueColor  = 4
	AllocNone  = 0
	CWColormap = 1 << 13

	PMinSize = 1 << 4
	PMaxSize = 1 << 5

	// netWMStateRemove and netWMStateAdd are actions of a _NET_WM_STATE
	// message.
	netWMStateRemove = 0
	netWMStateAdd    = 1
	// unbounded stands for an axis without a maximum, where WM_NORMAL_HINTS
	// has one for both or neither.
	unbounded = 1 << 15
)

// XVisualInfo is what XMatchVisualInfo fills.
type XVisualInfo struct {
	Visual       uintptr
	VisualID     uintptr
	Screen       int32
	Depth        int32
	Class        int32
	RedMask      uintptr
	GreenMask    uintptr
	BlueMask     uintptr
	ColormapSize int32
	BitsPerRGB   int32
}

// windowState is the state of the decorations of a Window.
type windowState struct {
	// colormap is that of the visual with alpha of a transparent window, or
	// zero.
	colormap XID
	// placed, placeX and placeY are the position of WM_NORMAL_HINTS, and
	// minSize and maxSize its bounds in logical pixels.
	placed           bool
	placeX, placeY   int
	minSize, maxSize geom.Size
	fullscreen       bool
}

// createWindowWithAlpha makes a window, not shown yet, with a visual of 32 bits
// whose fourth channel is alpha, which a compositing manager blends with what
// is below. ok is false where the server has no such visual.
func (p *Platform) createWindowWithAlpha(width, height int, events uintptr) (id, colormap XID, ok bool) {
	var info XVisualInfo
	if call(p.x.XMatchVisualInfo, uintptr(p.display), p.screen, 32, TrueColor, ptr(&info)) == 0 || info.Visual == 0 {
		return 0, 0, false
	}
	colormap = XID(call(p.x.XCreateColormap, uintptr(p.display), uintptr(p.root), info.Visual, AllocNone))
	// A visual other than the parent's needs its own colormap and a border
	// pixel, or the server answers BadMatch.
	attrs := XSetWindowAttributes{BitGravity: NorthWestGravity, EventMask: events, Colormap: colormap}
	id = XID(call(p.x.XCreateWindow, uintptr(p.display), uintptr(p.root), 0, 0, uintptr(width), uintptr(height), 0,
		32, InputOutput, info.Visual, CWBackPixmap|CWBorderPixel|CWBitGravity|CWEventMask|CWColormap, ptr(&attrs)))
	if id == 0 {
		call(p.x.XFreeColormap, uintptr(p.display), uintptr(colormap))
		return 0, 0, false
	}
	return id, colormap, true
}

// SetIcon implements platform.Window: _NET_WM_ICON, in every size of the
// ladder up to the largest image.
func (w *Window) SetIcon(images []*image.NRGBA) {
	if w.xid == 0 {
		return
	}
	p := w.platform
	data := iconData(images)
	if data == nil {
		call(p.x.XDeleteProperty, uintptr(p.display), uintptr(w.xid), uintptr(p.atoms.NET_WM_ICON))
	} else {
		w.setProperty(p.atoms.NET_WM_ICON, XA_CARDINAL, 32, unsafe.Pointer(&data[0]), len(data))
	}
	call(p.x.XFlush, uintptr(p.display))
}

// iconData returns the value of _NET_WM_ICON for images: for each size, the
// width, the height, and the pixels as ARGB, not premultiplied, row by row.
// Format 32 properties are longs to Xlib, whatever their width.
func iconData(images []*image.NRGBA) []uintptr {
	var data []uintptr
	for _, size := range icon.Ladder(images) {
		img := icon.Fit(images, size)
		data = append(data, uintptr(size), uintptr(size))
		for y := range size {
			row := img.Pix[y*img.Stride:]
			for x := range size {
				px := row[4*x:]
				data = append(data, uintptr(px[3])<<24|uintptr(px[0])<<16|uintptr(px[1])<<8|uintptr(px[2]))
			}
		}
	}
	return data
}

// SetSizeLimits implements platform.Window: the minimum and maximum of
// WM_NORMAL_HINTS. Window managers keep the window within them themselves.
func (w *Window) SetSizeLimits(minSize, maxSize geom.Size) {
	w.state.minSize, w.state.maxSize = minSize, maxSize
	if w.xid != 0 {
		w.writeNormalHints()
		call(w.platform.x.XFlush, uintptr(w.platform.display))
	}
}

// writeNormalHints replaces WM_NORMAL_HINTS with the position and the bounds
// of the window.
func (w *Window) writeNormalHints() {
	s := &w.state
	var hints [sizeHintsLongs]uintptr
	if s.placed {
		hints[0] |= USPosition | PWinGravity
		hints[1], hints[2] = uintptr(int32(s.placeX)), uintptr(int32(s.placeY))
		hints[17] = StaticGravity
	}
	scale := w.platform.scale
	physical := func(v float32) uintptr { return uintptr(int32(v*scale + 0.5)) }
	if s.minSize.W > 0 || s.minSize.H > 0 {
		hints[0] |= PMinSize
		hints[5], hints[6] = physical(s.minSize.W), physical(s.minSize.H)
	}
	if s.maxSize.W > 0 || s.maxSize.H > 0 {
		hints[0] |= PMaxSize
		hints[7], hints[8] = unbounded, unbounded
		if s.maxSize.W > 0 {
			hints[7] = physical(s.maxSize.W)
		}
		if s.maxSize.H > 0 {
			hints[8] = physical(s.maxSize.H)
		}
	}
	w.setProperty(XA_WM_NORMAL_HINTS, XA_WM_SIZE_HINTS, 32, unsafe.Pointer(&hints[0]), sizeHintsLongs)
}

// initialState writes _NET_WM_STATE before the window is mapped, as EWMH asks
// for a window that is not managed yet: modal, above and fullscreen.
func (w *Window) initialState(opts *platform.WindowOptions) {
	a := &w.platform.atoms
	var states []uintptr
	if owner, ok := opts.Owner.(*Window); ok && owner != nil && owner.xid != 0 && opts.Modal {
		states = append(states, uintptr(a.NET_WM_STATE_MODAL))
	}
	if opts.AlwaysOnTop {
		states = append(states, uintptr(a.NET_WM_STATE_ABOVE))
	}
	if opts.Fullscreen {
		states = append(states, uintptr(a.NET_WM_STATE_FULLSCREEN))
	}
	if len(states) > 0 {
		w.setProperty(a.NET_WM_STATE, XA_ATOM, 32, unsafe.Pointer(&states[0]), len(states))
	}
}

// SetAlwaysOnTop implements platform.Window: _NET_WM_STATE_ABOVE.
func (w *Window) SetAlwaysOnTop(on bool) {
	if w.xid != 0 {
		w.changeState(on, w.platform.atoms.NET_WM_STATE_ABOVE)
	}
}

// SetFullscreen implements platform.Window: _NET_WM_STATE_FULLSCREEN. The
// window manager answers with a new _NET_WM_STATE, which Fullscreen follows.
func (w *Window) SetFullscreen(on bool) {
	if w.xid != 0 {
		w.changeState(on, w.platform.atoms.NET_WM_STATE_FULLSCREEN)
	}
}

// changeState asks the window manager to add or remove a state of a mapped
// window.
func (w *Window) changeState(on bool, state Atom) {
	action := uintptr(netWMStateRemove)
	if on {
		action = netWMStateAdd
	}
	// Source 1 is an application.
	w.rootMessage(w.platform.atoms.NET_WM_STATE, action, uintptr(state), 0, 1)
}

// Fullscreen implements platform.Window. It is what _NET_WM_STATE last said.
func (w *Window) Fullscreen() bool { return w.state.fullscreen }

// Transparent implements platform.Window. Without a compositing manager the
// alpha shows as black.
func (w *Window) Transparent() bool { return w.state.colormap != 0 }

// readState updates maximized and fullscreen from _NET_WM_STATE, and reports
// whether either changed.
func (w *Window) readState() bool {
	a := &w.platform.atoms
	value, _ := w.platform.readProperty(w.xid, a.NET_WM_STATE, false)
	var vert, horz, full bool
	size := int(unsafe.Sizeof(uintptr(0)))
	for i := 0; i+size <= len(value); i += size {
		// value holds longs, as Xlib returns format 32.
		atom := Atom(*(*uintptr)(unsafe.Pointer(&value[i])))
		vert = vert || atom == a.NET_WM_STATE_MAXIMIZED_VERT
		horz = horz || atom == a.NET_WM_STATE_MAXIMIZED_HORZ
		full = full || atom == a.NET_WM_STATE_FULLSCREEN
	}
	// Maximized is only known for windows that show it on their own title
	// bar, as platform.Window says.
	maximized := w.customFrame && vert && horz
	changed := maximized != w.maximized || full != w.state.fullscreen
	w.maximized, w.state.fullscreen = maximized, full
	return changed
}

// wmClass returns WM_CLASS for appID: the instance and the class, each ended
// by a NUL.
func wmClass(appID string) string {
	if strings.IndexByte(appID, 0) >= 0 {
		panic("gowaku: app ID contains a NUL byte")
	}
	if appID == "" {
		appID = "gowaku"
	}
	return appID + "\x00" + appID + "\x00"
}
