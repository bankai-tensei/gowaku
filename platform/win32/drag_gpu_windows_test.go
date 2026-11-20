//go:build gpu

package win32

import (
	"slices"
	"testing"
	"unsafe"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/win32/com"
)

// These go through OLE and the shell, so they run with the GPU tests. A real
// drag needs the mouse of the user, so the IDropTarget is called through COM
// with a data object of the shell, as OLE would call it.

// The slots of IDropTarget, after IUnknown.
const (
	iIDropTarget_DragEnter = 3
	iIDropTarget_DragOver  = 4
	iIDropTarget_DragLeave = 5
	iIDropTarget_Drop      = 6
)

func TestDrop_ThroughIDropTarget(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Destroy()
	pw, err := p.CreateWindow(platform.WindowOptions{Title: "drop"})
	if err != nil {
		t.Fatal(err)
	}
	w := pw.(*Window)
	if !p.ole || !w.dropRegistered {
		t.Fatal("the window must be registered with OLE")
	}
	var events []platform.DropEvent
	w.SetDropHandler(func(e *platform.DropEvent) input.DropEffect {
		events = append(events, *e)
		if e.Phase == platform.DropLeave {
			return input.DropNone
		}
		return input.DropMove
	})
	data := newDataObject(input.DragData{Text: "和文", Files: []string{`C:\a\メモ.txt`, `D:\b`}})
	if data == nil {
		t.Fatal("no data object")
	}
	defer data.Release()
	target := com.New(&dropTarget{w: w}, dragSetup().target)
	defer target.Release()

	// The point is on the screen: the top left of the client area.
	pt := POINT{}
	_, _, _ = procClientToScreen.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&pt)))
	packed := uintptr(uint32(pt.x)) | uintptr(uint32(pt.y))<<32
	effect := uint32(input.DropCopy | input.DropMove)
	if hr := target.Call(iIDropTarget_DragEnter, data.Addr(), MK_CONTROL, packed, uintptr(unsafe.Pointer(&effect))); hr != com.S_OK {
		t.Fatalf("DragEnter: %#x", hr)
	}
	if effect != uint32(input.DropMove) {
		t.Errorf("DragEnter effect %d, want a move", effect)
	}
	effect = uint32(input.DropCopy)
	target.Call(iIDropTarget_DragOver, MK_CONTROL, packed, uintptr(unsafe.Pointer(&effect)))
	if effect != 0 {
		t.Errorf("a move that the source does not allow must be refused, got %d", effect)
	}
	target.Call(iIDropTarget_DragLeave)
	effect = uint32(input.DropMove)
	target.Call(iIDropTarget_DragEnter, data.Addr(), 0, packed, uintptr(unsafe.Pointer(&effect)))
	target.Call(iIDropTarget_Drop, data.Addr(), 0, packed, uintptr(unsafe.Pointer(&effect)))
	if effect != uint32(input.DropMove) {
		t.Errorf("Drop effect %d", effect)
	}

	phases := make([]platform.DropPhase, len(events))
	for i, e := range events {
		phases[i] = e.Phase
	}
	want := []platform.DropPhase{platform.DropOver, platform.DropOver, platform.DropLeave, platform.DropOver, platform.DropDrop}
	if !slices.Equal(phases, want) {
		t.Fatalf("phases %v, want %v", phases, want)
	}
	first, last := events[0], events[len(events)-1]
	if first.Kinds != input.DataText|input.DataFiles || first.Modifiers != input.ModControl || first.X != 0 || first.Y != 0 {
		t.Errorf("enter: %+v", first)
	}
	if last.Data.Text != "和文" || !slices.Equal(last.Data.Files, []string{`C:\a\メモ.txt`, `D:\b`}) {
		t.Errorf("drop data %+v", last.Data)
	}
}
