// Package shaders holds the custom shaders of examples/shader, written in HLSL
// and compiled by cmd/gowaku-shader, with the Go fallbacks that let the CPU
// renderer of gowakutest draw them, and the scene of the example, which the
// golden images of internal/golden draw too.
//
// Each fallback does in float32 what its HLSL does, so that the CPU draws
// what the GPUs draw.
package shaders

//go:generate go run ../../../cmd/gowaku-shader -fallback auroraFallback aurora.hlsl
//go:generate go run ../../../cmd/gowaku-shader -fallback shimmerFallback shimmer.hlsl
//go:generate go run ../../../cmd/gowaku-shader -fallback rippleFallback ripple.hlsl
//go:generate go run ../../../cmd/gowaku-shader -fallback crossfadeFallback crossfade.hlsl

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

func saturate(v float32) float32 { return min(max(v, 0), 1) }

func lerp(a, b, t float32) float32 { return a + t*(b-a) }

// auroraFallback is aurora.hlsl.
func auroraFallback(in *paint.ShaderInput) geom.Color {
	p := &in.Params
	cx, cy := in.Pos.X-in.Size.W*0.5, in.Pos.Y-in.Size.H*0.5
	d := geom.Sqrt32(cx*cx + cy*cy)
	wave := 0.5 + 0.5*geom.Sin32(d*p[8]-in.Time*p[9])
	t := saturate(in.UV.X + (wave-0.5)*p[10])
	r, g, b, a := lerp(p[0], p[4], t), lerp(p[1], p[5], t), lerp(p[2], p[6], t), lerp(p[3], p[7], t)
	return geom.Color{R: r * a, G: g * a, B: b * a, A: a}
}

// shimmerFallback is shimmer.hlsl.
func shimmerFallback(in *paint.ShaderInput) geom.Color {
	p := &in.Params
	c := in.Tex0.Sample(in.UV)
	pass := in.Time * p[0]
	at := (pass-geom.Floor32(pass))*1.6 - 0.3
	x := in.UV.X + (in.UV.Y-0.5)*p[2]
	band := saturate(1 - geom.Abs32(x-at)/p[1])
	w := band * p[7]
	return geom.Color{
		R: lerp(c.R, p[4]*c.A, w),
		G: lerp(c.G, p[5]*c.A, w),
		B: lerp(c.B, p[6]*c.A, w),
		A: c.A,
	}
}

// rippleFallback is ripple.hlsl.
func rippleFallback(in *paint.ShaderInput) geom.Color {
	p := &in.Params
	cx, cy := in.Pos.X-p[0]*in.Size.W, in.Pos.Y-p[1]*in.Size.H
	d := geom.Sqrt32(cx*cx + cy*cy)
	reach := 0.5 * geom.Sqrt32(in.Size.W*in.Size.W+in.Size.H*in.Size.H)
	push := p[2] * geom.Sin32(d*p[3]-in.Time*p[4]) * saturate(1-d/reach)
	var dx, dy float32
	if d > 0 {
		dx, dy = cx/d, cy/d
	}
	return in.Tex0.Sample(geom.Offset{X: in.UV.X + dx*push/in.Size.W, Y: in.UV.Y + dy*push/in.Size.H})
}

// crossfadeFallback is crossfade.hlsl.
func crossfadeFallback(in *paint.ShaderInput) geom.Color {
	p := &in.Params
	a, b := in.Tex0.Sample(in.UV), in.Tex1.Sample(in.UV)
	t := p[0]
	c := geom.Color{R: lerp(a.R, b.R, t), G: lerp(a.G, b.G, t), B: lerp(a.B, b.B, t), A: lerp(a.A, b.A, t)}
	w := p[7]
	return geom.Color{R: lerp(c.R, p[4]*c.A, w), G: lerp(c.G, p[5]*c.A, w), B: lerp(c.B, p[6]*c.A, w), A: c.A}
}
