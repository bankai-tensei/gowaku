package layout

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

func TestAspectRatioSize(t *testing.T) {
	inf := geom.Inf
	tests := []struct {
		name  string
		c     Constraints
		ratio float32
		want  geom.Size
	}{
		{"幅いっぱいで高さを決める", Constraints{MaxW: 320, MaxH: 400}, 16.0 / 9, geom.Size{W: 320, H: 180}},
		{"高さが足りなければ高さで決める", Constraints{MaxW: 320, MaxH: 90}, 16.0 / 9, geom.Size{W: 160, H: 90}},
		{"幅が無限なら高さから", Constraints{MaxW: inf, MaxH: 100}, 2, geom.Size{W: 200, H: 100}},
		{"高さが無限でも幅から", Constraints{MaxW: 100, MaxH: inf}, 0.5, geom.Size{W: 100, H: 200}},
		{"ゼロ値は正方形", Constraints{MaxW: 100, MaxH: 300}, 0, geom.Size{W: 100, H: 100}},
		{"最小幅に届かなければ広げる", Constraints{MinW: 200, MaxW: 300, MaxH: 50}, 1, geom.Size{W: 200, H: 50}},
		{"最小の高さに届かなければ伸ばす", Constraints{MaxW: 100, MinH: 80, MaxH: 200}, 2, geom.Size{W: 100, H: 80}},
		{"tight は比を無視する", geom.Tight(geom.Size{W: 50, H: 70}), 2, geom.Size{W: 50, H: 70}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AspectRatioSize(tt.c, tt.ratio)
			if geom.Abs32(got.W-tt.want.W) > 1e-3 || geom.Abs32(got.H-tt.want.H) > 1e-3 {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRenderAspectRatio_ChildGetsTheSizeTight(t *testing.T) {
	child := &RenderConstrainedBox{}
	r := &RenderAspectRatio{Ratio: 4.0 / 3}
	r.SetChild(child)
	got := r.Layout(Constraints{MaxW: 120, MaxH: 120})
	if want := (geom.Size{W: 120, H: 90}); got != want || child.Size() != want {
		t.Errorf("size %v, child %v, want %v", got, child.Size(), want)
	}
}

func TestAspectRatioSize_Panics(t *testing.T) {
	for name, f := range map[string]func(){
		"負の比":  func() { AspectRatioSize(Constraints{MaxW: 10, MaxH: 10}, -1) },
		"両方無限": func() { AspectRatioSize(Constraints{MaxW: geom.Inf, MaxH: geom.Inf}, 1) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("did not panic")
				}
			}()
			f()
		})
	}
}
