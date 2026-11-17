package editor

import (
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/text"
)

// MouseCursor implements core.CursorSource.
func (r *renderEditor) MouseCursor() input.Cursor { return input.CursorIBeam }

// CanFocus implements core.FocusTarget.
func (r *renderEditor) CanFocus() bool { return true }

// FocusChanged implements core.FocusTarget.
func (r *renderEditor) FocusChanged(focused bool) {
	if r.focused == focused {
		return
	}
	if focused {
		r.activate()
	} else {
		// The focus can move during a layout, as when the find bar takes
		// it, where OnSelectionChange must not run.
		r.ctl.quiet++
		r.commitComposition()
		r.ctl.quiet--
	}
	r.focused = focused
	r.imeRect = geom.Rect{}
	r.Host().SetIMEEnabled(focused && !r.editor.ReadOnly)
	r.showCaret()
	r.MarkNeedsPaint()
}

// HandleText implements core.FocusTarget.
func (r *renderEditor) HandleText(s string) {
	r.activate()
	if r.state != nil && r.state.typeAhead(s) {
		return
	}
	if r.editor.ReadOnly {
		return
	}
	r.endComposition()
	kind := kit.EditOther
	if utf8.RuneCountInString(s) == 1 {
		kind = kit.EditTyping
	}
	r.insert(s, kind)
}

// HandleComposition implements core.FocusTarget. The composition takes the
// place of the selection, which a composition first cuts down to one line.
func (r *renderEditor) HandleComposition(c *input.Composition) {
	if r.editor.ReadOnly || c == nil && r.composition == nil {
		return
	}
	if c == nil {
		r.endComposition()
		return
	}
	r.activate()
	ctl := r.ctl
	sel := ctl.Selection().Ordered()
	if r.composition == nil && sel.Start.Line != sel.End.Line {
		ctl.replace(sel.Start, sel.End, "", kit.EditOther)
		sel = ctl.Selection().Ordered()
	}
	line := ctl.buf.line(sel.Start.Line)
	r.composition = c
	r.compLine, r.compStart, r.compEnd = sel.Start.Line, sel.Start.Column, sel.End.Column
	r.compText = line[:r.compStart] + c.Text + line[r.compEnd:]
	r.reveal = true
	r.cache.invalidate(r.compLine, r.compLine)
	r.MarkNeedsLayout()
	r.showCaret()
}

// endComposition forgets the composition, whose text the IME either commits
// with HandleText or takes back.
func (r *renderEditor) endComposition() {
	if r.composition == nil {
		return
	}
	r.composition = nil
	r.cache.invalidate(r.compLine, r.compLine)
	r.MarkNeedsLayout()
}

// commitComposition keeps what the IME was converting, as Input does.
func (r *renderEditor) commitComposition() {
	if r.composition == nil {
		return
	}
	s := r.composition.Text
	r.activate()
	r.endComposition()
	r.Host().CancelComposition()
	r.insert(s, kit.EditOther)
}

// insert replaces the selection with s.
func (r *renderEditor) insert(s string, kind kit.EditKind) {
	sel := r.ctl.Selection().Ordered()
	r.ctl.replace(sel.Start, sel.End, s, kind)
}

// HandlePointer implements core.PointerTarget.
func (r *renderEditor) HandlePointer(e *core.PointerEvent) {
	if !r.rowsValid {
		return
	}
	if e.Kind == core.PointerScroll {
		r.wheel(e)
		return
	}
	if e.Button != input.ButtonPrimary {
		return
	}
	r.activate()
	switch e.Kind {
	case core.PointerDown:
		e.Handled = true
		r.commitComposition()
		r.RequestFocus()
		r.press(e)
	case core.PointerMove:
		if r.dragging {
			r.goalOK = false
			if e.Pos.Y < 0 || e.Pos.Y > r.size.H {
				// Past an edge, the view follows the drag.
				step := float32(0.25)
				if e.Pos.Y < 0 {
					step = -step
				}
				r.state.Wheel(step)
			}
			r.ctl.choose(r.sel().anchor, r.positionAt(e.Pos))
		}
	case core.PointerUp, core.PointerCancel:
		r.dragging = false
	}
}

// wheel scrolls down the lines, or sideways when the lines do not wrap: with
// a sideways wheel or touchpad, or with Shift, as in browsers.
func (r *renderEditor) wheel(e *core.PointerEvent) {
	dx, dy := e.Scroll.X, e.Scroll.Y
	if e.Modifiers&input.ModShift != 0 && dx == 0 {
		dx, dy = -dy, 0
	}
	if dy != 0 {
		r.state.Wheel(-dy)
		e.Handled = true
	}
	if h := &r.state.horz; dx != 0 && r.wrap >= geom.Inf && h.Metrics.Content > h.Metrics.View {
		h.Wheel(dx)
		e.Handled = true
	}
}

func (r *renderEditor) press(e *core.PointerEvent) {
	r.goalOK = false
	p := r.positionAt(e.Pos)
	ctl := r.ctl
	switch {
	case e.Clicks == 2:
		a, z := r.lineLayout(p.Line).WordRange(p.Column)
		ctl.choose(TextPosition{p.Line, a}, TextPosition{p.Line, z})
	case e.Clicks >= 3:
		end := TextPosition{Line: p.Line + 1}
		if p.Line+1 >= ctl.buf.lineCount() {
			end = TextPosition{Line: p.Line, Column: len(ctl.buf.line(p.Line))}
		}
		ctl.choose(TextPosition{Line: p.Line}, end)
	default:
		anchor := p
		if e.Modifiers&input.ModShift != 0 {
			anchor = r.sel().anchor
		}
		ctl.choose(anchor, p)
		r.dragging = true
	}
}

// positionAt returns the position of the text closest to pos.
func (r *renderEditor) positionAt(pos geom.Offset) TextPosition {
	y := float64(pos.Y-r.padTop()) + r.top()
	if y < 0 {
		return TextPosition{}
	}
	line, first := r.rows.find(int(y / float64(r.lineHeight)))
	l := r.lineLayout(line)
	local := geom.Offset{X: pos.X - r.textLeft + r.hscroll(), Y: float32(y - r.lineTop(line))}
	_ = first
	return r.ctl.buf.clamp(TextPosition{Line: line, Column: l.HitTest(local)})
}

// HandleKey implements core.FocusTarget.
func (r *renderEditor) HandleKey(e *core.KeyEvent) bool {
	r.activate()
	if !r.rowsValid {
		r.lineLayout(0)
	}
	// The editor uses Enter and Shift+Enter; Enter with Ctrl or Alt is left
	// to the shortcuts above it, unless it commits a composition.
	if kit.ShortcutEnter(e, r.composition != nil) {
		return false
	}
	control := e.Modifiers&input.ModControl != 0
	shift := e.Modifiers&input.ModShift != 0
	switch {
	case control && r.shortcut(e.Key, shift):
		return true
	case r.findKey(e.Key, control, shift):
		return true
	case r.move(e.Key, control, shift):
		return true
	}
	if r.edit(e.Key, control, shift) {
		return true
	}
	// The character of a typing key arrives as text of its own; the press is
	// the editor's, so that a bare-key shortcut does not run as well. A
	// read-only editor takes no text, except the query typed into a find bar
	// that has just opened.
	takes := !r.editor.ReadOnly || r.state != nil && r.state.queryPending && r.state.finding
	return takes && kit.TypingKey(e, r.composition != nil)
}

// shortcut runs the Ctrl combinations of the clipboard, the history and the
// find bar.
func (r *renderEditor) shortcut(key input.KeyCode, shift bool) bool {
	ctl := r.ctl
	sel := ctl.Selection().Ordered()
	ro := r.editor.ReadOnly
	switch key {
	case input.KeyA:
		ctl.choose(TextPosition{}, ctl.buf.end())
	case input.KeyC:
		if !sel.Empty() {
			r.Host().SetClipboardText(ctl.SelectedText())
		}
	case input.KeyX:
		if !sel.Empty() && !ro {
			r.Host().SetClipboardText(ctl.SelectedText())
			ctl.replace(sel.Start, sel.End, "", kit.EditOther)
		}
	case input.KeyV:
		if !ro {
			r.insert(r.Host().ClipboardText(), kit.EditOther)
		}
	case input.KeyZ:
		if ro {
			return true
		}
		if shift {
			ctl.Redo()
		} else {
			ctl.Undo()
		}
	case input.KeyY:
		if !ro {
			ctl.Redo()
		}
	case input.KeyF:
		r.state.openFind(false)
	case input.KeyH:
		r.state.openFind(!ro)
	default:
		return false
	}
	return true
}

// findKey moves between matches with F3 and closes the find bar with Escape.
func (r *renderEditor) findKey(key input.KeyCode, control, shift bool) bool {
	switch {
	case key == input.KeyF3 && !control:
		switch {
		case !r.ctl.find.active():
			r.state.openFind(false)
		case shift:
			r.ctl.FindPrevious()
		default:
			r.ctl.FindNext()
		}
		return true
	case key == input.KeyEscape && r.state.finding && r.composition == nil:
		r.state.closeFind()
		return true
	}
	return false
}

// move handles the keys that move the caret.
func (r *renderEditor) move(key input.KeyCode, control, shift bool) bool {
	ctl := r.ctl
	caret := r.sel().caret
	sel := ctl.Selection().Ordered()
	l := r.lineLayout(caret.Line)
	keepGoal := false
	var target TextPosition
	switch key {
	case input.KeyLeft:
		switch {
		case !sel.Empty() && !shift && !control:
			target = sel.Start
		case control:
			target = r.prevWord(caret, l)
		default:
			target = r.prevCaret(caret, l, true)
		}
	case input.KeyRight:
		switch {
		case !sel.Empty() && !shift && !control:
			target = sel.End
		case control:
			target = r.nextWord(caret, l)
		default:
			target = r.nextCaret(caret, l, true)
		}
	case input.KeyHome:
		target = TextPosition{Line: caret.Line, Column: l.LineStart(caret.Column)}
		if control {
			target = TextPosition{}
		}
	case input.KeyEnd:
		target = TextPosition{Line: caret.Line, Column: l.LineEnd(caret.Column)}
		if control {
			target = ctl.buf.end()
		}
	case input.KeyUp, input.KeyDown:
		if control {
			return false
		}
		keepGoal = true
		rows := 1
		if key == input.KeyUp {
			rows = -1
		}
		target = r.verticalMove(caret, l, rows)
	case input.KeyPageUp, input.KeyPageDown:
		keepGoal = true
		rows := max(int(r.size.H/r.lineHeight)-1, 1)
		if key == input.KeyPageUp {
			rows = -rows
		}
		target = r.pageMove(caret, l, rows)
	default:
		return false
	}
	if !keepGoal {
		r.goalOK = false
	}
	anchor := target
	if shift {
		anchor = r.sel().anchor
	}
	ctl.choose(anchor, target)
	return true
}

// prevCaret returns the caret position before p: the end of the line above at
// the start of a line. visual takes the step to the left on screen instead,
// which in right-to-left text is a step on.
func (r *renderEditor) prevCaret(p TextPosition, l *text.Layout, visual bool) TextPosition {
	if visual {
		if c := visualStep(l, p.Column, -1); c != p.Column {
			return TextPosition{Line: p.Line, Column: c}
		}
		if r.rightToLeft() {
			return r.lineAfter(p)
		}
		return r.lineBefore(p)
	}
	if c := l.PrevCaret(p.Column); c != p.Column {
		return TextPosition{Line: p.Line, Column: c}
	}
	return r.lineBefore(p)
}

// nextCaret returns the caret position after p. visual takes the step to the
// right on screen instead.
func (r *renderEditor) nextCaret(p TextPosition, l *text.Layout, visual bool) TextPosition {
	if visual {
		if c := visualStep(l, p.Column, 1); c != p.Column {
			return TextPosition{Line: p.Line, Column: c}
		}
		if r.rightToLeft() {
			return r.lineBefore(p)
		}
		return r.lineAfter(p)
	}
	if p.Column < len(r.ctl.buf.line(p.Line)) {
		if c := l.NextCaret(p.Column); c != p.Column {
			return TextPosition{Line: p.Line, Column: c}
		}
	}
	return r.lineAfter(p)
}

// visualStep returns the column one step from col on screen, to the right if
// dir is positive and to the left otherwise, or col at that end of the line.
func visualStep(l *text.Layout, col, dir int) int {
	c := l.VisualMove(col, dir)
	if c == col || rowOf(l, c) != rowOf(l, col) {
		return c
	}
	// At the end of a row VisualMove goes on in logical order. Where the row
	// ends against its direction, as "abc" does after Hebrew, that step stays
	// in the row and comes back on screen: the caret is at the end.
	if d := l.CaretPosition(c) - l.CaretPosition(col); dir > 0 && d < 0 || dir <= 0 && d > 0 {
		return col
	}
	return c
}

// rightToLeft reports whether the text reads right to left, which is the
// direction of its first line that is not empty. It decides which line the
// caret goes to from the left or right end of any line: if every line decided
// for itself, the caret could not leave the seam between two lines of
// different directions.
func (r *renderEditor) rightToLeft() bool {
	for i := range r.ctl.buf.lineCount() {
		if r.ctl.buf.line(i) != "" {
			return r.lineLayout(i).Direction() == text.DirectionRTL
		}
	}
	return false
}

// lineBefore returns the end of the line above p, or p in the first line.
func (r *renderEditor) lineBefore(p TextPosition) TextPosition {
	if p.Line == 0 {
		return p
	}
	return TextPosition{Line: p.Line - 1, Column: len(r.ctl.buf.line(p.Line - 1))}
}

// lineAfter returns the start of the line below p, or p in the last line.
func (r *renderEditor) lineAfter(p TextPosition) TextPosition {
	if p.Line+1 >= r.ctl.buf.lineCount() {
		return p
	}
	return TextPosition{Line: p.Line + 1}
}

func (r *renderEditor) prevWord(p TextPosition, l *text.Layout) TextPosition {
	if p.Column == 0 {
		return r.prevCaret(p, l, false)
	}
	return TextPosition{Line: p.Line, Column: l.PrevWord(p.Column)}
}

func (r *renderEditor) nextWord(p TextPosition, l *text.Layout) TextPosition {
	if p.Column >= len(r.ctl.buf.line(p.Line)) {
		return r.nextCaret(p, l, false)
	}
	return TextPosition{Line: p.Line, Column: l.NextWord(p.Column)}
}

// rowOf returns the row of l that holds the caret at col.
func rowOf(l *text.Layout, col int) int {
	n := l.LineCount()
	for i := range n {
		_, end := l.LineRange(i)
		if col < end || i == n-1 {
			return i
		}
		if col == end {
			if next, _ := l.LineRange(i + 1); next > col {
				return i
			}
		}
	}
	return n - 1
}

// verticalMove returns the position rows rows from p, keeping the goal
// position along the row.
func (r *renderEditor) verticalMove(p TextPosition, l *text.Layout, rows int) TextPosition {
	if !r.goalOK {
		r.goalX, r.goalOK = l.CaretPosition(r.displayColumn(p)), true
	}
	row := rowOf(l, p.Column) + rows
	line := p.Line
	for row < 0 {
		if line == 0 {
			return TextPosition{}
		}
		line--
		row += r.lineLayout(line).LineCount()
	}
	for {
		n := r.lineLayout(line).LineCount()
		if row < n {
			break
		}
		if line+1 >= r.ctl.buf.lineCount() {
			return r.ctl.buf.end()
		}
		row -= n
		line++
	}
	target := r.lineLayout(line)
	start, _ := target.LineRange(row)
	return TextPosition{Line: line, Column: target.VerticalMove(start, 0, r.goalX)}
}

// pageMove returns the position a page of rows from p, and moves the view by
// as much.
func (r *renderEditor) pageMove(p TextPosition, l *text.Layout, rows int) TextPosition {
	target := r.verticalMove(p, l, rows)
	limit := r.scrollLimit(r.size.H)
	top := max(min(r.top()+float64(rows)*float64(r.lineHeight), limit), 0)
	r.state.Jump(top)
	return target
}

// edit handles the keys that change the text.
func (r *renderEditor) edit(key input.KeyCode, control, shift bool) bool {
	ctl := r.ctl
	sel := ctl.Selection().Ordered()
	caret := r.sel().caret
	switch key {
	case input.KeyBackspace, input.KeyDelete, input.KeyEnter:
	case input.KeyTab:
		if r.editor.TabMovesFocus || shift || control || r.editor.ReadOnly {
			return false
		}
	default:
		return false
	}
	if r.editor.ReadOnly {
		return true
	}
	if key == input.KeyEnter && r.composition != nil {
		// The platforms keep the keys an IME works on, but should one arrive,
		// it confirms the conversion, which the IME commits.
		return true
	}
	l := r.lineLayout(caret.Line)
	switch key {
	case input.KeyBackspace:
		switch {
		case !sel.Empty():
			ctl.replace(sel.Start, sel.End, "", kit.EditOther)
		case control:
			ctl.replace(r.prevWord(caret, l), caret, "", kit.EditOther)
		default:
			ctl.replace(r.prevCaret(caret, l, false), caret, "", kit.EditBackspace)
		}
	case input.KeyDelete:
		switch {
		case !sel.Empty():
			ctl.replace(sel.Start, sel.End, "", kit.EditOther)
		case control:
			ctl.replace(caret, r.nextWord(caret, l), "", kit.EditOther)
		default:
			ctl.replace(caret, r.nextCaret(caret, l, false), "", kit.EditForward)
		}
	case input.KeyEnter:
		r.insert("\n", kit.EditTyping)
	case input.KeyTab:
		r.insert("\t", kit.EditTyping)
	}
	return true
}
