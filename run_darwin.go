//go:build darwin && arm64

package gowaku

import (
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/gpu/metal"
	"github.com/bankai-tensei/gowaku/platform/cocoa"
)

// Run opens the window of a and blocks until the last window is closed. The
// calling goroutine becomes the UI goroutine; on macOS it must be the main
// goroutine, which AppKit requires. The returned error reports a failed
// initialization; failures during a frame panic.
func Run(a App) error {
	fonts := loadShaper()
	devices := createDevice(func() (gpu.Backend, error) { return metal.New(metal.Options{}) })
	p, err := cocoa.New()
	if err != nil {
		if d := <-devices; d.err == nil {
			d.backend.Destroy()
		}
		return err
	}
	defer p.Destroy()
	return start(p, devices, fonts, a)
}
