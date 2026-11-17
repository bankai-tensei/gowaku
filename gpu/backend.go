package gpu

import (
	"errors"
	"image"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/platform"
)

// ErrNoGPU reports that no adapter can run gowaku.
var ErrNoGPU = errors.New("gowaku: no compatible GPU found")

// PowerPreference selects among the GPUs of a hybrid machine.
type PowerPreference uint8

// The power preferences. The zero value favors battery life, because a UI
// spends most of its time idle. See docs/ADR/0003-adapter-selection.md.
const (
	LowPower PowerPreference = iota
	HighPerformance
)

// String returns the name of the preference.
func (p PowerPreference) String() string {
	if p == HighPerformance {
		return "high-performance"
	}
	return "low-power"
}

// InstanceKind selects what the pixel shader draws for an Instance.
type InstanceKind uint32

// The instance kinds.
const (
	InstanceRRect InstanceKind = iota
	InstanceShadow
	InstanceGlyph
	// InstanceCover fills Rect without an edge of its own. Stencil and depth
	// tests give it its shape.
	InstanceCover
	// InstanceImage draws the image of the DrawCall into Rect. GradientStart and
	// GradientEnd hold the texture coordinates of the corners, and Gradient
	// the ImageFilter.
	InstanceImage
	// InstanceBackdrop draws the blurred backdrop of its DrawCall into the
	// rounded box of Rect and Radii. Blur holds the standard deviation in
	// logical pixels, the same as DrawCall.Blur, and Color.A the opacity. See
	// PlanBlur.
	InstanceBackdrop
	// InstanceFiltered draws the blurred Image of its DrawCall as
	// InstanceImage draws an image. Blur holds the standard deviation in
	// logical pixels, the same as DrawCall.Blur.
	InstanceFiltered
	// InstanceBlurDown, InstanceBlurX and InstanceBlurY are the passes of a
	// blur, which backends make with Blur.Passes. render never emits them.
	InstanceBlurDown
	InstanceBlurX
	InstanceBlurY
	// InstanceShader fills Rect with the Shader of its DrawCall. See
	// SetShaderParams for where its data lies.
	InstanceShader
	// InstanceEffect is one pass of a filter or a mask over the textures of a
	// group: its DrawCall has the effect shader of render as its Shader, and
	// gpu/soft computes it itself. See SetEffect and
	// docs/ADR/0114-gpu-layers-masks-filters.md.
	InstanceEffect
)

// GradientMode selects how Instance colors vary across a shape.
type GradientMode uint32

// The gradient modes.
const (
	GradientNone GradientMode = iota
	GradientLinear
	GradientRadial
	// GradientSweep turns clockwise around GradientStart, from 0 in the
	// direction of GradientEnd to 1 a full turn later.
	GradientSweep
	// GradientFocal is a radial gradient whose offset 0 lies at a focal point
	// inside the circle around GradientStart through GradientEnd, as in SVG.
	// It always comes with GradientRamp, and StopOffsets[1] and [2] hold the
	// focal point relative to GradientStart, in the coordinates of the
	// gradient. See docs/ADR/0112-svg-structure-paint-clip.md.
	GradientFocal
)

// GradientRamp is added to a gradient mode whose stops come from a row of
// Batch.Ramps instead of the instance. StopOffsets[0] then holds the row, and
// the colors and the other offsets are unused. See
// docs/ADR/0044-drawing-extensions.md.
const GradientRamp GradientMode = 1 << 4

// GradientReflect and GradientRepeat are added to a gradient mode to continue
// it past offsets 0 and 1 by mirroring or repeating it, as spreadMethod does
// in SVG. Without either the end colors continue.
const (
	GradientReflect GradientMode = 1 << 5
	GradientRepeat  GradientMode = 2 << 5
)

// gradientShape masks the shape out of a mode.
const gradientShape GradientMode = 1<<4 - 1

// Shape returns m without GradientRamp and the spread.
func (m GradientMode) Shape() GradientMode { return m & gradientShape }

// Spread returns the spread of m: zero, GradientReflect or GradientRepeat.
func (m GradientMode) Spread() GradientMode { return m & (GradientReflect | GradientRepeat) }

// SpreadOffset maps the offset t of a gradient with mode m into 0 to 1 as its
// spread says. Every backend follows these operations.
func SpreadOffset(t float32, m GradientMode) float32 {
	switch m.Spread() {
	case GradientRepeat:
		return t - geom.Floor32(t)
	case GradientReflect:
		u := t * 0.5
		u -= geom.Floor32(u)
		return 1 - geom.Abs32(u*2-1)
	}
	return t
}

// FocalOffset returns the offset at p of a focal gradient whose circle has
// its center at 0 and axis on its edge, with the focal point f inside. Every
// backend follows these operations.
func FocalOffset(p, axis, f geom.Offset) float32 {
	qx, qy := p.X-f.X, p.Y-f.Y
	dx, dy := -f.X, -f.Y
	// The conversions keep arm64 from fusing these into FMA, so every
	// architecture rounds alike; see ADR 0086.
	a := float32(dx*dx) + float32(dy*dy) - (float32(axis.X*axis.X) + float32(axis.Y*axis.Y))
	b := float32(qx*dx) + float32(qy*dy)
	c := float32(qx*qx) + float32(qy*qy)
	return (b - geom.Sqrt32(max(float32(b*b)-float32(a*c), 0))) / min(a, -1e-6)
}

// Instance is one quad of the SDF pipeline. It is the GPU's vertex format, so
// its layout must match the shaders of every backend. Coordinates are logical
// pixels from the top left of the surface.
type Instance struct {
	Rect geom.Rect
	// Radii are the corners, clockwise from the top left. InstanceGlyph has no
	// corners and keeps the x and y of its first texel in the first two.
	Radii [4]float32
	// Color fills the shape, and is the color of the first stop of a gradient.
	Color geom.Color
	// EndColor is the color of the last stop of a gradient.
	EndColor    geom.Color
	BorderColor geom.Color
	// GradientStart and GradientEnd are the gradient's control points.
	GradientStart, GradientEnd geom.Offset
	BorderWidth                float32
	// Blur is the blur radius of a shadow.
	Blur float32
	// Kind selects the shape. Its bits from 8 up hold the BlendMode of the draw
	// call, which the pixel shader needs to shape its output; render sets them.
	// See Base, Blend and WithBlend.
	Kind InstanceKind
	// Gradient selects how colors vary across the shape. An InstanceImage,
	// which has no gradient, keeps its ImageFilter here instead.
	Gradient GradientMode
	// MidColors are the colors of the second and third stop of a gradient.
	MidColors [2]geom.Color
	// StopOffsets are the positions of the stops of a gradient, in the order
	// Color, MidColors, EndColor, from 0 at GradientStart to 1 at GradientEnd.
	// They never decrease. A gradient of fewer than four stops fills the
	// missing middle ones with copies of its first stop, so that two stops are
	// Color and EndColor at offsets 0, 0, 0 and 1. See SetStops.
	StopOffsets [4]float32
	// Transform carries Rect, and every other coordinate of the instance, to
	// the surface, in logical pixels: x' = T[0]x + T[2]y + T[4] and
	// y' = T[1]x + T[3]y + T[5], the order of geom.Affine. The vertex shader
	// turns the quad, and the pixel shader evaluates the shape in the
	// coordinates of Rect. All zeros means none, which takes the exact path of
	// upright quads. See docs/ADR/0044-drawing-extensions.md.
	Transform [6]float32
	// GradientMatrix is the linear part of the map from the coordinates of
	// Rect to those of the gradient: a point p is at
	// M·(p - GradientStart) from the start of the gradient, with
	// M = [[G[0], G[2]], [G[1], G[3]]]. GradientEnd - GradientStart is then the
	// axis in the coordinates of the gradient. All zeros means none: the
	// gradient lives in the coordinates of Rect.
	GradientMatrix [4]float32
}

// SetTransform sets Transform to m. The identity, and the zero value, clear it.
func (in *Instance) SetTransform(m geom.Affine) {
	if m.IsIdentity() {
		in.Transform = [6]float32{}
		return
	}
	in.Transform = [6]float32{m.A, m.B, m.C, m.D, m.E, m.F}
}

// Affine returns Transform as a geom.Affine, and reports whether there is one.
func (in *Instance) Affine() (geom.Affine, bool) {
	t := &in.Transform
	if *t == ([6]float32{}) {
		return geom.Identity(), false
	}
	return geom.Affine{A: t[0], B: t[1], C: t[2], D: t[3], E: t[4], F: t[5]}, true
}

// MaxStops is the number of gradient stops an Instance holds.
const MaxStops = 4

// RampWidth is the number of texels in a row of Batch.Ramps. Texel i holds
// the color at i / (RampWidth-1) of the way along the gradient.
const RampWidth = 256

// FillRamp writes the stops given by colors and offsets as one row of
// Batch.Ramps into row, which holds RampWidth premultiplied RGBA8 texels.
// colors and offsets have the same length, at least one, and the offsets must
// not decrease. The colors blend in turn as in the pixel shader, so that a row
// of up to MaxStops stops matches what SetStops gives at every texel.
func FillRamp(row []byte, colors []geom.Color, offsets []float32) {
	if len(colors) == 0 || len(colors) != len(offsets) {
		panic("gowaku: FillRamp needs as many offsets as colors, and at least one")
	}
	row = row[:RampWidth*4]
	for i := range RampWidth {
		t := float32(i) / (RampWidth - 1)
		c := premultiplied(colors[0])
		for k := 1; k < len(colors); k++ {
			w := stopWeight(t, offsets[k-1], offsets[k])
			n := premultiplied(colors[k])
			for j := range c {
				c[j] += w * (n[j] - c[j])
			}
		}
		texel := row[i*4 : i*4+4]
		for j := range c {
			texel[j] = unorm8(c[j])
		}
	}
}

func premultiplied(c geom.Color) [4]float32 {
	return [4]float32{c.R * c.A, c.G * c.A, c.B * c.A, c.A}
}

// stopWeight is the weight of the pixel shader: how far t has come from one
// stop to the next.
func stopWeight(t, from, to float32) float32 {
	w := (t - from) / max(to-from, 1e-6)
	switch {
	case w >= 1:
		return 1
	case w > 0:
		return w
	}
	return 0
}

// unorm8 converts v the way a render target stores it: clamped and rounded to
// the nearest.
func unorm8(v float32) uint8 {
	switch {
	case v >= 1:
		return 255
	case v > 0:
		return uint8(v*255 + 0.5)
	}
	return 0
}

// SetStops stores the first n of colors and offsets as the stops of a
// gradient, padding as StopOffsets describes. n must be from 1 to MaxStops, and
// offsets must not decrease.
func (in *Instance) SetStops(colors *[MaxStops]geom.Color, offsets *[MaxStops]float32, n int) {
	if n < 1 || n > MaxStops {
		panic("gowaku: gradient needs 1 to gpu.MaxStops stops")
	}
	// The missing stops go first, so that the real ones keep their order.
	pad := MaxStops - n
	mid0, mid1 := max(1-pad, 0), max(2-pad, 0)
	in.Color, in.StopOffsets[0] = colors[0], offsets[0]
	in.MidColors[0], in.StopOffsets[1] = colors[mid0], offsets[mid0]
	in.MidColors[1], in.StopOffsets[2] = colors[mid1], offsets[mid1]
	in.EndColor, in.StopOffsets[3] = colors[n-1], offsets[n-1]
}

// TextureFormat is the pixel format of a Texture.
type TextureFormat uint8

// The texture formats.
const (
	// TextureR8 is 8-bit coverage, the format of the glyph atlas.
	TextureR8 TextureFormat = iota
	// TextureRGBA8 is premultiplied color.
	TextureRGBA8
	// TextureRGBA8Mipmapped is premultiplied color with the full mip chain
	// that FilterMipmap reads, MipLevels levels. UpdateTexture takes level 0,
	// all of it at once, and builds the other levels with BuildMips.
	TextureRGBA8Mipmapped
)

// BytesPerPixel returns the size of a pixel of level 0.
func (f TextureFormat) BytesPerPixel() int {
	if f == TextureR8 {
		return 1
	}
	return 4
}

// Texture is an image on the GPU.
type Texture interface {
	// Destroy releases the texture. Call it before destroying the Backend.
	Destroy()
}

// DrawKind says what a DrawCall does.
type DrawKind uint8

// The draw kinds. See docs/ADR/0009-paths-and-clips.md.
const (
	// DrawQuads draws instances (pipeline P1).
	DrawQuads DrawKind = iota
	// DrawFill marks a shape with the triangles of Vertices (P2) and paints it
	// with one InstanceCover (P3).
	DrawFill
	// DrawClipPush narrows the clip to the shape of Vertices. Its one
	// InstanceCover covers the shape, and the level goes up by one.
	DrawClipPush
	// DrawClipPop undoes a DrawClipPush inside its one InstanceCover.
	DrawClipPop
)

// DrawCall draws a run of instances under one clip.
type DrawCall struct {
	Kind         DrawKind
	First, Count int
	// VertexFirst and VertexCount delimit the triangles of the shape.
	VertexFirst, VertexCount int
	// EvenOdd selects the fill rule of the shape.
	EvenOdd bool
	// ClipLevel is the number of DrawClipPush in effect.
	ClipLevel uint8
	// Texture is what InstanceGlyph instances read. It can be nil if there are
	// none.
	Texture Texture
	// Image is what InstanceImage instances read, and tex0 of a Shader.
	Image Texture
	// Shader, if set, draws the instances, all InstanceShader, with this
	// pixel shader instead of the SDF one; Texture is then its tex1. The
	// backend must have prepared it with PrepareShader, or it skips the call.
	Shader *Shader
	// Clip limits drawing, in logical pixels, when Clipped is set.
	Clip    geom.Rect
	Clipped bool
	// Blend combines the instances with the target. A mode that ReadsBackdrop
	// makes the backend copy what Bounds covers of the target first, resolving
	// it if multisampled, and bind the copy as the backdrop. The instances of
	// one call all read that copy, so they should not overlap much.
	Blend BlendMode
	// Bounds encloses the quads of the instances, in logical pixels. It is set
	// for a Blend that ReadsBackdrop, and for a Blur.
	Bounds geom.Rect
	// Blur, when positive, makes the call draw one blurred picture: the
	// standard deviation of the Gaussian, in logical pixels. The call then has
	// a single instance. An InstanceBackdrop blurs what Bounds covers of the
	// target, as a backdrop filter does; an InstanceFiltered blurs its Image.
	// The backend runs the passes that PlanBlur describes before it draws the
	// instance. See docs/ADR/0044-drawing-extensions.md.
	Blur float32
}

// Batch is everything one Draw submits.
type Batch struct {
	Instances []Instance
	// Vertices are triangles in logical pixels.
	Vertices []geom.Offset
	Draws    []DrawCall
	// Clears are physical pixel rectangles that DrawToTarget makes transparent
	// before it draws. Draw ignores them.
	Clears []image.Rectangle
	// Ramps holds the gradients of instances with GradientRamp, a row of
	// RampWidth texels each, as a TextureRGBA8. It can be nil if there are none.
	Ramps Texture
}

// PhysicalBounds converts r from logical pixels to the edges of a physical
// pixel rectangle. Each edge rounds on its own, so that rectangles sharing a
// logical edge also share a physical one.
func PhysicalBounds(r geom.Rect, scale float32) (left, top, right, bottom int32) {
	round := func(v float32) int32 { return int32(geom.Floor32(v*scale + 0.5)) }
	return round(r.X), round(r.Y), round(r.X + r.W), round(r.Y + r.H)
}

// Multisampled reports whether b holds anything that needs multisampling: paths
// and clips of any shape, whose edges come from the rasterizer. Quads smooth their
// edges in the pixel shader, and small paths come from the atlas.
func (b *Batch) Multisampled() bool {
	for i := range b.Draws {
		if b.Draws[i].Kind != DrawQuads {
			return true
		}
	}
	return false
}

// Surface is the drawable area of one window.
type Surface interface {
	// SetMultisample chooses whether the frames from the next BeginFrame on draw
	// to the multisampled targets, which a surface creates when it first needs
	// them. New surfaces start with it on. It has no effect on a device without
	// MSAA. See docs/ADR/0017-multisample-on-demand.md.
	SetMultisample(on bool)
	// Resize adapts the surface to the window's new client area. It ignores
	// empty sizes, which is what a minimized window reports. Keep drawing
	// suspended until the next non-empty size.
	Resize(physicalWidth, physicalHeight int, scale float32) error
	// Destroy releases the surface. Call it before closing the window.
	Destroy()
}

// Backend draws to any number of surfaces with a single GPU device.
//
// All methods must be called on the UI goroutine.
type Backend interface {
	// CreateSurface creates a surface for a window.
	CreateSurface(h platform.NativeHandle, physicalWidth, physicalHeight int, scale float32) (Surface, error)
	// BeginFrame starts a frame on s and fills it with background.
	BeginFrame(s Surface, background geom.Color) error
	// Draw adds batch to the frame begun on s.
	Draw(s Surface, batch *Batch) error
	// EndFrame presents s. It panics if the device was lost.
	EndFrame(s Surface) error
	// CreateTexture creates an empty texture. Call it outside a frame: Vulkan
	// records uploads ahead of the render pass.
	CreateTexture(width, height int, format TextureFormat) (Texture, error)
	// UpdateTexture replaces region of t. pix holds the pixels of a larger
	// image with the given stride in bytes, and the region has the same place
	// in it. Call it outside a frame, like CreateTexture.
	UpdateTexture(t Texture, region image.Rectangle, pix []byte, stride int) error
	// CreateTarget creates a texture that DrawToTarget can draw to. It holds
	// premultiplied color, starts transparent, and serves as DrawCall.Image. Call
	// it outside a frame. See docs/ADR/0019-layers.md.
	CreateTarget(width, height int) (Texture, error)
	// DrawToTarget clears batch.Clears on t and draws batch over the rest of what
	// t holds. The coordinates of batch are relative to t, and scale is that of
	// the surface the result will end up on. Call it outside a frame. batch must
	// not read t.
	//
	// A batch that needs multisampling (Batch.Multisampled) is drawn on a
	// multisampled scratch target that starts transparent, with depth and
	// stencil at zero, and resolved into the bounds of batch.Clears, which it
	// replaces: it must have at least one clear, and what t held there before
	// is lost. Its calls must neither read the backdrop nor blur. See
	// docs/ADR/0114-gpu-layers-masks-filters.md.
	DrawToTarget(t Texture, batch *Batch, scale float32) error
	// PrepareShader makes ready what drawing with s needs, such as its
	// pipeline, and reports whether the GPU takes it. Call it outside a frame,
	// once per Shader; what it makes lasts as long as the device. Draw skips
	// the calls of a shader that was not prepared, or failed to be.
	PrepareShader(s *Shader) error
	// Destroy releases the device. It panics if a surface is still alive.
	Destroy()
}
