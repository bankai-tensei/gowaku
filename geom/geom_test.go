package geom

import (
	"image/color"
	"math"
	"testing"
)

func near(a, b float32) bool { return Abs32(a-b) < 1e-5 }

func TestRect_Intersect(t *testing.T) {
	tests := []struct {
		name string
		a, b Rect
		want Rect
	}{
		{"重なる", Rect{0, 0, 10, 10}, Rect{5, 5, 10, 10}, Rect{5, 5, 5, 5}},
		{"内包", Rect{0, 0, 10, 10}, Rect{2, 2, 2, 2}, Rect{2, 2, 2, 2}},
		{"辺で接するだけ", Rect{0, 0, 10, 10}, Rect{10, 0, 10, 10}, Rect{}},
		{"離れている", Rect{0, 0, 1, 1}, Rect{5, 5, 1, 1}, Rect{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Intersect(tt.b); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRect_ContainsExcludesFarEdges(t *testing.T) {
	r := Rect{1, 1, 2, 2}
	if !r.Contains(Offset{1, 1}) {
		t.Error("top-left corner must be inside")
	}
	if r.Contains(Offset{3, 1}) || r.Contains(Offset{1, 3}) {
		t.Error("right and bottom edges must be outside")
	}
}

func TestAffine_MulAppliesRightOperandFirst(t *testing.T) {
	m := Translation(Offset{10, 0}).Mul(Scaling(2, 2))
	if got, want := m.Apply(Offset{1, 1}), (Offset{12, 2}); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAffine_RotationIsClockwiseOnScreen(t *testing.T) {
	got := Rotation(math.Pi / 2).Apply(Offset{1, 0})
	if !near(got.X, 0) || !near(got.Y, 1) {
		t.Errorf("got %v, want (0, 1)", got)
	}
}

func TestAffine_IdentityIsNeutral(t *testing.T) {
	m := Affine{1, 2, 3, 4, 5, 6}
	if got := Identity().Mul(m); got != m {
		t.Errorf("got %v, want %v", got, m)
	}
	if got := m.Mul(Identity()); got != m {
		t.Errorf("got %v, want %v", got, m)
	}
}

func TestColor_Hex(t *testing.T) {
	if got, want := Hex(0x6750a4).Std(), (color.NRGBA{0x67, 0x50, 0xa4, 0xff}); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestColor_StdRoundTrip(t *testing.T) {
	for v := range 256 {
		in := color.NRGBA{uint8(v), uint8(255 - v), uint8(v / 2), uint8(v)}
		if got := FromStd(in).Std(); got != in {
			t.Fatalf("got %v, want %v", got, in)
		}
	}
}

func TestColor_StdClampsOutOfRange(t *testing.T) {
	if got, want := (Color{-1, 2, 0.5, 1}).Std(), (color.NRGBA{0, 255, 128, 255}); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMath32(t *testing.T) {
	if Abs32(-2) != 2 || Floor32(1.5) != 1 || Ceil32(1.5) != 2 || Sqrt32(9) != 3 {
		t.Error("float32 helpers disagree with math")
	}
	if Clamp32(5, 0, 1) != 1 || Clamp32(-5, 0, 1) != 0 || Clamp32(0.5, 0, 1) != 0.5 {
		t.Error("Clamp32 out of range")
	}
}
