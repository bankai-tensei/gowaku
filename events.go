package gowaku

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
)

// handle dispatches one event of the platform to its window.
func (ap *app) handle(ev platform.Event) {
	switch ev.Kind {
	case platform.EventBrightness:
		ap.followOS()
		return
	case platform.EventSettings:
		ap.followSettings()
		return
	case platform.EventQuit:
		ap.quitRequested()
		return
	case platform.EventReopen:
		ap.reopen()
		return
	}
	w := ap.windows[ev.Window]
	if w == nil || w.closing {
		return
	}
	if inspecting {
		if ev.Kind == platform.EventKeyDown && ap.inspectKey(w, &ev) {
			return
		}
		if ap.inspectPointer(w, &ev) {
			return
		}
	}
	if len(w.modals) > 0 && blockedByModal(ev.Kind) {
		if ev.Kind == platform.EventPointerDown || ev.Kind == platform.EventClose {
			w.modals[len(w.modals)-1].native.Activate()
		}
		return
	}
	switch ev.Kind {
	case platform.EventRedraw:
		w.tree.MarkNeedsPaint()
	case platform.EventResize:
		if err := w.surface.Resize(ev.PhysicalWidth, ev.PhysicalHeight, ev.Scale); err != nil {
			panic("gowaku: resize surface: " + err.Error())
		}
		w.resize(ev.PhysicalWidth, ev.PhysicalHeight, ev.Scale)
		w.syncFrame()
	case platform.EventClose:
		if ap.askClose(w) {
			ap.close(w)
		}
	case platform.EventDestroyed:
		ap.close(w)
	case platform.EventPointerMove:
		w.tree.DispatchPointer(w.pointer(core.PointerMove, &ev))
	case platform.EventPointerDown:
		w.tree.DispatchPointer(w.pointer(core.PointerDown, &ev))
	case platform.EventPointerUp:
		w.tree.DispatchPointer(w.pointer(core.PointerUp, &ev))
	case platform.EventPointerLeave:
		w.tree.PointerLeft()
	case platform.EventScroll:
		e := w.pointer(core.PointerScroll, &ev)
		e.Scroll = geom.Offset{X: ev.ScrollX, Y: ev.ScrollY}
		w.tree.DispatchPointer(e)
	case platform.EventPinch:
		e := w.pointer(core.PointerPinch, &ev)
		e.Pinch = ev.Pinch
		w.tree.DispatchPointer(e)
	case platform.EventKeyDown:
		w.tree.DispatchKey(core.KeyEvent{Key: ev.Key, Modifiers: ev.Modifiers, Repeat: ev.Repeat})
	case platform.EventKeyUp:
		w.tree.DispatchKeyUp(core.KeyEvent{Key: ev.Key, Modifiers: ev.Modifiers})
	case platform.EventText:
		w.tree.DispatchText(ev.Text)
	case platform.EventComposition:
		w.tree.DispatchComposition(ev.Composition)
	case platform.EventFocus:
		w.tree.SetWindowFocused(true)
		w.setFocused(true)
	case platform.EventBlur:
		w.tree.SetWindowFocused(false)
		w.setFocused(false)
	case platform.EventAccessibilityAction:
		w.tree.PerformSemanticsAction(ev.Node, ev.Action, core.SemanticsArg{
			Text: ev.Text, Number: ev.Number, SelectionStart: ev.SelectionStart, SelectionEnd: ev.SelectionEnd,
		})
		// The reader hears about the result after the next frame, even if the
		// action changed nothing that paints, such as the focus of a field.
		w.tree.MarkNeedsPaint()
	}
}

// blockedByModal reports whether a window with a modal window open ignores an
// event of kind: what the user does to it, and a request to close it.
func blockedByModal(kind platform.EventKind) bool {
	switch kind {
	case platform.EventPointerMove, platform.EventPointerDown, platform.EventPointerUp, platform.EventScroll, platform.EventPinch,
		platform.EventKeyDown, platform.EventKeyUp, platform.EventText, platform.EventComposition,
		platform.EventAccessibilityAction, platform.EventClose:
		return true
	}
	return false
}

// pointer converts a platform event to logical pixels.
func (w *window) pointer(kind core.PointerKind, ev *platform.Event) core.PointerEvent {
	return core.PointerEvent{
		Kind:      kind,
		Pos:       geom.Offset{X: ev.X / w.scale, Y: ev.Y / w.scale},
		Button:    ev.Button,
		Modifiers: ev.Modifiers,
		Time:      ev.Time,
		Device:    ev.Device,
		Pressure:  ev.Pressure,
		ID:        ev.PointerID,
	}
}

// drop answers the window system about a drag over the window.
func (w *window) drop(e *platform.DropEvent) input.DropEffect {
	if w.closing || len(w.modals) > 0 || w.scale <= 0 {
		return input.DropNone
	}
	phase := core.DragOver
	switch e.Phase {
	case platform.DropLeave:
		phase = core.DragLeave
	case platform.DropDrop:
		phase = core.DragDrop
	}
	effect := w.tree.DispatchDrag(phase, core.DragEvent{
		Pos: geom.Offset{X: e.X / w.scale, Y: e.Y / w.scale}, Kinds: e.Kinds, Data: e.Data,
		Allowed: e.Allowed, Modifiers: e.Modifiers,
	})
	if phase == core.DragDrop {
		// The drop changed what the window shows, and no event follows it.
		w.tree.MarkNeedsPaint()
	}
	return effect
}
