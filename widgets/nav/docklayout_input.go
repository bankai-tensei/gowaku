package nav

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
)

// The callbacks of the strips. Each finds its group by the ID of its node,
// since the strip may have been built before the tree last changed.

// selectAt selects panel i of the group gid.
func (s *dockState) selectAt(gid, i int) {
	c := s.ctrl
	g := c.groupByID(gid)
	if g == nil || i < 0 || i >= len(g.panels) {
		return
	}
	c.current = g
	if g.active != g.panels[i] {
		g.active = g.panels[i]
		s.rebuild()
		s.userChanged()
	}
}

// closeAt closes panel i of the group gid, if it is closable.
func (s *dockState) closeAt(gid, i int) {
	c := s.ctrl
	g := c.groupByID(gid)
	if g == nil || i < 0 || i >= len(g.panels) {
		return
	}
	id := g.panels[i]
	if !s.mayClose(id) || !c.close(id) {
		return
	}
	delete(s.mounted, id)
	s.rebuild()
	if f := s.Widget().OnClose; f != nil {
		f(id)
	}
	s.userChanged()
}

// mayClose reports whether the user may close the panel id: it is closable,
// and OnCloseRequest lets it go.
func (s *dockState) mayClose(id string) bool {
	p := s.panels[id]
	if p == nil || !p.Closable {
		return false
	}
	ask := s.Widget().OnCloseRequest
	return ask == nil || ask(id)
}

// reorderIn moves the panel at from to the index to within the group gid.
func (s *dockState) reorderIn(gid, from, to int) {
	if g := s.ctrl.groupByID(gid); g != nil {
		s.ctrl.reorder(g, from, to)
		s.rebuild()
		s.userChanged()
	}
}

// receive moves the panel with key to the group gid at index: its tab was
// dropped on the strip of the group.
func (s *dockState) receive(gid int, key core.Key, index int) {
	c := s.ctrl
	id, _ := key.(string)
	g := c.groupByID(gid)
	if g == nil || c.groupOf(id) == nil {
		return
	}
	c.move(id, g, DockCenter, index)
	s.rebuild()
	s.userChanged()
}

// toggleMaximize makes the group gid fill the layout, or brings the others
// back.
func (s *dockState) toggleMaximize(gid int) {
	c := s.ctrl
	g := c.groupByID(gid)
	switch {
	case g == nil:
		return
	case c.maximized == g:
		c.maximized = nil
	case c.root != g:
		c.maximized = g
	default:
		return
	}
	c.current = g
	s.rebuild()
	s.userChanged()
}

// focusIn and focusGroup follow the focus into a pane or a strip: that group
// becomes the current one.
func (s *dockState) focusIn(id string, within bool) {
	if g := s.ctrl.groupOf(id); within && g != nil {
		s.ctrl.current = g
	}
}

func (s *dockState) focusGroup(gid int, within bool) {
	if g := s.ctrl.groupByID(gid); within && g != nil {
		s.ctrl.current = g
	}
}

// resize gives child i of split the length first, within the smallest sizes
// of it and the next child, whose lengths were grip; the two share their room.
// It only lays out again, and does not allocate.
func (s *dockState) resize(split *dockNode, i int, grip [2]float32, first float32) {
	if i+1 >= len(split.weights) {
		return
	}
	total := grip[0] + grip[1]
	if total <= 0 {
		return
	}
	lo := min(split.children[i].minAlong(split.axis), total/2)
	hi := max(total-min(split.children[i+1].minAlong(split.axis), total/2), lo)
	first = geom.Clamp32(first, lo, hi)
	sum := split.weights[i] + split.weights[i+1]
	a := sum * first / total
	if a == split.weights[i] {
		return
	}
	split.weights[i], split.weights[i+1] = a, sum-a
	s.relayout()
}

// The drag of a tab off the strips. The strips find each other; the layout
// takes a tab that is over no strip.

// dockDrop is where a dragged tab would go.
type dockDrop struct {
	id string
	// group is the group the tab goes to or beside, or nil for the edge of
	// the whole layout.
	group *dockNode
	zone  DockZone
	// hide hides the panel at side instead, on the strip there.
	hide bool
	side DockZone
	// preview is where the panel would show, in the layout.
	preview geom.Rect
	ok      bool
}

// tabDragged implements tabDropper.
func (s *dockState) tabDragged(key core.Key, global geom.Offset, free bool) {
	r := s.view
	if r == nil || !r.Attached() {
		return
	}
	if !free {
		r.setPreview(geom.Rect{}, false)
		return
	}
	d := s.dropAt(key, global)
	r.setPreview(d.preview, d.ok)
}

// tabDropped implements tabDropper.
func (s *dockState) tabDropped(key core.Key, global geom.Offset) bool {
	d := s.dropAt(key, global)
	s.tabDragEnded()
	if !d.ok {
		return false
	}
	s.applyDrop(d)
	return true
}

// tabDragEnded implements tabDropper. It only paints, so that it is safe
// while the strip that was dragged from goes away during a build.
func (s *dockState) tabDragEnded() {
	if r := s.view; r != nil {
		r.setPreview(geom.Rect{}, false)
	}
}

// dropAt returns where the tab with key goes if it is let go at global, in
// the window. Only this layout counts: a tab never leaves the layout it
// belongs to, also where another layout lies within or around it.
func (s *dockState) dropAt(key core.Key, global geom.Offset) dockDrop {
	r, c := s.view, s.ctrl
	id, _ := key.(string)
	if r == nil || !r.Attached() || c.root == nil || (c.groupOf(id) == nil && c.hiddenIndex(id) < 0) {
		return dockDrop{}
	}
	p := global.Sub(core.GlobalOffset(r))
	sz := r.Size()
	full := geom.Rect{W: sz.W, H: sz.H}
	if !full.Contains(p) {
		return dockDrop{}
	}
	d := dockDrop{id: id}
	if side, rect, ok := r.stripAt(p); ok {
		if c.canHide(id, side) {
			d.hide, d.side, d.preview, d.ok = true, side, sideOf(rect, DockCenter, 1), true
		}
		return d
	}
	inner := r.inner
	if c.maximized == nil {
		if z := edgeZone(inner, p); z != DockCenter && c.canMove(id, nil, z) {
			d.zone, d.preview, d.ok = z, sideOf(inner, z, dockRootShare), true
			return d
		}
	}
	g := s.groupAt(p)
	if g == nil {
		return dockDrop{}
	}
	z := DockCenter
	if body := g.body(); p.Y >= body.Y {
		z = zoneIn(body, p)
	}
	if !c.canMove(id, g, z) {
		return dockDrop{}
	}
	d.group, d.zone, d.ok = g, z, true
	d.preview = sideOf(g.rect, z, dockGroupShare)
	return d
}

// groupAt returns the group shown at p of the layout, or nil.
func (s *dockState) groupAt(p geom.Offset) *dockNode {
	c := s.ctrl
	if m := c.maximized; m != nil {
		if m.rect.Contains(p) {
			return m
		}
		return nil
	}
	var out *dockNode
	c.root.eachGroup(func(g *dockNode) {
		if out == nil && g.rect.Contains(p) {
			out = g
		}
	})
	return out
}

// edgeZone returns the edge of r within dockEdgeBand of p, or DockCenter.
func edgeZone(r geom.Rect, p geom.Offset) DockZone {
	z, best := DockCenter, float32(dockEdgeBand)
	for _, e := range [...]struct {
		zone DockZone
		d    float32
	}{
		{DockLeft, p.X - r.X}, {DockRight, r.X + r.W - p.X},
		{DockTop, p.Y - r.Y}, {DockBottom, r.Y + r.H - p.Y},
	} {
		if e.d < best {
			z, best = e.zone, e.d
		}
	}
	return z
}

// zoneIn returns the zone of r that p is in: the side it is nearest to, as a
// share of the size of r, within dockEdgeFraction, or DockCenter.
func zoneIn(r geom.Rect, p geom.Offset) DockZone {
	if r.W <= 0 || r.H <= 0 {
		return DockCenter
	}
	fx, fy := (p.X-r.X)/r.W, (p.Y-r.Y)/r.H
	z, best := DockCenter, float32(dockEdgeFraction)
	for _, e := range [...]struct {
		zone DockZone
		d    float32
	}{
		{DockLeft, fx}, {DockRight, 1 - fx}, {DockTop, fy}, {DockBottom, 1 - fy},
	} {
		if e.d < best {
			z, best = e.zone, e.d
		}
	}
	return z
}

// sideOf returns the part of r that a panel docked at zone takes, with share
// of it, inset a little.
func sideOf(r geom.Rect, zone DockZone, share float32) geom.Rect {
	switch zone {
	case DockLeft:
		r.W *= share
	case DockRight:
		r.X += r.W * (1 - share)
		r.W *= share
	case DockTop:
		r.H *= share
	case DockBottom:
		r.Y += r.H * (1 - share)
		r.H *= share
	}
	return geom.Rect{X: r.X + dockPreviewInset, Y: r.Y + dockPreviewInset, W: max(r.W-2*dockPreviewInset, 0), H: max(r.H-2*dockPreviewInset, 0)}
}

// Keys.

// arrowZone returns the side an arrow key points to.
func arrowZone(k input.KeyCode) (DockZone, bool) {
	switch k {
	case input.KeyLeft:
		return DockLeft, true
	case input.KeyRight:
		return DockRight, true
	case input.KeyUp:
		return DockTop, true
	case input.KeyDown:
		return DockBottom, true
	}
	return DockCenter, false
}

// shortcutWithin takes the keys that reach the layout from the focus within
// it, or, for the outermost layout, from anywhere. A layout within another
// takes keys only while the focus is in it, so that one press never acts on
// two layouts; the keys it has no use for go on to the layout around it.
func (s *dockState) shortcutWithin(e *core.KeyEvent) bool {
	if s.nested() && !s.within {
		return false
	}
	return s.shortcut(e)
}

// shortcutElsewhere takes the keys while the focus is outside the layout,
// which only the outermost one does.
func (s *dockState) shortcutElsewhere(e *core.KeyEvent) bool {
	if s.nested() {
		return false
	}
	return s.shortcut(e)
}

// nested reports whether the layout lies within another DockLayout.
func (s *dockState) nested() bool { return s.view != nil && s.view.nested() }

// shortcut takes the keys of the layout, and those of a TabBar for the
// current group when the focus is in one of its panels.
func (s *dockState) shortcut(e *core.KeyEvent) bool {
	c := s.ctrl
	if c.root == nil {
		return false
	}
	const ctrlAlt = input.ModControl | input.ModAlt
	zone, arrow := arrowZone(e.Key)
	switch {
	case e.Key == input.KeyEscape && e.Modifiers == 0:
		if s.stripDrag != "" {
			s.stripDragEnded()
			return true
		}
		return s.hideFlyout(s.flyoutWithin)
	case arrow && e.Modifiers == ctrlAlt:
		return s.focusToward(zone)
	case arrow && e.Modifiers == ctrlAlt|input.ModShift:
		return s.moveToward(zone)
	case e.Key == input.KeyEnter && e.Modifiers == ctrlAlt:
		g := c.currentGroup()
		if c.maximized != g && c.root == g {
			// Nothing to maximize: a layout around this one may.
			return false
		}
		if !e.Repeat {
			s.toggleMaximize(g.id)
		}
		return true
	case e.Key == input.KeyPageDown && (e.Modifiers == ctrlAlt || e.Modifiers == ctrlAlt|input.ModShift):
		if s.inFlyout() {
			return s.hideFlyout(true)
		}
		return s.Widget().AutoHide && s.unpinCurrent(e.Modifiers&input.ModShift != 0)
	case e.Key == input.KeyPageUp && e.Modifiers == ctrlAlt:
		if c.flyout == "" {
			return false
		}
		s.pinFlyout(c.flyout)
		return true
	}
	if s.inFlyout() {
		return s.flyoutShortcut(e)
	}
	return s.groupShortcut(e)
}

// inFlyout reports whether the focus is in the panel that slid out.
func (s *dockState) inFlyout() bool { return s.flyoutWithin && s.ctrl.flyout != "" }

// flyoutShortcut takes the keys of a TabBar while the focus is in the panel
// that slid out: Ctrl+W and Ctrl+F4 close it, and the others, which have no
// tabs to go to, do nothing.
func (s *dockState) flyoutShortcut(e *core.KeyEvent) bool {
	if e.Modifiers&input.ModControl == 0 || e.Modifiers&input.ModAlt != 0 {
		return false
	}
	switch e.Key {
	case input.KeyW, input.KeyF4:
		if e.Modifiers&input.ModShift != 0 {
			return false
		}
		if !e.Repeat {
			s.closeFlyout()
		}
	case input.KeyTab, input.KeyPageUp, input.KeyPageDown:
	default:
		return false
	}
	return true
}

// groupShortcut takes the window-wide keys of a TabBar for the current group:
// Ctrl+Tab, Ctrl+PageUp and Ctrl+PageDown select, with Shift and a page key
// move, and Ctrl+W and Ctrl+F4 close.
func (s *dockState) groupShortcut(e *core.KeyEvent) bool {
	g := s.ctrl.currentGroup()
	n := len(g.panels)
	if n == 0 || e.Modifiers&input.ModControl == 0 || e.Modifiers&input.ModAlt != 0 {
		return false
	}
	i := g.indexOf(g.active)
	shift := e.Modifiers&input.ModShift != 0
	switch {
	case e.Key == input.KeyTab && shift, e.Key == input.KeyPageUp && !shift:
		s.selectAt(g.id, (i-1+n)%n)
	case e.Key == input.KeyTab, e.Key == input.KeyPageDown && !shift:
		s.selectAt(g.id, (i+1)%n)
	case e.Key == input.KeyPageUp, e.Key == input.KeyPageDown:
		to := i - 1
		if e.Key == input.KeyPageDown {
			to = i + 1
		}
		if to >= 0 && to < n {
			s.reorderIn(g.id, i, to)
		}
	case (e.Key == input.KeyW || e.Key == input.KeyF4) && !shift:
		if !e.Repeat {
			s.closeAt(g.id, i)
		}
	default:
		return false
	}
	return true
}

// focusToward gives the keyboard to the group on the side zone names of the
// current one.
func (s *dockState) focusToward(zone DockZone) bool {
	c := s.ctrl
	if c.maximized != nil {
		return false
	}
	n := c.neighbor(c.currentGroup(), zone)
	if n == nil {
		return false
	}
	c.current = n
	s.focusGroupContent(n)
	return true
}

// focusGroupContent gives the keyboard to the first control of the panel g
// shows, or to its strip.
func (s *dockState) focusGroupContent(g *dockNode) {
	if r := s.view; r != nil {
		if f := r.groupFocus(g); f != nil {
			core.RestoreFocus(f)
		}
	}
}

// moveToward moves the selected panel of the current group to the group on
// the side zone names, or docks it on that side of its group when there is
// none.
func (s *dockState) moveToward(zone DockZone) bool {
	c := s.ctrl
	g := c.currentGroup()
	id := g.active
	if id == "" {
		return false
	}
	var n *dockNode
	if c.maximized == nil {
		n = c.neighbor(g, zone)
	}
	switch {
	case n != nil:
		c.move(id, n, DockCenter, -1)
	case c.canMove(id, g, zone):
		c.move(id, g, zone, -1)
	default:
		return true
	}
	s.rebuild()
	s.userChanged()
	return true
}
