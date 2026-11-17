//go:build gpu

package x11

import (
	"image"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/platform"
)

func TestCreateWindow_Decorations(t *testing.T) {
	p := newTestPlatform(t)
	icon := image.NewNRGBA(image.Rect(0, 0, 48, 48))
	pw, err := p.CreateWindow(platform.WindowOptions{
		Title: "decorations", AppID: "dev.gowaku.Test", Icon: []*image.NRGBA{icon},
		MinSize: geom.Size{W: 200, H: 100}, MaxSize: geom.Size{W: 900}, AlwaysOnTop: true, Transparent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	w := pw.(*Window)
	drain(p, nil)
	a := &p.atoms

	if class, _ := p.readProperty(w.xid, XA_WM_CLASS, false); !strings.HasPrefix(string(class), "dev.gowaku.Test\x00") {
		t.Errorf("WM_CLASS %q, want the app ID", class)
	}
	// 16, 24, 32 and 48 pixels, each with its width and height.
	if got, want := len(longs(p, w.xid, a.NET_WM_ICON)), 4*2+16*16+24*24+32*32+48*48; got != want {
		t.Errorf("_NET_WM_ICON has %d longs, want %d", got, want)
	}
	hints := longs(p, w.xid, XA_WM_NORMAL_HINTS)
	scale := p.scale
	if len(hints) < sizeHintsLongs || hints[0]&(PMinSize|PMaxSize) != PMinSize|PMaxSize ||
		hints[5] != Atom(200*scale+0.5) || hints[7] != Atom(900*scale+0.5) || hints[8] != unbounded {
		t.Errorf("WM_NORMAL_HINTS %v, want the bounds", hints)
	}
	state := longs(p, w.xid, a.NET_WM_STATE)
	above := false
	for _, s := range state {
		above = above || s == a.NET_WM_STATE_ABOVE
	}
	// Weston's xwm rewrites _NET_WM_STATE with what it knows once the window
	// is mapped, as it does with MODAL (docs/ADR/0047-modal-windows.md).
	t.Logf("the window manager keeps the above state: %v", above)
	t.Logf("transparent: %v (a visual with alpha)", w.Transparent())

	w.SetIcon(nil)
	if got := longs(p, w.xid, a.NET_WM_ICON); got != nil {
		t.Errorf("_NET_WM_ICON still has %d longs after SetIcon(nil)", len(got))
	}
	w.SetSizeLimits(geom.Size{}, geom.Size{})
	if hints = longs(p, w.xid, XA_WM_NORMAL_HINTS); hints[0]&(PMinSize|PMaxSize) != 0 {
		t.Errorf("WM_NORMAL_HINTS %v keeps bounds after SetSizeLimits of zero", hints[:9])
	}
	w.SetFullscreen(true)
	drain(p, nil)
	t.Logf("fullscreen after SetFullscreen(true): %v (needs a window manager that does EWMH)", w.Fullscreen())
	w.Close()
}
