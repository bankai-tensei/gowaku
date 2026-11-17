package gowakutest_test

import (
	"math"
	"testing"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

// twoBands is a dark band with white text over a white band with dark text,
// 400 by 150 each: no problem of contrast where each text lies.
func twoBands() gowaku.Widget {
	return basic.Column{Children: []gowaku.Widget{
		basic.Container{Color: geom.Hex(0x202060), Width: 400, Height: 150, Child: basic.Text{Content: "on dark", Style: ink(geom.Hex(0xffffff))}},
		basic.Container{Color: geom.Hex(0xffffff), Width: 400, Height: 150, Child: basic.Text{Content: "on light", Style: ink(geom.Hex(0x303030))}},
	}}
}

// TestWarnings_FollowTheZoomOfAViewer zooms a viewer by 2 at the top left, so
// that the dark band fills it and the light band lies below it. The dark text
// on light was measured where it lies unzoomed, over the zoomed dark band, and
// reported; it must be left out, as it shows nowhere, and the white text on
// dark measured where it shows.
func TestWarnings_FollowTheZoomOfAViewer(t *testing.T) {
	c := &interact.ViewerController{}
	root := basic.SizedBox{Width: 400, Height: 300, Child: interact.InteractiveViewer{
		Controller: c, Constrained: true, MaxScale: 4, Child: twoBands(),
	}}
	tt := gowakutest.New(t, root, gowakutest.Options{Width: 400, Height: 300, Fonts: goFonts.Fonts})
	if ws := tt.Warnings(); len(ws) != 0 {
		t.Fatalf("unzoomed: got %v, want none", ws)
	}
	c.Set(2, geom.Offset{}, false)
	tt.Pump()
	if c.Scale() != 2 {
		t.Fatalf("scale %v, want 2", c.Scale())
	}
	if ws := tt.Warnings(); len(ws) != 0 {
		t.Errorf("zoomed: got %v, want none", ws)
	}
	// Panned so that the light band shows from y 100: its text shows there.
	c.Set(2, geom.Offset{Y: -200}, false)
	tt.Pump()
	if ws := tt.Warnings(); len(ws) != 0 {
		t.Errorf("zoomed and panned: got %v, want none", ws)
	}
}

// TestWarnings_FaintTextInAZoomedViewer is found where the zoom shows it, as
// large as it shows: 14 px text zoomed by 2 is large text, which needs only
// 3:1, and 0x999999 on white, at 2.85:1, is below that too.
func TestWarnings_FaintTextInAZoomedViewer(t *testing.T) {
	c := &interact.ViewerController{}
	root := basic.SizedBox{Width: 400, Height: 300, Child: interact.InteractiveViewer{
		Controller: c, Constrained: true, MaxScale: 4,
		Child: basic.Container{Color: geom.Hex(0xffffff), Child: basic.Padding{
			Insets: layout.Insets{Left: 50, Top: 50},
			Child:  basic.Text{Content: "faint", Style: ink(geom.Hex(0x999999))},
		}},
	}}
	tt := gowakutest.New(t, root, gowakutest.Options{Width: 400, Height: 300, Fonts: goFonts.Fonts})
	before, _ := tt.Find("faint")
	plain := before.Rect
	c.Set(2, geom.Offset{}, false)
	tt.Pump()
	ws := tt.Warnings()
	if len(ws) != 1 || ws[0].Kind != gowakutest.WarnContrast {
		t.Fatalf("got %v, want one contrast warning", ws)
	}
	// Where the viewer shows the text, as its semantics say, twice as large.
	after, _ := tt.Find("faint")
	want := after.Rect
	if r := ws[0].Rect; math.Abs(float64(r.X-want.X)) > 1 || math.Abs(float64(r.Y-want.Y)) > 1 ||
		math.Abs(float64(r.W-plain.W*2)) > 1 || math.Abs(float64(r.H-plain.H*2)) > 1 {
		t.Errorf("rect %+v, want the zoomed place %+v", r, want)
	}
	if ws[0].Amount > 3 {
		t.Errorf("ratio %v", ws[0].Amount)
	}
}

// TestWarnings_FollowAPaintedTranslate: a Translate moves the dark text on
// light down onto the dark band only in the picture; it is measured there.
func TestWarnings_FollowAPaintedTranslate(t *testing.T) {
	root := basic.Column{Children: []gowaku.Widget{
		basic.Container{Color: geom.Hex(0xffffff), Width: 300, Height: 100, Child: basic.Align{Child: basic.Translate{Y: 5,
			Child: basic.Text{Content: "moved", Style: ink(geom.Hex(0x303030))}}}},
		basic.Container{Color: geom.Hex(0x202060), Width: 300, Height: 200},
	}}
	tt := gowakutest.New(t, root, gowakutest.Options{Width: 300, Height: 300, Fonts: goFonts.Fonts})
	ws := tt.Warnings()
	if len(ws) != 1 || ws[0].Kind != gowakutest.WarnContrast || ws[0].Rect.Y < 100 {
		t.Errorf("got %v, want the moved text reported over the dark band", ws)
	}
}

// TestWarnings_SkipTurnedText: the upright bounds of a turned text hold what
// is around it, so its contrast is not judged.
func TestWarnings_SkipTurnedText(t *testing.T) {
	root := basic.Container{Color: geom.Hex(0xffffff), Child: basic.Center{Child: basic.Rotate(math.Pi/5,
		basic.Text{Content: "turned faint", Style: ink(geom.Hex(0xcccccc))})}}
	tt := gowakutest.New(t, root, gowakutest.Options{Width: 300, Height: 300, Fonts: goFonts.Fonts})
	if ws := tt.Warnings(); len(ws) != 0 {
		t.Errorf("got %v, want none", ws)
	}
}
