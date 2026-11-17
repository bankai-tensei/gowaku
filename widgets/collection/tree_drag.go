package collection

import (
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
)

const (
	// treeDragSlop is how far a press moves before it drags rows.
	treeDragSlop = 4
	// treeDropEdge is the band at the top and the bottom of a row that can
	// open where a drop goes before or after it rather than into it.
	treeDropEdge = treeRowHeight / 4
	// treeScrollZone is the band at each edge of the view where a drag
	// scrolls, at up to treeScrollSpeed pixels a second.
	treeScrollZone  = treeRowHeight
	treeScrollSpeed = 900
	// treeOpenDelay is how long a drag rests on a closed node to open it.
	treeOpenDelay = 700 * time.Millisecond
)

// treeDrag is a press on a row, and the drag it may become.
type treeDrag[K comparable] struct {
	pressed, dragging bool
	// narrow selects only the pressed row on a release that did not drag,
	// which a press on one of several selected rows leaves for later so that
	// it can drag them all.
	narrow bool
	key    K
	// start and pos are where the press began and the pointer is, relative
	// to the rows.
	start, pos geom.Offset
	nodes      []K
	moving     map[K]struct{}
	move       TreeMove[K]
	valid      bool
	// open is the closed node the drag rests on, and since when.
	open      int
	openSince time.Duration
	openTimed bool
	last      time.Duration
}

// rowPointer implements treeHost.
func (s *treeState[K]) rowPointer(kind core.PointerKind, pos geom.Offset) {
	d := &s.drag
	if !d.pressed || s.body == nil {
		return
	}
	p := pos.Sub(core.GlobalOffset(s.body))
	switch kind {
	case core.PointerMove:
		if !d.dragging {
			dx, dy := p.X-d.start.X, p.Y-d.start.Y
			if s.Widget().(Tree[K]).OnMove == nil || s.renaming || dx*dx+dy*dy < treeDragSlop*treeDragSlop {
				return
			}
			s.startDrag()
		}
		d.pos = p
		s.dragOver()
	case core.PointerUp:
		switch {
		case d.dragging:
			d.pos = p
			s.dragOver()
			if d.valid {
				move := d.move
				move.Nodes = append([]K(nil), d.nodes...)
				s.endDrag()
				s.Widget().(Tree[K]).OnMove(move)
				return
			}
		case d.narrow:
			if i := s.indexOf(d.key); i >= 0 {
				s.selectOnly(i)
			}
		}
		s.endDrag()
	case core.PointerCancel:
		s.endDrag()
	}
}

// startDrag lifts the selection if the pressed row is in it, else the
// pressed row alone.
func (s *treeState[K]) startDrag() {
	d := &s.drag
	d.dragging, d.narrow = true, false
	if d.moving == nil {
		d.moving = make(map[K]struct{})
	}
	clear(d.moving)
	d.nodes = d.nodes[:0]
	if !s.ctrl.Selected(d.key) {
		d.nodes = append(d.nodes, d.key)
		d.moving[d.key] = struct{}{}
	} else {
		for i := range s.rows {
			r := &s.rows[i]
			if r.loading || !s.ctrl.Selected(r.key) || s.selectedAbove(i) {
				continue
			}
			d.nodes = append(d.nodes, r.key)
			d.moving[r.key] = struct{}{}
		}
	}
	s.dragClock.Start()
	s.repaintRows()
}

// repaintRows paints the rows again, for a change that only shows in paint.
func (s *treeState[K]) repaintRows() {
	if s.body != nil {
		for _, c := range s.body.children {
			c.MarkNeedsPaint()
		}
	}
}

// dragging implements treeHost.
func (s *treeState[K]) dragging() bool { return s.drag.dragging }

// selectedAbove reports whether an ancestor of row i is selected.
func (s *treeState[K]) selectedAbove(i int) bool {
	for p := s.rows[i].parent; p >= 0; p = s.rows[p].parent {
		if s.ctrl.Selected(s.rows[p].key) {
			return true
		}
	}
	return false
}

// movingAt reports whether row i or one of its ancestors is being dragged.
func (s *treeState[K]) movingAt(i int) bool {
	for p := int32(i); p >= 0; p = s.rows[p].parent {
		if _, ok := s.drag.moving[s.rows[p].key]; ok {
			return true
		}
	}
	return false
}

// dragOver finds where the dragged rows would go, and shows it.
func (s *treeState[K]) dragOver() {
	d := &s.drag
	w := s.Widget().(Tree[K])
	d.valid = false
	ind := treeIndicator{}
	y := float64(d.pos.Y) + s.Offset.Value()
	i := int(y / treeRowHeight)
	if y >= 0 && i < len(s.rows) && d.pos.Y >= 0 && d.pos.Y < s.view && !s.rows[i].loading {
		r := &s.rows[i]
		frac := float32(y - float64(i)*treeRowHeight)
		drop := TreeDropInto
		switch {
		case r.expandable && frac < treeDropEdge, !r.expandable && frac < treeRowHeight/2:
			drop = TreeDropBefore
		case r.expandable && frac >= treeRowHeight-treeDropEdge, !r.expandable:
			drop = TreeDropAfter
		}
		m := TreeMove[K]{Target: r.key, Drop: drop}
		depth := int(r.depth)
		switch {
		case drop == TreeDropInto:
			m.Parent = r.key
			if w.ChildrenOf != nil {
				m.Index = len(w.ChildrenOf(r.key))
			}
		case drop == TreeDropAfter && r.expanded && i+1 < len(s.rows) && s.rows[i+1].parent == int32(i):
			m.Parent, m.Index = r.key, 0
			depth++
		default:
			if r.parent >= 0 {
				m.Parent = s.rows[r.parent].key
			} else {
				m.AtRoot = true
			}
			m.Index = int(r.pos) - 1
			if drop == TreeDropAfter {
				m.Index++
			}
		}
		m.Nodes = d.nodes
		d.valid = !s.movingAt(i) && (w.CanDrop == nil || w.CanDrop(m))
		if d.valid {
			d.move = m
			ind = treeIndicator{shown: true, into: drop == TreeDropInto, row: i, x: treeContentX(depth) + treeChevron}
			ind.line = float64(i) * treeRowHeight
			if drop != TreeDropBefore {
				ind.line += treeRowHeight
			}
		}
		if open := d.valid && drop == TreeDropInto && !r.expanded; open && d.open != i {
			d.open, d.openTimed = i, false
		} else if !open {
			d.open = -1
		}
	} else {
		d.open = -1
	}
	if s.body != nil && s.body.drop != ind {
		s.body.drop = ind
		s.body.MarkNeedsPaint()
	}
	if s.dragSpeed() != 0 || d.open >= 0 {
		s.dragClock.Start()
	}
}

// dragSpeed returns how fast a drag scrolls the view, as a share of
// treeScrollSpeed: negative up, positive down, zero away from the edges.
func (s *treeState[K]) dragSpeed() float32 {
	y := s.drag.pos.Y
	switch {
	case y < treeScrollZone:
		return -min((treeScrollZone-y)/treeScrollZone, 1.5)
	case y > s.view-treeScrollZone:
		return min((y-(s.view-treeScrollZone))/treeScrollZone, 1.5)
	}
	return 0
}

// dragTick scrolls near the edges and opens the closed node a drag rests on.
// It runs only while one of them is to do, so that a drag at rest costs no
// frames.
func (s *treeState[K]) dragTick(now time.Duration) {
	d := &s.drag
	speed := s.dragSpeed()
	if !d.dragging || speed == 0 && d.open < 0 {
		s.dragClock.Stop()
		d.last = 0
		return
	}
	dt := time.Duration(0)
	if d.last > 0 {
		dt = now - d.last
	}
	d.last = now
	if speed != 0 && dt > 0 {
		limit := max(float64(len(s.rows))*treeRowHeight-float64(s.view), 0)
		to := min(max(s.Offset.Value()+float64(speed*treeScrollSpeed)*dt.Seconds(), 0), limit)
		if to != s.Offset.Value() {
			s.Scroller.Jump(to)
			s.Moved()
		}
	}
	if d.open >= 0 && d.open < len(s.rows) {
		switch {
		case !d.openTimed:
			d.openSince, d.openTimed = now, true
		case now-d.openSince >= treeOpenDelay:
			i := d.open
			d.open = -1
			s.setExpanded(i, true)
		}
	}
}

// endDrag forgets the press and hides the indicator.
func (s *treeState[K]) endDrag() {
	if s.drag.dragging {
		s.repaintRows()
	}
	s.drag.pressed, s.drag.dragging, s.drag.narrow, s.drag.valid = false, false, false, false
	s.drag.open, s.drag.last = -1, 0
	s.dragClock.Stop()
	if s.body != nil && s.body.drop.shown {
		s.body.drop = treeIndicator{}
		s.body.MarkNeedsPaint()
	}
}
