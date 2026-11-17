package gowaku

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/platform"
)

// deviceResult is what the goroutine that creates the GPU device delivers.
type deviceResult struct {
	backend gpu.Backend
	err     error
}

// createDevice runs create on another goroutine, while the caller opens the
// platform and the window. The device has no tie to the goroutine that made
// it, and only the UI goroutine uses it once it has arrived.
func createDevice(create func() (gpu.Backend, error)) <-chan deviceResult {
	done := make(chan deviceResult, 1)
	go func() {
		b, err := create()
		done <- deviceResult{b, err}
	}()
	return done
}

// firstWindow is the native window of an App and its surface, which start
// makes before the fonts have arrived.
type firstWindow struct {
	native        platform.Window
	surface       gpu.Surface
	width, height int
	scale         float32
}

// windowOptions returns the options of the window of a.
func (a *App) windowOptions() WindowOptions {
	return WindowOptions{
		Title: a.Title, Width: a.Width, Height: a.Height, Background: a.Background, OnClose: a.OnClose,
		OnCloseRequest: a.OnCloseRequest, OnDrawError: a.OnDrawError, CustomFrame: a.CustomFrame,
		Icon: a.Icon, MinSize: a.MinSize, MaxSize: a.MaxSize, AlwaysOnTop: a.AlwaysOnTop,
		Fullscreen: a.Fullscreen, Transparent: a.Transparent,
	}
}

// start runs a on p once the device and the fonts have arrived. Neither the
// window nor its surface needs the fonts, so they are made while the fonts
// load, and the window shows its background until the first frame. See
// docs/ADR/0080-startup.md.
func start(p platform.Platform, devices <-chan deviceResult, fonts <-chan shaperResult, a App) error {
	if a.Home == nil {
		panic("gowaku: a window needs a root widget")
	}
	opts := a.windowOptions()
	// The same theme as app.theme, which needs the app that run makes.
	early := &app{themes: a.Theme, mode: a.ThemeMode, brightness: p.Brightness()}
	th := early.theme()
	native, windowErr := p.CreateWindow(nativeOptions(opts, th, a.AppID, nil, nil))
	device := <-devices
	if device.err != nil {
		if windowErr == nil {
			native.Close()
		}
		return device.err
	}
	defer device.backend.Destroy()
	if windowErr != nil {
		return windowErr
	}
	first := &firstWindow{native: native, scale: native.Scale()}
	first.width, first.height = native.PhysicalSize()
	surface, err := device.backend.CreateSurface(native.NativeHandle(), first.width, first.height, first.scale)
	if err != nil {
		native.Close()
		return err
	}
	first.surface = surface
	background, _ := windowBackground(opts.Background, th, native.Transparent())
	if native.Transparent() {
		background = premultiplied(background)
	}
	if err := clearFrame(device.backend, surface, background); err != nil {
		surface.Destroy()
		native.Close()
		return err
	}
	loaded := <-fonts
	if loaded.err != nil {
		surface.Destroy()
		native.Close()
		return loaded.err
	}
	return runWith(p, device.backend, loaded.shaper, a, first)
}

// clearFrame presents a frame of nothing but background, which the window shows
// until the first frame instead of what the OS would. It is also the first
// present of the surface, whose cost leaves the path to the first frame.
func clearFrame(b gpu.Backend, s gpu.Surface, background geom.Color) error {
	s.SetMultisample(false)
	if err := b.BeginFrame(s, background); err != nil {
		return err
	}
	return b.EndFrame(s)
}
