package gpu

import (
	"image"

	"github.com/bankai-tensei/gowaku/geom"
)

// EffectOp is what an InstanceEffect computes. Every effect reads its inputs
// from the Image (In0) and the Texture (In1) of its DrawCall, and writes one
// premultiplied sRGB color per pixel of Rect: render draws effects with a scale
// of 1, so that Rect, the inputs and the target share one grid of texels from
// the origin. Texels outside the region, P3.X by P3.Y texels from the origin,
// read as transparent, whatever the textures hold there. See
// docs/ADR/0114-gpu-layers-masks-filters.md.
type EffectOp uint8

// The effects. P0 to P7 are the parameters of SetEffect.
const (
	// EffectCopy reads In0 moved by P0.XY texels, blending the four texels
	// around a point between them.
	EffectCopy EffectOp = iota
	// EffectFlood fills with P0, a premultiplied color.
	EffectFlood
	// EffectMatrix multiplies the color of In0, not premultiplied, by the 4
	// by 5 matrix whose rows are P4 to P7 and whose last column is P0, as
	// feColorMatrix does.
	EffectMatrix
	// EffectTransfer maps each channel of In0, not premultiplied, through row
	// P0.X of In1, a texture of RampWidth texels per row, as
	// feComponentTransfer does: texel i of a channel holds what i / 255 maps to.
	EffectTransfer
	// EffectComposite combines In0 with In1 by the operator P0.X, a
	// CompositeOperator; the arithmetic one takes k1 to k4 from P1.
	EffectComposite
	// EffectBlend blends In0, the source, over In1, the backdrop, by the
	// BlendMode P0.X, as feBlend does. Modes other than BlendScreen and the
	// separable and non-separable ones draw source-over.
	EffectBlend
	// EffectMorphology takes the least, or with EffectDilate the greatest,
	// value of each channel of In0 along the axis P0.XY, P0.Z texels to each
	// side, inside the region.
	EffectMorphology
	// EffectGaussian blurs In0 along the axis P0.XY with the standard
	// deviation P0.Z, reading P0.W texels to each side.
	EffectGaussian
	// EffectBoxes blurs In0 along the axis P0.XY with three box blurs one after
	// the other, of P1.X, P1.Y and P1.Z texels, the first starting P1.W texels
	// before the pixel in all: the box blurs of the Filter Effects
	// specification, as one kernel.
	EffectBoxes
	// EffectMask multiplies In0 by the alpha of In1, or with EffectLuminance
	// by the luminance of its premultiplied color.
	EffectMask
	// EffectShadow draws In1 over the alpha of In0 moved by P0.XY and colored
	// with P1, a premultiplied color: the last step of feDropShadow.
	EffectShadow
)

// The flags of an effect, which SetEffect adds to its op.
const (
	// EffectLinear works in linearRGB: the inputs convert from sRGB, and the
	// result back.
	EffectLinear uint32 = 1 << (8 + iota)
	// EffectAlpha0 and EffectAlpha1 read only the alpha of In0 and In1, as
	// SourceAlpha.
	EffectAlpha0
	EffectAlpha1
	// EffectEmpty0 and EffectEmpty1 read In0 and In1 as transparent.
	EffectEmpty0
	EffectEmpty1
	// EffectDilate makes EffectMorphology take the greatest value.
	EffectDilate
	// EffectLuminance makes EffectMask read the luminance.
	EffectLuminance
)

// effectOpMask masks the op out of the word that SetEffect stores.
const effectOpMask = 1<<8 - 1

// CompositeOperator is an operator of EffectComposite, as in feComposite.
type CompositeOperator uint8

// The operators.
const (
	CompositeOver CompositeOperator = iota
	CompositeIn
	CompositeOut
	CompositeAtop
	CompositeXor
	CompositeArithmetic
	CompositeLighter
)

// EffectParams are the eight float4 that an InstanceEffect carries.
type EffectParams [8][4]float32

// The fields of an InstanceEffect. It is a custom shader of render, so it keeps
// its data where an InstanceShader does (see SetShaderParams):
//
//	Rect           the pixels it writes, from the origin of the target
//	Color.A        1
//	BorderWidth    the op and its flags, as a float
//	Radii          P0
//	StopOffsets    P1
//	GradientMatrix P2
//	GradientStart, GradientEnd  P3
//	EndColor, BorderColor       P4, P5
//	MidColors      P6, P7

// SetEffect makes in an InstanceEffect of op with the given flags and
// parameters, writing to rect.
func (in *Instance) SetEffect(rect geom.Rect, op EffectOp, flags uint32, p *EffectParams) {
	*in = Instance{
		Rect:           rect,
		Color:          geom.Color{A: 1},
		BorderWidth:    float32(uint32(op) | flags),
		Kind:           InstanceEffect,
		Radii:          p[0],
		StopOffsets:    p[1],
		GradientMatrix: p[2],
		GradientStart:  geom.Offset{X: p[3][0], Y: p[3][1]},
		GradientEnd:    geom.Offset{X: p[3][2], Y: p[3][3]},
		EndColor:       colorOf(p[4]),
		BorderColor:    colorOf(p[5]),
		MidColors:      [2]geom.Color{colorOf(p[6]), colorOf(p[7])},
	}
}

// Effect returns what SetEffect stored.
func (in *Instance) Effect() (op EffectOp, flags uint32, p EffectParams) {
	word := uint32(in.BorderWidth)
	p = EffectParams{
		in.Radii,
		in.StopOffsets,
		in.GradientMatrix,
		{in.GradientStart.X, in.GradientStart.Y, in.GradientEnd.X, in.GradientEnd.Y},
		floatsOf(in.EndColor),
		floatsOf(in.BorderColor),
		floatsOf(in.MidColors[0]),
		floatsOf(in.MidColors[1]),
	}
	return EffectOp(word & effectOpMask), word &^ effectOpMask, p
}

func colorOf(v [4]float32) geom.Color { return geom.Color{R: v[0], G: v[1], B: v[2], A: v[3]} }

func floatsOf(c geom.Color) [4]float32 { return [4]float32{c.R, c.G, c.B, c.A} }

// ClearBounds returns the bounds of b.Clears: what a batch that needs
// multisampling replaces of its target.
func (b *Batch) ClearBounds() image.Rectangle {
	var r image.Rectangle
	for _, c := range b.Clears {
		r = r.Union(c)
	}
	return r
}

// The box blurs of a Gaussian of standard deviation sigma, as the Filter
// Effects specification gives them for sigma of 2 and up.

// BoxSizes returns the three box blurs that stand in for a Gaussian of standard
// deviation sigma, in texels: their widths, and how far before the pixel the
// first texel of the combined kernel lies. A width of d = floor(sigma·3·√(2π)/4
// + 0.5) that is odd gives three boxes of d centered on the pixel; an even one
// gives two of d, one leaning each way, and one of d + 1 centered. A sigma too
// small for any box gives widths of 1.
func BoxSizes(sigma float32) (widths [3]int, before int) {
	d := int(geom.Floor32(sigma*3*2.5066283/4 + 0.5))
	if d < 1 {
		return [3]int{1, 1, 1}, 0
	}
	half := d / 2
	if d%2 == 1 {
		return [3]int{d, d, d}, 3 * half
	}
	// The boxes reach half before and half-1 after, half-1 before and half
	// after, and half on both sides.
	return [3]int{d, d, d + 1}, half + (half - 1) + half
}

// BoxWeight returns the weight of the combined kernel of three box blurs of the
// given widths at m texels from its first: how many ways three steps, one in
// each box, add up to m, over the product of the widths. Every backend follows
// these operations.
func BoxWeight(m float32, d [3]float32) float32 {
	n := boxTriangle(m) - boxTriangle(m-d[0]) - boxTriangle(m-d[1]) - boxTriangle(m-d[2]) +
		boxTriangle(m-d[0]-d[1]) + boxTriangle(m-d[0]-d[2]) + boxTriangle(m-d[1]-d[2]) -
		boxTriangle(m-d[0]-d[1]-d[2])
	return n / (d[0] * d[1] * d[2])
}

// boxTriangle counts the ways three steps from 0 up add up to n.
func boxTriangle(n float32) float32 {
	if n < 0 {
		return 0
	}
	return (n + 1) * (n + 2) * 0.5
}
