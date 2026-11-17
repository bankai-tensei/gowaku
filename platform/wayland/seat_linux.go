package wayland

import (
	"time"

	"golang.org/x/sys/unix"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/internal/keysym"
)

// The capabilities of wl_seat and the states of keys and buttons.
const (
	seatPointer  = 1
	seatKeyboard = 2
	keyPressed   = 1
	keymapXKBV1  = 1
)

// seat is the one wl_seat that gowaku follows: its pointer, its keyboard and
// what depends on them.
type seat struct {
	wlSeatNop
	wlPointerNop

	p       *Platform
	proxy   wlSeat
	version uint32

	pointer  wlPointer
	keyboard wlKeyboard
	kb       *keyboard

	// pointerFocus is the window under the pointer, and pointerX and pointerY
	// where it is, in logical pixels of the surface.
	pointerFocus       *Window
	pointerX, pointerY float64
	buttonsDown        uint8
	scroll             scrollAccumulator
	// enterSerial is the serial of the latest pointer enter, which setting the
	// cursor needs.
	enterSerial uint32

	keyboardFocus *Window
	// serial is the serial of the latest press of a key or button, which
	// moving a window and taking the clipboard need.
	serial uint32
	time   uint32

	repeatRate, repeatDelay int32
	repeatKey               uint32
	repeating               bool
	repeatAt                time.Time

	cursor    cursorState
	clipboard clipboard
	text      textInput
	// pinch takes the pinches of the touchpad, touch the fingers and tablet
	// the pens (docs/ADR/0052-pinch-touch-and-pen.md).
	pinch  pinchState
	touch  touchState
	tablet tabletState
}

// wlSeatCapabilities implements wlSeatListener.
func (s *seat) wlSeatCapabilities(capabilities uint32) {
	c := s.p.c
	switch {
	case capabilities&seatPointer != 0 && s.pointer == 0:
		s.pointer = s.proxy.getPointer(c)
		c.listenWlPointer(s.pointer, s)
		s.cursor.init(s.p)
		s.startPinch()
	case capabilities&seatPointer == 0 && s.pointer != 0:
		s.cursor.release()
		s.releasePointer()
	}
	switch {
	case capabilities&seatTouch != 0 && s.touch.proxy == 0:
		s.startTouch()
	case capabilities&seatTouch == 0 && s.touch.proxy != 0:
		s.stopTouch()
	}
	s.startTablet()
	switch {
	case capabilities&seatKeyboard != 0 && s.keyboard == 0:
		s.keyboard = s.proxy.getKeyboard(c)
		c.listenWlKeyboard(s.keyboard, s)
	case capabilities&seatKeyboard == 0 && s.keyboard != 0:
		s.releaseKeyboard()
	}
	s.clipboard.init(s.p)
}

func (s *seat) releasePointer() {
	s.stopPinch()
	if s.version >= wlPointerReleaseSince {
		s.pointer.release(s.p.c)
	} else {
		s.p.c.destroy(proxy(s.pointer))
	}
	s.pointer, s.pointerFocus = 0, nil
}

func (s *seat) releaseKeyboard() {
	if s.version >= wlKeyboardReleaseSince {
		s.keyboard.release(s.p.c)
	} else {
		s.p.c.destroy(proxy(s.keyboard))
	}
	s.keyboard, s.keyboardFocus, s.repeating = 0, nil, false
}

func (s *seat) destroy() {
	if s.proxy == 0 {
		return
	}
	s.text.release()
	s.clipboard.release()
	s.cursor.release()
	if s.pointer != 0 {
		s.releasePointer()
	}
	if s.keyboard != 0 {
		s.releaseKeyboard()
	}
	if s.version >= wlSeatReleaseSince {
		s.proxy.release(s.p.c)
	} else {
		s.p.c.destroy(proxy(s.proxy))
	}
	s.proxy = 0
}

// forget drops a window that closes.
func (s *seat) forget(w *Window) {
	s.text.forget(w)
	if s.pointerFocus == w {
		s.pointerFocus = nil
	}
	if s.keyboardFocus == w {
		s.keyboardFocus, s.repeating = nil, false
	}
}

func (s *seat) push(ev platform.Event) { s.p.queue.Push(ev) }

// wlPointerEnter implements wlPointerListener.
func (s *seat) wlPointerEnter(serial uint32, surface wlSurface, x, y float64) {
	s.enterSerial = serial
	s.pointerFocus = s.p.windows[surface]
	s.pointerX, s.pointerY = x, y
	if w := s.pointerFocus; w != nil {
		w.pointerEntered()
		// The cursor is undefined until the client sets one for this enter.
		s.cursor.set(shapeOf(w.cursor))
		w.pointerMoved(x, y, s.time)
	}
}

// wlPointerLeave implements wlPointerListener.
func (s *seat) wlPointerLeave(_ uint32, _ wlSurface) {
	if w := s.pointerFocus; w != nil {
		w.pointerLeft()
	}
	s.pointerFocus, s.buttonsDown = nil, 0
}

// wlPointerMotion implements wlPointerListener.
func (s *seat) wlPointerMotion(at uint32, x, y float64) {
	s.time = at
	s.pointerX, s.pointerY = x, y
	if w := s.pointerFocus; w != nil {
		w.pointerMoved(x, y, at)
	}
}

// wlPointerButton implements wlPointerListener.
func (s *seat) wlPointerButton(serial, at, button, state uint32) {
	s.serial, s.time = serial, at
	b, ok := buttonOf(button)
	w := s.pointerFocus
	if !ok || w == nil {
		return
	}
	pressed := state == keyPressed
	if pressed {
		s.buttonsDown |= 1 << b
	} else {
		s.buttonsDown &^= 1 << b
	}
	w.pointerButton(b, pressed, at)
}

// wlPointerAxis implements wlPointerListener.
func (s *seat) wlPointerAxis(at, axis uint32, value float64) {
	s.time = at
	s.scroll.axis(axis, value)
	if s.version < wlPointerFrameSince {
		s.wlPointerFrame()
	}
}

// wlPointerAxisDiscrete implements wlPointerListener.
func (s *seat) wlPointerAxisDiscrete(axis uint32, discrete int32) {
	if s.version < wlPointerAxisValue120Since {
		s.scroll.discrete(axis, discrete)
	}
}

// wlPointerAxisValue120 implements wlPointerListener.
func (s *seat) wlPointerAxisValue120(axis uint32, value120 int32) {
	s.scroll.value120(axis, value120)
}

// wlPointerFrame implements wlPointerListener. A frame ends a group of events
// that belong together, such as both axes of a diagonal scroll.
func (s *seat) wlPointerFrame() {
	x, y, ok := s.scroll.take()
	w := s.pointerFocus
	if !ok || w == nil {
		return
	}
	scale := w.scale
	s.push(platform.Event{
		Kind: platform.EventScroll, Window: w.id,
		X: float32(s.pointerX * scale), Y: float32(s.pointerY * scale),
		ScrollX: x, ScrollY: y, Modifiers: s.kb.modifiers(), Time: s.time,
	})
}

// wlKeyboardKeymap implements wlKeyboardListener.
func (s *seat) wlKeyboardKeymap(format uint32, fd int32, size uint32) {
	if format != keymapXKBV1 {
		_ = unix.Close(int(fd)) //nolint:errcheck // nothing to do about it
		return
	}
	s.kb.setKeymap(fd, size)
}

// wlKeyboardEnter implements wlKeyboardListener.
func (s *seat) wlKeyboardEnter(serial uint32, surface wlSurface, _ []byte) {
	s.serial = serial
	w := s.p.windows[surface]
	s.keyboardFocus = w
	if w != nil {
		w.focusChanged(true)
	}
}

// wlKeyboardLeave implements wlKeyboardListener.
func (s *seat) wlKeyboardLeave(_ uint32, _ wlSurface) {
	if w := s.keyboardFocus; w != nil {
		w.focusChanged(false)
	}
	s.keyboardFocus, s.repeating = nil, false
}

// wlKeyboardKey implements wlKeyboardListener.
func (s *seat) wlKeyboardKey(serial, stamp, key, state uint32) {
	s.time = stamp
	w := s.keyboardFocus
	if w == nil {
		return
	}
	if state != keyPressed {
		if s.repeating && s.repeatKey == key {
			s.repeating = false
		}
		if code := keysym.KeyCode(s.kb.plainSym(key)); code != input.KeyNone {
			s.push(platform.Event{Kind: platform.EventKeyUp, Window: w.id, Key: code, Modifiers: s.kb.modifiers(), Time: stamp})
		}
		return
	}
	s.serial = serial
	s.press(w, key, false)
	if s.repeatRate > 0 && s.kb.repeats(key) {
		s.repeating, s.repeatKey = true, key
		s.repeatAt = timeNow().Add(time.Duration(s.repeatDelay) * time.Millisecond)
	}
}

// press pushes a key and then its text.
func (s *seat) press(w *Window, key uint32, repeat bool) {
	mods := s.kb.modifiers()
	if code := keysym.KeyCode(s.kb.plainSym(key)); code != input.KeyNone {
		s.push(platform.Event{
			Kind: platform.EventKeyDown, Window: w.id, Key: code, Modifiers: mods, Repeat: repeat, Time: s.time,
		})
	}
	text := s.kb.keyText(key)
	if mods&(input.ModControl|input.ModAlt) != 0 {
		// A shortcut, not typing.
		return
	}
	if t := keysym.TypedText(text); t != "" {
		s.push(platform.Event{Kind: platform.EventText, Window: w.id, Text: t})
	}
}

// timeNow is time.Now, which tests replace.
var timeNow = time.Now

// nextRepeat returns when the held key repeats next.
func (s *seat) nextRepeat() (time.Time, bool) { return s.repeatAt, s.repeating }

// repeatDue pushes the repeats of the held key that are due at now. Wayland
// leaves repeating to the client.
func (s *seat) repeatDue(now time.Time) {
	if !s.repeating || now.Before(s.repeatAt) || s.keyboardFocus == nil {
		return
	}
	interval := time.Second / time.Duration(s.repeatRate)
	s.press(s.keyboardFocus, s.repeatKey, true)
	s.repeatAt = s.repeatAt.Add(interval)
	if s.repeatAt.Before(now) {
		// The loop was busy: skip what was missed instead of a burst.
		s.repeatAt = now.Add(interval)
	}
}

// wlKeyboardModifiers implements wlKeyboardListener.
func (s *seat) wlKeyboardModifiers(_, depressed, latched, locked, group uint32) {
	s.kb.updateMask(depressed, latched, locked, group)
}

// wlKeyboardRepeatInfo implements wlKeyboardListener.
func (s *seat) wlKeyboardRepeatInfo(rate, delay int32) {
	s.repeatRate, s.repeatDelay = rate, delay
	if rate <= 0 {
		s.repeating = false
	}
}
