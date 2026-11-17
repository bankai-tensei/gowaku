package gowakutest_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

// menuList is a list of 40 px buttons in a Scroll view of height, over
// anything below it; each press counts in presses.
func menuList(height float32, presses map[string]int) gowaku.Widget {
	var items []gowaku.Widget
	for i := range 30 {
		label := "Item " + strconv.Itoa(i)
		items = append(items, basic.SizedBox{Height: 40, Child: control.Button{Label: label, OnPress: func() { presses[label]++ }}})
	}
	return basic.Column{Children: []gowaku.Widget{
		basic.SizedBox{Height: height, Child: basic.Scroll{Child: basic.Column{Children: items}}},
	}}
}

func TestTap_BelowTheWindowFails(t *testing.T) {
	presses := map[string]int{}
	msg := failure(t, func(tb testing.TB) {
		tt := gowakutest.New(tb, menuList(600, presses), gowakutest.Options{Width: 300, Height: 200, Fonts: goFonts.Fonts})
		tt.Tap("Item 20")
	})
	for _, want := range []string{"Tap", `"Item 20"`, "y=800", "below", "300x200 window", "scroll it into view"} {
		if !strings.Contains(msg, want) {
			t.Errorf("failure %q does not say %q", msg, want)
		}
	}
	if presses["Item 20"] != 0 {
		t.Error("the item was pressed")
	}
}

func TestTap_ClippedAwayByAScrollFails(t *testing.T) {
	presses := map[string]int{}
	msg := failure(t, func(tb testing.TB) {
		tt := gowakutest.New(tb, menuList(100, presses), gowakutest.Options{Width: 300, Height: 400, Fonts: goFonts.Fonts})
		tt.DoubleTap("Item 5")
	})
	for _, want := range []string{"DoubleTap", `"Item 5"`, "cut away", "layout.RenderViewport", "scroll it into view"} {
		if !strings.Contains(msg, want) {
			t.Errorf("failure %q does not say %q", msg, want)
		}
	}
}

// TestTap_PartlyShownTapsWhatShows: the third item, from 80 to 120, shows
// only above the bottom of the 100 px view, where its middle is not.
func TestTap_PartlyShownTapsWhatShows(t *testing.T) {
	presses := map[string]int{}
	tt := gowakutest.New(t, menuList(100, presses), gowakutest.Options{Width: 300, Height: 400, Fonts: goFonts.Fonts})
	tt.Tap("Item 2")
	if presses["Item 2"] != 1 {
		t.Errorf("pressed %v, want Item 2 once", presses)
	}
}

// covered is a button under a pressable cover that fills the window.
func covered(pressed *int) gowaku.Widget {
	return basic.Stack{Fit: layout.StackExpand, Children: []gowaku.Widget{
		basic.Center{Child: control.Button{Label: "Under", OnPress: func() { *pressed++ }}},
		interact.Semantics{Role: semantics.RoleButton, Label: "Cover", Child: interact.Pressable{
			OnPress: func() {}, Child: basic.SizedBox{Width: 800, Height: 600},
		}},
	}}
}

func TestTap_CoveredFails(t *testing.T) {
	var pressed int
	msg := failure(t, func(tb testing.TB) {
		tt := gowakutest.New(tb, covered(&pressed), goFonts)
		tt.Tap("Under")
	})
	for _, want := range []string{"Tap", `"Under"`, "covered", `"Cover"`} {
		if !strings.Contains(msg, want) {
			t.Errorf("failure %q does not say %q", msg, want)
		}
	}
	if pressed != 0 {
		t.Error("the button under the cover was pressed")
	}
}

func TestDropText_BelowTheWindowFails(t *testing.T) {
	msg := failure(t, func(tb testing.TB) {
		tt := gowakutest.New(tb, menuList(600, map[string]int{}), gowakutest.Options{Width: 300, Height: 200, Fonts: goFonts.Fonts})
		tt.DropText("Item 20", "lemon")
	})
	if !strings.Contains(msg, "DropText") || !strings.Contains(msg, "below") {
		t.Errorf("failure %q does not say DropText and below", msg)
	}
}

func TestTapAt_OutsideTheWindowFails(t *testing.T) {
	for _, pos := range []geom.Offset{{X: 10, Y: 200}, {X: -1, Y: 10}, {X: 300, Y: 10}, {X: 10, Y: -5}} {
		msg := failure(t, func(tb testing.TB) {
			tt := gowakutest.New(tb, menuList(600, map[string]int{}), gowakutest.Options{Width: 300, Height: 200, Fonts: goFonts.Fonts})
			tt.TapAt(pos)
		})
		if !strings.Contains(msg, "TapAt") || !strings.Contains(msg, "300x200 window") {
			t.Errorf("TapAt(%v): failure %q does not name the window", pos, msg)
		}
	}
	// A raw position inside the window taps whatever is there, or nothing.
	tt := gowakutest.New(t, menuList(100, map[string]int{}), gowakutest.Options{Width: 300, Height: 400, Fonts: goFonts.Fonts})
	tt.TapAt(geom.Offset{X: 10, Y: 300})
}
