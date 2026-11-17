package d3d11

import (
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

const (
	vertexStride = uint32(unsafe.Sizeof(geom.Offset{}))

	initialVertexCapacity = 1024

	// clipDepthLevels spreads clip levels over the depth range. See
	// docs/ADR/0009-paths-and-clips.md.
	clipDepthLevels = 256
)

// stage is what the input assembler and shaders are set up for.
type stage uint8

const (
	stageNone stage = iota
	stageQuads
	stageFan
)

// pipeline holds the state objects of P1 to P3, and what is bound now, because
// a batch switches between them often.
type pipeline struct {
	fanShader *ID3D11VertexShader
	fanLayout *ID3D11InputLayout
	vertices  *ID3D11Buffer
	vertexCap int
	stride    uint32
	offset    uint32
	sampler   *ID3D11SamplerState
	noColor   *ID3D11BlendState

	// quad tests the clip level. The others are named after their step.
	quad, fanNonZero, fanEvenOdd, cover, clipCover, clipPop *ID3D11DepthStencilState

	stage stage
	// quadShader is the pixel shader of quads in the call in progress: the
	// SDF shader or a custom one. ps is the one bound, nil while drawing fans.
	quadShader *ID3D11PixelShader
	ps         *ID3D11PixelShader
	blend      *ID3D11BlendState
	depth      *ID3D11DepthStencilState
	level      int
	surface    D3D11_VIEWPORT
}

func (p *pipeline) create(d *Device) error {
	var err error
	p.stride = vertexStride
	if p.fanShader, err = d.device.createVertexShader(pathVS); err != nil {
		return err
	}
	name, err := windows.BytePtrFromString("POSITION")
	if err != nil {
		return err
	}
	p.fanLayout, err = d.device.createInputLayout([]D3D11_INPUT_ELEMENT_DESC{{
		SemanticName:   name,
		Format:         DXGI_FORMAT_R32G32_FLOAT,
		InputSlotClass: D3D11_INPUT_PER_VERTEX_DATA,
	}}, pathVS)
	runtime.KeepAlive(name)
	if err != nil {
		return err
	}
	if p.sampler, err = d.device.createSamplerState(&D3D11_SAMPLER_DESC{
		Filter:   D3D11_FILTER_MIN_MAG_MIP_LINEAR,
		AddressU: D3D11_TEXTURE_ADDRESS_CLAMP,
		AddressV: D3D11_TEXTURE_ADDRESS_CLAMP,
		AddressW: D3D11_TEXTURE_ADDRESS_CLAMP,
		MaxLOD:   D3D11_FLOAT32_MAX,
	}); err != nil {
		return err
	}
	var silent D3D11_BLEND_DESC // a write mask of zero
	if p.noColor, err = d.device.createBlendState(&silent); err != nil {
		return err
	}
	return p.createDepthStates(d)
}

func (p *pipeline) createDepthStates(d *Device) error {
	state := func(depthFunc, write uint32, front, back *D3D11_DEPTH_STENCILOP_DESC) (*ID3D11DepthStencilState, error) {
		desc := D3D11_DEPTH_STENCIL_DESC{
			DepthEnable:      1,
			DepthWriteMask:   write,
			DepthFunc:        depthFunc,
			StencilReadMask:  0xFF,
			StencilWriteMask: 0xFF,
		}
		if front != nil {
			desc.StencilEnable, desc.FrontFace, desc.BackFace = 1, *front, *back
		}
		return d.device.createDepthStencilState(&desc)
	}
	mark := func(op uint32) *D3D11_DEPTH_STENCILOP_DESC {
		return &D3D11_DEPTH_STENCILOP_DESC{
			StencilFailOp:      D3D11_STENCIL_OP_KEEP,
			StencilDepthFailOp: D3D11_STENCIL_OP_KEEP,
			StencilPassOp:      op,
			StencilFunc:        D3D11_COMPARISON_ALWAYS,
		}
	}
	// Covering a shape resets its stencil to zero for the next shape.
	take := &D3D11_DEPTH_STENCILOP_DESC{
		StencilFailOp:      D3D11_STENCIL_OP_KEEP,
		StencilDepthFailOp: D3D11_STENCIL_OP_ZERO,
		StencilPassOp:      D3D11_STENCIL_OP_ZERO,
		StencilFunc:        D3D11_COMPARISON_NOT_EQUAL,
	}
	var err error
	if p.quad, err = state(D3D11_COMPARISON_EQUAL, D3D11_DEPTH_WRITE_MASK_ZERO, nil, nil); err != nil {
		return err
	}
	if p.fanNonZero, err = state(D3D11_COMPARISON_EQUAL, D3D11_DEPTH_WRITE_MASK_ZERO, mark(D3D11_STENCIL_OP_INCR), mark(D3D11_STENCIL_OP_DECR)); err != nil {
		return err
	}
	invert := mark(D3D11_STENCIL_OP_INVERT)
	if p.fanEvenOdd, err = state(D3D11_COMPARISON_EQUAL, D3D11_DEPTH_WRITE_MASK_ZERO, invert, invert); err != nil {
		return err
	}
	if p.cover, err = state(D3D11_COMPARISON_EQUAL, D3D11_DEPTH_WRITE_MASK_ZERO, take, take); err != nil {
		return err
	}
	if p.clipCover, err = state(D3D11_COMPARISON_GREATER, D3D11_DEPTH_WRITE_MASK_ALL, take, take); err != nil {
		return err
	}
	p.clipPop, err = state(D3D11_COMPARISON_LESS, D3D11_DEPTH_WRITE_MASK_ALL, nil, nil)
	return err
}

func (p *pipeline) release() {
	for _, u := range []*unknown{
		unknownOf(p.clipPop), unknownOf(p.clipCover), unknownOf(p.cover), unknownOf(p.fanEvenOdd),
		unknownOf(p.fanNonZero), unknownOf(p.quad), unknownOf(p.noColor), unknownOf(p.sampler),
		unknownOf(p.vertices), unknownOf(p.fanLayout), unknownOf(p.fanShader),
	} {
		if u != nil {
			u.Release()
		}
	}
	*p = pipeline{}
}

// reset forgets what is bound, because the context is shared by all surfaces.
func (p *pipeline) reset(d *Device, sf *Surface) {
	p.stage, p.blend, p.depth, p.level = stageNone, nil, nil, -1
	p.quadShader, p.ps = d.pixelShader, nil
	p.surface = sf.viewport
	d.context.psSetSampler(&p.sampler)
}

func (d *Device) uploadVertices(batch *gpu.Batch) error {
	n := len(batch.Vertices)
	if n == 0 {
		return nil
	}
	p := &d.pipeline
	if n > p.vertexCap {
		capacity := max(n, p.vertexCap*2, initialVertexCapacity)
		b, err := d.device.createBuffer(&D3D11_BUFFER_DESC{
			ByteWidth:      uint32(capacity) * vertexStride,
			Usage:          D3D11_USAGE_DYNAMIC,
			BindFlags:      D3D11_BIND_VERTEX_BUFFER,
			CPUAccessFlags: D3D11_CPU_ACCESS_WRITE,
		})
		if err != nil {
			return err
		}
		if p.vertices != nil {
			p.vertices.Release()
		}
		p.vertices, p.vertexCap = b, capacity
	}
	res := uintptr(unsafe.Pointer(p.vertices))
	if err := d.context.mapResource(res, D3D11_MAP_WRITE_DISCARD, &d.mapped); err != nil {
		return err
	}
	// pData is valid until Unmap and holds vertexCap vertices.
	copy(unsafe.Slice((*geom.Offset)(unsafe.Pointer(d.mapped.pData)), n), batch.Vertices)
	d.context.unmapResource(res)
	return nil
}

// draw runs one call. See docs/ADR/0009-paths-and-clips.md for the steps.
func (p *pipeline) draw(d *Device, sf *Surface, call *gpu.DrawCall) {
	level := int(call.ClipLevel)
	switch call.Kind {
	case gpu.DrawQuads:
		p.use(d, sf, stageQuads, d.blends[call.Blend], p.quad, level)
		p.quads(d, call)
	case gpu.DrawFill:
		p.fan(d, sf, call, level)
		p.use(d, sf, stageQuads, d.blends[call.Blend], p.cover, level)
		p.quads(d, call)
	case gpu.DrawClipPush:
		p.fan(d, sf, call, level)
		p.use(d, sf, stageQuads, p.noColor, p.clipCover, level+1)
		p.quads(d, call)
	case gpu.DrawClipPop:
		p.use(d, sf, stageQuads, p.noColor, p.clipPop, level)
		p.quads(d, call)
	}
}

func (p *pipeline) quads(d *Device, call *gpu.DrawCall) {
	d.context.drawInstanced(quadVertices, uint32(call.Count), 0, uint32(call.First))
}

// fan marks the shape of call in the stencil buffer.
func (p *pipeline) fan(d *Device, sf *Surface, call *gpu.DrawCall, level int) {
	state := p.fanNonZero
	if call.EvenOdd {
		state = p.fanEvenOdd
	}
	p.use(d, sf, stageFan, p.noColor, state, level)
	d.context.draw(uint32(call.VertexCount), uint32(call.VertexFirst))
}

// use binds what differs from the last call.
func (p *pipeline) use(d *Device, sf *Surface, st stage, blend *ID3D11BlendState, depth *ID3D11DepthStencilState, level int) {
	ctx := d.context
	if st != p.stage {
		p.stage = st
		if st == stageFan {
			ctx.iaSetInputLayout(p.fanLayout)
			ctx.iaSetPrimitiveTopology(D3D11_PRIMITIVE_TOPOLOGY_TRIANGLELIST)
			ctx.iaSetVertexBuffer(&p.vertices, &p.stride, &p.offset)
			ctx.vsSetShader(p.fanShader)
			ctx.psSetShader(nil)
			p.ps = nil
		} else {
			ctx.iaSetInputLayout(d.inputLayout)
			ctx.iaSetPrimitiveTopology(D3D11_PRIMITIVE_TOPOLOGY_TRIANGLESTRIP)
			ctx.iaSetVertexBuffer(&d.instances, &d.instanceStride, &d.instanceOffset)
			ctx.vsSetShader(d.vertexShader)
			ctx.psSetShader(p.quadShader)
			p.ps = p.quadShader
		}
	} else if st == stageQuads && p.ps != p.quadShader {
		ctx.psSetShader(p.quadShader)
		p.ps = p.quadShader
	}
	if blend != p.blend {
		p.blend = blend
		ctx.omSetBlendState(blend)
	}
	if depth != p.depth {
		p.depth = depth
		ctx.omSetDepthStencilState(depth)
	}
	if level != p.level {
		p.level = level
		// Everything drawn has z = 0, so a viewport with no depth range puts it
		// at the depth of the level, with no shader involved.
		z := float32(level) / clipDepthLevels
		p.surface = sf.viewport
		p.surface.MinDepth, p.surface.MaxDepth = z, z
		ctx.rsSetViewport(&p.surface)
	}
}
