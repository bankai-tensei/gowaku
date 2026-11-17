package gowakutest_test

import (
	"testing"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

// clippedList is a header over a list whose first row has a faint text, a Row
// that overflows and an unlabeled control: problems while it shows, and none
// once it is scrolled up under the header, out of the viewport.
func clippedList(c *basic.ScrollController) gowaku.Widget {
	rows := []gowaku.Widget{
		basic.SizedBox{Height: 30, Child: basic.Text{Content: "faint row", Style: ink(geom.Hex(0xbbbbbb))}},
		basic.SizedBox{Width: 100, Height: 20, Child: basic.Row{Children: []gowaku.Widget{
			basic.SizedBox{Width: 80, Height: 10}, basic.SizedBox{Width: 80, Height: 10},
		}}},
		interact.Pressable{OnPress: func() {}, Child: basic.SizedBox{Width: 20, Height: 20}},
	}
	for range 30 {
		rows = append(rows, basic.SizedBox{Height: 30, Child: basic.Text{Content: "filler", Style: ink(geom.Hex(0x111111))}})
	}
	return basic.Container{Color: geom.Hex(0xffffff), Child: basic.Column{Children: []gowaku.Widget{
		basic.Container{Color: geom.Hex(0x333333), Height: 40, Child: basic.Text{
			Content: "Header", Style: text.Style{Size: 14, Color: geom.Hex(0xffffff)},
		}},
		basic.Expanded{Child: basic.Scroll{Controller: c, Child: basic.Column{Children: rows}}},
	}}}
}

// TestWarnings_SkipWhatIsClippedAway: text, an overflow and an unlabeled
// control wholly outside the viewport of a Scroll are not reported, as nobody
// can see them, and they are again once they scroll back into view.
func TestWarnings_SkipWhatIsClippedAway(t *testing.T) {
	c := &basic.ScrollController{}
	tt := gowakutest.New(t, clippedList(c), gowakutest.Options{Width: 300, Height: 200, Fonts: goFonts.Fonts})
	shown := kinds(tt.Warnings())
	if shown[gowakutest.WarnContrast] != 1 || shown[gowakutest.WarnOverflow] != 1 || shown[gowakutest.WarnUnlabeled] != 1 {
		t.Fatalf("in view: got %v, want one of each", tt.Warnings())
	}
	c.JumpTo(200)
	tt.Pump()
	if ws := tt.Warnings(); len(ws) != 0 {
		t.Errorf("scrolled away: got %v, want none", ws)
	}
	c.JumpTo(0)
	tt.Pump()
	if back := kinds(tt.Warnings()); back[gowakutest.WarnContrast] != 1 || back[gowakutest.WarnOverflow] != 1 || back[gowakutest.WarnUnlabeled] != 1 {
		t.Errorf("back in view: got %v", back)
	}
}

// TestWarnings_JudgeOnlyWhatShows: a text cut in half by the viewport is
// measured against what shows around it, not against the header it slides
// under.
func TestWarnings_JudgeOnlyWhatShows(t *testing.T) {
	c := &basic.ScrollController{}
	tt := gowakutest.New(t, clippedList(c), gowakutest.Options{Width: 300, Height: 200, Fonts: goFonts.Fonts})
	// The third filler row, from 130 to 160 in the list, shows its last 5 px
	// below the dark header, and the row above it lies wholly under the
	// header: the dark text on white reads well where it shows.
	c.JumpTo(155)
	tt.Pump()
	if ws := tt.Warnings(); len(ws) != 0 {
		t.Errorf("got %v, want none", ws)
	}
}
