package motion

import "github.com/bankai-tensei/gowaku/geom"

// Lerp blends from a to b. At t = 0 it returns a and at t = 1 it returns b. It
// must accept t outside that range, because curves overshoot.
type Lerp[T any] func(a, b T, t float32) T

// Tween is a value between Begin and End.
type Tween[T any] struct {
	Begin, End T
	// Lerp blends the two ends.
	Lerp Lerp[T]
}

// At returns the value at progress t.
func (w Tween[T]) At(t float32) T { return w.Lerp(w.Begin, w.End, t) }

// LerpFloat blends two numbers.
func LerpFloat(a, b, t float32) float32 { return a + (b-a)*t }

// LerpColor blends two colors on their premultiplied values. That way a fade
// from a transparent color does not pass through the black of its zero channels.
func LerpColor(a, b geom.Color, t float32) geom.Color {
	alpha := LerpFloat(a.A, b.A, t)
	if alpha <= 0 {
		return geom.Color{}
	}
	return geom.Color{
		R: geom.Clamp32(LerpFloat(a.R*a.A, b.R*b.A, t)/alpha, 0, 1),
		G: geom.Clamp32(LerpFloat(a.G*a.A, b.G*b.A, t)/alpha, 0, 1),
		B: geom.Clamp32(LerpFloat(a.B*a.A, b.B*b.A, t)/alpha, 0, 1),
		A: min(alpha, 1),
	}
}

// LerpOffset blends two offsets.
func LerpOffset(a, b geom.Offset, t float32) geom.Offset {
	return geom.Offset{X: LerpFloat(a.X, b.X, t), Y: LerpFloat(a.Y, b.Y, t)}
}

// LerpSize blends two sizes.
func LerpSize(a, b geom.Size, t float32) geom.Size {
	return geom.Size{W: LerpFloat(a.W, b.W, t), H: LerpFloat(a.H, b.H, t)}
}

// LerpRect blends two rectangles.
func LerpRect(a, b geom.Rect, t float32) geom.Rect {
	return geom.Rect{
		X: LerpFloat(a.X, b.X, t),
		Y: LerpFloat(a.Y, b.Y, t),
		W: LerpFloat(a.W, b.W, t),
		H: LerpFloat(a.H, b.H, t),
	}
}

// LerpRRect blends two rounded rectangles.
func LerpRRect(a, b geom.RRect, t float32) geom.RRect {
	return geom.RRect{
		Rect:        LerpRect(a.Rect, b.Rect, t),
		TopLeft:     LerpFloat(a.TopLeft, b.TopLeft, t),
		TopRight:    LerpFloat(a.TopRight, b.TopRight, t),
		BottomRight: LerpFloat(a.BottomRight, b.BottomRight, t),
		BottomLeft:  LerpFloat(a.BottomLeft, b.BottomLeft, t),
	}
}

// LerpInsets blends two insets.
func LerpInsets(a, b geom.Insets, t float32) geom.Insets {
	return geom.Insets{
		Left:   LerpFloat(a.Left, b.Left, t),
		Top:    LerpFloat(a.Top, b.Top, t),
		Right:  LerpFloat(a.Right, b.Right, t),
		Bottom: LerpFloat(a.Bottom, b.Bottom, t),
	}
}

// LerpAffine blends two transforms entry by entry. That is exact for translation
// and scale. Between two rotations it shrinks what it moves.
func LerpAffine(a, b geom.Affine, t float32) geom.Affine {
	return geom.Affine{
		A: LerpFloat(a.A, b.A, t),
		B: LerpFloat(a.B, b.B, t),
		C: LerpFloat(a.C, b.C, t),
		D: LerpFloat(a.D, b.D, t),
		E: LerpFloat(a.E, b.E, t),
		F: LerpFloat(a.F, b.F, t),
	}
}
