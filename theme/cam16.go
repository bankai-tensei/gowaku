// Ported from material-color-utilities, typescript/hct/cam16.ts and
// typescript/hct/viewing_conditions.ts.
// https://github.com/material-foundation/material-color-utilities
// Copyright 2021 Google LLC. Licensed under the Apache License, Version 2.0;
// see LICENSE.material-color-utilities in this directory.
// Changed for gowaku: only the default viewing conditions and the forward
// transform are kept, since HCT needs nothing else.

package theme

import "math"

// viewingConditions holds the CAM16 parameters that depend on the environment.
type viewingConditions struct {
	n, aw, nbb, ncb, c, nc, fl, flRoot, z float64
	rgbD                                  [3]float64
}

// defaultViewing returns the conditions HCT is defined under: a D65 white point,
// an average surround, and a mid gray background of L* 50. It is cheap next to a
// solve, so it is recomputed instead of being kept in a package variable.
func defaultViewing() viewingConditions {
	const (
		whiteX   = 95.047
		whiteY   = 100.0
		whiteZ   = 108.883
		surround = 2.0
		f        = 0.8 + surround/10
		// c interpolates 0.59 to 0.69 by (f-0.9)*10, which is 1 at this surround.
		c = 0.69
	)
	adaptingLuminance := 200 / math.Pi * yFromLstar(50) / 100

	white := [3]float64{
		whiteX*0.401288 + whiteY*0.650173 + whiteZ*-0.051461,
		whiteX*-0.250268 + whiteY*1.204414 + whiteZ*0.045854,
		whiteX*-0.002079 + whiteY*0.048952 + whiteZ*0.953127,
	}
	d := f * (1 - 1/3.6*math.Exp((-adaptingLuminance-42)/92))
	d = max(0, min(d, 1))

	var rgbD [3]float64
	for i, w := range white {
		rgbD[i] = d*(100/w) + 1 - d
	}

	k := 1 / (5*adaptingLuminance + 1)
	k4 := k * k * k * k
	k4F := 1 - k4
	fl := k4*adaptingLuminance + 0.1*k4F*k4F*math.Cbrt(5*adaptingLuminance)
	n := yFromLstar(50) / whiteY
	nbb := 0.725 / math.Pow(n, 0.2)

	var rgbA [3]float64
	for i, w := range white {
		factor := math.Pow(fl*rgbD[i]*w/100, 0.42)
		rgbA[i] = 400 * factor / (factor + 27.13)
	}

	return viewingConditions{
		n:      n,
		aw:     (2*rgbA[0] + rgbA[1] + 0.05*rgbA[2]) * nbb,
		nbb:    nbb,
		ncb:    nbb,
		c:      c,
		nc:     f,
		fl:     fl,
		flRoot: math.Pow(fl, 0.25),
		z:      1.48 + math.Sqrt(n),
		rgbD:   rgbD,
	}
}

// cam16 is a color in the CAM16 appearance model.
type cam16 struct {
	hue    float64 // degrees in [0, 360)
	chroma float64
	j      float64 // lightness
	q      float64 // brightness
	m      float64 // colorfulness
	s      float64 // saturation
}

// cam16FromLinear converts a linear sRGB color on 0..100 under the default
// viewing conditions.
func cam16FromLinear(linear [3]float64) cam16 {
	vc := defaultViewing()
	r, g, b := linear[0], linear[1], linear[2]

	x := 0.41233895*r + 0.35762064*g + 0.18051042*b
	y := yFromLinR*r + yFromLinG*g + yFromLinB*b
	z := 0.01932141*r + 0.11916382*g + 0.95034478*b

	rD := vc.rgbD[0] * (0.401288*x + 0.650173*y - 0.051461*z)
	gD := vc.rgbD[1] * (-0.250268*x + 1.204414*y + 0.045854*z)
	bD := vc.rgbD[2] * (-0.002079*x + 0.048952*y + 0.953127*z)

	rA := adapted(vc.fl, rD)
	gA := adapted(vc.fl, gD)
	bA := adapted(vc.fl, bD)

	// redness-greenness and yellowness-blueness
	ca := (11*rA + -12*gA + bA) / 11
	cb := (rA + gA - 2*bA) / 9
	u := (20*rA + 20*gA + 21*bA) / 20
	p2 := (40*rA + 20*gA + bA) / 20

	hue := sanitizeDegrees(math.Atan2(cb, ca) * 180 / math.Pi)

	ac := p2 * vc.nbb
	j := 100 * math.Pow(ac/vc.aw, vc.c*vc.z)
	q := 4 / vc.c * math.Sqrt(j/100) * (vc.aw + 4) * vc.flRoot

	huePrime := hue
	if hue < 20.14 {
		huePrime += 360
	}
	eHue := 0.25 * (math.Cos(huePrime*math.Pi/180+2) + 3.8)
	p1 := 50000.0 / 13.0 * eHue * vc.nc * vc.ncb
	t := p1 * math.Sqrt(ca*ca+cb*cb) / (u + 0.305)
	alpha := math.Pow(t, 0.9) * math.Pow(1.64-math.Pow(0.29, vc.n), 0.73)
	chroma := alpha * math.Sqrt(j/100)

	return cam16{
		hue:    hue,
		chroma: chroma,
		j:      j,
		q:      q,
		m:      chroma * vc.flRoot,
		s:      50 * math.Sqrt(alpha*vc.c/(vc.aw+4)),
	}
}

// adapted applies the CAM16 post-adaptation compression to one cone response.
func adapted(fl, component float64) float64 {
	af := math.Pow(fl*math.Abs(component)/100, 0.42)
	return signum(component) * 400 * af / (af + 27.13)
}
