package render

import (
	"errors"
	"fmt"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
)

// shaderLayerPageLimit bounds the pages of the layers that custom shaders read,
// on top of layerPageLimit: 64 MB.
const shaderLayerPageLimit = 4

// shaderEntry is what render keeps of a paint.Shader: the gpu.Shader that every
// draw call of it shares, and whether the backend took it.
type shaderEntry struct {
	code     gpu.Shader
	prepared bool
	failed   bool
}

// shaderOf returns the entry of s, made on first use. A new entry waits for
// Finish to prepare it.
func (g *Resources) shaderOf(s *paint.Shader) *shaderEntry {
	if e := g.shaders[s]; e != nil {
		return e
	}
	e := &shaderEntry{code: gpu.Shader{Name: s.Name, DXBC: s.DXBC, SPIRV: s.SPIRV, MSL: s.MSL}}
	if s.Fallback != nil {
		e.code.Fallback = fallbackOf(s.Fallback)
	}
	g.shaders[s] = e
	g.unprepared = append(g.unprepared, e)
	return e
}

// fallbackOf adapts a paint.ShaderFallback to the gpu.Shader that gpu/soft
// runs. The input lives as long as the closure, so that a call does not
// allocate; soft draws on one goroutine.
func fallbackOf(f paint.ShaderFallback) func(*gpu.ShaderInput) geom.Color {
	in := &paint.ShaderInput{}
	return func(g *gpu.ShaderInput) geom.Color {
		in.Pos, in.Size, in.UV, in.Time, in.Scale = g.Pos, g.Size, g.UV, g.Time, g.Scale
		in.Params = g.Params
		in.Tex0, in.Tex1 = g.Tex0, g.Tex1
		return f(in)
	}
}

// encodeShader adds the instance of an OpShader. A texture that names a layer
// that cannot be one, as one that has to be drawn inline, makes the call draw
// its layers as they are instead, without the shader.
func (g *Resources) encodeShader(op *paint.Op, dl *paint.DisplayList, scale float32, b *gpu.Batch) {
	if g == nil {
		// Without Resources nothing knows the shader.
		return
	}
	call := &dl.Shaders[op.ShaderCall]
	e := g.shaderOf(call.Shader)
	if e.failed {
		return
	}
	var textures [2]gpu.Texture
	var places [2]gpu.ShaderTexture
	for k, t := range [2]*paint.ShaderTexture{&call.Tex0, &call.Tex1} {
		var ok bool
		textures[k], places[k], ok = g.shaderTexture(t, op, scale)
		if !ok {
			g.drawLayersPlain(call, op, scale, b)
			return
		}
	}
	addShaderDraw(b, op, &e.code, textures[0], textures[1])
	var zoom float32
	if op.Scale != 0 && op.Scale != 1 {
		zoom = op.Scale
	}
	b.Instances = append(b.Instances, gpu.Instance{
		Rect:        op.RRect.Rect,
		Color:       op.Color,
		BorderWidth: float32(call.Time.Seconds()),
		Blur:        zoom,
		Kind:        gpu.InstanceShader,
	})
	in := &b.Instances[len(b.Instances)-1]
	in.SetShaderParams((*[gpu.ShaderParamCount]float32)(&call.Params))
	in.SetShaderTexture(0, places[0])
	in.SetShaderTexture(1, places[1])
	if op.Transform != (geom.Affine{}) {
		in.SetTransform(op.Transform)
	}
}

// The params of paint and gpu must have the same length; this fails to
// compile if they ever differ.
const _ = uint(gpu.ShaderParamCount-len(paint.ShaderParams{})) + uint(len(paint.ShaderParams{})-gpu.ShaderParamCount)

// shaderTexture returns the texture of t and where it lies for the shader of op.
// It reports false for a layer that cannot be a texture.
func (g *Resources) shaderTexture(t *paint.ShaderTexture, op *paint.Op, scale float32) (gpu.Texture, gpu.ShaderTexture, bool) {
	switch {
	case t.Image != nil:
		tex := g.imageTexture(t.Image)
		if tex == nil {
			return nil, gpu.ShaderTexture{}, true
		}
		_, width, height := t.Image.Pixels()
		w, h := float32(width), float32(height)
		return tex, gpu.ShaderTexture{Place: [4]float32{0, 0, w, h}, Texels: [4]float32{0, 0, w, h}}, true
	case t.Layer != nil:
		return g.layerTexture(t, op, scale)
	}
	return nil, gpu.ShaderTexture{}, true
}

// layerTexture puts the layer of t on a page of shaderLayers and returns where
// its pixels lie. A layer is one only from the frame or a layer of the frame:
// one that a layer of shaderLayers reads would have to be drawn before it, on
// a page that is drawn first.
func (g *Resources) layerTexture(t *paint.ShaderTexture, op *paint.Op, scale float32) (gpu.Texture, gpu.ShaderTexture, bool) {
	if g.shaderDepth > 0 || g.groupDepth > 0 {
		// The pages of these layers are drawn after the pictures of groups.
		return nil, gpu.ShaderTexture{}, false
	}
	pool := &g.shaderLayers
	e := pool.entry(t.Layer)
	g.shaderDepth++
	// It is drawn as one picture, which keeps it from waiting for calm.
	g.refresh(pool, t.Layer, e, scale, true)
	g.shaderDepth--
	if e.inline {
		return nil, gpu.ShaderTexture{}, false
	}
	if e.width == 0 {
		return nil, gpu.ShaderTexture{}, true
	}
	e.page.lastUsed = g.frame
	left, top := snapOrigin(t.Origin, scale)
	x := float32(e.slot.Min.X + layerPadding)
	y := float32(e.slot.Min.Y + layerPadding)
	r := op.RRect.Rect
	// The physical pixel at r.X*scale of the surface is the texel at
	// x + r.X*scale - (left + e.left) of the page.
	px := x + r.X*scale - (left + float32(e.left))
	py := y + r.Y*scale - (top + float32(e.top))
	return e.page.target, gpu.ShaderTexture{
		Place:  [4]float32{px, py, px + r.W*scale, py + r.H*scale},
		Texels: [4]float32{x, y, x + float32(e.width), y + float32(e.height)},
	}, true
}

// drawLayersPlain draws the layers of call where they are, under the clip and
// transform of op, as if there were no shader.
func (g *Resources) drawLayersPlain(call *paint.ShaderCall, op *paint.Op, scale float32, b *gpu.Batch) {
	for _, t := range [2]*paint.ShaderTexture{&call.Tex0, &call.Tex1} {
		if t.Layer == nil {
			continue
		}
		plain := paint.Op{
			Kind:      paint.OpLayer,
			Layer:     t.Layer,
			Origin:    t.Origin,
			Scale:     1,
			Transform: op.Transform,
			Clip:      op.Clip,
			Clipped:   op.Clipped,
			ClipLevel: op.ClipLevel,
		}
		scratch := g.takeScratch()
		encodeInline(&plain, g, scale, b, scratch)
		g.scratchUsed--
	}
}

// takeScratch returns a batch to encode into that nothing else uses. Plain
// layers can nest, each needing one of its own.
func (g *Resources) takeScratch() *gpu.Batch {
	if g.scratchUsed == len(g.scratches) {
		g.scratches = append(g.scratches, &gpu.Batch{})
	}
	s := g.scratches[g.scratchUsed]
	g.scratchUsed++
	return s
}

// addShaderDraw makes room for one more instance of shader in the last draw
// call, or starts a new one. Only the same shader with the same textures joins
// a call, or an empty one that beginBackdrop made for op.
func addShaderDraw(b *gpu.Batch, op *paint.Op, shader *gpu.Shader, tex0, tex1 gpu.Texture) {
	mode := gpu.BlendMode(op.Blend)
	if n := len(b.Draws); n > 0 {
		d := &b.Draws[n-1]
		empty := d.Count == 0 && d.Shader == nil && d.First == len(b.Instances)
		same := d.Shader == shader && d.Image == tex0 && d.Texture == tex1
		if d.Kind == gpu.DrawQuads && d.Blur == 0 && d.Blend == mode && sameClip(d, op) && (empty || same) &&
			(op.Blend == paint.BlendSrcOver || backdropFits(d, op)) {
			d.Count++
			d.Shader, d.Image, d.Texture = shader, tex0, tex1
			return
		}
	}
	b.Draws = append(b.Draws, gpu.DrawCall{
		First:     len(b.Instances),
		Count:     1,
		Texture:   tex1,
		Image:     tex0,
		Shader:    shader,
		Clip:      op.Clip,
		Clipped:   op.Clipped,
		ClipLevel: op.ClipLevel,
		Blend:     mode,
	})
	if op.Blend != paint.BlendSrcOver {
		b.Draws[len(b.Draws)-1].Bounds = opReach(op)
	}
}

// prepareShaders hands the shaders first used since the last Finish to the
// backend. One it refuses draws nothing from then on: its calls leave b and the
// pages, and DrawError reports it, once.
func (g *Resources) prepareShaders(b *gpu.Batch) {
	if len(g.unprepared) == 0 {
		return
	}
	failed := false
	for _, e := range g.unprepared {
		e.prepared = true
		if err := g.store.PrepareShader(&e.code); err != nil {
			e.failed, failed = true, true
			g.drawErrs = append(g.drawErrs, fmt.Errorf("render: shader %q: %w", e.code.Name, err))
		}
	}
	clear(g.unprepared)
	g.unprepared = g.unprepared[:0]
	if !failed {
		return
	}
	dropFailed(b, g.shaders)
	for i := range g.jobs {
		dropFailed(g.jobs[i].batch, g.shaders)
	}
	for _, pool := range [...]*layerPool{&g.layers, &g.shaderLayers} {
		for _, p := range pool.pages {
			dropFailed(&p.batch, g.shaders)
		}
	}
}

// dropFailed empties the calls of b whose shader the backend refused.
func dropFailed(b *gpu.Batch, shaders map[*paint.Shader]*shaderEntry) {
	for i := range b.Draws {
		d := &b.Draws[i]
		if d.Shader == nil {
			continue
		}
		for _, e := range shaders {
			if &e.code == d.Shader && e.failed {
				d.Count = 0
			}
		}
	}
}

// DrawError returns what kept part of the frames since the last call from being
// drawn, such as a custom shader that the GPU refused, and forgets it. Unlike
// an error of Finish it does not stop the frame: the rest is drawn. It returns
// nil if there was nothing.
func (g *Resources) DrawError() error {
	if len(g.drawErrs) == 0 {
		return nil
	}
	err := errors.Join(g.drawErrs...)
	g.drawErrs = nil
	return err
}
