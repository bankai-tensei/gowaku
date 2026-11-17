//go:build darwin && arm64

package metal

import (
	"errors"
	"fmt"
	"log/slog"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

const (
	// framesInFlight is how many frames the CPU may record ahead of the GPU.
	framesInFlight = 2

	// defaultSamples is the MSAA level of a frame that needs it. See
	// docs/ADR/0017-multisample-on-demand.md.
	defaultSamples = 4

	// quadVertices is the length of the triangle strip of one instance.
	quadVertices = 4

	// initialArena is the first size of the buffers of a frame.
	initialArena = 1 << 16

	// arenaAlign keeps every reservation where Metal accepts a buffer offset,
	// for vertices and for copies to textures alike.
	arenaAlign = 256

	instanceStride = int(unsafe.Sizeof(gpu.Instance{}))
	vertexStride   = int(unsafe.Sizeof(geom.Offset{}))

	// The buffer indices of the shaders.
	bufferFrame     = 0
	bufferInstances = 1
	bufferVertices  = 2
)

// Options configures New. The zero value waits for vertical blank.
type Options struct {
	// Samples is the MSAA level, which smooths paths and shaped clips. Zero
	// selects 4, and 1 turns it off.
	Samples int
	// PresentImmediately stops EndFrame from waiting for vertical blank. It
	// exists to measure how fast frames can be drawn.
	PresentImmediately bool
}

// frameConstants mirrors struct Frame in sdf.metal.
type frameConstants struct {
	viewportWidth, viewportHeight float32
	scale                         float32
	_                             float32 // pads Frame to 16 bytes
}

// arena is a buffer of shared memory that a frame fills from the start.
type arena struct {
	buffer   uintptr
	contents unsafe.Pointer
	size     int
	used     int
}

// frame is what one frame in flight owns: its committed command buffer, which
// the CPU waits for before it writes the arenas again.
type frame struct {
	commands  uintptr
	instances arena
	vertices  arena
	staging   arena
}

// Device is the Metal implementation of gpu.Backend. It owns the single device
// of the process; every Surface shares it.
//
// The structs that the hot path hands to Metal by pointer are fields, so that
// they stay put and nothing allocates.
type Device struct {
	libs *libs
	sel  selectors
	args [maxArgs]uintptr

	device  objc.ID
	queue   objc.ID
	name    string
	samples int
	// immediate turns off the vertical blank of new surfaces.
	immediate bool

	library     objc.ID
	sdfVertex   objc.ID
	sdfFragment objc.ID
	pathVertex  objc.ID
	pipelines   [variantCount]pipelineSet
	depthStates [pipelineCount]uintptr
	noDepth     uintptr

	frames  [framesInFlight]frame
	current int
	// recording means that commands is the command buffer of frames[current]
	// and accepts work.
	recording bool
	commands  uintptr
	// blit and render are the open encoder, at most one of them.
	blit, render uintptr

	// drawing is the surface between BeginFrame and EndFrame, and target the
	// stand-in surface of DrawToTarget.
	drawing *Surface
	target  Surface

	// blank stands for a texture that a draw call does not name.
	blank *Texture
	// backdrop is the copy of the target under blend modes that read it, and
	// blurA and blurB the scratch targets of blurs, all made when first needed.
	backdrop     *Texture
	blurA, blurB *Texture
	blurs        []blurPlan
	// zeros is a buffer of zero bytes that clears regions of textures.
	zeros     uintptr
	zerosSize int

	// What is bound to the render encoder, because a batch switches often.
	boundPipeline uintptr
	boundDepth    uintptr
	boundLevel    int
	textures      [5]uintptr
	pass          *pipelineSet
	// buffers is where the batch in progress went.
	buffers drawBuffers
	// targetDesc is the render pass into textures of CreateTarget, and
	// targetColor its attachment.
	targetDesc, targetColor uintptr
	// multi is where batches that need multisampling draw for a target.
	multi multiTarget

	constants   frameConstants
	viewport    MTLViewport
	surfaceW    float64
	surfaceH    float64
	scissor     MTLScissorRect
	lastScissor MTLScissorRect
	origin      MTLOrigin
	toOrigin    MTLOrigin
	size        MTLSize

	liveSurfaces int

	// shaders holds the custom shaders that PrepareShader took.
	shaders map[*gpu.Shader]*customShader
}

var _ gpu.Backend = (*Device)(nil)

// New creates the device. It returns gpu.ErrNoGPU if the system has no Metal.
func New(opts Options) (*Device, error) {
	l, err := loadLibs()
	if err != nil {
		return nil, err
	}
	d := &Device{libs: l, samples: opts.Samples, immediate: opts.PresentImmediately}
	if d.samples == 0 {
		d.samples = defaultSamples
	}
	d.sel.register()
	token := d.pushPool()
	err = d.init()
	d.popPool(token)
	if err != nil {
		d.release()
		return nil, err
	}
	slog.Debug("metal: device created", "adapter", d.name, "samples", d.samples)
	return d, nil
}

func (d *Device) init() error {
	r, _, _ := purego.SyscallN(d.libs.createSystemDefaultMT)
	d.device = objc.ID(r)
	if d.device == 0 {
		return fmt.Errorf("metal: no Metal device: %w", gpu.ErrNoGPU)
	}
	d.name = goString(d.device.Send(objc.RegisterName("name")))
	if d.samples > 1 && !objc.Send[bool](d.device, objc.RegisterName("supportsTextureSampleCount:"), uint64(d.samples)) {
		d.samples = 1
	}
	d.queue = d.device.Send(objc.RegisterName("newCommandQueue"))
	if d.queue == 0 {
		return errors.New("metal: create command queue")
	}
	if err := d.compile(); err != nil {
		return err
	}
	d.createDepthStates()
	if err := d.createPipelines(); err != nil {
		return err
	}
	for i := range d.frames {
		f := &d.frames[i]
		for _, a := range []*arena{&f.instances, &f.vertices, &f.staging} {
			if err := d.grow(a, initialArena); err != nil {
				return err
			}
		}
	}
	var err error
	d.blank, err = d.newTexture(1, 1, gpu.TextureR8, MTLPixelFormatR8Unorm, MTLTextureUsageShaderRead, 1)
	return err
}

// compile builds the library of sdf.metal, without fast math, so that the
// shaders compute what those of the other backends do.
func (d *Device) compile() error {
	source := nsString(sdfSource)
	defer source.Send(objc.RegisterName("release"))
	options := newCompileOptions()
	defer options.Send(objc.RegisterName("release"))
	var failure objc.ID
	d.library = d.device.Send(objc.RegisterName("newLibraryWithSource:options:error:"), source, options, unsafe.Pointer(&failure))
	if d.library == 0 {
		return fmt.Errorf("metal: compile shaders: %w", describe(failure))
	}
	for _, f := range []struct {
		name string
		fn   *objc.ID
	}{{"sdfVertex", &d.sdfVertex}, {"sdfFragment", &d.sdfFragment}, {"pathVertex", &d.pathVertex}} {
		name := nsString(f.name)
		*f.fn = d.library.Send(objc.RegisterName("newFunctionWithName:"), name)
		name.Send(objc.RegisterName("release"))
		if *f.fn == 0 {
			return fmt.Errorf("metal: shader %s is missing", f.name)
		}
	}
	return nil
}

// AdapterName returns the name of the GPU.
func (d *Device) AdapterName() string { return d.name }

// grow replaces the buffer of a with one of at least n bytes. What the
// command buffers have bound of the old one, they keep alive themselves.
func (d *Device) grow(a *arena, n int) error {
	size := max(a.size*2, n, initialArena)
	buffer := uintptr(d.device.Send(objc.RegisterName("newBufferWithLength:options:"), uint64(size),
		uint64(MTLResourceStorageModeShared|MTLResourceCPUCacheModeWriteCombined)))
	if buffer == 0 {
		return fmt.Errorf("metal: allocate a buffer of %d bytes", size)
	}
	if a.buffer != 0 {
		d.send(a.buffer, d.sel.release)
	}
	a.buffer, a.size = buffer, size
	// contents is the CPU address of shared storage, valid while the buffer lives.
	a.contents = cPointer(d.send(buffer, d.sel.contents))
	return nil
}

// reserve returns room for n bytes in a: its offset, and where to write.
func (d *Device) reserve(a *arena, n int) (offset int, at unsafe.Pointer, err error) {
	start := (a.used + arenaAlign - 1) &^ (arenaAlign - 1)
	if start+n > a.size {
		if err := d.grow(a, n); err != nil {
			return 0, nil, err
		}
		start = 0
	}
	a.used = start + n
	return start, unsafe.Add(a.contents, start), nil
}

// prepare makes the current frame accept commands. Uploads call it as well as
// BeginFrame, so that they go in the same command buffer ahead of the frame.
func (d *Device) prepare() error {
	if d.recording {
		return nil
	}
	f := &d.frames[d.current]
	if err := d.waitFor(f); err != nil {
		return err
	}
	f.instances.used, f.vertices.used, f.staging.used = 0, 0, 0
	d.commands = d.send(uintptr(d.queue), d.sel.commandBuffer)
	if d.commands == 0 {
		return errors.New("metal: create command buffer")
	}
	d.send(d.commands, d.sel.retain)
	d.recording = true
	return nil
}

// waitFor blocks until the GPU has finished the commands of f.
func (d *Device) waitFor(f *frame) error {
	if f.commands == 0 {
		return nil
	}
	d.send(f.commands, d.sel.waitUntilCompleted)
	status := d.send(f.commands, d.sel.status)
	d.send(f.commands, d.sel.release)
	f.commands = 0
	if status == MTLCommandBufferStatusError {
		panic("gowaku: GPU command buffer failed: the device may be lost")
	}
	return nil
}

// blitEncoder returns the open blit encoder, ending a render encoder first.
func (d *Device) blitEncoder() uintptr {
	if d.blit != 0 {
		return d.blit
	}
	if d.render != 0 {
		panic("gowaku: metal: blit with a render encoder open")
	}
	d.blit = d.send(d.commands, d.sel.blitCommandEncoder)
	d.send(d.blit, d.sel.retain)
	return d.blit
}

func (d *Device) endBlit() {
	if d.blit == 0 {
		return
	}
	d.send(d.blit, d.sel.endEncoding)
	d.send(d.blit, d.sel.release)
	d.blit = 0
}

// beginRender opens a render encoder for the pass that desc describes, of
// width by height pixels, drawing with the pipelines of pass. The textures of
// d.textures are bound again, blank where there is none, because an encoder
// starts without bindings.
func (d *Device) beginRender(desc uintptr, pass *pipelineSet, width, height int) {
	d.endBlit()
	d.render = d.send(d.commands, d.sel.renderCommandEncoderWithDescriptor, desc)
	d.send(d.render, d.sel.retain)
	d.pass = pass
	d.boundPipeline, d.boundDepth, d.boundLevel = 0, 0, -1
	d.lastScissor = MTLScissorRect{}
	d.surfaceW, d.surfaceH = float64(width), float64(height)
	d.send(d.render, d.sel.setVertexBytes, ptr(&d.constants), unsafe.Sizeof(d.constants), bufferFrame)
	d.send(d.render, d.sel.setFragmentBytes, ptr(&d.constants), unsafe.Sizeof(d.constants), bufferFrame)
	blank := uintptr(d.blank.texture)
	for i, t := range d.textures {
		if t == 0 {
			t = blank
		}
		d.send(d.render, d.sel.setFragmentTexture, t, uintptr(i))
	}
}

// endRender ends the open render encoder. last says that nothing resumes the
// pass, so that the multisampled targets and the depth need not be kept.
func (d *Device) endRender(last bool) {
	if d.render == 0 {
		return
	}
	if d.pass.depth {
		depth := uintptr(MTLStoreActionStore)
		if last {
			depth = MTLStoreActionDontCare
		}
		d.send(d.render, d.sel.setDepthStoreAction, depth)
		d.send(d.render, d.sel.setStencilStoreAction, depth)
	}
	color := uintptr(MTLStoreActionStore)
	if d.pass.samples > 1 {
		color = MTLStoreActionStoreAndMultisampleResolve
		if last {
			color = MTLStoreActionMultisampleResolve
		}
	}
	d.send(d.render, d.sel.setColorStoreAction, color, 0)
	d.send(d.render, d.sel.endEncoding)
	d.send(d.render, d.sel.release)
	d.render = 0
}

func (d *Device) bindTexture(slot int, texture uintptr) {
	if d.textures[slot] == texture {
		return
	}
	d.textures[slot] = texture
	d.send(d.render, d.sel.setFragmentTexture, texture, uintptr(slot))
}

// BeginFrame implements gpu.Backend.
func (d *Device) BeginFrame(s gpu.Surface, background geom.Color) error {
	sf := s.(*Surface)
	if sf.inFrame || d.drawing != nil {
		panic("gowaku: BeginFrame called twice without EndFrame")
	}
	token := d.pushPool()
	defer d.popPool(token)
	if err := d.prepare(); err != nil {
		return err
	}
	if err := sf.acquire(); err != nil {
		return err
	}
	sf.inFrame = true
	d.drawing = sf
	sf.setClear(background)
	d.textures = [5]uintptr{}
	d.constants = frameConstants{viewportWidth: float32(sf.width), viewportHeight: float32(sf.height), scale: sf.scale}
	sf.begin(false)
	return nil
}

// Draw implements gpu.Backend.
func (d *Device) Draw(s gpu.Surface, batch *gpu.Batch) error {
	sf := s.(*Surface)
	if !sf.inFrame || d.render == 0 {
		panic("gowaku: Draw called outside BeginFrame and EndFrame")
	}
	token := d.pushPool()
	defer d.popPool(token)
	return d.drawBatch(sf, batch)
}

// drawBatch records batch into the render encoder in progress.
func (d *Device) drawBatch(sf *Surface, batch *gpu.Batch) error {
	n := len(batch.Instances)
	if n == 0 {
		return nil
	}
	f := &d.frames[d.current]
	// Each blur adds the instances of its passes after those of the batch.
	blurs := batch.CountBlurs()
	total := n + 3*blurs
	offset, at, err := d.reserve(&f.instances, total*instanceStride)
	if err != nil {
		return err
	}
	// at is shared memory with room for total instances.
	instances := unsafe.Slice((*gpu.Instance)(at), total)
	copy(instances, batch.Instances)
	if blurs > 0 {
		d.planBlurs(sf, batch, instances[n:])
	}
	vertexOffset := 0
	if v := len(batch.Vertices); v > 0 {
		var at unsafe.Pointer
		if vertexOffset, at, err = d.reserve(&f.vertices, v*vertexStride); err != nil {
			return err
		}
		// at is shared memory with room for v vertices.
		copy(unsafe.Slice((*geom.Offset)(at), v), batch.Vertices)
	}
	d.buffers = drawBuffers{instances: f.instances.buffer, offset: offset, vertices: f.vertices.buffer, vertexOffset: vertexOffset}
	b := &d.buffers
	d.bindBuffers(b)
	if t, ok := batch.Ramps.(*Texture); ok {
		d.bindTexture(slotRamps, uintptr(t.texture))
	}
	blur := 0
	for i := range batch.Draws {
		c := &batch.Draws[i]
		if c.Count == 0 {
			continue
		}
		var shader *customShader
		if c.Shader != nil {
			if shader = d.shaders[c.Shader]; shader == nil {
				// Not prepared, or refused: render reports it.
				continue
			}
		}
		if c.Blur > 0 {
			drawn, err := d.drawBlur(sf, c, &d.blurs[blur], n+3*blur)
			blur++
			if err != nil {
				return err
			}
			if !drawn {
				continue
			}
		}
		if c.Blend != gpu.BlendSrcOver {
			drawn, err := d.prepareBlend(sf, c)
			if err != nil {
				return err
			}
			if !drawn {
				continue
			}
		}
		if !d.setScissor(sf, c) {
			continue
		}
		d.bindTextures(c)
		if shader != nil {
			pipeline, err := d.shaderPipeline(shader, c.Blend)
			if err != nil {
				return err
			}
			d.use(pipeline, d.depthState(pipelineQuad), int(c.ClipLevel))
			d.quads(c)
			continue
		}
		if err := d.draw(c); err != nil {
			return err
		}
	}
	return nil
}

// drawBuffers is where the instances and vertices of a batch went, for binding
// them again after an interruption of the render encoder.
type drawBuffers struct {
	instances    uintptr
	offset       int
	vertices     uintptr
	vertexOffset int
	// first is the instance that the offset of the instance buffer points at.
	first int
}

func (d *Device) bindBuffers(b *drawBuffers) {
	d.send(d.render, d.sel.setVertexBuffer, b.instances, uintptr(b.offset), bufferInstances)
	d.send(d.render, d.sel.setVertexBuffer, b.vertices, uintptr(b.vertexOffset), bufferVertices)
	b.first = 0
}

// setScissor limits drawing to the clip of c, and reports false if nothing of
// c can show.
func (d *Device) setScissor(sf *Surface, c *gpu.DrawCall) bool {
	width, height := int32(sf.width), int32(sf.height)
	left, top, right, bottom := int32(0), int32(0), width, height
	if c.Clipped {
		left, top, right, bottom = gpu.PhysicalBounds(c.Clip, sf.scale)
		left, top = min(max(left, 0), width), min(max(top, 0), height)
		right, bottom = min(max(right, left), width), min(max(bottom, top), height)
	}
	if right <= left || bottom <= top {
		return false
	}
	d.scissor = MTLScissorRect{uint64(left), uint64(top), uint64(right - left), uint64(bottom - top)}
	if d.scissor != d.lastScissor {
		d.lastScissor = d.scissor
		d.send(d.render, d.sel.setScissorRect, ptr(&d.scissor))
	}
	return true
}

// bindTextures binds the glyph atlas and the image of c.
func (d *Device) bindTextures(c *gpu.DrawCall) {
	if t, ok := c.Texture.(*Texture); ok {
		d.bindTexture(0, uintptr(t.texture))
	}
	if t, ok := c.Image.(*Texture); ok {
		d.bindTexture(1, uintptr(t.texture))
	}
}

// draw runs one call. See docs/ADR/0009-paths-and-clips.md for the steps.
func (d *Device) draw(c *gpu.DrawCall) error {
	level := int(c.ClipLevel)
	switch c.Kind {
	case gpu.DrawQuads:
		quad, err := d.quadPipeline(c.Blend)
		if err != nil {
			return err
		}
		d.use(quad, d.depthState(pipelineQuad), level)
		d.quads(c)
	case gpu.DrawFill:
		quad, err := d.quadPipeline(c.Blend)
		if err != nil {
			return err
		}
		d.fan(c, level)
		d.use(quad, d.depthState(pipelineCover), level)
		d.quads(c)
	case gpu.DrawClipPush:
		d.fan(c, level)
		d.use(d.pass.noColor, d.depthState(pipelineClipCover), level+1)
		d.quads(c)
	case gpu.DrawClipPop:
		d.use(d.pass.noColor, d.depthState(pipelineClipPop), level)
		d.quads(c)
	}
	return nil
}

// quads draws the instances of c. The instance buffer moves to the first of
// them, because the instance ID of the shader counts from zero.
func (d *Device) quads(c *gpu.DrawCall) {
	b := &d.buffers
	if c.First != b.first {
		b.first = c.First
		d.send(d.render, d.sel.setVertexBufferOffset, uintptr(b.offset+c.First*instanceStride), bufferInstances)
	}
	d.send(d.render, d.sel.drawPrimitivesInstanced, MTLPrimitiveTypeTriangleStrip, 0, quadVertices, uintptr(c.Count))
}

// fan marks the shape of c in the stencil buffer.
func (d *Device) fan(c *gpu.DrawCall, level int) {
	kind := pipelineFanNonZero
	if c.EvenOdd {
		kind = pipelineFanEvenOdd
	}
	d.use(d.pass.fan, d.depthState(kind), level)
	d.send(d.render, d.sel.drawPrimitives, MTLPrimitiveTypeTriangle, uintptr(c.VertexFirst), uintptr(c.VertexCount))
}

// use binds what differs from the last call. Everything is drawn at z = 0, so
// a viewport without depth range puts it at the depth of the clip level.
func (d *Device) use(pipeline, depth uintptr, level int) {
	if pipeline != d.boundPipeline {
		d.boundPipeline = pipeline
		d.send(d.render, d.sel.setRenderPipelineState, pipeline)
	}
	if depth != d.boundDepth {
		d.boundDepth = depth
		d.send(d.render, d.sel.setDepthStencilState, depth)
	}
	if level != d.boundLevel {
		d.boundLevel = level
		z := float64(level) / clipDepthLevels
		d.viewport = MTLViewport{width: d.surfaceW, height: d.surfaceH, znear: z, zfar: z}
		d.send(d.render, d.sel.setViewport, ptr(&d.viewport))
	}
}

// EndFrame implements gpu.Backend.
func (d *Device) EndFrame(s gpu.Surface) error {
	sf := s.(*Surface)
	if !sf.inFrame {
		panic("gowaku: EndFrame called without BeginFrame")
	}
	token := d.pushPool()
	defer d.popPool(token)
	if d.recording {
		d.endRender(true)
		d.send(d.commands, d.sel.presentDrawable, sf.drawable)
		d.submit()
	} else {
		// ReadPixels has sent the commands already.
		d.send(sf.drawable, d.sel.present)
	}
	sf.release()
	sf.inFrame = false
	d.drawing = nil
	return nil
}

// submit commits the command buffer of the current frame.
func (d *Device) submit() {
	if !d.recording {
		return
	}
	d.endRender(true)
	d.endBlit()
	d.send(d.commands, d.sel.commit)
	d.frames[d.current].commands = d.commands
	d.commands = 0
	d.recording = false
	d.current = (d.current + 1) % framesInFlight
}

// Destroy implements gpu.Backend.
func (d *Device) Destroy() {
	if d.liveSurfaces != 0 {
		panic("gowaku: Device.Destroy called while surfaces are alive")
	}
	d.release()
}

// release frees everything. It also serves a partly initialized Device.
func (d *Device) release() {
	if d.device == 0 {
		*d = Device{}
		return
	}
	token := d.pushPool()
	if d.recording {
		d.submit()
	}
	for i := range d.frames {
		f := &d.frames[i]
		_ = d.waitFor(f) //nolint:errcheck // waitFor fails only by panicking
		for _, a := range []*arena{&f.instances, &f.vertices, &f.staging} {
			if a.buffer != 0 {
				d.send(a.buffer, d.sel.release)
			}
		}
	}
	for _, t := range []*Texture{d.blank, d.backdrop, d.blurA, d.blurB} {
		if t != nil {
			t.Destroy()
		}
	}
	d.multi.release(d)
	for _, id := range []uintptr{d.zeros, d.targetDesc} {
		if id != 0 {
			d.send(id, d.sel.release)
		}
	}
	for _, sh := range d.shaders {
		sh.release(d)
	}
	d.releasePipelines()
	for _, id := range []objc.ID{d.sdfVertex, d.sdfFragment, d.pathVertex, d.library, d.queue, d.device} {
		if id != 0 {
			d.send(uintptr(id), d.sel.release)
		}
	}
	d.popPool(token)
	*d = Device{}
}
