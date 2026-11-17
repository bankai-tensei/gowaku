package win32

import (
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
)

// keyCodeOf maps a virtual key to an input.KeyCode. A punctuation key is what
// typed returns for it, the character of the key without modifiers on the
// layout in use, because the OEM keys move between layouts: VK_OEM_3 types `
// on a US layout and @ on a Japanese one (ADR 0116).
func keyCodeOf(vk uintptr, typed func(vk uintptr) rune) input.KeyCode {
	switch vk {
	case VK_OEM_1, VK_OEM_PLUS, VK_OEM_COMMA, VK_OEM_MINUS, VK_OEM_PERIOD, VK_OEM_2, VK_OEM_3,
		VK_OEM_4, VK_OEM_5, VK_OEM_6, VK_OEM_7, VK_OEM_8, VK_OEM_102:
		return input.PunctuationKey(typed(vk))
	case VK_MULTIPLY:
		return input.KeyAsterisk
	case VK_ADD:
		return input.KeyPlus
	case VK_SUBTRACT:
		return input.KeyMinus
	case VK_DIVIDE:
		return input.KeySlash
	}
	switch {
	case vk >= 'A' && vk <= 'Z', vk >= '0' && vk <= '9':
		return input.KeyCode(vk)
	case vk >= VK_F1 && vk <= VK_F12:
		return input.KeyF1 + input.KeyCode(vk-VK_F1)
	}
	switch vk {
	case VK_BACK:
		return input.KeyBackspace
	case VK_TAB:
		return input.KeyTab
	case VK_RETURN:
		return input.KeyEnter
	case VK_ESCAPE:
		return input.KeyEscape
	case VK_SPACE:
		return input.KeySpace
	case VK_LEFT:
		return input.KeyLeft
	case VK_RIGHT:
		return input.KeyRight
	case VK_UP:
		return input.KeyUp
	case VK_DOWN:
		return input.KeyDown
	case VK_HOME:
		return input.KeyHome
	case VK_END:
		return input.KeyEnd
	case VK_PRIOR:
		return input.KeyPageUp
	case VK_NEXT:
		return input.KeyPageDown
	case VK_DELETE:
		return input.KeyDelete
	case VK_INSERT:
		return input.KeyInsert
	case VK_MENU:
		return input.KeyAlt
	case VK_APPS:
		return input.KeyContextMenu
	}
	return input.KeyNone
}

// layoutChar is what vk types without modifiers on the keyboard layout of the
// thread. A dead key, such as ^ on a German layout, sets the top bit, and
// stands for its accent.
func layoutChar(vk uintptr) rune {
	r, _, _ := syscall.SyscallN(procMapVirtualKeyW.Addr(), vk, MAPVK_VK_TO_CHAR)
	return rune(r & 0x7FFFFFFF)
}

func modifiers() input.Modifiers {
	var m input.Modifiers
	down := func(vk uintptr) bool {
		// The high bit of the SHORT is the key's state.
		r, _, _ := syscall.SyscallN(procGetKeyState.Addr(), vk)
		return r&0x8000 != 0
	}
	if down(VK_SHIFT) {
		m |= input.ModShift
	}
	if down(VK_CONTROL) {
		m |= input.ModControl
	}
	if down(VK_MENU) {
		m |= input.ModAlt
	}
	return m
}

// pointerPos reads the client coordinates of a mouse message. They are signed
// 16-bit values, because capture reports positions left of the window.
func pointerPos(lParam uintptr) (x, y float32) {
	return float32(int16(lParam & 0xFFFF)), float32(int16(lParam >> 16 & 0xFFFF))
}

func (p *Platform) messageTime() uint32 {
	r, _, _ := syscall.SyscallN(procGetMessageTime.Addr())
	return uint32(r)
}

func (p *Platform) pushPointer(w *Window, kind platform.EventKind, button input.Button, lParam uintptr) {
	x, y := pointerPos(lParam)
	p.queue.Push(platform.Event{
		Kind: kind, Window: w.id, X: x, Y: y, Button: button, Modifiers: modifiers(), Time: p.messageTime(),
	})
	p.runModalHandler()
}

func (p *Platform) buttonDown(w *Window, button input.Button, lParam uintptr) {
	if w.buttonsDown == 0 {
		_, _, _ = syscall.SyscallN(procSetCapture.Addr(), uintptr(w.hwnd))
	}
	w.buttonsDown |= 1 << button
	p.pushPointer(w, platform.EventPointerDown, button, lParam)
}

func (p *Platform) buttonUp(w *Window, button input.Button, lParam uintptr) {
	w.buttonsDown &^= 1 << button
	if w.buttonsDown == 0 {
		_, _, _ = syscall.SyscallN(procReleaseCapture.Addr())
	}
	p.pushPointer(w, platform.EventPointerUp, button, lParam)
}

// pointerMove also asks for WM_MOUSELEAVE, which the OS sends only once per
// request.
func (p *Platform) pointerMove(w *Window, lParam uintptr) {
	if !w.tracking {
		w.tracking = true
		track := TRACKMOUSEEVENT{dwFlags: TME_LEAVE, hwndTrack: w.hwnd}
		track.cbSize = uint32(unsafe.Sizeof(track))
		// track is only read during the call.
		_, _, _ = syscall.SyscallN(procTrackMouseEvent.Addr(), uintptr(unsafe.Pointer(&track)))
	}
	p.pushPointer(w, platform.EventPointerMove, input.ButtonPrimary, lParam)
}

// wheel pushes a scroll. The delta is in notches, positive away from the user
// for the vertical wheel. The message reports screen coordinates.
func (p *Platform) wheel(w *Window, wParam, lParam uintptr, horizontal bool) {
	x, y := pointerPos(lParam)
	pt := POINT{int32(x), int32(y)}
	// pt is only used during the call.
	_, _, _ = syscall.SyscallN(procScreenToClient.Addr(), uintptr(w.hwnd), uintptr(unsafe.Pointer(&pt)))
	delta := float32(int16(wParam>>16&0xFFFF)) / WHEEL_DELTA
	ev := platform.Event{
		Kind: platform.EventScroll, Window: w.id, X: float32(pt.x), Y: float32(pt.y),
		Modifiers: modifiers(), Time: p.messageTime(),
	}
	if horizontal {
		ev.ScrollX = delta
	} else {
		ev.ScrollY = delta
	}
	p.queue.Push(ev)
	p.runModalHandler()
}

func (p *Platform) keyEvent(w *Window, kind platform.EventKind, wParam, lParam uintptr) {
	key := keyCodeOf(wParam, layoutChar)
	if key == input.KeyNone {
		return
	}
	p.queue.Push(platform.Event{
		Kind:      kind,
		Window:    w.id,
		Key:       key,
		Modifiers: modifiers(),
		// Bit 30 is the previous key state.
		Repeat: lParam&(1<<30) != 0,
		Time:   p.messageTime(),
	})
	p.runModalHandler()
}

// char handles WM_CHAR, which delivers characters outside the BMP in two halves.
func (p *Platform) char(w *Window, wParam uintptr) {
	u := rune(uint16(wParam))
	switch {
	case utf16.IsSurrogate(u) && u < 0xDC00:
		w.highSurrogate = u
		return
	case utf16.IsSurrogate(u):
		u = utf16.DecodeRune(w.highSurrogate, u)
		w.highSurrogate = 0
	case u < 0x20 || u == 0x7F:
		// Control characters arrive as keys.
		return
	}
	p.pushText(w, string(u))
}

func (p *Platform) pushText(w *Window, s string) {
	p.queue.Push(platform.Event{Kind: platform.EventText, Window: w.id, Text: s})
	p.runModalHandler()
}

func (p *Platform) pushComposition(w *Window, c *input.Composition) {
	p.queue.Push(platform.Event{Kind: platform.EventComposition, Window: w.id, Composition: c})
	p.runModalHandler()
}

// imeComposition reads what WM_IME_COMPOSITION announces. The committed text
// comes first, because one message can both commit and start a new composition.
func (p *Platform) imeComposition(w *Window, lParam uintptr) {
	himc, _, _ := syscall.SyscallN(procImmGetContext.Addr(), uintptr(w.hwnd))
	if himc == 0 {
		return
	}
	defer func() {
		_, _, _ = syscall.SyscallN(procImmReleaseContext.Addr(), uintptr(w.hwnd), himc)
	}()
	if lParam&GCS_RESULTSTR != 0 {
		if units := immUnits(himc, GCS_RESULTSTR); len(units) > 0 {
			p.pushComposition(w, nil)
			p.pushText(w, string(utf16.Decode(units)))
		}
	}
	if lParam&GCS_COMPSTR != 0 {
		units := immUnits(himc, GCS_COMPSTR)
		attrs := immBytes(himc, GCS_COMPATTR)
		clauses := immClauses(himc)
		r, _, _ := syscall.SyscallN(procImmGetCompositionStringW.Addr(), himc, GCS_CURSORPOS, 0, 0)
		if len(units) == 0 {
			p.pushComposition(w, nil)
			return
		}
		p.pushComposition(w, buildComposition(units, attrs, clauses, int(int32(r))))
	}
}

// immBytes reads a composition value. The first call asks for the size.
func immBytes(himc, index uintptr) []byte {
	n, _, _ := syscall.SyscallN(procImmGetCompositionStringW.Addr(), himc, index, 0, 0)
	size := int(int32(n))
	if size <= 0 {
		return nil
	}
	buf := make([]byte, size)
	// buf is only written during the call.
	n, _, _ = syscall.SyscallN(procImmGetCompositionStringW.Addr(), himc, index, uintptr(unsafe.Pointer(&buf[0])), uintptr(size))
	if int(int32(n)) <= 0 {
		return nil
	}
	return buf[:int(int32(n))]
}

func immUnits(himc, index uintptr) []uint16 {
	b := immBytes(himc, index)
	units := make([]uint16, len(b)/2)
	for i := range units {
		units[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	return units
}

func immClauses(himc uintptr) []uint32 {
	b := immBytes(himc, GCS_COMPCLAUSE)
	out := make([]uint32, len(b)/4)
	for i := range out {
		out[i] = uint32(b[4*i]) | uint32(b[4*i+1])<<8 | uint32(b[4*i+2])<<16 | uint32(b[4*i+3])<<24
	}
	return out
}

// SetIMEEnabled implements platform.Window. Disabling takes the input context
// away from the window, so that keys reach it unconverted.
func (w *Window) SetIMEEnabled(enabled bool) {
	w.imeWanted = enabled
	w.associateIME()
}

// SetSecureInput implements platform.SecureInputWindow. Without an input
// context no IME, of IMM or of TSF, sees the keys, and that is all Windows
// offers: it has no secure event input.
func (w *Window) SetSecureInput(secure bool) {
	if secure == w.imeSecure {
		return
	}
	w.imeSecure = secure
	w.associateIME()
}

// associateIME gives the window the default input context, or takes it away.
func (w *Window) associateIME() {
	if w.hwnd == 0 {
		return
	}
	flags := uintptr(0)
	if w.imeWanted && !w.imeSecure {
		flags = IACE_DEFAULT
	}
	_, _, _ = syscall.SyscallN(procImmAssociateContextEx.Addr(), uintptr(w.hwnd), 0, flags)
}

// SetIMECursorRect implements platform.Window. The composition window moves to
// the caret, and the candidate list keeps clear of the caret's line.
func (w *Window) SetIMECursorRect(physicalX, physicalY, physicalWidth, physicalHeight int) {
	himc, _, _ := syscall.SyscallN(procImmGetContext.Addr(), uintptr(w.hwnd))
	if himc == 0 {
		return
	}
	x, y := int32(physicalX), int32(physicalY)
	area := RECT{x, y, x + int32(physicalWidth), y + int32(physicalHeight)}
	composition := COMPOSITIONFORM{dwStyle: CFS_POINT, ptCurrentPos: POINT{x, y}, rcArea: area}
	// Both forms are copied during the call.
	_, _, _ = syscall.SyscallN(procImmSetCompositionWindow.Addr(), himc, uintptr(unsafe.Pointer(&composition)))
	// The candidate list starts below a horizontal caret. Beside a vertical
	// one the IME, told by the composition font, opens it to the left.
	below := POINT{x, y + int32(physicalHeight)}
	if w.imeVertical {
		below = POINT{x, y}
	}
	candidate := CANDIDATEFORM{dwStyle: CFS_EXCLUDE, ptCurrentPos: below, rcArea: area}
	_, _, _ = syscall.SyscallN(procImmSetCandidateWindow.Addr(), himc, uintptr(unsafe.Pointer(&candidate)))
	_, _, _ = syscall.SyscallN(procImmReleaseContext.Addr(), uintptr(w.hwnd), himc)
}

// SetIMEVertical implements platform.Window. IMM has no flag for it: an IME
// knows vertical text by a composition font turned 270°, as Word sets it.
func (w *Window) SetIMEVertical(vertical bool) {
	if w.hwnd == 0 || vertical == w.imeVertical {
		return
	}
	himc, _, _ := syscall.SyscallN(procImmGetContext.Addr(), uintptr(w.hwnd))
	if himc == 0 {
		return
	}
	w.imeVertical = vertical
	var font LOGFONTW
	face := "Yu Gothic"
	if vertical {
		font.lfEscapement, font.lfOrientation = 2700, 2700
		face = "@" + face // the vertical variant, by GDI's convention
	}
	font.lfCharSet = SHIFTJIS_CHARSET
	for i, c := range face {
		font.lfFaceName[i] = uint16(c)
	}
	// The font is copied during the call.
	_, _, _ = syscall.SyscallN(procImmSetCompositionFontW.Addr(), himc, uintptr(unsafe.Pointer(&font)))
	_, _, _ = syscall.SyscallN(procImmReleaseContext.Addr(), uintptr(w.hwnd), himc)
}

// CancelComposition implements platform.Window.
func (w *Window) CancelComposition() {
	himc, _, _ := syscall.SyscallN(procImmGetContext.Addr(), uintptr(w.hwnd))
	if himc == 0 {
		return
	}
	_, _, _ = syscall.SyscallN(procImmNotifyIME.Addr(), himc, NI_COMPOSITIONSTR, CPS_CANCEL, 0)
	_, _, _ = syscall.SyscallN(procImmReleaseContext.Addr(), uintptr(w.hwnd), himc)
}

// SetCursor implements platform.Window.
func (w *Window) SetCursor(c input.Cursor) {
	w.cursor = c
	w.applyCursor()
}

func (w *Window) applyCursor() {
	id := uintptr(IDC_ARROW)
	switch w.cursor {
	case input.CursorIBeam:
		id = IDC_IBEAM
	case input.CursorHand:
		id = IDC_HAND
	case input.CursorColResize:
		id = IDC_SIZEWE
	case input.CursorRowResize:
		id = IDC_SIZENS
	}
	h, _, _ := syscall.SyscallN(procLoadCursorW.Addr(), 0, id)
	_, _, _ = syscall.SyscallN(procSetCursor.Addr(), h)
}

// ClipboardText implements platform.Window. It returns "" if the clipboard holds
// no text or another program has it open.
func (w *Window) ClipboardText() string {
	if r, _, _ := syscall.SyscallN(procOpenClipboard.Addr(), uintptr(w.hwnd)); r == 0 {
		return ""
	}
	defer func() { _, _, _ = syscall.SyscallN(procCloseClipboard.Addr()) }()
	h, _, _ := syscall.SyscallN(procGetClipboardData.Addr(), CF_UNICODETEXT)
	if h == 0 {
		return ""
	}
	p, _, _ := syscall.SyscallN(procGlobalLock.Addr(), h)
	if p == 0 {
		return ""
	}
	defer func() { _, _, _ = syscall.SyscallN(procGlobalUnlock.Addr(), h) }()
	size, _, _ := syscall.SyscallN(procGlobalSize.Addr(), h)
	// Reading through &p turns the integer into a pointer without a
	// uintptr-to-pointer conversion. The block stays valid until GlobalUnlock.
	units := unsafe.Slice(*(**uint16)(unsafe.Pointer(&p)), int(size)/2)
	n := 0
	for n < len(units) && units[n] != 0 {
		n++
	}
	return string(utf16.Decode(units[:n]))
}

// SetClipboardText implements platform.Window.
func (w *Window) SetClipboardText(s string) {
	units := utf16.Encode([]rune(s))
	units = append(units, 0)
	h, _, _ := syscall.SyscallN(procGlobalAlloc.Addr(), GMEM_MOVEABLE, uintptr(len(units)*2))
	if h == 0 {
		return
	}
	p, _, _ := syscall.SyscallN(procGlobalLock.Addr(), h)
	if p == 0 {
		_, _, _ = syscall.SyscallN(procGlobalFree.Addr(), h)
		return
	}
	// See ClipboardText for the conversion. The block is ours until it is
	// handed to the clipboard below.
	copy(unsafe.Slice(*(**uint16)(unsafe.Pointer(&p)), len(units)), units)
	_, _, _ = syscall.SyscallN(procGlobalUnlock.Addr(), h)
	if r, _, _ := syscall.SyscallN(procOpenClipboard.Addr(), uintptr(w.hwnd)); r == 0 {
		_, _, _ = syscall.SyscallN(procGlobalFree.Addr(), h)
		return
	}
	_, _, _ = syscall.SyscallN(procEmptyClipboard.Addr())
	// On success the clipboard owns the block.
	if r, _, _ := syscall.SyscallN(procSetClipboardData.Addr(), CF_UNICODETEXT, h); r == 0 {
		_, _, _ = syscall.SyscallN(procGlobalFree.Addr(), h)
	}
	_, _, _ = syscall.SyscallN(procCloseClipboard.Addr())
}
