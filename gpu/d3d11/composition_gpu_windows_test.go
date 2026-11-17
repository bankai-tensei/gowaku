//go:build gpu

package d3d11

import (
	"syscall"
	"testing"
	"unsafe"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/platform"
)

// A transparent window gets a composition swap chain with alpha, which keeps
// what the frame clears to and draws, resizes, and leaves no reference behind.
func TestSurface_TransparentWindowKeepsAlpha(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	refs := func() uint32 {
		u := &f.device.device.unknown
		const addRef = 1 // IUnknown::AddRef
		_, _, _ = syscall.SyscallN(u.vtbl[addRef], uintptr(unsafe.Pointer(u)))
		return u.Release()
	}
	before := refs()
	for range 3 {
		w, err := f.platform.CreateWindow(platform.WindowOptions{Width: 200, Height: 200, Transparent: true})
		if err != nil {
			t.Fatal(err)
		}
		width, height := w.PhysicalSize()
		s, err := f.device.CreateSurface(w.NativeHandle(), width, height, 1)
		if err != nil {
			t.Fatal(err)
		}
		sf := s.(*Surface)
		if sf.composition.visual == nil || sf.presentFlags != 0 {
			t.Fatal("the surface of a transparent window is not one of DirectComposition")
		}
		tg := &target{w, s}
		// Cleared to nothing, the pixels have no alpha.
		f.renderCheck(tg, geom.Color{}, nil, 0, 0, geom.Color{}.Std())
		// Half-transparent red keeps its alpha. ReadPixels gives it back
		// as it was before premultiplying.
		half := geom.Color{R: 1, A: 0.5}
		f.renderCheck(tg, geom.Color{}, cover(half, 400, 400), 10, 10, half.Std())
		if err := s.Resize(300, 260, 1); err != nil {
			t.Fatal(err)
		}
		f.renderCheck(tg, geom.Color{}, nil, 299, 259, geom.Color{}.Std())
		s.Destroy()
		w.Close()
	}
	if got := refs(); got != before {
		t.Errorf("device references: got %d, want %d", got, before)
	}
}
