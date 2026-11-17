package gowaku

import (
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/gpu/d3d11"
	"github.com/bankai-tensei/gowaku/platform/win32"
)

// Run opens the window of a and blocks until the last window is closed. The
// calling goroutine becomes the UI goroutine. The returned error reports a
// failed initialization; failures during a frame panic.
func Run(a App) error {
	fonts := loadShaper()
	devices := createDevice(func() (gpu.Backend, error) { return d3d11.New(d3d11.Options{}) })
	p, err := win32.New()
	if err != nil {
		if d := <-devices; d.err == nil {
			d.backend.Destroy()
		}
		return err
	}
	defer p.Destroy()
	return start(p, devices, fonts, a)
}
