package soft

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

// adapterName is what AdapterName reports.
const adapterName = "gowaku CPU reference"

// Device is the CPU implementation of gpu.Backend. Like the GPU backends, it
// belongs to the goroutine that calls it.
type Device struct {
	liveSurfaces int
	// openFrames counts the surfaces between BeginFrame and EndFrame.
	openFrames int
}

var _ gpu.Backend = (*Device)(nil)

// New creates a device. It cannot fail, because it needs nothing from the system.
func New() *Device { return &Device{} }

// AdapterName returns a constant that identifies this backend.
func (d *Device) AdapterName() string { return adapterName }

// BeginFrame implements gpu.Backend.
func (d *Device) BeginFrame(s gpu.Surface, background geom.Color) error {
	sf := s.(*Surface)
	if sf.inFrame {
		panic("gowaku: BeginFrame called twice without EndFrame")
	}
	sf.inFrame = true
	d.openFrames++
	samples := 1
	if sf.multisample {
		samples = multisampleCount
	}
	sf.canvas = newCanvas(sf.width, sf.height, samples, sf.scale)
	sf.canvas.fill(background)
	return nil
}

// Draw implements gpu.Backend.
func (d *Device) Draw(s gpu.Surface, batch *gpu.Batch) error {
	sf := s.(*Surface)
	if !sf.inFrame {
		panic("gowaku: Draw called outside BeginFrame and EndFrame")
	}
	sf.canvas.draw(batch)
	return nil
}

// EndFrame implements gpu.Backend. What it presents stays readable through Frame.
func (d *Device) EndFrame(s gpu.Surface) error {
	sf := s.(*Surface)
	if !sf.inFrame {
		panic("gowaku: EndFrame called without BeginFrame")
	}
	sf.inFrame = false
	d.openFrames--
	sf.presented = sf.canvas.resolve()
	sf.canvas = nil
	return nil
}

// Destroy implements gpu.Backend.
func (d *Device) Destroy() {
	if d.liveSurfaces != 0 {
		panic("gowaku: Device.Destroy called while surfaces are alive")
	}
}
