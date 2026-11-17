//go:build darwin && arm64

package shot

import (
	"time"

	"github.com/bankai-tensei/gowaku/gpu/metal"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/cocoa"
)

// open starts the platform and GPU of macOS: AppKit and Metal.
func open(opts *Options) (platform.Platform, device, error) {
	p, err := cocoa.New()
	if err != nil {
		return nil, nil, err
	}
	d, err := metal.New(metal.Options{PresentImmediately: opts.Immediate})
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
