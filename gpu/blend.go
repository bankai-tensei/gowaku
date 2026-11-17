package gpu

import (
	"image"

	"github.com/bankai-tensei/gowaku/geom"
)

// BlendMode selects how a DrawCall combines with the target. The values are
// those of paint.BlendMode, which render passes through. Every mode is bounded
// by the shape: the pixel shader keeps coverage apart from color, so that outside
// the shape the target stays as it was. See docs/ADR/0044-drawing-extensions.md.
type BlendMode uint8

// The blend modes, in the order of paint.BlendMode.
const (
	BlendSrcOver BlendMode = iota
	BlendClear
	BlendSrc
	BlendDst
	BlendDstOver
	BlendSrcIn
	BlendDstIn
	BlendSrcOut
	BlendDstOut
	BlendSrcAtop
	BlendDstAtop
	BlendXor
	BlendPlus
	BlendScreen
	BlendModulate
	BlendMultiply
	BlendOverlay
	BlendDarken
	BlendLighten
	BlendColorDodge
	BlendColorBurn
	BlendHardLight
	BlendSoftLight
	BlendDifference
	BlendExclusion
	BlendHue
	BlendSaturation
	BlendColor
	BlendLuminosity

	// BlendModeCount is the number of blend modes.
	BlendModeCount = int(BlendLuminosity) + 1
)

// BlendFactor is a factor of the fixed-function blend: the output merger
// computes src·SrcFactor + dst·DstFactor. A factor that names a color stands for
// its alpha in the alpha channel.
type BlendFactor uint8

// The blend factors. Src is what the pixel shader outputs, Dst the target.
const (
	FactorZero BlendFactor = iota
	FactorOne
	FactorSrcColor
	FactorInvSrcColor
	FactorSrcAlpha
	FactorInvSrcAlpha
	FactorDstAlpha
	FactorInvDstAlpha
)

// blendFactors holds the factors of each mode. The modes that read the target
// copy it into the backdrop, compute the whole result in the pixel shader and
// write it with the factors here: source-over, or a plain replace for those
// that take the place of the target by coverage.
var blendFactors = [BlendModeCount][2]BlendFactor{
	BlendSrcOver:  {FactorOne, FactorInvSrcAlpha},
	BlendClear:    {FactorZero, FactorInvSrcAlpha},
	BlendSrc:      {FactorOne, FactorZero},
	BlendDst:      {FactorZero, FactorOne},
	BlendDstOver:  {FactorInvDstAlpha, FactorOne},
	BlendSrcIn:    {FactorOne, FactorZero},
	BlendDstIn:    {FactorZero, FactorInvSrcAlpha},
	BlendSrcOut:   {FactorOne, FactorZero},
	BlendDstOut:   {FactorZero, FactorInvSrcAlpha},
	BlendSrcAtop:  {FactorDstAlpha, FactorInvSrcAlpha},
	BlendDstAtop:  {FactorOne, FactorZero},
	BlendXor:      {FactorInvDstAlpha, FactorInvSrcAlpha},
	BlendPlus:     {FactorOne, FactorOne},
	BlendScreen:   {FactorOne, FactorInvSrcColor},
	BlendModulate: {FactorZero, FactorInvSrcColor},
}

// Factors returns the fixed-function blend of m. The modes from BlendMultiply
// on composite source-over.
func (m BlendMode) Factors() (src, dst BlendFactor) {
	if int(m) >= len(blendFactors) {
		return FactorOne, FactorInvSrcAlpha
	}
	f := blendFactors[m]
	if m >= BlendMultiply {
		f = blendFactors[BlendSrcOver]
	}
	return f[0], f[1]
}

// State returns the first mode with the fixed-function blend of m, so that a
// backend keeps one pipeline per blend state: the modes that read the backdrop
// share that of source-over or of BlendSrc.
func (m BlendMode) State() BlendMode {
	src, dst := m.Factors()
	for s := range m {
		if a, b := s.Factors(); a == src && b == dst {
			return s
		}
	}
	return m
}

// ReadsBackdrop reports whether a draw call with m needs a copy of the target
// under it, bound as the backdrop. Besides the modes of the W3C formulas these
// are the Porter-Duff modes that replace the target by coverage: one output
// cannot carry both the color and the coverage they need.
func (m BlendMode) ReadsBackdrop() bool {
	switch m {
	case BlendSrc, BlendSrcIn, BlendSrcOut, BlendDstAtop:
		return true
	}
	return m >= BlendMultiply && int(m) < BlendModeCount
}

// blendShift places the blend mode in InstanceKind.
const blendShift = 8

// Base returns k without the blend mode that render stores in its upper bits.
func (k InstanceKind) Base() InstanceKind { return k & (1<<blendShift - 1) }

// Blend returns the blend mode stored in k.
func (k InstanceKind) Blend() BlendMode { return BlendMode(k >> blendShift) }

// WithBlend returns k with the blend mode m, which the pixel shader needs to
// shape its output.
func (k InstanceKind) WithBlend(m BlendMode) InstanceKind {
	return k.Base() | InstanceKind(m)<<blendShift
}

// BackdropRect returns the physical pixels of a target of the given size that
// the backdrop of c must hold: Bounds, clipped, grown to whole pixels and kept
// inside the target. It is empty if c draws nothing there.
func BackdropRect(c *DrawCall, scale float32, width, height int) image.Rectangle {
	r := c.Bounds
	if c.Clipped {
		r = r.Intersect(c.Clip)
	}
	if r.Size().Empty() {
		return image.Rectangle{}
	}
	left := int(geom.Floor32(r.X * scale))
	top := int(geom.Floor32(r.Y * scale))
	right := int(geom.Floor32((r.X+r.W)*scale)) + 1
	bottom := int(geom.Floor32((r.Y+r.H)*scale)) + 1
	return image.Rect(left, top, right, bottom).Intersect(image.Rect(0, 0, width, height))
}
