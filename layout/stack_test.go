package layout

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
)

// fillStack is a stack sized by a 120x80 box, with a fill child over it.
func fillStack(in Insets) (*RenderStack, *RenderPositionedFill, *RenderProxy) {
	sizer := &RenderConstrainedBox{Extra: geom.Tight(geom.Size{W: 120, H: 80})}
	leaf := &RenderProxy{}
	fill := &RenderPositionedFill{Insets: in}
	fill.SetChild(leaf)
	s := &RenderStack{}
	s.SetChildren([]core.RenderObject{sizer, fill})
	return s, fill, leaf
}

func TestRenderStack_PositionedFillTakesTheStackLessInsets(t *testing.T) {
	tests := []struct {
		name     string
		in       Insets
		wantSize geom.Size
		wantOff  geom.Offset
	}{
		{"ゼロ値は全面", Insets{}, geom.Size{W: 120, H: 80}, geom.Offset{}},
		{"余白の分だけ縮む", Insets{Left: 10, Top: 5, Right: 20, Bottom: 15}, geom.Size{W: 90, H: 60}, geom.Offset{X: 10, Y: 5}},
		{"縦の線", Insets{Left: 30, Right: 89}, geom.Size{W: 1, H: 80}, geom.Offset{X: 30}},
		{"余白が大きすぎれば 0", Insets{Left: 100, Right: 100}, geom.Size{W: 0, H: 80}, geom.Offset{X: 100}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, fill, leaf := fillStack(tt.in)
			// Loose and larger than the sizer: the stack must still be 120x80.
			got := core.LayoutChild(s, Constraints{MaxW: 500, MaxH: 400})
			if got != (geom.Size{W: 120, H: 80}) {
				t.Errorf("stack size = %v, want 120x80", got)
			}
			if fill.Size() != tt.wantSize || leaf.Size() != tt.wantSize {
				t.Errorf("fill size = %v, child %v, want %v", fill.Size(), leaf.Size(), tt.wantSize)
			}
			if fill.Offset() != tt.wantOff {
				t.Errorf("fill offset = %v, want %v", fill.Offset(), tt.wantOff)
			}
		})
	}
}

func TestRenderStack_PositionedFillFollowsStackExpand(t *testing.T) {
	fill := &RenderPositionedFill{Insets: geom.All(4)}
	s := &RenderStack{Fit: StackExpand}
	s.SetChildren([]core.RenderObject{fill})
	core.LayoutChild(s, geom.Tight(geom.Size{W: 50, H: 30}))
	if fill.Size() != (geom.Size{W: 42, H: 22}) || fill.Offset() != (geom.Offset{X: 4, Y: 4}) {
		t.Errorf("fill = %v at %v, want 42x22 at (4,4)", fill.Size(), fill.Offset())
	}
}

func TestRenderStack_LayoutWithPositionedFillAllocatesNothing(t *testing.T) {
	s, _, _ := fillStack(Insets{Left: 3, Top: 4, Right: 5, Bottom: 6})
	c := Constraints{MaxW: 500, MaxH: 400}
	core.LayoutChild(s, c)
	allocs := testing.AllocsPerRun(100, func() { s.Layout(c) })
	if allocs != 0 {
		t.Errorf("allocs = %v, want 0", allocs)
	}
}
