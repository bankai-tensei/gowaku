//go:build darwin && arm64

package cocoa

import (
	"fmt"
	"math"

	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
)

// notchPoints is how many points of a precise scroll, from a touchpad or a
// Magic Mouse, make one wheel notch: the 100 logical pixels that a notch
// scrolls (ADR 0036). Fractions of a notch move at once.
const notchPoints = 100

// registerClasses makes the content view and the delegate of this Platform.
func (p *Platform) registerClasses() error {
	serial := classSerial.Add(1)
	view, err := objc.RegisterClass(className("View", serial), objc.GetClass("NSView"),
		[]*objc.Protocol{objc.GetProtocol("NSTextInputClient")}, nil, p.viewMethods())
	if err != nil {
		return fmt.Errorf("cocoa: register the view class: %w", err)
	}
	delegate, err := objc.RegisterClass(className("Delegate", serial), objc.GetClass("NSObject"),
		[]*objc.Protocol{objc.GetProtocol("NSApplicationDelegate"), objc.GetProtocol("NSWindowDelegate")}, nil, p.delegateMethods())
	if err != nil {
		return fmt.Errorf("cocoa: register the delegate class: %w", err)
	}
	p.viewClass, p.delegateClass = view, delegate
	return nil
}

func (p *Platform) viewMethods() []objc.MethodDef {
	yes := func(objc.ID, objc.SEL) bool { return true }
	mouse := func(kind platform.EventKind, button input.Button) func(objc.ID, objc.SEL, objc.ID) {
		return func(self objc.ID, _ objc.SEL, ev objc.ID) { p.mouse(self, ev, kind, button) }
	}
	move := func(self objc.ID, _ objc.SEL, ev objc.ID) { p.mouse(self, ev, platform.EventPointerMove, 0) }
	methods := []objc.MethodDef{
		{Cmd: sel("acceptsFirstResponder"), Fn: yes},
		{Cmd: sel("canBecomeKeyView"), Fn: yes},
		{Cmd: sel("isFlipped"), Fn: yes},
		{Cmd: sel("acceptsFirstMouse:"), Fn: func(objc.ID, objc.SEL, objc.ID) bool { return true }},
		{Cmd: sel("mouseDown:"), Fn: mouse(platform.EventPointerDown, input.ButtonPrimary)},
		{Cmd: sel("mouseUp:"), Fn: mouse(platform.EventPointerUp, input.ButtonPrimary)},
		{Cmd: sel("rightMouseDown:"), Fn: mouse(platform.EventPointerDown, input.ButtonSecondary)},
		{Cmd: sel("rightMouseUp:"), Fn: mouse(platform.EventPointerUp, input.ButtonSecondary)},
		{Cmd: sel("otherMouseDown:"), Fn: mouse(platform.EventPointerDown, input.ButtonMiddle)},
		{Cmd: sel("otherMouseUp:"), Fn: mouse(platform.EventPointerUp, input.ButtonMiddle)},
		{Cmd: sel("mouseMoved:"), Fn: move},
		{Cmd: sel("mouseDragged:"), Fn: move},
		{Cmd: sel("rightMouseDragged:"), Fn: move},
		{Cmd: sel("otherMouseDragged:"), Fn: move},
		{Cmd: sel("mouseEntered:"), Fn: func(objc.ID, objc.SEL, objc.ID) {}},
		{Cmd: sel("mouseExited:"), Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) {
			if w := p.byView[self]; w != nil {
				p.queue.Push(platform.Event{Kind: platform.EventPointerLeave, Window: w.id})
			}
		}},
		{Cmd: sel("cursorUpdate:"), Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) {
			if w := p.byView[self]; w != nil {
				w.applyCursor()
			}
		}},
		{Cmd: sel("scrollWheel:"), Fn: func(self objc.ID, _ objc.SEL, ev objc.ID) { p.scroll(self, ev) }},
		{Cmd: sel("magnifyWithEvent:"), Fn: func(self objc.ID, _ objc.SEL, ev objc.ID) { p.magnify(self, ev) }},
		{Cmd: sel("keyDown:"), Fn: func(self objc.ID, _ objc.SEL, ev objc.ID) { p.keyDown(self, ev) }},
		{Cmd: sel("keyUp:"), Fn: func(self objc.ID, _ objc.SEL, ev objc.ID) { p.keyUp(self, ev) }},
		{Cmd: sel("viewDidChangeBackingProperties"), Fn: func(self objc.ID, _ objc.SEL) {
			if w := p.byView[self]; w != nil {
				w.resized()
			}
		}},
		{Cmd: sel("viewDidChangeEffectiveAppearance"), Fn: func(objc.ID, objc.SEL) { p.checkSettings() }},
		{Cmd: sel("inputContext"), Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
			if w := p.byView[self]; w == nil || !w.converts() {
				return 0
			}
			return objc.SendSuper[objc.ID](self, cmd)
		}},

		// NSTextInputClient
		{Cmd: sel("insertText:"), Fn: func(self objc.ID, _ objc.SEL, text objc.ID) { p.insertText(self, text) }},
		{Cmd: sel("insertText:replacementRange:"), Fn: func(self objc.ID, _ objc.SEL, text objc.ID, _ NSRange) {
			p.insertText(self, text)
		}},
		{Cmd: sel("setMarkedText:selectedRange:replacementRange:"), Fn: func(self objc.ID, _ objc.SEL, text objc.ID, selected, _ NSRange) {
			p.setMarkedText(self, text, selected)
		}},
		{Cmd: sel("unmarkText"), Fn: func(self objc.ID, _ objc.SEL) { p.unmark(self) }},
		{Cmd: sel("hasMarkedText"), Fn: func(self objc.ID, _ objc.SEL) bool {
			w := p.byView[self]
			return w != nil && w.marked != ""
		}},
		{Cmd: sel("markedRange"), Fn: func(self objc.ID, _ objc.SEL) NSRange {
			if w := p.byView[self]; w != nil && w.marked != "" {
				return NSRange{0, uint64(utf16Len(w.marked))}
			}
			return NSRange{NSNotFound, 0}
		}},
		{Cmd: sel("selectedRange"), Fn: func(objc.ID, objc.SEL) NSRange { return NSRange{NSNotFound, 0} }},
		{Cmd: sel("validAttributesForMarkedText"), Fn: func(objc.ID, objc.SEL) objc.ID {
			return nsArray(nsString("NSMarkedClauseSegment"), nsString("NSUnderline"))
		}},
		{Cmd: sel("attributedSubstringForProposedRange:actualRange:"), Fn: func(objc.ID, objc.SEL, NSRange, *NSRange) objc.ID {
			return 0
		}},
		{Cmd: sel("characterIndexForPoint:"), Fn: func(objc.ID, objc.SEL, NSPoint) uint64 { return NSNotFound }},
		{Cmd: sel("firstRectForCharacterRange:actualRange:"), Fn: func(self objc.ID, _ objc.SEL, _ NSRange, _ *NSRange) NSRect {
			return p.caretRect(self)
		}},
		{Cmd: sel("doCommandBySelector:"), Fn: func(objc.ID, objc.SEL, objc.SEL) {
			// The key went to the application as EventKeyDown already.
		}},
	}
	// Drag and drop and NSAccessibility add methods of their own, in
	// drag_darwin.go and accessibility_darwin.go.
	methods = append(methods, p.dragViewMethods()...)
	return append(methods, p.accessibilityViewMethods()...)
}

func (p *Platform) delegateMethods() []objc.MethodDef {
	window := func(notification objc.ID) *Window {
		return p.byWindow[notification.Send(sel("object"))]
	}
	return []objc.MethodDef{
		{Cmd: sel("applicationDidFinishLaunching:"), Fn: func(objc.ID, objc.SEL, objc.ID) {
			p.app.Send(sel("stop:"), objc.ID(0))
			// stop takes effect after the next event, so post one.
			p.postWake()
		}},
		{Cmd: sel("applicationShouldTerminate:"), Fn: func(objc.ID, objc.SEL, objc.ID) uint64 {
			// The application decides, and quits by closing its windows.
			p.enqueue(platform.Event{Kind: platform.EventQuit})
			return NSTerminateCancel
		}},
		{Cmd: sel("applicationShouldHandleReopen:hasVisibleWindows:"), Fn: func(_ objc.ID, _ objc.SEL, _ objc.ID, visible bool) bool {
			if !visible {
				p.enqueue(platform.Event{Kind: platform.EventReopen})
			}
			return true
		}},
		{Cmd: sel("windowShouldClose:"), Fn: func(_ objc.ID, _ objc.SEL, sender objc.ID) bool {
			if w := p.byWindow[sender]; w != nil {
				p.enqueue(platform.Event{Kind: platform.EventClose, Window: w.id})
			}
			return false
		}},
		{Cmd: sel("windowDidResize:"), Fn: func(_ objc.ID, _ objc.SEL, n objc.ID) {
			if w := window(n); w != nil {
				w.resized()
			}
		}},
		{Cmd: sel("windowDidChangeBackingProperties:"), Fn: func(_ objc.ID, _ objc.SEL, n objc.ID) {
			if w := window(n); w != nil {
				w.resized()
			}
		}},
		{Cmd: sel("windowWillStartLiveResize:"), Fn: func(objc.ID, objc.SEL, objc.ID) { p.liveResize++ }},
		{Cmd: sel("windowDidEndLiveResize:"), Fn: func(objc.ID, objc.SEL, objc.ID) {
			if p.liveResize > 0 {
				p.liveResize--
			}
		}},
		{Cmd: sel("windowDidMiniaturize:"), Fn: func(_ objc.ID, _ objc.SEL, n objc.ID) {
			if w := window(n); w != nil {
				p.enqueue(platform.Event{Kind: platform.EventResize, Window: w.id, Scale: w.scale})
			}
		}},
		{Cmd: sel("windowDidDeminiaturize:"), Fn: func(_ objc.ID, _ objc.SEL, n objc.ID) {
			if w := window(n); w != nil {
				w.pushSize()
				p.enqueue(platform.Event{Kind: platform.EventRedraw, Window: w.id})
			}
		}},
		{Cmd: sel("gowakuMenu:"), Fn: func(_ objc.ID, _ objc.SEL, sender objc.ID) { p.chooseMenuItem(sender) }},
		{Cmd: sel("gowakuTray:"), Fn: func(_ objc.ID, _ objc.SEL, sender objc.ID) { p.chooseTray(sender) }},
		{Cmd: sel("userNotificationCenter:didReceiveNotificationResponse:withCompletionHandler:"),
			Fn: func(_ objc.ID, _ objc.SEL, _, _ objc.ID, completion objc.Block) { p.notificationClicked(completion) }},
		{Cmd: sel("userNotificationCenter:willPresentNotification:withCompletionHandler:"),
			Fn: func(_ objc.ID, _ objc.SEL, _, _ objc.ID, completion objc.Block) { presentNotification(completion) }},
		{Cmd: sel("windowDidBecomeKey:"), Fn: func(_ objc.ID, _ objc.SEL, n objc.ID) {
			if w := window(n); w != nil {
				w.key = true
				w.syncSecureInput()
				if p.menuOwner != w && (w.menu != nil || p.menuOwner != nil) {
					p.installMenu(w)
				}
				p.enqueue(platform.Event{Kind: platform.EventFocus, Window: w.id})
				p.checkSettings()
			}
		}},
		{Cmd: sel("windowDidResignKey:"), Fn: func(_ objc.ID, _ objc.SEL, n objc.ID) {
			if w := window(n); w != nil {
				w.key = false
				w.syncSecureInput()
				p.enqueue(platform.Event{Kind: platform.EventBlur, Window: w.id})
			}
		}},
		{Cmd: sel("windowDidEnterFullScreen:"), Fn: func(_ objc.ID, _ objc.SEL, n objc.ID) {
			// The size may already be known; the state is news.
			if w := window(n); w != nil {
				w.updateSize()
				w.pushSize()
			}
		}},
		{Cmd: sel("windowDidExitFullScreen:"), Fn: func(_ objc.ID, _ objc.SEL, n objc.ID) {
			if w := window(n); w != nil {
				w.updateSize()
				w.pushSize()
			}
		}},
		{Cmd: sel("windowDidChangeOcclusionState:"), Fn: func(_ objc.ID, _ objc.SEL, n objc.ID) {
			// Hidden, the window stops drawing as a minimized one does; shown
			// again, it draws what it missed.
			if w := window(n); w != nil {
				w.pushSize()
				if !w.hidden() {
					p.enqueue(platform.Event{Kind: platform.EventRedraw, Window: w.id})
				}
			}
		}},
	}
}

// location returns where an event happened in physical pixels of the client
// area of w.
func (w *Window) location(ev objc.ID) (x, y float32) {
	inWindow := objc.Send[NSPoint](ev, sel("locationInWindow"))
	local := objc.Send[NSPoint](w.view, sel("convertPoint:fromView:"), inWindow, objc.ID(0))
	return float32(local.X) * w.scale, float32(local.Y) * w.scale
}

// eventTime returns the millisecond clock of an event.
func eventTime(ev objc.ID) uint32 {
	return uint32(uint64(objc.Send[float64](ev, sel("timestamp")) * 1000))
}

// eventKey returns the key of a key event. For a punctuation key it asks what
// the key types without modifiers on the layout of the event, which
// charactersByApplyingModifiers: (macOS 10.15, below what Go needs) answers.
func eventKey(ev objc.ID) input.KeyCode {
	code := objc.Send[uint16](ev, sel("keyCode"))
	if !isPunctuationCode(code) {
		return keyCodeOf(code, "")
	}
	return keyCodeOf(code, goString(ev.Send(sel("charactersByApplyingModifiers:"), uint64(0))))
}

func eventModifiers(ev objc.ID) input.Modifiers {
	return modifiersOf(objc.Send[uint64](ev, sel("modifierFlags")))
}

func (p *Platform) mouse(view, ev objc.ID, kind platform.EventKind, button input.Button) {
	w := p.byView[view]
	if w == nil {
		return
	}
	flags := objc.Send[uint64](ev, sel("modifierFlags"))
	switch {
	case button == input.ButtonMiddle:
		switch objc.Send[int64](ev, sel("buttonNumber")) {
		case 3:
			button = input.ButtonBack
		case 4:
			button = input.ButtonForward
		}
	case button == input.ButtonPrimary && kind == platform.EventPointerDown:
		// Control-click is the secondary click of the Mac.
		w.secondaryByControl = flags&NSEventModifierFlagControl != 0
		if w.secondaryByControl {
			button = input.ButtonSecondary
		}
	case button == input.ButtonPrimary && kind == platform.EventPointerUp && w.secondaryByControl:
		button = input.ButtonSecondary
		w.secondaryByControl = false
	}
	if kind == platform.EventPointerDown {
		if w.lastMouseDown != 0 {
			w.lastMouseDown.Send(sel("release"))
		}
		w.lastMouseDown = ev.Send(sel("retain"))
	}
	x, y := w.location(ev)
	p.queue.Push(platform.Event{
		Kind: kind, Window: w.id, X: x, Y: y, Button: button,
		Modifiers: modifiersOf(flags), Time: eventTime(ev),
	})
	p.runModalHandler()
}

func (p *Platform) scroll(view, ev objc.ID) {
	w := p.byView[view]
	if w == nil {
		return
	}
	dx := objc.Send[float64](ev, sel("scrollingDeltaX"))
	dy := objc.Send[float64](ev, sel("scrollingDeltaY"))
	if objc.Send[bool](ev, sel("hasPreciseScrollingDeltas")) {
		dx, dy = dx/notchPoints, dy/notchPoints
	}
	if dx == 0 && dy == 0 {
		return
	}
	x, y := w.location(ev)
	// Up is positive, as for the wheel of Windows. Right is positive too,
	// where AppKit counts a scroll to the left.
	p.queue.Push(platform.Event{
		Kind: platform.EventScroll, Window: w.id, X: x, Y: y,
		ScrollX: float32(-dx), ScrollY: float32(dy),
		Modifiers: eventModifiers(ev), Time: eventTime(ev),
	})
}

func (p *Platform) magnify(view, ev objc.ID) {
	w := p.byView[view]
	if w == nil {
		return
	}
	factor := 1 + objc.Send[float64](ev, sel("magnification"))
	if factor <= 0 || math.IsNaN(factor) {
		return
	}
	x, y := w.location(ev)
	p.queue.Push(platform.Event{
		Kind: platform.EventPinch, Window: w.id, X: x, Y: y, Pinch: float32(factor),
		Modifiers: eventModifiers(ev), Time: eventTime(ev),
	})
}

// keyDown sends the key, unless the IME takes it, and then the text it types.
// With Command the key is a shortcut and types nothing.
func (p *Platform) keyDown(view, ev objc.ID) {
	w := p.byView[view]
	if w == nil {
		return
	}
	flags := objc.Send[uint64](ev, sel("modifierFlags"))
	key := platform.Event{
		Kind: platform.EventKeyDown, Window: w.id,
		Key:       eventKey(ev),
		Repeat:    objc.Send[bool](ev, sel("isARepeat")),
		Modifiers: modifiersOf(flags), Time: eventTime(ev),
	}
	if flags&NSEventModifierFlagCommand != 0 {
		p.queue.Push(key)
		return
	}
	composing := w.marked != ""
	p.pending = p.pending[:0]
	p.interpreted = true
	view.Send(sel("interpretKeyEvents:"), nsArray(ev))
	p.interpreted = false
	// A key that the IME works on, or that starts a composition, is its own.
	if !composing && w.marked == "" {
		p.queue.Push(key)
	}
	for i := range p.pending {
		p.queue.Push(p.pending[i])
	}
	p.pending = p.pending[:0]
	p.runModalHandler()
}

func (p *Platform) keyUp(view, ev objc.ID) {
	w := p.byView[view]
	if w == nil || w.marked != "" {
		return
	}
	p.queue.Push(platform.Event{
		Kind: platform.EventKeyUp, Window: w.id,
		Key:       eventKey(ev),
		Modifiers: eventModifiers(ev), Time: eventTime(ev),
	})
}

// push queues an event that interpretKeyEvents produced after the key.
func (p *Platform) push(e platform.Event) {
	if p.interpreted {
		p.pending = append(p.pending, e)
		return
	}
	p.queue.Push(e)
}

// stringOf returns the text of an NSString or an NSAttributedString.
func stringOf(text objc.ID) string {
	if objc.Send[bool](text, sel("isKindOfClass:"), objc.GetClass("NSAttributedString")) {
		text = text.Send(sel("string"))
	}
	return goString(text)
}

func (p *Platform) insertText(view, text objc.ID) {
	w := p.byView[view]
	if w == nil {
		return
	}
	s := stringOf(text)
	if w.marked != "" {
		w.marked = ""
		p.push(platform.Event{Kind: platform.EventComposition, Window: w.id})
	}
	if s = typedText(s); s != "" {
		p.push(platform.Event{Kind: platform.EventText, Window: w.id, Text: s})
	}
}

func (p *Platform) setMarkedText(view, text objc.ID, selected NSRange) {
	w := p.byView[view]
	if w == nil {
		return
	}
	s := stringOf(text)
	if s == "" {
		p.unmark(view)
		return
	}
	var runs []markedRun
	if objc.Send[bool](text, sel("isKindOfClass:"), objc.GetClass("NSAttributedString")) {
		runs = markedRuns(text, utf16Len(s))
	}
	w.marked = s
	caret := int(selected.Location + selected.Length)
	if selected.Location == NSNotFound {
		caret = utf16Len(s)
	}
	p.push(platform.Event{Kind: platform.EventComposition, Window: w.id, Composition: composition(s, runs, caret)})
}

// markedRuns reads the runs of the attributes of marked text.
func markedRuns(text objc.ID, length int) []markedRun {
	var runs []markedRun
	underline := nsString("NSUnderline")
	for i := 0; i < length; {
		var r NSRange
		attrs := text.Send(sel("attributesAtIndex:effectiveRange:"), uint64(i), &r)
		end := int(r.Location + r.Length)
		if end <= i {
			break
		}
		thick := false
		if style := attrs.Send(sel("objectForKey:"), underline); style != 0 {
			thick = objc.Send[int64](style, sel("integerValue"))&NSUnderlineStyleThick != 0
		}
		runs = append(runs, markedRun{start: i, end: end, thick: thick})
		i = end
	}
	return runs
}

func (p *Platform) unmark(view objc.ID) {
	w := p.byView[view]
	if w == nil || w.marked == "" {
		return
	}
	w.marked = ""
	p.push(platform.Event{Kind: platform.EventComposition, Window: w.id})
}

// caretRect returns where the IME puts its candidates: under the caret, or
// beside it in vertical text, in the coordinates of the screen.
func (p *Platform) caretRect(view objc.ID) NSRect {
	w := p.byView[view]
	if w == nil || w.scale <= 0 {
		return NSRect{}
	}
	s := float64(w.scale)
	local := NSRect{
		Origin: NSPoint{float64(w.caret[0]) / s, float64(w.caret[1]) / s},
		Size:   NSSize{max(float64(w.caret[2])/s, 1), max(float64(w.caret[3])/s, 1)},
	}
	inWindow := objc.Send[NSRect](view, sel("convertRect:toView:"), local, objc.ID(0))
	return objc.Send[NSRect](w.window, sel("convertRectToScreen:"), inWindow)
}
