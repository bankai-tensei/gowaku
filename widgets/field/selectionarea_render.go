package field

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// renderSelectionRegion is the render object of SelectionArea. It sits above
// the texts it selects, so it gets the presses that nothing below it takes:
// a button or a field inside keeps its own.
type renderSelectionRegion struct {
	layout.RenderProxy
	sel    *selection
	onMenu func(geom.Offset)
	// secondary is a press of the secondary button that may open the menu.
	secondary bool
	// overText is whether the last hit test found a text of the area under
	// the pointer, for the cursor.
	overText bool
}

// HitTest implements core.RenderObject. It notes whether the pointer is over a
// text of the area, which is what the cursor follows. It does not allocate.
func (r *renderSelectionRegion) HitTest(pos geom.Offset, res *core.HitTestResult) bool {
	n := len(res.Path)
	if !r.RenderProxy.HitTest(pos, res) {
		return false
	}
	found, control := false, false
	for _, ro := range res.Path[n : len(res.Path)-1] {
		switch t := ro.(type) {
		case basic.SelectableText:
			found = found || t.TextLayout() != nil
		case core.FocusTarget:
			control = true
		}
	}
	r.overText = found && !control
	return true
}

// MouseCursor implements core.CursorSource: the I-beam over text, the arrow
// elsewhere.
func (r *renderSelectionRegion) MouseCursor() input.Cursor {
	if r.overText {
		return input.CursorIBeam
	}
	return input.CursorArrow
}

// HandlePointer implements core.PointerTarget.
func (r *renderSelectionRegion) HandlePointer(e *core.PointerEvent) {
	s := r.sel
	switch e.Kind {
	case core.PointerDown:
		if e.Device == input.DeviceTouch {
			return
		}
		switch e.Button {
		case input.ButtonPrimary:
			pt, ok := s.pointAt(e.Pos)
			if !ok {
				return
			}
			e.Handled = true
			s.press(pt, e.Clicks, e.Modifiers&input.ModShift != 0)
		case input.ButtonSecondary:
			if r.onMenu != nil {
				e.Handled = true
				r.secondary = true
			}
		}
	case core.PointerMove:
		s.dragTo(e.Pos)
	case core.PointerUp:
		s.dragging = false
		if r.secondary {
			r.secondary = false
			if r.onMenu != nil {
				r.onMenu(e.Pos)
			}
		}
	case core.PointerCancel:
		s.dragging = false
		r.secondary = false
	}
}

// CanFocus implements core.FocusTarget. The area takes the keyboard once a
// press has put an anchor down, and is no stop for Tab before that.
func (r *renderSelectionRegion) CanFocus() bool { return r.sel != nil && r.sel.anchor.p != nil }

// FocusChanged implements core.FocusTarget. Losing the focus to something else
// in the window drops the selection; the window going to the background, or
// the menu of the area opening, does not.
func (r *renderSelectionRegion) FocusChanged(focused bool) {
	s := r.sel
	if focused {
		s.menuOpen = false
		return
	}
	if s.menuOpen || r.Focused() == core.FocusTarget(r) {
		return
	}
	s.dragging = false
	s.clear()
}

// HandleText implements core.FocusTarget. The area takes no text.
func (r *renderSelectionRegion) HandleText(string) {}

// HandleComposition implements core.FocusTarget.
func (r *renderSelectionRegion) HandleComposition(*input.Composition) {}

// HandleKey implements core.FocusTarget.
func (r *renderSelectionRegion) HandleKey(e *core.KeyEvent) bool {
	s := r.sel
	control := e.Modifiers&input.ModControl != 0
	shift := e.Modifiers&input.ModShift != 0
	switch {
	case e.Key == input.KeyC && e.Modifiers == input.ModControl:
		if !s.hasRange() {
			return false
		}
		s.copy()
		return true
	case e.Key == input.KeyA && e.Modifiers == input.ModControl:
		s.selectAll()
		return true
	case e.Key == input.KeyEscape && e.Modifiers == 0:
		if !s.hasRange() {
			return false
		}
		// The selection collapses to its free end, which keeps an anchor for
		// Shift and the keyboard in the area.
		s.anchor = s.focus
		s.apply()
		return true
	case e.Key == input.KeyContextMenu || e.Key == input.KeyF10 && e.Modifiers == input.ModShift:
		if r.onMenu == nil || !s.valid() {
			return false
		}
		r.onMenu(r.caretBottom())
		return true
	case shift && !s.dragging:
		return r.extend(e.Key, control)
	}
	return false
}

// caretBottom returns the foot of the free end, relative to the area: where a
// menu opened from the keyboard goes.
func (r *renderSelectionRegion) caretBottom() geom.Offset {
	f := r.sel.focus
	c := f.p.TextLayout().CaretRect(f.off).Translate(f.p.TextOrigin())
	global := core.LocalToGlobal(f.p, geom.Offset{X: c.X, Y: c.Y + c.H})
	pos, _ := core.GlobalToLocal(r, global)
	return pos
}

// extend moves the free end for Shift with an arrow, Home or End, and reports
// whether the key was one of them.
func (r *renderSelectionRegion) extend(key input.KeyCode, control bool) bool {
	s := r.sel
	if !s.valid() {
		return false
	}
	p, off := s.focus.p, s.focus.off
	l := p.TextLayout()
	vertical := l.Orientation() == text.Vertical
	key = logicalArrow(key, vertical)
	target := s.focus
	vertMove := false
	switch key {
	case input.KeyLeft, input.KeyRight:
		dir := 1
		if key == input.KeyLeft {
			dir = -1
		}
		forward := dir > 0 != (l.Direction() == text.DirectionRTL)
		var next int
		switch {
		case control && forward:
			next = l.NextWord(off)
		case control:
			next = l.PrevWord(off)
		default:
			next = l.VisualMove(off, dir)
		}
		target.off = next
		if next == off {
			target = s.neighbor(p, forward)
		}
	case input.KeyHome:
		target.off = l.LineStart(off)
		if control {
			target = s.edge(false)
		}
	case input.KeyEnd:
		target.off = l.LineEnd(off)
		if control {
			target = s.edge(true)
		}
	case input.KeyUp, input.KeyDown:
		vertMove = true
		target = r.lineMove(key == input.KeyDown)
	default:
		return false
	}
	if !vertMove {
		s.goalOK = false
	}
	if target.p != nil && target != s.focus {
		s.focus = target
		s.apply()
	}
	return true
}

// neighbor returns the start of the paragraph after p, or the end of the one
// before it, where a move by a character leaves p. At the ends of the area it
// stays where it is.
func (s *selection) neighbor(p basic.SelectableText, forward bool) selPoint {
	i := indexOf(s.paras, p)
	switch {
	case forward && i+1 < len(s.paras):
		return selPoint{s.paras[i+1], 0}
	case !forward && i > 0:
		q := s.paras[i-1]
		return selPoint{q, len(q.Text())}
	}
	return s.focus
}

// edge returns the start of the first paragraph, or the end of the last.
func (s *selection) edge(end bool) selPoint {
	if !end {
		return selPoint{s.paras[0], 0}
	}
	q := s.paras[len(s.paras)-1]
	return selPoint{q, len(q.Text())}
}

// lineMove returns the place a line down or up from the free end, keeping to
// the goal position along the lines, into the next or previous paragraph at
// the last or first line.
func (r *renderSelectionRegion) lineMove(down bool) selPoint {
	s := r.sel
	p, off := s.focus.p, s.focus.off
	l := p.TextLayout()
	vertical := l.Orientation() == text.Vertical
	if !s.goalOK {
		s.goal, s.goalOK = l.CaretPosition(off)+r.inlineOrigin(p, vertical), true
	}
	line := lineIndex(l, off)
	rows := -1
	if down {
		rows = 1
	}
	if line+rows >= 0 && line+rows < l.LineCount() {
		return selPoint{p, l.VerticalMove(off, rows, s.goal-r.inlineOrigin(p, vertical))}
	}
	i := indexOf(s.paras, p) + rows
	if i < 0 || i >= len(s.paras) {
		return s.focus
	}
	q := s.paras[i]
	lq := q.TextLayout()
	from := 0
	if !down {
		from = len(q.Text())
	}
	qVertical := lq.Orientation() == text.Vertical
	return selPoint{q, lq.VerticalMove(from, 0, s.goal-r.inlineOrigin(q, qVertical))}
}

// inlineOrigin returns where the lines of p start along themselves, relative
// to the area: its x in horizontal text, its y in vertical text.
func (r *renderSelectionRegion) inlineOrigin(p basic.SelectableText, vertical bool) float32 {
	at, _ := core.GlobalToLocal(r, core.LocalToGlobal(p, p.TextOrigin()))
	if vertical {
		return at.Y
	}
	return at.X
}

// lineIndex returns the line of l that holds the caret at off.
func lineIndex(l *text.Layout, off int) int {
	start := l.LineStart(off)
	for i := range l.LineCount() {
		if s, _ := l.LineRange(i); s == start {
			return i
		}
	}
	return 0
}

// logicalArrow turns an arrow key into the one that does the same in
// horizontal text: in vertical text Down goes on along the line and Left on to
// the next line, as in Input.
func logicalArrow(key input.KeyCode, vertical bool) input.KeyCode {
	if !vertical {
		return key
	}
	switch key {
	case input.KeyUp:
		return input.KeyLeft
	case input.KeyDown:
		return input.KeyRight
	case input.KeyLeft:
		return input.KeyDown
	case input.KeyRight:
		return input.KeyUp
	}
	return key
}
