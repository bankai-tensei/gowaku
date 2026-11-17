package interact_test

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

// layered is a Stack over the whole screen: a control that fills it at the
// bottom, and layer on top. It counts the taps of each.
type layered struct {
	under, top int
}

func (l *layered) button(w, h float32) core.Widget {
	return interact.GestureDetector{OnTap: func() { l.top++ }, Child: basic.Container{Width: w, Height: h, Color: wtest.Red}}
}

func (l *layered) tree(t *testing.T, layer core.Widget) *core.Tree {
	t.Helper()
	return wtest.A11yTree(t, basic.Stack{Fit: layout.StackExpand, Children: []core.Widget{
		interact.GestureDetector{OnTap: func() { l.under++ }, Child: basic.Container{Color: wtest.Blue}},
		layer,
	}})
}

func TestStack_EmptySpaceOfALayerLetsThePressThrough(t *testing.T) {
	// The button sits at the bottom center of the 400x300 screen: 40x40 from
	// (180, 260).
	tests := []struct {
		name  string
		layer func(l *layered) core.Widget
	}{
		{"PositionedFill と Align", func(l *layered) core.Widget {
			return basic.PositionedFill{Child: basic.Align{Alignment: layout.AlignBottomCenter, Child: l.button(40, 40)}}
		}},
		{"全面の Column", func(l *layered) core.Widget {
			return basic.SizedBox{Width: 400, Height: 300, Child: basic.Column{
				Main: layout.MainEnd, Cross: layout.CrossCenter,
				Children: []core.Widget{basic.Padding{Insets: geom.Insets{Top: 8}, Child: l.button(40, 40)}},
			}}
		}},
		{"IgnorePointer の中の Center", func(_ *layered) core.Widget {
			return basic.PositionedFill{Child: basic.IgnorePointer{Child: basic.Center{Child: basic.Text{Content: "Loading"}}}}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &layered{}
			tree := l.tree(t, tt.layer(l))
			wtest.Tap(tree, geom.Offset{X: 20, Y: 20})
			wtest.Tap(tree, geom.Offset{X: 200, Y: 150})
			if l.under != 2 {
				t.Errorf("the control under the layer got %d of 2 taps on empty space", l.under)
			}
			if tt.name == "IgnorePointer の中の Center" {
				return
			}
			wtest.Tap(tree, geom.Offset{X: 200, Y: 280})
			if l.top != 1 || l.under != 2 {
				t.Errorf("a tap on the button: button %d, under %d; want 1 and 2", l.top, l.under)
			}
		})
	}
}

func TestStack_PaintedLayerTakesThePress(t *testing.T) {
	l := &layered{}
	// A Container with a color is hit over its whole box, as a card is.
	tree := l.tree(t, basic.PositionedFill{Child: basic.Align{Alignment: layout.AlignBottomCenter, Child: basic.Container{
		Width: 200, Height: 100, Color: wtest.Green, Padding: geom.Insets{Top: 40}, Child: basic.Center{Child: l.button(20, 20)},
	}}})
	wtest.Tap(tree, geom.Offset{X: 110, Y: 210})
	if l.under != 0 {
		t.Errorf("a tap on the colored box reached the control under it")
	}
	wtest.Tap(tree, geom.Offset{X: 20, Y: 20})
	if l.under != 1 {
		t.Errorf("a tap beside the box got %d taps under it, want 1", l.under)
	}
}
