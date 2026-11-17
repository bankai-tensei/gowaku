package gowakutest_test

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	fieldw "github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
)

// grower widens a labeled box when its Grow button is pressed.
type grower struct{ core.StatefulBase }

type growerState struct {
	core.StateBase
	wide bool
}

func (grower) CreateState() core.State { return &growerState{} }

func (s *growerState) Build(core.BuildContext) core.Widget {
	width := float32(40)
	if s.wide {
		width = 200
	}
	return basic.Column{Children: []core.Widget{
		control.Button{Label: "Grow", OnPress: func() { s.SetState(func() { s.wide = true }) }},
		interact.Semantics{Role: semantics.RoleGroup, Label: "box", Child: basic.AnimatedContainer{
			Width: width, Height: 20, Color: geom.Hex(0x336699),
		}},
	}}
}

func boxWidth(t *testing.T, tt *gowakutest.Tester) float32 {
	t.Helper()
	n, ok := tt.Find("box")
	if !ok {
		t.Fatal("no box")
	}
	return n.Rect.W
}

func TestReduceMotion_ResizeJumps(t *testing.T) {
	for _, reduce := range []bool{false, true} {
		tt := gowakutest.New(t, grower{}, gowakutest.Options{ReduceMotion: reduce})
		// Tap would settle the motion, so press by hand and look after a frame.
		n, _ := tt.Find("Grow")
		at := geom.Offset{X: n.Rect.X + n.Rect.W/2, Y: n.Rect.Y + n.Rect.H/2}
		tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at, Button: input.ButtonPrimary})
		tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: at, Button: input.ButtonPrimary})
		tt.Pump()
		// The first tick starts the clock of the motion.
		tt.Advance(16 * time.Millisecond)
		tt.Advance(50 * time.Millisecond)
		w := boxWidth(t, tt)
		switch {
		case reduce && w != 200:
			t.Errorf("with less motion the box must jump to 200, got %v", w)
		case !reduce && (w <= 40 || w >= 200):
			t.Errorf("the box must be on its way after a frame, got %v", w)
		}
	}
}

func TestPressTakesFocusWithoutRing(t *testing.T) {
	tt := gowakutest.New(t, basic.Column{Children: []core.Widget{
		fieldw.Input{Label: "件名"},
		control.Button{Label: "送る", OnPress: func() {}},
	}}, gowakutest.Options{})
	tt.Tap("件名")
	tt.Tap("送る")
	n, _ := tt.Find("送る")
	if n.State&semantics.Focused == 0 {
		t.Fatal("the pressed button must take the focus from the field")
	}
	if tt.Tree().FocusVisible() {
		t.Error("a press must not show the focus ring")
	}
	tt.Type("x")
	field := findRole(tt.Semantics(), semantics.RoleTextField)
	if field.Value != "" {
		t.Errorf("typing after the click reached the field: %q", field.Value)
	}
}

func findRole(n *gowakutest.Node, r semantics.Role) *gowakutest.Node {
	if n.Role == r {
		return n
	}
	for _, c := range n.Children {
		if f := findRole(c, r); f != nil {
			return f
		}
	}
	return nil
}
