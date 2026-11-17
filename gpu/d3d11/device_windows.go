package d3d11

import (
	"fmt"
	"log/slog"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

const (
	instanceStride = uint32(unsafe.Sizeof(gpu.Instance{}))

	// initialInstanceCapacity is the starting size of the instance buffer,
	// which grows on demand.
	initialInstanceCapacity = 1024

	// defaultSamples is the MSAA level of a frame that needs it. See
	// docs/ADR/0017-multisample-on-demand.md.
	defaultSamples = 4

	// quadVertices is the length of the triangle strip of one instance.
	quadVertices = 4
)

// Options configures New. The zero value selects the most power-efficient GPU
// and waits for vertical blank.
type Options struct {
	Power gpu.PowerPreference
	// Samples is the MSAA level, which smooths paths and shaped clips. Zero
	// selects 4, and 1 turns it off.
	Samples uint32
	// PresentImmediately stops EndFrame from waiting for vertical blank, even
	// at the cost of tearing. It exists to measure how fast frames can be
	// drawn. A plain sync interval of 0 would not do: the compositor still
	// paces flip-model swap chains to the display. It needs a system that
	// supports tearing, or creating a surface fails.
	PresentImmediately bool
}

// frameConstants mirrors cbuffer Frame in sdf.hlsli.
type frameConstants struct {
	viewportWidth, viewportHeight float32
	scale                         float32
	padding                       float32
}

// Device is the Direct3D 11 implementation of gpu.Backend. It owns the single
// device of the process; every Surface shares it.
type Device struct {
	factory *IDXGIFactory2
	adapter *IDXGIAdapter1
	device  *ID3D11Device
	context *ID3D11DeviceContext
	// context1 is the same context, for clearing part of a target.
	context1 *ID3D11DeviceContext1
	name     string

	vertexShader *ID3D11VertexShader
	pixelShader  *ID3D11PixelShader
	inputLayout  *ID3D11InputLayout
	blend        *ID3D11BlendState
	// blends holds the state of each blend mode, made on first use. Modes
	// with the same factors share the object of the first, gpu.BlendMode.State;
	// that of source-over is blend.
	blends     [gpu.BlendModeCount]*ID3D11BlendState
	rasterizer *ID3D11RasterizerState
	samples    uint32
	pipeline   pipeline

	constants      *ID3D11Buffer
	instances      *ID3D11Buffer
	instanceCap    int
	instanceStride uint32
	instanceOffset uint32
	mapped         D3D11_MAPPED_SUBRESOURCE
	scissor        D3D11_RECT
	box            D3D11_BOX
	boundTexture   *Texture
	boundImage     *Texture
	boundRamps     *Texture
	boundBackdrop  *Texture
	boundFiltered  *Texture
	syncInterval   uint32
	presentFlags   uint32
	swapChainFlags uint32

	// targetSurface stands in for a surface while drawing to a target.
	targetSurface Surface
	// targetBackdrop is the backdrop of blend modes that read a target.
	targetBackdrop *Texture
	// multi is where batches that need multisampling draw for a target.
	multi multiTarget
	// blurA and blurB are the scratch targets of blurs, made when first
	// needed, and blurs the blurs of the batch in progress.
	blurA, blurB *Texture
	blurs        []blurPlan
	clearRect    D3D11_RECT
	transparent  [4]float32

	// shaders holds the pixel shaders of the custom shaders that
	// PrepareShader took.
	shaders map[*gpu.Shader]*ID3D11PixelShader

	liveSurfaces int
	// timer measures frames on the GPU, from EnableGPUTiming on.
	timer *timer
}

var _ gpu.Backend = (*Device)(nil)

// New creates the device. It returns gpu.ErrNoGPU if no hardware adapter
// supports feature level 11_0.
func New(opts Options) (*Device, error) {
	for _, p := range []*windows.LazyProc{procD3D11CreateDevice, procCreateDXGIFactory1} {
		if err := p.Find(); err != nil {
			return nil, fmt.Errorf("d3d11: load %s: %w", p.Name, err)
		}
	}
	d := &Device{instanceStride: instanceStride, syncInterval: 1, samples: opts.Samples}
	if d.samples == 0 {
		d.samples = defaultSamples
	}
	if opts.PresentImmediately {
		d.syncInterval = 0
		d.presentFlags = DXGI_PRESENT_ALLOW_TEARING
		d.swapChainFlags = DXGI_SWAP_CHAIN_FLAG_ALLOW_TEARING
	}
	if err := d.init(opts); err != nil {
		d.release()
		return nil, err
	}
	slog.Debug("d3d11: device created", "adapter", d.name, "power", opts.Power)
	return d, nil
}

func (d *Device) init(opts Options) error {
	r, _, _ := procCreateDXGIFactory1.Call(uintptr(unsafe.Pointer(&IID_IDXGIFactory2)), uintptr(unsafe.Pointer(&d.factory)))
	if err := hr(r).err("create dxgi factory"); err != nil {
		return fmt.Errorf("d3d11: %w", err)
	}
	if err := d.pickAdapter(opts.Power); err != nil {
		return err
	}
	if err := d.createDevice(); err != nil {
		return err
	}
	if err := d.createPipeline(); err != nil {
		return fmt.Errorf("d3d11: %w", err)
	}
	return nil
}

// pickAdapter keeps the first hardware adapter in preference order. Windows
// before 1803 has no IDXGIFactory6, and there the OS default adapter is used.
func (d *Device) pickAdapter(power gpu.PowerPreference) error {
	preference := uint32(DXGI_GPU_PREFERENCE_MINIMUM_POWER)
	if power == gpu.HighPerformance {
		preference = DXGI_GPU_PREFERENCE_HIGH_PERFORMANCE
	}
	factory6 := d.factory.queryFactory6()
	if factory6 != nil {
		defer factory6.Release()
	}
	for i := uint32(0); ; i++ {
		var adapter *IDXGIAdapter1
		var status hresult
		if factory6 != nil {
			adapter, status = factory6.enumAdapterByGpuPreference(i, preference)
		} else {
			adapter, status = d.factory.enumAdapters1(i)
		}
		if uint32(status) == DXGI_ERROR_NOT_FOUND {
			return gpu.ErrNoGPU
		}
		if err := status.err("enumerate adapters"); err != nil {
			return fmt.Errorf("d3d11: %w", err)
		}
		desc, err := adapter.desc1()
		if err != nil {
			adapter.Release()
			return fmt.Errorf("d3d11: %w", err)
		}
		if desc.Flags&DXGI_ADAPTER_FLAG_SOFTWARE != 0 {
			adapter.Release()
			continue
		}
		d.adapter, d.name = adapter, adapterName(&desc)
		return nil
	}
}

func (d *Device) createDevice() error {
	levels := [...]uint32{D3D_FEATURE_LEVEL_11_1, D3D_FEATURE_LEVEL_11_0}
	r, _, _ := procD3D11CreateDevice.Call(
		uintptr(unsafe.Pointer(d.adapter)), D3D_DRIVER_TYPE_UNKNOWN, 0, D3D11_CREATE_DEVICE_BGRA_SUPPORT,
		uintptr(unsafe.Pointer(&levels[0])), uintptr(len(levels)), D3D11_SDK_VERSION,
		uintptr(unsafe.Pointer(&d.device)), 0, uintptr(unsafe.Pointer(&d.context)),
	)
	if hr(r).failed() {
		return fmt.Errorf("d3d11: create device on %q: %w: %w", d.name, gpu.ErrNoGPU, hr(r).err("D3D11CreateDevice"))
	}
	var err error
	if d.context1, err = d.context.queryContext1(); err != nil {
		return fmt.Errorf("d3d11: %w", err)
	}
	return nil
}

func (d *Device) createPipeline() error {
	var err error
	if d.vertexShader, err = d.device.createVertexShader(sdfVS); err != nil {
		return err
	}
	if d.pixelShader, err = d.device.createPixelShader(sdfPS); err != nil {
		return err
	}

	if err := d.createInputLayout(); err != nil {
		return err
	}

	// The pixel shader outputs premultiplied colors. They stay sRGB and are
	// composited without linearizing (docs/ADR/0005-sdf-pipeline.md).
	var blend D3D11_BLEND_DESC
	blend.RenderTarget[0] = D3D11_RENDER_TARGET_BLEND_DESC{
		BlendEnable:           1,
		SrcBlend:              D3D11_BLEND_ONE,
		DestBlend:             D3D11_BLEND_INV_SRC_ALPHA,
		BlendOp:               D3D11_BLEND_OP_ADD,
		SrcBlendAlpha:         D3D11_BLEND_ONE,
		DestBlendAlpha:        D3D11_BLEND_INV_SRC_ALPHA,
		BlendOpAlpha:          D3D11_BLEND_OP_ADD,
		RenderTargetWriteMask: D3D11_COLOR_WRITE_ENABLE_ALL,
	}
	if d.blend, err = d.device.createBlendState(&blend); err != nil {
		return err
	}
	d.blends[gpu.BlendSrcOver] = d.blend
	if d.rasterizer, err = d.device.createRasterizerState(&D3D11_RASTERIZER_DESC{
		FillMode:        D3D11_FILL_SOLID,
		CullMode:        D3D11_CULL_NONE,
		DepthClipEnable: 1,
		ScissorEnable:   1,
		// Without this the rasterizer ignores the samples of a triangle edge.
		MultisampleEnable: 1,
	}); err != nil {
		return err
	}
	if err := d.pipeline.create(d); err != nil {
		return err
	}

	if d.constants, err = d.device.createBuffer(&D3D11_BUFFER_DESC{
		ByteWidth:      uint32(unsafe.Sizeof(frameConstants{})),
		Usage:          D3D11_USAGE_DYNAMIC,
		BindFlags:      D3D11_BIND_CONSTANT_BUFFER,
		CPUAccessFlags: D3D11_CPU_ACCESS_WRITE,
	}); err != nil {
		return err
	}
	return d.growInstances(initialInstanceCapacity)
}

func (d *Device) createInputLayout() error {
	names := [...]string{"RECT", "RADII", "COLOR", "GRADIENT", "PARAMS", "MODE", "STOPS", "TRANSFORM"}
	var pointers [len(names)]*byte
	for i, name := range names {
		p, err := windows.BytePtrFromString(name)
		if err != nil {
			return err
		}
		pointers[i] = p
	}
	element := func(name *byte, index, format uint32, offset uintptr) D3D11_INPUT_ELEMENT_DESC {
		return D3D11_INPUT_ELEMENT_DESC{
			SemanticName:         name,
			SemanticIndex:        index,
			Format:               format,
			AlignedByteOffset:    uint32(offset),
			InputSlotClass:       D3D11_INPUT_PER_INSTANCE_DATA,
			InstanceDataStepRate: 1,
		}
	}
	var in gpu.Instance
	elems := []D3D11_INPUT_ELEMENT_DESC{
		element(pointers[0], 0, DXGI_FORMAT_R32G32B32A32_FLOAT, unsafe.Offsetof(in.Rect)),
		element(pointers[1], 0, DXGI_FORMAT_R32G32B32A32_FLOAT, unsafe.Offsetof(in.Radii)),
		element(pointers[2], 0, DXGI_FORMAT_R32G32B32A32_FLOAT, unsafe.Offsetof(in.Color)),
		element(pointers[2], 1, DXGI_FORMAT_R32G32B32A32_FLOAT, unsafe.Offsetof(in.EndColor)),
		element(pointers[2], 2, DXGI_FORMAT_R32G32B32A32_FLOAT, unsafe.Offsetof(in.BorderColor)),
		element(pointers[3], 0, DXGI_FORMAT_R32G32B32A32_FLOAT, unsafe.Offsetof(in.GradientStart)),
		element(pointers[4], 0, DXGI_FORMAT_R32G32_FLOAT, unsafe.Offsetof(in.BorderWidth)),
		element(pointers[5], 0, DXGI_FORMAT_R32G32_UINT, unsafe.Offsetof(in.Kind)),
		element(pointers[2], 3, DXGI_FORMAT_R32G32B32A32_FLOAT, unsafe.Offsetof(in.MidColors)),
		element(pointers[2], 4, DXGI_FORMAT_R32G32B32A32_FLOAT, unsafe.Offsetof(in.MidColors)+16),
		element(pointers[6], 0, DXGI_FORMAT_R32G32B32A32_FLOAT, unsafe.Offsetof(in.StopOffsets)),
		element(pointers[7], 0, DXGI_FORMAT_R32G32B32A32_FLOAT, unsafe.Offsetof(in.Transform)),
		element(pointers[7], 1, DXGI_FORMAT_R32G32_FLOAT, unsafe.Offsetof(in.Transform)+16),
		element(pointers[3], 1, DXGI_FORMAT_R32G32B32A32_FLOAT, unsafe.Offsetof(in.GradientMatrix)),
	}
	var err error
	d.inputLayout, err = d.device.createInputLayout(elems, sdfVS)
	// The call reads the names through pointers stored in elems.
	runtime.KeepAlive(pointers)
	return err
}

func (d *Device) growInstances(capacity int) error {
	b, err := d.device.createBuffer(&D3D11_BUFFER_DESC{
		ByteWidth:      uint32(capacity) * instanceStride,
		Usage:          D3D11_USAGE_DYNAMIC,
		BindFlags:      D3D11_BIND_VERTEX_BUFFER,
		CPUAccessFlags: D3D11_CPU_ACCESS_WRITE,
	})
	if err != nil {
		return err
	}
	if d.instances != nil {
		d.instances.Release()
	}
	d.instances, d.instanceCap = b, capacity
	return nil
}

// AdapterName returns the description of the GPU in use.
func (d *Device) AdapterName() string { return d.name }

// BeginFrame implements gpu.Backend.
func (d *Device) BeginFrame(s gpu.Surface, background geom.Color) error {
	sf := s.(*Surface)
	if sf.inFrame {
		panic("gowaku: BeginFrame called twice without EndFrame")
	}
	if err := sf.selectTargets(); err != nil {
		return fmt.Errorf("d3d11: %w", err)
	}
	sf.inFrame = true
	d.startTiming()

	sf.background = [4]float32{background.R, background.G, background.B, background.A}
	d.context.clearRenderTargetView(sf.rtv, &sf.background)
	d.context.clearDepthStencilView(sf.dsv)

	if err := d.writeConstants(sf); err != nil {
		return fmt.Errorf("d3d11: %w", err)
	}

	d.bindFrame(sf, sf.dsv)
	return nil
}

// bindFrame sets up the context to draw to sf. The context is shared by all
// surfaces and targets, so everything is set again each time instead of being
// assumed from the last time.
func (d *Device) bindFrame(sf *Surface, dsv *ID3D11DepthStencilView) {
	d.context.omSetRenderTarget(&sf.rtv, dsv)
	d.context.rsSetViewport(&sf.viewport)
	d.context.rsSetState(d.rasterizer)
	d.context.omSetBlendState(d.blend)
	d.pipeline.reset(d, sf)
	d.context.vsSetConstantBuffer(0, &d.constants)
	d.context.psSetConstantBuffer(0, &d.constants)
	d.boundTexture, d.boundImage, d.boundRamps, d.boundBackdrop, d.boundFiltered = nil, nil, nil, nil, nil
}

func (d *Device) writeConstants(sf *Surface) error {
	res := uintptr(unsafe.Pointer(d.constants))
	if err := d.context.mapResource(res, D3D11_MAP_WRITE_DISCARD, &d.mapped); err != nil {
		return err
	}
	// pData is valid until Unmap and has room for frameConstants.
	*(*frameConstants)(unsafe.Pointer(d.mapped.pData)) = frameConstants{
		viewportWidth:  sf.viewport.Width,
		viewportHeight: sf.viewport.Height,
		scale:          sf.scale,
	}
	d.context.unmapResource(res)
	return nil
}

// Draw implements gpu.Backend.
func (d *Device) Draw(s gpu.Surface, batch *gpu.Batch) error {
	sf := s.(*Surface)
	if !sf.inFrame {
		panic("gowaku: Draw called outside BeginFrame and EndFrame")
	}
	return d.drawBatch(sf, batch)
}

// drawBatch draws batch to what bindFrame set up.
func (d *Device) drawBatch(sf *Surface, batch *gpu.Batch) error {
	n := len(batch.Instances)
	if n == 0 {
		return nil
	}
	// Each blur adds the instances of its passes after those of the batch.
	blurs := batch.CountBlurs()
	total := n + 3*blurs
	if total > d.instanceCap {
		if err := d.growInstances(max(total, d.instanceCap*2)); err != nil {
			return fmt.Errorf("d3d11: %w", err)
		}
	}
	// WRITE_DISCARD makes the driver hand out fresh memory while the GPU still
	// reads the previous frame, so that the buffer acts as a ring.
	res := uintptr(unsafe.Pointer(d.instances))
	if err := d.context.mapResource(res, D3D11_MAP_WRITE_DISCARD, &d.mapped); err != nil {
		return fmt.Errorf("d3d11: %w", err)
	}
	// pData is valid until Unmap and holds instanceCap instances.
	mapped := unsafe.Slice((*gpu.Instance)(unsafe.Pointer(d.mapped.pData)), total)
	copy(mapped, batch.Instances)
	if blurs > 0 {
		d.planBlurs(sf, batch, mapped[n:])
	}
	d.context.unmapResource(res)

	if err := d.uploadVertices(batch); err != nil {
		return fmt.Errorf("d3d11: %w", err)
	}
	if t, ok := batch.Ramps.(*Texture); ok && t != d.boundRamps {
		d.boundRamps = t
		d.context.psSetShaderResource(2, &t.view)
	}
	blur := 0
	for i := range batch.Draws {
		call := &batch.Draws[i]
		if call.Count == 0 {
			continue
		}
		d.pipeline.quadShader = d.pixelShader
		var custom *ID3D11PixelShader
		if call.Shader != nil {
			if custom = d.shaders[call.Shader]; custom == nil {
				// Not prepared, or refused: render reports it.
				continue
			}
		}
		if call.Blur > 0 {
			drawn, err := d.drawBlur(sf, call, &d.blurs[blur], n+3*blur)
			blur++
			if err != nil {
				return err
			}
			if !drawn {
				continue
			}
		}
		if call.Blend != gpu.BlendSrcOver {
			drawn, err := d.prepareBlend(sf, call)
			if err != nil {
				return err
			}
			if !drawn {
				continue
			}
		}
		if custom != nil {
			d.pipeline.quadShader = custom
		}
		d.setScissor(sf, call)
		d.bindTexture(call)
		d.pipeline.draw(d, sf, call)
	}
	return nil
}

// setScissor limits drawing to the clip of call. The rasterizer always has
// scissoring on, so an unclipped call gets the whole surface.
func (d *Device) setScissor(sf *Surface, call *gpu.DrawCall) {
	width, height := int32(sf.viewport.Width), int32(sf.viewport.Height)
	d.scissor = D3D11_RECT{right: width, bottom: height}
	if call.Clipped {
		left, top, right, bottom := gpu.PhysicalBounds(call.Clip, sf.scale)
		d.scissor = D3D11_RECT{
			left:   min(max(left, 0), width),
			top:    min(max(top, 0), height),
			right:  min(max(right, 0), width),
			bottom: min(max(bottom, 0), height),
		}
	}
	d.context.rsSetScissorRect(&d.scissor)
}

// bindTexture binds the texture of call, unless it is already bound.
func (d *Device) bindTexture(call *gpu.DrawCall) {
	if t, ok := call.Texture.(*Texture); ok && t != d.boundTexture {
		d.boundTexture = t
		d.context.psSetShaderResource(0, &t.view)
	}
	if t, ok := call.Image.(*Texture); ok && t != d.boundImage {
		d.boundImage = t
		d.context.psSetShaderResource(1, &t.view)
	}
}

// EndFrame implements gpu.Backend.
func (d *Device) EndFrame(s gpu.Surface) error {
	sf := s.(*Surface)
	if !sf.inFrame {
		panic("gowaku: EndFrame called without BeginFrame")
	}
	sf.inFrame = false
	if err := sf.resolve(); err != nil {
		return err
	}
	d.stopTiming()
	status := sf.swapChain.present(d.syncInterval, sf.presentFlags)
	d.panicIfLost(status)
	if err := status.err("present"); err != nil {
		return fmt.Errorf("d3d11: %w", err)
	}
	return nil
}

// panicIfLost turns a lost device into a panic: a device lost during a frame is
// a broken contract, not an error to return.
func (d *Device) panicIfLost(status hresult) {
	switch uint32(status) {
	case DXGI_ERROR_DEVICE_REMOVED, DXGI_ERROR_DEVICE_HUNG, DXGI_ERROR_DEVICE_RESET:
		panic(fmt.Sprintf("gowaku: GPU device lost: HRESULT %#08x, reason %#08x",
			uint32(status), uint32(d.device.deviceRemovedReason())))
	}
}

// Destroy implements gpu.Backend.
func (d *Device) Destroy() {
	if d.liveSurfaces != 0 {
		panic("gowaku: Device.Destroy called while surfaces are alive")
	}
	d.release()
}

// release drops every reference in reverse order of creation. It also serves
// a partly initialized Device.
func (d *Device) release() {
	if d.context != nil {
		d.context.clearState()
		d.context.flush()
	}
	d.pipeline.release()
	for _, ps := range d.shaders {
		unknownOf(ps).Release()
	}
	if d.timer != nil {
		d.timer.release()
	}
	d.multi.release()
	for _, t := range []*Texture{d.targetBackdrop, d.blurA, d.blurB} {
		if t != nil {
			t.Destroy()
		}
	}
	// Modes that share a state share the object, which the first holds.
	for i, s := range d.blends {
		if s != nil && gpu.BlendMode(i).State() == gpu.BlendMode(i) && i != int(gpu.BlendSrcOver) {
			unknownOf(s).Release()
		}
	}
	for _, u := range []*unknown{
		unknownOf(d.instances), unknownOf(d.constants), unknownOf(d.rasterizer), unknownOf(d.blend),
		unknownOf(d.inputLayout), unknownOf(d.pixelShader), unknownOf(d.vertexShader),
		unknownOf(d.context1), unknownOf(d.context), unknownOf(d.device), unknownOf(d.adapter), unknownOf(d.factory),
	} {
		if u != nil {
			u.Release()
		}
	}
	*d = Device{}
}

// comObject is any of the interface types, whose pointers are all COM pointers.
type comObject interface {
	IDXGIFactory2 | IDXGIAdapter1 | ID3D11Device | ID3D11DeviceContext | ID3D11DeviceContext1 | ID3D11Buffer |
		ID3D11DepthStencilState | ID3D11SamplerState | ID3D11DepthStencilView | ID3D11RenderTargetView |
		ID3D11InputLayout | ID3D11VertexShader | ID3D11PixelShader | ID3D11BlendState |
		ID3D11RasterizerState | ID3D11Texture2D | ID3D11ShaderResourceView | ID3D11Query
}

func unknownOf[T comObject](p *T) *unknown { return (*unknown)(unsafe.Pointer(p)) }

// checkRemoved reports a lost device as a panic and anything else as an error.
func (d *Device) checkRemoved(status hresult, op string) error {
	d.panicIfLost(status)
	if err := status.err(op); err != nil {
		return fmt.Errorf("d3d11: %w", err)
	}
	return nil
}
