// Ported from material-color-utilities, typescript/hct/hct_solver.ts.
// https://github.com/material-foundation/material-color-utilities
// Copyright 2021 Google LLC. Licensed under the Apache License, Version 2.0;
// see LICENSE.material-color-utilities in this directory.
// Changed for gowaku: CRITICAL_PLANES is computed instead of tabulated, and the
// "not found" result is a bool instead of the color 0.

package theme

import "math"

// scaledDiscountFromLinRGB takes linear sRGB to cone responses, with the
// discount and scale of the viewing conditions folded in.
func scaledDiscountFromLinRGB() [3][3]float64 {
	return [3][3]float64{
		{0.001200833568784504, 0.002389694492170889, 0.0002795742885861124},
		{0.0005891086651375999, 0.0029785502573438758, 0.0003270666104008398},
		{0.00010146692491640572, 0.0005364214359186694, 0.0032979401770712076},
	}
}

// linRGBFromScaledDiscount is the inverse of scaledDiscountFromLinRGB.
func linRGBFromScaledDiscount() [3][3]float64 {
	return [3][3]float64{
		{1373.2198709594231, -1100.4251190754821, -7.278681089101213},
		{-271.815969077903, 559.6580465940733, -32.46047482791194},
		{1.9622899599665666, -57.173814538844006, 308.7233197812385},
	}
}

// solveRGB finds the 0xRRGGBB color with the given hue, chroma and L*. When that
// color is outside sRGB, hue and L* are kept and chroma is as high as sRGB allows.
func solveRGB(hue, chroma, lstar float64) uint32 {
	if chroma < 0.0001 || lstar < 0.0001 || lstar > 99.9999 {
		return rgbFromLstar(lstar)
	}
	hueRadians := sanitizeDegrees(hue) / 180 * math.Pi
	y := yFromLstar(lstar)
	if rgb, ok := findResultByJ(hueRadians, chroma, y); ok {
		return rgb
	}
	return rgbFromLinear(bisectToLimit(y, hueRadians))
}

// findResultByJ runs Newton's method on J and reports whether it reached an
// in-gamut color with the wanted hue, chroma and Y.
func findResultByJ(hueRadians, chroma, y float64) (uint32, bool) {
	vc := defaultViewing()
	fromScaled := linRGBFromScaledDiscount()

	// The initial estimate of J.
	j := math.Sqrt(y) * 11
	tInnerCoeff := 1 / math.Pow(1.64-math.Pow(0.29, vc.n), 0.73)
	eHue := 0.25 * (math.Cos(hueRadians+2) + 3.8)
	p1 := eHue * (50000.0 / 13.0) * vc.nc * vc.ncb
	hSin, hCos := math.Sin(hueRadians), math.Cos(hueRadians)

	for round := range 5 {
		jNormalized := j / 100
		alpha := 0.0
		if chroma != 0 && j != 0 {
			alpha = chroma / math.Sqrt(jNormalized)
		}
		t := math.Pow(alpha*tInnerCoeff, 1/0.9)
		ac := vc.aw * math.Pow(jNormalized, 1/vc.c/vc.z)
		p2 := ac / vc.nbb
		gamma := 23 * (p2 + 0.305) * t / (23*p1 + 11*t*hCos + 108*t*hSin)
		a := gamma * hCos
		b := gamma * hSin
		rA := (460*p2 + 451*a + 288*b) / 1403
		gA := (460*p2 - 891*a - 261*b) / 1403
		bA := (460*p2 - 220*a - 6300*b) / 1403

		linear := multiply(&fromScaled, [3]float64{
			inverseChromaticAdaptation(rA),
			inverseChromaticAdaptation(gA),
			inverseChromaticAdaptation(bA),
		})
		if linear[0] < 0 || linear[1] < 0 || linear[2] < 0 {
			return 0, false
		}
		fnj := yFromLinR*linear[0] + yFromLinG*linear[1] + yFromLinB*linear[2]
		if fnj <= 0 {
			return 0, false
		}
		if round == 4 || math.Abs(fnj-y) < 0.002 {
			if linear[0] > 100.01 || linear[1] > 100.01 || linear[2] > 100.01 {
				return 0, false
			}
			return rgbFromLinear(linear), true
		}
		// 2 * fn(j) / j approximates fn'(j).
		j -= (fnj - y) * j / (2 * fnj)
	}
	return 0, false
}

// bisectToLimit finds the color with the given Y and hue on the boundary of the
// RGB cube, in linear RGB.
func bisectToLimit(y, targetHue float64) [3]float64 {
	left, right := bisectToSegment(y, targetHue)
	leftHue := hueOf(left)
	for axis := range 3 {
		if left[axis] == right[axis] {
			continue
		}
		var lPlane, rPlane int
		if left[axis] < right[axis] {
			lPlane = criticalPlaneBelow(trueDelinearized(left[axis]))
			rPlane = criticalPlaneAbove(trueDelinearized(right[axis]))
		} else {
			lPlane = criticalPlaneAbove(trueDelinearized(left[axis]))
			rPlane = criticalPlaneBelow(trueDelinearized(right[axis]))
		}
		for range 8 {
			if max(lPlane, rPlane)-min(lPlane, rPlane) <= 1 {
				break
			}
			// The planes are at least -1 and two apart, so the sum is not
			// negative and the division floors.
			mPlane := (lPlane + rPlane) / 2
			mid := setCoordinate(left, criticalPlane(mPlane), right, axis)
			midHue := hueOf(mid)
			if areInCyclicOrder(leftHue, targetHue, midHue) {
				right = mid
				rPlane = mPlane
			} else {
				left = mid
				leftHue = midHue
				lPlane = mPlane
			}
		}
	}
	return [3]float64{
		(left[0] + right[0]) / 2,
		(left[1] + right[1]) / 2,
		(left[2] + right[2]) / 2,
	}
}

// bisectToSegment finds the edge of the polygon where the plane of Y cuts the
// RGB cube that contains the target hue, and returns its two endpoints.
func bisectToSegment(y, targetHue float64) (left, right [3]float64) {
	var leftHue, rightHue float64
	initialized := false
	uncut := true
	for n := range 12 {
		mid, ok := nthVertex(y, n)
		if !ok {
			continue
		}
		midHue := hueOf(mid)
		if !initialized {
			left, right = mid, mid
			leftHue, rightHue = midHue, midHue
			initialized = true
			continue
		}
		if uncut || areInCyclicOrder(leftHue, midHue, rightHue) {
			uncut = false
			if areInCyclicOrder(leftHue, targetHue, midHue) {
				right, rightHue = mid, midHue
			} else {
				left, leftHue = mid, midHue
			}
		}
	}
	return left, right
}

// nthVertex returns possible vertex n, 0 <= n < 12, of the polygon where the
// plane of Y cuts the RGB cube, and reports whether it lies on the cube.
func nthVertex(y float64, n int) ([3]float64, bool) {
	coordA, coordB := 100.0, 100.0
	if n%4 <= 1 {
		coordA = 0
	}
	if n%2 == 0 {
		coordB = 0
	}
	var v [3]float64
	var free float64
	switch {
	case n < 4:
		free = (y - coordA*yFromLinG - coordB*yFromLinB) / yFromLinR
		v = [3]float64{free, coordA, coordB}
	case n < 8:
		free = (y - coordB*yFromLinR - coordA*yFromLinB) / yFromLinG
		v = [3]float64{coordB, free, coordA}
	default:
		free = (y - coordA*yFromLinR - coordB*yFromLinG) / yFromLinB
		v = [3]float64{coordA, coordB, free}
	}
	return v, 0 <= free && free <= 100
}

// hueOf returns the CAM16 hue in radians of a linear RGB color.
func hueOf(linear [3]float64) float64 {
	toScaled := scaledDiscountFromLinRGB()
	scaled := multiply(&toScaled, linear)
	rA := chromaticAdaptation(scaled[0])
	gA := chromaticAdaptation(scaled[1])
	bA := chromaticAdaptation(scaled[2])
	// redness-greenness and yellowness-blueness
	a := (11*rA + -12*gA + bA) / 11
	b := (rA + gA - 2*bA) / 9
	return math.Atan2(b, a)
}

func chromaticAdaptation(component float64) float64 {
	af := math.Pow(math.Abs(component), 0.42)
	return signum(component) * 400 * af / (af + 27.13)
}

func inverseChromaticAdaptation(response float64) float64 {
	abs := math.Abs(response)
	base := max(0, 27.13*abs/(400-abs))
	return signum(response) * math.Pow(base, 1/0.42)
}

// sanitizeRadians returns the angle in [0, 2pi) for a small enough angle.
func sanitizeRadians(angle float64) float64 {
	return math.Mod(angle+math.Pi*8, math.Pi*2)
}

func areInCyclicOrder(a, b, c float64) bool {
	return sanitizeRadians(b-a) < sanitizeRadians(c-a)
}

// setCoordinate intersects the segment from source to target with the plane
// where the given axis equals coordinate.
func setCoordinate(source [3]float64, coordinate float64, target [3]float64, axis int) [3]float64 {
	t := (coordinate - source[axis]) / (target[axis] - source[axis])
	return [3]float64{
		source[0] + (target[0]-source[0])*t,
		source[1] + (target[1]-source[1])*t,
		source[2] + (target[2]-source[2])*t,
	}
}

// criticalPlane returns the linear value at which an 8-bit channel rounds from
// i to i+1. Bisecting on these planes stops as soon as the rounded color is
// settled, which is what upstream's CRITICAL_PLANES table is for.
func criticalPlane(i int) float64 { return linearized(float64(i) + 0.5) }

func criticalPlaneBelow(x float64) int { return int(math.Floor(x - 0.5)) }

func criticalPlaneAbove(x float64) int { return int(math.Ceil(x - 0.5)) }
