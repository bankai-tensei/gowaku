//go:build gpu

package wayland

import (
	"image"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/platform"
)

func TestCreateWindow_Decorations(t *testing.T) {
	p := newTestPlatform(t)
	icon := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	pw, err := p.CreateWindow(platform.WindowOptions{
		Title: "decorations", AppID: "dev.gowaku.Test", Icon: []*image.NRGBA{icon},
		MinSize: geom.Size{W: 200, H: 100}, MaxSize: geom.Size{W: 900}, Transparent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	w := pw.(*Window)
	mapWindow(t, w)
	drain(p, nil)
	if !w.Transparent() || !w.NativeHandle().(platform.WaylandHandle).Transparent {
		t.Error("a transparent window does not say so")
	}
	t.Logf("xdg-toplevel-icon-v1: %v", p.iconManager != nil)
	if p.iconManager != nil && w.icon.proxy == 0 {
		t.Error("the compositor has xdg-toplevel-icon-v1, but the window got no icon")
	}
	w.SetIcon(nil)
	w.SetSizeLimits(geom.Size{W: 300, H: 200}, geom.Size{})
	w.SetFullscreen(true)
	got := drain(p, func(ev *platform.Event) bool { return ev.Kind == platform.EventResize && w.Fullscreen() })
	if !got {
		t.Error("SetFullscreen(true) brought no configure with the fullscreen state")
	}
	w.SetFullscreen(false)
	drain(p, func(ev *platform.Event) bool { return ev.Kind == platform.EventResize && !w.Fullscreen() })
	if w.Fullscreen() {
		t.Error("SetFullscreen(false) left the window fullscreen")
	}
	w.Close()
}
