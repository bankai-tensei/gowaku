// Ported from material-color-utilities, typescript/utils/color_utils.ts and
// typescript/utils/math_utils.ts.
// https://github.com/material-foundation/material-color-utilities
// Copyright 2021 Google LLC. Licensed under the Apache License, Version 2.0;
// see LICENSE.material-color-utilities in this directory.
// Changed for gowaku: channels are taken as floats, and only the functions that
// HCT needs are kept.

package theme

import (
	"math"

	"github.com/bankai-tensei/gowaku/geom"
)

// Luminance weights of the linear sRGB channels.
const (
	yFromLinR = 0.2126
	yFromLinG = 0.7152
	yFromLinB = 0.0722
)

// channel255 returns a geom.Color channel on the 0 to 255 scale.
//
// A channel that came from an 8-bit value is off by the rounding of float32.
// Snapping it back makes every 8-bit color exactly the input upstream sees, so
// that the results agree with upstream to the last bit.
func channel255(v float32) float64 {
	x := float64(geom.Clamp32(v, 0, 1)) * 255
	if n := math.Round(x); math.Abs(x-n) < 1e-4 {
		return n
	}
	return x
}

// linearized converts an sRGB channel from 0..255 to linear light on 0..100.
func linearized(channel float64) float64 {
	normalized := channel / 255
	if normalized <= 0.040449936 {
		return normalized / 12.92 * 100
	}
	return math.Pow((normalized+0.055)/1.055, 2.4) * 100
}

// trueDelinearized converts linear light on 0..100 to an sRGB channel on 0..255
// without rounding.
func trueDelinearized(linear float64) float64 {
	normalized := linear / 100
	if normalized <= 0.0031308 {
		return normalized * 12.92 * 255
	}
	return (1.055*math.Pow(normalized, 1.0/2.4) - 0.055) * 255
}

// delinearized converts linear light on 0..100 to an 8-bit sRGB channel. Values
// outside the gamut, and NaN, end up in range.
func delinearized(linear float64) uint32 {
	v := math.Round(trueDelinearized(linear))
	switch {
	case v >= 255:
		return 255
	case v > 0:
		return uint32(v)
	}
	return 0
}

// rgbFromLinear returns the 0xRRGGBB value of a linear color on 0..100.
func rgbFromLinear(linear [3]float64) uint32 {
	return delinearized(linear[0])<<16 | delinearized(linear[1])<<8 | delinearized(linear[2])
}

// rgbFromLstar returns the 0xRRGGBB value of the gray with the given L*.
func rgbFromLstar(lstar float64) uint32 {
	c := delinearized(yFromLstar(lstar))
	return c<<16 | c<<8 | c
}

// yFromLstar converts L* to the Y of XYZ, both on 0..100.
func yFromLstar(lstar float64) float64 { return 100 * labInvF((lstar+16)/116) }

// lstarFromY converts the Y of XYZ to L*, both on 0..100.
func lstarFromY(y float64) float64 { return labF(y/100)*116 - 16 }

func labF(t float64) float64 {
	const (
		e     = 216.0 / 24389.0
		kappa = 24389.0 / 27.0
	)
	if t > e {
		return math.Cbrt(t)
	}
	return (kappa*t + 16) / 116
}

func labInvF(ft float64) float64 {
	const (
		e     = 216.0 / 24389.0
		kappa = 24389.0 / 27.0
	)
	ft3 := ft * ft * ft
	if ft3 > e {
		return ft3
	}
	return (116*ft - 16) / kappa
}

// sanitizeDegrees returns the angle in [0, 360) that is coterminal with degrees.
func sanitizeDegrees(degrees float64) float64 {
	degrees = math.Mod(degrees, 360)
	if degrees < 0 {
		degrees += 360
	}
	return degrees
}

// signum returns -1, 0 or 1 by the sign of x.
func signum(x float64) float64 {
	switch {
	case x < 0:
		return -1
	case x > 0:
		return 1
	}
	return 0
}

// multiply returns the product of the matrix m and the column vector v.
func multiply(m *[3][3]float64, v [3]float64) [3]float64 {
	return [3]float64{
		v[0]*m[0][0] + v[1]*m[0][1] + v[2]*m[0][2],
		v[0]*m[1][0] + v[1]*m[1][1] + v[2]*m[1][2],
		v[0]*m[2][0] + v[1]*m[2][1] + v[2]*m[2][2],
	}
}
