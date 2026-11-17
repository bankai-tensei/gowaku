package nav_test

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/nav"
)

// slideRow is the row of the window where the flyouts are looked for.
const slideRow = 400

// slideDock is a DockLayout whose panel "files" is red and hides at the left,
// and "tools" is blue and hides at the right, over a group with a button.
type slideDock struct {
	ctrl    *nav.DockController
	presses int
}

func (d *slideDock) widget() core.Widget {
	fill := func(c geom.Color) func(core.BuildContext) core.Widget {
		return func(core.BuildContext) core.Widget { return basic.Container{Color: c} }
	}
	def := nav.DockArrangement{Root: nav.DockNode{Panels: []string{"main"}}, Hidden: []nav.DockHidden{
		{ID: "files", Side: nav.DockLeft, Size: 250},
		{ID: "tools", Side: nav.DockRight, Size: 250},
	}}
	return nav.DockLayout{Controller: d.ctrl, Default: def, Panels: []nav.DockPanel{
		{ID: "main", Title: "Main", Build: func(core.BuildContext) core.Widget {
			return basic.Align{Alignment: layout.AlignTopLeft, Child: control.Button{
				Label: "Under", OnPress: func() { d.presses++ },
			}}
		}},
		{ID: "files", Title: "Files", Build: fill(geom.RGBA(1, 0, 0, 1))},
		{ID: "tools", Title: "Tools", Build: fill(geom.RGBA(0, 0, 1, 1))},
	}}
}

func newSlideDock(t *testing.T, reduce bool) (*slideDock, *gowakutest.Tester) {
	t.Helper()
	d := &slideDock{ctrl: &nav.DockController{}}
	tt := gowakutest.New(t, d.widget(), gowakutest.Options{
		Width: 800, Height: 600, ReduceMotion: reduce, Fonts: [][]byte{goregular.TTF},
	})
	return d, tt
}

func isRedPixel(c color.NRGBA) bool  { return c.R > 200 && c.G < 60 && c.B < 60 }
func isBluePixel(c color.NRGBA) bool { return c.B > 200 && c.R < 60 && c.G < 60 }

// redEdge returns one past the rightmost red pixel on the slide row, or 0.
func redEdge(img *image.NRGBA) int {
	edge := 0
	for x := range img.Bounds().Dx() {
		if isRedPixel(img.NRGBAAt(x, slideRow)) {
			edge = x + 1
		}
	}
	return edge
}

// blueSeen reports whether the slide row has a blue pixel.
func blueSeen(img *image.NRGBA) bool {
	for x := range img.Bounds().Dx() {
		if isBluePixel(img.NRGBAAt(x, slideRow)) {
			return true
		}
	}
	return false
}

// shot saves img under $GOWAKU_SHOTS, to look at, when it is set.
func shot(t *testing.T, img *image.NRGBA, name string) {
	dir := os.Getenv("GOWAKU_SHOTS")
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDockLayout_FlyoutSlidesBackIn(t *testing.T) {
	d, tt := newSlideDock(t, false)
	under, ok := tt.Find("Under")
	if !ok {
		t.Fatal("no button")
	}
	d.ctrl.Show("files")
	tt.Settle()
	open := redEdge(tt.Screenshot())
	if open < 200 {
		t.Fatalf("the panel is out, its edge at %d", open)
	}
	if b := under.Rect; b.X+b.W > float32(open) {
		t.Fatalf("the button lies under the panel: %v, edge %d", b, open)
	}

	d.ctrl.HideFlyout()
	tt.Advance(90 * time.Millisecond)
	img := tt.Screenshot()
	shot(t, img, "dock-closing")
	mid := redEdge(img)
	if mid <= 0 || mid >= open {
		t.Fatalf("half way in the panel is still drawn, between 0 and %d, got %d", open, mid)
	}
	if _, ok := tt.FindRole(semantics.RoleGroup, "Files"); ok {
		t.Error("a screen reader does not find the panel sliding in")
	}
	// A press where the panel still shows goes through to the button below.
	tt.TapAt(geom.Offset{X: under.Rect.X + 4, Y: under.Rect.Y + under.Rect.H/2})
	if d.presses != 1 {
		t.Errorf("the press reaches what is under the panel, got %d presses", d.presses)
	}
	if d.ctrl.Flyout() != "" {
		t.Error("and does not bring it out again")
	}
	tt.Settle()
	if e := redEdge(tt.Screenshot()); e != 0 {
		t.Errorf("at the end it is gone, edge at %d", e)
	}
}

func TestDockLayout_FlyoutTurnsRoundAndGivesWay(t *testing.T) {
	d, tt := newSlideDock(t, false)
	d.ctrl.Show("files")
	tt.Settle()
	open := redEdge(tt.Screenshot())

	d.ctrl.HideFlyout()
	tt.Advance(100 * time.Millisecond)
	mid := redEdge(tt.Screenshot())
	d.ctrl.Show("files")
	tt.Advance(60 * time.Millisecond)
	if e := redEdge(tt.Screenshot()); e <= mid || e >= open {
		t.Errorf("shown again, it turns round from %d, got %d", mid, e)
	}
	if _, ok := tt.FindRole(semantics.RoleGroup, "Files"); !ok {
		t.Error("and is found again")
	}
	tt.Settle()
	if e := redEdge(tt.Screenshot()); e != open {
		t.Errorf("it ends out, at %d, got %d", open, e)
	}

	d.ctrl.HideFlyout()
	tt.Advance(70 * time.Millisecond)
	d.ctrl.Show("tools")
	tt.Advance(70 * time.Millisecond)
	img := tt.Screenshot()
	shot(t, img, "dock-replaced")
	if e := redEdge(img); e != 0 {
		t.Errorf("another panel coming out takes the place of the one going at once, red to %d", e)
	}
	if !blueSeen(img) {
		t.Error("the other panel slides out")
	}
}

func TestDockLayout_FlyoutReduceMotionHidesAtOnce(t *testing.T) {
	d, tt := newSlideDock(t, true)
	d.ctrl.Show("files")
	tt.Pump()
	if e := redEdge(tt.Screenshot()); e < 200 {
		t.Fatalf("the panel is out at once, edge %d", e)
	}
	d.ctrl.HideFlyout()
	tt.Pump()
	if e := redEdge(tt.Screenshot()); e != 0 {
		t.Errorf("with less motion it goes at once, edge %d", e)
	}
}
