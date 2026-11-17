package x11

// The parts of WM_NORMAL_HINTS (ICCCM 4.1.2.3) that placing a window needs.
const (
	XA_WM_NORMAL_HINTS = 40
	XA_WM_SIZE_HINTS   = 41
	USPosition         = 1 << 0
	PWinGravity        = 1 << 9
	StaticGravity      = 10
	// sizeHintsLongs is the length of WM_SIZE_HINTS, in longs.
	sizeHintsLongs = 18
)

// ClientOrigin implements platform.Placement.
func (w *Window) ClientOrigin() (physicalX, physicalY int, ok bool) {
	if w.xid == 0 {
		return 0, 0, false
	}
	p := w.platform
	var x, y int32
	var child XID
	if call(p.x.XTranslateCoordinates, uintptr(p.display), uintptr(w.xid), uintptr(p.root), 0, 0,
		ptr(&x), ptr(&y), ptr(&child)) == 0 {
		return 0, 0, false
	}
	return int(x), int(y), true
}

// MoveClientTo implements platform.Placement. Static gravity asks the window
// manager to put the client area itself at the point, not its frame; window
// managers that ignore the hint place the frame there instead.
func (w *Window) MoveClientTo(physicalX, physicalY int) {
	if w.xid == 0 {
		return
	}
	w.placeHints(physicalX, physicalY)
	p := w.platform
	call(p.x.XMoveWindow, uintptr(p.display), uintptr(w.xid), uintptr(int32(physicalX)), uintptr(int32(physicalY)))
	call(p.x.XFlush, uintptr(p.display))
}

// placeHints tells the window manager that the user asked for the place, so
// that it does not place the window by itself when it maps it.
func (w *Window) placeHints(x, y int) {
	w.state.placed, w.state.placeX, w.state.placeY = true, x, y
	w.writeNormalHints()
}

// ShowsScreenPoint implements platform.Placement: the topmost window that takes
// drops at the point is this one, as every gowaku window does.
func (w *Window) ShowsScreenPoint(physicalX, physicalY int) bool {
	cx, cy, ok := w.ClientOrigin()
	if !ok || physicalX < cx || physicalY < cy || physicalX >= cx+w.physicalWidth || physicalY >= cy+w.physicalHeight {
		return false
	}
	return w.platform.awareAt(int32(physicalX), int32(physicalY)) == w.xid
}
