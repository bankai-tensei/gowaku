package paint

// BlendMode selects how a shape combines with what is below it, the backdrop.
// Colors are premultiplied, and blending happens on the stored sRGB values, as
// all compositing in gowaku does.
//
// Every mode is bounded by the shape: outside it, the backdrop stays as it was,
// and along its antialiased edge the result fades from the backdrop to the
// mode's result by coverage. So a Clear or a SrcIn only affects what the shape
// covers, not the whole layer. The shape of an image,
// and of a layer drawn with Blend, is its rectangle.
//
// The modes up to BlendModulate are the Porter-Duff operators, and the GPU
// blends most of them on its own. The modes from BlendMultiply on follow the
// W3C Compositing and Blending Level 1 formulas, then composite source-over.
// Those, and BlendSrc, BlendSrcIn, BlendSrcOut and BlendDstAtop, which replace
// the backdrop and so need coverage apart from color, read the backdrop: each
// draw that uses one copies the pixels under it first, so they cost more. See
// docs/ADR/0044-drawing-extensions.md.
type BlendMode uint8

// The blend modes. S is the source, D the backdrop, both premultiplied, and
// Sa and Da their alphas.
const (
	// BlendSrcOver draws S over D: S + D(1-Sa). It is the zero value.
	BlendSrcOver BlendMode = iota
	// BlendClear erases D: 0.
	BlendClear
	// BlendSrc replaces D with S.
	BlendSrc
	// BlendDst keeps D, which draws nothing.
	BlendDst
	// BlendDstOver draws S behind D: S(1-Da) + D.
	BlendDstOver
	// BlendSrcIn keeps S where D is: S·Da.
	BlendSrcIn
	// BlendDstIn keeps D where S is: D·Sa.
	BlendDstIn
	// BlendSrcOut keeps S where D is not: S(1-Da).
	BlendSrcOut
	// BlendDstOut keeps D where S is not: D(1-Sa).
	BlendDstOut
	// BlendSrcAtop draws S over D, only where D is: S·Da + D(1-Sa).
	BlendSrcAtop
	// BlendDstAtop draws D over S, only where S is: S(1-Da) + D·Sa.
	BlendDstAtop
	// BlendXor keeps S and D where they do not overlap: S(1-Da) + D(1-Sa).
	BlendXor
	// BlendPlus adds S and D, clamped to 1.
	BlendPlus
	// BlendScreen lightens: S + D - S·D.
	BlendScreen
	// BlendModulate multiplies S and D, alpha included: S·D.
	BlendModulate

	// BlendMultiply darkens by multiplying the colors.
	BlendMultiply
	// BlendOverlay multiplies or screens, depending on the backdrop.
	BlendOverlay
	// BlendDarken keeps the darker of the two colors.
	BlendDarken
	// BlendLighten keeps the lighter of the two colors.
	BlendLighten
	// BlendColorDodge brightens the backdrop toward the source.
	BlendColorDodge
	// BlendColorBurn darkens the backdrop toward the source.
	BlendColorBurn
	// BlendHardLight multiplies or screens, depending on the source.
	BlendHardLight
	// BlendSoftLight darkens or lightens, depending on the source.
	BlendSoftLight
	// BlendDifference subtracts the darker color from the lighter one.
	BlendDifference
	// BlendExclusion is Difference with less contrast.
	BlendExclusion

	// BlendHue takes the hue of the source, and the saturation and luminosity
	// of the backdrop.
	BlendHue
	// BlendSaturation takes the saturation of the source, and the hue and
	// luminosity of the backdrop.
	BlendSaturation
	// BlendColor takes the hue and saturation of the source, and the
	// luminosity of the backdrop.
	BlendColor
	// BlendLuminosity takes the luminosity of the source, and the hue and
	// saturation of the backdrop.
	BlendLuminosity

	// BlendModeCount is the number of blend modes.
	BlendModeCount = int(BlendLuminosity) + 1
)

var blendNames = [BlendModeCount]string{
	"src-over", "clear", "src", "dst", "dst-over", "src-in", "dst-in", "src-out", "dst-out",
	"src-atop", "dst-atop", "xor", "plus", "screen", "modulate",
	"multiply", "overlay", "darken", "lighten", "color-dodge", "color-burn", "hard-light", "soft-light",
	"difference", "exclusion", "hue", "saturation", "color", "luminosity",
}

// String returns the name of the mode, in the style of CSS.
func (m BlendMode) String() string {
	if int(m) < BlendModeCount {
		return blendNames[m]
	}
	return "unknown"
}
