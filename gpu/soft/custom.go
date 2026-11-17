package soft

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

// custom is main of the epilogue of gowaku-shader, with Shader.Fallback in
// place of the HLSL: it builds the input, runs the fallback, and applies the
// opacity and the coverage of a turned edge as image does. Keep it in step with
// internal/shadertool/hlsl/epilogue.hlsli.
func (f *fragment) custom() (vec4, float32) {
	in := f.in
	zoom := in.Blur
	if !(zoom > 0) {
		zoom = 1
	}
	r := in.Rect
	coverage := float32(1)
	var local geom.Offset
	if f.turned() {
		// turnedQuad hands the exact inverse of the pixel center.
		local = geom.Offset{X: clamp(f.logical.X-r.X, 0, r.W), Y: clamp(f.logical.Y-r.Y, 0, r.H)}
		p, halfSize := f.centered()
		var square [4]float32
		distance := roundedBoxDistance(p, halfSize, square)
		coverage = saturate(0.5 - distance/f.edgeStep(roundedBoxNormal(p, halfSize, square)))
	} else {
		local = geom.Offset{X: (float32(f.pixelX)+0.5)/f.scale - r.X, Y: (float32(f.pixelY)+0.5)/f.scale - r.Y}
	}
	f.samplers[0] = shaderSampler{t: f.picture, at: in.ShaderTexture(0)}
	f.samplers[1] = shaderSampler{t: f.atlas, at: in.ShaderTexture(1)}
	f.input = gpu.ShaderInput{
		Pos:    geom.Offset{X: local.X / zoom, Y: local.Y / zoom},
		Size:   geom.Size{W: r.W / zoom, H: r.H / zoom},
		UV:     geom.Offset{X: local.X / r.W, Y: local.Y / r.H},
		Time:   in.BorderWidth,
		Scale:  f.scale * zoom,
		Params: in.ShaderParams(),
		Tex0:   &f.samplers[0],
		Tex1:   &f.samplers[1],
	}
	c := f.shader.Fallback(&f.input)
	return vec4{c.R, c.G, c.B, c.A}.times(in.Color.A * coverage), coverage
}

func clamp(v, low, high float32) float32 { return min(max(v, low), high) }

// shaderSampler is sample0 and size0 of the prelude of gowaku-shader for one
// texture: bilinear with exact weights, transparent outside the texels of the
// picture.
type shaderSampler struct {
	t  *Texture
	at gpu.ShaderTexture
}

// Sample implements gpu.ShaderSampler.
func (s *shaderSampler) Sample(uv geom.Offset) geom.Color {
	p := &s.at.Place
	x := p[0] + uv.X*(p[2]-p[0]) - 0.5
	y := p[1] + uv.Y*(p[3]-p[1]) - 0.5
	bx, by := geom.Floor32(x), geom.Floor32(y)
	fx, fy := x-bx, y-by
	tx, ty := int(bx), int(by)
	top := lerp(s.texel(tx, ty), s.texel(tx+1, ty), fx)
	bottom := lerp(s.texel(tx, ty+1), s.texel(tx+1, ty+1), fx)
	c := lerp(top, bottom, fy)
	return geom.Color{R: c[0], G: c[1], B: c[2], A: c[3]}
}

// texel is gowakuTexel0: what Load reads inside the picture, zero outside.
func (s *shaderSampler) texel(x, y int) vec4 {
	r := &s.at.Texels
	fx, fy := float32(x), float32(y)
	if s.t == nil || fx < r[0] || fy < r[1] || fx >= r[2] || fy >= r[3] || x >= s.t.width || y >= s.t.height {
		return vec4{}
	}
	return s.t.texel(x, y)
}

// Size implements gpu.ShaderSampler.
func (s *shaderSampler) Size() geom.Size {
	p := &s.at.Place
	return geom.Size{W: geom.Abs32(p[2] - p[0]), H: geom.Abs32(p[3] - p[1])}
}
