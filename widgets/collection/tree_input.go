package collection

import (
	"strings"
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
)

// This file holds what a Tree does with the keyboard and the pointer:
// moving the focus, selecting, opening, renaming and dragging rows.

var _ treeHost = (*treeState[int])(nil)

// setFocus moves the focus of the tree to row i. With claim, the row takes
// the keyboard and scrolls into view.
func (s *treeState[K]) setFocus(i int, claim bool) {
	c := s.ctrl
	c.focus, c.hasFocus = s.rows[i].key, true
	s.focusAt = i
	if claim {
		s.claim = true
		s.reveal(i)
	}
	kit.Rebuild(&s.StateBase)
}

// selectionChanged tells the application about a change it made.
func (s *treeState[K]) selectionChanged() {
	if f := s.Widget().(Tree[K]).OnSelectionChange; f != nil {
		f(s.ctrl.Selection())
	}
	kit.Rebuild(&s.StateBase)
}

// selectOnly selects row i alone and makes it the anchor of ranges.
func (s *treeState[K]) selectOnly(i int) {
	c := s.ctrl
	k := s.rows[i].key
	if len(c.order) == 1 && c.order[0] == k {
		c.anchor, c.hasAnchor = k, true
		return
	}
	c.clear()
	c.setSelected(k, true)
	c.anchor, c.hasAnchor = k, true
	s.selectionChanged()
}

// selectRange selects the rows from the anchor to row i, adding them to the
// selection with add and replacing it otherwise.
func (s *treeState[K]) selectRange(i int, add bool) {
	c := s.ctrl
	a := -1
	if c.hasAnchor {
		a = s.indexOf(c.anchor)
	}
	if a < 0 {
		a = max(s.focusIndex(), 0)
		c.anchor, c.hasAnchor = s.rows[a].key, true
	}
	if !add {
		c.clear()
	}
	for j := min(a, i); j <= max(a, i); j++ {
		if !s.rows[j].loading {
			c.setSelected(s.rows[j].key, true)
		}
	}
	s.selectionChanged()
}

// toggleSelected selects row i or takes it out, and makes it the anchor.
func (s *treeState[K]) toggleSelected(i int) {
	c := s.ctrl
	k := s.rows[i].key
	c.setSelected(k, !c.Selected(k))
	c.anchor, c.hasAnchor = k, true
	s.selectionChanged()
}

// selectAll selects every row that shows.
func (s *treeState[K]) selectAll() {
	for i := range s.rows {
		if !s.rows[i].loading {
			s.ctrl.setSelected(s.rows[i].key, true)
		}
	}
	s.selectionChanged()
}

// moveTo moves the focus to row j by the keyboard, selecting as the modifiers
// say: Shift extends from the anchor, Ctrl moves the focus alone, and nothing
// selects row j alone.
func (s *treeState[K]) moveTo(j int, m input.Modifiers) {
	if j < 0 || j >= len(s.rows) {
		return
	}
	multi := s.Widget().(Tree[K]).Multiselect
	switch {
	case multi && m&input.ModShift != 0:
		s.selectRange(j, m&input.ModControl != 0)
	case multi && m&input.ModControl != 0:
	default:
		s.selectOnly(j)
	}
	s.setFocus(j, true)
}

// step returns the row d rows from i, passing loading rows, within the tree.
func (s *treeState[K]) step(i, d int) int {
	j := min(max(i+d, 0), len(s.rows)-1)
	for j >= 0 && j < len(s.rows) && s.rows[j].loading {
		if d < 0 {
			j--
		} else {
			j++
		}
	}
	if j < 0 || j >= len(s.rows) {
		// Only loading rows lie that way; stay on the nearest node.
		j = min(max(i, 0), len(s.rows)-1)
		for j > 0 && s.rows[j].loading {
			j--
		}
	}
	return j
}

// isAncestor reports whether row a is above row i in the tree.
func (s *treeState[K]) isAncestor(a, i int) bool {
	for p := s.rows[i].parent; p >= 0; p = s.rows[p].parent {
		if int(p) == a {
			return true
		}
	}
	return false
}

// setExpanded opens or closes row i. A focus inside a node that closes goes
// to the node, as ARIA asks.
func (s *treeState[K]) setExpanded(i int, open bool) {
	r := s.rows[i]
	if !r.expandable || r.expanded == open {
		return
	}
	w := s.Widget().(Tree[K])
	if !open {
		if f := s.focusIndex(); f > i && s.isAncestor(i, f) {
			s.ctrl.focus = r.key
			s.claim = s.active
		}
		delete(s.requested, r.key)
	}
	if !basic.ReduceMotionOf(s.Context()) {
		from, to := float32(0), float32(90)
		if !open {
			from, to = to, from
		}
		s.turnKey, s.turning = r.key, true
		s.turn.Jump(from)
		s.turn.Set(to)
	}
	s.ctrl.SetExpanded(r.key, open)
	if open && w.OnLoad != nil && (w.ChildrenOf == nil || len(w.ChildrenOf(r.key)) == 0) {
		// Ask at once rather than on the next frame.
		s.askLoad(&w, r.key)
		s.loadPending(0)
	}
}

// activate runs OnActivate for row i, or opens or closes it without one.
func (s *treeState[K]) activate(i int) {
	w := s.Widget().(Tree[K])
	if w.OnActivate != nil {
		w.OnActivate(s.rows[i].key)
		return
	}
	s.setExpanded(i, !s.rows[i].expanded)
}

// expandSiblings opens the siblings of the focus, for *.
func (s *treeState[K]) expandSiblings() {
	f := s.focusIndex()
	if f < 0 {
		return
	}
	parent := s.rows[f].parent
	for j := range s.rows {
		if r := &s.rows[j]; r.parent == parent && !r.loading && r.expandable && !r.expanded {
			s.ctrl.Expand(r.key)
		}
	}
}

// rowKey implements treeHost.
func (s *treeState[K]) rowKey(e *core.KeyEvent) bool {
	if s.renaming {
		return false
	}
	if s.drag.dragging {
		if e.Key == input.KeyEscape {
			s.endDrag()
			return true
		}
		return false
	}
	i := s.focusIndex()
	if i < 0 || e.Modifiers&input.ModAlt != 0 {
		return false
	}
	w := s.Widget().(Tree[K])
	control := e.Modifiers&input.ModControl != 0
	r := s.rows[i]
	switch e.Key {
	case input.KeyUp:
		s.moveTo(s.step(i, -1), e.Modifiers)
	case input.KeyDown:
		s.moveTo(s.step(i, 1), e.Modifiers)
	case input.KeyHome:
		s.moveTo(s.step(-1, 1), e.Modifiers)
	case input.KeyEnd:
		s.moveTo(s.step(len(s.rows), -1), e.Modifiers)
	case input.KeyPageUp, input.KeyPageDown:
		page := max(int(s.view/treeRowHeight)-1, 1)
		if e.Key == input.KeyPageUp {
			page = -page
		}
		s.moveTo(s.step(i, page), e.Modifiers)
	case input.KeyRight:
		switch {
		case r.expandable && !r.expanded:
			s.setExpanded(i, true)
		case r.expanded && i+1 < len(s.rows) && !s.rows[i+1].loading:
			s.moveTo(i+1, e.Modifiers&^input.ModShift)
		}
	case input.KeyLeft:
		switch {
		case r.expanded:
			s.setExpanded(i, false)
		case r.parent >= 0:
			s.moveTo(int(r.parent), e.Modifiers&^input.ModShift)
		}
	case input.KeyEnter:
		if !e.Repeat {
			s.activate(i)
		}
	case input.KeySpace:
		switch {
		case e.Repeat:
		case w.Multiselect:
			s.toggleSelected(i)
		default:
			s.selectOnly(i)
		}
	case input.KeyA:
		if !control || !w.Multiselect {
			return false
		}
		s.selectAll()
	case input.KeyF2:
		if w.OnRename == nil || e.Modifiers != 0 {
			return false
		}
		s.renameKey, s.renaming = r.key, true
		kit.Rebuild(&s.StateBase)
	default:
		return false
	}
	return true
}

// rowText implements treeHost: * opens the siblings of the focus, and other
// text finds a row by the start of its label.
func (s *treeState[K]) rowText(str string) {
	if s.renaming || s.drag.dragging || str == "" {
		return
	}
	if str == "*" {
		s.expandSiblings()
		return
	}
	if strings.TrimSpace(str) == "" && len(s.ahead.Typed) == 0 {
		// Space selects; it starts no search.
		return
	}
	s.find(strings.ToLower(str))
}

// find adds typed to the search and moves to the next row whose label starts
// with what was typed, as typeAhead does for a list. Repeating one letter
// steps through the rows that start with it.
func (s *treeState[K]) find(typed string) {
	t := &s.ahead
	t.Typed = append(t.Typed, typed...)
	t.Armed = false
	t.Ticker.Start()
	f := s.focusIndex()
	if f < 0 {
		return
	}
	prefix := string(t.Typed)
	start := f
	if first, size := utf8.DecodeRune(t.Typed); sameRune(t.Typed, first) {
		prefix = prefix[:size]
		start = f + 1
	}
	w := s.Widget().(Tree[K])
	n := len(s.rows)
	for k := range n {
		j := (start + k) % n
		if !s.rows[j].loading && strings.HasPrefix(strings.ToLower(s.labelOf(&w, s.rows[j].key)), prefix) {
			s.moveTo(j, 0)
			return
		}
	}
}

// sameRune reports whether b repeats r and nothing else.
func sameRune(b []byte, r rune) bool {
	for len(b) > 0 {
		c, size := utf8.DecodeRune(b)
		if c != r {
			return false
		}
		b = b[size:]
	}
	return true
}

// rowFocused implements treeHost: the row of index i took the keyboard, by
// Tab, a press or a claim.
func (s *treeState[K]) rowFocused(i int) {
	if i < 0 || i >= len(s.rows) || s.rows[i].loading {
		return
	}
	c := s.ctrl
	if !c.hasFocus || c.focus != s.rows[i].key {
		c.focus, c.hasFocus = s.rows[i].key, true
		s.focusAt = i
		kit.Rebuild(&s.StateBase)
	}
	s.reveal(i)
}

// rowAction implements treeHost, for assistive technology.
func (s *treeState[K]) rowAction(i int, a semantics.Actions) bool {
	if i < 0 || i >= len(s.rows) || s.rows[i].loading {
		return false
	}
	switch a {
	case semantics.ActionTap:
		s.selectOnly(i)
		s.setFocus(i, false)
	case semantics.ActionExpand:
		s.setExpanded(i, true)
	case semantics.ActionCollapse:
		s.setExpanded(i, false)
	case semantics.ActionScrollIntoView:
		s.reveal(i)
	default:
		return false
	}
	return true
}

// rowPress implements treeHost.
func (s *treeState[K]) rowPress(i int, pos geom.Offset, e *core.PointerEvent) bool {
	if i < 0 || i >= len(s.rows) || s.rows[i].loading {
		return false
	}
	w := s.Widget().(Tree[K])
	r := s.rows[i]
	if x := treeContentX(int(r.depth)); r.expandable && pos.X >= x-treeGap && pos.X < x+treeChevron+treeGap {
		// The chevron opens and closes, and leaves the selection alone.
		s.setExpanded(i, !r.expanded)
		return false
	}
	multi := w.Multiselect
	control := multi && e.Modifiers&input.ModControl != 0
	shift := multi && e.Modifiers&input.ModShift != 0
	s.drag = treeDrag[K]{pressed: true, key: r.key, nodes: s.drag.nodes[:0], moving: s.drag.moving, open: -1}
	s.drag.start = s.rowTop(i).Add(pos)
	switch {
	case e.Clicks == 2 && !control && !shift:
		s.selectOnly(i)
		s.drag.pressed = false
		s.setFocus(i, false)
		s.activate(i)
		return true
	case shift:
		s.selectRange(i, control)
	case control:
		s.toggleSelected(i)
	case s.ctrl.Selected(r.key) && len(s.ctrl.order) > 1:
		s.drag.narrow = true
	default:
		s.selectOnly(i)
	}
	s.setFocus(i, false)
	return true
}

// rowSecondaryPress implements treeHost: a secondary press selects the row
// alone, unless it is selected already, in which case the selection stays so
// that a context menu acts on all of it, as in Explorer. Either way the row
// takes the focus.
func (s *treeState[K]) rowSecondaryPress(i int) bool {
	if i < 0 || i >= len(s.rows) || s.rows[i].loading {
		return false
	}
	if !s.ctrl.Selected(s.rows[i].key) {
		s.selectOnly(i)
	}
	s.setFocus(i, false)
	return s.Widget().(Tree[K]).OnContextMenu != nil
}

// rowContextMenu implements treeHost.
func (s *treeState[K]) rowContextMenu(i int, at geom.Offset) bool {
	f := s.Widget().(Tree[K]).OnContextMenu
	if f == nil || i < 0 || i >= len(s.rows) || s.rows[i].loading {
		return false
	}
	f(s.rows[i].key, toLocal(s.Context(), at))
	return true
}

// rowTop returns the top of row i relative to the view.
func (s *treeState[K]) rowTop(i int) geom.Offset {
	return geom.Offset{Y: float32(float64(i)*treeRowHeight - s.Offset.Value())}
}

// bodyPress implements treeHost: a press below the rows ends a rename and
// clears the selection.
func (s *treeState[K]) bodyPress() {
	if s.renaming {
		if e := field.FindEditable(s.body); e != nil {
			s.commitRename(s.renameKey, e.Content, false)
		}
		return
	}
	if len(s.ctrl.order) > 0 {
		s.ctrl.clear()
		s.selectionChanged()
	}
}

// commitRename ends the rename of node with name. A name left the same or
// blank changes nothing. The row takes the keyboard back unless the field
// lost it to something else.
func (s *treeState[K]) commitRename(node K, name string, blurred bool) {
	if !s.renaming || s.renameKey != node {
		return
	}
	s.renaming = false
	s.claim = !blurred
	kit.Rebuild(&s.StateBase)
	w := s.Widget().(Tree[K])
	if strings.TrimSpace(name) == "" || name == s.labelOf(&w, node) || w.OnRename == nil {
		return
	}
	w.OnRename(node, name)
}

// cancelRename ends the rename without a change.
func (s *treeState[K]) cancelRename() {
	if s.renaming {
		s.renaming, s.claim = false, true
		kit.Rebuild(&s.StateBase)
	}
}
