package basic

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

func TestDivider_Lines(t *testing.T) {
	th := theme.Default().Light
	tests := []struct {
		name string
		w    core.Widget
		want geom.Rect
	}{
		{"横はゼロ値で幅いっぱい", Column{Cross: layout.CrossStretch, Children: []core.Widget{Divider{}}}, geom.Rect{W: 400, H: 1}},
		{"横の字下げ", Column{Cross: layout.CrossStretch, Children: []core.Widget{Divider{Indent: 16, EndIndent: 8}}},
			geom.Rect{X: 16, W: 376, H: 1}},
		{"縦は高さいっぱい", Row{Size: layout.MainMin, Children: []core.Widget{
			SizedBox{Height: 30, Child: Divider{Axis: layout.Vertical, Indent: 7, EndIndent: 7}},
		}}, geom.Rect{Y: 7, W: 1, H: 16}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := wtest.TextTree(t, tt.w)
			got := wtest.Rrects(tree, wtest.Screen)
			if len(got) != 1 {
				t.Fatalf("got %d boxes", len(got))
			}
			if got[0].RRect.Rect != tt.want || got[0].Color != th.Color.Hair {
				t.Errorf("got %+v in %+v, want %+v in Hair", got[0].RRect.Rect, got[0].Color, tt.want)
			}
		})
	}
}

func TestDivider_UnboundedLengthDrawsNothing(t *testing.T) {
	tree := wtest.TextTree(t, Row{Size: layout.MainMin, Children: []core.Widget{Divider{}}})
	if got := wtest.Rrects(tree, wtest.Screen); len(got) != 0 {
		t.Errorf("a line of no length must draw nothing: %+v", got)
	}
}

func TestDivider_ColorReplacesHair(t *testing.T) {
	tree := wtest.TextTree(t, Column{Cross: layout.CrossStretch, Children: []core.Widget{Divider{Color: wtest.Red}}})
	if got := wtest.Rrects(tree, wtest.Screen); len(got) != 1 || got[0].Color != wtest.Red {
		t.Errorf("got %+v", got)
	}
}

func TestDivider_Semantics(t *testing.T) {
	tree := wtest.TextTree(t, Column{Cross: layout.CrossStretch, Children: []core.Widget{Divider{}}})
	wtest.Ops(tree, wtest.Screen)
	n := wtest.FindRole(tree.Semantics(), semantics.RoleSeparator)
	if n == nil {
		t.Fatal("no separator node")
	}
	if n.Rect != (geom.Rect{W: 400, H: 1}) || n.Actions != 0 || n.Label != "" {
		t.Errorf("got %+v", n)
	}
}
