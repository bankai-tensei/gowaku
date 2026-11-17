//go:build darwin && arm64

package metal

import (
	"errors"
	"fmt"

	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/gpu"
)

// customShader is what PrepareShader made of a gpu.Shader: its fragment
// function, and its pipelines, by variant and blend state, made when first
// needed.
type customShader struct {
	library   objc.ID
	fragment  objc.ID
	pipelines [variantCount][gpu.BlendModeCount]uintptr
}

// newCompileOptions returns MTLCompileOptions without fast math, so that the
// shaders compute what those of the other backends do. The caller releases
// them. macOS 15 replaced fastMathEnabled, which it deprecates, with mathMode
// and mathFloatingPointFunctions; an older system only has the former.
func newCompileOptions() objc.ID {
	options := objc.ID(objc.GetClass("MTLCompileOptions")).Send(objc.RegisterName("new"))
	setMathMode := objc.RegisterName("setMathMode:")
	if objc.Send[bool](options, objc.RegisterName("respondsToSelector:"), setMathMode) {
		options.Send(setMathMode, int64(MTLMathModeSafe))
		options.Send(objc.RegisterName("setMathFloatingPointFunctions:"), int64(MTLMathFloatingPointFunctionsPrecise))
		return options
	}
	options.Send(objc.RegisterName("setFastMathEnabled:"), false)
	return options
}

// PrepareShader implements gpu.Backend. The MSL of gowaku-shader is the
// fragment function main0 of spirv-cross, whose inputs carry the locations of
// Varyings in sdf.metal, so it pairs with sdfVertex.
func (d *Device) PrepareShader(s *gpu.Shader) error {
	if d.drawing != nil {
		panic("gowaku: PrepareShader called between BeginFrame and EndFrame")
	}
	if _, ok := d.shaders[s]; ok {
		return nil
	}
	if len(s.MSL) == 0 {
		return fmt.Errorf("metal: shader %q has no MSL", s.Name)
	}
	token := d.pushPool()
	defer d.popPool(token)
	source := nsString(string(s.MSL))
	defer source.Send(objc.RegisterName("release"))
	options := newCompileOptions()
	defer options.Send(objc.RegisterName("release"))
	var failure objc.ID
	library := d.device.Send(objc.RegisterName("newLibraryWithSource:options:error:"), source, options, &failure)
	if library == 0 {
		return fmt.Errorf("metal: compile shader %q: %w", s.Name, describe(failure))
	}
	name := nsString("main0")
	fragment := library.Send(objc.RegisterName("newFunctionWithName:"), name)
	name.Send(objc.RegisterName("release"))
	if fragment == 0 {
		library.Send(objc.RegisterName("release"))
		return fmt.Errorf("metal: shader %q has no fragment function main0", s.Name)
	}
	sh := &customShader{library: library, fragment: fragment}
	// Making the pipeline of source-over now reports a shader whose inputs do
	// not match the vertex shader, as Vulkan does when it prepares.
	p, err := d.newPipeline(&d.pipelines[variantPlain], d.sdfVertex, fragment, gpu.BlendSrcOver, true)
	if err != nil {
		sh.release(d)
		return fmt.Errorf("metal: shader %q: %w", s.Name, err)
	}
	sh.pipelines[variantPlain][gpu.BlendSrcOver] = p
	if d.shaders == nil {
		d.shaders = make(map[*gpu.Shader]*customShader)
	}
	d.shaders[s] = sh
	return nil
}

// shaderPipeline returns the pipeline of sh for the pass in progress and mode.
func (d *Device) shaderPipeline(sh *customShader, mode gpu.BlendMode) (uintptr, error) {
	state := mode.State()
	if int(state) >= gpu.BlendModeCount {
		state = gpu.BlendSrcOver
	}
	variant := -1
	for i := range d.pipelines {
		if &d.pipelines[i] == d.pass {
			variant = i
		}
	}
	if variant < 0 {
		return 0, errors.New("metal: no pass in progress")
	}
	if p := sh.pipelines[variant][state]; p != 0 {
		return p, nil
	}
	p, err := d.newPipeline(d.pass, d.sdfVertex, sh.fragment, state, true)
	if err != nil {
		return 0, err
	}
	sh.pipelines[variant][state] = p
	return p, nil
}

func (sh *customShader) release(d *Device) {
	for i := range sh.pipelines {
		for _, p := range sh.pipelines[i] {
			if p != 0 {
				d.send(p, d.sel.release)
			}
		}
	}
	d.send(uintptr(sh.fragment), d.sel.release)
	d.send(uintptr(sh.library), d.sel.release)
}
