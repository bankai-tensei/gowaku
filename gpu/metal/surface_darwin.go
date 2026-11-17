//go:build darwin && arm64

package metal

import (
	"errors"
	"image"
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/platform"
)

// Surface is the CAMetalLayer of one window, and the stand-in for a texture of
// CreateTarget while DrawToTarget draws to it.
type Surface struct {
	device        *Device
	layer         objc.ID
	width, height int
	scale         float32
	multisample   bool

	// The targets of the pass besides the drawable, made when first needed at
	// the size of the drawable.
	msaa, depth, depthMSAA uintptr

	// drawable and its texture belong to the frame in progress.
	drawable, drawableTexture uintptr
	inFrame                   bool

	// pass is the descriptor of the render pass, and color, depthAttachment
	// and stencilAttachment its attachments.
	pass              uintptr
	color             uintptr
	depthAttachment   uintptr
	stencilAttachment uintptr
	clear             geom.Color
	clearSet          bool
	// variant is that of the frame in progress.
	variant int

	// drawn is the texture that the stand-in of DrawToTarget draws to.
	drawn *Texture
}

var _ gpu.Surface = (*Surface)(nil)

// CreateSurface implements gpu.Backend. h must be a platform.CocoaHandle.
func (d *Device) CreateSurface(h platform.NativeHandle, physicalWidth, physicalHeight int, scale float32) (gpu.Surface, error) {
	handle, ok := h.(platform.CocoaHandle)
	if !ok || handle.Layer == 0 {
		return nil, errors.New("metal: the window has no CAMetalLayer")
	}
	token := d.pushPool()
	defer d.popPool(token)
	s := &Surface{device: d, layer: objc.ID(handle.Layer), multisample: true}
	s.layer.Send(objc.RegisterName("retain"))
	s.layer.Send(objc.RegisterName("setDevice:"), d.device)
	s.layer.Send(objc.RegisterName("setPixelFormat:"), uint64(MTLPixelFormatBGRA8Unorm))
	// The backdrop of blend modes and ReadPixels copy from the drawable.
	s.layer.Send(objc.RegisterName("setFramebufferOnly:"), false)
	s.layer.Send(objc.RegisterName("setDisplaySyncEnabled:"), !d.immediate)
	s.pass = d.newPassDescriptor(true)
	s.color, s.depthAttachment, s.stencilAttachment = attachments(s.pass, true)
	d.liveSurfaces++
	if err := s.Resize(physicalWidth, physicalHeight, scale); err != nil {
		s.Destroy()
		return nil, err
	}
	return s, nil
}

// newPassDescriptor makes a render pass descriptor. Its depth clears to zero,
// the level outside every clip.
func (d *Device) newPassDescriptor(depth bool) uintptr {
	desc := objc.ID(objc.GetClass("MTLRenderPassDescriptor")).Send(objc.RegisterName("new"))
	if depth {
		desc.Send(objc.RegisterName("depthAttachment")).Send(objc.RegisterName("setClearDepth:"), 0.0)
		desc.Send(objc.RegisterName("stencilAttachment")).Send(objc.RegisterName("setClearStencil:"), uint32(0))
	}
	return uintptr(desc)
}

// attachments returns the attachments of a descriptor, which it keeps alive.
func attachments(desc uintptr, depth bool) (color, depthAttachment, stencil uintptr) {
	pass := objc.ID(desc)
	color = uintptr(pass.Send(objc.RegisterName("colorAttachments")).Send(objc.RegisterName("objectAtIndexedSubscript:"), uint64(0)))
	if depth {
		depthAttachment = uintptr(pass.Send(objc.RegisterName("depthAttachment")))
		stencil = uintptr(pass.Send(objc.RegisterName("stencilAttachment")))
	}
	return color, depthAttachment, stencil
}

// SetMultisample implements gpu.Surface.
func (s *Surface) SetMultisample(on bool) { s.multisample = on }

// Resize implements gpu.Surface.
func (s *Surface) Resize(physicalWidth, physicalHeight int, scale float32) error {
	if physicalWidth <= 0 || physicalHeight <= 0 {
		return nil
	}
	if s.inFrame {
		panic("gowaku: Surface.Resize called between BeginFrame and EndFrame")
	}
	d := s.device
	token := d.pushPool()
	defer d.popPool(token)
	s.scale = scale
	if physicalWidth == s.width && physicalHeight == s.height {
		return nil
	}
	s.width, s.height = physicalWidth, physicalHeight
	s.layer.Send(objc.RegisterName("setDrawableSize:"), CGSize{float64(physicalWidth), float64(physicalHeight)})
	s.releaseTargets()
	return nil
}

// releaseTargets drops the targets of the old size. Command buffers in flight
// keep what they use.
func (s *Surface) releaseTargets() {
	d := s.device
	for _, t := range []*uintptr{&s.msaa, &s.depth, &s.depthMSAA} {
		if *t != 0 {
			d.send(*t, d.sel.release)
			*t = 0
		}
	}
}

// renderTexture makes a texture that only the passes of the surface use.
func (s *Surface) renderTexture(pixel uint64, samples int) (uintptr, error) {
	desc := objc.ID(objc.GetClass("MTLTextureDescriptor")).Send(objc.RegisterName("new"))
	defer desc.Send(objc.RegisterName("release"))
	if samples > 1 {
		desc.Send(objc.RegisterName("setTextureType:"), uint64(MTLTextureType2DMultisample))
		desc.Send(objc.RegisterName("setSampleCount:"), uint64(samples))
	}
	desc.Send(objc.RegisterName("setPixelFormat:"), pixel)
	desc.Send(objc.RegisterName("setWidth:"), uint64(s.width))
	desc.Send(objc.RegisterName("setHeight:"), uint64(s.height))
	desc.Send(objc.RegisterName("setUsage:"), uint64(MTLTextureUsageRenderTarget))
	desc.Send(objc.RegisterName("setStorageMode:"), uint64(MTLStorageModePrivate))
	t := uintptr(s.device.device.Send(objc.RegisterName("newTextureWithDescriptor:"), desc))
	if t == 0 {
		return 0, errors.New("metal: create a render target")
	}
	return t, nil
}

// acquire takes the next drawable and makes the targets the frame needs.
func (s *Surface) acquire() error {
	d := s.device
	s.variant = variantPlain
	if s.multisample && d.samples > 1 {
		s.variant = variantMulti
	}
	var err error
	if s.variant == variantMulti {
		if s.msaa == 0 {
			if s.msaa, err = s.renderTexture(MTLPixelFormatBGRA8Unorm, d.samples); err != nil {
				return err
			}
		}
		if s.depthMSAA == 0 {
			if s.depthMSAA, err = s.renderTexture(MTLPixelFormatDepth32Float_Stencil8, d.samples); err != nil {
				return err
			}
		}
	} else if s.depth == 0 {
		if s.depth, err = s.renderTexture(MTLPixelFormatDepth32Float_Stencil8, 1); err != nil {
			return err
		}
	}
	s.drawable = d.send(uintptr(s.layer), d.sel.nextDrawable)
	if s.drawable == 0 {
		return errors.New("metal: the layer has no drawable")
	}
	d.send(s.drawable, d.sel.retain)
	s.drawableTexture = d.send(s.drawable, d.sel.texture)
	return nil
}

// release lets go of the drawable after the frame.
func (s *Surface) release() {
	d := s.device
	if s.drawable != 0 {
		d.send(s.drawable, d.sel.release)
	}
	s.drawable, s.drawableTexture = 0, 0
}

// setClear sets the color that the first pass of a frame clears to. Metal
// takes it in floating-point registers, so it goes through objc.Send, and only
// when it changes.
func (s *Surface) setClear(c geom.Color) {
	if s.clearSet && c == s.clear {
		return
	}
	s.clear, s.clearSet = c, true
	objc.ID(s.color).Send(objc.RegisterName("setClearColor:"),
		MTLClearColor{float64(c.R), float64(c.G), float64(c.B), float64(c.A)})
}

// target returns the texture that the color of the frame ends up in, resolved.
func (s *Surface) target() uintptr {
	if s.drawn != nil {
		return uintptr(s.drawn.texture)
	}
	return s.drawableTexture
}

// begin opens the render encoder of the surface. resume continues a pass that
// a copy or a blur interrupted, keeping what it drew.
func (s *Surface) begin(resume bool) {
	d := s.device
	if s.drawn != nil {
		desc := d.targetPass()
		color := d.targetColor
		d.send(color, d.sel.setTexture, uintptr(s.drawn.texture))
		d.send(color, d.sel.setLoadAction, MTLLoadActionLoad)
		d.send(color, d.sel.setStoreAction, MTLStoreActionUnknown)
		d.beginRender(desc, &d.pipelines[variantFlat], s.width, s.height)
		return
	}
	load := uintptr(MTLLoadActionClear)
	if resume {
		load = MTLLoadActionLoad
	}
	depth := s.depth
	if s.variant == variantMulti {
		d.send(s.color, d.sel.setTexture, s.msaa)
		d.send(s.color, d.sel.setResolveTexture, s.drawableTexture)
		depth = s.depthMSAA
	} else {
		d.send(s.color, d.sel.setTexture, s.drawableTexture)
		d.send(s.color, d.sel.setResolveTexture, 0)
	}
	d.send(s.color, d.sel.setLoadAction, load)
	d.send(s.color, d.sel.setStoreAction, MTLStoreActionUnknown)
	for _, a := range []uintptr{s.depthAttachment, s.stencilAttachment} {
		d.send(a, d.sel.setTexture, depth)
		d.send(a, d.sel.setLoadAction, load)
		d.send(a, d.sel.setStoreAction, MTLStoreActionUnknown)
	}
	d.beginRender(s.pass, &d.pipelines[s.variant], s.width, s.height)
}

// releaseTarget ends the stand-in of DrawToTarget.
func (s *Surface) releaseTarget() {
	d := s.device
	d.send(d.targetColor, d.sel.setTexture, 0)
	*s = Surface{}
}

// targetPass returns the descriptor of the passes into textures of
// CreateTarget, made when first needed.
func (d *Device) targetPass() uintptr {
	if d.targetDesc == 0 {
		d.targetDesc = d.newPassDescriptor(false)
		d.targetColor, _, _ = attachments(d.targetDesc, false)
	}
	return d.targetDesc
}

// Destroy implements gpu.Surface.
func (s *Surface) Destroy() {
	d := s.device
	if d == nil {
		return
	}
	token := d.pushPool()
	s.releaseTargets()
	if s.pass != 0 {
		d.send(s.pass, d.sel.release)
	}
	d.send(uintptr(s.layer), d.sel.release)
	d.popPool(token)
	d.liveSurfaces--
	*s = Surface{}
}

// ReadPixels returns the frame drawn so far. Call it between Draw and EndFrame:
// it sends the commands of the frame, so that nothing more can be drawn. It
// stalls the GPU and exists for tests and screenshots.
func (d *Device) ReadPixels(s gpu.Surface) (*image.NRGBA, error) {
	sf := s.(*Surface)
	if !sf.inFrame {
		panic("gowaku: ReadPixels called outside BeginFrame and EndFrame")
	}
	token := d.pushPool()
	defer d.popPool(token)
	width, height := sf.width, sf.height
	row := width * 4
	readback := d.device.Send(objc.RegisterName("newBufferWithLength:options:"), uint64(row*height), uint64(MTLResourceStorageModeShared))
	if readback == 0 {
		return nil, errors.New("metal: allocate the readback buffer")
	}
	defer readback.Send(objc.RegisterName("release"))
	d.endRender(true)
	blit := d.blitEncoder()
	d.origin = MTLOrigin{}
	d.size = MTLSize{uint64(width), uint64(height), 1}
	d.send(blit, d.sel.copyTextureToBuffer, sf.drawableTexture, 0, 0, ptr(&d.origin), ptr(&d.size),
		uintptr(readback), 0, uintptr(row), uintptr(row*height))
	f := &d.frames[d.current]
	// EndFrame presents the drawable on its own.
	d.submit()
	if err := d.waitFor(f); err != nil {
		return nil, err
	}
	out := image.NewNRGBA(image.Rect(0, 0, width, height))
	// The buffer is shared memory of row*height bytes, complete after the wait.
	src := unsafe.Slice((*byte)(cPointer(d.send(uintptr(readback), d.sel.contents))), row*height)
	for i := 0; i < len(src); i += 4 {
		// BGRA. The window ignores alpha, so the frame is opaque by definition.
		out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = src[i+2], src[i+1], src[i], 0xff
	}
	return out, nil
}
