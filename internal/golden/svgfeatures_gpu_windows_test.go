//go:build svgsets && gpu

package golden

import (
	"fmt"
	"image"

	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/gpu/d3d11"
	"github.com/bankai-tensei/gowaku/gpu/vulkan"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/win32"
)

// featGPU draws s with D3D11 or Vulkan, the latter on the fast GPU as the
// goldens do, in a window of the size of the scene.
func featGPU(backend string, s *Scene) (*image.NRGBA, error) {
	p, err := win32.New()
	if err != nil {
		return nil, err
	}
	defer p.Destroy()
	var b Backend
	switch backend {
	case "d3d11":
		d, err := d3d11.New(d3d11.Options{})
		if err != nil {
			return nil, err
		}
		defer d.Destroy()
		b = d
	case "vulkan":
		v, err := vulkan.New(vulkan.Options{Power: gpu.HighPerformance})
		if err != nil {
			return nil, err
		}
		defer v.Destroy()
		b = v
	default:
		return nil, fmt.Errorf("unknown backend %q: want d3d11 or vulkan", backend)
	}
	r, err := NewRenderer(b)
	if err != nil {
		return nil, err
	}
	defer r.Destroy()
	probe, err := p.CreateWindow(platform.WindowOptions{Title: "svgfeatures", Width: 100, Height: 100})
	if err != nil {
		return nil, err
	}
	system := probe.Scale()
	probe.Close()
	w, err := p.CreateWindow(platform.WindowOptions{
		Title: "svgfeatures", Width: float32(s.Width) / system, Height: float32(s.Height) / system,
	})
	if err != nil {
		return nil, err
	}
	defer w.Close()
	surface, err := b.CreateSurface(w.NativeHandle(), s.Width, s.Height, s.Scale)
	if err != nil {
		return nil, err
	}
	defer surface.Destroy()
	return r.Render(s, surface)
}
