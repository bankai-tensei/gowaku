package render

import (
	"image"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
)

// TextureStore is the part of gpu.Backend that resources need.
type TextureStore interface {
	CreateTexture(width, height int, format gpu.TextureFormat) (gpu.Texture, error)
	UpdateTexture(t gpu.Texture, region image.Rectangle, pix []byte, stride int) error
	CreateTarget(width, height int) (gpu.Texture, error)
	DrawToTarget(t gpu.Texture, batch *gpu.Batch, scale float32) error
	PrepareShader(s *gpu.Shader) error
}

// Resources keeps what batches refer to on the GPU: the pages of the atlas, the
// images and the gradient ramps. One serves every window of the process.
type Resources struct {
	atlas    *text.Atlas
	store    TextureStore
	textures []gpu.Texture
	// colorTextures hold the color pages of the atlas.
	colorTextures []gpu.Texture

	images     map[*paint.Image]*imageEntry
	imageBytes int
	frame      uint64
	err        error

	// layers are those the frame draws, and shaderLayers those that custom
	// shaders read.
	layers       layerPool
	shaderLayers layerPool
	// layerScale is the scale of the frame in progress.
	layerScale float32

	// ramps holds the gradients of more than gpu.MaxStops stops. It is nil
	// until the first one.
	ramps *rampTable

	// shaders holds the custom shaders drawn so far, and unprepared those
	// that Finish has yet to hand to the backend.
	shaders    map[*paint.Shader]*shaderEntry
	unprepared []*shaderEntry
	// drawErrs are what DrawError reports.
	drawErrs []error
	// shaderDepth counts the layers of shaderLayers being encoded.
	shaderDepth int
	// scratches are batches for layers drawn plain in place of a shader, and
	// scratchUsed how many of them are in use.
	scratches   []*gpu.Batch
	scratchUsed int

	// The groups of the frame: the targets that their pictures borrow, those
	// taken this frame in order, and the jobs that draw to them. See
	// groups.go.
	groupTextures []*groupTexture
	groupTaken    []*groupTexture
	jobs          []groupJob
	jobBatches    []*gpu.Batch
	// groupDepth counts the groups being encoded, and rasterizing the layers
	// being put on pages.
	groupDepth  int
	rasterizing int
	// The cached groups (groupcache.go): their pictures by key and in a list,
	// the bytes those hold, and pictures to give back to the pool at the next
	// frame.
	cachedGroups    map[*paint.GroupCache]*cachedGroup
	cachedList      []*cachedGroup
	groupCacheBytes int
	retired         []*groupTexture
	// filterResults and filterLast are scratch for the filter in progress,
	// and transferTable for a table of feComponentTransfer.
	filterResults []*groupTexture
	filterLast    []int
	transferTable [gpu.RampWidth * 4]byte
}

// NewResources returns Resources that draw glyphs from atlas.
func NewResources(atlas *text.Atlas, store TextureStore) *Resources {
	return &Resources{
		atlas:        atlas,
		store:        store,
		images:       make(map[*paint.Image]*imageEntry),
		layers:       newLayerPool(layerPageLimit),
		shaderLayers: newLayerPool(shaderLayerPageLimit),
		shaders:      make(map[*paint.Shader]*shaderEntry),
	}
}

// Finish sends what the atlas rasterized to the GPU, and completes b, which
// Encode filled, for drawing.
func (g *Resources) Finish(b *gpu.Batch) error {
	if g.err != nil {
		err := g.err
		g.err = nil
		return err
	}
	if err := g.upload(); err != nil {
		return err
	}
	g.prepareShaders(b)
	g.evictImages()
	g.resolvePages(b)
	if err := g.drawGroups(); err != nil {
		return err
	}
	return g.drawLayers(g.layerScale)
}

func (g *Resources) upload() error {
	if g.ramps != nil {
		if err := g.ramps.upload(g.store); err != nil {
			return err
		}
	}
	for i := range g.atlas.ColorPageCount() {
		if i == len(g.colorTextures) {
			size := g.atlas.PageSize()
			t, err := g.store.CreateTexture(size, size, gpu.TextureRGBA8)
			if err != nil {
				return err
			}
			g.colorTextures = append(g.colorTextures, t)
		}
		if r, pix := g.atlas.TakeColorDirty(i); !r.Empty() {
			if err := g.store.UpdateTexture(g.colorTextures[i], r, pix, g.atlas.PageSize()*4); err != nil {
				return err
			}
		}
	}
	for i := range g.atlas.PageCount() {
		if i == len(g.textures) {
			size := g.atlas.PageSize()
			t, err := g.store.CreateTexture(size, size, gpu.TextureR8)
			if err != nil {
				return err
			}
			g.textures = append(g.textures, t)
		}
		if r, pix := g.atlas.TakeDirty(i); !r.Empty() {
			if err := g.store.UpdateTexture(g.textures[i], r, pix, g.atlas.PageSize()); err != nil {
				return err
			}
		}
	}
	return nil
}

// Destroy releases the textures. Call it before destroying the backend.
func (g *Resources) Destroy() {
	for _, t := range g.textures {
		t.Destroy()
	}
	g.textures = nil
	for _, t := range g.colorTextures {
		t.Destroy()
	}
	g.colorTextures = nil
	for _, e := range g.images {
		e.texture.Destroy()
	}
	clear(g.images)
	g.layers.destroy()
	g.shaderLayers.destroy()
	g.destroyGroups()
	g.destroyGroupCache()
	if g.ramps != nil {
		g.ramps.destroy()
		g.ramps = nil
	}
}

// rasterSize returns the size to rasterize a font of size physical pixels at
// under zoom. A scale transition would otherwise fill the atlas with a new size
// per frame, so a zoomed size lands on whole pixels. The glyph positions stay
// exact, and the difference lasts as long as the transition.
func rasterSize(size, zoom float32) float32 {
	if zoom == 1 {
		return size
	}
	return max(geom.Floor32(size*zoom+0.5), 1)
}

// pageTexture is a stand-in for the texture of an atlas page. Encode cannot use
// the real one, which Finish creates only after it.
type pageTexture int

// Destroy implements gpu.Texture.
func (pageTexture) Destroy() {}

// colorPageTexture stands in for the texture of a color page.
type colorPageTexture int

// Destroy implements gpu.Texture.
func (colorPageTexture) Destroy() {}

// encodeRun appends the instances of a glyph run to b. Glyphs land on whole
// physical pixels vertically, and on quarters of one horizontally.
func (g *Resources) encodeRun(op *paint.Op, scale float32, b *gpu.Batch) {
	run := op.Glyphs
	zoom := op.Zoom()
	size := rasterSize(run.Size*scale, zoom)
	for i := range run.Glyphs {
		gl := &run.Glyphs[i]
		// The float32 conversions keep arm64 from fusing these into FMA, so
		// every architecture picks the same pixel and subpixel step; see
		// ADR 0086. x - pixel is the one the goldens caught.
		x := float32((op.Origin.X + float32(gl.X*zoom)) * scale)
		pixel := geom.Floor32(x)
		step := uint8((x - pixel) * text.SubpixelSteps)
		y := geom.Floor32(float32((op.Origin.Y+float32(gl.Y*zoom))*scale) + 0.5)
		img := g.atlas.Glyph(text.GlyphKey{Face: run.Face, ID: gl.ID, Size: size, Subpixel: step, Sideways: run.Sideways})
		if img.Width == 0 {
			continue
		}
		if img.Color {
			g.encodeColorGlyph(op, img, pixel, y, scale, b)
			continue
		}
		addDraw(b, op, pageTexture(img.Page), nil)
		b.Instances = append(b.Instances, gpu.Instance{
			Rect: geom.Rect{
				X: (pixel + float32(img.Left)) / scale,
				Y: (y + float32(img.Top)) / scale,
				W: float32(img.Width) / scale,
				H: float32(img.Height) / scale,
			},
			Radii: [4]float32{float32(img.X), float32(img.Y)},
			Color: op.Color,
			Kind:  gpu.InstanceGlyph,
		})
		if op.Transform != (geom.Affine{}) {
			b.Instances[len(b.Instances)-1].SetTransform(op.Transform)
		}
	}
}

// encodeColorGlyph draws a color glyph, such as an emoji, as an image of its
// page. It keeps its own colors, and takes only the opacity of the text.
func (g *Resources) encodeColorGlyph(op *paint.Op, img text.GlyphImage, x, y, scale float32, b *gpu.Batch) {
	addDraw(b, op, nil, colorPageTexture(img.Page))
	size := float32(g.atlas.PageSize())
	b.Instances = append(b.Instances, gpu.Instance{
		Rect: geom.Rect{
			X: (x + float32(img.Left)) / scale,
			Y: (y + float32(img.Top)) / scale,
			W: float32(img.Width) / scale,
			H: float32(img.Height) / scale,
		},
		Color:         geom.Color{R: 1, G: 1, B: 1, A: op.Color.A},
		GradientStart: geom.Offset{X: float32(img.X) / size, Y: float32(img.Y) / size},
		GradientEnd:   geom.Offset{X: float32(img.X+img.Width) / size, Y: float32(img.Y+img.Height) / size},
		Kind:          gpu.InstanceImage,
	})
	if op.Transform != (geom.Affine{}) {
		b.Instances[len(b.Instances)-1].SetTransform(op.Transform)
	}
}
