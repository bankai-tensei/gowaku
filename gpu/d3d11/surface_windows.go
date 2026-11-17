package d3d11

import (
	"errors"
	"fmt"
	"image"
	"unsafe"

	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/platform"
)

// Surface is the swap chain of one window.
type Surface struct {
	device    *Device
	swapChain *IDXGISwapChain1
	// rtv and dsv are what the frame in progress draws to, one of the two sets
	// below.
	rtv *ID3D11RenderTargetView
	dsv *ID3D11DepthStencilView
	// plain draws straight to the back buffer.
	plain targets
	// multi holds multisampled targets, which exist from the first frame that
	// needs them until the next resize.
	multi targets
	// color is the multisampled color, nil until it is needed.
	color *ID3D11Texture2D
	// multisample is what the next frame asks for, and resolving what this one
	// does.
	multisample, resolving bool

	viewport   D3D11_VIEWPORT
	scale      float32
	background [4]float32
	inFrame    bool
	// backdrop is the copy of the frame that blend modes which read it see, as
	// large as the surface and made when first needed.
	backdrop *Texture
	// drawn is the target that the stand-in surface of DrawToTarget draws to.
	drawn *Texture
	// chainFlags and presentFlags are those of the swap chain and its
	// presents, and composition shows the swap chain of a transparent
	// window.
	chainFlags, presentFlags uint32
	composition              composition
}

// targets is one set of what a frame draws to.
type targets struct {
	rtv   *ID3D11RenderTargetView
	dsv   *ID3D11DepthStencilView
	depth *ID3D11Texture2D
}

var _ gpu.Surface = (*Surface)(nil)

// CreateSurface implements gpu.Backend.
func (d *Device) CreateSurface(h platform.NativeHandle, physicalWidth, physicalHeight int, scale float32) (gpu.Surface, error) {
	win, ok := h.(platform.Win32Handle)
	if !ok {
		return nil, errors.New("d3d11: native handle is not a Win32 window")
	}
	// The flip model needs at least two buffers, and it is what lets the
	// compositor skip a copy. Without an sRGB format, blending happens on the
	// stored sRGB values (docs/ADR/0005-sdf-pipeline.md).
	desc := &DXGI_SWAP_CHAIN_DESC1{
		Width:       uint32(physicalWidth),
		Height:      uint32(physicalHeight),
		Format:      DXGI_FORMAT_B8G8R8A8_UNORM,
		SampleDesc:  DXGI_SAMPLE_DESC{Count: 1},
		BufferUsage: DXGI_USAGE_RENDER_TARGET_OUTPUT,
		BufferCount: 2,
		SwapEffect:  DXGI_SWAP_EFFECT_FLIP_DISCARD,
		Flags:       d.swapChainFlags,
	}
	s := &Surface{device: d, multisample: true, chainFlags: d.swapChainFlags, presentFlags: d.presentFlags}
	if win.Transparent {
		swapChain, c, err := d.createCompositionSurface(win, desc)
		if err != nil {
			return nil, fmt.Errorf("d3d11: %w", err)
		}
		s.swapChain, s.composition, s.chainFlags, s.presentFlags = swapChain, c, 0, 0
	} else {
		swapChain, err := d.factory.createSwapChainForHwnd(d.device, win.HWND, desc)
		if err != nil {
			return nil, fmt.Errorf("d3d11: %w", err)
		}
		s.swapChain = swapChain
		// gowaku windows have no exclusive fullscreen mode.
		if err := d.factory.makeWindowAssociation(win.HWND, DXGI_MWA_NO_ALT_ENTER); err != nil {
			swapChain.Release()
			return nil, fmt.Errorf("d3d11: %w", err)
		}
	}
	if err := s.acquireBackBuffer(physicalWidth, physicalHeight, scale); err != nil {
		s.swapChain.Release()
		s.composition.release()
		return nil, err
	}
	d.liveSurfaces++
	return s, nil
}

// Resize implements gpu.Surface.
func (s *Surface) Resize(physicalWidth, physicalHeight int, scale float32) error {
	if s.inFrame {
		panic("gowaku: Surface.Resize called between BeginFrame and EndFrame")
	}
	if physicalWidth <= 0 || physicalHeight <= 0 {
		return nil
	}
	if float32(physicalWidth) == s.viewport.Width && float32(physicalHeight) == s.viewport.Height {
		s.scale = scale
		return nil
	}

	// ResizeBuffers fails while anything refers to the back buffers: the view
	// here, the binding on the context, and commands not yet flushed.
	s.releaseTargets()
	// Without targets the surface has no size, so that a Resize after a failed
	// one is not taken for one that changes nothing.
	s.viewport = D3D11_VIEWPORT{}
	ctx := s.device.context
	ctx.clearState()
	ctx.flush()

	status := s.swapChain.resizeBuffers(uint32(physicalWidth), uint32(physicalHeight), s.chainFlags)
	if err := s.device.checkRemoved(status, "resize buffers"); err != nil {
		return err
	}
	return s.acquireBackBuffer(physicalWidth, physicalHeight, scale)
}

// Destroy implements gpu.Surface.
func (s *Surface) Destroy() {
	ctx := s.device.context
	ctx.clearState()
	ctx.flush()
	s.releaseTargets()
	s.composition.release()
	s.swapChain.Release()
	s.device.liveSurfaces--
	*s = Surface{}
}

func (s *Surface) releaseTargets() {
	for _, t := range []*targets{&s.plain, &s.multi} {
		for _, u := range []*unknown{unknownOf(t.dsv), unknownOf(t.depth), unknownOf(t.rtv)} {
			if u != nil {
				u.Release()
			}
		}
		*t = targets{}
	}
	if s.color != nil {
		unknownOf(s.color).Release()
	}
	if s.backdrop != nil {
		s.backdrop.Destroy()
	}
	s.color, s.rtv, s.dsv, s.backdrop = nil, nil, nil, nil
}

// SetMultisample implements gpu.Surface.
func (s *Surface) SetMultisample(on bool) { s.multisample = on }

// selectTargets picks the targets of a new frame.
func (s *Surface) selectTargets() error {
	s.resolving = s.multisample && s.device.samples > 1
	if !s.resolving {
		s.rtv, s.dsv = s.plain.rtv, s.plain.dsv
		return nil
	}
	if s.color == nil {
		if err := s.createMultisampled(); err != nil {
			// A set made in part would pass for a whole one on the next frame,
			// which would then draw into views that are nil.
			s.releaseMultisampled()
			return err
		}
	}
	s.rtv, s.dsv = s.multi.rtv, s.multi.dsv
	return nil
}

// releaseMultisampled releases the multisampled targets, whole or in part.
func (s *Surface) releaseMultisampled() {
	for _, u := range []*unknown{unknownOf(s.multi.dsv), unknownOf(s.multi.depth), unknownOf(s.multi.rtv), unknownOf(s.color)} {
		if u != nil {
			u.Release()
		}
	}
	s.multi, s.color = targets{}, nil
}

func (s *Surface) createMultisampled() error {
	d := s.device
	desc := s.targetDesc(d.samples)
	desc.Format, desc.BindFlags = DXGI_FORMAT_B8G8R8A8_UNORM, D3D11_BIND_RENDER_TARGET
	var err error
	if s.color, err = d.device.createTexture2D(&desc); err != nil {
		return err
	}
	if s.multi.rtv, err = d.device.createRenderTargetView(s.color); err != nil {
		return err
	}
	return s.createDepth(&s.multi, d.samples)
}

func (s *Surface) targetDesc(samples uint32) D3D11_TEXTURE2D_DESC {
	return D3D11_TEXTURE2D_DESC{
		Width:      uint32(s.viewport.Width),
		Height:     uint32(s.viewport.Height),
		MipLevels:  1,
		ArraySize:  1,
		SampleDesc: DXGI_SAMPLE_DESC{Count: samples},
		Usage:      D3D11_USAGE_DEFAULT,
	}
}

func (s *Surface) createDepth(t *targets, samples uint32) error {
	desc := s.targetDesc(samples)
	desc.Format, desc.BindFlags = DXGI_FORMAT_D24_UNORM_S8_UINT, D3D11_BIND_DEPTH_STENCIL
	var err error
	if t.depth, err = s.device.device.createTexture2D(&desc); err != nil {
		return err
	}
	t.dsv, err = s.device.device.createDepthStencilView(t.depth)
	return err
}

func (s *Surface) acquireBackBuffer(physicalWidth, physicalHeight int, scale float32) error {
	s.viewport = D3D11_VIEWPORT{Width: float32(physicalWidth), Height: float32(physicalHeight), MaxDepth: 1}
	s.scale = scale
	if err := s.createPlain(); err != nil {
		// A set made in part would leak from CreateSurface, which drops the
		// surface, and its view keeps the back buffer alive. No size is left
		// either, so that a Resize after this one starts over.
		s.releaseTargets()
		s.viewport = D3D11_VIEWPORT{}
		return fmt.Errorf("d3d11: %w", err)
	}
	return nil
}

func (s *Surface) createPlain() error {
	back, err := s.swapChain.backBuffer()
	if err != nil {
		return err
	}
	// The view holds its own reference.
	defer back.Release()
	if s.plain.rtv, err = s.device.device.createRenderTargetView(back); err != nil {
		return err
	}
	return s.createDepth(&s.plain, 1)
}

// resolve brings the multisampled frame into the back buffer.
func (s *Surface) resolve() error {
	if !s.resolving {
		return nil
	}
	back, err := s.swapChain.backBuffer()
	if err != nil {
		return fmt.Errorf("d3d11: %w", err)
	}
	s.device.context.resolveSubresource(back, s.color, DXGI_FORMAT_B8G8R8A8_UNORM)
	back.Release()
	return nil
}

// ReadPixels returns the frame drawn so far. Call it between Draw and EndFrame,
// because a flip-model back buffer is undefined after Present. It stalls the
// GPU and exists for tests and screenshots.
func (d *Device) ReadPixels(s gpu.Surface) (*image.NRGBA, error) {
	sf := s.(*Surface)
	if !sf.inFrame {
		panic("gowaku: ReadPixels called outside BeginFrame and EndFrame")
	}
	width, height := int(sf.viewport.Width), int(sf.viewport.Height)
	staging, err := d.device.createTexture2D(&D3D11_TEXTURE2D_DESC{
		Width:          uint32(width),
		Height:         uint32(height),
		MipLevels:      1,
		ArraySize:      1,
		Format:         DXGI_FORMAT_B8G8R8A8_UNORM,
		SampleDesc:     DXGI_SAMPLE_DESC{Count: 1},
		Usage:          D3D11_USAGE_STAGING,
		CPUAccessFlags: D3D11_CPU_ACCESS_READ,
	})
	if err != nil {
		return nil, fmt.Errorf("d3d11: %w", err)
	}
	defer staging.Release()

	if err := sf.resolve(); err != nil {
		return nil, err
	}
	back, err := sf.swapChain.backBuffer()
	if err != nil {
		return nil, fmt.Errorf("d3d11: %w", err)
	}
	defer back.Release()
	d.context.copyResource(staging, back)

	res := uintptr(unsafe.Pointer(staging))
	var mapped D3D11_MAPPED_SUBRESOURCE
	if err := d.context.mapResource(res, D3D11_MAP_READ, &mapped); err != nil {
		return nil, fmt.Errorf("d3d11: %w", err)
	}
	defer d.context.unmapResource(res)

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	// pData is valid until Unmap and holds height rows of RowPitch bytes.
	src := unsafe.Slice(mapped.pData, int(mapped.RowPitch)*height)
	for y := range height {
		row := src[y*int(mapped.RowPitch):]
		dst := img.Pix[y*img.Stride:]
		for x := range width {
			b, g, r, a := row[x*4], row[x*4+1], row[x*4+2], byte(0xff)
			if sf.composition.visual != nil {
				// A transparent window shows the alpha, over which the colors
				// are premultiplied.
				a = row[x*4+3]
				b, g, r = unpremultiply(b, a), unpremultiply(g, a), unpremultiply(r, a)
			}
			// An opaque window ignores alpha, so its frame is opaque by
			// definition.
			dst[x*4], dst[x*4+1], dst[x*4+2], dst[x*4+3] = r, g, b, a
		}
	}
	return img, nil
}

// unpremultiply divides a channel by the alpha it was multiplied with.
func unpremultiply(c, a byte) byte {
	if a == 0 {
		return 0
	}
	return byte(min(255, (uint32(c)*255+uint32(a)/2)/uint32(a)))
}
