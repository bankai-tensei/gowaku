package gpu

import "github.com/bankai-tensei/gowaku/geom"

// Shader is a custom pixel shader that gowaku-shader made from one HLSL source.
// Backends pair it with the vertex shader of the SDF pipeline, so that clips,
// transforms, layers and blend modes apply as to any instance. render hands the
// same *Shader to every draw call of one paint.Shader, so that a backend can
// keep what it makes of it by pointer. See
// docs/ADR/0045-shaders-svg-animated-images.md.
type Shader struct {
	// Name says which shader it is in error messages.
	Name string
	// DXBC is the pixel shader for Direct3D 11, SPIRV the fragment shader for
	// Vulkan and MSL the source for Metal, which nothing runs yet.
	DXBC, SPIRV, MSL []byte
	// Fallback paints the shader on gpu/soft, which cannot run it. It
	// returns the premultiplied color at in. Without one, soft draws nothing.
	Fallback func(in *ShaderInput) geom.Color
}

// ShaderParamCount is the number of floats a shader instance carries for its
// shader, as four float4.
const ShaderParamCount = 16

// ShaderInput is what a Shader sees at one pixel. It mirrors struct ShaderInput
// of the prelude that gowaku-shader adds, and paint.ShaderInput, field by
// field.
type ShaderInput struct {
	// Pos is the position of the pixel center in the rectangle, from its top
	// left, in the logical pixels of the canvas that drew it.
	Pos geom.Offset
	// Size is the size of the rectangle in the same units.
	Size geom.Size
	// UV is Pos divided by Size.
	UV geom.Offset
	// Time is in seconds.
	Time float32
	// Scale is the number of physical pixels per unit of Pos.
	Scale float32
	// Params are params[0] to params[3] in a row.
	Params [ShaderParamCount]float32
	// Tex0 and Tex1 read the textures, transparent where there is none.
	Tex0, Tex1 ShaderSampler
}

// ShaderSampler reads one texture of a shader, as sample0 and size0 of the
// prelude do. Its methods use only geom types, so that paint.ShaderSampler
// accepts the same values.
type ShaderSampler interface {
	// Sample blends the four texels around uv, where 0 and 1 are the edges of
	// the rectangle. Texels outside the texture read as transparent. The color
	// is premultiplied.
	Sample(uv geom.Offset) geom.Color
	// Size is how many texels span the rectangle along each axis.
	Size() geom.Size
}

// The fields of an InstanceShader. It has no shape of its own, so the fields
// of the others carry its data:
//
//	Rect           the rectangle, as for any instance
//	Color.A        the opacity
//	BorderWidth    the time in seconds
//	Blur           the zoom of the canvas: units of Pos per logical pixel of Rect, zero for 1
//	Radii          params[0]
//	StopOffsets    params[1]
//	GradientMatrix params[2]
//	GradientStart, GradientEnd  params[3]
//	EndColor       where tex0 lies: its texel at the top left of Rect and at the bottom right
//	BorderColor    the texels of tex0 that hold the picture, left, top, right, bottom
//	MidColors      the same for tex1
//
// render does not move GradientStart and GradientEnd with the instance, nor
// fade the other colors with Color.A.

// SetShaderParams stores p in an InstanceShader.
func (in *Instance) SetShaderParams(p *[ShaderParamCount]float32) {
	in.Radii = [4]float32{p[0], p[1], p[2], p[3]}
	in.StopOffsets = [4]float32{p[4], p[5], p[6], p[7]}
	in.GradientMatrix = [4]float32{p[8], p[9], p[10], p[11]}
	in.GradientStart = geom.Offset{X: p[12], Y: p[13]}
	in.GradientEnd = geom.Offset{X: p[14], Y: p[15]}
}

// ShaderParams returns what SetShaderParams stored.
func (in *Instance) ShaderParams() [ShaderParamCount]float32 {
	r, s, m := in.Radii, in.StopOffsets, in.GradientMatrix
	return [ShaderParamCount]float32{
		r[0], r[1], r[2], r[3],
		s[0], s[1], s[2], s[3],
		m[0], m[1], m[2], m[3],
		in.GradientStart.X, in.GradientStart.Y, in.GradientEnd.X, in.GradientEnd.Y,
	}
}

// ShaderTexture places a texture of an InstanceShader. Place is where its texels
// lie: the texel position at the top left corner of Rect, in X and Y, and at the
// bottom right, in Z and W, with texel centers at half numbers. Texels holds the
// texels that hold the picture, from Left and Top up to but not including Right
// and Bottom; the others read as transparent. The zero value has no texels.
type ShaderTexture struct {
	Place  [4]float32
	Texels [4]float32
}

// SetShaderTexture stores where texture slot, 0 or 1, of an InstanceShader lies.
func (in *Instance) SetShaderTexture(slot int, t ShaderTexture) {
	place := geom.Color{R: t.Place[0], G: t.Place[1], B: t.Place[2], A: t.Place[3]}
	texels := geom.Color{R: t.Texels[0], G: t.Texels[1], B: t.Texels[2], A: t.Texels[3]}
	if slot == 0 {
		in.EndColor, in.BorderColor = place, texels
		return
	}
	in.MidColors[0], in.MidColors[1] = place, texels
}

// ShaderTexture returns what SetShaderTexture stored for slot.
func (in *Instance) ShaderTexture(slot int) ShaderTexture {
	place, texels := in.EndColor, in.BorderColor
	if slot != 0 {
		place, texels = in.MidColors[0], in.MidColors[1]
	}
	return ShaderTexture{
		Place:  [4]float32{place.R, place.G, place.B, place.A},
		Texels: [4]float32{texels.R, texels.G, texels.B, texels.A},
	}
}
