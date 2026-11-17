package geom

import "math"

// Sqrt32 returns the square root of x.
func Sqrt32(x float32) float32 { return float32(math.Sqrt(float64(x))) }

// Abs32 returns the absolute value of x.
func Abs32(x float32) float32 { return math.Float32frombits(math.Float32bits(x) &^ (1 << 31)) }

// Floor32 returns the greatest integer value less than or equal to x.
func Floor32(x float32) float32 { return float32(math.Floor(float64(x))) }

// Ceil32 returns the least integer value greater than or equal to x.
func Ceil32(x float32) float32 { return float32(math.Ceil(float64(x))) }

// Sin32 returns the sine of the radian argument x.
func Sin32(x float32) float32 { return float32(math.Sin(float64(x))) }

// Cos32 returns the cosine of the radian argument x.
func Cos32(x float32) float32 { return float32(math.Cos(float64(x))) }

// Tan32 returns the tangent of the radian argument x.
func Tan32(x float32) float32 { return float32(math.Tan(float64(x))) }

// Clamp32 returns x limited to the range [lo, hi].
func Clamp32(x, lo, hi float32) float32 { return max(lo, min(x, hi)) }
