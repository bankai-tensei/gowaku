package gowakutest_test

import (
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

func kinds(ws []gowakutest.Warning) map[gowakutest.WarningKind]int {
	m := map[gowakutest.WarningKind]int{}
	for _, w := range ws {
		m[w.Kind]++
	}
	return m
}

func ink(c geom.Color) text.Style { return text.Style{Size: 14, Color: c} }

func TestWarnings_CleanTreeHasNone(t *testing.T) {
	tt := gowakutest.New(t, basic.Container{Color: geom.Hex(0xffffff), Child: basic.VStack(
		basic.Text{Content: "readable", Style: ink(geom.Hex(0x111111))},
		control.Button{Label: "OK", OnPress: func() {}},
	)}, goFonts)
	if ws := tt.Warnings(); len(ws) != 0 {
		t.Errorf("got %v", ws)
	}
}

func TestWarnings_RowOverflow(t *testing.T) {
	tt := gowakutest.New(t, basic.Row{Children: []gowaku.Widget{
		basic.SizedBox{Width: 300, Height: 10}, basic.SizedBox{Width: 300, Height: 10},
	}}, gowakutest.Options{Width: 400, Height: 100, Fonts: goFonts.Fonts})
	ws := tt.Warnings()
	if len(ws) != 1 || ws[0].Kind != gowakutest.WarnOverflow || ws[0].Amount != 200 {
		t.Errorf("got %v, want one overflow of 200 px", ws)
	}
}

func TestWarnings_TextTallerThanItsBox(t *testing.T) {
	tt := gowakutest.New(t, basic.Align{Child: basic.SizedBox{Width: 100, Height: 6, Child: basic.Text{Content: "cut off", Style: ink(geom.Hex(0x111111))}}}, goFonts)
	ws := tt.Warnings()
	if kinds(ws)[gowakutest.WarnOverflow] != 1 || ws[0].Text != "cut off" {
		t.Errorf("got %v", ws)
	}
}

func TestWarnings_Unlabeled(t *testing.T) {
	tt := gowakutest.New(t, basic.VStack(
		interact.Pressable{OnPress: func() {}, Child: basic.SizedBox{Width: 20, Height: 20}},
		interact.Pressable{OnPress: func() {}, Child: basic.Text{Content: "named by its text", Style: ink(geom.Hex(0x111111))}},
	), goFonts)
	if got := kinds(tt.Warnings()); got[gowakutest.WarnUnlabeled] != 1 {
		t.Errorf("got %v, want the empty area only", got)
	}
}

func TestWarnings_LowContrast(t *testing.T) {
	tt := gowakutest.New(t, basic.Container{Color: geom.Hex(0xffffff), Child: basic.VStack(
		basic.Text{Content: "faint", Style: ink(geom.Hex(0xbbbbbb))},
		basic.Text{Content: "large and faint enough", Style: text.Style{Size: 24, Color: geom.Hex(0x8a8a8a)}},
	)}, goFonts)
	ws := tt.Warnings()
	if len(ws) != 1 || ws[0].Kind != gowakutest.WarnContrast || ws[0].Text != "faint" {
		t.Fatalf("got %v, want only the small faint text", ws)
	}
	if ws[0].Amount > 2 || ws[0].Amount < 1.8 {
		t.Errorf("contrast %v, want about 1.9 for #bbb on white", ws[0].Amount)
	}
}

func TestWarnings_HiddenIsNotChecked(t *testing.T) {
	tt := gowakutest.New(t, basic.Opacity{Child: basic.Row{Children: []gowaku.Widget{
		basic.SizedBox{Width: 900, Height: 10}, interact.Pressable{OnPress: func() {}, Child: basic.SizedBox{Width: 9, Height: 9}},
	}}}, goFonts)
	if ws := tt.Warnings(); len(ws) != 0 {
		t.Errorf("got %v", ws)
	}
}

// TestWarnings_GradientIsNotTheText: on a gradient no one color is common, and
// the text must still be measured against the background, not against itself.
func TestWarnings_GradientIsNotTheText(t *testing.T) {
	g := &paint.Gradient{Kind: paint.GradientLinear, StartColor: geom.Hex(0x2b1f6e), EndColor: geom.Hex(0x8a4fd6), End: geom.Offset{X: 400, Y: 120}}
	tt := gowakutest.New(t, basic.Align{Child: basic.Container{Width: 400, Height: 120, Gradient: g, Padding: geom.All(16), Child: basic.Text{
		Content: "Quiet night", Style: text.Style{Size: 36, Weight: 700, Color: geom.Hex(0xffffff)},
	}}}, goFonts)
	if ws := tt.Warnings(); len(ws) != 0 {
		t.Errorf("got %v", ws)
	}
}

// TestWarnings_DisabledControlsAreExempt: WCAG 1.4.3 exempts the text of
// inactive components, so a disabled button and the text inside a disabled
// group pass, while the same faint text outside them does not.
func TestWarnings_DisabledControlsAreExempt(t *testing.T) {
	faint := ink(geom.Hex(0xbbbbbb))
	tt := gowakutest.New(t, basic.Container{Color: geom.Hex(0xffffff), Child: basic.VStack(
		control.Button{Label: "Disabled"},
		interact.Semantics{Role: semantics.RoleGroup, Label: "Options", State: semantics.Disabled, Child: basic.Text{Content: "inside", Style: faint}},
		basic.Text{Content: "outside", Style: faint},
	)}, goFonts)
	ws := tt.Warnings()
	if len(ws) != 1 || ws[0].Kind != gowakutest.WarnContrast || ws[0].Text != "outside" {
		t.Errorf("got %v, want only the text outside the disabled controls", ws)
	}
}

// TestWarnings_ChildPastItsParent: a SegmentedControl in less room than its
// segments need lays them out past its own right edge. No Row or text says
// so; the child that crosses the edge of its parent does.
func TestWarnings_ChildPastItsParent(t *testing.T) {
	tt := gowakutest.New(t, basic.Align{Child: basic.SizedBox{Width: 160, Child: control.SegmentedControl{
		Label: "Priority", Options: []string{"None", "Low", "Medium", "High"}, OnChange: func(int) {},
	}}}, goFonts)
	ws := tt.Warnings()
	if len(ws) != 1 || ws[0].Kind != gowakutest.WarnOverflow || ws[0].Amount < 1 || !strings.Contains(ws[0].Detail, "right edge") {
		t.Errorf("got %v, want one overflow past the right edge", ws)
	}
}

// TestWarnings_PositionedMayHangOut: a Positioned child of a Stack is placed
// by hand, as a badge over the corner of an icon, and is no overflow.
func TestWarnings_PositionedMayHangOut(t *testing.T) {
	tt := gowakutest.New(t, basic.Align{Child: control.Badge{Count: 120, Child: basic.SizedBox{Width: 24, Height: 24}}}, goFonts)
	if ws := tt.Warnings(); kinds(ws)[gowakutest.WarnOverflow] != 0 {
		t.Errorf("got %v", ws)
	}
}
