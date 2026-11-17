package interact_test

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
)

// tile is a Pressable that shows a FocusRing, the way an app's own tile does.
func tile(label string, onPress func(), skip bool, onFocus func(bool)) core.Widget {
	return interact.Pressable{Label: label, OnPress: onPress, SkipTab: skip, OnFocusChange: onFocus,
		Build: func(s interact.PressState) core.Widget {
			return interact.FocusRing{Visible: s.Focused, Radius: 8, Child: basic.SizedBox{Width: 100, Height: 40}}
		}}
}

func TestFocusRing_ShowsWithTheFocus(t *testing.T) {
	tt := gowakutest.New(t, basic.Center{Child: tile("Tile", func() {}, false, nil)}, pressableFonts)
	n, _ := tt.Find("Tile")
	// The ring is 3px wide just outside the box: look 1.5px out of its top edge.
	x, y := int(n.Rect.X+n.Rect.W/2), int(n.Rect.Y-1.5)
	before := tt.Screenshot().NRGBAAt(x, y)
	tt.Key(input.KeyTab, 0)
	tt.Settle()
	after := tt.Screenshot().NRGBAAt(x, y)
	if before == after {
		t.Fatalf("no ring at (%d, %d) after Tab: %v", x, y, after)
	}
	tt.Tree().RequestFocus(nil)
	tt.Settle()
	if gone := tt.Screenshot().NRGBAAt(x, y); gone != before {
		t.Errorf("the ring stays after the focus left: %v, want %v", gone, before)
	}
}

func TestPressable_OnFocusChange(t *testing.T) {
	var changes []bool
	tt := gowakutest.New(t, basic.HStack(
		tile("A", func() {}, false, func(f bool) { changes = append(changes, f) }),
		tile("B", func() {}, false, nil),
	), pressableFonts)
	tt.Key(input.KeyTab, 0)
	tt.Key(input.KeyTab, 0)
	if len(changes) != 2 || !changes[0] || changes[1] {
		t.Errorf("got %v, want [true false]", changes)
	}
}

func TestPressable_SkipTab(t *testing.T) {
	presses := map[string]int{}
	tt := gowakutest.New(t, basic.HStack(
		tile("A", func() { presses["A"]++ }, true, nil),
		tile("B", func() { presses["B"]++ }, false, nil),
	), pressableFonts)
	tt.Key(input.KeyTab, 0)
	tt.Key(input.KeySpace, 0)
	if presses["B"] != 1 || presses["A"] != 0 {
		t.Errorf("Tab did not skip A: %v", presses)
	}
	tt.Tap("A")
	if presses["A"] != 1 {
		t.Errorf("the pointer does not reach a SkipTab area: %v", presses)
	}
}
