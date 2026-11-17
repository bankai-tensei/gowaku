package paint

import (
	"hash/fnv"
	"time"

	"github.com/bankai-tensei/gowaku/geom"
)

// Shader is a custom pixel shader: one HLSL function, compiled for each GPU by
// cmd/gowaku-shader, which writes the Go file that creates it with NewShader.
// It fills a rectangle, reading the fixed values of ShaderInput, sixteen floats
// of ShaderParams and up to two textures. See
// docs/ADR/0045-shaders-svg-animated-images.md.
//
// A Shader is shared by everything that draws with it, and must not change
// once drawn.
type Shader struct {
	// Name says which shader it is in error messages, usually the name of
	// its source file without the extension.
	Name string
	// DXBC is the pixel shader for Direct3D 11, SPIRV the fragment shader for
	// Vulkan, and MSL the source that Metal compiles.
	DXBC, SPIRV, MSL []byte
	// Fallback paints the shader where no GPU runs it: the CPU renderer of
	// gowakutest. Without one, the shader draws nothing there, and
	// gowakutest.Warnings reports it.
	Fallback ShaderFallback
	// ID identifies the code: a hash of DXBC, SPIRV and MSL, which NewShader
	// sets. Two shaders of the same code have the same ID.
	ID uint64
}

// NewShader returns a copy of s with its ID set.
func NewShader(s Shader) *Shader {
	h := fnv.New64a()
	for _, b := range [...][]byte{s.DXBC, s.SPIRV, s.MSL} {
		_, _ = h.Write(b) // a hash.Hash never fails to write
		_, _ = h.Write([]byte{0})
	}
	s.ID = h.Sum64()
	return &s
}

// ShaderFallback computes the premultiplied color of a Shader at one pixel on
// the CPU. It should do what the HLSL does, in float32, so that the CPU
// renderer draws what the GPUs draw.
type ShaderFallback func(in *ShaderInput) geom.Color

// ShaderParams are the sixteen floats a Shader reads as params[0] to params[3]
// of ShaderInput: params[k] is the float4 of P[4k] to P[4k+3].
type ShaderParams [16]float32

// ShaderInput is what a ShaderFallback sees at one pixel. It mirrors the HLSL
// struct ShaderInput:
//
//	struct ShaderInput {
//	    float2 pos;       // Pos
//	    float2 size;      // Size
//	    float2 uv;        // UV
//	    float time;       // Time
//	    float scale;      // Scale
//	    float4 params[4]; // Params
//	};
//
// and Tex0 and Tex1 stand for sample0, size0, sample1 and size1.
type ShaderInput struct {
	// Pos is the position of the pixel center from the top left of the
	// rectangle, in logical pixels of the canvas that drew it.
	Pos geom.Offset
	// Size is the size of the rectangle in the same units.
	Size geom.Size
	// UV is Pos divided by Size, from 0 to 1 across the rectangle.
	UV geom.Offset
	// Time is ShaderCall.Time in seconds.
	Time float32
	// Scale is the number of physical pixels per unit of Pos.
	Scale  float32
	Params ShaderParams
	// Tex0 and Tex1 read the textures. They read transparent if there are
	// none.
	Tex0, Tex1 ShaderSampler
}

// ShaderSampler reads a texture of a Shader.
type ShaderSampler interface {
	// Sample is sample0 or sample1 in HLSL: it blends the four texels around
	// uv, where 0 and 1 are the edges of the rectangle. Texels outside the
	// texture read as transparent. The color is premultiplied.
	Sample(uv geom.Offset) geom.Color
	// Size is size0 or size1: how many texels span the rectangle along each
	// axis.
	Size() geom.Size
}

// ShaderTexture is a texture a Shader reads: an Image, or the pixels of a Layer
// whose origin lies at Origin, in the coordinates of the canvas. The zero value
// is none.
type ShaderTexture struct {
	Image  *Image
	Layer  *Layer
	Origin geom.Offset
}

// ShaderCall is what Canvas.DrawShader draws: a Shader with its parameters,
// the time it reads and its textures. In a DisplayList, the origins of the
// textures are on the surface.
type ShaderCall struct {
	Shader *Shader
	Params ShaderParams
	Time   time.Duration
	// Tex0 and Tex1 are what sample0 and sample1 read. Texture coordinates
	// follow the rectangle of the shader: an image is stretched over it, and
	// a layer stays at its own place and size, so that a layer of what is
	// under the rectangle lines up with it.
	Tex0, Tex1 ShaderTexture
}

// DrawShader fills dst with call.Shader, faded by the opacity in effect and
// blended by the mode that PushBlend set. Clips and transforms apply as to any
// shape. A nil Shader draws nothing. It keeps pointers to the shader and the
// textures until the next Reset.
//
// A ShaderTexture may name an Image or a Layer, not both. A Layer must have been
// recorded at the scale of the surface: DrawShader panics if a texture names a
// Layer while a Scale is in effect, which CanShadeLayer reports.
func (c *Canvas) DrawShader(dst geom.Rect, call ShaderCall) {
	if call.Shader == nil {
		return
	}
	for _, t := range [...]*ShaderTexture{&call.Tex0, &call.Tex1} {
		if t.Image != nil && t.Layer != nil {
			panic("gowaku: a ShaderTexture names both an Image and a Layer")
		}
		if t.Layer != nil {
			if !c.CanShadeLayer() {
				panic("gowaku: Canvas.DrawShader got a Layer while CanShadeLayer reports false")
			}
			t.Origin = c.place(t.Origin)
		}
	}
	bounds := c.placeRect(dst)
	if bounds.Size().Empty() || c.state.opacity <= 0 || !c.visible(bounds) {
		return
	}
	c.list.Shaders = append(c.list.Shaders, call)
	c.list.Ops = append(c.list.Ops, Op{
		Kind:       OpShader,
		Blend:      c.state.blend,
		RRect:      geom.RRect{Rect: bounds},
		Color:      geom.Color{R: 1, G: 1, B: 1, A: c.state.opacity},
		Scale:      c.state.scale,
		ShaderCall: int32(len(c.list.Shaders) - 1),
		Transform:  c.state.transform,
		Clip:       c.state.clip,
		Clipped:    c.state.clipped,
		ClipLevel:  c.state.level,
	})
}

// CanShadeLayer reports whether DrawShader can read a Layer: no Scale may be in
// effect, as the pixels of a layer follow the surface. Unlike DrawLayer it
// works inside a layer, so that a ShaderMask works anywhere but under a scale
// transition.
func (c *Canvas) CanShadeLayer() bool { return c.state.scale == 1 }
