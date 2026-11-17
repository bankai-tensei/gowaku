package layout

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
)

func TestAlignNamed_Within(t *testing.T) {
	outer, child := geom.Size{W: 200, H: 100}, geom.Size{W: 20, H: 10}
	cases := []struct {
		name string
		a    Alignment
		want geom.Offset
	}{
		{"AlignTopLeft", AlignTopLeft, geom.Offset{X: 0, Y: 0}},
		{"AlignTopCenter", AlignTopCenter, geom.Offset{X: 90, Y: 0}},
		{"AlignTopRight", AlignTopRight, geom.Offset{X: 180, Y: 0}},
		{"AlignCenterLeft", AlignCenterLeft, geom.Offset{X: 0, Y: 45}},
		{"AlignCenter", AlignCenter, geom.Offset{X: 90, Y: 45}},
		{"AlignCenterRight", AlignCenterRight, geom.Offset{X: 180, Y: 45}},
		{"AlignBottomLeft", AlignBottomLeft, geom.Offset{X: 0, Y: 90}},
		{"AlignBottomCenter", AlignBottomCenter, geom.Offset{X: 90, Y: 90}},
		{"AlignBottomRight", AlignBottomRight, geom.Offset{X: 180, Y: 90}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.a.Within(outer, child); got != c.want {
				t.Errorf("Within = %v, want %v", got, c.want)
			}
		})
	}
}

func TestAlignNamed_ZeroIsTopLeft(t *testing.T) {
	if (Alignment{}) != AlignTopLeft {
		t.Errorf("the zero Alignment is not AlignTopLeft")
	}
}

// An overshooting curve carries an AnimatedAlign past 1, so RenderAlign must
// place the child there instead of stopping.
func TestAlignNamed_OutOfRangeIsAllowed(t *testing.T) {
	child := &RenderConstrainedBox{Extra: Constraints{MinW: 20, MaxW: 20, MinH: 10, MaxH: 10}}
	r := &RenderAlign{Alignment: Alignment{X: 1.1, Y: -0.1}}
	r.SetChild(child)
	core.LayoutChild(r, geom.Tight(geom.Size{W: 120, H: 110}))
	if got, want := child.Offset(), (geom.Offset{X: 110, Y: -10}); got != want {
		t.Errorf("child offset = %v, want %v", got, want)
	}
}
