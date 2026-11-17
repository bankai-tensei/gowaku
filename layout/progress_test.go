package layout

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

func TestRingPath_Shapes(t *testing.T) {
	cases := []struct {
		name     string
		value    float32
		empty    bool
		contours int
	}{
		{"ゼロは何も描かない", 0, true, 0},
		{"負も何も描かない", -1, true, 0},
		{"途中は丸い端の 1 本", 0.3, false, 1},
		{"満タンは穴のある 2 本", 1, false, 2},
		{"1 を超えても満タン", 1.5, false, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var p paint.Path
			RingPath(&p, 100, 10, tc.value)
			if p.Empty() != tc.empty {
				t.Fatalf("empty = %v", p.Empty())
			}
			if tc.empty {
				return
			}
			_, starts := p.Polygon()
			if len(starts) != tc.contours {
				t.Errorf("got %d contours, want %d", len(starts), tc.contours)
			}
			b := p.Bounds()
			if b.X < -0.5 || b.Y < -0.5 || b.X+b.W > 100.5 || b.Y+b.H > 100.5 {
				t.Errorf("bounds %+v leave the square", b)
			}
		})
	}
}

func TestRingPath_FullRingFillsTheBox(t *testing.T) {
	var p paint.Path
	RingPath(&p, 100, 10, 1)
	b := p.Bounds()
	if b.W < 99.5 || b.H < 99.5 {
		t.Errorf("bounds %+v", b)
	}
}

func TestRenderProgressRing_ThicknessDefault(t *testing.T) {
	r := &RenderProgressRing{}
	if got := r.thickness(240); got != 20 {
		t.Errorf("got %v, want a twelfth", got)
	}
	if got := r.thickness(12); got != progressRingMinThickness {
		t.Errorf("got %v, want the minimum", got)
	}
	r.Thickness = 80
	if got := r.thickness(100); got != 50 {
		t.Errorf("got %v, want at most the radius", got)
	}
}

func BenchmarkRenderProgressRing_Paint(b *testing.B) {
	r := &RenderProgressRing{Value: 0.4, Active: geom.Hex(0xff0000), Track: geom.Hex(0xdddddd)}
	core.LayoutChild(r, geom.Tight(geom.Size{W: 200, H: 200}))
	var cv paint.Canvas
	cv.Reset()
	r.Paint(&cv, geom.Offset{})
	b.ReportAllocs()
	for b.Loop() {
		cv.Reset()
		r.Paint(&cv, geom.Offset{})
	}
}
