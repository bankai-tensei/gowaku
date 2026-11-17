package geom

import "image/color"

// Color is a non-premultiplied sRGB color. Channels range from 0 to 1.
// The zero value is fully transparent.
type Color struct {
	R, G, B, A float32
}

// Hex returns the opaque color for a 0xRRGGBB value.
func Hex(rgb uint32) Color {
	return Color{
		R: float32(rgb>>16&0xff) / 255,
		G: float32(rgb>>8&0xff) / 255,
		B: float32(rgb&0xff) / 255,
		A: 1,
	}
}

// RGBA returns the color with the given channels.
func RGBA(r, g, b, a float32) Color { return Color{r, g, b, a} }

// FromStd converts a standard library color.
func FromStd(c color.Color) Color {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	return Color{float32(n.R) / 255, float32(n.G) / 255, float32(n.B) / 255, float32(n.A) / 255}
}

// Std converts c to a standard library color, rounding to 8 bits per channel.
func (c Color) Std() color.NRGBA {
	return color.NRGBA{R: channel8(c.R), G: channel8(c.G), B: channel8(c.B), A: channel8(c.A)}
}

// WithAlpha returns c with its alpha replaced by a.
func (c Color) WithAlpha(a float32) Color {
	c.A = a
	return c
}

func channel8(v float32) uint8 { return uint8(Clamp32(v, 0, 1)*255 + 0.5) }
