// The yellow exception in Tone is ported from material-color-utilities,
// typescript/palettes/tonal_palette.ts.
// https://github.com/material-foundation/material-color-utilities
// Copyright 2021 Google LLC. Licensed under the Apache License, Version 2.0;
// see LICENSE.material-color-utilities in this directory.

package theme

import "github.com/bankai-tensei/gowaku/geom"

// TonalPalette is the family of colors that share a hue and chroma, indexed by
// tone. The zero value is the grays.
type TonalPalette struct {
	Hue, Chroma float64
}

// Tone returns the color of p at tone t, from 0 (black) to 100 (white).
func (p TonalPalette) Tone(t float64) geom.Color { return geom.Hex(p.rgb(t)) }

func (p TonalPalette) rgb(t float64) uint32 {
	if t == 99 && isYellow(p.Hue) {
		return averageRGB(p.rgb(98), p.rgb(100))
	}
	return solveRGB(p.Hue, p.Chroma, t)
}

func isYellow(hue float64) bool {
	hue = sanitizeDegrees(hue)
	return hue >= 105 && hue < 125
}

// averageRGB returns the per-channel mean of two 0xRRGGBB values, rounded up.
func averageRGB(a, b uint32) uint32 {
	var out uint32
	for shift := 0; shift <= 16; shift += 8 {
		sum := a>>shift&0xff + b>>shift&0xff
		out |= (sum + 1) / 2 << shift
	}
	return out
}
