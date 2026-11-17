package shot

import (
	"time"

	"github.com/bankai-tensei/gowaku/gpu/vulkan"
	"github.com/bankai-tensei/gowaku/internal/linuxwin"
	"github.com/bankai-tensei/gowaku/platform"
)

// open starts the platform and GPU of Linux, which is always Vulkan.
func open(opts *Options) (platform.Platform, device, error) {
	p, err := linuxwin.Open()
	if err != nil {
		return nil, nil, err
	}
	d, err := vulkan.New(vulkan.Options{Power: opts.Power, PresentImmediately: opts.Immediate})
	if err != nil {
		p.Destroy()
		return nil, nil, err
	}
	return p, d, nil
}

// origin is where clock counts from.
var origin = time.Now()

// clock is a monotonic time fine enough to time parts of a frame.
func clock() time.Duration { return time.Since(origin) }
