package core

import (
	"cmp"
	"math"
	"slices"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
)

const (
	// multiClickTime and multiClickDistance bound a double click. They match
	// the Windows defaults (milliseconds, logical pixels).
	multiClickTime     = 500
	multiClickDistance = 4
)

// Host is what a tree needs from its window. Rectangles are in logical pixels
// from the top left of the window. The zero Tree has a Host that does nothing,
// so that trees work without a window.
type Host interface {
	SetIMEEnabled(enabled bool)
	SetIMECursorRect(r geom.Rect)
	CancelComposition()
	SetCursor(c input.Cursor)
	ClipboardText() string
	SetClipboardText(s string)
}

// VerticalIMEHost is a Host that can tell the IME that the text runs in
// vertical lines, so that the candidate list opens beside the line instead of
// over it. A Host need not implement it.
type VerticalIMEHost interface {
	SetIMEVertical(vertical bool)
}

// SecureInputHost is a Host that can treat what is typed as a secret while a
// password field has the keyboard: the IME stays off whatever SetIMEEnabled
// says, and on systems that have it, other programs cannot read the keys
// (secure event input on macOS). A field turns it on when it gains the
// keyboard and off when it loses it; the tree turns it off when the focused
// object leaves. A Host need not implement it, and must take repeated calls
// with the same value as one. See docs/ADR/0088-input-obscure.md.
type SecureInputHost interface {
	SetSecureInput(secure bool)
}

// noHost is the Host of a tree without a window.
type noHost struct{}

var _ Host = noHost{}

// SetIMEEnabled implements Host.
func (noHost) SetIMEEnabled(bool) {}

// SetIMECursorRect implements Host.
func (noHost) SetIMECursorRect(geom.Rect) {}

// CancelComposition implements Host.
func (noHost) CancelComposition() {}

// SetCursor implements Host.
func (noHost) SetCursor(input.Cursor) {}

// ClipboardText implements Host.
func (noHost) ClipboardText() string { return "" }

// SetClipboardText implements Host.
func (noHost) SetClipboardText(string) {}

// PointerKind says what happened to the pointer.
type PointerKind uint8

// The pointer kinds.
const (
	PointerDown PointerKind = iota
	PointerMove
	PointerUp
	// PointerCancel ends a press that will not complete.
	PointerCancel
	PointerScroll
	// PointerPinch is two fingers moving apart or together on a touchpad, by
	// the factor Pinch. A Ctrl+wheel that no pinch handler takes is also
	// delivered as one: 1.1 to the power of the notches. See
	// docs/ADR/0052-pinch-touch-and-pen.md.
	PointerPinch
)

// PointerEvent is a pointer action.
type PointerEvent struct {
	Kind PointerKind
	// Pos is the position in logical pixels. Handlers see it relative to
	// themselves; Dispatch takes it relative to the window.
	Pos       geom.Offset
	Button    input.Button
	Modifiers input.Modifiers
	// Device is what moves the pointer, and Pressure that of a pen, from 0 to
	// 1, or 0 where it is not known. ID tells the fingers of a touch screen
	// apart; the tree delivers the first finger as the pointer, a drag of it as
	// a scroll, and two fingers as a pinch.
	Device   input.PointerDevice
	Pressure float32
	ID       uint32
	// Clicks counts the presses in a row, from 1. It is set for PointerDown.
	Clicks int
	// Scroll is the wheel movement of PointerScroll, in notches, with the
	// signs of the Windows wheel on every platform. Positive Y is the wheel
	// turned away from the user: it moves the view toward the top of the
	// content, so a list shows earlier rows. Negative Y moves it toward the
	// bottom.
	// Positive X moves the view to the right, toward the end of a line, and
	// negative X to the left. A handler that keeps a scroll offset growing
	// downward therefore subtracts Y and adds X.
	Scroll geom.Offset
	// Pinch is the change of scale of PointerPinch since the last one: above
	// 1 when the fingers move apart.
	Pinch float32
	// Time is a millisecond clock. It wraps around.
	Time uint32
	// Handled stops the event from reaching the objects below. A PointerDown
	// that is handled also keeps the rest of the press for that object.
	Handled bool
}

// PointerTarget is a RenderObject that reacts to the pointer. Between a press
// that it handled and the release, it gets every event, wherever the pointer is.
type PointerTarget interface {
	RenderObject
	HandlePointer(e *PointerEvent)
}

// HoverTarget is a RenderObject that wants to know when the pointer is over it.
type HoverTarget interface {
	RenderObject
	HandleHover(inside bool)
}

// HoverMoveTarget is a RenderObject that wants to know where the pointer is
// while it is over it, pressed or not, as a tooltip that follows the pointer
// over a chart does. HandleHoverMove gets the position in the object's own
// coordinates on every move of the pointer over it, after HandleHover(true)
// of a HoverTarget when the pointer has just entered.
type HoverMoveTarget interface {
	RenderObject
	HandleHoverMove(pos geom.Offset)
}

// FocusWithinListener is a RenderObject that wants to know when the focus
// enters or leaves the objects below it, as a tooltip shows for the control
// inside it. It is told when the focus moves, whether or not the window has the
// keyboard.
type FocusWithinListener interface {
	RenderObject
	FocusWithinChanged(within bool)
}

// CursorSource is a RenderObject that picks the mouse cursor over it. The one
// deepest under the pointer wins.
type CursorSource interface {
	RenderObject
	MouseCursor() input.Cursor
}

// KeyEvent is a key press.
type KeyEvent struct {
	Key       input.KeyCode
	Modifiers input.Modifiers
	// Repeat means that the key is being held down.
	Repeat bool
}

// FocusTarget is a RenderObject that can have the keyboard.
type FocusTarget interface {
	RenderObject
	// CanFocus reports whether the object takes focus now.
	CanFocus() bool
	// HandleKey reacts to a key press. It returns false to let the tree use the
	// key, for moving the focus with Tab.
	HandleKey(e *KeyEvent) bool
	// HandleText receives typed text and text committed by an IME.
	HandleText(s string)
	// HandleComposition receives the text an IME is converting. Nil means
	// that the composition ended.
	HandleComposition(c *input.Composition)
	// FocusChanged reports that the object gained or lost the keyboard. That
	// includes the whole window doing so.
	FocusChanged(focused bool)
}

// FocusScope is a RenderObject that keeps Tab inside itself while it is modal, as
// a dialog does. The last one painted wins.
type FocusScope interface {
	RenderObject
	// Modal reports whether the scope holds the focus now.
	Modal() bool
}

// GlobalOffset returns the position of ro in the window.
func GlobalOffset(ro RenderObject) geom.Offset {
	var off geom.Offset
	for ; ro != nil; ro = ro.Parent() {
		off = off.Add(ro.Offset())
	}
	return off
}

// Host returns the window services of the tree, which are never nil.
func (b *RenderBase) Host() Host {
	if b.tree == nil {
		return noHost{}
	}
	return b.tree.Host()
}

// RequestFocus gives the keyboard to the object, which must implement
// FocusTarget.
func (b *RenderBase) RequestFocus() {
	if b.tree != nil {
		b.tree.RequestFocus(b.self.(FocusTarget))
	}
}

// ClearFocus takes the keyboard away from whatever has it in the tree of the
// object, as a press on nothing does.
func (b *RenderBase) ClearFocus() {
	if b.tree != nil {
		b.tree.clearFocus()
	}
}

// Focused returns what has the keyboard in the tree of the object, or nil.
func (b *RenderBase) Focused() FocusTarget {
	if b.tree == nil {
		return nil
	}
	return b.tree.focus
}

// Attached reports whether the object is in a tree.
func (b *RenderBase) Attached() bool { return b.tree != nil }

// RestoreFocus gives the keyboard back to target, unless it left the tree in the
// meantime.
func RestoreFocus(target FocusTarget) {
	if b := target.base(); b.tree != nil {
		b.tree.RequestFocus(target)
	}
}

// Host returns the window services, which are never nil.
func (t *Tree) Host() Host {
	if t.host == nil {
		return noHost{}
	}
	return t.host
}

// SetHost connects the tree to its window.
func (t *Tree) SetHost(h Host) { t.host = h }

// DispatchPointer delivers e, whose Pos is relative to the window.
func (t *Tree) DispatchPointer(e PointerEvent) {
	if e.Device == input.DeviceTouch && e.Kind != PointerPinch {
		t.dispatchTouch(e)
		return
	}
	t.dispatchPointer(e)
}

// dispatchPointer is DispatchPointer after the fingers are sorted out.
func (t *Tree) dispatchPointer(e PointerEvent) {
	if forward, ok := navigationButton(e.Button); ok && (e.Kind == PointerDown || e.Kind == PointerUp) {
		// A thumb button is a request to go back or forward, not a press: it
		// neither takes the pointer nor moves the focus.
		if e.Kind == PointerUp {
			t.navigateAt(e.Pos, forward)
		}
		return
	}
	switch e.Kind {
	case PointerDown:
		t.pointerDown(&e)
	case PointerMove:
		t.updateHover(e.Pos)
		if len(t.captured) > 0 {
			t.deliver(t.captured, &e)
		}
	case PointerUp, PointerCancel:
		t.deliver(t.captured, &e)
		t.captured = t.captured[:0]
		t.updateHover(e.Pos)
	case PointerScroll:
		t.hit(e.Pos)
		if e.Modifiers&input.ModControl != 0 && e.Scroll.Y != 0 {
			pinch := e
			pinch.Kind, pinch.Pinch = PointerPinch, ctrlWheelFactor(e.Scroll.Y)
			if t.deliver(t.hits.Path, &pinch) {
				return
			}
		}
		t.deliver(t.hits.Path, &e)
	case PointerPinch:
		t.hit(e.Pos)
		t.deliver(t.hits.Path, &e)
	}
}

// PointerLeft reports that the pointer left the window.
func (t *Tree) PointerLeft() {
	t.setHovered(nil)
	t.pointerInside = false
}

func (t *Tree) pointerDown(e *PointerEvent) {
	e.Clicks = t.countClicks(e)
	t.altDown = false
	t.setFocusVisible(false)
	t.hit(e.Pos)
	t.captured = t.captured[:0]
	path := t.hits.Path
	for i, ro := range path {
		target, ok := ro.(PointerTarget)
		if !ok {
			continue
		}
		pos, ok := GlobalToLocal(ro, e.Pos)
		if !ok {
			continue
		}
		if t.handOver(target, e, pos) {
			t.captured = append(t.captured, ro)
			t.focusOnPress(path[i:])
			return
		}
	}
	// A press on nothing takes the keyboard away from whoever has it.
	t.clearFocus()
}

func (t *Tree) countClicks(e *PointerEvent) int {
	d := e.Pos.Sub(t.lastPress)
	near := d.X*d.X+d.Y*d.Y <= multiClickDistance*multiClickDistance
	if t.clicks > 0 && near && e.Button == t.lastButton && e.Time-t.lastPressTime <= multiClickTime {
		t.clicks++
	} else {
		t.clicks = 1
	}
	t.lastPress, t.lastPressTime, t.lastButton = e.Pos, e.Time, e.Button
	return t.clicks
}

// ctrlWheelFactor is the pinch of a Ctrl+wheel of notches, as browsers zoom.
func ctrlWheelFactor(notches float32) float32 {
	return float32(math.Pow(1.1, float64(notches)))
}

// deliver hands e to the targets of path, until one handles it, and reports
// whether one did.
func (t *Tree) deliver(path []RenderObject, e *PointerEvent) bool {
	for _, ro := range path {
		target, ok := ro.(PointerTarget)
		if !ok || ro.base().tree == nil {
			continue
		}
		pos, ok := GlobalToLocal(ro, e.Pos)
		if !ok {
			continue
		}
		if t.handOver(target, e, pos) {
			return true
		}
	}
	return false
}

// handOver gives target its own copy of e, at pos in its coordinates, and
// reports whether it handled it. A copy on the stack would escape through the
// interface call and cost an allocation per target and event, so the copies
// are kept: one per depth of delivery, as a target may dispatch another event
// from its handler. A target must not keep the event past its HandlePointer.
func (t *Tree) handOver(target PointerTarget, e *PointerEvent, pos geom.Offset) bool {
	if t.eventDepth == len(t.events) {
		t.events = append(t.events, new(PointerEvent))
	}
	local := t.events[t.eventDepth]
	t.eventDepth++
	*local = *e
	local.Pos = pos
	target.HandlePointer(local)
	t.eventDepth--
	return local.Handled
}

// hit fills t.hits with what is at pos, reusing its storage.
func (t *Tree) hit(pos geom.Offset) {
	t.hits.Path = t.hits.Path[:0]
	if !isHidden(t.rootRO) {
		t.rootRO.HitTest(pos, &t.hits)
	}
}

func (t *Tree) updateHover(pos geom.Offset) {
	t.pointerInside = true
	t.hit(pos)
	t.setHovered(t.hits.Path)
	for _, ro := range t.hits.Path {
		if target, ok := ro.(HoverMoveTarget); ok && ro.base().tree != nil {
			if local, ok := GlobalToLocal(ro, pos); ok {
				target.HandleHoverMove(local)
			}
		}
	}
	cursor := input.CursorArrow
	for _, ro := range t.hits.Path {
		if c, ok := ro.(CursorSource); ok {
			cursor = c.MouseCursor()
			break
		}
	}
	if cursor != t.cursor {
		t.cursor = cursor
		t.Host().SetCursor(cursor)
	}
}

// setHovered tells the objects that the pointer left or entered.
func (t *Tree) setHovered(path []RenderObject) {
	for _, ro := range t.hovered {
		if target, ok := ro.(HoverTarget); ok && ro.base().tree != nil && !contains(path, ro) {
			target.HandleHover(false)
		}
	}
	for _, ro := range path {
		if target, ok := ro.(HoverTarget); ok && !contains(t.hovered, ro) {
			target.HandleHover(true)
		}
	}
	t.hovered = append(t.hovered[:0], path...)
}

func contains(path []RenderObject, ro RenderObject) bool {
	for _, p := range path {
		if p == ro {
			return true
		}
	}
	return false
}

// Focused returns what has the keyboard, or nil.
func (t *Tree) Focused() FocusTarget { return t.focus }

// RequestFocus gives the keyboard to target. Nil takes it away.
//
// Focused answers target at once. A FocusChanged or FocusWithinChanged that
// calls RequestFocus itself moves the focus on: the call returns right away,
// and the objects hear of the move once the callback is back, so an object the
// focus only passed through hears nothing.
func (t *Tree) RequestFocus(target FocusTarget) {
	t.anchor = nil
	if t.focus == target {
		return
	}
	t.focus = target
	t.syncFocus()
}

// syncFocus tells the objects what changed since they were last told: the one
// that believes it has the keyboard, focusTold, and the listeners above
// focusWithin. Any callback may move the focus or the keyboard of the window
// again, so it looks anew at what is true after each one instead of finishing
// a list made before. A call from inside a callback leaves the work to the
// call that is already running, which keeps every FocusChanged(true) paired
// with one false, and needs no storage.
func (t *Tree) syncFocus() {
	if t.focusSyncing {
		return
	}
	t.focusSyncing = true
	// A panic in a callback must not leave the tree deaf to every later move.
	defer t.focusSynced()
	for {
		var want FocusTarget
		if t.windowFocused {
			want = t.focus
		}
		switch {
		case t.focusTold != want && t.focusTold != nil:
			old := t.focusTold
			t.focusTold = nil
			old.FocusChanged(false)
		case t.focusTold != want:
			t.focusTold = want
			want.FocusChanged(true)
		case t.focusWithin != t.focus:
			old := t.focusWithin
			t.focusWithin = t.focus
			focusWithinChanged(old, t.focusWithin)
		default:
			return
		}
	}
}

func (t *Tree) focusSynced() { t.focusSyncing = false }

// focusWithinChanged tells the listeners above old that do not hold target that
// the focus left them, and those above target that did not hold old that it
// came in. Focus moves seldom, so the walks need no storage.
func focusWithinChanged(old, target FocusTarget) {
	var from, to RenderObject
	if old != nil {
		from = old
	}
	if target != nil {
		to = target
	}
	for ro := from; ro != nil; ro = ro.Parent() {
		if l, ok := ro.(FocusWithinListener); ok && !isAncestor(ro, to) {
			l.FocusWithinChanged(false)
		}
	}
	for ro := to; ro != nil; ro = ro.Parent() {
		if l, ok := ro.(FocusWithinListener); ok && !isAncestor(ro, from) {
			l.FocusWithinChanged(true)
		}
	}
}

// isAncestor reports whether a is ro or one of its ancestors.
func isAncestor(a, ro RenderObject) bool {
	for ; ro != nil; ro = ro.Parent() {
		if ro == a {
			return true
		}
	}
	return false
}

func (t *Tree) clearFocus() { t.RequestFocus(nil) }

// WindowFocused reports whether the window has the keyboard.
func (t *Tree) WindowFocused() bool { return t.windowFocused }

// focusOnPress gives the keyboard to the nearest object that takes focus at or
// above the one that handled a press, path[0], as a native button takes it on
// click. A FocusTarget that does not take focus now (CanFocus is false, or it
// is inside a hidden Hider) is passed over, so that a Focus around an
// InteractiveViewer without keys still gets the press. If there is none, the
// focus stays where it is, so that dragging a scroll bar keeps a field in focus.
func (t *Tree) focusOnPress(path []RenderObject) {
	for _, ro := range path {
		if f, ok := ro.(FocusTarget); ok && f.CanFocus() && !insideHidden(ro) {
			t.RequestFocus(f)
			return
		}
	}
}

// FocusVisibleListener is a FocusTarget that wants to know when the focus starts
// or stops showing while it has it. See Tree.FocusVisible.
type FocusVisibleListener interface {
	FocusVisibleChanged(visible bool)
}

// FocusVisible reports whether the focus should show, as a ring: the last input
// came from the keyboard, or from a screen reader, rather than the pointer. It is
// true until the first press.
func (t *Tree) FocusVisible() bool { return t.focusVisible }

// FocusVisible reports whether the focus should show in the tree of the object.
// See Tree.FocusVisible.
func (b *RenderBase) FocusVisible() bool { return b.tree != nil && b.tree.focusVisible }

func (t *Tree) setFocusVisible(v bool) {
	if t.focusVisible == v {
		return
	}
	t.focusVisible = v
	if l, ok := t.focus.(FocusVisibleListener); ok {
		l.FocusVisibleChanged(v)
	}
}

// SetWindowFocused reports whether the window has the keyboard. The object in
// focus stays the same, and gets it back with the window.
func (t *Tree) SetWindowFocused(focused bool) {
	if t.windowFocused == focused {
		return
	}
	t.windowFocused = focused
	// Alt+Tab leaves with Alt down, and its release must not open a menu.
	t.altDown = false
	// The callback may move the focus, or be running inside a move already.
	t.syncFocus()
}

// DispatchKey delivers a key press to what has the focus, and then to the focus
// targets above it, the way Escape reaches a dialog from a button inside it.
// Unused Tab presses move the focus.
func (t *Tree) DispatchKey(e KeyEvent) {
	if t.altPress(&e) {
		return
	}
	t.dispatchKey(e)
}

// dispatchKey is DispatchKey after the check for a lone Alt.
func (t *Tree) dispatchKey(e KeyEvent) {
	t.dropHiddenFocus()
	t.setFocusVisible(true)
	if forward, ok := navigationKey(&e); ok && t.navigateFromFocus(forward) {
		return
	}
	if t.focus != nil {
		for ro := RenderObject(t.focus); ro != nil; ro = ro.Parent() {
			if f, ok := ro.(FocusTarget); ok && f.HandleKey(&e) {
				return
			}
		}
	}
	if t.dispatchShortcut(&e) {
		return
	}
	if e.Key == input.KeyTab && e.Modifiers&(input.ModControl|input.ModAlt) == 0 {
		t.moveFocus(e.Modifiers&input.ModShift != 0)
	}
}

// DispatchText delivers typed or committed text.
func (t *Tree) DispatchText(s string) {
	if t.focus != nil {
		t.focus.HandleText(s)
	}
}

// DispatchComposition delivers the state of an IME composition.
func (t *Tree) DispatchComposition(c *input.Composition) {
	if t.focus != nil {
		t.focus.HandleComposition(c)
	}
}

// moveFocus goes to the next object that takes focus: first those with a
// FocusOrder above zero, lowest first, then the others in the order of painting.
func (t *Tree) moveFocus(backward bool) {
	root := t.rootRO
	if scope := modalScope(root); scope != nil {
		root = scope
	}
	t.anchorAt = -1
	t.anchorEntry = focusEntry{}
	t.order = t.collectFocus(root, t.order[:0], false, 0)
	n := len(t.order)
	if n == 0 {
		return
	}
	slices.SortStableFunc(t.order, compareFocusEntries)
	if t.anchorAt >= 0 {
		// anchorAt was the place of the anchor in painting order. After sorting
		// it is the first object that comes after it.
		t.anchorAt = n
		for i := range t.order {
			if compareFocusEntries(t.order[i], t.anchorEntry) >= 0 {
				t.anchorAt = i
				break
			}
		}
	}
	at := -1
	for i, e := range t.order {
		if e.target == t.focus {
			at = i
			break
		}
	}
	next := at + 1
	switch {
	case at < 0 && t.anchor != nil && t.anchorAt >= 0:
		// The anchor is hidden, so it is not in order: anchorAt is the place of
		// the next object after it.
		next = t.anchorAt
		if backward {
			next = t.anchorAt - 1
		}
	case backward:
		next = at - 1
		if at < 0 {
			next = n - 1
		}
	}
	t.RequestFocus(t.order[(next+n)%n].target)
	clear(t.order)
}

// focusEntry is an object in the order of Tab, with its FocusOrder and its
// place in the order of painting.
type focusEntry struct {
	target FocusTarget
	order  int
	seq    int
}

func compareFocusEntries(a, b focusEntry) int {
	if c := cmp.Compare(focusRank(a.order), focusRank(b.order)); c != 0 {
		return c
	}
	return cmp.Compare(a.seq, b.seq)
}

// focusRank puts the objects without an order after all those with one.
func focusRank(order int) int {
	if order <= 0 {
		return math.MaxInt
	}
	return order
}

// FocusOrderer is a RenderObject that sets the place in the order of Tab of the
// objects below it, as the tabindex of HTML does: those with an order above
// zero come first, lowest first, and those with zero follow in the order of
// painting. The nearest one above an object wins.
type FocusOrderer interface {
	RenderObject
	FocusOrder() int
}

// dropHiddenFocus takes the focus from an object that no longer takes it, or
// that is inside a hidden subtree, so that keys cannot reach what cannot be
// seen or used, and remembers it as the anchor for the next Tab. It walks up
// from the focus only, without allocating.
func (t *Tree) dropHiddenFocus() {
	if t.focus == nil || t.focus.CanFocus() && !insideHidden(t.focus) {
		return
	}
	f := t.focus
	t.RequestFocus(nil)
	t.anchor = f
}

// insideHidden reports whether ro or one of its ancestors hides its subtree.
func insideHidden(ro RenderObject) bool {
	for ; ro != nil; ro = ro.Parent() {
		if isHidden(ro) {
			return true
		}
	}
	return false
}

// modalScope returns the modal FocusScope painted last below ro, or nil.
func modalScope(ro RenderObject) RenderObject {
	if isHidden(ro) {
		return nil
	}
	for i := ro.ChildCount() - 1; i >= 0; i-- {
		if s := modalScope(ro.ChildAt(i)); s != nil {
			return s
		}
	}
	if s, ok := ro.(FocusScope); ok && s.Modal() {
		return s
	}
	return nil
}

// collectFocus appends the objects below ro that take focus, in the order of
// painting, with the order of the nearest FocusOrderer. It looks into hidden
// subtrees only to find the anchor, whose place it records in anchorAt.
func (t *Tree) collectFocus(ro RenderObject, out []focusEntry, hidden bool, order int) []focusEntry {
	hidden = hidden || isHidden(ro)
	if hidden && t.anchor == nil {
		return out
	}
	if o, ok := ro.(FocusOrderer); ok {
		order = o.FocusOrder()
	}
	if f, ok := ro.(FocusTarget); ok {
		if t.anchor != nil && f == t.anchor {
			t.anchorAt = len(out)
			t.anchorEntry = focusEntry{order: order, seq: len(out)}
		}
		if !hidden && f.CanFocus() {
			out = append(out, focusEntry{target: f, order: order, seq: len(out)})
		}
	}
	for i := range ro.ChildCount() {
		out = t.collectFocus(ro.ChildAt(i), out, hidden, order)
	}
	return out
}

// detached forgets an object that left the tree. It does not call FocusChanged:
// this runs during Build, where a handler must not call SetState, and the object
// has no use for the news.
func (t *Tree) detached(ro RenderObject) {
	if f, ok := ro.(FocusTarget); ok && f == t.focus {
		t.focus = nil
		t.Host().SetIMEEnabled(false)
		// A password field that leaves with the keyboard is not told, so it
		// cannot end the secure input it began.
		if s, ok := t.Host().(SecureInputHost); ok {
			s.SetSecureInput(false)
		}
	}
	// Nor is it told later, by a move that finds it still waiting for news.
	if f, ok := ro.(FocusTarget); ok && f == t.focusTold {
		t.focusTold = nil
	}
	if f, ok := ro.(FocusTarget); ok && f == t.focusWithin {
		t.focusWithin = nil
	}
	if f, ok := ro.(FocusTarget); ok && f == t.anchor {
		t.anchor = nil
	}
	if d, ok := ro.(DropTarget); ok && d == t.dropTarget {
		t.dropTarget = nil
	}
}
