package gowaku

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/widgets/nav"
)

// TearOffWindow opens a window for a tab torn off a TabBar, as a browser does:
// placed so that the tab comes out under the pointer where it was let go, if
// root shows its TabBar where the bar the tab left has it, with the tab first.
// Where the window system does not let applications place windows, as on
// Wayland, the window opens where the system puts it. Call it from
// TabBar.OnTearOff, after taking the tab out of its bar:
//
//	OnTearOff: func(e nav.TabTearOff) {
//		if e.Alone {
//			win.MoveTo(e) // the window goes with its only tab
//			return
//		}
//		s.remove(e.Index)
//		gowaku.TearOffWindow(e, gowaku.WindowOptions{Title: "…"}, docWindow{tabs: []int{e.Key.(int)}})
//	}
//
// It must be called on the UI goroutine, during Run.
func TearOffWindow(e nav.TabTearOff, opts WindowOptions, root Widget) (*WindowHandle, error) {
	ap := currentApp.Load()
	if ap == nil {
		panic("gowaku: TearOffWindow called outside Run")
	}
	var place *geom.Offset
	if e.Placed {
		place = &e.Origin
	}
	w, err := ap.openPlaced(opts, root, nil, place)
	if err != nil {
		return nil, err
	}
	return &WindowHandle{ap, w}, nil
}

// MoveTo moves the window to where e says a window for its tab goes, so that
// a tab torn off a bar where it was alone takes its window along. It does
// nothing where the window system does not let applications place windows. It
// must be called on the UI goroutine.
func (h *WindowHandle) MoveTo(e nav.TabTearOff) {
	if w := h.window; e.Placed && h.app.windows[w.native.ID()] == w {
		w.native.MoveClientTo(int(geom.Floor32(e.Origin.X+0.5)), int(geom.Floor32(e.Origin.Y+0.5)))
	}
}

// Activate brings the window to the front and gives it the keyboard, as far as
// the window system lets an application do that. It must be called on the UI
// goroutine.
func (h *WindowHandle) Activate() {
	if w := h.window; h.app.windows[w.native.ID()] == w {
		w.native.Activate()
	}
}
