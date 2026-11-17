package control_test

import (
	"testing"

	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/icons"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

func mountButton(t *testing.T, b control.Button) *gowakutest.Tester {
	t.Helper()
	return gowakutest.New(t, basic.Align{Alignment: layout.AlignTopLeft, Child: b}, gowakutest.Options{})
}

func TestButton_HideLabelDrawsTheIconInASquare(t *testing.T) {
	for _, c := range []struct {
		size control.ButtonSize
		side float32
	}{{control.ButtonRegular, 30}, {control.ButtonSmall, 24}, {control.ButtonLarge, 38}} {
		for _, style := range []control.ButtonStyle{control.ButtonNormal, control.ButtonPrimary, control.ButtonQuiet, control.ButtonDanger} {
			pressed := 0
			tt := mountButton(t, control.Button{Label: "Copy", Icon: icons.Copy(), HideLabel: true, Size: c.size, Style: style,
				OnPress: func() { pressed++ }})
			n, ok := tt.FindRole(semantics.RoleButton, "Copy")
			if !ok {
				t.Fatalf("no button named Copy:\n%s", tt.Dump())
			}
			if n.Rect.W != c.side || n.Rect.H != c.side {
				t.Errorf("size %v, style %v: %vx%v, want a square of %v", c.size, style, n.Rect.W, n.Rect.H, c.side)
			}
			if _, ok := tt.FindRole(semantics.RoleText, "Copy"); ok {
				t.Errorf("the label is drawn:\n%s", tt.Dump())
			}
			for _, w := range tt.Warnings() {
				t.Errorf("size %v, style %v: %v", c.size, style, w)
			}
			tt.Tap("Copy")
			if pressed != 1 {
				t.Errorf("pressed %d times, want 1", pressed)
			}
		}
	}
}

func TestButton_HideLabelWithoutAnIconShowsTheLabel(t *testing.T) {
	tt := mountButton(t, control.Button{Label: "Copy", HideLabel: true, OnPress: func() {}})
	n, ok := tt.FindRole(semantics.RoleButton, "Copy")
	if !ok || n.Rect.W <= 30 {
		t.Errorf("a button without an icon keeps its label:\n%s", tt.Dump())
	}
}

func TestButton_HideLabelDisabled(t *testing.T) {
	tt := mountButton(t, control.Button{Label: "Copy", Icon: icons.Copy(), HideLabel: true})
	if n, _ := tt.FindRole(semantics.RoleButton, "Copy"); n == nil || n.State&semantics.Disabled == 0 {
		t.Errorf("a button without OnPress is disabled: %v", n)
	}
}
