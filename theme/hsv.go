package theme

import (
	"math"

	"github.com/bankai-tensei/gowaku/geom"
)

// HSV is a color as hue, saturation and value (brightness) over sRGB, the
// model of the plane of a color picker. H is in degrees from 0 up to 360; S, V
// and A run from 0 to 1. A is the alpha of geom.Color, carried along.
//
// The hue of a gray, and the saturation of black, have no value of their own.
// A picker that keeps an HSV, rather than going back to RGB after every move,
// does not lose them: see HSVFromColorLike.
type HSV struct {
	H, S, V, A float32
}

// HSVFromColor returns c as HSV. A gray gets hue 0, and black saturation 0.
func HSVFromColor(c geom.Color) HSV {
	r, g, b := geom.Clamp32(c.R, 0, 1), geom.Clamp32(c.G, 0, 1), geom.Clamp32(c.B, 0, 1)
	hi := max(r, g, b)
	lo := min(r, g, b)
	out := HSV{V: hi, A: geom.Clamp32(c.A, 0, 1)}
	d := hi - lo
	if hi <= 0 || d <= 0 {
		return out
	}
	out.S = d / hi
	var h float32
	switch hi {
	case r:
		h = (g - b) / d
	case g:
		h = 2 + (b-r)/d
	default:
		h = 4 + (r-g)/d
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	if h >= 360 {
		h -= 360
	}
	out.H = h
	return out
}

// HSVFromColorLike returns c as HSV, taking from like what c does not decide:
// the hue when c is a gray or black, and the saturation too when c is black.
// It keeps the hue of a picker steady while its saturation or value passes
// through zero.
func HSVFromColorLike(c geom.Color, like HSV) HSV {
	out := HSVFromColor(c)
	if out.V <= 0 {
		out.H, out.S = like.H, like.S
	} else if out.S <= 0 {
		out.H = like.H
	}
	return out
}

// Color returns the sRGB color of h. Values outside their ranges are clamped,
// and the hue wraps around.
func (h HSV) Color() geom.Color {
	s, v := geom.Clamp32(h.S, 0, 1), geom.Clamp32(h.V, 0, 1)
	a := geom.Clamp32(h.A, 0, 1)
	hue := float32(math.Mod(float64(h.H), 360))
	if hue < 0 {
		hue += 360
	}
	// f returns the channel whose peak sits at n sixths of the circle, as in
	// the formula of CSS Color 4.
	f := func(n float32) float32 {
		k := float32(math.Mod(float64(n+hue/60), 6))
		return v - v*s*max(0, min(k, 4-k, 1))
	}
	return geom.Color{R: f(5), G: f(3), B: f(1), A: a}
}
