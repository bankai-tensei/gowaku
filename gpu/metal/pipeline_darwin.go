//go:build darwin && arm64

package metal

import (
	"fmt"

	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/gpu"
)

// clipDepthLevels spreads clip levels over the depth range, as in Vulkan. See
// docs/ADR/0009-paths-and-clips.md.
const clipDepthLevels = 256

// pipelineKind is a step of a draw call. Metal keeps the depth and stencil
// tests apart from the pipeline, so each kind is a depth-stencil state, and
// the pipelines are the quads by blend mode, the quads without color and the
// fan.
type pipelineKind uint8

// The steps, as in the Vulkan backend.
const (
	pipelineQuad pipelineKind = iota
	pipelineFanNonZero
	pipelineFanEvenOdd
	pipelineCover
	pipelineClipCover
	pipelineClipPop
	pipelineCount
)

// The variants of the render passes: a surface without and with
// multisampling, and a texture of CreateTarget, which has no depth because
// its batches have no paths or clips.
const (
	variantPlain = iota
	variantMulti
	variantFlat
	variantCount
)

// pipelineSet holds the render pipelines of one variant.
type pipelineSet struct {
	samples int
	depth   bool
	// quad holds the quads of each blend state, made when first needed and
	// indexed by gpu.BlendMode.State.
	quad    [gpu.BlendModeCount]uintptr
	noColor uintptr
	fan     uintptr
}

// stencilOp is a MTLStencilDescriptor.
type stencilOp struct {
	compare, fail, depthFail, pass uint64
}

func mark(op uint64) *stencilOp {
	return &stencilOp{MTLCompareFunctionAlways, MTLStencilOperationKeep, MTLStencilOperationKeep, op}
}

// take passes where the stencil is set and clears it on the way, so that the
// next shape starts from zero.
func take() *stencilOp {
	return &stencilOp{MTLCompareFunctionNotEqual, MTLStencilOperationKeep, MTLStencilOperationZero, MTLStencilOperationZero}
}

// createDepthStates makes the tests of each step, the same as pipelineSteps of
// the Vulkan backend.
func (d *Device) createDepthStates() {
	invert := mark(MTLStencilOperationInvert)
	states := [pipelineCount]struct {
		compare     uint64
		write       bool
		front, back *stencilOp
	}{
		pipelineQuad:       {compare: MTLCompareFunctionEqual},
		pipelineFanNonZero: {MTLCompareFunctionEqual, false, mark(MTLStencilOperationIncrementWrap), mark(MTLStencilOperationDecrementWrap)},
		pipelineFanEvenOdd: {MTLCompareFunctionEqual, false, invert, invert},
		pipelineCover:      {MTLCompareFunctionEqual, false, take(), take()},
		pipelineClipCover:  {MTLCompareFunctionGreater, true, take(), take()},
		pipelineClipPop:    {compare: MTLCompareFunctionLess, write: true},
	}
	for i, s := range states {
		d.depthStates[i] = d.newDepthState(s.compare, s.write, s.front, s.back)
	}
	d.noDepth = d.newDepthState(MTLCompareFunctionAlways, false, nil, nil)
}

func (d *Device) newDepthState(compare uint64, write bool, front, back *stencilOp) uintptr {
	desc := objc.ID(objc.GetClass("MTLDepthStencilDescriptor")).Send(objc.RegisterName("new"))
	defer desc.Send(objc.RegisterName("release"))
	desc.Send(objc.RegisterName("setDepthCompareFunction:"), compare)
	desc.Send(objc.RegisterName("setDepthWriteEnabled:"), write)
	stencil := func(op *stencilOp) objc.ID {
		s := objc.ID(objc.GetClass("MTLStencilDescriptor")).Send(objc.RegisterName("new"))
		s.Send(objc.RegisterName("setStencilCompareFunction:"), op.compare)
		s.Send(objc.RegisterName("setStencilFailureOperation:"), op.fail)
		s.Send(objc.RegisterName("setDepthFailureOperation:"), op.depthFail)
		s.Send(objc.RegisterName("setDepthStencilPassOperation:"), op.pass)
		s.Send(objc.RegisterName("setReadMask:"), uint32(0xFF))
		s.Send(objc.RegisterName("setWriteMask:"), uint32(0xFF))
		return s
	}
	if front != nil {
		f, b := stencil(front), stencil(back)
		desc.Send(objc.RegisterName("setFrontFaceStencil:"), f)
		desc.Send(objc.RegisterName("setBackFaceStencil:"), b)
		f.Send(objc.RegisterName("release"))
		b.Send(objc.RegisterName("release"))
	}
	return uintptr(d.device.Send(objc.RegisterName("newDepthStencilStateWithDescriptor:"), desc))
}

// depthState returns the tests of kind for the pass in progress.
func (d *Device) depthState(kind pipelineKind) uintptr {
	if !d.pass.depth {
		return d.noDepth
	}
	return d.depthStates[kind]
}

func (d *Device) createPipelines() error {
	d.pipelines[variantPlain] = pipelineSet{samples: 1, depth: true}
	d.pipelines[variantMulti] = pipelineSet{samples: d.samples, depth: true}
	d.pipelines[variantFlat] = pipelineSet{samples: 1}
	for i := range d.pipelines {
		p := &d.pipelines[i]
		if i == variantMulti && d.samples == 1 {
			*p = d.pipelines[variantPlain]
			continue
		}
		var err error
		if p.quad[gpu.BlendSrcOver], err = d.newPipeline(p, d.sdfVertex, d.sdfFragment, gpu.BlendSrcOver, true); err != nil {
			return err
		}
		if !p.depth {
			continue
		}
		if p.noColor, err = d.newPipeline(p, d.sdfVertex, d.sdfFragment, gpu.BlendSrcOver, false); err != nil {
			return err
		}
		if p.fan, err = d.newPipeline(p, d.pathVertex, 0, gpu.BlendSrcOver, false); err != nil {
			return err
		}
	}
	return nil
}

var blendFactors = [...]uint64{
	gpu.FactorZero:        MTLBlendFactorZero,
	gpu.FactorOne:         MTLBlendFactorOne,
	gpu.FactorSrcColor:    MTLBlendFactorSourceColor,
	gpu.FactorInvSrcColor: MTLBlendFactorOneMinusSourceColor,
	gpu.FactorSrcAlpha:    MTLBlendFactorSourceAlpha,
	gpu.FactorInvSrcAlpha: MTLBlendFactorOneMinusSourceAlpha,
	gpu.FactorDstAlpha:    MTLBlendFactorDestinationAlpha,
	gpu.FactorInvDstAlpha: MTLBlendFactorOneMinusDestinationAlpha,
}

// alphaFactor is the factor of the alpha channel: a color stands for its alpha.
func alphaFactor(f gpu.BlendFactor) uint64 {
	switch f {
	case gpu.FactorSrcColor:
		return MTLBlendFactorSourceAlpha
	case gpu.FactorInvSrcColor:
		return MTLBlendFactorOneMinusSourceAlpha
	}
	return blendFactors[f]
}

// newPipeline makes a render pipeline of p. color says whether it writes
// color, with the fixed-function blend of mode; fragment may be zero for a
// pipeline that only marks the stencil.
func (d *Device) newPipeline(p *pipelineSet, vertex, fragment objc.ID, mode gpu.BlendMode, color bool) (uintptr, error) {
	desc := objc.ID(objc.GetClass("MTLRenderPipelineDescriptor")).Send(objc.RegisterName("new"))
	defer desc.Send(objc.RegisterName("release"))
	desc.Send(objc.RegisterName("setVertexFunction:"), vertex)
	if color {
		desc.Send(objc.RegisterName("setFragmentFunction:"), fragment)
	}
	desc.Send(objc.RegisterName("setRasterSampleCount:"), uint64(p.samples))
	attachment := desc.Send(objc.RegisterName("colorAttachments")).Send(objc.RegisterName("objectAtIndexedSubscript:"), uint64(0))
	attachment.Send(objc.RegisterName("setPixelFormat:"), uint64(MTLPixelFormatBGRA8Unorm))
	if color {
		src, dst := mode.Factors()
		attachment.Send(objc.RegisterName("setBlendingEnabled:"), true)
		attachment.Send(objc.RegisterName("setSourceRGBBlendFactor:"), blendFactors[src])
		attachment.Send(objc.RegisterName("setDestinationRGBBlendFactor:"), blendFactors[dst])
		attachment.Send(objc.RegisterName("setRgbBlendOperation:"), uint64(MTLBlendOperationAdd))
		attachment.Send(objc.RegisterName("setSourceAlphaBlendFactor:"), alphaFactor(src))
		attachment.Send(objc.RegisterName("setDestinationAlphaBlendFactor:"), alphaFactor(dst))
		attachment.Send(objc.RegisterName("setAlphaBlendOperation:"), uint64(MTLBlendOperationAdd))
		attachment.Send(objc.RegisterName("setWriteMask:"), uint64(MTLColorWriteMaskAll))
	} else {
		attachment.Send(objc.RegisterName("setWriteMask:"), uint64(MTLColorWriteMaskNone))
	}
	if p.depth {
		desc.Send(objc.RegisterName("setDepthAttachmentPixelFormat:"), uint64(MTLPixelFormatDepth32Float_Stencil8))
		desc.Send(objc.RegisterName("setStencilAttachmentPixelFormat:"), uint64(MTLPixelFormatDepth32Float_Stencil8))
	}
	var failure objc.ID
	pipeline := d.device.Send(objc.RegisterName("newRenderPipelineStateWithDescriptor:error:"), desc, &failure)
	if pipeline == 0 {
		return 0, fmt.Errorf("metal: create pipeline: %w", describe(failure))
	}
	return uintptr(pipeline), nil
}

// quadPipeline returns the pipeline of the quads of mode in the pass in
// progress, made when first needed.
func (d *Device) quadPipeline(mode gpu.BlendMode) (uintptr, error) {
	state := mode.State()
	if int(state) >= gpu.BlendModeCount {
		state = gpu.BlendSrcOver
	}
	p := d.pass
	if p.quad[state] != 0 {
		return p.quad[state], nil
	}
	pipeline, err := d.newPipeline(p, d.sdfVertex, d.sdfFragment, state, true)
	if err != nil {
		return 0, err
	}
	p.quad[state] = pipeline
	return pipeline, nil
}

func (d *Device) releasePipelines() {
	for i := range d.pipelines {
		if i == variantMulti && d.samples == 1 {
			continue
		}
		p := &d.pipelines[i]
		for _, q := range p.quad {
			if q != 0 {
				d.send(q, d.sel.release)
			}
		}
		for _, q := range []uintptr{p.noColor, p.fan} {
			if q != 0 {
				d.send(q, d.sel.release)
			}
		}
	}
	for _, s := range d.depthStates {
		if s != 0 {
			d.send(s, d.sel.release)
		}
	}
	if d.noDepth != 0 {
		d.send(d.noDepth, d.sel.release)
	}
}
