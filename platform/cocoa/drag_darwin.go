//go:build darwin && arm64

package cocoa

import (
	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
)

// Drag and drop through AppKit. The content view is the NSDraggingDestination
// of its window and the NSDraggingSource of the drags that start in it; both
// sets of methods are closures over a dragState of the Platform, like the
// other methods of the view. AppKit calls them on the main thread, inside
// nextEventMatchingMask or sendEvent, so the drop handler is asked at once.
// See docs/ADR/0050-drag-and-drop.md.

// dragState is what drag and drop keeps between the calls of AppKit. There is
// one drag at a time on the Mac.
type dragState struct {
	// over is the view that a drag from anywhere is over, kinds what it
	// carries and last what the handler answered for it.
	over  objc.ID
	kinds input.DataKinds
	last  input.DropEffect

	// done is the block that StartDrag gave for the drag out of a view of
	// this Platform in progress, which owns it; zero if there is none.
	done    objc.Block
	allowed input.DropEffect
	source  objc.ID
}

// registerDropTypes makes view a destination for the drags of files and text.
func registerDropTypes(view objc.ID) {
	view.Send(sel("registerForDraggedTypes:"), nsArray(nsString(typeFileURL), nsString(typeString)))
}

// dragViewMethods returns the methods of NSDraggingDestination and
// NSDraggingSource that the content view adds, and the private method through
// which StartDrag hands its drag to them.
func (p *Platform) dragViewMethods() []objc.MethodDef {
	st := &dragState{}
	over := func(self objc.ID, _ objc.SEL, sender objc.ID) uint64 {
		return p.dragOver(st, self, sender)
	}
	return []objc.MethodDef{
		// NSDraggingDestination
		{Cmd: sel("draggingEntered:"), Fn: func(self objc.ID, _ objc.SEL, sender objc.ID) uint64 {
			st.over, st.last = self, input.DropNone
			st.kinds = pasteboardKinds(sender.Send(sel("draggingPasteboard")))
			return p.dragOver(st, self, sender)
		}},
		{Cmd: sel("draggingUpdated:"), Fn: over},
		{Cmd: sel("wantsPeriodicDraggingUpdates"), Fn: func(objc.ID, objc.SEL) bool { return false }},
		{Cmd: sel("draggingExited:"), Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) { p.dragLeave(st, self) }},
		{Cmd: sel("draggingEnded:"), Fn: func(_ objc.ID, _ objc.SEL, _ objc.ID) {
			// A drag canceled over the view ends without leaving it.
			if st.over != 0 {
				p.dragLeave(st, st.over)
			}
		}},
		{Cmd: sel("prepareForDragOperation:"), Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) bool {
			return self == st.over && st.last != input.DropNone
		}},
		{Cmd: sel("performDragOperation:"), Fn: func(self objc.ID, _ objc.SEL, sender objc.ID) bool {
			return p.dragDrop(st, self, sender)
		}},

		// NSDraggingSource
		{Cmd: sel("draggingSession:sourceOperationMaskForDraggingContext:"), Fn: func(objc.ID, objc.SEL, objc.ID, int64) uint64 {
			return maskOf(st.allowed)
		}},
		{Cmd: sel("draggingSession:movedToPoint:"), Fn: func(objc.ID, objc.SEL, objc.ID, NSPoint) {
			// AppKit holds the main thread for the drag: the application
			// draws from here, as in a live resize.
			p.runDragHandler(st)
		}},
		{Cmd: sel("draggingSession:endedAtPoint:operation:"), Fn: func(self objc.ID, _ objc.SEL, _ objc.ID, _ NSPoint, op uint64) {
			p.dragEnded(st, self, op)
		}},

		// gowakuBeginDrag:allowed:done: starts a drag of items from the view,
		// taking the block done on success. It is not part of any protocol.
		{Cmd: sel("gowakuBeginDrag:allowed:done:"), Fn: func(self objc.ID, _ objc.SEL, items objc.ID, allowed uint64, done objc.Block) bool {
			return p.beginDrag(st, self, items, input.DropEffect(allowed), done)
		}},
	}
}

// pasteboardKinds returns the kinds of data that a pasteboard holds.
func pasteboardKinds(board objc.ID) input.DataKinds {
	var k input.DataKinds
	types := board.Send(sel("types"))
	for i := range count(types) {
		k |= kindOfType(goString(at(types, i)))
	}
	return k
}

// dropAsk hands a drag over view to the drop handler of its window and returns
// what the handler answered, within what the source allows.
func (p *Platform) dropAsk(st *dragState, view, sender objc.ID, phase platform.DropPhase, data input.DragData) input.DropEffect {
	w := p.byView[view]
	if w == nil || w.closed || w.drop == nil {
		return input.DropNone
	}
	e := platform.DropEvent{Phase: phase, Kinds: st.kinds, Data: data,
		Modifiers: modifiersOf(objc.Send[uint64](class("NSEvent"), sel("modifierFlags")))}
	if sender != 0 {
		inWindow := objc.Send[NSPoint](sender, sel("draggingLocation"))
		local := objc.Send[NSPoint](view, sel("convertPoint:fromView:"), inWindow, objc.ID(0))
		e.X, e.Y = float32(local.X)*w.scale, float32(local.Y)*w.scale
		e.Allowed = allowedOf(objc.Send[uint64](sender, sel("draggingSourceOperationMask")))
	}
	effect := w.drop(&e) & e.Allowed
	p.dragRedraw(st, w)
	return effect
}

// dragRedraw gives the application a frame after a drag step: the handler
// may have changed what shows, and WaitEvent would not return otherwise.
func (p *Platform) dragRedraw(st *dragState, w *Window) {
	p.enqueue(platform.Event{Kind: platform.EventRedraw, Window: w.id})
	p.runDragHandler(st)
}

// runDragHandler runs the modal handler while a drag out of this Platform
// holds the main thread.
func (p *Platform) runDragHandler(st *dragState) {
	if st.done == 0 || p.modalHandler == nil || p.inHandler {
		return
	}
	p.inHandler = true
	p.modalHandler()
	p.inHandler = false
}

func (p *Platform) dragOver(st *dragState, view, sender objc.ID) uint64 {
	if view != st.over {
		return NSDragOperationNone
	}
	st.last = p.dropAsk(st, view, sender, platform.DropOver, input.DragData{})
	return operationOf(st.last, objc.Send[uint64](sender, sel("draggingSourceOperationMask")))
}

func (p *Platform) dragLeave(st *dragState, view objc.ID) {
	if view != st.over {
		return
	}
	p.dropAsk(st, view, 0, platform.DropLeave, input.DragData{})
	st.over, st.kinds, st.last = 0, 0, input.DropNone
}

func (p *Platform) dragDrop(st *dragState, view, sender objc.ID) bool {
	if view != st.over {
		return false
	}
	data := readDragData(sender.Send(sel("draggingPasteboard")), st.kinds)
	effect := p.dropAsk(st, view, sender, platform.DropDrop, data)
	st.over, st.kinds, st.last = 0, 0, input.DropNone
	return effect != input.DropNone
}

// readDragData reads the files and the text of a dragged pasteboard.
func readDragData(board objc.ID, kinds input.DataKinds) input.DragData {
	var d input.DragData
	if kinds&input.DataFiles != 0 {
		d.Files = readFiles(board)
	}
	if kinds&input.DataText != 0 {
		d.Text = goString(board.Send(sel("stringForType:"), nsString(typeString)))
	}
	return d
}

// SetDropHandler implements platform.Dragging.
func (w *Window) SetDropHandler(fn func(e *platform.DropEvent) input.DropEffect) { w.drop = fn }

// StartDrag implements platform.Dragging. The drag starts from the press in
// progress, the last mouse down of the window, which AppKit requires: an
// NSDraggingSession whose items are the files, as file URLs, and the text. It
// shows no image, as on the other platforms.
func (w *Window) StartDrag(d input.DragData, allowed input.DropEffect, done func(input.DropEffect)) {
	if done == nil {
		done = func(input.DropEffect) {}
	}
	if w.closed || w.lastMouseDown == 0 || d.Kinds() == 0 || allowed == input.DropNone {
		done(input.DropNone)
		return
	}
	started := false
	w.p.libs.pool(func() {
		block := objc.NewBlock(func(_ objc.Block, effect uint64) { done(input.DropEffect(effect)) })
		started = objc.Send[bool](w.view, sel("gowakuBeginDrag:allowed:done:"), w.draggingItems(d), uint64(allowed), block)
		if !started {
			block.Release()
		}
	})
	if !started {
		done(input.DropNone)
	}
}

// draggingItems returns the NSDraggingItems of d, with frames around the
// press and no image.
func (w *Window) draggingItems(d input.DragData) objc.ID {
	inWindow := objc.Send[NSPoint](w.lastMouseDown, sel("locationInWindow"))
	press := objc.Send[NSPoint](w.view, sel("convertPoint:fromView:"), inWindow, objc.ID(0))
	frame := NSRect{Origin: NSPoint{press.X - 16, press.Y - 16}, Size: NSSize{32, 32}}
	items := make([]objc.ID, 0, len(d.Files)+1)
	add := func(writer objc.ID) {
		item := class("NSDraggingItem").Send(sel("alloc")).Send(sel("initWithPasteboardWriter:"), writer).Send(sel("autorelease"))
		item.Send(sel("setDraggingFrame:contents:"), frame, objc.ID(0))
		items = append(items, item)
	}
	for _, f := range d.Files {
		add(class("NSURL").Send(sel("fileURLWithPath:"), nsString(f)))
	}
	if d.Text != "" {
		text := class("NSPasteboardItem").Send(sel("new")).Send(sel("autorelease"))
		text.Send(sel("setString:forType:"), nsString(d.Text), nsString(typeString))
		add(text)
	}
	return nsArray(items...)
}

// beginDrag starts the drag of items from view. It refuses while another drag
// of this Platform goes on.
func (p *Platform) beginDrag(st *dragState, view, items objc.ID, allowed input.DropEffect, done objc.Block) bool {
	w := p.byView[view]
	if w == nil || st.done != 0 || w.lastMouseDown == 0 || count(items) == 0 {
		return false
	}
	st.done, st.allowed, st.source = done, allowed, view
	session := view.Send(sel("beginDraggingSessionWithItems:event:source:"), items, w.lastMouseDown, view)
	if session == 0 {
		st.done, st.allowed, st.source = 0, input.DropNone, 0
		return false
	}
	return true
}

// dragEnded finishes the drag out of view with what the drop did.
func (p *Platform) dragEnded(st *dragState, view objc.ID, op uint64) {
	if st.done == 0 || view != st.source {
		return
	}
	done, effect := st.done, effectOf(op)&st.allowed
	st.done, st.allowed, st.source = 0, input.DropNone, 0
	if w := p.byView[view]; w != nil {
		p.enqueue(platform.Event{Kind: platform.EventRedraw, Window: w.id})
	}
	done.Invoke(uint64(effect))
	done.Release()
}
