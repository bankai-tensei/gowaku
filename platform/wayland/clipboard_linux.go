package wayland

import (
	"errors"
	"time"

	"golang.org/x/sys/unix"

	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/internal/clip"
)

// selectionTimeout bounds the wait for another program to hand over the
// clipboard. A program that hangs must not hang this one for long.
const selectionTimeout = 300 * time.Millisecond

// offer is a wl_data_offer and the types it has.
type offer struct {
	wlDataOfferNop
	proxy wlDataOffer
	types []string
	// actions are what the source of a drag allows (source_actions).
	actions uint32
}

// wlDataOfferOffer implements wlDataOfferListener.
func (o *offer) wlDataOfferOffer(mimeType string) { o.types = append(o.types, mimeType) }

// clipboard is the selection of a seat.
type clipboard struct {
	wlDataSourceNop

	p      *Platform
	device wlDataDevice
	offers map[wlDataOffer]*offer
	// selection is what another client put on the clipboard.
	selection *offer
	// source is what this one put there, with its data in each MIME type
	// (docs/ADR/0049-clipboard.md).
	source  wlDataSource
	content platform.ClipboardContent
	encoded []clip.Offer
	// drop is a drag over a window of this client, and dragging the drag
	// that this client started (docs/ADR/0050-drag-and-drop.md).
	drop     dropState
	dragging *dragSource
}

func (cb *clipboard) init(p *Platform) {
	if cb.device != 0 || p.dataManager == 0 || p.seat.proxy == 0 {
		return
	}
	cb.p = p
	cb.offers = map[wlDataOffer]*offer{}
	cb.device = p.dataManager.getDataDevice(p.c, p.seat.proxy)
	p.c.listenWlDataDevice(cb.device, cb)
}

func (cb *clipboard) release() {
	if cb.p == nil {
		return
	}
	c := cb.p.c
	for _, o := range cb.offers {
		o.proxy.destroy(c)
	}
	if cb.source != 0 {
		cb.source.destroy(c)
	}
	if cb.device != 0 {
		if cb.p.dataManager != 0 && c.version(proxy(cb.device)) >= wlDataDeviceReleaseSince {
			cb.device.release(c)
		} else {
			c.destroy(proxy(cb.device))
		}
	}
	*cb = clipboard{}
}

// wlDataDeviceDataOffer implements wlDataDeviceListener. The offer is for the
// selection or for a drag, which the events after it say.
func (cb *clipboard) wlDataDeviceDataOffer(id wlDataOffer) {
	o := &offer{proxy: id}
	cb.offers[id] = o
	cb.p.c.listenWlDataOffer(id, o)
}

// wlDataDeviceSelection implements wlDataDeviceListener.
func (cb *clipboard) wlDataDeviceSelection(id wlDataOffer) {
	cb.selection = cb.offers[id]
	cb.destroyUnused()
}

// wlDataDeviceLeave implements wlDataDeviceListener. A drag left without a drop.
func (cb *clipboard) wlDataDeviceLeave() {
	cb.dropLeft()
	cb.destroyUnused()
}

// destroyUnused destroys the offers that nothing refers to any more.
func (cb *clipboard) destroyUnused() {
	for _, o := range cb.takeUnused() {
		o.proxy.destroy(cb.p.c)
	}
}

// takeUnused removes from cb.offers those that are neither the selection nor
// the offer of the drag over a window, and returns them. The clipboard may
// change in the middle of a drag, whose offer has to live until the drag
// leaves or drops.
func (cb *clipboard) takeUnused() []*offer {
	var unused []*offer
	for p, o := range cb.offers {
		if o != cb.selection && o != cb.drop.offer {
			unused = append(unused, o)
			delete(cb.offers, p)
		}
	}
	return unused
}

// read asks the owner of the clipboard for its data as MIME type t.
func (cb *clipboard) read(o *offer, t string) ([]byte, bool) {
	var pipe [2]int
	if err := unix.Pipe2(pipe[:], unix.O_CLOEXEC); err != nil {
		return nil, false
	}
	c := cb.p.c
	o.proxy.receive(c, t, int32(pipe[1]))
	c.hot(c.lib.flush, c.display)
	// libwayland sent a copy of the descriptor.
	_ = unix.Close(pipe[1])   //nolint:errcheck // nothing to do about it
	defer unix.Close(pipe[0]) //nolint:errcheck // nothing to do about it
	return readAll(pipe[0])
}

// get returns the text on the clipboard, or "".
func (cb *clipboard) get() string {
	return cb.readKinds(platform.ClipboardText).Text
}

// kinds reports what the clipboard holds.
func (cb *clipboard) kinds() platform.ClipboardKinds {
	if cb.source != 0 {
		return cb.content.Kinds()
	}
	if cb.selection == nil {
		return 0
	}
	return clip.Kinds(cb.selection.types)
}

// readKinds reads the kinds asked for.
func (cb *clipboard) readKinds(kinds platform.ClipboardKinds) platform.ClipboardContent {
	var c platform.ClipboardContent
	if cb.source != 0 {
		own := cb.content
		if kinds&platform.ClipboardText != 0 {
			c.Text = own.Text
		}
		if kinds&platform.ClipboardHTML != 0 {
			c.HTML = own.HTML
		}
		if kinds&platform.ClipboardImage != 0 {
			c.Image = own.Image
		}
		if kinds&platform.ClipboardFiles != 0 {
			c.Files = own.Files
		}
		return c
	}
	o := cb.selection
	if o == nil {
		return c
	}
	for _, k := range []struct {
		kind platform.ClipboardKinds
		want []string
	}{
		{platform.ClipboardText, clip.TextTypes}, {platform.ClipboardHTML, clip.HTMLTypes},
		{platform.ClipboardImage, clip.ImageTypes}, {platform.ClipboardFiles, clip.FileTypes},
	} {
		if kinds&k.kind == 0 {
			continue
		}
		if t := clip.Best(o.types, k.want); t != "" {
			if data, ok := cb.read(o, t); ok {
				clip.Decode(&c, t, data)
			}
		}
	}
	return c
}

// readAll reads fd until its end. Each piece gives the writer more time, so
// that a large image arrives, while a writer that stops does not hold this
// one up for long.
func readAll(fd int) ([]byte, bool) {
	var out []byte
	var buf [1 << 16]byte
	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	deadline := time.Now().Add(selectionTimeout)
	for {
		ms := int(time.Until(deadline) / time.Millisecond)
		if ms <= 0 {
			return nil, false
		}
		n, err := unix.Poll(fds, ms)
		if err != nil && !errors.Is(err, unix.EINTR) {
			return nil, false
		}
		if n == 0 {
			continue
		}
		r, err := unix.Read(fd, buf[:])
		if r > 0 {
			out = append(out, buf[:r]...)
			deadline = time.Now().Add(selectionTimeout)
		}
		if r == 0 || (err != nil && !errors.Is(err, unix.EINTR) && !errors.Is(err, unix.EAGAIN)) {
			return out, true
		}
	}
}

// set puts c on the clipboard. Without a data device it stays inside this
// application.
func (cb *clipboard) set(c platform.ClipboardContent) {
	cb.content, cb.encoded = c, clip.Encode(&c)
	if cb.device == 0 {
		return
	}
	wc := cb.p.c
	if cb.source != 0 {
		cb.source.destroy(wc)
	}
	cb.source = cb.p.dataManager.createDataSource(wc)
	wc.listenWlDataSource(cb.source, cb)
	for _, o := range cb.encoded {
		cb.source.offer(wc, o.Type)
	}
	cb.device.setSelection(wc, cb.source, cb.p.seat.serial)
}

// wlDataSourceSend implements wlDataSourceListener: another client reads what
// this one put on the clipboard.
func (cb *clipboard) wlDataSourceSend(mimeType string, fd int32) {
	var data []byte
	for _, o := range cb.encoded {
		if o.Type == mimeType {
			data = o.Data
			break
		}
	}
	// A slow reader must not hold up the UI goroutine.
	go func() {
		defer unix.Close(int(fd)) //nolint:errcheck // nothing to do about it
		b := data
		for len(b) > 0 {
			n, err := unix.Write(int(fd), b)
			if err != nil && !errors.Is(err, unix.EINTR) {
				return
			}
			b = b[max(n, 0):]
		}
	}()
}

// wlDataSourceCancelled implements wlDataSourceListener: another client took
// the clipboard.
func (cb *clipboard) wlDataSourceCancelled() {
	if cb.source != 0 {
		cb.source.destroy(cb.p.c)
	}
	cb.source, cb.content, cb.encoded = 0, platform.ClipboardContent{}, nil
}

// ClipboardText implements platform.Window.
func (w *Window) ClipboardText() string { return w.p.seat.clipboard.get() }

// SetClipboardText implements platform.Window.
func (w *Window) SetClipboardText(s string) {
	w.p.seat.clipboard.set(platform.ClipboardContent{Text: s})
}

// ClipboardKinds implements platform.Clipboard.
func (w *Window) ClipboardKinds() platform.ClipboardKinds { return w.p.seat.clipboard.kinds() }

// ReadClipboard implements platform.Clipboard.
func (w *Window) ReadClipboard(kinds platform.ClipboardKinds) platform.ClipboardContent {
	return w.p.seat.clipboard.readKinds(kinds)
}

// WriteClipboard implements platform.Clipboard.
func (w *Window) WriteClipboard(c platform.ClipboardContent) { w.p.seat.clipboard.set(c) }
