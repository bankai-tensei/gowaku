package soft

import (
	"errors"
	"image"

	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/platform"
)

// Surface is a drawable area in memory.
type Surface struct {
	device        *Device
	width, height int
	scale         float32
	// multisample is what the next frame asks for.
	multisample bool
	inFrame     bool
	// canvas is what the frame in progress draws to.
	canvas *canvas
	// presented is the result of the last EndFrame.
	presented *image.NRGBA
}

var _ gpu.Surface = (*Surface)(nil)

// CreateSurface implements gpu.Backend. It ignores h, which can be nil.
func (d *Device) CreateSurface(_ platform.NativeHandle, physicalWidth, physicalHeight int, scale float32) (gpu.Surface, error) {
	if physicalWidth <= 0 || physicalHeight <= 0 {
		return nil, errors.New("soft: surface size is empty")
	}
	d.liveSurfaces++
	return &Surface{device: d, width: physicalWidth, height: physicalHeight, scale: scale, multisample: true}, nil
}

// SetMultisample implements gpu.Surface.
func (s *Surface) SetMultisample(on bool) { s.multisample = on }

// Resize implements gpu.Surface.
func (s *Surface) Resize(physicalWidth, physicalHeight int, scale float32) error {
	if s.inFrame {
		panic("gowaku: Surface.Resize called between BeginFrame and EndFrame")
	}
	if physicalWidth <= 0 || physicalHeight <= 0 {
		return nil
	}
	s.width, s.height, s.scale = physicalWidth, physicalHeight, scale
	return nil
}

// Destroy implements gpu.Surface.
func (s *Surface) Destroy() {
	if s.inFrame {
		s.device.openFrames--
	}
	s.device.liveSurfaces--
	*s = Surface{}
}

// ReadPixels returns the frame drawn so far, resolved and opaque like the frame of
// the D3D11 backend. Call it between Draw and EndFrame.
func (d *Device) ReadPixels(s gpu.Surface) (*image.NRGBA, error) {
	sf := s.(*Surface)
	if !sf.inFrame {
		panic("gowaku: ReadPixels called outside BeginFrame and EndFrame")
	}
	return sf.canvas.resolve(), nil
}

// Frame returns what the last EndFrame on s presented, or nil if there was none.
// Unlike ReadPixels it works outside a frame, which a GPU backend cannot offer. The
// image stays valid after later frames.
func (d *Device) Frame(s gpu.Surface) *image.NRGBA { return s.(*Surface).presented }
