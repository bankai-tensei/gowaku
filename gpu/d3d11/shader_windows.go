package d3d11

import (
	"fmt"

	"github.com/bankai-tensei/gowaku/gpu"
)

// PrepareShader implements gpu.Backend. It creates the pixel shader of s from
// its DXBC; the SDF vertex shader goes with it, which the input signature of a
// shader that gowaku-shader made matches. See
// docs/ADR/0045-shaders-svg-animated-images.md.
func (d *Device) PrepareShader(s *gpu.Shader) error {
	if _, ok := d.shaders[s]; ok {
		return nil
	}
	if len(s.DXBC) == 0 {
		return fmt.Errorf("d3d11: shader %q has no DXBC", s.Name)
	}
	ps, err := d.device.createPixelShader(s.DXBC)
	if err != nil {
		return fmt.Errorf("d3d11: shader %q: %w", s.Name, err)
	}
	if d.shaders == nil {
		d.shaders = make(map[*gpu.Shader]*ID3D11PixelShader)
	}
	d.shaders[s] = ps
	return nil
}
