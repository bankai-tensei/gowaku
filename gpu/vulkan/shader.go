//go:build linux || windows

package vulkan

import (
	"encoding/binary"
	"fmt"

	"github.com/bankai-tensei/gowaku/gpu"
)

// spirvMagic starts every SPIR-V module, in the byte order of the machine that
// wrote it; dxc writes little-endian.
const spirvMagic = 0x07230203

// customShader is a custom pixel shader: its module, and the pipelines of its
// quads, which pair it with the SDF vertex shader. They are made per render
// pass, which differ in their sample count, and per blend state, on first use.
// Targets draw with those of the plain pass, which theirs is compatible with.
type customShader struct {
	module    VkShaderModule
	pipelines [passCount][gpu.BlendModeCount]VkPipeline
}

// PrepareShader implements gpu.Backend. It creates the module of s from its
// SPIR-V and, if a surface or target exists, the pipeline of the plain pass with
// source-over, where the driver compiles the shader and would refuse it. See
// docs/ADR/0045-shaders-svg-animated-images.md.
func (d *Device) PrepareShader(s *gpu.Shader) error {
	if _, ok := d.shaders[s]; ok {
		return nil
	}
	// Vulkan leaves code that is not SPIR-V undefined, so that a driver may
	// take it or crash; the magic number at least tells it apart.
	if len(s.SPIRV) < 20 || len(s.SPIRV)%4 != 0 || binary.LittleEndian.Uint32(s.SPIRV) != spirvMagic {
		return fmt.Errorf("vulkan: shader %q has no SPIR-V", s.Name)
	}
	module, err := d.createShader(s.SPIRV)
	if err != nil {
		return fmt.Errorf("vulkan: shader %q: %w", s.Name, err)
	}
	sh := &customShader{module: module}
	if plain := &d.passes[passPlain]; plain.renderPass != 0 {
		steps := pipelineSteps()
		p, err := d.createPipeline(plain, &steps[pipelineQuad], gpu.BlendSrcOver, module)
		if err != nil {
			call(d.vkDestroyShaderModule, uintptr(d.device), uintptr(module), 0)
			return fmt.Errorf("vulkan: shader %q: %w", s.Name, err)
		}
		sh.pipelines[passPlain][gpu.BlendSrcOver] = p
	}
	if d.shaders == nil {
		d.shaders = make(map[*gpu.Shader]*customShader)
	}
	d.shaders[s] = sh
	return nil
}

// shaderPipeline returns the pipeline of sh for the pass of the frame in
// progress and the state of mode, made if it is the first time.
func (d *Device) shaderPipeline(sh *customShader, mode gpu.BlendMode) (VkPipeline, error) {
	pass := passPlain
	if d.pass == &d.passes[passMulti] {
		pass = passMulti
	}
	mode = mode.State()
	if p := sh.pipelines[pass][mode]; p != 0 {
		return p, nil
	}
	steps := pipelineSteps()
	p, err := d.createPipeline(d.pass, &steps[pipelineQuad], mode, sh.module)
	if err != nil {
		return 0, err
	}
	sh.pipelines[pass][mode] = p
	return p, nil
}
