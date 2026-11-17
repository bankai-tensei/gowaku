package x11

import (
	"encoding/binary"
	"unsafe"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/internal/clip"
)

// Drag and drop is XDND 5: client messages between the window under the
// pointer and the source of the drag, and the XdndSelection for the data. The
// helper window is the source of the drags of this application, as it owns
// the clipboard. See docs/ADR/0050-drag-and-drop.md.

const (
	xdndVersion = 5
	// xdndManyTypes is the bit of XdndEnter that says the types are in the
	// XdndTypeList of the source.
	xdndManyTypes = 1
	// xdndAccept and xdndWantPositions are the bits of XdndStatus.
	xdndAccept        = 1
	xdndWantPositions = 2
)

// The grab mode and the key of Xlib that a drag uses.
const (
	GrabModeAsync = 1
	XK_Escape     = 0xff1b
)

// dropState is a drag from some source over a window.
type dropState struct {
	source  XID
	version int
	kinds   input.DataKinds
	types   []string
	// effect is what the window last said a drop would do.
	effect input.DropEffect
}

// dragState is a drag that this application started.
type dragState struct {
	w       *Window
	allowed input.DropEffect
	done    func(input.DropEffect)
	types   []Atom
	// target is the window under the pointer that takes drops, or zero, and
	// accepted and action what it last answered.
	target   XID
	accepted bool
	action   Atom
	// dropped means that XdndDrop went, and XdndFinished is awaited.
	dropped bool
}

// xdndAware marks w as taking drops.
func (w *Window) xdndAware() {
	v := [1]uintptr{xdndVersion}
	w.setProperty(w.platform.atoms.XdndAware, XA_ATOM, 32, unsafe.Pointer(&v[0]), 1)
}

// SetDropHandler implements platform.Dragging.
func (w *Window) SetDropHandler(fn func(e *platform.DropEvent) input.DropEffect) { w.dropHandler = fn }

// send sends a client message of kind to window to, from this application.
func (p *Platform) send(to XID, kind Atom, data ...uintptr) {
	m := XClientMessageEvent{MessageType: kind, Format: 32}
	m.Type, m.Display, m.Window = ClientMessage, p.display, to
	copy(m.Data[:], data)
	call(p.x.XSendEvent, uintptr(p.display), uintptr(to), 0, 0, ptr(&m))
	call(p.x.XFlush, uintptr(p.display))
}

// effectOf turns an XDND action into a drop effect.
func (p *Platform) effectOf(action Atom) input.DropEffect {
	a := &p.atoms
	switch action {
	case a.XdndActionMove:
		return input.DropMove
	case a.XdndActionLink:
		return input.DropLink
	case a.XdndActionCopy:
		return input.DropCopy
	}
	return input.DropNone
}

// actionOf turns a drop effect into an XDND action.
func (p *Platform) actionOf(e input.DropEffect) Atom {
	a := &p.atoms
	switch e {
	case input.DropMove:
		return a.XdndActionMove
	case input.DropLink:
		return a.XdndActionLink
	case input.DropCopy:
		return a.XdndActionCopy
	}
	return 0
}

// xdnd handles the client messages of XDND for w. It reports whether m was one.
func (w *Window) xdnd(m *XClientMessageEvent) bool {
	a := &w.platform.atoms
	switch m.MessageType {
	case a.XdndEnter:
		w.xdndEnter(m)
	case a.XdndPosition:
		w.xdndPosition(m)
	case a.XdndLeave:
		if XID(m.Data[0]) == w.drop.source {
			w.ask(platform.DropEvent{Phase: platform.DropLeave, Kinds: w.drop.kinds})
			w.drop = dropState{}
		}
	case a.XdndDrop:
		w.xdndDrop(m)
	default:
		return false
	}
	return true
}

func (w *Window) xdndEnter(m *XClientMessageEvent) {
	p := w.platform
	d := dropState{source: XID(m.Data[0]), version: int(m.Data[1] >> 24)}
	var types []Atom
	if m.Data[1]&xdndManyTypes != 0 {
		value, _ := p.readProperty(d.source, p.atoms.XdndTypeList, false)
		size := int(unsafe.Sizeof(uintptr(0)))
		for i := 0; i+size <= len(value); i += size {
			types = append(types, Atom(binary.NativeEndian.Uint64(value[i:])))
		}
	} else {
		for _, t := range m.Data[2:5] {
			if t != 0 {
				types = append(types, Atom(t))
			}
		}
	}
	for _, t := range types {
		if name := p.atomName(t); name != "" {
			d.types = append(d.types, name)
		}
	}
	k := clip.Kinds(d.types)
	if k&platform.ClipboardText != 0 {
		d.kinds |= input.DataText
	}
	if k&platform.ClipboardFiles != 0 {
		d.kinds |= input.DataFiles
	}
	w.drop = d
}

// rootToWindow turns a place on the root into one in w.
func (w *Window) rootToWindow(x, y int32) (int32, int32) {
	p := w.platform
	var wx, wy int32
	var child XID
	call(p.x.XTranslateCoordinates, uintptr(p.display), uintptr(p.root), uintptr(w.xid), uintptr(x), uintptr(y),
		ptr(&wx), ptr(&wy), ptr(&child))
	return wx, wy
}

// allowedOf is what a source that asks for action allows. Every XDND source
// takes a copy.
func (p *Platform) allowedOf(action Atom) input.DropEffect {
	return p.effectOf(action) | input.DropCopy
}

func (w *Window) xdndPosition(m *XClientMessageEvent) {
	p := w.platform
	if XID(m.Data[0]) != w.drop.source {
		return
	}
	x, y := w.rootToWindow(int32(m.Data[2]>>16&0xFFFF), int32(m.Data[2]&0xFFFF))
	w.drop.effect = w.ask(platform.DropEvent{
		Phase: platform.DropOver, X: float32(x), Y: float32(y), Kinds: w.drop.kinds,
		Allowed: p.allowedOf(Atom(m.Data[4])),
	})
	flags := uintptr(xdndWantPositions)
	if w.drop.effect != input.DropNone {
		flags |= xdndAccept
	}
	w.dropAllowed = p.allowedOf(Atom(m.Data[4]))
	w.dropX, w.dropY = x, y
	p.send(w.drop.source, p.atoms.XdndStatus, uintptr(w.xid), flags, 0, 0, uintptr(p.actionOf(w.drop.effect)))
}

func (w *Window) xdndDrop(m *XClientMessageEvent) {
	p := w.platform
	d := w.drop
	if XID(m.Data[0]) != d.source {
		return
	}
	w.drop = dropState{}
	effect := input.DropNone
	if d.effect != input.DropNone {
		var c platform.ClipboardContent
		for _, want := range [][]string{clip.TextTypes, clip.FileTypes} {
			if t := clip.Best(d.types, want); t != "" {
				if value, _, ok := p.convertSelection(p.atoms.XdndSelection, p.atom(t), m.Data[2]); ok {
					clip.Decode(&c, t, value)
				}
			}
		}
		effect = w.ask(platform.DropEvent{
			Phase: platform.DropDrop, X: float32(w.dropX), Y: float32(w.dropY), Kinds: d.kinds,
			Data: input.DragData{Text: c.Text, Files: c.Files}, Allowed: w.dropAllowed,
		})
	} else {
		w.ask(platform.DropEvent{Phase: platform.DropLeave, Kinds: d.kinds})
	}
	accepted := uintptr(0)
	if effect != input.DropNone {
		accepted = 1
	}
	p.send(d.source, p.atoms.XdndFinished, uintptr(w.xid), accepted, uintptr(p.actionOf(effect)))
}

// ask hands a drag to the application, and queues a frame: the handler may
// have changed what shows.
func (w *Window) ask(e platform.DropEvent) input.DropEffect {
	if w.dropHandler == nil {
		return input.DropNone
	}
	effect := w.dropHandler(&e)
	w.platform.queue.Push(platform.Event{Kind: platform.EventRedraw, Window: w.id})
	return effect & e.Allowed
}

// StartDrag implements platform.Dragging. The drag starts at once: the
// pointer, still down, is grabbed by w until it comes up.
func (w *Window) StartDrag(d input.DragData, allowed input.DropEffect, done func(input.DropEffect)) {
	p := w.platform
	if p.dragging != nil || d.Kinds() == 0 || w.buttonsDown&(1<<input.ButtonPrimary) == 0 {
		done(input.DropNone)
		return
	}
	p.dragOffers = clip.Encode(&platform.ClipboardContent{Text: d.Text, Files: d.Files})
	types := make([]Atom, 0, len(p.dragOffers))
	for _, o := range p.dragOffers {
		types = append(types, p.atom(o.Type))
	}
	b := atomBytes(types...)
	call(p.x.XChangeProperty, uintptr(p.display), uintptr(p.helper), uintptr(p.atoms.XdndTypeList), XA_ATOM, 32,
		PropModeReplace, ptr(&b[0]), uintptr(len(types)))
	call(p.x.XSetSelectionOwner, uintptr(p.display), uintptr(p.atoms.XdndSelection), uintptr(p.helper), CurrentTime)
	call(p.x.XGrabPointer, uintptr(p.display), uintptr(w.xid), 0, ButtonReleaseMask|PointerMotionMask,
		GrabModeAsync, GrabModeAsync, 0, uintptr(p.cursors[cursorIndex(input.CursorHand)]), CurrentTime)
	call(p.x.XFlush, uintptr(p.display))
	p.dragging = &dragState{w: w, allowed: allowed, done: done, types: types}
}

// awareAt returns the window at (x, y) on the root that takes drops, or zero.
// Window managers put a frame around each window, so it looks down the stack
// of windows under the pointer.
func (p *Platform) awareAt(x, y int32) XID {
	parent := p.root
	for range 8 {
		var dx, dy int32
		var child XID
		call(p.x.XTranslateCoordinates, uintptr(p.display), uintptr(p.root), uintptr(parent), uintptr(x), uintptr(y),
			ptr(&dx), ptr(&dy), ptr(&child))
		if child == 0 {
			return 0
		}
		if value, _ := p.readProperty(child, p.atoms.XdndAware, false); len(value) > 0 {
			return child
		}
		parent = child
	}
	return 0
}

// dragMotion follows the pointer of a drag: it tells the windows it leaves and
// enters, and where it is.
func (p *Platform) dragMotion(e *XKeyEvent) {
	d := p.dragging
	a := &p.atoms
	target := p.awareAt(e.XRoot, e.YRoot)
	if target != d.target {
		if d.target != 0 {
			p.send(d.target, a.XdndLeave, uintptr(p.helper))
		}
		d.target, d.accepted, d.action = target, false, 0
		if target != 0 {
			flags := uintptr(xdndVersion) << 24
			types := [3]uintptr{}
			for i, t := range d.types {
				if i < 3 {
					types[i] = uintptr(t)
				}
			}
			if len(d.types) > 3 {
				flags |= xdndManyTypes
			}
			p.send(target, a.XdndEnter, uintptr(p.helper), flags, types[0], types[1], types[2])
		}
	}
	if target != 0 {
		action := p.actionOf(d.allowed.Pick(input.DropCopy | input.DropMove | input.DropLink))
		p.send(target, a.XdndPosition, uintptr(p.helper), 0, uintptr(e.XRoot)<<16|uintptr(e.YRoot)&0xFFFF,
			e.Time, uintptr(action))
	}
}

// dragRelease drops where the pointer came up, or gives up.
func (p *Platform) dragRelease(e *XKeyEvent) {
	d := p.dragging
	call(p.x.XUngrabPointer, uintptr(p.display), CurrentTime)
	d.w.buttonsDown = 0
	if d.target == 0 || !d.accepted {
		p.dragCancel()
		return
	}
	d.dropped = true
	p.send(d.target, p.atoms.XdndDrop, uintptr(p.helper), 0, e.Time)
}

// dragCancel ends the drag without a drop.
func (p *Platform) dragCancel() {
	d := p.dragging
	call(p.x.XUngrabPointer, uintptr(p.display), CurrentTime)
	if d.target != 0 && !d.dropped {
		p.send(d.target, p.atoms.XdndLeave, uintptr(p.helper))
	}
	p.dragEnd(input.DropNone)
}

func (p *Platform) dragEnd(effect input.DropEffect) {
	d := p.dragging
	p.dragging, p.dragOffers = nil, nil
	d.done(effect)
}

// dragMessage handles the answers of the target to the helper.
func (p *Platform) dragMessage(m *XClientMessageEvent) {
	d := p.dragging
	if d == nil || XID(m.Data[0]) != d.target {
		return
	}
	a := &p.atoms
	switch m.MessageType {
	case a.XdndStatus:
		d.accepted, d.action = m.Data[1]&xdndAccept != 0, Atom(m.Data[4])
	case a.XdndFinished:
		if !d.dropped {
			return
		}
		effect := input.DropNone
		if m.Data[1]&1 != 0 {
			effect = p.effectOf(Atom(m.Data[2]))
			if effect == input.DropNone {
				effect = p.effectOf(d.action)
			}
		}
		p.dragEnd(effect & d.allowed)
	}
}

// dragEvent takes the pointer and key events of the window of a drag in
// progress. It reports whether it took ev.
func (w *Window) dragEvent(ev *XEvent) bool {
	p := w.platform
	d := p.dragging
	if d == nil || d.w != w || d.dropped {
		return false
	}
	switch ev.Type {
	case MotionNotify:
		p.dragMotion(as[XKeyEvent](ev))
	case ButtonRelease:
		if as[XKeyEvent](ev).Keycode == 1 {
			p.dragRelease(as[XKeyEvent](ev))
		}
	case KeyPress:
		if call(p.x.XLookupKeysym, ptr(ev), 0) == XK_Escape {
			p.dragCancel()
		}
	default:
		return false
	}
	return true
}
