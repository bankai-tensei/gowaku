package paint

// A resolved filter element becomes a Filter of the canvas, which the GPU runs
// when the group of the element ends. See docs/ADR/0113-svg-filters-text.md and
// docs/ADR/0114-gpu-layers-masks-filters.md.

// svgBlendModes maps the modes of feBlend to those of the canvas.
var svgBlendModes = map[string]BlendMode{
	"normal": BlendSrcOver, "multiply": BlendMultiply, "screen": BlendScreen, "overlay": BlendOverlay,
	"darken": BlendDarken, "lighten": BlendLighten, "color-dodge": BlendColorDodge, "color-burn": BlendColorBurn,
	"hard-light": BlendHardLight, "soft-light": BlendSoftLight, "difference": BlendDifference,
	"exclusion": BlendExclusion, "hue": BlendHue, "saturation": BlendSaturation, "color": BlendColor,
	"luminosity": BlendLuminosity,
}

// svgCompositeOperators maps the operators of feComposite to those of the
// canvas.
var svgCompositeOperators = map[string]CompositeOperator{
	"over": CompositeOver, "in": CompositeIn, "out": CompositeOut, "atop": CompositeAtop, "xor": CompositeXor,
	"arithmetic": CompositeArithmetic, "lighter": CompositeLighter,
}

// svgTransferKinds maps the types of the transfer functions to those of the
// canvas.
var svgTransferKinds = map[string]TransferKind{
	"identity": TransferIdentity, "table": TransferTable, "discrete": TransferDiscrete,
	"linear": TransferLinear, "gamma": TransferGamma,
}

// filterInput converts an input of a primitive.
func filterInput(in int) FilterInput {
	switch in {
	case feSourceGraphic:
		return FilterSourceGraphic
	case feSourceAlpha:
		return FilterSourceAlpha
	case feNothing:
		return FilterTransparent
	}
	return FilterResult(in)
}

// build makes the Filter of f, in user space: Transform is the ctm of f, which
// drawing at a size multiplies by what takes the drawing to the canvas.
func (f *svgFilter) build() {
	f.out = Filter{Region: f.region, Transform: f.ctm}
	if len(f.prims) == 0 {
		return
	}
	f.out.Primitives = make([]FilterPrimitive, len(f.prims))
	for i := range f.prims {
		pr := &f.prims[i]
		fp := &f.out.Primitives[i]
		fp.In, fp.In2 = filterInput(pr.in), filterInput(pr.in2)
		fp.SRGB = !pr.linear
		if pr.hasSub {
			fp.Subregion = pr.sub
		}
		fp.StdDev.X, fp.StdDev.Y = pr.stdDev[0], pr.stdDev[1]
		fp.Offset.X, fp.Offset.Y = pr.dx, pr.dy
		fp.Color = pr.color
		switch pr.kind {
		case feBlur:
			fp.Kind = FilterBlur
		case feOffset:
			fp.Kind = FilterOffset
		case feDropShadow:
			fp.Kind = FilterDropShadow
		case feFlood:
			fp.Kind = FilterFlood
		case feColorMatrix:
			fp.Kind = FilterColorMatrix
			fp.Matrix = pr.matrix
		case feComposite:
			fp.Kind = FilterComposite
			fp.Operator = svgCompositeOperators[pr.op]
			fp.K = pr.k
		case feMerge:
			fp.Kind = FilterMerge
			fp.Inputs = make([]FilterInput, len(pr.merge))
			for k, in := range pr.merge {
				fp.Inputs[k] = filterInput(in)
			}
		case feBlend:
			fp.Kind = FilterBlend
			fp.Mode = svgBlendModes[pr.op]
		case feComponentTransfer:
			fp.Kind = FilterComponentTransfer
			for k := range pr.funcs {
				tf := &pr.funcs[k]
				fp.Transfer[k] = TransferFunc{
					Kind: svgTransferKinds[tf.kind], Table: tf.table,
					Slope: tf.slope, Intercept: tf.intercept,
					Amplitude: tf.amp, Exponent: tf.exp, Offset: tf.off,
				}
			}
		case feMorphology:
			fp.Kind = FilterMorphology
			fp.Radius.X, fp.Radius.Y = pr.radius[0], pr.radius[1]
			fp.Dilate = pr.op == "dilate"
		}
	}
}
