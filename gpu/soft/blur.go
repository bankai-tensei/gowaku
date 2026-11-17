package soft

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

// blur runs the passes of the blur of call, as the GPU backends do, and leaves
// the blurred picture in c.filtered for the instance of the call. It reports
// whether the call draws anything. Like the GPUs, each pass stores 8 bits per
// channel.
func (c *canvas) blur(batch *gpu.Batch, call *gpu.DrawCall) bool {
	in := &batch.Instances[call.First]
	picture, _ := call.Image.(*Texture)
	width, height := c.width, c.height
	fromImage := in.Kind.Base() == gpu.InstanceFiltered
	if fromImage {
		if picture == nil {
			return false
		}
		width, height = picture.width, picture.height
	}
	b, ok := gpu.PlanBlur(call, in, c.scale, width, height)
	if !ok {
		return false
	}
	if !fromImage {
		c.copyBackdrop(b.Source)
	}
	c.blurA = scratch(c.blurA, b.Out.Max.X, b.Out.Max.Y)
	c.blurB = scratch(c.blurB, b.Out.Max.X, b.Out.Max.Y)
	var passes [3]gpu.Instance
	b.Passes(&passes, c.scale, fromImage)
	for y := b.Out.Min.Y; y < b.Out.Max.Y; y++ {
		for x := b.Out.Min.X; x < b.Out.Max.X; x++ {
			c.blurA.store(x, y, c.blurDown(&passes[0], picture, x, y))
		}
	}
	for y := b.Out.Min.Y; y < b.Out.Max.Y; y++ {
		for x := b.Out.Min.X; x < b.Out.Max.X; x++ {
			c.blurB.store(x, y, blurLine(&passes[1], c.blurA, x, y, 1, 0))
		}
	}
	for y := b.Out.Min.Y; y < b.Out.Max.Y; y++ {
		for x := b.Out.Min.X; x < b.Out.Max.X; x++ {
			c.blurA.store(x, y, blurLine(&passes[2], c.blurB, x, y, 0, 1))
		}
	}
	c.filtered = c.blurA
	return true
}

// scratch returns t, or a larger texture if t is smaller than width by height.
func scratch(t *Texture, width, height int) *Texture {
	if t != nil && t.width >= width && t.height >= height {
		return t
	}
	if t != nil {
		width, height = max(width, t.width), max(height, t.height)
	}
	return &Texture{width: width, height: height, format: gpu.TextureRGBA8, target: true, pix: make([]uint8, width*height*4)}
}

// store writes v to a texel, as a render target stores it.
func (t *Texture) store(x, y int, v vec4) {
	s := v.stored()
	copy(t.pix[(y*t.width+x)*4:], s[:])
}

// clampTexel keeps (x, y) inside the inclusive rectangle r.
func clampTexel(x, y int, r *[4]float32) (int, int) {
	return min(max(x, int(r[0])), int(r[2])), min(max(y, int(r[1])), int(r[3]))
}

// blurDown is a port of blurDown in gpu/shaders/sdf.ps.hlsl.
func (c *canvas) blurDown(in *gpu.Instance, picture *Texture, ox, oy int) vec4 {
	f := int(in.BorderWidth)
	var sum vec4
	for y := range f {
		for x := range f {
			tx, ty := clampTexel(ox*f+x, oy*f+y, &in.Radii)
			if in.Blur > 0 {
				sum = sum.add(picture.texel(tx, ty))
			} else {
				sum = sum.add(c.backdropAt(tx, ty))
			}
		}
	}
	return sum.over(float32(f * f))
}

// blurLine is a port of blurLine in gpu/shaders/sdf.ps.hlsl.
func blurLine(in *gpu.Instance, src *Texture, ox, oy, dx, dy int) vec4 {
	sigma := in.BorderWidth
	radius := gpu.BlurRadius(sigma)
	falloff := -0.5 / (sigma * sigma)
	var sum vec4
	var total float32
	for k := -radius; k <= radius; k++ {
		w := exp(float32(k*k) * falloff)
		tx, ty := clampTexel(ox+dx*k, oy+dy*k, &in.Radii)
		sum = sum.add(src.texel(tx, ty).times(w))
		total += w
	}
	return sum.over(total)
}

// over divides every channel by d, as the shader does.
func (a vec4) over(d float32) vec4 { return vec4{a[0] / d, a[1] / d, a[2] / d, a[3] / d} }

// blurredTexels is a port of blurredTexels in gpu/shaders/sdf.ps.hlsl.
func (f *fragment) blurredTexels(px, py float32) vec4 {
	t := f.filtered
	if t == nil {
		return vec4{}
	}
	baseX, baseY := geom.Floor32(px), geom.Floor32(py)
	fx, fy := px-baseX, py-baseY
	lowX, lowY := max(int(baseX), 0), max(int(baseY), 0)
	highX, highY := max(int(baseX)+1, 0), max(int(baseY)+1, 0)
	top := lerp(t.loadColor(lowX, lowY), t.loadColor(highX, lowY), fx)
	bottom := lerp(t.loadColor(lowX, highY), t.loadColor(highX, highY), fx)
	return lerp(top, bottom, fy)
}

// loadColor is Texture2D<float4>.Load: zero outside the texture.
func (t *Texture) loadColor(x, y int) vec4 {
	if x < 0 || y < 0 || x >= t.width || y >= t.height {
		return vec4{}
	}
	return t.texel(x, y)
}

// blurFactor is gpu.BlurFactor as the pixel shader computes it.
func blurFactor(sigma float32) float32 { return float32(gpu.BlurFactor(sigma)) }

// backdropBlur is a port of backdropBlur in gpu/shaders/sdf.ps.hlsl.
func (f *fragment) backdropBlur() (vec4, float32) {
	_, coverage := f.rrect()
	factor := blurFactor(f.in.Blur * f.scale)
	px := (float32(f.pixelX) + 0.5) / factor
	py := (float32(f.pixelY) + 0.5) / factor
	return f.blurredTexels(px-0.5, py-0.5).times(f.in.Color.A * coverage), coverage
}

// filteredImage is a port of filteredImage in gpu/shaders/sdf.ps.hlsl.
func (f *fragment) filteredImage() (vec4, float32) {
	in := f.in
	tx := (f.logical.X - in.Rect.X) / in.Rect.W
	ty := (f.logical.Y - in.Rect.Y) / in.Rect.H
	coverage := float32(1)
	if f.turned() {
		tx, ty = saturate(tx), saturate(ty)
		p, halfSize := f.centered()
		var square [4]float32
		distance := roundedBoxDistance(p, halfSize, square)
		coverage = saturate(0.5 - distance/f.edgeStep(roundedBoxNormal(p, halfSize, square)))
	}
	if f.picture == nil {
		return vec4{}, coverage
	}
	u := in.GradientStart.X + tx*(in.GradientEnd.X-in.GradientStart.X)
	v := in.GradientStart.Y + ty*(in.GradientEnd.Y-in.GradientStart.Y)
	factor := blurFactor(in.Blur * f.scale)
	px := u * float32(f.picture.width) / factor
	py := v * float32(f.picture.height) / factor
	return f.blurredTexels(px-0.5, py-0.5).times(in.Color.A * coverage), coverage
}
