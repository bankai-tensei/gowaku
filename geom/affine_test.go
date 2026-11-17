package geom

import (
	"math"
	"testing"
)

func nearOffset(a, b Offset) bool { return near(a.X, b.X) && near(a.Y, b.Y) }

func TestAffine_ZeroValueIsIdentity(t *testing.T) {
	if !(Affine{}).IsIdentity() || !Identity().IsIdentity() {
		t.Error("the zero value and Identity must report IsIdentity")
	}
	if Translation(Offset{1, 0}).IsIdentity() || Rotation(0.1).IsIdentity() {
		t.Error("a move or a turn is not the identity")
	}
}

func TestAffine_IsScaleTranslate(t *testing.T) {
	cases := []struct {
		name string
		m    Affine
		want bool
	}{
		{"ゼロ値", Affine{}, true},
		{"移動", Translation(Offset{3, 4}), true},
		{"等倍の拡大", Scaling(2, 2), true},
		{"縦横で違う拡大", Scaling(2, 3), false},
		{"回転", Rotation(0.5), false},
		{"歪み", Skewing(0.2, 0), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.m.IsScaleTranslate(); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestAffine_InvertUndoes(t *testing.T) {
	m := Translation(Offset{10, -4}).Mul(Rotation(0.7)).Mul(Skewing(0.3, -0.2)).Mul(Scaling(2, 0.5))
	inv, ok := m.Invert()
	if !ok {
		t.Fatal("an invertible transform reported singular")
	}
	for _, p := range []Offset{{0, 0}, {1, 2}, {-30, 17.5}} {
		if got := inv.Apply(m.Apply(p)); !nearOffset(got, p) {
			t.Errorf("inv(m(%v)) = %v", p, got)
		}
	}
	if got := m.Mul(inv); !nearOffset(got.Apply(Offset{5, 6}), Offset{5, 6}) {
		t.Errorf("m*inv = %v, want the identity", got)
	}
}

func TestAffine_InvertSingular(t *testing.T) {
	if _, ok := Scaling(0, 1).Invert(); ok {
		t.Error("a transform that flattens the plane must not invert")
	}
	if _, ok := (Affine{}).Invert(); ok {
		t.Error("the zero matrix itself is singular, whatever IsIdentity says")
	}
}

func TestAffine_ApplyRectIsBoundingBox(t *testing.T) {
	r := Rect{X: 0, Y: 0, W: 2, H: 2}
	got := Rotation(math.Pi / 4).ApplyRect(r)
	d := float32(math.Sqrt2)
	want := Rect{X: -d, Y: 0, W: 2 * d, H: 2 * d}
	if !near(got.X, want.X) || !near(got.Y, want.Y) || !near(got.W, want.W) || !near(got.H, want.H) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := Translation(Offset{3, 4}).ApplyRect(r); got != (Rect{X: 3, Y: 4, W: 2, H: 2}) {
		t.Errorf("a move must keep the size: got %v", got)
	}
}

func TestAffine_SkewingSlants(t *testing.T) {
	got := Skewing(math.Pi/4, 0).Apply(Offset{0, 2})
	if !nearOffset(got, Offset{2, 2}) {
		t.Errorf("got %v, want (2, 2)", got)
	}
}

func TestAffine_ApplyVectorIgnoresMove(t *testing.T) {
	m := Translation(Offset{100, 100}).Mul(Scaling(2, 3))
	if got := m.ApplyVector(Offset{1, 1}); got != (Offset{2, 3}) {
		t.Errorf("got %v, want (2, 3)", got)
	}
}

func TestAffine_MinScale(t *testing.T) {
	cases := []struct {
		name string
		m    Affine
		want float32
	}{
		{"恒等", Identity(), 1},
		{"回転は長さを変えない", Rotation(1.1), 1},
		{"縦横で違う拡大", Scaling(4, 0.25), 0.25},
		{"潰れた変換", Scaling(0, 3), 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.m.MinScale(); !near(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}
