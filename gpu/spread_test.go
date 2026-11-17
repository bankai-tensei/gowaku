package gpu

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

func TestSpreadOffset(t *testing.T) {
	cases := []struct {
		t    float32
		m    GradientMode
		want float32
	}{
		{1.25, 0, 1.25},
		{1.25, GradientRepeat, 0.25},
		{-0.25, GradientRepeat, 0.75},
		{1.25, GradientReflect, 0.75},
		{2.25, GradientReflect, 0.25},
		{-0.25, GradientReflect, 0.25},
		{0.5, GradientReflect | GradientRamp | GradientFocal, 0.5},
	}
	for _, c := range cases {
		if got := SpreadOffset(c.t, c.m); geom.Abs32(got-c.want) > 1e-6 {
			t.Errorf("SpreadOffset(%v, %v) = %v, want %v", c.t, c.m, got, c.want)
		}
	}
	if (GradientFocal | GradientRamp | GradientRepeat).Shape() != GradientFocal {
		t.Error("Shape must leave out the ramp and the spread")
	}
}

func TestFocalOffset(t *testing.T) {
	axis := geom.Offset{X: 10}
	f := geom.Offset{X: -5}
	for _, c := range []struct {
		p    geom.Offset
		want float32
	}{
		{f, 0},
		{geom.Offset{X: 10}, 1},
		{geom.Offset{X: -10}, 1},
		{geom.Offset{Y: 10}, 1},
		// Halfway from the focus to the edge on the far side.
		{geom.Offset{X: 2.5}, 0.5},
	} {
		if got := FocalOffset(c.p, axis, f); geom.Abs32(got-c.want) > 1e-4 {
			t.Errorf("FocalOffset(%v) = %v, want %v", c.p, got, c.want)
		}
	}
	// Without a focus it is the radial offset.
	if got := FocalOffset(geom.Offset{X: 3, Y: 4}, axis, geom.Offset{}); geom.Abs32(got-0.5) > 1e-4 {
		t.Errorf("got %v", got)
	}
}
