package win32

import (
	"log/slog"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/internal/clip"
	"github.com/bankai-tensei/gowaku/platform/win32/com"
)

// Drag and drop through OLE. Each window registers an IDropTarget, written in
// Go with package com, which OLE calls on the UI thread while something is
// dragged over the window, from this or another program. A drag out of the
// window puts the data in the generic data object of the shell
// (SHCreateDataObject), so that no IDataObject has to be written here, and runs
// DoDragDrop with an IDropSource written in Go. See
// docs/ADR/0050-drag-and-drop.md.

var (
	ole32   = windows.NewLazySystemDLL("ole32.dll")
	shell32 = windows.NewLazySystemDLL("shell32.dll")

	procOleInitialize      = ole32.NewProc("OleInitialize")
	procOleUninitialize    = ole32.NewProc("OleUninitialize")
	procRegisterDragDrop   = ole32.NewProc("RegisterDragDrop")
	procRevokeDragDrop     = ole32.NewProc("RevokeDragDrop")
	procDoDragDrop         = ole32.NewProc("DoDragDrop")
	procReleaseStgMedium   = ole32.NewProc("ReleaseStgMedium")
	procSHCreateDataObject = shell32.NewProc("SHCreateDataObject")
)

// The values of OLE drag and drop.
const (
	DRAGDROP_S_DROP              = 0x00040100
	DRAGDROP_S_CANCEL            = 0x00040101
	DRAGDROP_S_USEDEFAULTCURSORS = 0x00040102
	DVASPECT_CONTENT             = 1
	TYMED_HGLOBAL                = 1
	MK_LBUTTON                   = 0x0001
	MK_SHIFT                     = 0x0004
	MK_CONTROL                   = 0x0008
	MK_ALT                       = 0x0020
	RPC_E_CHANGED_MODE           = 0x80010106
	iIDataObject_GetData         = 3 // IDataObject::GetData
	iIDataObject_QueryGetData    = 5 // IDataObject::QueryGetData
	iIDataObject_SetData         = 7 // IDataObject::SetData
	dropTargetMethodCount        = 4
	dropSourceMethodCount        = 2
	dropEffectMask               = 0x7
)

// The interfaces of drag and drop.
var (
	IID_IDropTarget = windows.GUID{Data1: 0x00000122, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	IID_IDropSource = windows.GUID{Data1: 0x00000121, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	IID_IDataObject = windows.GUID{Data1: 0x0000010E, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
)

// FORMATETC is the OLE structure that names a format.
type FORMATETC struct {
	cfFormat uint16
	_        [6]byte
	ptd      uintptr
	dwAspect uint32
	lindex   int32
	tymed    uint32
	_        uint32
}

// STGMEDIUM is the OLE structure that holds data, here an HGLOBAL.
type STGMEDIUM struct {
	tymed          uint32
	_              uint32
	hGlobal        uintptr
	pUnkForRelease uintptr
}

func formatOf(cf uint16) FORMATETC {
	return FORMATETC{cfFormat: cf, dwAspect: DVASPECT_CONTENT, lindex: -1, tymed: TYMED_HGLOBAL}
}

// dragInterfaces are the vtables, built once per process.
type dragInterfaces struct {
	target, source *com.Interface
}

var dragSetup = sync.OnceValue(func() *dragInterfaces {
	cb := syscall.NewCallback
	return &dragInterfaces{
		target: com.NewInterface(IID_IDropTarget, cb(dropTargetDragEnter), cb(dropTargetDragOver),
			cb(dropTargetDragLeave), cb(dropTargetDrop)),
		source: com.NewInterface(IID_IDropSource, cb(dropSourceQueryContinueDrag), cb(dropSourceGiveFeedback)),
	}
})

// initOLE prepares the thread for drag and drop. A thread already in the
// multithreaded apartment cannot have it, which leaves drag and drop off.
func (p *Platform) initOLE() {
	hr, _, _ := syscall.SyscallN(procOleInitialize.Addr(), 0)
	switch {
	case !com.Failed(hr):
		p.ole = true
	case hr == RPC_E_CHANGED_MODE:
		slog.Debug("win32: drag and drop is off: the thread is in the multithreaded apartment")
	default:
		slog.Debug("win32: OleInitialize failed", "hr", hr)
	}
}

func (p *Platform) uninitOLE() {
	if p.ole {
		_, _, _ = syscall.SyscallN(procOleUninitialize.Addr())
		p.ole = false
	}
}

// dropTarget is the Go side of the IDropTarget of a window.
type dropTarget struct {
	w *Window
	// kinds is what the drag in progress carries.
	kinds input.DataKinds
}

// registerDrop makes the window a drop target.
func (w *Window) registerDrop() {
	if !w.platform.ole {
		return
	}
	obj := com.New(&dropTarget{w: w}, dragSetup().target)
	if hr, _, _ := syscall.SyscallN(procRegisterDragDrop.Addr(), uintptr(w.hwnd), obj.Addr()); com.Failed(hr) {
		slog.Debug("win32: RegisterDragDrop failed", "hr", hr)
		obj.Release()
		return
	}
	// OLE holds its own reference now.
	obj.Release()
	w.dropRegistered = true
}

// revokeDrop undoes registerDrop, before the window goes.
func (w *Window) revokeDrop() {
	if w.dropRegistered {
		_, _, _ = syscall.SyscallN(procRevokeDragDrop.Addr(), uintptr(w.hwnd))
		w.dropRegistered = false
	}
}

// SetDropHandler implements platform.Dragging.
func (w *Window) SetDropHandler(fn func(e *platform.DropEvent) input.DropEffect) { w.dropHandler = fn }

// keyModifiers turns the key state of OLE into modifiers.
func keyModifiers(keys uintptr) input.Modifiers {
	var m input.Modifiers
	if keys&MK_SHIFT != 0 {
		m |= input.ModShift
	}
	if keys&MK_CONTROL != 0 {
		m |= input.ModControl
	}
	if keys&MK_ALT != 0 {
		m |= input.ModAlt
	}
	return m
}

// ask hands a drag to the application and writes its answer to *effect, which
// holds what the source allows on entry.
func (t *dropTarget) ask(phase platform.DropPhase, keys, pt uintptr, effect *uint32, data input.DragData) {
	w := t.w
	allowed := input.DropEffect(*effect & dropEffectMask)
	*effect = 0
	if w.dropHandler == nil || w.hwnd == 0 {
		return
	}
	// POINTL arrives by value in one register: x in the low half.
	p := POINT{int32(uint32(pt)), int32(uint32(pt >> 32))}
	// p is only used during the call.
	_, _, _ = syscall.SyscallN(procScreenToClient.Addr(), uintptr(w.hwnd), uintptr(unsafe.Pointer(&p)))
	e := platform.DropEvent{
		Phase: phase, X: float32(p.x), Y: float32(p.y), Kinds: t.kinds, Data: data,
		Allowed: allowed, Modifiers: keyModifiers(keys),
	}
	*effect = uint32(w.dropHandler(&e) & allowed)
	w.redrawAfterDrag()
}

// redrawAfterDrag gives the application a frame after a drag step: the
// handler may have changed what shows, and WaitEvent would not return
// otherwise.
func (w *Window) redrawAfterDrag() {
	w.platform.queue.Push(platform.Event{Kind: platform.EventRedraw, Window: w.id})
	w.platform.runModalHandler()
}

// targetOf returns the Go side of an IDropTarget.
func targetOf(this *com.Unknown) (*dropTarget, bool) { return com.Get[*dropTarget](this) }

// effectPtr reads the DWORD* argument of a method. OLE owns it for the call.
func effectPtr(p uintptr) *uint32 {
	// Reading through &p turns the integer into a pointer without a
	// uintptr-to-pointer conversion.
	return *(**uint32)(unsafe.Pointer(&p))
}

// dropTargetDragEnter is IDropTarget::DragEnter.
func dropTargetDragEnter(this, data *com.Unknown, keys, pt, effect uintptr) uintptr {
	t, ok := targetOf(this)
	if !ok || effect == 0 {
		return com.E_INVALIDARG
	}
	t.kinds = dataKinds(data)
	t.ask(platform.DropOver, keys, pt, effectPtr(effect), input.DragData{})
	return com.S_OK
}

// dropTargetDragOver is IDropTarget::DragOver.
func dropTargetDragOver(this *com.Unknown, keys, pt, effect uintptr) uintptr {
	t, ok := targetOf(this)
	if !ok || effect == 0 {
		return com.E_INVALIDARG
	}
	t.ask(platform.DropOver, keys, pt, effectPtr(effect), input.DragData{})
	return com.S_OK
}

// dropTargetDragLeave is IDropTarget::DragLeave.
func dropTargetDragLeave(this *com.Unknown) uintptr {
	t, ok := targetOf(this)
	if !ok {
		return com.E_INVALIDARG
	}
	if t.w.dropHandler != nil && t.w.hwnd != 0 {
		t.w.dropHandler(&platform.DropEvent{Phase: platform.DropLeave, Kinds: t.kinds})
		t.w.redrawAfterDrag()
	}
	t.kinds = 0
	return com.S_OK
}

// dropTargetDrop is IDropTarget::Drop.
func dropTargetDrop(this, data *com.Unknown, keys, pt, effect uintptr) uintptr {
	t, ok := targetOf(this)
	if !ok || effect == 0 {
		return com.E_INVALIDARG
	}
	t.kinds = dataKinds(data)
	t.ask(platform.DropDrop, keys, pt, effectPtr(effect), readDragData(data))
	t.kinds = 0
	return com.S_OK
}

// dataKinds asks a data object which kinds it has.
func dataKinds(data *com.Unknown) input.DataKinds {
	var k input.DataKinds
	if data == nil {
		return k
	}
	for _, f := range []struct {
		cf   uint16
		kind input.DataKinds
	}{{CF_UNICODETEXT, input.DataText}, {CF_HDROP, input.DataFiles}} {
		fe := formatOf(f.cf)
		// fe is only read during the call.
		if data.Call(iIDataObject_QueryGetData, uintptr(unsafe.Pointer(&fe))) == com.S_OK {
			k |= f.kind
		}
	}
	return k
}

// dataBytes reads one format of a data object.
func dataBytes(data *com.Unknown, cf uint16) []byte {
	fe := formatOf(cf)
	var m STGMEDIUM
	// fe and m are used during the call; m then owns what it holds until
	// ReleaseStgMedium.
	if hr := data.Call(iIDataObject_GetData, uintptr(unsafe.Pointer(&fe)), uintptr(unsafe.Pointer(&m))); com.Failed(hr) {
		return nil
	}
	defer func() { _, _, _ = syscall.SyscallN(procReleaseStgMedium.Addr(), uintptr(unsafe.Pointer(&m))) }()
	if m.tymed != TYMED_HGLOBAL || m.hGlobal == 0 {
		return nil
	}
	return globalCopy(m.hGlobal)
}

// readDragData reads the text and files of a data object.
func readDragData(data *com.Unknown) input.DragData {
	var d input.DragData
	if data == nil {
		return d
	}
	if b := dataBytes(data, CF_HDROP); b != nil {
		d.Files = clip.DecodeDropFiles(b)
	}
	if b := dataBytes(data, CF_UNICODETEXT); len(b) >= 2 {
		units := unsafe.Slice((*uint16)(unsafe.Pointer(&b[0])), len(b)/2)
		n := 0
		for n < len(units) && units[n] != 0 {
			n++
		}
		d.Text = string(utf16Decode(units[:n]))
	}
	return d
}

// pendingDrag is a drag that StartDrag asked for, which the next WaitEvent
// runs.
type pendingDrag struct {
	w       *Window
	data    input.DragData
	allowed input.DropEffect
	done    func(input.DropEffect)
}

// StartDrag implements platform.Dragging.
func (w *Window) StartDrag(d input.DragData, allowed input.DropEffect, done func(input.DropEffect)) {
	p := w.platform
	if !p.ole || p.drag != nil || d.Kinds() == 0 {
		done(input.DropNone)
		return
	}
	p.drag = &pendingDrag{w: w, data: d, allowed: allowed, done: done}
}

// runDrag runs the pending drag, a modal loop of OLE, during which the modal
// handler keeps the application drawing.
func (p *Platform) runDrag() {
	d := p.drag
	p.drag = nil
	effect := input.DropNone
	defer func() { d.done(effect) }()
	if d.w.hwnd == 0 {
		return
	}
	data := newDataObject(d.data)
	if data == nil {
		return
	}
	defer data.Release()
	source := com.New(struct{}{}, dragSetup().source)
	defer source.Release()
	var result uint32
	p.enterModal(d.w.hwnd)
	// result is written during the call.
	hr, _, _ := syscall.SyscallN(procDoDragDrop.Addr(), data.Addr(), source.Addr(), uintptr(d.allowed),
		uintptr(unsafe.Pointer(&result)))
	p.exitModal()
	if hr == DRAGDROP_S_DROP {
		effect = input.DropEffect(result&dropEffectMask) & d.allowed
	}
}

// newDataObject puts d in the generic data object of the shell.
func newDataObject(d input.DragData) *com.Unknown {
	var obj *com.Unknown
	// obj is written during the call.
	if hr, _, _ := syscall.SyscallN(procSHCreateDataObject.Addr(), 0, 0, 0, 0, uintptr(unsafe.Pointer(&IID_IDataObject)),
		uintptr(unsafe.Pointer(&obj))); com.Failed(hr) || obj == nil {
		slog.Debug("win32: SHCreateDataObject failed", "hr", hr)
		return nil
	}
	if d.Text != "" {
		setData(obj, CF_UNICODETEXT, utf16Bytes(d.Text))
	}
	if len(d.Files) > 0 {
		setData(obj, CF_HDROP, clip.EncodeDropFiles(d.Files))
	}
	return obj
}

// setData hands data to a data object as format cf.
func setData(obj *com.Unknown, cf uint16, data []byte) {
	h := globalBytes(data)
	if h == 0 {
		return
	}
	fe := formatOf(cf)
	m := STGMEDIUM{tymed: TYMED_HGLOBAL, hGlobal: h}
	// fe and m are read during the call; with fRelease set, the object owns
	// the block on success.
	if hr := obj.Call(iIDataObject_SetData, uintptr(unsafe.Pointer(&fe)), uintptr(unsafe.Pointer(&m)), 1); com.Failed(hr) {
		_, _, _ = syscall.SyscallN(procGlobalFree.Addr(), h)
	}
}

// dropSourceQueryContinueDrag is IDropSource::QueryContinueDrag: the drag goes
// on while the primary button is down, and Escape cancels it.
func dropSourceQueryContinueDrag(_ *com.Unknown, escape, keys uintptr) uintptr {
	switch {
	case escape != 0:
		return DRAGDROP_S_CANCEL
	case keys&MK_LBUTTON == 0:
		return DRAGDROP_S_DROP
	}
	return com.S_OK
}

// dropSourceGiveFeedback is IDropSource::GiveFeedback: the cursors of OLE.
func dropSourceGiveFeedback(*com.Unknown, uintptr) uintptr { return DRAGDROP_S_USEDEFAULTCURSORS }
