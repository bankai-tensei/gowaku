//go:build linux || windows

package vulkan

import (
	"unsafe"

	"github.com/bankai-tensei/gowaku/gpu"
)

// clipDepthLevels spreads clip levels over the depth range. See
// docs/ADR/0009-paths-and-clips.md.
const clipDepthLevels = 256

// pipelineKind names the state of one step. Direct3D 11 combines these from
// separate state objects, where Vulkan needs a pipeline for each combination.
type pipelineKind uint8

const (
	// pipelineQuad draws instances and tests the clip level.
	pipelineQuad pipelineKind = iota
	pipelineFanNonZero
	pipelineFanEvenOdd
	pipelineCover
	pipelineClipCover
	pipelineClipPop
	pipelineCount
)

// The steps that write color, which have a pipeline per blend mode.
const (
	blendQuad = iota
	blendCover
	blendSteps
)

// framePass is a render pass with the pipelines that go with it. The sample count
// is part of both, so that frames with and without MSAA need one each.
type framePass struct {
	samples    uint32
	renderPass VkRenderPass
	// resume is the same pass, compatible with it, that goes on with what the
	// attachments hold. A blend mode that reads the target ends the pass to
	// copy it, and resumes. See docs/ADR/0044-drawing-extensions.md.
	resume    VkRenderPass
	pipelines [pipelineCount]VkPipeline
	// blended holds the pipelines of the steps that write color for each blend
	// state but that of source-over, made on first use and indexed by
	// gpu.BlendMode.State.
	blended [blendSteps][gpu.BlendModeCount]VkPipeline
}

// The render passes. The multisampled one is made when a frame first needs it.
const (
	passPlain = iota
	passMulti
	passCount
)

// createRenderPass makes what frames without MSAA need. It depends on the format
// of the surfaces, so the first one decides.
func (d *Device) createRenderPass(format uint32) error {
	d.colorFormat = format
	return d.createPass(&d.passes[passPlain], VK_SAMPLE_COUNT_1_BIT)
}

// createPass makes p for the given sample count.
func (d *Device) createPass(p *framePass, samples uint32) error {
	var err error
	if p.renderPass, err = d.newRenderPass(samples, false); err != nil {
		return err
	}
	if p.resume, err = d.newRenderPass(samples, true); err != nil {
		return err
	}
	p.samples = samples
	return d.createPipelines(p)
}

// newRenderPass makes a render pass of frames. Everything is stored, so that a
// resumed pass finds what the one before it drew, the clip levels included.
func (d *Device) newRenderPass(samples uint32, resume bool) (VkRenderPass, error) {
	format := d.colorFormat
	multisampled := samples > 1
	present := uint32(VK_IMAGE_LAYOUT_PRESENT_SRC_KHR)
	color := VkAttachmentDescription{
		format:         format,
		samples:        samples,
		loadOp:         VK_ATTACHMENT_LOAD_OP_CLEAR,
		storeOp:        VK_ATTACHMENT_STORE_OP_STORE,
		stencilLoadOp:  VK_ATTACHMENT_LOAD_OP_DONT_CARE,
		stencilStoreOp: VK_ATTACHMENT_STORE_OP_DONT_CARE,
		finalLayout:    present,
	}
	if multisampled {
		color.finalLayout = VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL
	}
	depth := VkAttachmentDescription{
		format:         d.depthFormat,
		samples:        samples,
		loadOp:         VK_ATTACHMENT_LOAD_OP_CLEAR,
		storeOp:        VK_ATTACHMENT_STORE_OP_STORE,
		stencilLoadOp:  VK_ATTACHMENT_LOAD_OP_CLEAR,
		stencilStoreOp: VK_ATTACHMENT_STORE_OP_STORE,
		finalLayout:    VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL,
	}
	resolve := VkAttachmentDescription{
		format:         format,
		samples:        VK_SAMPLE_COUNT_1_BIT,
		loadOp:         VK_ATTACHMENT_LOAD_OP_DONT_CARE,
		storeOp:        VK_ATTACHMENT_STORE_OP_STORE,
		stencilLoadOp:  VK_ATTACHMENT_LOAD_OP_DONT_CARE,
		stencilStoreOp: VK_ATTACHMENT_STORE_OP_DONT_CARE,
		finalLayout:    present,
	}
	if resume {
		// The copy leaves the color in this layout, and the depth stays in
		// the one the pass before ended with.
		color.loadOp, color.initialLayout = VK_ATTACHMENT_LOAD_OP_LOAD, VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL
		depth.loadOp, depth.stencilLoadOp = VK_ATTACHMENT_LOAD_OP_LOAD, VK_ATTACHMENT_LOAD_OP_LOAD
		depth.initialLayout = VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL
		resolve.initialLayout = VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL
	}
	attachments := []VkAttachmentDescription{color, depth}
	colorRef := VkAttachmentReference{0, VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL}
	depthRef := VkAttachmentReference{1, VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL}
	subpass := VkSubpassDescription{
		pipelineBindPoint:       VK_PIPELINE_BIND_POINT_GRAPHICS,
		colorAttachmentCount:    1,
		pColorAttachments:       &colorRef,
		pDepthStencilAttachment: &depthRef,
	}
	if multisampled {
		attachments = append(attachments, resolve)
		subpass.pResolveAttachments = &VkAttachmentReference{2, VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL}
	}
	dependency := frameDependency()
	info := VkRenderPassCreateInfo{
		sType:           VK_STRUCTURE_TYPE_RENDER_PASS_CREATE_INFO,
		attachmentCount: uint32(len(attachments)),
		pAttachments:    &attachments[0],
		subpassCount:    1,
		pSubpasses:      &subpass,
		dependencyCount: 1,
		pDependencies:   &dependency,
	}
	var pass VkRenderPass
	r := callResult(d.vkCreateRenderPass, uintptr(d.device), ptr(&info), 0, ptr(&pass))
	return pass, failure(r, "create render pass")
}

// frameDependency is the one dependency of every render pass, which must be the
// same for their pipelines to be interchangeable. The multisampled color and the
// depth buffer belong to the surface, not to a frame, so that a frame must wait
// for the writes of the one before it.
func frameDependency() VkSubpassDependency {
	const stages = VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT | VK_PIPELINE_STAGE_EARLY_FRAGMENT_TESTS_BIT | VK_PIPELINE_STAGE_LATE_FRAGMENT_TESTS_BIT
	return VkSubpassDependency{
		srcSubpass:    VK_SUBPASS_EXTERNAL,
		srcStageMask:  stages,
		dstStageMask:  stages,
		srcAccessMask: VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT | VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_WRITE_BIT,
		dstAccessMask: VK_ACCESS_COLOR_ATTACHMENT_READ_BIT | VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT |
			VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_READ_BIT | VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_WRITE_BIT,
	}
}

// depthState is the part of a pipeline that differs between its steps.
type depthState struct {
	compare     uint32
	write       uint32
	stencil     bool
	front, back VkStencilOpState
}

func mark(op uint32) VkStencilOpState {
	return VkStencilOpState{
		failOp:      VK_STENCIL_OP_KEEP,
		depthFailOp: VK_STENCIL_OP_KEEP,
		passOp:      op,
		compareOp:   VK_COMPARE_OP_ALWAYS,
		compareMask: 0xFF,
		writeMask:   0xFF,
	}
}

// take makes a cover read the stencil and leave it at zero for the next shape.
func take() VkStencilOpState {
	return VkStencilOpState{
		failOp:      VK_STENCIL_OP_KEEP,
		depthFailOp: VK_STENCIL_OP_ZERO,
		passOp:      VK_STENCIL_OP_ZERO,
		compareOp:   VK_COMPARE_OP_NOT_EQUAL,
		compareMask: 0xFF,
		writeMask:   0xFF,
	}
}

// pipelineStep is what one pipelineKind stands for.
type pipelineStep struct {
	fan   bool
	color bool
	depth depthState
}

func pipelineSteps() [pipelineCount]pipelineStep {
	invert := mark(VK_STENCIL_OP_INVERT)
	return [pipelineCount]pipelineStep{
		pipelineQuad: {color: true, depth: depthState{compare: VK_COMPARE_OP_EQUAL}},
		pipelineFanNonZero: {fan: true, depth: depthState{
			compare: VK_COMPARE_OP_EQUAL, stencil: true,
			front: mark(VK_STENCIL_OP_INCREMENT_AND_WRAP), back: mark(VK_STENCIL_OP_DECREMENT_AND_WRAP),
		}},
		pipelineFanEvenOdd: {fan: true, depth: depthState{compare: VK_COMPARE_OP_EQUAL, stencil: true, front: invert, back: invert}},
		pipelineCover:      {color: true, depth: depthState{compare: VK_COMPARE_OP_EQUAL, stencil: true, front: take(), back: take()}},
		pipelineClipCover:  {depth: depthState{compare: VK_COMPARE_OP_GREATER, write: 1, stencil: true, front: take(), back: take()}},
		pipelineClipPop:    {depth: depthState{compare: VK_COMPARE_OP_LESS, write: 1}},
	}
}

func (d *Device) createPipelines(p *framePass) error {
	steps := pipelineSteps()
	for i := range steps {
		pipeline, err := d.createPipeline(p, &steps[i], gpu.BlendSrcOver, d.sdfFragment)
		if err != nil {
			return err
		}
		p.pipelines[i] = pipeline
	}
	return nil
}

// blendFactors maps the factors of gpu to those of Vulkan.
var blendFactors = [...]uint32{
	gpu.FactorZero:        VK_BLEND_FACTOR_ZERO,
	gpu.FactorOne:         VK_BLEND_FACTOR_ONE,
	gpu.FactorSrcColor:    VK_BLEND_FACTOR_SRC_COLOR,
	gpu.FactorInvSrcColor: VK_BLEND_FACTOR_ONE_MINUS_SRC_COLOR,
	gpu.FactorSrcAlpha:    VK_BLEND_FACTOR_SRC_ALPHA,
	gpu.FactorInvSrcAlpha: VK_BLEND_FACTOR_ONE_MINUS_SRC_ALPHA,
	gpu.FactorDstAlpha:    VK_BLEND_FACTOR_DST_ALPHA,
	gpu.FactorInvDstAlpha: VK_BLEND_FACTOR_ONE_MINUS_DST_ALPHA,
}

// alphaFactor returns the factor of the alpha channel that stands for f, as
// Direct3D 11 requires and Vulkan computes anyway.
func alphaFactor(f gpu.BlendFactor) uint32 {
	switch f {
	case gpu.FactorSrcColor:
		return VK_BLEND_FACTOR_SRC_ALPHA
	case gpu.FactorInvSrcColor:
		return VK_BLEND_FACTOR_ONE_MINUS_SRC_ALPHA
	}
	return blendFactors[f]
}

// createPipeline makes the pipeline of step in p, which blends by mode if the
// step writes color, with fragment as the pixel shader of quads.
func (d *Device) createPipeline(p *framePass, step *pipelineStep, mode gpu.BlendMode, fragment VkShaderModule) (VkPipeline, error) {
	name := cstring("main")
	stage := func(kind uint32, module VkShaderModule) VkPipelineShaderStageCreateInfo {
		return VkPipelineShaderStageCreateInfo{
			sType:  VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO,
			stage:  kind,
			module: module,
			pName:  name,
		}
	}
	quadStages := []VkPipelineShaderStageCreateInfo{
		stage(VK_SHADER_STAGE_VERTEX_BIT, d.sdfVertex),
		stage(VK_SHADER_STAGE_FRAGMENT_BIT, fragment),
	}
	if fragment == d.sdfFragment && d.scissorInShader && p.samples > 1 {
		// SCISSOR_IN_SHADER, constant 0 of sdf.ps.hlsl, a 32-bit bool.
		on := uint32(1)
		entry := VkSpecializationMapEntry{size: unsafe.Sizeof(on)}
		quadStages[1].pSpecializationInfo = unsafe.Pointer(&VkSpecializationInfo{
			mapEntryCount: 1, pMapEntries: &entry, dataSize: entry.size, pData: unsafe.Pointer(&on),
		})
	}
	// The triangles of a shape have no color, so no fragment stage goes with them.
	fanStages := []VkPipelineShaderStageCreateInfo{stage(VK_SHADER_STAGE_VERTEX_BIT, d.pathVertex)}

	var in gpu.Instance
	attribute := func(location, format uint32, offset uintptr) VkVertexInputAttributeDescription {
		return VkVertexInputAttributeDescription{location: location, format: format, offset: uint32(offset)}
	}
	quadBinding := VkVertexInputBindingDescription{0, uint32(instanceStride), VK_VERTEX_INPUT_RATE_INSTANCE}
	quadAttributes := []VkVertexInputAttributeDescription{
		attribute(0, VK_FORMAT_R32G32B32A32_SFLOAT, unsafe.Offsetof(in.Rect)),
		attribute(1, VK_FORMAT_R32G32B32A32_SFLOAT, unsafe.Offsetof(in.Radii)),
		attribute(2, VK_FORMAT_R32G32B32A32_SFLOAT, unsafe.Offsetof(in.Color)),
		attribute(3, VK_FORMAT_R32G32B32A32_SFLOAT, unsafe.Offsetof(in.EndColor)),
		attribute(4, VK_FORMAT_R32G32B32A32_SFLOAT, unsafe.Offsetof(in.BorderColor)),
		attribute(5, VK_FORMAT_R32G32B32A32_SFLOAT, unsafe.Offsetof(in.GradientStart)),
		attribute(6, VK_FORMAT_R32G32_SFLOAT, unsafe.Offsetof(in.BorderWidth)),
		attribute(7, VK_FORMAT_R32G32_UINT, unsafe.Offsetof(in.Kind)),
		attribute(8, VK_FORMAT_R32G32B32A32_SFLOAT, unsafe.Offsetof(in.MidColors)),
		attribute(9, VK_FORMAT_R32G32B32A32_SFLOAT, unsafe.Offsetof(in.MidColors)+16),
		attribute(10, VK_FORMAT_R32G32B32A32_SFLOAT, unsafe.Offsetof(in.StopOffsets)),
		attribute(11, VK_FORMAT_R32G32B32A32_SFLOAT, unsafe.Offsetof(in.Transform)),
		attribute(12, VK_FORMAT_R32G32_SFLOAT, unsafe.Offsetof(in.Transform)+16),
		attribute(13, VK_FORMAT_R32G32B32A32_SFLOAT, unsafe.Offsetof(in.GradientMatrix)),
	}
	quadInput := VkPipelineVertexInputStateCreateInfo{
		sType:                           VK_STRUCTURE_TYPE_PIPELINE_VERTEX_INPUT_STATE_CREATE_INFO,
		vertexBindingDescriptionCount:   1,
		pVertexBindingDescriptions:      &quadBinding,
		vertexAttributeDescriptionCount: uint32(len(quadAttributes)),
		pVertexAttributeDescriptions:    &quadAttributes[0],
	}
	fanBinding := VkVertexInputBindingDescription{1, uint32(vertexStride), VK_VERTEX_INPUT_RATE_VERTEX}
	fanAttribute := VkVertexInputAttributeDescription{location: 0, binding: 1, format: VK_FORMAT_R32G32_SFLOAT}
	fanInput := VkPipelineVertexInputStateCreateInfo{
		sType:                           VK_STRUCTURE_TYPE_PIPELINE_VERTEX_INPUT_STATE_CREATE_INFO,
		vertexBindingDescriptionCount:   1,
		pVertexBindingDescriptions:      &fanBinding,
		vertexAttributeDescriptionCount: 1,
		pVertexAttributeDescriptions:    &fanAttribute,
	}
	quadAssembly := VkPipelineInputAssemblyStateCreateInfo{
		sType:    VK_STRUCTURE_TYPE_PIPELINE_INPUT_ASSEMBLY_STATE_CREATE_INFO,
		topology: VK_PRIMITIVE_TOPOLOGY_TRIANGLE_STRIP,
	}
	fanAssembly := VkPipelineInputAssemblyStateCreateInfo{
		sType:    VK_STRUCTURE_TYPE_PIPELINE_INPUT_ASSEMBLY_STATE_CREATE_INFO,
		topology: VK_PRIMITIVE_TOPOLOGY_TRIANGLE_LIST,
	}
	viewport := VkPipelineViewportStateCreateInfo{
		sType:         VK_STRUCTURE_TYPE_PIPELINE_VIEWPORT_STATE_CREATE_INFO,
		viewportCount: 1,
		scissorCount:  1,
	}
	raster := VkPipelineRasterizationStateCreateInfo{
		sType:       VK_STRUCTURE_TYPE_PIPELINE_RASTERIZATION_STATE_CREATE_INFO,
		polygonMode: VK_POLYGON_MODE_FILL,
		cullMode:    VK_CULL_MODE_NONE,
		frontFace:   VK_FRONT_FACE_CLOCKWISE,
		lineWidth:   1,
	}
	multisample := VkPipelineMultisampleStateCreateInfo{
		sType:                VK_STRUCTURE_TYPE_PIPELINE_MULTISAMPLE_STATE_CREATE_INFO,
		rasterizationSamples: p.samples,
	}
	// The pixel shader outputs premultiplied colors. They stay sRGB and are
	// composited without linearizing (docs/ADR/0005-sdf-pipeline.md). A blend
	// mode other than source-over changes the factors.
	src, dst := mode.Factors()
	blendOn := VkPipelineColorBlendAttachmentState{
		blendEnable:         1,
		srcColorBlendFactor: blendFactors[src],
		dstColorBlendFactor: blendFactors[dst],
		colorBlendOp:        VK_BLEND_OP_ADD,
		srcAlphaBlendFactor: alphaFactor(src),
		dstAlphaBlendFactor: alphaFactor(dst),
		alphaBlendOp:        VK_BLEND_OP_ADD,
		colorWriteMask:      VK_COLOR_COMPONENT_ALL,
	}
	blendOff := VkPipelineColorBlendAttachmentState{} // a write mask of zero
	blend := func(b *VkPipelineColorBlendAttachmentState) *VkPipelineColorBlendStateCreateInfo {
		return &VkPipelineColorBlendStateCreateInfo{
			sType:           VK_STRUCTURE_TYPE_PIPELINE_COLOR_BLEND_STATE_CREATE_INFO,
			attachmentCount: 1,
			pAttachments:    b,
		}
	}
	states := []uint32{VK_DYNAMIC_STATE_VIEWPORT, VK_DYNAMIC_STATE_SCISSOR}
	dynamic := VkPipelineDynamicStateCreateInfo{
		sType:             VK_STRUCTURE_TYPE_PIPELINE_DYNAMIC_STATE_CREATE_INFO,
		dynamicStateCount: uint32(len(states)),
		pDynamicStates:    &states[0],
	}
	depth := VkPipelineDepthStencilStateCreateInfo{
		sType:            VK_STRUCTURE_TYPE_PIPELINE_DEPTH_STENCIL_STATE_CREATE_INFO,
		depthTestEnable:  1,
		depthWriteEnable: step.depth.write,
		depthCompareOp:   step.depth.compare,
		front:            step.depth.front,
		back:             step.depth.back,
	}
	if step.depth.stencil {
		depth.stencilTestEnable = 1
	}
	info := VkGraphicsPipelineCreateInfo{
		sType:               VK_STRUCTURE_TYPE_GRAPHICS_PIPELINE_CREATE_INFO,
		stageCount:          uint32(len(quadStages)),
		pStages:             &quadStages[0],
		pVertexInputState:   &quadInput,
		pInputAssemblyState: &quadAssembly,
		pViewportState:      &viewport,
		pRasterizationState: &raster,
		pMultisampleState:   &multisample,
		pDepthStencilState:  &depth,
		pColorBlendState:    blend(&blendOff),
		pDynamicState:       &dynamic,
		layout:              d.pipeLayout,
		renderPass:          p.renderPass,
		basePipelineIndex:   -1,
	}
	if step.fan {
		info.stageCount, info.pStages = uint32(len(fanStages)), &fanStages[0]
		info.pVertexInputState, info.pInputAssemblyState = &fanInput, &fanAssembly
	}
	if step.color {
		info.pColorBlendState = blend(&blendOn)
	}
	var pipeline VkPipeline
	r := callResult(d.vkCreateGraphicsPipelines, uintptr(d.device), 0, 1, ptr(&info), 0, ptr(&pipeline))
	return pipeline, failure(r, "create pipeline")
}

// pipeline returns the pipeline of kind in the pass of the frame in progress,
// for mode if kind writes color. blendPipeline must have made it.
func (d *Device) pipeline(kind pipelineKind, mode gpu.BlendMode) VkPipeline {
	if mode = mode.State(); mode != gpu.BlendSrcOver {
		switch kind {
		case pipelineQuad:
			return d.pass.blended[blendQuad][mode]
		case pipelineCover:
			return d.pass.blended[blendCover][mode]
		}
	}
	return d.pass.pipelines[kind]
}

// blendPipeline makes sure that the pipeline of the color step of c exists in
// the pass of the frame in progress.
func (d *Device) blendPipeline(c *gpu.DrawCall) error {
	kind, slot := pipelineQuad, blendQuad
	if c.Kind == gpu.DrawFill {
		kind, slot = pipelineCover, blendCover
	}
	mode := c.Blend.State()
	if c.Shader != nil || mode == gpu.BlendSrcOver || int(mode) >= gpu.BlendModeCount || d.pass.blended[slot][mode] != 0 {
		// A custom shader has pipelines of its own; see shaderPipeline.
		return nil
	}
	steps := pipelineSteps()
	pipeline, err := d.createPipeline(d.pass, &steps[kind], mode, d.sdfFragment)
	if err != nil {
		return err
	}
	d.pass.blended[slot][mode] = pipeline
	return nil
}

// draw runs one call. See docs/ADR/0009-paths-and-clips.md for the steps. A
// custom shader draws with the pipeline custom.
func (d *Device) draw(f *frame, c *gpu.DrawCall, custom VkPipeline) {
	level := int(c.ClipLevel)
	switch c.Kind {
	case gpu.DrawQuads:
		pipeline := custom
		if pipeline == 0 {
			pipeline = d.pipeline(pipelineQuad, c.Blend)
		}
		d.use(f, pipeline, level)
		d.quads(f, c)
	case gpu.DrawFill:
		d.fan(f, c, level)
		d.use(f, d.pipeline(pipelineCover, c.Blend), level)
		d.quads(f, c)
	case gpu.DrawClipPush:
		d.fan(f, c, level)
		d.use(f, d.pass.pipelines[pipelineClipCover], level+1)
		d.quads(f, c)
	case gpu.DrawClipPop:
		d.use(f, d.pass.pipelines[pipelineClipPop], level)
		d.quads(f, c)
	}
}

func (d *Device) quads(f *frame, c *gpu.DrawCall) {
	d.hot(d.vkCmdDraw, uintptr(f.commands), quadVertices, uintptr(c.Count), 0, uintptr(c.First))
}

// fan marks the shape of c in the stencil buffer.
func (d *Device) fan(f *frame, c *gpu.DrawCall, level int) {
	kind := pipelineFanNonZero
	if c.EvenOdd {
		kind = pipelineFanEvenOdd
	}
	d.use(f, d.pass.pipelines[kind], level)
	d.hot(d.vkCmdDraw, uintptr(f.commands), uintptr(c.VertexCount), 1, uintptr(c.VertexFirst), 0)
}

// use binds what differs from the last call.
func (d *Device) use(f *frame, pipeline VkPipeline, level int) {
	if pipeline != d.bound {
		d.bound = pipeline
		d.hot(d.vkCmdBindPipeline, uintptr(f.commands), VK_PIPELINE_BIND_POINT_GRAPHICS, uintptr(pipeline))
		if d.scissorInShader {
			d.hot(d.vkCmdPushConstants, uintptr(f.commands), uintptr(d.pipeLayout), VK_SHADER_STAGE_FRAGMENT_BIT, 0,
				unsafe.Sizeof(d.scissorBounds), ptr(&d.scissorBounds))
		}
	}
	if level != d.boundLevel {
		d.boundLevel = level
		// Everything drawn has z = 0, so a viewport with no depth range puts it
		// at the depth of the level, with no shader involved.
		z := float32(level) / clipDepthLevels
		d.viewport = d.surfaceSize
		d.viewport.minDepth, d.viewport.maxDepth = z, z
		d.hot(d.vkCmdSetViewport, uintptr(f.commands), 0, 1, ptr(&d.viewport))
	}
}
