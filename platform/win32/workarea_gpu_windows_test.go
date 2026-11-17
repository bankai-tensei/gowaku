//go:build gpu

package win32

import (
	"testing"
	"unsafe"

	"github.com/bankai-tensei/gowaku/platform"
)

// A window asked for more than the screen holds opens smaller and wholly on
// the work area; one that fits keeps its size.
func TestCreateWindow_FitsTheWorkArea(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	for _, c := range []struct {
		name          string
		width, height float32
		fit           bool
	}{
		{"huge", 10000, 10000, true},
		{"small", 320, 200, false},
	} {
		pw, err := p.CreateWindow(platform.WindowOptions{Title: c.name, Width: c.width, Height: c.height})
		if err != nil {
			t.Fatal(err)
		}
		w := pw.(*Window)
		a, ok := workAreaOf(w.hwnd)
		if !ok {
			t.Fatal("no work area")
		}
		var r RECT
		procGetWindowRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&r)))
		if r.left < a.left || r.top < a.top || r.right > a.right || r.bottom > a.bottom {
			t.Errorf("%s: window %+v reaches past the work area %+v", c.name, r, a)
		}
		width, height := w.PhysicalSize()
		wantWidth, wantHeight := int(c.width*w.Scale()+0.5), int(c.height*w.Scale()+0.5)
		if c.fit {
			if width >= wantWidth || height >= wantHeight {
				t.Errorf("%s: client %dx%d, want smaller than %dx%d", c.name, width, height, wantWidth, wantHeight)
			}
			if outer := r.right - r.left; outer > (a.right-a.left)*workAreaShare/100 {
				t.Errorf("%s: width %d is more than %d%% of the work area %d", c.name, outer, workAreaShare, a.right-a.left)
			}
		} else if width != wantWidth || height != wantHeight {
			t.Errorf("%s: client %dx%d, want %dx%d", c.name, width, height, wantWidth, wantHeight)
		}
		w.Close()
	}
}
