//go:build gpu

package d3d11

import (
	"image"
	"image/color"
	"math"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/win32"
	"github.com/bankai-tensei/gowaku/render"
)

const (
	wmSyscommand = 0x0112
	wmKeydown    = 0x0100
	vkEscape     = 0x1B
	// scSizeBottomRight is SC_SIZE | WMSZ_BOTTOMRIGHT.
	scSizeBottomRight = 0xF000 | 8

	swMinimize = 6
	swRestore  = 9

	swpNoMove     = 0x0002
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
)

var (
	testUser32       = windows.NewLazySystemDLL("user32.dll")
	testSetWindowPos = testUser32.NewProc("SetWindowPos")
	testShowWindow   = testUser32.NewProc("ShowWindow")
	testPostMessage  = testUser32.NewProc("PostMessageW")
)

type fixture struct {
	t        *testing.T
	platform *win32.Platform
	device   *Device
}

func newFixture(t *testing.T, power gpu.PowerPreference) *fixture {
	t.Helper()
	p, err := win32.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Destroy)
	d, err := New(Options{Power: power})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Destroy)
	return &fixture{t, p, d}
}

type target struct {
	window  platform.Window
	surface gpu.Surface
}

func (f *fixture) open() *target {
	f.t.Helper()
	w, err := f.platform.CreateWindow(platform.WindowOptions{Title: "gowaku test", Width: 320, Height: 240})
	if err != nil {
		f.t.Fatal(err)
	}
	width, height := w.PhysicalSize()
	s, err := f.device.CreateSurface(w.NativeHandle(), width, height, w.Scale())
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(s.Destroy)
	return &target{w, s}
}

// cover returns a batch that fills any surface of up to the given logical size.
func cover(c geom.Color, w, h float32) *gpu.Batch {
	return &gpu.Batch{
		Instances: []gpu.Instance{{Rect: geom.Rect{X: -w, Y: -h, W: 3 * w, H: 3 * h}, Color: c}},
		Draws:     []gpu.DrawCall{{Count: 1}},
	}
}

// renderCheck draws one frame and verifies the pixel at (x, y).
func (f *fixture) renderCheck(tg *target, clear geom.Color, batch *gpu.Batch, x, y int, want color.NRGBA) {
	f.t.Helper()
	if err := f.device.BeginFrame(tg.surface, clear); err != nil {
		f.t.Fatal(err)
	}
	if batch != nil {
		if err := f.device.Draw(tg.surface, batch); err != nil {
			f.t.Fatal(err)
		}
	}
	img, err := f.device.ReadPixels(tg.surface)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.device.EndFrame(tg.surface); err != nil {
		f.t.Fatal(err)
	}
	if got := img.NRGBAAt(x, y); got != want {
		f.t.Errorf("pixel (%d, %d): got %v, want %v", x, y, got, want)
	}
}

// drain handles queued events and applies resizes, as an application would.
func (f *fixture) drain(tg *target) {
	f.t.Helper()
	for {
		ev := f.platform.WaitEvent(0)
		switch ev.Kind {
		case platform.EventNone:
			return
		case platform.EventResize:
			if ev.Window == tg.window.ID() {
				if err := tg.surface.Resize(ev.PhysicalWidth, ev.PhysicalHeight, ev.Scale); err != nil {
					f.t.Fatal(err)
				}
			}
		}
	}
}

func hwndOf(w platform.Window) uintptr {
	return uintptr(w.NativeHandle().(platform.Win32Handle).HWND)
}

func setWindowSize(w platform.Window, width, height int) {
	_, _, _ = testSetWindowPos.Call(hwndOf(w), 0, 0, 0, uintptr(width), uintptr(height), swpNoMove|swpNoZOrder|swpNoActivate)
}

var (
	red  = geom.Hex(0xff0000)
	blue = geom.Hex(0x0000ff)
)

func TestDevice_BothAdaptersDraw(t *testing.T) {
	for _, power := range []gpu.PowerPreference{gpu.LowPower, gpu.HighPerformance} {
		t.Run(power.String(), func(t *testing.T) {
			f := newFixture(t, power)
			t.Logf("adapter: %s", f.device.AdapterName())
			tg := f.open()
			w, h := tg.window.PhysicalSize()
			f.renderCheck(tg, blue, nil, 0, 0, blue.Std())
			f.renderCheck(tg, blue, cover(red, float32(w), float32(h)), w/2, h/2, red.Std())
		})
	}
}

func TestDraw_MapsLogicalPixelsThroughScale(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	tg := f.open()
	if err := tg.surface.Resize(400, 200, 2); err != nil {
		t.Fatal(err)
	}
	// In logical pixels the surface is 200x100. This covers its top-left quarter.
	quarter := &gpu.Batch{
		Instances: []gpu.Instance{{Rect: geom.Rect{W: 100, H: 50}, Color: red}},
		Draws:     []gpu.DrawCall{{Count: 1}},
	}
	f.renderCheck(tg, blue, quarter, 10, 10, red.Std())
	f.renderCheck(tg, blue, quarter, 199, 99, red.Std())
	f.renderCheck(tg, blue, quarter, 201, 99, blue.Std())
	f.renderCheck(tg, blue, quarter, 199, 101, blue.Std())
}

func TestDraw_BlendsNonPremultipliedInSRGB(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	tg := f.open()
	w, h := tg.window.PhysicalSize()
	half := geom.RGBA(1, 1, 1, 0.5)
	f.renderCheck(tg, geom.Hex(0), cover(half, float32(w), float32(h)), w/2, h/2,
		color.NRGBA{0x80, 0x80, 0x80, 0xff})
}

func TestDraw_GrowsInstanceBuffer(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	tg := f.open()
	w, h := tg.window.PhysicalSize()
	one := cover(red, float32(w), float32(h)).Instances[0]
	n := initialInstanceCapacity*3 + 1
	batch := &gpu.Batch{Draws: []gpu.DrawCall{{Count: n}}}
	for range n {
		batch.Instances = append(batch.Instances, one)
	}
	f.renderCheck(tg, blue, batch, w/2, h/2, red.Std())
}

func TestResize_Stress(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	tg := f.open()
	check := func() {
		t.Helper()
		f.drain(tg)
		w, h := tg.window.PhysicalSize()
		if w == 0 || h == 0 {
			return
		}
		f.renderCheck(tg, blue, cover(red, float32(w), float32(h)), w-1, h-1, red.Std())
	}
	for i := range 200 {
		setWindowSize(tg.window, 200+i*7%900, 150+i*13%700)
		check()
	}
	// Far below the minimum and far beyond the screen: the OS clamps these.
	setWindowSize(tg.window, 1, 1)
	check()
	setWindowSize(tg.window, 7680, 4320)
	check()

	_, _, _ = testShowWindow.Call(hwndOf(tg.window), swMinimize)
	check()
	if w, h := tg.window.PhysicalSize(); w != 0 || h != 0 {
		t.Errorf("minimized window: got %dx%d, want 0x0", w, h)
	}
	_, _, _ = testShowWindow.Call(hwndOf(tg.window), swRestore)
	check()

	// The swap chain itself must handle 4K whatever the desktop size is.
	if err := tg.surface.Resize(3840, 2160, 1); err != nil {
		t.Fatal(err)
	}
	f.renderCheck(tg, blue, cover(red, 3840, 2160), 3839, 2159, red.Std())
}

// This enters the real DefWindowProc size loop. A second goroutine resizes the
// window from outside, which does not touch the user's mouse, then leaves the
// loop with Escape.
//
// The OS occasionally ends the loop by itself within moments, before any resize
// arrives. Only such attempts are repeated: an attempt that does reach the
// handler has to meet every check.
func TestResize_InsideRealModalLoop(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	tg := f.open()
	for attempt := range 5 {
		frames, resizes := f.resizeInsideModalLoop(tg)
		t.Logf("attempt %d: %d frames, %d resizes", attempt, frames, resizes)
		if resizes > 0 {
			if frames < resizes {
				t.Errorf("got %d frames for %d resizes", frames, resizes)
			}
			return
		}
	}
	t.Error("no resize reached the modal handler in any attempt")
}

func (f *fixture) resizeInsideModalLoop(tg *target) (frames, resizes int) {
	f.t.Helper()
	// The surface has the size of the last event, which the window may have left
	// behind by now. Until the first event of this loop, that size is not known.
	var w, h int
	f.platform.SetModalHandler(func() {
		for {
			ev := f.platform.WaitEvent(0)
			if ev.Kind == platform.EventNone {
				break
			}
			if ev.Kind == platform.EventResize {
				resizes++
				if err := tg.surface.Resize(ev.PhysicalWidth, ev.PhysicalHeight, ev.Scale); err != nil {
					f.t.Error(err)
				}
				w, h = ev.PhysicalWidth, ev.PhysicalHeight
			}
		}
		frames++
		// A drag can leave the window without a client area for a moment, and an
		// application draws nothing then either.
		if w > 0 && h > 0 {
			f.renderCheck(tg, blue, cover(red, float32(w), float32(h)), w-1, h-1, red.Std())
		}
	})
	defer f.platform.SetModalHandler(nil)

	hwnd := hwndOf(tg.window)
	var finished atomic.Bool
	go func() {
		defer finished.Store(true)
		_, _, _ = testPostMessage.Call(hwnd, wmSyscommand, scSizeBottomRight, 0)
		time.Sleep(200 * time.Millisecond) // real time: the OS loop must be running
		for i := range 30 {
			_, _, _ = testSetWindowPos.Call(hwnd, 0, 0, 0, uintptr(300+i*10), uintptr(200+i*5), swpNoMove|swpNoZOrder|swpNoActivate)
		}
		_, _, _ = testPostMessage.Call(hwnd, wmKeydown, vkEscape, 0)
	}()

	for !finished.Load() {
		f.platform.WaitEvent(10 * time.Millisecond)
	}
	f.platform.WaitEvent(100 * time.Millisecond)
	f.drain(tg)
	return frames, resizes
}

func TestMultiWindow_SurfacesAreIndependent(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	a, b := f.open(), f.open()

	f.renderCheck(a, red, nil, 0, 0, red.Std())
	f.renderCheck(b, blue, nil, 0, 0, blue.Std())

	// Resizing one surface must leave the other drawable with its old size.
	wb, hb := b.window.PhysicalSize()
	setWindowSize(a.window, 500, 400)
	f.drain(a)
	wa, ha := a.window.PhysicalSize()
	f.renderCheck(a, red, nil, wa-1, ha-1, red.Std())
	f.renderCheck(b, blue, nil, wb-1, hb-1, blue.Std())

	// Interleaved frames share one context and must not leak state.
	f.renderCheck(a, blue, cover(red, float32(wa), float32(ha)), wa/2, ha/2, red.Std())
	f.renderCheck(b, red, cover(blue, float32(wb), float32(hb)), wb/2, hb/2, blue.Std())
}

// Every object created from the device holds a reference to it, so a leak of
// any of them shows up in the reference count of the device.
func TestSurface_ReleasesEveryReference(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	refs := func() uint32 {
		u := &f.device.device.unknown
		const addRef = 1 // IUnknown::AddRef
		_, _, _ = syscall.SyscallN(u.vtbl[addRef], uintptr(unsafe.Pointer(u)))
		return u.Release()
	}
	before := refs()

	for range 5 {
		w, err := f.platform.CreateWindow(platform.WindowOptions{Width: 200, Height: 200})
		if err != nil {
			t.Fatal(err)
		}
		width, height := w.PhysicalSize()
		s, err := f.device.CreateSurface(w.NativeHandle(), width, height, 1)
		if err != nil {
			t.Fatal(err)
		}
		tg := &target{w, s}
		f.renderCheck(tg, blue, cover(red, 400, 400), 0, 0, red.Std())
		if err := s.Resize(300, 300, 1); err != nil {
			t.Fatal(err)
		}
		f.renderCheck(tg, blue, nil, 0, 0, blue.Std())
		s.Destroy()
		w.Close()
	}

	if got := refs(); got != before {
		t.Errorf("device references: got %d, want %d", got, before)
	}
}

func TestFrame_DoesNotAllocate(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	tg := f.open()
	var c paint.Canvas
	batch := &gpu.Batch{}
	record := func() {
		c.Reset()
		rr := geom.RRectAll(geom.Rect{X: 10, Y: 10, W: 100, H: 50}, 8)
		c.DrawShadow(rr, 8, geom.Hex(0).WithAlpha(0.3), geom.Offset{Y: 2})
		c.Save()
		c.ClipRect(geom.Rect{W: 60, H: 60})
		c.DrawRRect(rr, paint.Paint{
			Stroke:   &paint.Stroke{Width: 1, Color: blue},
			Gradient: &paint.Gradient{End: geom.Offset{X: 100}, StartColor: red, EndColor: blue},
		})
		c.Restore()
		render.Encode(c.DisplayList(), nil, 1, batch)
	}
	record()
	allocs := testing.AllocsPerRun(30, func() {
		record()
		if err := f.device.BeginFrame(tg.surface, blue); err != nil {
			t.Fatal(err)
		}
		if err := f.device.Draw(tg.surface, batch); err != nil {
			t.Fatal(err)
		}
		if err := f.device.EndFrame(tg.surface); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Errorf("got %v allocs per frame, want 0", allocs)
	}
}

// paintFrame records with draw, renders on a 400x300 surface at scale 1 and
// returns the pixels.
func (f *fixture) paintFrame(background geom.Color, draw func(c *paint.Canvas)) *image.NRGBA {
	f.t.Helper()
	tg := f.open()
	if err := tg.surface.Resize(400, 300, 1); err != nil {
		f.t.Fatal(err)
	}
	var c paint.Canvas
	c.Reset()
	draw(&c)
	batch := &gpu.Batch{}
	render.Encode(c.DisplayList(), nil, 1, batch)
	if err := f.device.BeginFrame(tg.surface, background); err != nil {
		f.t.Fatal(err)
	}
	if err := f.device.Draw(tg.surface, batch); err != nil {
		f.t.Fatal(err)
	}
	img, err := f.device.ReadPixels(tg.surface)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.device.EndFrame(tg.surface); err != nil {
		f.t.Fatal(err)
	}
	return img
}

func expectPixel(t *testing.T, img *image.NRGBA, name string, x, y int, want color.NRGBA) {
	t.Helper()
	if got := img.NRGBAAt(x, y); got != want {
		t.Errorf("%s (%d, %d): got %v, want %v", name, x, y, got, want)
	}
}

func expectNear(t *testing.T, img *image.NRGBA, name string, x, y int, want color.NRGBA, tolerance int) {
	t.Helper()
	got := img.NRGBAAt(x, y)
	for _, d := range []int{int(got.R) - int(want.R), int(got.G) - int(want.G), int(got.B) - int(want.B)} {
		if d < -tolerance || d > tolerance {
			t.Errorf("%s (%d, %d): got %v, want %v within %d", name, x, y, got, want, tolerance)
			return
		}
	}
}

func TestRRect_CornersFollowEachRadius(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	img := f.paintFrame(blue, func(c *paint.Canvas) {
		c.DrawRRect(geom.RRect{
			Rect:    geom.Rect{X: 100, Y: 100, W: 100, H: 100},
			TopLeft: 40, BottomRight: 20,
		}, paint.Paint{Color: red})
	})
	expectPixel(t, img, "中央", 150, 150, red.Std())
	expectPixel(t, img, "左上は丸い", 102, 102, blue.Std())
	expectPixel(t, img, "右上は角", 199, 100, red.Std())
	expectPixel(t, img, "左下は角", 100, 199, red.Std())
	expectPixel(t, img, "右下は丸い", 198, 198, blue.Std())
	// A point at 45 degrees on the arc of radius 40 around (140, 140).
	expectPixel(t, img, "左上の円弧の内側", 113, 113, red.Std())
	expectPixel(t, img, "左上の円弧の外側", 110, 110, blue.Std())
}

func TestRRect_RadiusIsLimitedToHalfTheSize(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	img := f.paintFrame(blue, func(c *paint.Canvas) {
		c.DrawRRect(geom.RRectAll(geom.Rect{X: 100, Y: 100, W: 100, H: 40}, 500), paint.Paint{Color: red})
	})
	expectPixel(t, img, "中央", 150, 120, red.Std())
	expectPixel(t, img, "丸い端の先端", 101, 120, red.Std())
	expectPixel(t, img, "角の外", 102, 102, blue.Std())
}

func TestRRect_EdgeIsAntialiased(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	img := f.paintFrame(geom.Hex(0), func(c *paint.Canvas) {
		// A plain box would snap to the pixels (render/snap.go). A scaled one, as in
		// a transition, keeps its place and shows the edge of the shader.
		c.Scale(0.5)
		c.DrawRRect(geom.RRect{Rect: geom.Rect{X: 201, Y: 200, W: 100, H: 100}}, paint.Paint{Color: geom.Hex(0xffffff)})
	})
	expectNear(t, img, "半分かかったピクセル", 100, 120, color.NRGBA{0x80, 0x80, 0x80, 0xff}, 2)
	expectPixel(t, img, "内側", 101, 120, color.NRGBA{0xff, 0xff, 0xff, 0xff})
	expectPixel(t, img, "外側", 99, 120, color.NRGBA{0, 0, 0, 0xff})
}

func TestRRect_PlainBoxLandsOnThePixels(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	img := f.paintFrame(geom.Hex(0), func(c *paint.Canvas) {
		c.DrawRRect(geom.RRect{Rect: geom.Rect{X: 100.4, Y: 100, W: 50, H: 50}}, paint.Paint{Color: geom.Hex(0xffffff)})
	})
	expectPixel(t, img, "縁の内側", 100, 120, color.NRGBA{0xff, 0xff, 0xff, 0xff})
	expectPixel(t, img, "縁の外側", 99, 120, color.NRGBA{0, 0, 0, 0xff})
}

func TestRRect_BorderIsInsideTheEdge(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	green := geom.Hex(0x00ff00)
	img := f.paintFrame(blue, func(c *paint.Canvas) {
		c.DrawRRect(geom.RRect{Rect: geom.Rect{X: 100, Y: 100, W: 100, H: 100}}, paint.Paint{
			Color:  red,
			Stroke: &paint.Stroke{Width: 10, Color: green},
		})
	})
	expectPixel(t, img, "外側", 99, 150, blue.Std())
	expectPixel(t, img, "ボーダー", 105, 150, green.Std())
	expectPixel(t, img, "ボーダーの内端", 109, 150, green.Std())
	expectPixel(t, img, "塗り", 110, 150, red.Std())
}

func TestRRect_BorderOnlyLeavesTheInsideOpen(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	img := f.paintFrame(blue, func(c *paint.Canvas) {
		c.DrawRRect(geom.RRect{Rect: geom.Rect{X: 100, Y: 100, W: 100, H: 100}}, paint.Paint{
			Stroke: &paint.Stroke{Width: 4, Color: red},
		})
	})
	expectPixel(t, img, "ボーダー", 102, 150, red.Std())
	expectPixel(t, img, "中は背景のまま", 150, 150, blue.Std())
}

func TestRRect_LinearGradient(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	img := f.paintFrame(geom.Hex(0), func(c *paint.Canvas) {
		c.Translate(100, 100)
		c.DrawRRect(geom.RRect{Rect: geom.Rect{W: 200, H: 50}}, paint.Paint{Gradient: &paint.Gradient{
			Start: geom.Offset{X: 50}, End: geom.Offset{X: 150},
			StartColor: geom.Hex(0x000000), EndColor: geom.Hex(0xffffff),
		}})
	})
	expectNear(t, img, "開始点より前は始点の色", 110, 120, color.NRGBA{0, 0, 0, 0xff}, 1)
	expectNear(t, img, "中点", 200, 120, color.NRGBA{0x80, 0x80, 0x80, 0xff}, 2)
	expectNear(t, img, "終点より後は終点の色", 290, 120, color.NRGBA{0xff, 0xff, 0xff, 0xff}, 1)
}

func TestRRect_RadialGradient(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	img := f.paintFrame(geom.Hex(0), func(c *paint.Canvas) {
		c.DrawRRect(geom.RRect{Rect: geom.Rect{X: 100, Y: 50, W: 200, H: 200}}, paint.Paint{Gradient: &paint.Gradient{
			Kind:  paint.GradientRadial,
			Start: geom.Offset{X: 200, Y: 150}, End: geom.Offset{X: 280, Y: 150},
			StartColor: geom.Hex(0xffffff), EndColor: geom.Hex(0x000000),
		}})
	})
	expectNear(t, img, "中心", 200, 150, color.NRGBA{0xff, 0xff, 0xff, 0xff}, 3)
	expectNear(t, img, "半径の半分", 240, 150, color.NRGBA{0x80, 0x80, 0x80, 0xff}, 3)
	expectNear(t, img, "同じ距離の別方向", 200, 190, color.NRGBA{0x80, 0x80, 0x80, 0xff}, 3)
	expectNear(t, img, "半径の外", 295, 150, color.NRGBA{0, 0, 0, 0xff}, 1)
}

func TestClip_LimitsDrawingAndEndsWithRestore(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	green := geom.Hex(0x00ff00)
	img := f.paintFrame(blue, func(c *paint.Canvas) {
		c.Save()
		c.ClipRect(geom.Rect{X: 50, Y: 50, W: 100, H: 100})
		c.DrawRRect(geom.RRect{Rect: geom.Rect{W: 400, H: 300}}, paint.Paint{Color: red})
		c.Restore()
		c.DrawRRect(geom.RRect{Rect: geom.Rect{X: 300, Y: 200, W: 50, H: 50}}, paint.Paint{Color: green})
	})
	expectPixel(t, img, "クリップの内側", 50, 50, red.Std())
	expectPixel(t, img, "クリップの右下端", 149, 149, red.Std())
	expectPixel(t, img, "クリップの外", 150, 149, blue.Std())
	expectPixel(t, img, "クリップの外", 49, 50, blue.Std())
	expectPixel(t, img, "Restore 後の描画", 320, 220, green.Std())
}

func TestClip_OutsideTheSurfaceIsSafe(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	img := f.paintFrame(blue, func(c *paint.Canvas) {
		c.ClipRect(geom.Rect{X: -100, Y: -100, W: 150, H: 5000})
		c.DrawRRect(geom.RRect{Rect: geom.Rect{W: 400, H: 300}}, paint.Paint{Color: red})
	})
	expectPixel(t, img, "内側", 49, 299, red.Std())
	expectPixel(t, img, "外側", 50, 150, blue.Std())
}

func TestOpacity_FadesFillAndBorder(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	white := geom.Hex(0xffffff)
	img := f.paintFrame(geom.Hex(0), func(c *paint.Canvas) {
		c.PushOpacity(0.5)
		c.DrawRRect(geom.RRect{Rect: geom.Rect{X: 100, Y: 100, W: 100, H: 100}}, paint.Paint{
			Color:  white,
			Stroke: &paint.Stroke{Width: 10, Color: white},
		})
	})
	expectNear(t, img, "塗り", 150, 150, color.NRGBA{0x80, 0x80, 0x80, 0xff}, 1)
	expectNear(t, img, "ボーダー", 105, 150, color.NRGBA{0x80, 0x80, 0x80, 0xff}, 1)
}

// referenceShadow blurs a supersampled mask of the rounded box with a true
// Gaussian, in float64. It is deliberately naive: it shares no math with the
// shader. The result maps pixel coordinates to coverage.
func referenceShadow(width, height int, box geom.Rect, radius, sigma float64) []float64 {
	const samples = 4
	mask := make([]float64, width*height)
	bx, by, bw, bh := float64(box.X), float64(box.Y), float64(box.W), float64(box.H)
	inside := func(x, y float64) bool {
		if x < bx || y < by || x > bx+bw || y > by+bh {
			return false
		}
		dx := math.Max(math.Max(bx+radius-x, x-(bx+bw-radius)), 0)
		dy := math.Max(math.Max(by+radius-y, y-(by+bh-radius)), 0)
		return dx*dx+dy*dy <= radius*radius
	}
	for y := range height {
		for x := range width {
			hits := 0
			for sy := range samples {
				for sx := range samples {
					if inside(float64(x)+(float64(sx)+0.5)/samples, float64(y)+(float64(sy)+0.5)/samples) {
						hits++
					}
				}
			}
			mask[y*width+x] = float64(hits) / (samples * samples)
		}
	}

	reach := int(math.Ceil(sigma * 4))
	kernel := make([]float64, 2*reach+1)
	sum := 0.0
	for i := range kernel {
		d := float64(i - reach)
		kernel[i] = math.Exp(-d * d / (2 * sigma * sigma))
		sum += kernel[i]
	}
	for i := range kernel {
		kernel[i] /= sum
	}
	blur := func(src []float64, stepX, stepY int) []float64 {
		dst := make([]float64, len(src))
		for y := range height {
			for x := range width {
				v := 0.0
				for i, k := range kernel {
					sx, sy := x+(i-reach)*stepX, y+(i-reach)*stepY
					if sx >= 0 && sy >= 0 && sx < width && sy < height {
						v += src[sy*width+sx] * k
					}
				}
				dst[y*width+x] = v
			}
		}
		return dst
	}
	return blur(blur(mask, 1, 0), 0, 1)
}

func TestShadow_MatchesTrueGaussianBlur(t *testing.T) {
	tests := []struct {
		name         string
		box          geom.Rect
		radius, blur float32
	}{
		{"カード", geom.Rect{X: 140, Y: 110, W: 120, H: 80}, 12, 16},
		{"角丸なし", geom.Rect{X: 140, Y: 110, W: 120, H: 80}, 0, 8},
		{"円", geom.Rect{X: 170, Y: 120, W: 60, H: 60}, 30, 24},
		{"細長い", geom.Rect{X: 80, Y: 140, W: 240, H: 20}, 10, 30},
		{"小さなぼかし", geom.Rect{X: 140, Y: 110, W: 120, H: 80}, 12, 3},
		{"大きなぼかし", geom.Rect{X: 170, Y: 130, W: 60, H: 40}, 8, 60},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, gpu.LowPower)
			img := f.paintFrame(geom.Hex(0xffffff), func(c *paint.Canvas) {
				c.DrawShadow(geom.RRectAll(tt.box, tt.radius), tt.blur, geom.Hex(0), geom.Offset{})
			})
			want := referenceShadow(400, 300, tt.box, float64(tt.radius), float64(tt.blur)/2)
			var worst, total float64
			wx, wy := 0, 0
			for y := range 300 {
				for x := range 400 {
					got := 1 - float64(img.NRGBAAt(x, y).R)/255
					d := math.Abs(got - want[y*400+x])
					total += d
					if d > worst {
						worst, wx, wy = d, x, y
					}
				}
			}
			t.Logf("max error %.4f at (%d, %d), mean %.5f", worst, wx, wy, total/(400*300))
			if worst > shadowTolerance {
				t.Errorf("max error %.4f at (%d, %d), want at most %.4f", worst, wx, wy, shadowTolerance)
			}
		})
	}
}

// shadowTolerance is the largest coverage error allowed against the reference,
// out of 1: 5/255 for opaque black, and under 2/255 at the alpha of an MD3
// shadow. See docs/ADR/0005-sdf-pipeline.md for the trade-off with speed.
const shadowTolerance = 0.02

func TestShadow_SitsUnderTheShapeDrawnAfterIt(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	white := geom.Hex(0xffffff)
	rr := geom.RRectAll(geom.Rect{X: 100, Y: 100, W: 100, H: 100}, 8)
	img := f.paintFrame(white, func(c *paint.Canvas) {
		c.DrawShadow(rr, 20, geom.Hex(0), geom.Offset{Y: 10})
		c.DrawRRect(rr, paint.Paint{Color: red})
	})
	expectPixel(t, img, "形の上", 150, 150, red.Std())
	if below, above := img.NRGBAAt(150, 205).R, img.NRGBAAt(150, 95).R; below >= above {
		t.Errorf("offset shadow: below %d must be darker than above %d", below, above)
	}
}

func TestWaitEvent_DoesNotAllocateWhenIdle(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	tg := f.open()
	f.drain(tg)
	allocs := testing.AllocsPerRun(100, func() { f.platform.WaitEvent(0) })
	if allocs != 0 {
		t.Errorf("got %v allocs, want 0", allocs)
	}
}

func TestWake_UnblocksWaitEventFromAnotherGoroutine(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	go func() {
		time.Sleep(50 * time.Millisecond) // real time: WaitEvent must be blocked first
		f.platform.Wake()
	}()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if f.platform.WaitEvent(time.Second).Kind == platform.EventWake {
			return
		}
	}
	t.Error("WaitEvent did not return EventWake")
}

func TestWaitEvent_TimesOut(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	for ev := f.platform.WaitEvent(0); ev.Kind != platform.EventNone; ev = f.platform.WaitEvent(0) {
		t.Logf("discarding %v", ev.Kind)
	}
	start := time.Now()
	ev := f.platform.WaitEvent(30 * time.Millisecond)
	if ev.Kind != platform.EventNone {
		t.Errorf("got %v, want none", ev.Kind)
	}
	if d := time.Since(start); d < 25*time.Millisecond {
		t.Errorf("returned after %v, want about 30ms", d)
	}
}

func TestCreateSurface_RejectsForeignHandle(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	if _, err := f.device.CreateSurface(nil, 1, 1, 1); err == nil {
		t.Error("got nil, want an error")
	}
}

func TestDevice_DestroyWithLiveSurfacePanics(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	f.open()
	defer func() {
		if recover() == nil {
			t.Error("got no panic")
		}
	}()
	f.device.Destroy()
}
