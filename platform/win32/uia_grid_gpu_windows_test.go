//go:build gpu

package win32

import (
	"errors"
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/win32/com"
	"github.com/bankai-tensei/gowaku/semantics"
)

// The pattern properties of UIAutomationClient.h that the grid test reads.
// UI Automation answers them by asking the provider for the pattern
// interface, so they also check the IIDs of the providers.
const (
	UIA_SelectionCanSelectMultiplePropertyId = 30060
	UIA_GridRowCountPropertyId               = 30062
	UIA_GridColumnCountPropertyId            = 30063
	UIA_GridItemRowPropertyId                = 30064
	UIA_GridItemColumnPropertyId             = 30065
	UIA_TableRowOrColumnMajorPropertyId      = 30083
)

func TestUIA_ClientReadsAVirtualGrid(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	w, err := p.CreateWindow(platform.WindowOptions{Title: "gowaku uia grid test", Width: 400, Height: 300})
	if err != nil {
		t.Fatal(err)
	}
	a := w.Accessibility()
	hwnd := w.(*Window).hwnd
	tree := virtualGrid()
	tree.Rect = geom.Rect{W: 400, H: 300}
	tree.Walk(func(n *semantics.Node) bool {
		if n.ID != 0 {
			n.Rect = geom.Rect{X: 10, Y: 10, W: 100, H: 30}
		}
		return true
	})

	ready := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		runGridClient(t, hwnd, ready)
	}()
	var diff semantics.Diff
	deadline := time.Now().Add(60 * time.Second)
	for sent, sawRedraw := false, false; ; {
		if time.Now().After(deadline) {
			t.Fatal("the client did not finish")
		}
		if ev := p.WaitEvent(5 * time.Millisecond); ev.Kind == platform.EventRedraw {
			sawRedraw = true
		}
		if a.Listening() && sawRedraw && !sent {
			semantics.Compare(nil, tree, 0, 0, &diff)
			a.Update(tree, 0, &diff)
			sent = true
			close(ready)
		}
		select {
		case <-done:
			return
		default:
		}
	}
}

func runGridClient(t *testing.T, hwnd HWND, ready <-chan struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, COINIT_MULTITHREADED); err != nil && !errors.Is(err, syscall.Errno(1)) {
		t.Errorf("CoInitializeEx: %v", err)
		return
	}
	defer windows.CoUninitialize()
	var uia *com.Unknown
	if hr, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(&CLSID_CUIAutomation)), 0, CLSCTX_INPROC_SERVER,
		uintptr(unsafe.Pointer(&IID_IUIAutomation)), uintptr(unsafe.Pointer(&uia))); com.Failed(hr) {
		t.Errorf("CoCreateInstance(CUIAutomation): %#x", uint32(hr))
		return
	}
	defer uia.Release()
	var window *com.Unknown
	if hr := uia.Call(iUIAutomation_ElementFromHandle, uintptr(hwnd), uintptr(unsafe.Pointer(&window))); com.Failed(hr) {
		t.Errorf("ElementFromHandle: %#x", uint32(hr))
		return
	}
	defer window.Release()
	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		t.Error("the window never started listening")
		return
	}
	var grid, cell *com.Unknown
	for _, e := range findAll(t, uia, window) {
		switch {
		case grid == nil && elementType(e) == UIA_DataGridControlTypeId:
			grid = e
		case cell == nil && elementName(e) == "2 KB":
			cell = e
		default:
			e.Release()
		}
	}
	if grid == nil || cell == nil {
		t.Fatalf("grid %v, cell %v", grid, cell)
	}
	defer grid.Release()
	defer cell.Release()
	ints := []struct {
		name string
		e    *com.Unknown
		prop int32
		want int32
	}{
		{"RowCount", grid, UIA_GridRowCountPropertyId, 1001},
		{"ColumnCount", grid, UIA_GridColumnCountPropertyId, 2},
		{"RowOrColumnMajor", grid, UIA_TableRowOrColumnMajorPropertyId, RowOrColumnMajor_RowMajor},
		{"Row", cell, UIA_GridItemRowPropertyId, 501},
		{"Column", cell, UIA_GridItemColumnPropertyId, 1},
	}
	for _, tt := range ints {
		if v := property(t, tt.e, tt.prop); v.VT != com.VT_I4 || v.I4() != tt.want {
			t.Errorf("%s: type %d value %d, want %d", tt.name, v.VT, v.I4(), tt.want)
		}
	}
	if v := property(t, grid, UIA_SelectionCanSelectMultiplePropertyId); v.VT != com.VT_BOOL || !v.Bool() {
		t.Errorf("CanSelectMultiple: type %d", v.VT)
	}
}
