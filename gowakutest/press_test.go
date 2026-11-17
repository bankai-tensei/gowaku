package gowakutest_test

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

// TestPress_HoldsADragInProgress: between Press and Release the drag has
// started and moved but not ended, so a test can look at it on the way.
func TestPress_HoldsADragInProgress(t *testing.T) {
	var starts, ends int
	var moved geom.Offset
	tt := gowakutest.New(t, basic.Center{Child: interact.GestureDetector{
		OnDragStart:  func(geom.Offset) { starts++ },
		OnDragUpdate: func(_, d geom.Offset) { moved = moved.Add(d) },
		OnDragEnd:    func(geom.Offset) { ends++ },
		Child:        area("Handle"),
	}}, goFonts)
	n, _ := tt.Find("Handle")
	from := n.Rect.Center()
	tt.Press("Handle")
	if starts != 0 {
		t.Fatalf("a press alone started %d drags", starts)
	}
	tt.MoveTo(from.Add(geom.Offset{X: 20}))
	tt.MoveTo(from.Add(geom.Offset{X: 40, Y: 10}))
	if starts != 1 || ends != 0 {
		t.Fatalf("while held: %d starts, %d ends, want 1 and 0", starts, ends)
	}
	if moved != (geom.Offset{X: 40, Y: 10}) {
		t.Errorf("moved %v while held, want 40,10", moved)
	}
	_ = tt.Screenshot() // the drag on the way
	tt.Release()
	if ends != 1 {
		t.Errorf("after Release: %d ends, want 1", ends)
	}
}

// TestPress_SliderShowsItsValueWhileHeld: the value of a slider follows the
// held pointer before the release.
func TestPress_SliderShowsItsValueWhileHeld(t *testing.T) {
	var value float32
	tt := gowakutest.New(t, basic.Center{Child: basic.SizedBox{Width: 200, Child: sliderHolder{value: &value}}}, goFonts)
	n, ok := tt.FindFunc(func(n *gowakutest.Node) bool { return n.Role == semantics.RoleSlider })
	if !ok {
		t.Fatalf("no slider in\n%s", tt.Dump())
	}
	tt.PressAt(geom.Offset{X: n.Rect.X + 2, Y: n.Rect.Center().Y})
	tt.MoveTo(geom.Offset{X: n.Rect.X + n.Rect.W*0.75, Y: n.Rect.Center().Y})
	held := value
	tt.Release()
	if held < 0.5 {
		t.Errorf("value %v while held at three quarters, want past 0.5", held)
	}
}

// sliderHolder keeps the value of a control.Slider.
type sliderHolder struct {
	core.StatefulBase
	value *float32
}

func (sliderHolder) CreateState() core.State { return &sliderHolderState{} }

type sliderHolderState struct {
	core.StateBase
}

func (s *sliderHolderState) Build(core.BuildContext) core.Widget {
	h := s.Widget().(sliderHolder)
	return control.Slider{Label: "Volume", Value: *h.value, OnChange: func(v float32) {
		s.SetState(func() { *h.value = v })
	}}
}

// TestMiddleTap_PressesTheMiddleButton reaches a Listener with the middle
// button, down and up.
func TestMiddleTap_PressesTheMiddleButton(t *testing.T) {
	var buttons []input.Button
	tt := gowakutest.New(t, basic.Center{Child: interact.Listener{
		OnPointer: func(e *core.PointerEvent) {
			if e.Kind == core.PointerDown || e.Kind == core.PointerUp {
				buttons = append(buttons, e.Button)
			}
		},
		Child: area("Tab"),
	}}, goFonts)
	tt.MiddleTap("Tab")
	if len(buttons) != 2 || buttons[0] != input.ButtonMiddle || buttons[1] != input.ButtonMiddle {
		t.Errorf("buttons %v, want the middle one down and up", buttons)
	}
}

// TestFindAllFunc_FindsByRoleWithAnyLabel finds every button, whatever its
// label, in reading order.
func TestFindAllFunc_FindsByRoleWithAnyLabel(t *testing.T) {
	tt := gowakutest.New(t, basic.Column{Children: []core.Widget{
		control.Button{Label: "One", OnPress: func() {}},
		basic.Text{Content: "Between"},
		control.Button{Label: "Two", OnPress: func() {}},
	}}, goFonts)
	buttons := tt.FindAllFunc(func(n *gowakutest.Node) bool { return n.Role == semantics.RoleButton })
	if len(buttons) != 2 || buttons[0].Label != "One" || buttons[1].Label != "Two" {
		t.Errorf("got %d buttons in\n%s", len(buttons), tt.Dump())
	}
	if _, ok := tt.FindFunc(func(n *gowakutest.Node) bool { return n.Role == semantics.RoleSlider }); ok {
		t.Error("found a slider where there is none")
	}
}
