package shot

import (
	"time"

	"github.com/bankai-tensei/gowaku/gpu/d3d11"
	"github.com/bankai-tensei/gowaku/gpu/vulkan"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/win32"
)

// open starts the platform and GPU of Windows: D3D11, or Vulkan if asked.
func open(opts *Options) (platform.Platform, device, error) {
	p, err := win32.New()
	if err != nil {
		return nil, nil, err
	}
	var d device
	if opts.Vulkan {
		d, err = vulkan.New(vulkan.Options{Power: opts.Power, PresentImmediately: opts.Immediate})
	} else {
		d, err = d3d11.New(d3d11.Options{Power: opts.Power, PresentImmediately: opts.Immediate})
	}
	if err != nil {
		p.Destroy()
		return nil, nil, err
	}
	return p, d, nil
}

// clock is a monotonic time fine enough to time parts of a frame: time.Now
// steps by about half a millisecond on Windows.
func clock() time.Duration { return win32.PreciseTime() }
