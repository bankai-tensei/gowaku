package wayland

import (
	"github.com/bankai-tensei/gowaku/platform"
)

// textInput is the zwp_text_input_v3 of the seat, through which an input
// method shows its composition in the window and commits text.
type textInput struct {
	zwpTextInputV3Nop

	p     *Platform
	proxy zwpTextInputV3
	// focus is the window the text input entered, and enabled whether it is
	// enabled there, as the compositor knows it.
	focus   *Window
	enabled bool
	// composing says whether the focused window shows a composition.
	composing bool

	// What arrived since the last done.
	preedit    string
	begin, end int32
	commit     string
}

func (ti *textInput) init(p *Platform) {
	ti.p = p
	if p.textInputs == 0 || p.seat.proxy == 0 {
		return
	}
	ti.proxy = p.textInputs.getTextInput(p.c, p.seat.proxy)
	p.c.listenZwpTextInputV3(ti.proxy, ti)
}

func (ti *textInput) release() {
	if ti.proxy != 0 {
		ti.proxy.destroy(ti.p.c)
	}
	*ti = textInput{}
}

func (ti *textInput) forget(w *Window) {
	if ti.focus == w {
		ti.focus, ti.enabled, ti.composing = nil, false, false
	}
}

// sync enables the text input where the focused window wants conversion, and
// disables it elsewhere.
func (ti *textInput) sync() {
	if ti.proxy == 0 {
		return
	}
	want := ti.focus != nil && ti.focus.imeEnabled && !ti.focus.imeSecure
	if want == ti.enabled {
		return
	}
	c := ti.p.c
	ti.enabled = want
	if want {
		ti.proxy.enable(c)
		ti.proxy.setContentType(c, zwpTextInputV3ContentHintNone, zwpTextInputV3ContentPurposeNormal)
		ti.sendRect()
	} else {
		ti.proxy.disable(c)
		ti.endComposition()
	}
	ti.proxy.commit(c)
}

// sendRect tells the input method where the caret of the focused window is,
// so that it can place its candidates. The caller commits.
func (ti *textInput) sendRect() {
	r := ti.focus.imeRect
	ti.proxy.setCursorRectangle(ti.p.c, r[0], r[1], max(r[2], 1), max(r[3], 1))
}

// rectChanged sends a new caret rectangle of w.
func (ti *textInput) rectChanged(w *Window) {
	if ti.proxy != 0 && ti.focus == w && ti.enabled {
		ti.sendRect()
		ti.proxy.commit(ti.p.c)
	}
}

// cancel drops the composition of w. The protocol has no request for it:
// disabling resets the input method, and enabling again starts afresh.
func (ti *textInput) cancel(w *Window) {
	if ti.proxy == 0 || ti.focus != w || !ti.enabled {
		return
	}
	c := ti.p.c
	ti.proxy.disable(c)
	ti.proxy.commit(c)
	ti.enabled = false
	ti.endComposition()
	ti.sync()
}

func (ti *textInput) endComposition() {
	if ti.composing && ti.focus != nil {
		ti.p.queue.Push(platform.Event{Kind: platform.EventComposition, Window: ti.focus.id})
	}
	ti.composing = false
}

// zwpTextInputV3Enter implements zwpTextInputV3Listener. It follows the
// keyboard focus.
func (ti *textInput) zwpTextInputV3Enter(surface wlSurface) {
	ti.focus, ti.enabled, ti.composing = ti.p.windows[surface], false, false
	ti.sync()
}

// zwpTextInputV3Leave implements zwpTextInputV3Listener.
func (ti *textInput) zwpTextInputV3Leave(wlSurface) {
	if ti.enabled {
		ti.proxy.disable(ti.p.c)
		ti.proxy.commit(ti.p.c)
	}
	ti.endComposition()
	ti.focus, ti.enabled = nil, false
}

// zwpTextInputV3PreeditString implements zwpTextInputV3Listener.
func (ti *textInput) zwpTextInputV3PreeditString(text string, cursorBegin, cursorEnd int32) {
	ti.preedit, ti.begin, ti.end = text, cursorBegin, cursorEnd
}

// zwpTextInputV3CommitString implements zwpTextInputV3Listener.
func (ti *textInput) zwpTextInputV3CommitString(text string) { ti.commit = text }

// zwpTextInputV3Done implements zwpTextInputV3Listener: what came before it
// applies now. Its serial counts the commits it answers; an old one still
// carries the latest state of the input method, so it applies as well.
func (ti *textInput) zwpTextInputV3Done(uint32) {
	u := textInputDone(ti.composing, ti.preedit, ti.begin, ti.end, ti.commit)
	ti.preedit, ti.commit, ti.begin, ti.end = "", "", 0, 0
	w := ti.focus
	if w == nil {
		ti.composing = false
		return
	}
	q := &ti.p.queue
	if u.endComposition {
		q.Push(platform.Event{Kind: platform.EventComposition, Window: w.id})
	}
	if u.commit != "" {
		q.Push(platform.Event{Kind: platform.EventText, Window: w.id, Text: u.commit})
	}
	if u.composition != nil {
		q.Push(platform.Event{Kind: platform.EventComposition, Window: w.id, Composition: u.composition})
	}
	ti.composing = u.composition != nil
}
