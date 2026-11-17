//go:build gpu

package win32

import (
	"image"
	"image/color"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/platform"
)

// The messages and styles the test reads back.
const (
	WM_GETICON       = 0x007F
	GWL_EXSTYLE      = ^uintptr(19) // -20
	WS_EX_TOPMOST    = 0x00000008
	testIconPixels   = 256
	sizeLimitsLogicW = 400
)

func testIcon() []*image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, testIconPixels, testIconPixels))
	for y := range testIconPixels {
		for x := range testIconPixels {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: 0x6b, B: 0xff, A: 0xff})
		}
	}
	return []*image.NRGBA{img}
}

// pump handles the messages that the calls left, as the event loop would.
func pump(p *Platform) {
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		p.WaitEvent(10 * time.Millisecond)
	}
}

func windowRect(w *Window) RECT {
	var r RECT
	procGetWindowRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&r)))
	return r
}

func TestWindow_Decorations(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	pw, err := p.CreateWindow(platform.WindowOptions{
		Title: "decorations", Width: 500, Height: 400, Icon: testIcon(),
		MinSize: geom.Size{W: sizeLimitsLogicW, H: 300}, MaxSize: geom.Size{W: 800},
	})
	if err != nil {
		t.Fatal(err)
	}
	w := pw.(*Window)
	defer w.Close()
	pump(p)

	// アイコン. Not a subtest: those run on another thread, which would wait on
	// the window forever.
	func() {
		for _, kind := range []uintptr{ICON_BIG, ICON_SMALL} {
			if r, _, _ := syscall.SyscallN(procSendMessageW.Addr(), uintptr(w.hwnd), WM_GETICON, kind, 0); r == 0 {
				t.Errorf("WM_GETICON %d returned no icon", kind)
			}
		}
		old := w.deco.bigIcon
		w.SetIcon(testIcon())
		if w.deco.bigIcon == old || w.deco.bigIcon == 0 {
			t.Error("SetIcon did not replace the icon")
		}
		w.SetIcon(nil)
		if w.deco.bigIcon != 0 || w.deco.smallIcon != 0 {
			t.Error("SetIcon(nil) kept icons")
		}
		if r, _, _ := syscall.SyscallN(procSendMessageW.Addr(), uintptr(w.hwnd), WM_GETICON, ICON_BIG, 0); r != 0 {
			t.Error("the window still has a big icon after SetIcon(nil)")
		}
		w.SetIcon(testIcon())
	}()

	// 最小・最大の大きさ. Not a subtest: those run on another thread, which would wait on
	// the window forever.
	func() {
		// Asking for something tiny or huge ends at the limits.
		syscall.SyscallN(procSetWindowPos.Addr(), uintptr(w.hwnd), 0, 0, 0, 100, 100, SWP_NOMOVE|SWP_NOZORDER|SWP_NOACTIVATE)
		pump(p)
		width, height := w.PhysicalSize()
		if lo := int(sizeLimitsLogicW*w.Scale() + 0.5); width != lo || height != int(300*w.Scale()+0.5) {
			t.Errorf("client %dx%d after shrinking, want the minimum %dx%d", width, height, lo, int(300*w.Scale()+0.5))
		}
		syscall.SyscallN(procSetWindowPos.Addr(), uintptr(w.hwnd), 0, 0, 0, 5000, 600, SWP_NOMOVE|SWP_NOZORDER|SWP_NOACTIVATE)
		pump(p)
		if width, _ = w.PhysicalSize(); width != int(800*w.Scale()+0.5) {
			t.Errorf("client width %d after widening, want the maximum %d", width, int(800*w.Scale()+0.5))
		}
		// New limits resize a window outside them.
		w.SetSizeLimits(geom.Size{}, geom.Size{W: 600, H: 350})
		pump(p)
		if width, height = w.PhysicalSize(); width != int(600*w.Scale()+0.5) || height > int(350*w.Scale()+0.5) {
			t.Errorf("client %dx%d after SetSizeLimits, want at most 600x350 logical", width, height)
		}
		w.SetSizeLimits(geom.Size{}, geom.Size{})
	}()

	// 常に手前. Not a subtest: those run on another thread, which would wait on
	// the window forever.
	func() {
		exStyle := func() uintptr {
			r, _, _ := syscall.SyscallN(procGetWindowLongW.Addr(), uintptr(w.hwnd), GWL_EXSTYLE)
			return r
		}
		w.SetAlwaysOnTop(true)
		if exStyle()&WS_EX_TOPMOST == 0 {
			t.Error("SetAlwaysOnTop(true) left the window without WS_EX_TOPMOST")
		}
		w.SetAlwaysOnTop(false)
		if exStyle()&WS_EX_TOPMOST != 0 {
			t.Error("SetAlwaysOnTop(false) left WS_EX_TOPMOST")
		}
	}()

	// フルスクリーン. Not a subtest: those run on another thread, which would wait on
	// the window forever.
	func() {
		before := windowRect(w)
		w.SetFullscreen(true)
		pump(p)
		mon, _, _ := syscall.SyscallN(procMonitorFromWindow.Addr(), uintptr(w.hwnd), MONITOR_DEFAULTTONEAREST)
		info := MONITORINFO{cbSize: uint32(unsafe.Sizeof(MONITORINFO{}))}
		syscall.SyscallN(procGetMonitorInfoW.Addr(), mon, uintptr(unsafe.Pointer(&info)))
		if r := windowRect(w); r != info.rcMonitor || !w.Fullscreen() {
			t.Errorf("fullscreen window %+v, want the monitor %+v", r, info.rcMonitor)
		}
		width, height := w.PhysicalSize()
		if width != int(info.rcMonitor.right-info.rcMonitor.left) || height != int(info.rcMonitor.bottom-info.rcMonitor.top) {
			t.Errorf("fullscreen client %dx%d, want the whole monitor", width, height)
		}
		w.SetFullscreen(false)
		pump(p)
		if r := windowRect(w); r != before || w.Fullscreen() {
			t.Errorf("restored window %+v, want %+v as before", r, before)
		}
	}()
}

func TestWindow_TransparentHasNoRedirectionSurface(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	for _, transparent := range []bool{false, true} {
		pw, err := p.CreateWindow(platform.WindowOptions{Title: "transparent", Width: 300, Height: 200, Transparent: transparent})
		if err != nil {
			t.Fatal(err)
		}
		w := pw.(*Window)
		r, _, _ := syscall.SyscallN(procGetWindowLongW.Addr(), uintptr(w.hwnd), GWL_EXSTYLE)
		if got := r&WS_EX_NOREDIRECTIONBITMAP != 0; got != transparent || w.Transparent() != transparent {
			t.Errorf("Transparent %v: WS_EX_NOREDIRECTIONBITMAP %v, Transparent() %v", transparent, got, w.Transparent())
		}
		if h := w.NativeHandle().(platform.Win32Handle); h.Transparent != transparent {
			t.Errorf("Transparent %v: the handle says %v", transparent, h.Transparent)
		}
		w.Close()
	}
}
