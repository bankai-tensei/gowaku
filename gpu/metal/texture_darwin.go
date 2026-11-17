//go:build darwin && arm64

package metal

import (
	"errors"
	"image"
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/gpu"
)

// Texture is the Metal implementation of gpu.Texture.
type Texture struct {
	device  *Device
	texture objc.ID
	format  gpu.TextureFormat
	// target says that CreateTarget made it, so that DrawToTarget may draw to it.
	target        bool
	width, height int
	levels        int
}

var _ gpu.Texture = (*Texture)(nil)

// CreateTexture implements gpu.Backend.
func (d *Device) CreateTexture(width, height int, format gpu.TextureFormat) (gpu.Texture, error) {
	if d.drawing != nil {
		panic("gowaku: CreateTexture called between BeginFrame and EndFrame")
	}
	var pixel uint64
	levels := 1
	switch format {
	case gpu.TextureR8:
		pixel = MTLPixelFormatR8Unorm
	case gpu.TextureRGBA8:
		pixel = MTLPixelFormatRGBA8Unorm
	case gpu.TextureRGBA8Mipmapped:
		pixel = MTLPixelFormatRGBA8Unorm
		levels = gpu.MipLevels(width, height)
	default:
		return nil, errors.New("metal: unknown texture format")
	}
	token := d.pushPool()
	defer d.popPool(token)
	t, err := d.newTexture(width, height, format, pixel, MTLTextureUsageShaderRead, levels)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// newTexture makes a texture in private memory and clears it, as the textures
// of the other backends start transparent.
func (d *Device) newTexture(width, height int, format gpu.TextureFormat, pixel, usage uint64, levels int) (*Texture, error) {
	if err := d.prepare(); err != nil {
		return nil, err
	}
	desc := objc.ID(objc.GetClass("MTLTextureDescriptor")).Send(objc.RegisterName("new"))
	defer desc.Send(objc.RegisterName("release"))
	desc.Send(objc.RegisterName("setTextureType:"), uint64(MTLTextureType2D))
	desc.Send(objc.RegisterName("setPixelFormat:"), pixel)
	desc.Send(objc.RegisterName("setWidth:"), uint64(width))
	desc.Send(objc.RegisterName("setHeight:"), uint64(height))
	desc.Send(objc.RegisterName("setMipmapLevelCount:"), uint64(levels))
	desc.Send(objc.RegisterName("setUsage:"), usage)
	desc.Send(objc.RegisterName("setStorageMode:"), uint64(MTLStorageModePrivate))
	texture := d.device.Send(objc.RegisterName("newTextureWithDescriptor:"), desc)
	if texture == 0 {
		return nil, errors.New("metal: create texture")
	}
	t := &Texture{device: d, texture: texture, format: format, width: width, height: height, levels: levels}
	bpp := format.BytesPerPixel()
	for level := range levels {
		if err := d.clearRegion(t, level, gpu.MipRegion(width, height, level), bpp); err != nil {
			t.Destroy()
			return nil, err
		}
	}
	return t, nil
}

// clearRegion makes a region of a level of t transparent, by copying zeros from
// a buffer, which Metal fills with zeros when it makes it.
func (d *Device) clearRegion(t *Texture, level int, r image.Rectangle, bpp int) error {
	row := r.Dx() * bpp
	n := row * r.Dy()
	if n == 0 {
		return nil
	}
	if n > d.zerosSize {
		size := max(n, 1<<20)
		zeros := uintptr(d.device.Send(objc.RegisterName("newBufferWithLength:options:"), uint64(size), uint64(MTLResourceStorageModeShared)))
		if zeros == 0 {
			return errors.New("metal: allocate a buffer of zeros")
		}
		if d.zeros != 0 {
			d.send(d.zeros, d.sel.release)
		}
		d.zeros, d.zerosSize = zeros, size
	}
	d.copyToTexture(d.zeros, 0, row, t, level, r)
	return nil
}

// copyToTexture records the copy of a region from a buffer whose rows are row
// bytes apart.
func (d *Device) copyToTexture(buffer uintptr, offset, row int, t *Texture, level int, r image.Rectangle) {
	blit := d.blitEncoder()
	d.size = MTLSize{uint64(r.Dx()), uint64(r.Dy()), 1}
	d.toOrigin = MTLOrigin{uint64(r.Min.X), uint64(r.Min.Y), 0}
	d.send(blit, d.sel.copyBufferToTexture, buffer, uintptr(offset), uintptr(row), uintptr(row*r.Dy()), ptr(&d.size),
		uintptr(t.texture), 0, uintptr(level), ptr(&d.toOrigin))
}

// UpdateTexture implements gpu.Backend.
func (d *Device) UpdateTexture(t gpu.Texture, region image.Rectangle, pix []byte, stride int) error {
	if region.Empty() {
		return nil
	}
	if d.drawing != nil {
		panic("gowaku: UpdateTexture called between BeginFrame and EndFrame")
	}
	tx := t.(*Texture)
	mipmapped := tx.format == gpu.TextureRGBA8Mipmapped
	if mipmapped && region != image.Rect(0, 0, tx.width, tx.height) {
		panic("gowaku: UpdateTexture of a mipmapped texture must cover all of it")
	}
	token := d.pushPool()
	defer d.popPool(token)
	if err := d.prepare(); err != nil {
		return err
	}
	if err := d.uploadLevel(tx, 0, region, pix, stride); err != nil {
		return err
	}
	if mipmapped {
		mips := gpu.BuildMips(pix, stride, tx.width, tx.height)
		at := 0
		for level := 1; level < tx.levels; level++ {
			r := gpu.MipRegion(tx.width, tx.height, level)
			if err := d.uploadLevel(tx, level, r, mips[at:], r.Dx()*4); err != nil {
				return err
			}
			at += r.Dx() * r.Dy() * 4
		}
	}
	return nil
}

// uploadLevel copies region of pix, with the given stride, to the same place of
// a level of tx, through the staging buffer of the frame.
func (d *Device) uploadLevel(tx *Texture, level int, region image.Rectangle, pix []byte, stride int) error {
	bpp := tx.format.BytesPerPixel()
	width, height := region.Dx(), region.Dy()
	row := width * bpp
	f := &d.frames[d.current]
	offset, at, err := d.reserve(&f.staging, row*height)
	if err != nil {
		return err
	}
	// at is shared memory with room for the whole region.
	dst := unsafe.Slice((*byte)(at), row*height)
	for y := range height {
		start := (region.Min.Y+y)*stride + region.Min.X*bpp
		copy(dst[y*row:(y+1)*row], pix[start:start+row])
	}
	d.copyToTexture(f.staging.buffer, offset, row, tx, level, region)
	return nil
}

// Destroy implements gpu.Texture. The command buffers that use the texture
// keep it alive until the GPU is done with it.
func (t *Texture) Destroy() {
	if t.texture == 0 {
		return
	}
	d := t.device
	for i, bound := range d.textures {
		if bound == uintptr(t.texture) {
			d.textures[i] = 0
		}
	}
	d.send(uintptr(t.texture), d.sel.release)
	t.texture = 0
}

// CreateTarget implements gpu.Backend.
func (d *Device) CreateTarget(width, height int) (gpu.Texture, error) {
	if d.drawing != nil {
		panic("gowaku: CreateTarget called between BeginFrame and EndFrame")
	}
	token := d.pushPool()
	defer d.popPool(token)
	t, err := d.newTarget(width, height)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (d *Device) newTarget(width, height int) (*Texture, error) {
	t, err := d.newTexture(width, height, gpu.TextureRGBA8, MTLPixelFormatBGRA8Unorm,
		MTLTextureUsageShaderRead|MTLTextureUsageRenderTarget, 1)
	if err != nil {
		return nil, err
	}
	t.target = true
	return t, nil
}

// DrawToTarget implements gpu.Backend.
func (d *Device) DrawToTarget(t gpu.Texture, batch *gpu.Batch, scale float32) error {
	if d.drawing != nil {
		panic("gowaku: DrawToTarget called between BeginFrame and EndFrame")
	}
	tx := t.(*Texture)
	if !tx.target {
		panic("gowaku: DrawToTarget called on a texture that CreateTarget did not make")
	}
	token := d.pushPool()
	defer d.popPool(token)
	if batch.Multisampled() {
		return d.drawMultisampled(tx, batch, scale)
	}
	if err := d.prepare(); err != nil {
		return err
	}
	for _, r := range batch.Clears {
		r = r.Intersect(image.Rect(0, 0, tx.width, tx.height))
		if err := d.clearRegion(tx, 0, r, 4); err != nil {
			return err
		}
	}
	sf := &d.target
	d.textures = [5]uintptr{}
	*sf = Surface{device: d, width: tx.width, height: tx.height, scale: scale, drawn: tx}
	d.constants = frameConstants{viewportWidth: float32(tx.width), viewportHeight: float32(tx.height), scale: scale}
	sf.begin(false)
	err := d.drawBatch(sf, batch)
	d.endRender(true)
	sf.releaseTarget()
	return err
}
