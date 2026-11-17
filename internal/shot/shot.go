// Package shot runs a widget tree outside the event loop of gowaku.Run, on a
// real window and GPU, for samples that take screenshots or measure frames.
package shot

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"os"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/render"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

// device is what a harness needs of the GPU backend of the OS.
type device interface {
	gpu.Backend
	AdapterName() string
	ReadPixels(gpu.Surface) (*image.NRGBA, error)
}

// Options configures Open.
type Options struct {
	Width, Height float32
	Background    geom.Color
	// Immediate draws without waiting for vertical blank.
	Immediate bool
	// Multisample draws every frame multisampled, whether it needs to or not.
	Multisample bool
	// Theme, if set, goes around the root with an Overlay, as Run does, and
	// gives the Background.
	Theme *theme.Theme
	// Vulkan selects Vulkan on Windows, where D3D11 is the default, and Power
	// the GPU of a hybrid machine.
	Vulkan bool
	Power  gpu.PowerPreference
}

// Harness is a window with its tree.
type Harness struct {
	// Tree is the tree of the root widget.
	Tree *core.Tree
	// Shaper serves the tree.
	Shaper *text.Shaper
	// AdapterName is the GPU in use.
	AdapterName string

	opts      Options
	platform  platform.Platform
	device    device
	window    platform.Window
	surface   gpu.Surface
	resources *render.Resources
	canvas    paint.Canvas
	batch     gpu.Batch
	size      geom.Size
	start     time.Time
}

// DefaultTheme returns the light half of the default theme: screenshots should
// not depend on the mode of the OS they were taken on.
func DefaultTheme() *theme.Theme { return theme.Set{}.For(theme.BrightnessLight) }

// Open creates the window and tree of root.
func Open(root core.Widget, opts Options) (*Harness, error) {
	p, device, err := open(&opts)
	if err != nil {
		return nil, err
	}
	shaper, err := text.NewSystemShaper()
	if err != nil {
		device.Destroy()
		p.Destroy()
		return nil, err
	}
	w, err := p.CreateWindow(platform.WindowOptions{Title: "gowaku", Width: opts.Width, Height: opts.Height})
	if err != nil {
		device.Destroy()
		p.Destroy()
		return nil, err
	}
	width, height := w.PhysicalSize()
	surface, err := device.CreateSurface(w.NativeHandle(), width, height, w.Scale())
	if err != nil {
		w.Close()
		device.Destroy()
		p.Destroy()
		return nil, err
	}
	if opts.Theme != nil {
		root = basic.Theme{Data: opts.Theme, Child: overlay.Overlay{Child: root}}
		opts.Background = opts.Theme.Color.Canvas
		w.SetDarkTitleBar(opts.Theme.Brightness == theme.BrightnessDark)
	}
	return &Harness{
		Tree:        core.NewTree(core.Inherited[*text.Shaper]{Value: shaper, Child: root}),
		Shaper:      shaper,
		AdapterName: device.AdapterName(),
		opts:        opts,
		platform:    p,
		device:      device,
		window:      w,
		surface:     surface,
		resources:   render.NewResources(text.NewAtlas(shaper), device),
		size:        geom.Size{W: float32(width) / w.Scale(), H: float32(height) / w.Scale()},
		start:       time.Now(),
	}, nil
}

// Close releases everything.
func (h *Harness) Close() {
	h.Tree.Dispose()
	h.resources.Destroy()
	h.surface.Destroy()
	h.window.Close()
	h.device.Destroy()
	h.platform.Destroy()
}

// timing is what a backend offers to measure frames on the GPU.
type timing interface {
	EnableGPUTiming() error
	GPUFrameTimes([]time.Duration) []time.Duration
}

// EnableGPUTiming makes the backend time each frame on the GPU.
func (h *Harness) EnableGPUTiming() error {
	t, ok := h.device.(timing)
	if !ok {
		return errors.New("shot: the backend cannot time frames")
	}
	return t.EnableGPUTiming()
}

// GPUFrameTimes appends the GPU times of the frames finished since the last
// call to times. Frames the backend could not measure, which it reports as 0,
// are left out.
func (h *Harness) GPUFrameTimes(times []time.Duration) []time.Duration {
	t, ok := h.device.(timing)
	if !ok {
		return times
	}
	n := len(times)
	times = t.GPUFrameTimes(times)
	kept := times[:n]
	for _, d := range times[n:] {
		if d > 0 {
			kept = append(kept, d)
		}
	}
	return kept
}

// PhysicalSize returns the size of the window in physical pixels.
func (h *Harness) PhysicalSize() (width, height int) { return h.window.PhysicalSize() }

// Pump handles what the OS has sent, so that the window stays responsive.
func (h *Harness) Pump() {
	for h.platform.WaitEvent(0).Kind != platform.EventNone {
		continue
	}
}

// encode runs everything of a frame up to the GPU.
func (h *Harness) encode() error {
	h.Tree.Tick(time.Since(h.start))
	h.Tree.Build()
	h.Tree.Layout(geom.Tight(h.size))
	for h.Tree.NeedsBuild() {
		h.Tree.Build()
		h.Tree.Layout(geom.Tight(h.size))
	}
	h.canvas.Reset()
	h.Tree.Paint(&h.canvas)
	render.Encode(h.canvas.DisplayList(), h.resources, h.window.Scale(), &h.batch)
	if err := h.resources.Finish(&h.batch); err != nil {
		return err
	}
	// A sample should not hide what it failed to draw.
	return h.resources.DrawError()
}

func (h *Harness) background() geom.Color {
	if h.opts.Background == (geom.Color{}) {
		return geom.Hex(0xffffff)
	}
	return h.opts.Background
}

// Settle draws frames until nothing moves any more, sleeping tickers aside.
func (h *Harness) Settle() error {
	for {
		if _, err := h.Frame(); err != nil {
			return err
		}
		h.Pump()
		if !h.Tree.Ticking() {
			return nil
		}
	}
}

// Frame draws and presents one frame. It returns the instances drawn.
func (h *Harness) Frame() (instances int, err error) {
	if err := h.encode(); err != nil {
		return 0, err
	}
	h.surface.SetMultisample(h.batch.Multisampled() || h.opts.Multisample)
	if err := h.device.BeginFrame(h.surface, h.background()); err != nil {
		return 0, err
	}
	if err := h.device.Draw(h.surface, &h.batch); err != nil {
		return 0, err
	}
	return len(h.batch.Instances), h.device.EndFrame(h.surface)
}

// FrameTimes is how long the parts of one frame took on the CPU.
type FrameTimes struct {
	// Encode runs the tree, from Tick to Encode and Finish, which can draw
	// layers to their pages.
	Encode time.Duration
	// Begin is BeginFrame, which can wait for a buffer of the swap chain or
	// for the GPU to finish the frame before last.
	Begin time.Duration
	// Draw records and submits the batch.
	Draw time.Duration
	// Present is EndFrame, which can wait for the GPU.
	Present time.Duration
}

// Now returns the clock of TimedFrame, a monotonic time finer than time.Now on
// Windows. Only differences between readings mean anything.
func Now() time.Duration { return clock() }

// TimedFrame draws and presents one frame like Frame, and says how long its parts
// took, with a clock finer than time.Now on Windows.
func (h *Harness) TimedFrame() (FrameTimes, error) {
	var ft FrameTimes
	start := clock()
	if err := h.encode(); err != nil {
		return ft, err
	}
	encoded := clock()
	h.surface.SetMultisample(h.batch.Multisampled() || h.opts.Multisample)
	if err := h.device.BeginFrame(h.surface, h.background()); err != nil {
		return ft, err
	}
	begun := clock()
	if err := h.device.Draw(h.surface, &h.batch); err != nil {
		return ft, err
	}
	drawn := clock()
	err := h.device.EndFrame(h.surface)
	ft.Encode, ft.Begin, ft.Draw, ft.Present = encoded-start, begun-encoded, drawn-begun, clock()-drawn
	return ft, err
}

// Screenshot draws one frame and writes it to path as a PNG.
func (h *Harness) Screenshot(path string) error {
	if err := h.encode(); err != nil {
		return err
	}
	h.surface.SetMultisample(h.batch.Multisampled() || h.opts.Multisample)
	if err := h.device.BeginFrame(h.surface, h.background()); err != nil {
		return err
	}
	if err := h.device.Draw(h.surface, &h.batch); err != nil {
		return err
	}
	img, err := h.device.ReadPixels(h.surface)
	if err != nil {
		return err
	}
	if err := h.device.EndFrame(h.surface); err != nil {
		return err
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		return err
	}
	return os.WriteFile(path, encoded.Bytes(), 0o644)
}
