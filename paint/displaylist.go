package paint

import "github.com/bankai-tensei/gowaku/geom"

// OpKind identifies a drawing command.
type OpKind uint8

// The drawing commands.
const (
	OpRRect OpKind = iota
	OpShadow
	OpGlyphs
	OpPath
	OpImage
	// OpClipPush narrows the clip to Path, and OpClipPop undoes one push.
	OpClipPush
	OpClipPop
	// OpLayer draws Layer with its origin at Origin.
	OpLayer
	// OpBackdrop blurs what is already drawn inside RRect, as a backdrop
	// filter does. Blur is the standard deviation and Color.A the opacity.
	OpBackdrop
	// OpShader fills RRect.Rect with the custom shader of
	// DisplayList.Shaders[ShaderCall]. Color.A is the opacity, and Scale the
	// scale of the canvas, which the shader sees as its units.
	OpShader
	// OpGroupPush starts the group of DisplayList.Groups[ShaderCall]: the
	// commands up to its OpGroupMask or OpGroupPop draw into a picture of its
	// own, and those between OpGroupMask and OpGroupPop into its mask. Color.A
	// is the opacity of the group as one picture, Blend how it combines, and
	// Origin, Scale and Transform place the coordinates of its filter, as for
	// OpPath. Clip and ClipLevel apply to the picture. See
	// docs/ADR/0114-gpu-layers-masks-filters.md.
	OpGroupPush
	// OpGroupMask ends what a group draws and starts its mask.
	OpGroupMask
	// OpGroupPop ends a group.
	OpGroupPop
)

// String returns the name of the kind.
func (k OpKind) String() string {
	switch k {
	case OpRRect:
		return "rrect"
	case OpShadow:
		return "shadow"
	case OpGlyphs:
		return "glyphs"
	case OpPath:
		return "path"
	case OpImage:
		return "image"
	case OpClipPush:
		return "clip-push"
	case OpClipPop:
		return "clip-pop"
	case OpLayer:
		return "layer"
	case OpBackdrop:
		return "backdrop"
	case OpShader:
		return "shader"
	case OpGroupPush:
		return "group-push"
	case OpGroupMask:
		return "group-mask"
	case OpGroupPop:
		return "group-pop"
	}
	return "unknown"
}

// Op is one recorded drawing command with the Canvas state already applied:
// coordinates are relative to the surface, and opacity is part of the colors.
type Op struct {
	Kind OpKind
	// Blend combines the command with what is below it. It sits next to Kind,
	// where it takes no room of its own.
	Blend BlendMode
	// Filter is how OpImage filters its picture. It also takes no room of
	// its own.
	Filter FilterQuality
	// RRect is the shape. For OpShadow it is the shape that casts the shadow,
	// already moved by the shadow offset. For OpGlyphs it is the bounds of the
	// run.
	RRect geom.RRect
	// Color fills the shape, unless HasGradient is set.
	Color geom.Color
	// Gradient.MoreStops, if set, points to a copy that the Canvas keeps
	// until its next Reset.
	Gradient    Gradient
	HasGradient bool
	// BorderWidth and BorderColor describe the border of OpRRect.
	BorderWidth float32
	BorderColor geom.Color
	// Blur is the blur radius of OpShadow, as in CSS: twice the standard
	// deviation of the Gaussian. For OpBackdrop it is the standard deviation
	// itself, on the surface.
	Blur float32
	// ShaderCall is the index in DisplayList.Shaders of what OpShader draws,
	// and in DisplayList.Groups of the group of OpGroupPush, OpGroupMask and
	// OpGroupPop. It fills padding, and takes no room of its own.
	ShaderCall int32
	// Glyphs is the run of OpGlyphs, at Origin. The run belongs to whoever
	// drew it, and must stay unchanged until the list is reset.
	Glyphs *GlyphRun
	// Origin is where OpGlyphs, OpPath, OpLayer and the clip operations place
	// their coordinates.
	Origin geom.Offset
	// Scale multiplies the coordinates of OpGlyphs, OpPath and OpClipPush before
	// they move to Origin. On OpRRect it only reports the scale already applied.
	// Zero counts as 1.
	Scale float32
	// Path is the shape of OpPath and OpClipPush. RRect.Rect holds its bounds on
	// the surface. Color and Gradient fill it, unless they are empty. A Canvas
	// points it at a copy of its own, valid until the next Reset.
	Path *Path
	// Stroke outlines OpPath, centered on the path, when its Width is positive.
	Stroke Stroke
	// Image is the picture of OpImage, drawn into RRect.Rect. Color.A is its
	// opacity.
	Image *Image
	// Layer is the part of the picture that OpLayer draws. RRect.Rect holds an
	// estimate of its bounds on the surface.
	Layer *Layer
	// Transform carries the coordinates of the command, once Origin and Scale
	// have placed them, to the surface. It is set only for a rotation or a skew
	// from Canvas.Transform; the zero value is the identity, and then RRect and
	// the rest are surface coordinates already. Clip is always on the surface.
	Transform geom.Affine
	// ClipLevel counts the OpClipPush in effect.
	ClipLevel uint8
	// Clip limits drawing when Clipped is set.
	Clip    geom.Rect
	Clipped bool
}

// Zoom returns Scale, with zero as 1 for commands built by hand.
func (o *Op) Zoom() float32 {
	if o.Scale == 0 {
		return 1
	}
	return o.Scale
}

// Bounds returns the upright bounds of RRect.Rect on the surface.
func (o *Op) Bounds() geom.Rect {
	if o.Transform.IsIdentity() {
		return o.RRect.Rect
	}
	return o.Transform.ApplyRect(o.RRect.Rect)
}

// DisplayList is the output of a Canvas.
type DisplayList struct {
	Ops []Op
	// Shaders holds what the OpShader commands draw, which is too large for
	// an Op.
	Shaders []ShaderCall
	// Groups holds the groups that OpGroupPush opens.
	Groups []GroupCall
}
