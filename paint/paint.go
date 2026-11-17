package paint

import "github.com/bankai-tensei/gowaku/geom"

// GradientKind selects the shape of a Gradient.
type GradientKind uint8

// The gradient kinds.
const (
	// GradientLinear runs along the line from Start to End.
	GradientLinear GradientKind = iota
	// GradientRadial is centered on Start and reaches its last color at the
	// distance of End from Start.
	GradientRadial
	// GradientSweep turns around Start: offset 0 lies in the direction of End,
	// and the offsets grow clockwise to 1 after a full turn. Like the other
	// kinds, offsets outside 0 to 1 continue the end colors, and two stops at
	// one offset meet in a hard edge.
	GradientSweep
	// GradientFocal is a radial gradient as SVG draws one with a focal point:
	// Start is the center of the circle that holds offset 1, End a point on
	// it, and offset 0 lies at Focus, which must be inside the circle. A
	// focus on or past the circle is drawn at 0.99 of the radius.
	GradientFocal
)

// SpreadMode says how a Gradient continues past offsets 0 and 1.
type SpreadMode uint8

// The spread modes, as spreadMethod in SVG.
const (
	// SpreadPad continues the end colors.
	SpreadPad SpreadMode = iota
	// SpreadReflect mirrors the gradient back and forth.
	SpreadReflect
	// SpreadRepeat starts it over.
	SpreadRepeat
)

// MaxGradientStops is the most stops Gradient.Stops holds. Four colors are
// what the GPU instance carries without a texture; Gradient.MoreStops takes
// more. See docs/ADR/0032-gradient-stops.md and
// docs/ADR/0044-drawing-extensions.md.
const MaxGradientStops = 4

// GradientStop is a color at a position along a Gradient.
type GradientStop struct {
	// Offset is the position of the stop, from 0 at Start to 1 at End.
	Offset float32
	Color  geom.Color
}

// Gradient blends between colors. Start and End use the same coordinates as
// the shape they fill. Beyond the first and last stop their colors continue.
//
// The zero StopCount blends from StartColor at Start to EndColor at End. A
// positive StopCount uses the first StopCount entries of Stops instead, and
// ignores StartColor and EndColor:
//
//	paint.Gradient{
//		End:       geom.Offset{X: 200},
//		Stops: [paint.MaxGradientStops]paint.GradientStop{
//			{Offset: 0, Color: purple},
//			{Offset: 0.5, Color: pink},
//			{Offset: 1, Color: orange},
//		},
//		StopCount: 3,
//	}
//
// Offsets should not decrease. As in CSS, an offset smaller than the one
// before it is moved up to it, which makes a hard edge. Drawing a Gradient
// with a StopCount above MaxGradientStops panics.
//
// A non-empty MoreStops holds every stop instead, as many as needed, and
// Stops, StopCount, StartColor and EndColor are ignored:
//
//	paint.Gradient{
//		End: geom.Offset{X: 200},
//		MoreStops: &[]paint.GradientStop{
//			{Offset: 0, Color: red}, {Offset: 0.2, Color: orange}, {Offset: 0.4, Color: yellow},
//			{Offset: 0.6, Color: green}, {Offset: 0.8, Color: blue}, {Offset: 1, Color: violet},
//		},
//	}
//
// Gradients of more than four stops blend between colors sampled at 256 points
// along the way, so a hard edge among them is spread over 1/255 of the length.
type Gradient struct {
	Kind GradientKind
	// Spread continues the gradient past its ends. The zero value continues
	// the end colors.
	Spread               SpreadMode
	Start, End           geom.Offset
	StartColor, EndColor geom.Color
	Stops                [MaxGradientStops]GradientStop
	StopCount            int
	// MoreStops, when it holds any stops, replaces Stops. It is a pointer so
	// that a Gradient stays comparable; two gradients with different pointers
	// compare unequal even if the stops are the same. The Canvas copies the
	// stops when it draws, so the slice can change afterwards.
	MoreStops *[]GradientStop
	// Transform carries the coordinates of Start and End, the space of the
	// gradient, to those of the shape, as gradientTransform does in SVG. A
	// radial gradient scaled on one axis becomes an ellipse. The zero value
	// is none.
	Transform geom.Affine
	// Focus is where offset 0 of a GradientFocal lies, in the coordinates of
	// Start and End.
	Focus geom.Offset
}

// more returns MoreStops, or nil if it holds no stops.
func (g *Gradient) more() []GradientStop {
	if g.MoreStops == nil {
		return nil
	}
	return *g.MoreStops
}

// visible reports whether any color of g can be seen.
func (g *Gradient) visible() bool {
	if more := g.more(); len(more) > 0 {
		for i := range more {
			if more[i].Color.A > 0 {
				return true
			}
		}
		return false
	}
	if g.StopCount == 0 {
		return g.StartColor.A > 0 || g.EndColor.A > 0
	}
	for i := range g.StopCount {
		if g.Stops[i].Color.A > 0 {
			return true
		}
	}
	return false
}

// Stroke is a border drawn inside the edge of a rounded rectangle, or centered
// on a path, over the fill.
type Stroke struct {
	Width float32
	Color geom.Color
}

// Paint describes how to fill and outline a shape. The zero value draws
// nothing, so a border without a fill is a Paint that sets only Stroke.
type Paint struct {
	Color geom.Color
	// Gradient replaces Color when set.
	Gradient *Gradient
	Stroke   *Stroke
	// Blend combines the shape with what is below it. The zero value draws
	// source-over. See BlendMode.
	Blend BlendMode
}

// FaceID names a font face. The text package hands out the IDs, so paint needs
// to know nothing about fonts.
type FaceID uint32

// Glyph is one glyph of a GlyphRun.
type Glyph struct {
	// ID is the index of the glyph in its font.
	ID uint32
	// X and Y are the position of the glyph origin, relative to the run.
	X, Y float32
}

// GlyphRun is a sequence of glyphs of one font at one size.
type GlyphRun struct {
	Face FaceID
	// Size is the font size in logical pixels.
	Size float32
	// Color overrides the color given to DrawGlyphs, unless it is zero.
	Color geom.Color
	// Origin is the position of the run, usually on the baseline.
	Origin geom.Offset
	Glyphs []Glyph
	// Bounds encloses the ink of the glyphs, relative to Origin.
	Bounds geom.Rect
	// Sideways turns every glyph 90° clockwise about its origin, as Western
	// text in vertical lines is set.
	Sideways bool
}
