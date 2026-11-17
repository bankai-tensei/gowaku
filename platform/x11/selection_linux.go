package x11

import (
	"encoding/binary"
	"time"
	"unsafe"

	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/platform/internal/clip"
)

// The clipboard is the CLIPBOARD selection, which the helper window owns and
// asks for. Its data goes in the MIME types of the clip package, as GTK and Qt
// put it. Values above incrThreshold go by INCR in both directions (ICCCM
// 2.7.2). See docs/ADR/0049-clipboard.md.

const (
	// selectionTimeout bounds the wait for another program to hand over the
	// clipboard, or one more piece of it. A program that hangs must not hang
	// this one for long.
	selectionTimeout = 300 * time.Millisecond
	// incrThreshold is the largest value sent in one property, well below the
	// maximum request of every server, and incrChunk the size of the pieces of
	// a larger one.
	incrThreshold = 1 << 18
	incrChunk     = 1 << 16
)

// The states of a PropertyNotify.
const (
	PropertyNewValue = 0
	PropertyDelete   = 1
)

// selection is the state of one read of the clipboard, or of the data of a
// drop (XdndSelection).
type selection struct {
	// which is the selection asked for.
	which Atom
	done  bool
	// incr means that the owner sends the value in pieces, which data
	// gathers.
	incr bool
	data []byte
	kind Atom
}

// transfer is a value that another program reads by INCR.
type transfer struct {
	requestor XID
	property  Atom
	target    Atom
	data      []byte
	// sent counts the bytes sent; the empty piece that ends the transfer goes
	// when it reaches len(data).
	sent int
}

// atom returns the atom of name, interned on first use.
func (p *Platform) atom(name string) Atom {
	if a, ok := p.atomCache[name]; ok {
		return a
	}
	if p.atomCache == nil {
		p.atomCache = make(map[string]Atom)
	}
	a := Atom(call(p.x.XInternAtom, uintptr(p.display), ptr(cstring(name)), 0))
	p.atomCache[name] = a
	return a
}

// convert asks the owner of the clipboard for its value as target, and waits
// for it. It reports false if there is none, or the owner does not answer.
func (p *Platform) convert(target Atom) ([]byte, Atom, bool) {
	return p.convertSelection(p.atoms.CLIPBOARD, target, CurrentTime)
}

// convertSelection asks the owner of a selection for its value as target.
func (p *Platform) convertSelection(which, target Atom, stamp uintptr) ([]byte, Atom, bool) {
	a := &p.atoms
	p.sel = selection{which: which}
	call(p.x.XConvertSelection, uintptr(p.display), uintptr(which), uintptr(target),
		uintptr(a.GOWAKU_SELECTION), uintptr(p.helper), stamp)
	call(p.x.XFlush, uintptr(p.display))
	// The answer comes as events. Others that arrive meanwhile go to the queue
	// as usual. Each piece of an INCR transfer gives the owner more time.
	deadline := time.Now().Add(selectionTimeout)
	pieces := 0
	for !p.sel.done {
		if p.dispatchPending() {
			if n := len(p.sel.data); p.sel.incr && n != pieces {
				pieces, deadline = n, time.Now().Add(selectionTimeout)
			}
			continue
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			p.sel = selection{}
			return nil, 0, false
		}
		p.pollOnce(int(remaining/time.Millisecond) + 1)
	}
	s := p.sel
	p.sel = selection{}
	return s.data, s.kind, s.kind != 0
}

// takeSelection receives the answer to XConvertSelection.
func (p *Platform) takeSelection(e *XSelectionEvent) {
	if e.Window != p.helper || e.Selection != p.sel.which {
		return
	}
	if e.Property == 0 {
		p.sel.done = true
		return
	}
	value, kind := p.readProperty(p.helper, e.Property, true)
	if kind == p.atoms.INCR {
		// Deleting the property, which readProperty did, asks for the first
		// piece.
		p.sel.incr, p.sel.data = true, p.sel.data[:0]
		return
	}
	p.sel.done, p.sel.data, p.sel.kind = true, value, kind
}

// takePiece receives a piece of an INCR transfer to the helper.
func (p *Platform) takePiece(e *XPropertyEvent) {
	if !p.sel.incr || p.sel.done || e.State != PropertyNewValue || e.Atom != p.atoms.GOWAKU_SELECTION {
		return
	}
	value, kind := p.readProperty(p.helper, e.Atom, true)
	if len(value) == 0 {
		p.sel.done, p.sel.kind = true, kind
		if p.sel.kind == 0 {
			p.sel.kind = p.atoms.INCR
		}
		return
	}
	p.sel.data = append(p.sel.data, value...)
}

// targets returns the MIME types that the owner offers.
func (p *Platform) targets() []string {
	value, _, ok := p.convert(p.atoms.TARGETS)
	if !ok {
		return nil
	}
	size := int(unsafe.Sizeof(uintptr(0)))
	var out []string
	for i := 0; i+size <= len(value); i += size {
		a := Atom(binary.NativeEndian.Uint64(value[i:]))
		if name := p.atomName(a); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// atomName returns the name of one of the atoms gowaku knows, or "".
func (p *Platform) atomName(a Atom) string {
	for _, list := range [][]string{clip.TextTypes, clip.HTMLTypes, clip.ImageTypes, clip.FileTypes} {
		for _, name := range list {
			if p.atom(name) == a {
				return name
			}
		}
	}
	if a == XA_STRING {
		return "STRING"
	}
	return ""
}

// ClipboardText implements platform.Window. It returns "" if the clipboard holds
// no text or its owner does not answer in time.
func (w *Window) ClipboardText() string {
	p := w.platform
	if p.clipboardOwned {
		return p.clipboard.Text
	}
	value, _, _ := p.convert(p.atoms.UTF8_STRING)
	return string(value)
}

// SetClipboardText implements platform.Window.
func (w *Window) SetClipboardText(s string) { w.WriteClipboard(platform.ClipboardContent{Text: s}) }

// ClipboardKinds implements platform.Clipboard.
func (w *Window) ClipboardKinds() platform.ClipboardKinds {
	p := w.platform
	if p.clipboardOwned {
		return p.clipboard.Kinds()
	}
	return clip.Kinds(p.targets())
}

// ReadClipboard implements platform.Clipboard.
func (w *Window) ReadClipboard(kinds platform.ClipboardKinds) platform.ClipboardContent {
	p := w.platform
	if p.clipboardOwned {
		return keep(p.clipboard, kinds)
	}
	types := p.targets()
	var c platform.ClipboardContent
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
		t := clip.Best(types, k.want)
		if t == "" {
			continue
		}
		if value, _, ok := p.convert(p.atom(t)); ok {
			clip.Decode(&c, t, value)
		}
	}
	return c
}

// keep returns the kinds of c asked for.
func keep(c platform.ClipboardContent, kinds platform.ClipboardKinds) platform.ClipboardContent {
	var out platform.ClipboardContent
	if kinds&platform.ClipboardText != 0 {
		out.Text = c.Text
	}
	if kinds&platform.ClipboardHTML != 0 {
		out.HTML = c.HTML
	}
	if kinds&platform.ClipboardImage != 0 {
		out.Image = c.Image
	}
	if kinds&platform.ClipboardFiles != 0 {
		out.Files = c.Files
	}
	return out
}

// WriteClipboard implements platform.Clipboard.
func (w *Window) WriteClipboard(c platform.ClipboardContent) {
	p := w.platform
	p.clipboard, p.clipboardOwned = c, true
	p.offers = clip.Encode(&c)
	call(p.x.XSetSelectionOwner, uintptr(p.display), uintptr(p.atoms.CLIPBOARD), uintptr(p.helper), CurrentTime)
	call(p.x.XFlush, uintptr(p.display))
}

// offerIn returns the data of target among offers. target may be one of the
// old names of text.
func offerIn(p *Platform, offers []clip.Offer, target Atom) ([]byte, bool) {
	a := &p.atoms
	if target == a.TEXT || target == XA_STRING {
		target = a.UTF8_STRING
	}
	for _, o := range offers {
		if p.atom(o.Type) == target {
			return o.Data, true
		}
	}
	return nil, false
}

// answerSelection serves a request of another program for the clipboard.
func (p *Platform) answerSelection(r *XSelectionRequestEvent) {
	a := &p.atoms
	property := r.Property
	if property == 0 {
		// Obsolete clients leave this to the owner.
		property = r.Target
	}
	display := uintptr(p.display)
	offers, owned, text := p.offers, p.clipboardOwned, p.clipboard.Text != ""
	if r.Selection == a.XdndSelection {
		offers, owned, text = p.dragOffers, p.dragOffers != nil, false
	}
	data, ok := offerIn(p, offers, r.Target)
	switch {
	case !owned || (r.Selection != a.CLIPBOARD && r.Selection != a.XdndSelection):
		property = 0
	case r.Target == a.TARGETS:
		list := []Atom{a.TARGETS}
		for _, o := range offers {
			list = append(list, p.atom(o.Type))
		}
		if text {
			list = append(list, XA_STRING, a.TEXT)
		}
		b := atomBytes(list...)
		call(p.x.XChangeProperty, display, uintptr(r.Requestor), uintptr(property), XA_ATOM, 32,
			PropModeReplace, ptr(&b[0]), uintptr(len(list)))
	case !ok:
		property = 0
	case len(data) > incrThreshold:
		p.startTransfer(r.Requestor, property, r.Target, data)
	default:
		var at uintptr
		if len(data) > 0 {
			at = uintptr(unsafe.Pointer(&data[0]))
		}
		// data is only read during the call.
		call(p.x.XChangeProperty, display, uintptr(r.Requestor), uintptr(property), uintptr(r.Target), 8,
			PropModeReplace, at, uintptr(len(data)))
	}
	reply := XSelectionEvent{Selection: r.Selection, Target: r.Target, Property: property, Time: r.Time}
	reply.Type, reply.Display, reply.Window = SelectionNotify, p.display, r.Requestor
	call(p.x.XSendEvent, display, uintptr(r.Requestor), 0, 0, ptr(&reply))
	call(p.x.XFlush, display)
}

// startTransfer answers with INCR and the size, and sends the pieces as the
// requestor deletes the property.
func (p *Platform) startTransfer(requestor XID, property, target Atom, data []byte) {
	call(p.x.XSelectInput, uintptr(p.display), uintptr(requestor), PropertyChangeMask)
	size := []uintptr{uintptr(len(data))}
	call(p.x.XChangeProperty, uintptr(p.display), uintptr(requestor), uintptr(property), uintptr(p.atoms.INCR), 32,
		PropModeReplace, ptr(&size[0]), 1)
	p.transfers = append(p.transfers, &transfer{requestor: requestor, property: property, target: target, data: data})
}

// sendPiece sends the next piece of a transfer whose requestor deleted the
// property. It reports whether the event belonged to a transfer.
func (p *Platform) sendPiece(e *XPropertyEvent) bool {
	if e.State != PropertyDelete {
		return false
	}
	for i, t := range p.transfers {
		if t.requestor != e.Window || t.property != e.Atom {
			continue
		}
		piece := t.data[t.sent:min(t.sent+incrChunk, len(t.data))]
		var at uintptr
		if len(piece) > 0 {
			at = uintptr(unsafe.Pointer(&piece[0]))
		}
		// piece is only read during the call.
		call(p.x.XChangeProperty, uintptr(p.display), uintptr(t.requestor), uintptr(t.property), uintptr(t.target), 8,
			PropModeReplace, at, uintptr(len(piece)))
		call(p.x.XFlush, uintptr(p.display))
		if len(piece) == 0 {
			// The empty piece ended it.
			p.transfers = append(p.transfers[:i], p.transfers[i+1:]...)
			call(p.x.XSelectInput, uintptr(p.display), uintptr(t.requestor), 0)
		}
		t.sent += len(piece)
		return true
	}
	return false
}
