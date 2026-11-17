package soft

import (
	"errors"
	"image"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

// filterSteps is the precision of the weights of bilinear filtering: Direct3D asks
// for at least 8 fractional bits, and hardware stores no more than that.
const filterSteps = 256

// Texture is an image in memory.
type Texture struct {
	width, height int
	format        gpu.TextureFormat
	// target says that DrawToTarget can draw to it.
	target bool
	pix    []uint8
	// mips holds the levels below level 0 of a TextureRGBA8Mipmapped, as
	// gpu.BuildMips makes them, and mipAt where each starts.
	mips  []uint8
	mipAt []int
}

var _ gpu.Texture = (*Texture)(nil)

// CreateTexture implements gpu.Backend.
func (d *Device) CreateTexture(width, height int, format gpu.TextureFormat) (gpu.Texture, error) {
	if format != gpu.TextureR8 && format != gpu.TextureRGBA8 && format != gpu.TextureRGBA8Mipmapped {
		return nil, errors.New("soft: unknown texture format")
	}
	if width <= 0 || height <= 0 {
		return nil, errors.New("soft: texture size is empty")
	}
	return &Texture{width: width, height: height, format: format, pix: make([]uint8, width*height*format.BytesPerPixel())}, nil
}

// UpdateTexture implements gpu.Backend.
func (d *Device) UpdateTexture(t gpu.Texture, region image.Rectangle, pix []byte, stride int) error {
	tx := t.(*Texture)
	size := tx.format.BytesPerPixel()
	if tx.format == gpu.TextureRGBA8Mipmapped && region != image.Rect(0, 0, tx.width, tx.height) {
		panic("gowaku: UpdateTexture of a mipmapped texture must cover all of it")
	}
	for y := region.Min.Y; y < region.Max.Y; y++ {
		src := pix[y*stride+region.Min.X*size : y*stride+region.Max.X*size]
		copy(tx.pix[(y*tx.width+region.Min.X)*size:], src)
	}
	if tx.format == gpu.TextureRGBA8Mipmapped {
		tx.mips = gpu.BuildMips(pix, stride, tx.width, tx.height)
		tx.mipAt = tx.mipAt[:0]
		at := 0
		for k := 1; k < gpu.MipLevels(tx.width, tx.height); k++ {
			tx.mipAt = append(tx.mipAt, at)
			w, h := gpu.MipSize(tx.width, tx.height, k)
			at += w * h * 4
		}
	}
	return nil
}

// levels returns how many mip levels the texture has.
func (t *Texture) levels() int { return len(t.mipAt) + 1 }

// levelSize returns the size of a mip level.
func (t *Texture) levelSize(level int) (width, height int) {
	return gpu.MipSize(t.width, t.height, level)
}

// levelTexel is Texture2D<float4>.Load of the texel at (x, y), inside the
// level.
func (t *Texture) levelTexel(level, x, y int) vec4 {
	if level == 0 {
		return t.texel(x, y)
	}
	w, _ := t.levelSize(level)
	p := t.mips[t.mipAt[level-1]+(y*w+x)*4:]
	return vec4{float32(p[0]) / 255, float32(p[1]) / 255, float32(p[2]) / 255, float32(p[3]) / 255}
}

// CreateTarget implements gpu.Backend.
func (d *Device) CreateTarget(width, height int) (gpu.Texture, error) {
	t, err := d.CreateTexture(width, height, gpu.TextureRGBA8)
	if err != nil {
		return nil, err
	}
	tx := t.(*Texture)
	tx.target = true
	return tx, nil
}

// DrawToTarget implements gpu.Backend. It draws like a frame without multisampling
// whose depth and stencil start at zero.
func (d *Device) DrawToTarget(t gpu.Texture, batch *gpu.Batch, scale float32) error {
	if d.openFrames != 0 {
		panic("gowaku: DrawToTarget called between BeginFrame and EndFrame")
	}
	tx := t.(*Texture)
	if !tx.target {
		panic("gowaku: DrawToTarget called on a texture that CreateTarget did not make")
	}
	for i := range batch.Draws {
		if batch.Draws[i].Image == t || batch.Draws[i].Texture == t {
			panic("gowaku: DrawToTarget called with a batch that reads its own target")
		}
	}
	if batch.Multisampled() {
		d.drawMultisampled(tx, batch, scale)
		return nil
	}
	c := newCanvasOn(tx.pix, tx.width, tx.height, 1, scale)
	for _, r := range batch.Clears {
		c.clear(r)
	}
	c.draw(batch)
	return nil
}

// drawMultisampled draws a batch that needs multisampling on a canvas of
// multisampleCount samples that starts transparent, and resolves the bounds of
// its clears into tx, as the GPU backends resolve their scratch target. See
// docs/ADR/0114-gpu-layers-masks-filters.md.
func (d *Device) drawMultisampled(tx *Texture, batch *gpu.Batch, scale float32) {
	for i := range batch.Draws {
		if batch.Draws[i].Blur > 0 || batch.Draws[i].Blend.ReadsBackdrop() {
			panic("gowaku: DrawToTarget called with a multisampled batch that blurs or reads the backdrop")
		}
	}
	r := batch.ClearBounds().Intersect(image.Rect(0, 0, tx.width, tx.height))
	if r.Empty() {
		panic("gowaku: DrawToTarget called with a multisampled batch without clears")
	}
	c := newCanvas(tx.width, tx.height, multisampleCount, scale)
	c.draw(batch)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			first := c.sampleIndex(x, y) * 4
			out := tx.pix[(y*tx.width+x)*4:]
			for k := range 4 {
				// Halves round down, as the resolve of frames.
				sum := (multisampleCount - 1) / 2
				for s := range multisampleCount {
					sum += int(c.color[first+s*4+k])
				}
				out[k] = uint8(sum / multisampleCount)
			}
		}
	}
}

// Destroy implements gpu.Texture.
func (t *Texture) Destroy() { *t = Texture{} }

// texel returns what a shader reads from a texel inside the texture: a missing
// green, blue and alpha read as 0, 0 and 1.
func (t *Texture) texel(x, y int) vec4 {
	if t.format == gpu.TextureR8 {
		return vec4{float32(t.pix[y*t.width+x]) / 255, 0, 0, 1}
	}
	p := t.pix[(y*t.width+x)*4:]
	return vec4{float32(p[0]) / 255, float32(p[1]) / 255, float32(p[2]) / 255, float32(p[3]) / 255}
}

// load is Texture2D<float>.Load: it reads the first channel of one texel, and zero
// outside the texture or if none is bound.
func (t *Texture) load(x, y int) float32 {
	if t == nil || x < 0 || y < 0 || x >= t.width || y >= t.height {
		return 0
	}
	return t.texel(x, y)[0]
}

// sample is Texture2D<float4>.Sample with a bilinear sampler that clamps. Without
// a texture bound it reads zero.
func (t *Texture) sample(u, v float32) vec4 {
	if t == nil || t.width == 0 {
		return vec4{}
	}
	x0, fx := filterPosition(u, t.width)
	y0, fy := filterPosition(v, t.height)
	x1, y1 := min(max(x0+1, 0), t.width-1), min(max(y0+1, 0), t.height-1)
	x0, y0 = min(max(x0, 0), t.width-1), min(max(y0, 0), t.height-1)
	top := lerp(t.texel(x0, y0), t.texel(x1, y0), fx)
	bottom := lerp(t.texel(x0, y1), t.texel(x1, y1), fx)
	return lerp(top, bottom, fy)
}

// filterPosition returns the texel at or before the normalized coordinate v, and
// how far it is on the way to the next texel.
func filterPosition(v float32, size int) (texel int, weight float32) {
	p := v*float32(size) - 0.5
	floor := geom.Floor32(p)
	return int(floor), geom.Floor32((p-floor)*filterSteps+0.5) / filterSteps
}

// PrepareShader implements gpu.Backend. The CPU runs Shader.Fallback, which
// needs nothing made.
func (d *Device) PrepareShader(*gpu.Shader) error { return nil }
