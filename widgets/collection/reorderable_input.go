package collection

import (
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
)

// This file holds what a ReorderableList does with the keyboard and the
// pointer: moving the focus, dragging an item and moving it by keys.

// setFocus moves the focus to item i. With claim, the item takes the keyboard
// and scrolls into view.
func (s *reorderState) setFocus(i int, claim bool) {
	s.focus = i
	if claim {
		s.claim = true
		s.reveal(i)
	}
	kit.Rebuild(&s.StateBase)
}

// key handles a key pressed while the item of the focus has the keyboard.
func (s *reorderState) key(e *core.KeyEvent) bool {
	n, i := s.count, s.focus
	if n == 0 {
		return false
	}
	if e.Key == input.KeyEscape && s.drag.pressed {
		s.cancelDrag()
		return true
	}
	if s.drag.dragging || e.Modifiers&input.ModAlt != 0 {
		return false
	}
	if e.Modifiers == input.ModControl|input.ModShift {
		to := i
		switch e.Key {
		case input.KeyUp:
			to--
		case input.KeyDown:
			to++
		case input.KeyHome:
			to = 0
		case input.KeyEnd:
			to = n - 1
		default:
			return false
		}
		to = min(max(to, 0), n-1)
		if to != i && s.Widget().(ReorderableList).OnReorder != nil {
			s.move(i, to, false, 0)
			s.setFocus(to, true)
		}
		return true
	}
	if e.Modifiers != 0 {
		return false
	}
	j := i
	switch e.Key {
	case input.KeyEnter, input.KeySpace:
		if tap := s.Widget().(ReorderableList).OnTap; tap != nil {
			tap(i)
			return true
		}
		return false
	case input.KeyUp:
		j--
	case input.KeyDown:
		j++
	case input.KeyHome:
		j = 0
	case input.KeyEnd:
		j = n - 1
	case input.KeyPageUp, input.KeyPageDown:
		page := 1
		if h := s.extentOf(i); h > 0 {
			page = max(int(s.view/h)-1, 1)
		}
		if e.Key == input.KeyPageUp {
			page = -page
		}
		j += page
	default:
		return false
	}
	s.setFocus(min(max(j, 0), n-1), true)
	return true
}

// move asks the caller to move the item at from to the index to. The items
// slide to their new places at the next layout: from top, in the content,
// for an item dropped by the pointer.
func (s *reorderState) move(from, to int, dropped bool, top float64) {
	w := s.Widget().(ReorderableList)
	label := ""
	if w.LabelOf != nil {
		label = w.LabelOf(from)
	}
	s.settle = reorderSettle{pending: !s.still, dropped: dropped, from: from, to: to, top: top}
	// The moved item keeps the keyboard, though another object now shows it.
	s.focus, s.claim = to, true
	if s.words != nil {
		s.announce = s.words.MovedTo(label, to+1, s.count)
	}
	kit.Rebuild(&s.StateBase)
	w.OnReorder(from, to)
}

// shiftTarget returns how far item i stands from its place at rest while an
// item is dragged: those between the dragged item and where it would go make
// room for it.
func (s *reorderState) shiftTarget(i int) float32 {
	d := &s.drag
	if !d.dragging || i == d.index {
		return 0
	}
	h := s.extentOf(d.index)
	switch {
	case d.index < i && i <= d.target:
		return -h
	case d.target <= i && i < d.index:
		return h
	}
	return 0
}

// settleStart returns where item i starts to slide from after the move of
// settle, relative to its new place.
func (s *reorderState) settleStart(i int) float32 {
	m := &s.settle
	from, to := m.from, m.to
	moved := s.extentOf(to)
	if i == to {
		switch {
		case m.dropped:
			return float32(m.top - s.top(to))
		case from < to:
			return float32(s.top(from) - s.top(to))
		}
		return float32(s.top(from)-s.top(to)) + s.extentOf(from) - moved
	}
	switch {
	case m.dropped:
		// The others made room already while the item was dragged.
		return 0
	case from < to && from <= i && i < to:
		return moved
	case to < from && to < i && i <= from:
		return -moved
	}
	return 0
}

// local returns a point of the window relative to the view.
func (s *reorderState) local(pos geom.Offset) geom.Offset {
	if s.body == nil {
		return pos
	}
	return pos.Sub(core.GlobalOffset(s.body))
}

// press starts the press of item i at pos in the window.
func (s *reorderState) press(i int, pos geom.Offset) {
	if i < 0 || i >= s.count {
		return
	}
	p := s.local(pos)
	s.drag = reorderDrag{pressed: true, index: i, target: i, start: p, pos: p, grab: float32(float64(p.Y) + s.Offset.Value() - s.top(i))}
	s.setFocus(i, false)
}

// pointer follows the pointer after a press, at pos in the window.
func (s *reorderState) pointer(kind core.PointerKind, pos geom.Offset) {
	d := &s.drag
	if !d.pressed || s.body == nil {
		return
	}
	p := s.local(pos)
	switch kind {
	case core.PointerMove:
		if !d.dragging {
			dx, dy := p.X-d.start.X, p.Y-d.start.Y
			if s.Widget().(ReorderableList).OnReorder == nil || dx*dx+dy*dy < reorderDragSlop*reorderDragSlop {
				return
			}
			d.dragging = true
		}
		d.pos = p
		s.dragOver()
	case core.PointerUp:
		if !d.dragging {
			i := d.index
			s.endDrag()
			s.tapAt(i, p)
			return
		}
		d.pos = p
		s.dragOver()
		s.drop()
	case core.PointerCancel:
		s.cancelDrag()
	}
}

// tapAt runs OnTap for item i, pressed and released at p in the view without
// a drag, if the release is over the same item.
func (s *reorderState) tapAt(i int, p geom.Offset) {
	tap := s.Widget().(ReorderableList).OnTap
	if tap == nil || s.body == nil {
		return
	}
	sz := s.body.Size()
	if p.X < 0 || p.Y < 0 || p.X >= sz.W || p.Y >= sz.H || s.indexAt(float64(p.Y)+s.Offset.Value()) != i {
		return
	}
	tap(i)
}

// dragTop returns the top of the dragged item in the view, which follows the
// pointer but stays half in view.
func (s *reorderState) dragTop() float32 {
	d := &s.drag
	h := s.extentOf(d.index)
	return geom.Clamp32(d.pos.Y-d.grab, -h/2, max(s.view-h/2, -h/2))
}

// dragOver finds where the dragged item would go: the place under its
// middle. The items between make room.
func (s *reorderState) dragOver() {
	d := &s.drag
	if !d.dragging || s.count == 0 {
		return
	}
	mid := float64(s.dragTop()+s.extentOf(d.index)/2) + s.Offset.Value()
	d.target = min(max(s.indexAt(mid), 0), s.count-1)
	if s.body != nil {
		s.body.retarget()
		s.body.scrolled()
	}
	if s.dragSpeed() != 0 {
		s.clock.Start()
	}
}

// dragSpeed returns how fast a drag scrolls the view, as a share of
// reorderScrollSpeed: negative up, positive down, zero away from the edges
// and where the view cannot go further.
func (s *reorderState) dragSpeed() float32 {
	if !s.drag.dragging {
		return 0
	}
	zone := min(float32(reorderScrollZone), s.view/4)
	if zone <= 0 {
		return 0
	}
	y, off := s.drag.pos.Y, s.Offset.Value()
	switch {
	case y < zone && off > 0:
		return -min((zone-y)/zone, 1.5)
	case y > s.view-zone && off < s.content()-float64(s.view):
		return min((y-(s.view-zone))/zone, 1.5)
	}
	return 0
}

// drop ends a drag where the pointer let go.
func (s *reorderState) drop() {
	d := s.drag
	top := float64(s.dragTop()) + s.Offset.Value()
	s.drag = reorderDrag{}
	if d.target != d.index && s.Widget().(ReorderableList).OnReorder != nil {
		s.move(d.index, d.target, true, top)
		if s.body != nil {
			s.body.retarget()
		}
		return
	}
	s.slideBack(d.index, top)
}

// cancelDrag puts a dragged item back, as Escape does.
func (s *reorderState) cancelDrag() {
	d := s.drag
	if !d.dragging {
		s.endDrag()
		return
	}
	top := float64(s.dragTop()) + s.Offset.Value()
	s.drag = reorderDrag{}
	s.slideBack(d.index, top)
}

// slideBack lets item i slide from top, in the content, to its place, and the
// others close the room they made.
func (s *reorderState) slideBack(i int, top float64) {
	if s.body != nil {
		s.body.startAt(i, float32(top-s.top(i)))
		s.body.retarget()
		s.body.scrolled()
	}
	kit.Rebuild(&s.StateBase)
}

// endDrag forgets the press, and puts the items back at once.
func (s *reorderState) endDrag() {
	was := s.drag.dragging
	s.drag = reorderDrag{}
	if was && s.body != nil {
		s.body.retarget()
		s.body.scrolled()
	}
}

// tick scrolls a drag near an edge and slides the items. It runs only while
// one of them is to do, so that a drag at rest costs no frames.
func (s *reorderState) tick(now time.Duration) {
	dt := time.Duration(0)
	if s.lastTick > 0 {
		dt = now - s.lastTick
	}
	s.lastTick = now
	busy := false
	if speed := s.dragSpeed(); speed != 0 {
		busy = true
		if dt > 0 {
			limit := max(s.content()-float64(s.view), 0)
			to := min(max(s.Offset.Value()+float64(speed*reorderScrollSpeed)*dt.Seconds(), 0), limit)
			if to != s.Offset.Value() {
				s.Scroller.Jump(to)
				s.Moved()
			}
		}
	}
	if s.body != nil && s.body.slide(dt) {
		busy = true
	}
	if !busy {
		s.clock.Stop()
		s.lastTick = 0
	}
}

// itemFocused follows item i taking the keyboard, by Tab, a press or a claim.
func (s *reorderState) itemFocused(i int) {
	if i < 0 || i >= s.count {
		return
	}
	if s.focus != i {
		s.focus = i
		kit.Rebuild(&s.StateBase)
	}
	s.reveal(i)
}

// itemAction does what assistive technology asks of item i.
func (s *reorderState) itemAction(i int, a semantics.Actions) bool {
	if i < 0 || i >= s.count {
		return false
	}
	switch a {
	case semantics.ActionScrollIntoView:
		s.reveal(i)
		return true
	case semantics.ActionTap:
		if tap := s.Widget().(ReorderableList).OnTap; tap != nil {
			tap(i)
			return true
		}
	}
	return false
}
