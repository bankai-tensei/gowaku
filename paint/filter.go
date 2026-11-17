package paint

import (
	"math"

	"github.com/bankai-tensei/gowaku/geom"
)

// Filter is a graph of filter primitives that a group runs over what it draws,
// as the filter element of SVG does: each primitive reads the picture of the
// group, its alpha, or the results of primitives before it, and the last result
// is what the group draws. The GPU runs it, pass by pass, on textures of the
// group. See GroupStyle and docs/ADR/0114-gpu-layers-masks-filters.md.
//
// Lengths, the region and the subregions are in the coordinates of Transform,
// which takes them to the canvas at PushGroup; a rotation or a skew is
// approximated: a rectangle becomes its upright bounds, and a standard
// deviation or a radius scales by how much the axis it runs along stretches.
//
// A Filter is held by pointer until the canvas is reset, and must not change
// in the meantime.
type Filter struct {
	// Region is where the filter draws: the primitives see nothing outside it,
	// and the result is cut to it. An empty region draws nothing.
	Region geom.Rect
	// Transform takes the coordinates of the filter to those of the canvas.
	// The zero value is the identity.
	Transform geom.Affine
	// Primitives run in order. A filter without primitives draws nothing.
	Primitives []FilterPrimitive
}

// FilterInput names what a primitive reads.
type FilterInput int32

// The inputs that are not results of a primitive. FilterResult names those that
// are. The zero value is the result of the primitive before, or the picture of
// the group for the first one, as an input left out in SVG.
const (
	FilterPrevious FilterInput = 0
	// FilterSourceGraphic is the picture of the group.
	FilterSourceGraphic FilterInput = -1
	// FilterSourceAlpha is its alpha, with black for the color.
	FilterSourceAlpha FilterInput = -2
	// FilterTransparent reads nothing, as BackgroundImage and the other inputs
	// that gowaku cannot draw do in SVG.
	FilterTransparent FilterInput = -3
)

// FilterResult returns the input that reads the result of primitive i, which
// must come before the primitive that reads it; a later one reads as
// FilterPrevious.
func FilterResult(i int) FilterInput { return FilterInput(i + 1) }

// FilterKind selects what a FilterPrimitive does.
type FilterKind uint8

// The primitives, those of SVG that gowaku draws.
const (
	// FilterBlur blurs In by StdDev, as feGaussianBlur: a Gaussian below a
	// standard deviation of 2 pixels, three box blurs from there, as the Filter
	// Effects specification suggests and browsers do. A standard deviation of
	// zero on both axes passes In through.
	FilterBlur FilterKind = iota
	// FilterOffset moves In by Offset, blending neighbors for a fraction of a
	// pixel.
	FilterOffset
	// FilterDropShadow draws In over its alpha blurred by StdDev, moved by
	// Offset and colored with Color.
	FilterDropShadow
	// FilterFlood fills the subregion with Color.
	FilterFlood
	// FilterColorMatrix multiplies the color of In, not premultiplied, by
	// Matrix: R' = M[0]·R + M[1]·G + M[2]·B + M[3]·A + M[4], and so on for G',
	// B' and A'.
	FilterColorMatrix
	// FilterComposite combines In with In2 by Operator.
	FilterComposite
	// FilterMerge draws Inputs over each other, the first at the bottom.
	FilterMerge
	// FilterBlend blends In over In2 by Mode, as feBlend; BlendSrcOver is
	// normal, and modes other than BlendScreen and the separable and
	// non-separable ones draw as normal.
	FilterBlend
	// FilterComponentTransfer maps each channel of In, not premultiplied,
	// through Transfer.
	FilterComponentTransfer
	// FilterMorphology erodes In by Radius, or dilates it if Dilate is set.
	// Radii round to whole pixels, at most 64.
	FilterMorphology
)

// CompositeOperator is an operator of FilterComposite, as in feComposite.
type CompositeOperator uint8

// The operators. Arithmetic computes K[0]·In·In2 + K[1]·In + K[2]·In2 + K[3]
// on premultiplied values.
const (
	CompositeOver CompositeOperator = iota
	CompositeIn
	CompositeOut
	CompositeAtop
	CompositeXor
	CompositeArithmetic
	CompositeLighter
)

// TransferKind is the type of a TransferFunc.
type TransferKind uint8

// The transfer functions of feComponentTransfer.
const (
	TransferIdentity TransferKind = iota
	// TransferTable interpolates linearly between the values of Table, spread
	// evenly from 0 to 1.
	TransferTable
	// TransferDiscrete steps through the values of Table.
	TransferDiscrete
	// TransferLinear is Slope·C + Intercept.
	TransferLinear
	// TransferGamma is Amplitude·C^Exponent + Offset.
	TransferGamma
)

// TransferFunc maps one channel of FilterComponentTransfer.
type TransferFunc struct {
	Kind                        TransferKind
	Table                       []float32
	Slope, Intercept            float32
	Amplitude, Exponent, Offset float32
}

// Apply maps c, from 0 to 1, through f. The result is not clamped.
func (f *TransferFunc) Apply(c float32) float32 {
	switch f.Kind {
	case TransferTable:
		n := len(f.Table) - 1
		if n < 0 {
			return c
		}
		if n == 0 {
			return f.Table[0]
		}
		x := c * float32(n)
		k := min(max(int(x), 0), n-1)
		return f.Table[k] + (x-float32(k))*(f.Table[k+1]-f.Table[k])
	case TransferDiscrete:
		n := len(f.Table)
		if n == 0 {
			return c
		}
		return f.Table[min(max(int(c*float32(n)), 0), n-1)]
	case TransferLinear:
		return f.Slope*c + f.Intercept
	case TransferGamma:
		return f.Amplitude*float32(math.Pow(float64(c), float64(f.Exponent))) + f.Offset
	}
	return c
}

// FilterPrimitive is one step of a Filter. Each kind reads the fields its
// documentation names.
type FilterPrimitive struct {
	Kind FilterKind
	// In and In2 are the inputs.
	In, In2 FilterInput
	// Inputs are those of FilterMerge.
	Inputs []FilterInput
	// SRGB computes in sRGB instead of linearRGB, as
	// color-interpolation-filters: sRGB does. The zero value works in
	// linearRGB, the default of SVG.
	SRGB bool
	// Subregion, unless it is the zero value, limits the result, inside the
	// region of the filter.
	Subregion geom.Rect
	// StdDev is the standard deviation of a blur along x and y.
	StdDev geom.Offset
	// Offset moves FilterOffset and the shadow of FilterDropShadow.
	Offset geom.Offset
	// Color fills FilterFlood and colors the shadow of FilterDropShadow. It is
	// not premultiplied.
	Color geom.Color
	// Matrix is that of FilterColorMatrix, row by row.
	Matrix [20]float32
	// Operator and K are those of FilterComposite.
	Operator CompositeOperator
	K        [4]float32
	// Mode is that of FilterBlend.
	Mode BlendMode
	// Radius and Dilate are those of FilterMorphology.
	Radius geom.Offset
	Dilate bool
	// Transfer holds the functions of R, G, B and A of
	// FilterComponentTransfer.
	Transfer [4]TransferFunc
}
