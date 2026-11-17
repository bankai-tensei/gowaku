package gowaku

import (
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/gpu/vulkan"
	"github.com/bankai-tensei/gowaku/internal/linuxwin"
)

// Run opens the window of a and blocks until the last window is closed. The
// calling goroutine becomes the UI goroutine. The returned error reports a
// failed initialization; failures during a frame panic.
func Run(a App) error {
	fonts := loadShaper()
	devices := createDevice(func() (gpu.Backend, error) { return vulkan.New(vulkan.Options{}) })
	p, err := linuxwin.Open()
	if err != nil {
		if d := <-devices; d.err == nil {
			d.backend.Destroy()
		}
		return err
	}
	defer p.Destroy()
	return start(p, devices, fonts, a)
}
