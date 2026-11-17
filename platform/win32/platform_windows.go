package win32

import (
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/internal/evqueue"
)

const (
	windowClassName = "gowaku.window"

	wmWake = WM_USER + 1

	// modalTimerID ticks the modal handler while the OS modal loop runs, so
	// that animations continue even if the pointer holds still.
	modalTimerID     = 1
	modalTimerPeriod = 16 // milliseconds
)

// Platform is the Win32 implementation of platform.Platform.
// Only one Platform can exist in a process at a time.
type Platform struct {
	hInstance uintptr
	className *uint16
	// formats are the registered clipboard formats, found on first use.
	formats formats
	// ole means that OleInitialize succeeded, which drag and drop needs, and
	// drag is a drag that the next WaitEvent runs.
	ole  bool
	drag *pendingDrag
	// trayHWND is the hidden window of the tray icons, taskbarCreated the
	// message that says Explorer restarted, and notifyIcon that the icon of a
	// notification without a tray is shown.
	trayHWND       HWND
	taskbarCreated uint32
	trays          []*tray
	lastTray       uint32
	notifyIcon     bool

	// wakeWindow is a message-only window. Wake posts to it rather than to the
	// thread because OS modal loops drop thread messages.
	wakeWindow  HWND
	wakePending atomic.Bool

	windows map[HWND]*Window
	lastID  platform.WindowID
	queue   evqueue.Queue

	modalHandler func()
	// modalWindow is the window whose modal loop is running, or zero.
	modalWindow HWND
	inHandler   bool

	// msg is a field so that passing its address to the OS does not allocate.
	msg MSG
}

var _ platform.Platform = (*Platform)(nil)

// New initializes Win32 and locks the calling goroutine to its OS thread, which
// makes it the UI goroutine. Destroy releases the lock.
func New() (*Platform, error) {
	if err := loadProcs(); err != nil {
		return nil, err
	}
	runtime.LockOSThread()
	p, err := newPlatform()
	if err != nil {
		runtime.UnlockOSThread()
		return nil, err
	}
	return p, nil
}

func newPlatform() (*Platform, error) {
	// This fails with ACCESS_DENIED if the awareness was already set by a
	// manifest or an earlier Platform. Either way the process has one.
	if r, _, err := procSetProcessDpiAwarenessContext.Call(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2); r == 0 &&
		!errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return nil, fmt.Errorf("win32: set dpi awareness: %w", err)
	}

	hInstance, _, err := procGetModuleHandleW.Call(0)
	if hInstance == 0 {
		return nil, fmt.Errorf("win32: get module handle: %w", err)
	}
	className, err := windows.UTF16PtrFromString(windowClassName)
	if err != nil {
		return nil, fmt.Errorf("win32: class name: %w", err)
	}
	cursor, _, err := procLoadCursorW.Call(0, IDC_ARROW)
	if cursor == 0 {
		return nil, fmt.Errorf("win32: load cursor: %w", err)
	}

	p := &Platform{
		hInstance: hInstance,
		className: className,
		windows:   make(map[HWND]*Window),
	}
	// The method value carries p, which spares us a global to find the
	// Platform from. The runtime keeps callbacks forever, so p stays alive
	// for as long as the OS can call back.
	wc := WNDCLASSEXW{
		lpfnWndProc:   windows.NewCallback(p.wndProc),
		hInstance:     hInstance,
		hCursor:       cursor,
		lpszClassName: className,
	}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	// wc is only read during the call; the OS copies it.
	if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return nil, fmt.Errorf("win32: register class: %w", err)
	}

	wake, err := p.createNativeWindow(0, 0, HWND_MESSAGE)
	if err != nil {
		p.unregisterClass()
		return nil, fmt.Errorf("win32: create wake window: %w", err)
	}
	p.wakeWindow = wake
	p.initOLE()
	return p, nil
}

// SetModalHandler implements platform.Platform.
func (p *Platform) SetModalHandler(fn func()) { p.modalHandler = fn }

// Wake implements platform.Platform.
func (p *Platform) Wake() {
	if p.wakePending.Swap(true) {
		return
	}
	if r, _, err := procPostMessageW.Call(uintptr(p.wakeWindow), wmWake, 0, 0); r == 0 {
		panic("gowaku: post wake message: " + err.Error())
	}
}

// WaitEvent implements platform.Platform.
func (p *Platform) WaitEvent(timeout time.Duration) platform.Event {
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	for {
		if ev, ok := p.queue.Pop(); ok {
			return ev
		}
		// The modal handler runs inside the OS's own message pump.
		// Pumping again from there would reorder messages.
		if p.inHandler {
			return platform.Event{}
		}
		if p.drag != nil && p.modalWindow == 0 {
			// Outside any dispatch to the tree, so that the modal loop of
			// OLE nests as the ones of moving and sizing do.
			p.runDrag()
			continue
		}
		if p.dispatchOne() {
			continue
		}
		if timeout == 0 {
			return platform.Event{}
		}
		wait := uintptr(INFINITE)
		if timeout > 0 {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return platform.Event{}
			}
			// Round up so that a sub-millisecond rest does not spin.
			wait = uintptr((remaining + time.Millisecond - 1) / time.Millisecond)
		}
		r, _, _ := syscall.SyscallN(procMsgWaitForMultipleObjectsEx.Addr(), 0, 0, wait, QS_ALLINPUT, MWMO_INPUTAVAILABLE)
		if r == WAIT_TIMEOUT {
			return platform.Event{}
		}
	}
}

// Destroy implements platform.Platform.
func (p *Platform) Destroy() {
	for _, w := range p.windows {
		w.Close()
	}
	p.destroyTrays()
	if r, _, err := procDestroyWindow.Call(uintptr(p.wakeWindow)); r == 0 {
		panic("gowaku: destroy wake window: " + err.Error())
	}
	p.unregisterClass()
	p.uninitOLE()
	runtime.UnlockOSThread()
}

func (p *Platform) unregisterClass() {
	// Failure leaves a class behind until the process exits, and the next New
	// reports it, so there is nothing useful to do here.
	_, _, _ = procUnregisterClassW.Call(uintptr(unsafe.Pointer(p.className)), p.hInstance)
}

// dispatchOne handles one pending OS message and reports whether there was one.
func (p *Platform) dispatchOne() bool {
	m := uintptr(unsafe.Pointer(&p.msg))
	if r, _, _ := syscall.SyscallN(procPeekMessageW.Addr(), m, 0, 0, 0, PM_REMOVE); r == 0 {
		return false
	}
	_, _, _ = syscall.SyscallN(procTranslateMessage.Addr(), m)
	_, _, _ = syscall.SyscallN(procDispatchMessageW.Addr(), m)
	return true
}

// createNativeWindow creates a window of the Platform's class.
func (p *Platform) createNativeWindow(exStyle, style, parent uintptr) (HWND, error) {
	r, _, err := procCreateWindowExW.Call(
		exStyle, uintptr(unsafe.Pointer(p.className)), 0, style,
		CW_USEDEFAULT, CW_USEDEFAULT, CW_USEDEFAULT, CW_USEDEFAULT,
		parent, 0, p.hInstance, 0,
	)
	if r == 0 {
		return 0, err
	}
	return HWND(r), nil
}

// wndProc is the window procedure of every gowaku window. Its signature is
// fixed by the OS callback ABI, hence the raw uintptr values.
func (p *Platform) wndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	h := HWND(hwnd)
	if h == p.wakeWindow && msg == wmWake {
		p.wakePending.Store(false)
		p.queue.Push(platform.Event{Kind: platform.EventWake})
		p.runModalHandler()
		return 0
	}
	if h == p.trayHWND && h != 0 {
		if r, ok := p.trayMessage(msg, wParam, lParam); ok {
			return r
		}
		return defWindowProc(hwnd, msg, wParam, lParam)
	}
	// Windows also get messages during CreateWindowExW, before they are
	// registered. None of those need handling.
	w := p.windows[h]
	if w == nil {
		return defWindowProc(hwnd, msg, wParam, lParam)
	}

	switch msg {
	case WM_NCCALCSIZE:
		if w.customFrame && wParam != 0 {
			return w.ncCalcSize(hwnd, wParam, lParam)
		}
	case WM_NCHITTEST:
		if w.customFrame {
			return w.ncHitTest(hwnd, lParam)
		}
	case WM_NCMOUSEMOVE, WM_NCLBUTTONDOWN, WM_NCLBUTTONUP, WM_NCLBUTTONDBLCLK, WM_NCMOUSELEAVE:
		if p.maxButton(w, msg, wParam, lParam) {
			return 0
		}
	case WM_GETMINMAXINFO:
		if w.minMaxInfo(lParam) {
			return 0
		}
	case WM_CLOSE:
		p.queue.Push(platform.Event{Kind: platform.EventClose, Window: w.id})
		p.runModalHandler()
		return 0
	case WM_DESTROY:
		w.a11y.destroy()
		if p.modalWindow == h {
			p.exitModal()
		}
		delete(p.windows, h)
		w.hwnd = 0
		w.destroyIcons()
		return 0
	case WM_SIZE:
		// LOWORD and HIWORD of lParam.
		w.physicalWidth, w.physicalHeight = int(lParam&0xFFFF), int(lParam>>16&0xFFFF)
		if w.pinch != nil {
			w.pinch.resize()
		}
		p.queue.Push(w.resizeEvent())
		p.runModalHandler()
		return 0
	case WM_DPICHANGED:
		w.dpi = uint32(wParam & 0xFFFF)
		// Reading through &lParam turns the integer into a pointer without a
		// uintptr-to-pointer conversion. The OS owns the RECT for this call.
		w.applySuggestedRect(*(**RECT)(unsafe.Pointer(&lParam)))
		if w.deco.icons != nil {
			// The metrics of icons follow the DPI.
			w.applyIcons()
		}
		p.queue.Push(w.resizeEvent())
		p.runModalHandler()
		return 0
	case WM_PAINT:
		_, _, _ = syscall.SyscallN(procValidateRect.Addr(), hwnd, 0)
		p.queue.Push(platform.Event{Kind: platform.EventRedraw, Window: w.id})
		p.runModalHandler()
		return 0
	case WM_GETOBJECT:
		// lParam is a DWORD object ID, sign-extended or not.
		if int32(lParam) == UiaRootObjectId {
			if r, ok := w.a11y.getObject(wParam, lParam); ok {
				return r
			}
		}
	case wmAccessibility:
		w.a11y.drain()
		return 0
	case WM_ERASEBKGND:
		// The GPU covers the whole client area. Erasing would flash.
		return 1
	case WM_MOUSEMOVE:
		p.pointerMove(w, lParam)
		return 0
	case WM_MOUSELEAVE:
		w.tracking = false
		p.queue.Push(platform.Event{Kind: platform.EventPointerLeave, Window: w.id})
		p.runModalHandler()
		return 0
	case WM_LBUTTONDOWN:
		p.buttonDown(w, input.ButtonPrimary, lParam)
		return 0
	case WM_LBUTTONUP:
		p.buttonUp(w, input.ButtonPrimary, lParam)
		return 0
	case WM_RBUTTONDOWN:
		p.buttonDown(w, input.ButtonSecondary, lParam)
		return 0
	case WM_RBUTTONUP:
		p.buttonUp(w, input.ButtonSecondary, lParam)
		return 0
	case WM_MBUTTONDOWN:
		p.buttonDown(w, input.ButtonMiddle, lParam)
		return 0
	case WM_MBUTTONUP:
		p.buttonUp(w, input.ButtonMiddle, lParam)
		return 0
	case WM_XBUTTONDOWN:
		// The thumb buttons answer TRUE, unlike the others. Not passing them
		// on also keeps DefWindowProc from sending WM_APPCOMMAND, which would
		// report the same press a second time.
		if b, ok := xbuttonOf(wParam); ok {
			p.buttonDown(w, b, lParam)
		}
		return 1
	case WM_XBUTTONUP:
		if b, ok := xbuttonOf(wParam); ok {
			p.buttonUp(w, b, lParam)
		}
		return 1
	case WM_CAPTURECHANGED:
		w.buttonsDown = 0
		return 0
	case WM_MOUSEWHEEL:
		p.wheel(w, wParam, lParam, false)
		return 0
	case WM_MOUSEHWHEEL:
		p.wheel(w, wParam, lParam, true)
		return 0
	case WM_KEYDOWN:
		p.keyEvent(w, platform.EventKeyDown, wParam, lParam)
		return 0
	case WM_KEYUP:
		p.keyEvent(w, platform.EventKeyUp, wParam, lParam)
		return 0
	case WM_CHAR:
		p.char(w, wParam)
		return 0
	case WM_SYSKEYDOWN:
		// Keys with Alt, and F10. Alt+F4 and Alt+Space stay with the system,
		// which closes the window or opens its menu.
		if wParam == VK_F4 || wParam == VK_SPACE {
			break
		}
		p.keyEvent(w, platform.EventKeyDown, wParam, lParam)
		return 0
	case WM_SYSKEYUP:
		// The system would enter its menu loop on a lone Alt or F10; gowaku
		// draws its own menu bar. See docs/ADR/0046-menus-and-shortcuts.md.
		p.keyEvent(w, platform.EventKeyUp, wParam, lParam)
		return 0
	case WM_SYSCHAR:
		// Alt+letter is a shortcut, not typing, and the system would beep at it.
		if wParam == ' ' {
			break
		}
		return 0
	case WM_SETFOCUS:
		p.queue.Push(platform.Event{Kind: platform.EventFocus, Window: w.id})
		p.runModalHandler()
		return 0
	case WM_KILLFOCUS:
		p.queue.Push(platform.Event{Kind: platform.EventBlur, Window: w.id})
		p.runModalHandler()
		return 0
	case WM_SETCURSOR:
		if lParam&0xFFFF == HTCLIENT {
			w.applyCursor()
			return 1
		}
	case WM_IME_SETCONTEXT:
		// gowaku draws the composition itself, inside the text field.
		return defWindowProc(hwnd, msg, wParam, lParam&^ISC_SHOWUICOMPOSITIONWINDOW)
	case WM_IME_STARTCOMPOSITION:
		return 0
	case WM_IME_COMPOSITION:
		p.imeComposition(w, lParam)
		return 0
	case WM_IME_ENDCOMPOSITION:
		p.pushComposition(w, nil)
		return 0
	case WM_SETTINGCHANGE:
		p.settingChange(wParam, lParam)
	case WM_ENTERSIZEMOVE, WM_ENTERMENULOOP:
		p.enterModal(h)
	case WM_EXITSIZEMOVE, WM_EXITMENULOOP:
		p.exitModal()
	case WM_TIMER:
		if wParam == modalTimerID {
			p.runModalHandler()
			return 0
		}
		if wParam == pinchTimerID {
			w.pinchTick()
			return 0
		}
	case WM_POINTERDOWN, WM_POINTERUPDATE, WM_POINTERUP:
		if p.pointerMessage(w, msg, wParam, lParam) {
			return 0
		}
	case DM_POINTERHITTEST:
		if w.pointerHitTest(wParam) {
			return 0
		}
	}
	return defWindowProc(hwnd, msg, wParam, lParam)
}

func (p *Platform) enterModal(h HWND) {
	p.modalWindow = h
	_, _, _ = syscall.SyscallN(procSetTimer.Addr(), uintptr(h), modalTimerID, modalTimerPeriod, 0)
}

func (p *Platform) exitModal() {
	_, _, _ = syscall.SyscallN(procKillTimer.Addr(), uintptr(p.modalWindow), modalTimerID)
	p.modalWindow = 0
}

// runModalHandler gives the application a turn if an OS modal loop is keeping
// WaitEvent from returning. Drawing can itself send messages to the window,
// which is why re-entry is refused; those events wait in the queue.
func (p *Platform) runModalHandler() {
	if p.modalWindow == 0 || p.modalHandler == nil || p.inHandler {
		return
	}
	p.inHandler = true
	p.modalHandler()
	p.inHandler = false
}

func defWindowProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	r, _, _ := syscall.SyscallN(procDefWindowProcW.Addr(), hwnd, msg, wParam, lParam)
	return r
}
