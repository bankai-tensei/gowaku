package wayland

import (
	"errors"

	"golang.org/x/sys/unix"

	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/internal/clip"
)

// Drag and drop goes through the wl_data_device of the seat, as the clipboard
// does: the compositor introduces an offer when a drag enters a surface, and a
// drag out of a window is a wl_data_source given to start_drag. Wayland knows
// copy and move; a link is refused. See docs/ADR/0050-drag-and-drop.md.

// dndActionsSince is the version of wl_data_device_manager that has actions.
const dndActionsSince = 3

// dropState is a drag over a window of this client.
type dropState struct {
	offer  *offer
	window *Window
	serial uint32
	x, y   float64
	kinds  input.DataKinds
	effect input.DropEffect
}

// dragSource is a drag that this client started.
type dragSource struct {
	wlDataSourceNop
	cb     *clipboard
	proxy  wlDataSource
	offers []clip.Offer
	data   input.DragData
	done   func(input.DropEffect)
	action uint32
}

// wlDataOfferSourceActions implements wlDataOfferListener.
func (o *offer) wlDataOfferSourceActions(actions uint32) { o.actions = actions }

// actionsOf turns drop effects into the actions of Wayland.
func actionsOf(e input.DropEffect) uint32 {
	return uint32(e & (input.DropCopy | input.DropMove))
}

// allowedOf is what the source of o allows. A source older than actions
// allows a copy.
func (cb *clipboard) allowedOf(o *offer) input.DropEffect {
	if cb.p.c.version(proxy(cb.device)) < dndActionsSince || o.actions == 0 {
		return input.DropCopy
	}
	return input.DropEffect(o.actions) & (input.DropCopy | input.DropMove)
}

// wlDataDeviceEnter implements wlDataDeviceListener.
func (cb *clipboard) wlDataDeviceEnter(serial uint32, surface wlSurface, x, y float64, id wlDataOffer) {
	o := cb.offers[id]
	w := cb.p.windows[surface]
	if o == nil || w == nil {
		return
	}
	cb.drop = dropState{offer: o, window: w, serial: serial, x: x, y: y}
	k := clip.Kinds(o.types)
	if k&platform.ClipboardText != 0 {
		cb.drop.kinds |= input.DataText
	}
	if k&platform.ClipboardFiles != 0 {
		cb.drop.kinds |= input.DataFiles
	}
	cb.dropMoved()
}

// wlDataDeviceMotion implements wlDataDeviceListener.
func (cb *clipboard) wlDataDeviceMotion(_ uint32, x, y float64) {
	if cb.drop.offer == nil {
		return
	}
	cb.drop.x, cb.drop.y = x, y
	cb.dropMoved()
}

// dropMoved asks the window about the drag at its new place, and tells the
// compositor what a drop would do.
func (cb *clipboard) dropMoved() {
	d := &cb.drop
	effect := cb.ask(d.window, platform.DropEvent{
		Phase: platform.DropOver, X: float32(d.x * d.window.scale), Y: float32(d.y * d.window.scale),
		Kinds: d.kinds, Allowed: cb.allowedOf(d.offer),
	})
	if effect == d.effect && effect != input.DropNone {
		return
	}
	d.effect = effect
	c := cb.p.c
	if effect == input.DropNone {
		// A NULL type refuses the drop.
		c.args[0], c.args[1] = uint64(d.serial), 0
		c.marshal(proxy(d.offer.proxy), 0, nil, 0, 0)
	} else {
		t := clip.Best(d.offer.types, clip.FileTypes)
		if t == "" {
			t = clip.Best(d.offer.types, clip.TextTypes)
		}
		d.offer.proxy.accept(c, d.serial, t)
	}
	if c.version(proxy(cb.device)) >= dndActionsSince {
		d.offer.proxy.setActions(c, actionsOf(effect), actionsOf(effect))
	}
}

// wlDataDeviceDrop implements wlDataDeviceListener.
func (cb *clipboard) wlDataDeviceDrop() {
	d := cb.drop
	if d.offer == nil {
		return
	}
	cb.drop = dropState{}
	var data input.DragData
	if cb.dragging != nil {
		// The drag is this client's own, which cannot be read through a pipe
		// while the UI goroutine waits on it.
		data = cb.dragging.data
	} else if d.effect != input.DropNone {
		var c platform.ClipboardContent
		for _, want := range [][]string{clip.TextTypes, clip.FileTypes} {
			if t := clip.Best(d.offer.types, want); t != "" {
				if b, ok := cb.read(d.offer, t); ok {
					clip.Decode(&c, t, b)
				}
			}
		}
		data = input.DragData{Text: c.Text, Files: c.Files}
	}
	effect := input.DropNone
	if d.effect != input.DropNone {
		effect = cb.ask(d.window, platform.DropEvent{
			Phase: platform.DropDrop, X: float32(d.x * d.window.scale), Y: float32(d.y * d.window.scale),
			Kinds: d.kinds, Data: data, Allowed: cb.allowedOf(d.offer),
		})
	}
	wc := cb.p.c
	if effect != input.DropNone && wc.version(proxy(cb.device)) >= dndActionsSince {
		d.offer.proxy.finish(wc)
	}
	d.offer.proxy.destroy(wc)
	delete(cb.offers, d.offer.proxy)
}

// dropLeft ends the drag over the window, which left or was canceled.
func (cb *clipboard) dropLeft() {
	if cb.drop.offer != nil {
		cb.ask(cb.drop.window, platform.DropEvent{Phase: platform.DropLeave, Kinds: cb.drop.kinds})
		cb.drop = dropState{}
	}
}

// ask hands a drag to the window, and queues a frame: the handler may have
// changed what shows.
func (cb *clipboard) ask(w *Window, e platform.DropEvent) input.DropEffect {
	if w == nil || w.dropHandler == nil {
		return input.DropNone
	}
	effect := w.dropHandler(&e) & e.Allowed
	cb.p.queue.Push(platform.Event{Kind: platform.EventRedraw, Window: w.id})
	return effect
}

// SetDropHandler implements platform.Dragging.
func (w *Window) SetDropHandler(fn func(e *platform.DropEvent) input.DropEffect) { w.dropHandler = fn }

// StartDrag implements platform.Dragging, from the latest press of the seat.
func (w *Window) StartDrag(d input.DragData, allowed input.DropEffect, done func(input.DropEffect)) {
	cb := &w.p.seat.clipboard
	if cb.device == 0 || cb.dragging != nil || d.Kinds() == 0 {
		done(input.DropNone)
		return
	}
	c := w.p.c
	s := &dragSource{cb: cb, data: d, done: done}
	s.offers = clip.Encode(&platform.ClipboardContent{Text: d.Text, Files: d.Files})
	s.proxy = w.p.dataManager.createDataSource(c)
	c.listenWlDataSource(s.proxy, s)
	for _, o := range s.offers {
		s.proxy.offer(c, o.Type)
	}
	if c.version(proxy(cb.device)) >= dndActionsSince {
		actions := actionsOf(allowed)
		if actions == 0 {
			actions = actionsOf(input.DropCopy)
		}
		s.proxy.setActions(c, actions)
	}
	cb.device.startDrag(c, s.proxy, w.surface, 0, w.p.seat.serial)
	cb.dragging = s
}

// end finishes the drag with effect.
func (s *dragSource) end(effect input.DropEffect) {
	cb := s.cb
	if cb.dragging != s {
		return
	}
	cb.dragging = nil
	s.proxy.destroy(cb.p.c)
	s.done(effect)
}

// wlDataSourceSend implements wlDataSourceListener: the target reads the data.
func (s *dragSource) wlDataSourceSend(mimeType string, fd int32) {
	var data []byte
	for _, o := range s.offers {
		if o.Type == mimeType {
			data = o.Data
			break
		}
	}
	// A slow reader must not hold up the UI goroutine.
	go func() {
		defer unix.Close(int(fd)) //nolint:errcheck // nothing to do about it
		for len(data) > 0 {
			n, err := unix.Write(int(fd), data)
			if err != nil && !errors.Is(err, unix.EINTR) {
				return
			}
			data = data[max(n, 0):]
		}
	}()
}

// wlDataSourceAction implements wlDataSourceListener.
func (s *dragSource) wlDataSourceAction(action uint32) { s.action = action }

// wlDataSourceDndFinished implements wlDataSourceListener: the target took the
// data.
func (s *dragSource) wlDataSourceDndFinished() {
	effect := input.DropEffect(s.action) & (input.DropCopy | input.DropMove)
	if effect == input.DropNone {
		effect = input.DropCopy
	}
	s.end(effect)
}

// wlDataSourceCancelled implements wlDataSourceListener: the drop was refused,
// or the drag canceled.
func (s *dragSource) wlDataSourceCancelled() { s.end(input.DropNone) }

// wlDataSourceDndDropPerformed implements wlDataSourceListener. An old
// compositor sends no dnd_finished, so the drop counts as done here for it.
func (s *dragSource) wlDataSourceDndDropPerformed() {
	if s.cb.p.c.version(proxy(s.cb.device)) < dndActionsSince {
		s.end(input.DropCopy)
	}
}
