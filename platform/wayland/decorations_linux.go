package wayland

import (
	"image"
	"slices"

	"golang.org/x/sys/unix"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/platform/internal/icon"
)

// The icon, size limits, always on top, fullscreen and transparency of a
// window. The icon goes through xdg-toplevel-icon-v1 where the compositor has
// it; elsewhere the compositor takes the icon of the desktop file that the
// app_id names. Always on top has no protocol. See
// docs/ADR/0111-window-decorations.md.

// iconManager is the xdg_toplevel_icon_manager_v1 of the compositor and the
// sizes it prefers, in surface pixels.
type iconManager struct {
	xdgToplevelIconManagerV1Nop
	proxy xdgToplevelIconManagerV1
	sizes []int32
}

// xdgToplevelIconManagerV1IconSize implements xdgToplevelIconManagerV1Listener.
func (m *iconManager) xdgToplevelIconManagerV1IconSize(size int32) {
	if size > 0 && !slices.Contains(m.sizes, size) {
		m.sizes = append(m.sizes, size)
	}
}

// toplevelIcon is an icon given to a toplevel, with the buffers that must
// outlive it.
type toplevelIcon struct {
	proxy   xdgToplevelIconV1
	buffers []wlBuffer
}

// destroy destroys the icon before its buffers, as the protocol requires.
func (t *toplevelIcon) destroy(c *conn) {
	if t.proxy != 0 {
		t.proxy.destroy(c)
	}
	for _, b := range t.buffers {
		b.destroy(c)
	}
	*t = toplevelIcon{}
}

// SetIcon implements platform.Window. Without xdg-toplevel-icon-v1 it does
// nothing, and the compositor shows the icon of the desktop file of the
// app_id.
func (w *Window) SetIcon(images []*image.NRGBA) {
	w.icons = images
	if w.toplevel == 0 || w.p.iconManager == nil {
		return
	}
	w.applyIcon()
	w.surface.commit(w.p.c)
	w.p.c.hot(w.p.c.lib.flush, w.p.c.display)
}

// applyIcon gives the toplevel an icon of w.icons in the sizes the compositor
// prefers and those of the ladder, or takes it away. It takes effect with the
// next commit. The icon given before is destroyed once it is replaced.
func (w *Window) applyIcon() {
	c, m := w.p.c, w.p.iconManager
	old := w.icon
	w.icon = toplevelIcon{}
	if icon.Largest(w.icons) == 0 {
		m.proxy.setIcon(c, w.toplevel, 0)
		old.destroy(c)
		return
	}
	sizes := icon.Ladder(w.icons)
	for _, s := range m.sizes {
		if !slices.Contains(sizes, int(s)) {
			sizes = append(sizes, int(s))
		}
	}
	t := toplevelIcon{proxy: m.proxy.createIcon(c)}
	for _, size := range sizes {
		if b, ok := w.p.iconBuffer(icon.Fit(w.icons, size)); ok {
			t.buffers = append(t.buffers, b)
			t.proxy.addBuffer(c, b, 1)
		}
	}
	m.proxy.setIcon(c, w.toplevel, t.proxy)
	w.icon = t
	old.destroy(c)
}

// iconBuffer copies img to a new wl_shm buffer of premultiplied ARGB8888, as
// Wayland takes, in the byte order of the machine: blue first on little-endian
// ones. The pool goes at once; the buffer keeps its memory.
func (p *Platform) iconBuffer(img *image.NRGBA) (wlBuffer, bool) {
	size := img.Rect.Dx()
	length := size * size * 4
	fd, err := unix.MemfdCreate("gowaku-icon", unix.MFD_CLOEXEC)
	if err != nil {
		return 0, false
	}
	// The compositor has its own mapping of the pool once it is made.
	defer unix.Close(fd) //nolint:errcheck // nothing to do about a failed close of a memfd
	if unix.Ftruncate(fd, int64(length)) != nil {
		return 0, false
	}
	data, err := unix.Mmap(fd, 0, length, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		return 0, false
	}
	premultiplyBGRA(data, img)
	_ = unix.Munmap(data) //nolint:errcheck // the pages stay in the memfd either way
	c := p.c
	pool := p.shm.createPool(c, int32(fd), int32(length))
	buffer := pool.createBuffer(c, 0, int32(size), int32(size), int32(size*4), wlShmFormatArgb8888)
	pool.destroy(c)
	return buffer, true
}

// premultiplyBGRA writes the pixels of img to dst as premultiplied B, G, R, A
// bytes, which ARGB8888 is in memory on a little-endian machine.
func premultiplyBGRA(dst []byte, img *image.NRGBA) {
	size := img.Rect.Dx()
	for y := range img.Rect.Dy() {
		row := img.Pix[y*img.Stride:]
		for x := range size {
			s, d := row[4*x:4*x+4], dst[4*(y*size+x):]
			a := uint32(s[3])
			d[0] = byte((uint32(s[2])*a + 127) / 255)
			d[1] = byte((uint32(s[1])*a + 127) / 255)
			d[2] = byte((uint32(s[0])*a + 127) / 255)
			d[3] = s[3]
		}
	}
}

// SetSizeLimits implements platform.Window: set_min_size and set_max_size of
// xdg_toplevel, in logical pixels of the window geometry, which is the whole
// surface. The compositor keeps the window within them.
func (w *Window) SetSizeLimits(minSize, maxSize geom.Size) {
	w.minSize, w.maxSize = minSize, maxSize
	if w.toplevel == 0 {
		return
	}
	w.applySizeLimits()
	w.surface.commit(w.p.c)
	w.p.c.hot(w.p.c.lib.flush, w.p.c.display)
}

// applySizeLimits sends the limits, for the next commit. Zero is no limit on
// an axis, as in the protocol.
func (w *Window) applySizeLimits() {
	c := w.p.c
	w.toplevel.setMinSize(c, int32(w.minSize.W+0.5), int32(w.minSize.H+0.5))
	w.toplevel.setMaxSize(c, int32(w.maxSize.W+0.5), int32(w.maxSize.H+0.5))
}

// SetAlwaysOnTop implements platform.Window. Wayland has no protocol for it,
// and the compositor decides what stays on top.
func (w *Window) SetAlwaysOnTop(bool) {}

// SetFullscreen implements platform.Window: set_fullscreen on the output the
// compositor picks, or unset_fullscreen. The configure that follows carries the
// state.
func (w *Window) SetFullscreen(on bool) {
	if w.toplevel == 0 {
		return
	}
	if on {
		w.toplevel.setFullscreen(w.p.c, 0)
	} else {
		w.toplevel.unsetFullscreen(w.p.c)
	}
	w.p.c.hot(w.p.c.lib.flush, w.p.c.display)
}

// Fullscreen implements platform.Window: what the last configure said.
func (w *Window) Fullscreen() bool { return w.fullscreen }

// Transparent implements platform.Window. Every Wayland compositor blends
// surfaces with their alpha, so it is what the window asked for.
func (w *Window) Transparent() bool { return w.transparent }
