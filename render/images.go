package render

import (
	"image"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
)

// imageBudget is how many bytes of images stay on the GPU. Past it, images the
// current frame does not use leave, and come back when drawn again.
const imageBudget = 256 << 20

type imageEntry struct {
	texture  gpu.Texture
	bytes    int
	lastUsed uint64
}

// imageTexture returns the GPU copy of img, uploading it on first use. A failure
// is reported by Finish, and the image is left out of the frame.
func (g *Resources) imageTexture(img *paint.Image) gpu.Texture {
	if e, ok := g.images[img]; ok {
		e.lastUsed = g.frame
		return e.texture
	}
	pix, width, height := img.Pixels()
	// Every image gets its mip levels, whatever quality draws it first, so
	// that a texture never has to be replaced while a batch refers to it.
	t, err := g.store.CreateTexture(width, height, gpu.TextureRGBA8Mipmapped)
	if err == nil {
		err = g.store.UpdateTexture(t, image.Rect(0, 0, width, height), pix, width*4)
	}
	if err != nil {
		g.err = err
		return nil
	}
	// The levels below add a third.
	bytes := len(pix) + len(pix)/3
	g.images[img] = &imageEntry{texture: t, bytes: bytes, lastUsed: g.frame}
	g.imageBytes += bytes
	return t
}

// imageFilter returns the gpu.ImageFilter of q. The default, and values it does
// not know, are FilterMipmap.
func imageFilter(q paint.FilterQuality) gpu.ImageFilter {
	switch q {
	case paint.FilterNearest:
		return gpu.FilterNearest
	case paint.FilterLinear:
		return gpu.FilterLinear
	}
	return gpu.FilterMipmap
}

func (g *Resources) encodeImage(op *paint.Op, b *gpu.Batch) {
	t := g.imageTexture(op.Image)
	if t == nil {
		return
	}
	addDraw(b, op, nil, t)
	b.Instances = append(b.Instances, gpu.Instance{
		Rect:        op.RRect.Rect,
		Color:       op.Color,
		GradientEnd: geom.Offset{X: 1, Y: 1},
		Kind:        gpu.InstanceImage,
	})
	in := &b.Instances[len(b.Instances)-1]
	in.SetImageFilter(imageFilter(op.Filter))
	if op.Transform != (geom.Affine{}) {
		in.SetTransform(op.Transform)
	}
}

// evictImages drops images until the budget holds. Only an image of an earlier
// frame can go, so the batch in hand stays valid.
func (g *Resources) evictImages() {
	for g.imageBytes > imageBudget {
		var oldest *paint.Image
		for img, e := range g.images {
			if e.lastUsed != g.frame && (oldest == nil || e.lastUsed < g.images[oldest].lastUsed) {
				oldest = img
			}
		}
		if oldest == nil {
			return
		}
		e := g.images[oldest]
		e.texture.Destroy()
		g.imageBytes -= e.bytes
		delete(g.images, oldest)
	}
}
