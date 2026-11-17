//go:build gpu

package win32

import (
	"testing"

	"github.com/bankai-tensei/gowaku/platform"
)

var procIsWindow = user32.NewProc("IsWindow")

// The close button, Alt+F4 and the window menu all come as WM_CLOSE, which
// must only ask: the window stays until the application closes it, so that it
// can refuse.
func TestClose_RequestLeavesTheWindowToTheApplication(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	pw, err := p.CreateWindow(platform.WindowOptions{Title: "close"})
	if err != nil {
		t.Fatal(err)
	}
	w := pw.(*Window)
	for p.WaitEvent(0).Kind != platform.EventNone {
	}
	const wmSysCommand, scClose = 0x0112, 0xF060 // SC_CLOSE comes from Alt+F4 and the window menu
	for _, m := range [][2]uintptr{{WM_CLOSE, 0}, {wmSysCommand, scClose}} {
		procPostMessageW.Call(uintptr(w.hwnd), m[0], m[1], 0)
		closes := 0
		for ev := p.WaitEvent(50e6); ev.Kind != platform.EventNone; ev = p.WaitEvent(50e6) {
			if ev.Kind == platform.EventClose && ev.Window == w.ID() {
				closes++
			}
		}
		if closes != 1 {
			t.Errorf("message %#x: got %d close events, want 1", m[0], closes)
		}
		if r, _, _ := procIsWindow.Call(uintptr(w.hwnd)); r == 0 {
			t.Fatalf("message %#x destroyed the window", m[0])
		}
	}
	w.Close()
}
