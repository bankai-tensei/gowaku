package theme

import "github.com/bankai-tensei/gowaku/geom"

// HCT is a color as hue, chroma and tone. Hue and chroma are those of CAM16 and
// tone is the L* of L*a*b*, so a difference in tone maps directly to a contrast
// ratio. Hue is in degrees, tone runs from 0 to 100, and the maximum chroma
// depends on hue and tone.
//
// The zero value is black.
type HCT struct {
	Hue, Chroma, Tone float64
}

// HCTFromColor converts c. Alpha is ignored, and channels outside 0..1 are
// clamped.
func HCTFromColor(c geom.Color) HCT {
	linear := [3]float64{
		linearized(channel255(c.R)),
		linearized(channel255(c.G)),
		linearized(channel255(c.B)),
	}
	cam := cam16FromLinear(linear)
	y := yFromLinR*linear[0] + yFromLinG*linear[1] + yFromLinB*linear[2]
	return HCT{Hue: cam.hue, Chroma: cam.chroma, Tone: lstarFromY(y)}
}

// Color solves h to the nearest opaque sRGB color with 8 bits per channel. A
// chroma beyond the sRGB gamut is reduced, keeping hue and tone. Colors of
// geom.Hex survive a round trip through HCTFromColor exactly.
func (h HCT) Color() geom.Color { return geom.Hex(solveRGB(h.Hue, h.Chroma, h.Tone)) }
