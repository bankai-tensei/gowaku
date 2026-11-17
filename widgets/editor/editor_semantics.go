package editor

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

const (
	// editorA11yWhole is the most lines a Editor hands a screen reader
	// whole. A longer document shows it a window of lines.
	editorA11yWhole = 2000
	// editorA11yMargin is how many lines the window reaches past the view
	// and the caret, and editorA11yWindow the most lines it holds.
	editorA11yMargin = 100
	editorA11yWindow = 1000
)

// editorSemantics is the window of text the editor last gave assistive
// technology, kept until the text or the window changes, so that a screen
// reader listening to every frame does not join and measure the lines again.
type editorSemantics struct {
	valid       bool
	gen         uint64
	first, last int
	text        string
	lineStarts  []int
}

// window returns the lines that assistive technology sees: all of a modest
// document, and otherwise the lines around the view and the caret.
func (r *renderEditor) window() (first, last int) {
	n := r.ctl.buf.lineCount()
	if n <= editorA11yWhole {
		return 0, n - 1
	}
	caret := r.sel().caret.Line
	first = max(min(r.first, caret)-editorA11yMargin, 0)
	last = min(max(r.last, caret)+editorA11yMargin, n-1)
	if last-first+1 > editorA11yWindow {
		// The caret is far from the view: the reader follows the caret.
		first = max(caret-editorA11yMargin, 0)
		last = min(caret+editorA11yMargin, n-1)
	}
	return first, last
}

// semanticsText brings the window of text up to date.
func (r *renderEditor) semanticsText() *editorSemantics {
	first, last := r.window()
	a := &r.a11y
	if a.valid && a.gen == r.ctl.buf.gen && a.first == first && a.last == last {
		return a
	}
	b := &r.ctl.buf
	a.valid, a.gen, a.first, a.last = true, b.gen, first, last
	a.text = b.slice(TextPosition{Line: first}, TextPosition{Line: last, Column: len(b.line(last))})
	a.lineStarts = a.lineStarts[:0]
	u := 0
	for i := first; i < last; i++ {
		u += kit.Utf16Len(b.line(i)) + 1
		a.lineStarts = append(a.lineStarts, u)
	}
	return a
}

// windowOffset returns p as a UTF-16 offset into the window, kept within it.
func (r *renderEditor) windowOffset(a *editorSemantics, p TextPosition) int {
	switch {
	case p.Line < a.first:
		return 0
	case p.Line > a.last:
		return kit.Utf16Len(a.text)
	}
	start := 0
	if p.Line > a.first {
		start = a.lineStarts[p.Line-a.first-1]
	}
	return start + kit.Utf16Len(r.ctl.buf.line(p.Line)[:p.Column])
}

// Semantics implements core.RenderObject. The editor is a multiline text field.
// A document of up to 2,000 lines gives its whole text; a longer one gives the
// lines around the view and the caret, as editors for the web do, so that a
// screen reader does not get 100,000 lines on every frame.
func (r *renderEditor) Semantics(sb *core.SemanticsBuilder) {
	a := r.semanticsText()
	info := &semantics.TextInfo{
		Text:           a.text,
		SelectionStart: r.windowOffset(a, r.sel().anchor),
		SelectionEnd:   r.windowOffset(a, r.sel().caret),
		LineStarts:     a.lineStarts,
	}
	if r.rowsValid && r.rows.count() == r.ctl.buf.lineCount() {
		bounds := core.LocalRectToGlobal(r, geom.Rect{W: r.size.W, H: r.size.H})
		for i := a.first; i <= min(r.last, a.last); i++ {
			rc := geom.Rect{
				X: r.textLeft - r.hscroll(), Y: r.viewTop(i),
				W: r.size.W - r.textLeft, H: float32(r.rows.at(i)) * r.lineHeight,
			}
			if e := r.cache.get(i); e != nil && e.valid && r.wrap >= geom.Inf {
				rc.W = max(e.layout.Size().W, kit.CaretWidth)
			}
			info.LineRects = append(info.LineRects, core.LocalRectToGlobal(r, rc).Intersect(bounds))
		}
	}
	state := semantics.Multiline
	actions := semantics.ActionSetSelection
	if r.editor.ReadOnly {
		state |= semantics.ReadOnly
	} else {
		actions |= semantics.ActionSetValue
	}
	sb.Open(semantics.Node{
		Role:    semantics.RoleTextField,
		Label:   r.editor.Label,
		Value:   a.text,
		State:   state,
		Actions: actions,
		Text:    info,
	}, r.Size())
	sb.Close()
}

// PerformSemanticsAction implements core.SemanticsActor. A screen reader can
// replace the text it sees, as typing would, and select a part of it.
func (r *renderEditor) PerformSemanticsAction(act semantics.Actions, arg core.SemanticsArg) bool {
	r.activate()
	a := r.semanticsText()
	b := &r.ctl.buf
	start := TextPosition{Line: a.first}
	switch act {
	case semantics.ActionSetValue:
		if r.editor.ReadOnly {
			return false
		}
		r.ctl.replace(start, TextPosition{Line: a.last, Column: len(b.line(a.last))}, arg.Text, kit.EditOther)
		return true
	case semantics.ActionSetSelection:
		base := b.lineStart(a.first)
		anchor := b.position(base + kit.ByteOffset(a.text, arg.SelectionStart))
		caret := b.position(base + kit.ByteOffset(a.text, arg.SelectionEnd))
		r.ctl.choose(anchor, caret)
		return true
	}
	return false
}
