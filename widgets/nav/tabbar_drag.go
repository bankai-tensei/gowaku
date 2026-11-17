package nav

import (
	"slices"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
)

// TabGroup joins TabBars, in one window or in several, so that a tab dragged
// off one can be dropped on another. Give each bar the same *TabGroup in its
// Group field; the bars join it while they are built and leave it when they
// go. The zero value is ready to use. It belongs to the UI goroutine, which
// every window shares.
//
// A bar in another window is found by where the windows are on the screen,
// which Wayland does not tell; there, only bars of the same window are found.
type TabGroup struct {
	bars []*tabBarState
	// dropper follows the tabs dragged off the bars, or is nil.
	dropper tabDropper
}

// tabDropper follows a tab dragged off the bars of a group, and may take it
// where it is let go over none of them, as DockLayout docks it. Positions are
// in the window of the bar the tab left.
type tabDropper interface {
	// tabDragged reports the pointer of a drag at global; free says the tab
	// is off its strip and over no bar of the group.
	tabDragged(key core.Key, global geom.Offset, free bool)
	// tabDropped asks to take a tab let go at global over no bar, and reports
	// whether it did.
	tabDropped(key core.Key, global geom.Offset) bool
	// tabDragEnded reports that the drag is over. It may come while the tree
	// is being built, and must not set state.
	tabDragEnded()
}

// dropper returns the dropper of the group of s, or nil.
func (s *tabBarState) dropper() tabDropper {
	if s.group == nil {
		return nil
	}
	return s.group.dropper
}

// join adds s to g.
func (g *TabGroup) join(s *tabBarState) {
	if !slices.Contains(g.bars, s) {
		g.bars = append(g.bars, s)
	}
}

// leave takes s out of g, and out of any drag of another bar.
func (g *TabGroup) leave(s *tabBarState) {
	g.bars = slices.DeleteFunc(g.bars, func(b *tabBarState) bool { return b == s })
	for _, b := range g.bars {
		if b.drag.over == s {
			b.drag.over = nil
		}
	}
}

// joinGroup moves s to the group g, which may be nil.
func (s *tabBarState) joinGroup(g *TabGroup) {
	if s.group == g {
		return
	}
	if s.group != nil {
		s.setOver(nil, 0)
		s.group.leave(s)
	}
	s.group = g
	if g != nil {
		g.join(s)
	}
}

// barAt returns the bar of g other than from whose strip is under the point
// pos of the strip of from, with the point in its own strip. A bar of another
// window counts where the window is seen at the point.
func (g *TabGroup) barAt(from *tabBarState, pos geom.Offset) (*tabBarState, geom.Offset) {
	origin, scale, screenOK := core.ScreenOf(from.row)
	screen := origin.Add(pos.Scale(scale))
	global := core.GlobalOffset(from.row).Add(pos)
	for _, b := range g.bars {
		if b == from || b.row == nil || !b.row.Attached() || b.Widget().OnReceive == nil || b.Widget().OnSelect == nil {
			continue
		}
		var local geom.Offset
		switch {
		case core.SameTree(b.row, from.row):
			local = global.Sub(core.GlobalOffset(b.row))
		case screenOK:
			o, sc, ok := core.ScreenOf(b.row)
			h, isScreen := core.ScreenHostOf(b.row)
			if !ok || !isScreen || !h.ShowsScreenPoint(screen) {
				continue
			}
			local = screen.Sub(o).Scale(1 / sc)
		default:
			continue
		}
		sz := b.row.Size()
		if local.X >= 0 && local.X < sz.W && local.Y >= -tabTearDistance/2 && local.Y < sz.H+tabTearDistance/2 {
			return b, local
		}
	}
	return nil, geom.Offset{}
}

// dragPhase is how far the press of a tab has gone.
type dragPhase uint8

const (
	dragNone dragPhase = iota
	// dragPressed is a press that has not moved far enough to drag.
	dragPressed
	// dragDragging moves the tab along the strip.
	dragDragging
	// dragDetached has the tab off the strip, over another bar or nothing.
	dragDetached
)

// tabDrag is the press or drag of a tab. Positions are in the strip, whose
// left edge scrolls with the tabs; x is the left edge of the dragged tab
// there.
type tabDrag struct {
	phase dragPhase
	index int
	key   core.Key
	item  TabItem
	// start is where the press was, grab where it held the tab, from the
	// tab's top-left corner, and pos where the pointer is.
	start, grab, pos geom.Offset
	x                float32
	// target is where the tab goes on the strip.
	target int
	// over is the bar of the group that the detached tab is over, or nil,
	// and overIndex where the tab would go there.
	over      *tabBarState
	overIndex int
}

// clamp forgets a drag whose tab is gone.
func (d *tabDrag) clamp(n int) {
	if d.phase != dragNone && d.index >= n {
		*d = tabDrag{}
	}
}

// tabIncoming is a tab of another bar that would drop on this one at index.
type tabIncoming struct {
	key   core.Key
	item  TabItem
	index int
}

// setIncoming shows where a tab of another bar would drop, or nothing for nil.
func (s *tabBarState) setIncoming(in *tabIncoming) {
	if s.incoming == nil && in == nil {
		return
	}
	if s.incoming != nil && in != nil && *s.incoming == *in {
		return
	}
	s.incoming = in
	kit.Rebuild(&s.StateBase)
}

// setOver moves the detached tab over the bar b, at index, or over nothing.
func (s *tabBarState) setOver(b *tabBarState, index int) {
	d := &s.drag
	if d.over != nil && d.over != b {
		d.over.setIncoming(nil)
	}
	d.over, d.overIndex = b, index
	if b != nil {
		b.setIncoming(&tabIncoming{key: d.key, item: d.item, index: index})
	}
}

// tabWidth returns the width of a tab at the last layout.
func (s *tabBarState) tabWidth() float32 {
	if s.run == nil || s.run.tabW <= 0 {
		return tabMaxWidth
	}
	return s.run.tabW
}

// canTear reports whether a tab may come off the strip: to a window of its
// own, or to another bar.
func (s *tabBarState) canTear() bool {
	w := s.Widget()
	return w.OnTearOff != nil || (s.group != nil && w.OnDetach != nil)
}

// down starts the press of tab i, which selects it, at pos in the strip.
func (s *tabBarState) down(i int, pos geom.Offset) {
	w := s.Widget()
	if i < 0 || i >= len(w.Tabs) {
		return
	}
	s.selectTab(i)
	var grab geom.Offset
	if s.run != nil && i < len(s.run.children) {
		grab = pos.Sub(s.run.children[i].Offset())
	}
	s.drag = tabDrag{phase: dragPressed, index: i, key: s.keyOf(i), item: w.Tabs[i], start: pos, grab: grab, pos: pos, target: i}
}

// move follows the pointer of a press.
func (s *tabBarState) move(pos geom.Offset) {
	d := &s.drag
	w := s.Widget()
	n := len(w.Tabs)
	if d.phase == dragNone || d.index >= n || s.run == nil {
		return
	}
	if d.phase == dragPressed {
		if geom.Abs32(pos.X-d.start.X) < tabDragSlop && geom.Abs32(pos.Y-d.start.Y) < tabDragSlop {
			return
		}
		if w.OnMove == nil && !s.canTear() {
			return
		}
		d.phase = dragDragging
	}
	d.pos = pos
	// Across, the strip reaches as far as the whole bar.
	sz := s.run.Size()
	if s.row != nil {
		sz.W = s.row.Size().W
	}
	away := pos.Y < -tabTearDistance || pos.Y > sz.H+tabTearDistance ||
		pos.X < -tabTearDistance || pos.X > sz.W+tabTearDistance
	switch {
	case away && s.canTear():
		d.phase = dragDetached
	case d.phase == dragDetached && !away:
		d.phase = dragDragging
	}
	if d.phase == dragDragging {
		s.setOver(nil, 0)
		tw := s.tabWidth()
		d.x = geom.Clamp32(pos.X+s.run.scroll-d.grab.X, 0, float32(n-1)*tw)
		d.target = d.index
		if w.OnMove != nil {
			d.target = min(max(int(geom.Floor32(d.x/tw+0.5)), 0), n-1)
		}
	} else {
		s.findOver(pos)
	}
	if dr := s.dropper(); dr != nil {
		dr.tabDragged(d.key, core.GlobalOffset(s.run).Add(pos), d.phase == dragDetached && d.over == nil)
	}
	kit.Rebuild(&s.StateBase)
}

// findOver finds the bar under the detached tab at pos of the strip.
func (s *tabBarState) findOver(pos geom.Offset) {
	if s.group == nil || s.row == nil {
		s.setOver(nil, 0)
		return
	}
	inRow := core.GlobalOffset(s.run).Add(pos).Sub(core.GlobalOffset(s.row))
	b, local := s.group.barAt(s, inRow)
	if b == nil {
		s.setOver(nil, 0)
		return
	}
	s.setOver(b, b.insertionAt(local.X-s.drag.grab.X))
}

// insertionAt returns where a tab whose left edge is at x of the row goes in.
func (s *tabBarState) insertionAt(x float32) int {
	n := len(s.Widget().Tabs)
	scroll := float32(0)
	if s.run != nil {
		scroll = s.run.scroll
	}
	tw := s.tabWidth()
	return min(max(int(geom.Floor32((x+scroll)/tw+0.5)), 0), n)
}

// release ends the press of a tab at pos of the strip.
func (s *tabBarState) release(pos geom.Offset) {
	d := s.drag
	s.drag = tabDrag{}
	w := s.Widget()
	if d.index >= len(w.Tabs) {
		return
	}
	switch d.phase {
	case dragDragging:
		// The tab goes on from where the pointer left it.
		if v := s.slides[d.key]; v != nil && !basic.ReduceMotionOf(s.Context()) {
			v.Jump(d.x / s.tabWidth())
		}
		s.moveTab(d.index, d.target)
	case dragDetached:
		over, at := d.over, d.overIndex
		dr := s.dropper()
		switch {
		case over != nil:
			over.setIncoming(nil)
			over.receive(d.key, at, s)
			if w.OnDetach != nil {
				w.OnDetach(d.index)
			}
		case dr != nil && s.run != nil && dr.tabDropped(d.key, core.GlobalOffset(s.run).Add(pos)):
		case w.OnTearOff != nil:
			w.OnTearOff(s.tearOff(&d, pos))
		}
	}
	if dr := s.dropper(); dr != nil && d.phase >= dragDragging {
		dr.tabDragEnded()
	}
	kit.Rebuild(&s.StateBase)
}

// receive takes the tab with key from the bar from, at index.
func (s *tabBarState) receive(key core.Key, index int, from *tabBarState) {
	w := s.Widget()
	w.OnReceive(key, index)
	if w.OnSelect != nil {
		w.OnSelect(index)
	}
	if s.row != nil && from.row != nil && !core.SameTree(s.row, from.row) {
		if h, ok := core.ScreenHostOf(s.row); ok {
			h.Activate()
		}
	}
}

// tearOff describes the drop of the tab of d away from every bar, at pos.
func (s *tabBarState) tearOff(d *tabDrag, pos geom.Offset) TabTearOff {
	e := TabTearOff{Index: d.index, Key: d.key, Alone: len(s.Widget().Tabs) == 1}
	origin, scale, ok := core.ScreenOf(s.run)
	if !ok {
		return e
	}
	e.Screen = origin.Add(pos.Scale(scale))
	// In the new window, the tab is the first one of a bar where this bar is.
	at := core.GlobalOffset(s.run).Add(d.grab)
	e.Origin = e.Screen.Sub(at.Scale(scale))
	e.Placed = true
	return e
}

// cancelDrag puts a dragged tab back.
func (s *tabBarState) cancelDrag() {
	d := &s.drag
	if d.phase == dragNone {
		return
	}
	if d.phase == dragDragging && s.run != nil {
		if v := s.slides[d.key]; v != nil && s.Mounted() && !basic.ReduceMotionOf(s.Context()) {
			v.Jump(d.x / s.tabWidth())
		}
	}
	s.setOver(nil, 0)
	if dr := s.dropper(); dr != nil && d.phase >= dragDragging {
		dr.tabDragEnded()
	}
	*d = tabDrag{}
	kit.Rebuild(&s.StateBase)
}

// pressButton reports whether a press of button b is one the strip takes.
func pressButton(b input.Button) bool { return b == input.ButtonPrimary || b == input.ButtonMiddle }
