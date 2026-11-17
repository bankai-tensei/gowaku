package gowakutest_test

import (
	"slices"
	"testing"
	"time"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

// area is a labeled box of 120 by 80 to aim at.
func area(label string) gowaku.Widget {
	return interact.Semantics{Role: semantics.RoleGroup, Label: label, Child: basic.SizedBox{Width: 120, Height: 80}}
}

func TestHover_EntersAndLeaves(t *testing.T) {
	var entered, exited int
	tt := gowakutest.New(t, basic.Center{Child: interact.MouseRegion{
		OnEnter: func() { entered++ }, OnExit: func() { exited++ }, Child: area("Card"),
	}}, goFonts)
	tt.Hover("Card")
	if entered != 1 || exited != 0 {
		t.Fatalf("after Hover: entered %d, exited %d", entered, exited)
	}
	tt.HoverAt(geom.Offset{X: 1, Y: 1})
	if exited != 1 {
		t.Fatalf("HoverAt away: exited %d, want 1", exited)
	}
	tt.Hover("Card")
	tt.Unhover()
	if entered != 2 || exited != 2 {
		t.Errorf("after Unhover: entered %d, exited %d, want 2 and 2", entered, exited)
	}
}

// TestHover_TooltipNeedsAdvance: Hover settles, which does not wait for the
// delay of a tooltip; Advance does.
func TestHover_TooltipNeedsAdvance(t *testing.T) {
	tt := gowakutest.New(t, basic.Center{Child: overlay.Tooltip{Message: "Bold", Child: control.Button{Label: "B", OnPress: func() {}}}}, goFonts)
	tt.Hover("B")
	if _, ok := tt.FindRole(semantics.RoleTooltip, "Bold"); ok {
		t.Fatal("the tooltip opened without its delay")
	}
	tt.Advance(600 * time.Millisecond)
	if _, ok := tt.FindRole(semantics.RoleTooltip, "Bold"); !ok {
		t.Fatalf("no tooltip after the delay:\n%s", tt.Dump())
	}
}

func TestScroll_MovesDownAndZoomsWithCtrl(t *testing.T) {
	var c basic.ScrollController
	var zoom float32 = 1
	rows := make([]gowaku.Widget, 40)
	for i := range rows {
		rows[i] = basic.SizedBox{Height: 40}
	}
	tt := gowakutest.New(t, basic.Column{Children: []gowaku.Widget{
		interact.Semantics{Role: semantics.RoleGroup, Label: "List", Child: basic.SizedBox{Height: 200, Child: basic.Scroll{
			Controller: &c, Child: basic.Column{Children: rows},
		}}},
		interact.GestureDetector{OnPinch: func(f float32, _ geom.Offset) { zoom *= f }, Child: area("Map")},
	}}, goFonts)
	tt.Scroll("List", 0, -3, 0)
	if c.Offset() <= 0 {
		t.Errorf("offset %v after three notches down, want more than 0", c.Offset())
	}
	tt.Scroll("Map", 0, 1, input.ModControl)
	if zoom <= 1 {
		t.Errorf("zoom %v after Ctrl and the wheel away, want more than 1", zoom)
	}
}

func TestDrag_MovesASlider(t *testing.T) {
	var value float32 = 50
	tt := gowakutest.New(t, basic.Center{Child: basic.SizedBox{Width: 200, Child: control.Slider{
		Label: "Volume", Value: value, Max: 100, OnChange: func(v float32) { value = v },
	}}}, goFonts)
	tt.Drag("Volume", geom.Offset{X: 60})
	if value < 70 {
		t.Errorf("value %v after a drag to the right, want well above 50", value)
	}
}

func TestDropFiles_ReachesADragTarget(t *testing.T) {
	var got []string
	tt := gowakutest.New(t, basic.Center{Child: interact.DragTarget{
		Kinds:  input.DataFiles,
		OnDrop: func(d input.DragData, _ geom.Offset) { got = d.Files },
		Child:  area("Drop here"),
	}}, goFonts)
	if e := tt.DropText("Drop here", "words"); e != input.DropNone || got != nil {
		t.Fatalf("a target of files took text: %v, %v", e, got)
	}
	if e := tt.DropFiles("Drop here", "/a.txt", "/b.txt"); e != input.DropCopy {
		t.Errorf("effect %v, want DropCopy", e)
	}
	if !slices.Equal(got, []string{"/a.txt", "/b.txt"}) {
		t.Errorf("dropped %v", got)
	}
}

func TestFindRole_TellsApartTheSameLabel(t *testing.T) {
	pressed := 0
	tt := gowakutest.New(t, basic.VStack(
		basic.Text{Content: "Settings"},
		control.Button{Label: "Settings", OnPress: func() { pressed++ }},
	), goFonts)
	text, ok := tt.FindRole(semantics.RoleText, "Settings")
	if !ok {
		t.Fatalf("no text:\n%s", tt.Dump())
	}
	button, ok := tt.FindRole(semantics.RoleButton, "Settings")
	if !ok || button == text {
		t.Fatalf("no button apart from the text:\n%s", tt.Dump())
	}
	if n := len(tt.FindAllRole(semantics.RoleButton, "Settings")); n != 1 {
		t.Errorf("%d buttons, want 1", n)
	}
	if _, ok := tt.FindRole(semantics.RoleSlider, "Settings"); ok {
		t.Error("found a slider")
	}
	tt.TapAt(button.Rect.Center())
	if pressed != 1 {
		t.Errorf("pressed %d times", pressed)
	}
}

// todayLabel shows the word of the Today button of the locale.
type todayLabel struct{ gowaku.StatelessBase }

func (todayLabel) Build(ctx gowaku.BuildContext) gowaku.Widget {
	return basic.Text{Content: basic.StringsOf(ctx).Today}
}

func TestOptions_Locale(t *testing.T) {
	tt := gowakutest.New(t, todayLabel{}, goFonts)
	if _, ok := tt.Find("Today"); !ok {
		t.Errorf("the zero locale is not English:\n%s", tt.Dump())
	}
	tt = gowakutest.New(t, todayLabel{}, gowakutest.Options{Fonts: goFonts.Fonts, Locale: language.Japanese})
	if _, ok := tt.Find("今日"); !ok {
		t.Errorf("no Japanese word:\n%s", tt.Dump())
	}
	fr := &basic.Strings{Language: language.French, Today: "Aujourd'hui"}
	tt = gowakutest.New(t, todayLabel{}, gowakutest.Options{Fonts: goFonts.Fonts, Locale: language.French, Strings: []*basic.Strings{fr}})
	if _, ok := tt.Find("Aujourd'hui"); !ok {
		t.Errorf("Strings not used:\n%s", tt.Dump())
	}
}
