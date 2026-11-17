package nav

import (
	"slices"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/geom"
)

// The shares of a hidden panel docked back at an edge whose group is gone.
const (
	dockMinShare = 0.1
	dockMaxShare = 0.6
)

// dockHidden is a panel hidden at an edge, as DockHidden.
type dockHidden struct {
	id     string
	side   DockZone
	size   float32
	origin []string
	share  float32
}

// dockFocusKind says where the keyboard goes at the next layout of a
// DockLayout: what it is to have does not exist before the build.
type dockFocusKind uint8

const (
	dockFocusNone dockFocusKind = iota
	// dockFocusFlyout is the panel that slid out.
	dockFocusFlyout
	// dockFocusStripTab is the tab of the hidden panel id on its strip.
	dockFocusStripTab
	// dockFocusPanel is the panel id in its group.
	dockFocusPanel
)

// dockFocus is where the keyboard goes at the next layout.
type dockFocus struct {
	kind dockFocusKind
	id   string
}

// Unpin hides the panel with id at side of the layout, DockLeft, DockRight,
// DockTop or DockBottom, as the pin button of its group does for all of them:
// a tab on the strip along that edge stands for it, and slides it out over the
// layout when pressed. Any other side, such as DockCenter, picks the edge
// nearest its group. A panel hidden already moves to side. The panel keeps its
// state, and Pin puts it back.
func (c *DockController) Unpin(id string, side DockZone) {
	if c.adopt(id) && c.hide([]string{id}, side) {
		c.refresh()
	}
}

// UnpinGroup hides every panel of the group that holds the panel id at side,
// as Unpin does. Pinning one of them puts them all back in one group.
func (c *DockController) UnpinGroup(id string, side DockZone) {
	if !c.adopt(id) {
		return
	}
	if g := c.groupOf(id); g != nil && c.hide(slices.Clone(g.panels), side) {
		c.refresh()
	}
}

// Pin docks the hidden panel with id back in the group it came from, with
// those hidden along with it; with that group gone, they dock along the edge
// they were hidden at. A panel that is not hidden stays as it is.
func (c *DockController) Pin(id string) {
	if c.pin(id) {
		c.refresh()
	}
}

// Show slides out the hidden panel with id over the layout and gives it the
// keyboard. A panel that is not hidden is left alone; Activate selects it.
func (c *DockController) Show(id string) {
	if c.show(id) {
		c.refresh()
	}
}

// HideFlyout hides the panel that slid out, if any.
func (c *DockController) HideFlyout() {
	if c.flyout != "" {
		c.flyout = ""
		c.refresh()
	}
}

// Flyout returns the ID of the hidden panel that slid out, or empty.
func (c *DockController) Flyout() string { return c.flyout }

// hiddenIndex returns the index of the hidden panel id, or -1.
func (c *DockController) hiddenIndex(id string) int {
	for i := range c.hidden {
		if c.hidden[i].id == id {
			return i
		}
	}
	return -1
}

// unhide forgets the hidden panel id, and reports whether it was hidden.
func (c *DockController) unhide(id string) bool {
	i := c.hiddenIndex(id)
	if i < 0 {
		return false
	}
	c.hidden = slices.Delete(c.hidden, i, i+1)
	if c.flyout == id {
		c.flyout = ""
	}
	return true
}

// show slides out the hidden panel id, and reports whether it is hidden.
func (c *DockController) show(id string) bool {
	if c.hiddenIndex(id) < 0 {
		return false
	}
	c.flyout = id
	c.focus = dockFocus{kind: dockFocusFlyout}
	return true
}

// canHide reports whether hiding the panel id at side changes anything.
func (c *DockController) canHide(id string, side DockZone) bool {
	if i := c.hiddenIndex(id); i >= 0 {
		return c.hidden[i].side != side
	}
	return c.groupOf(id) != nil
}

// hide hides the panels ids, which are all of one group, at side; one panel
// that is hidden already moves to side. It reports whether anything changed.
func (c *DockController) hide(ids []string, side DockZone) bool {
	if len(ids) == 0 {
		return false
	}
	if i := c.hiddenIndex(ids[0]); i >= 0 && len(ids) == 1 {
		if side.side() != side || c.hidden[i].side == side {
			return false
		}
		h := c.hidden[i]
		h.side = side
		c.hidden = append(slices.Delete(c.hidden, i, i+1), h)
		return true
	}
	g := c.groupOf(ids[0])
	if g == nil {
		return false
	}
	for _, id := range ids[1:] {
		if c.groupOf(id) != g {
			return false
		}
	}
	if side.side() != side {
		side = c.nearestSide(g)
	}
	size, share := c.lengthAcross(g, side)
	// Panels hidden together have equal origins, which is how Pin knows them.
	origin := slices.Clone(g.panels)
	for _, id := range ids {
		c.take(id)
		c.hidden = append(c.hidden, dockHidden{id: id, side: side, size: size, origin: slices.Clone(origin), share: share})
	}
	c.flyout = ""
	c.retidy()
	return true
}

// nearestSide returns the edge of the layout nearest the group g, by where it
// was at the last layout.
func (c *DockController) nearestSide(g *dockNode) DockZone {
	r, full := g.rect, c.root.rect
	side, best := DockLeft, r.X-full.X
	for _, e := range [...]struct {
		side DockZone
		d    float32
	}{
		{DockRight, full.X + full.W - (r.X + r.W)},
		{DockBottom, full.Y + full.H - (r.Y + r.H)},
		{DockTop, r.Y - full.Y},
	} {
		if e.d < best {
			side, best = e.side, e.d
		}
	}
	return side
}

// lengthAcross returns the length of the group g away from side, and its part
// of that of the layout, at the last layout.
func (c *DockController) lengthAcross(g *dockNode, side DockZone) (float32, float32) {
	size, total := g.rect.W, c.root.rect.W
	if side == DockTop || side == DockBottom {
		size, total = g.rect.H, c.root.rect.H
	}
	if total <= 0 || size <= 0 {
		return 0, 0
	}
	return size, size / total
}

// pin docks the hidden panel id, with those hidden along with it, back where
// they came from, and reports whether it was hidden.
func (c *DockController) pin(id string) bool {
	i := c.hiddenIndex(id)
	if i < 0 || c.root == nil {
		return false
	}
	h := c.hidden[i]
	// The panels hidden together, in the order of their group.
	set := []string{id}
	if slices.Contains(h.origin, id) {
		set = set[:0]
		for _, p := range h.origin {
			if j := c.hiddenIndex(p); j >= 0 && slices.Equal(c.hidden[j].origin, h.origin) {
				set = append(set, p)
			}
		}
	}
	for _, p := range set {
		c.unhide(p)
	}
	c.maximized = nil
	var g *dockNode
	for _, p := range h.origin {
		if g = c.groupOf(p); g != nil {
			break
		}
	}
	switch {
	case g != nil:
		for _, p := range set {
			g.panels = slices.Insert(g.panels, insertionIn(g, h.origin, p), p)
		}
	case c.root.isGroup() && len(c.root.panels) == 0:
		g = c.root
		g.panels = set
	default:
		g = c.newNode()
		g.panels = set
		share := h.share
		if share <= 0 {
			share = dockRootShare
		}
		c.dockAt(c.root, g, h.side, geom.Clamp32(share, dockMinShare, dockMaxShare))
	}
	g.active = id
	c.current = g
	c.focus = dockFocus{kind: dockFocusPanel, id: id}
	c.retidy()
	return true
}

// insertionIn returns where the panel p goes in g to keep the order of
// origin: after the nearest panel before it there, or before the nearest one
// after it, or at the end.
func insertionIn(g *dockNode, origin []string, p string) int {
	k := slices.Index(origin, p)
	for j := k - 1; j >= 0; j-- {
		if at := g.indexOf(origin[j]); at >= 0 {
			return at + 1
		}
	}
	for j := k + 1; j < len(origin); j++ {
		if at := g.indexOf(origin[j]); at >= 0 {
			return at
		}
	}
	return len(g.panels)
}

// hiddenEntry returns the hidden panel id, or nil.
func (c *DockController) hiddenEntry(id string) *dockHidden {
	if i := c.hiddenIndex(id); i >= 0 {
		return &c.hidden[i]
	}
	return nil
}

// The parts of the layout that hidden panels add.

// toggleFlyout slides out the hidden panel id, or hides it if it is out.
func (s *dockState) toggleFlyout(id string) {
	c := s.ctrl
	switch {
	case c.flyout == id:
		c.flyout = ""
	case !c.show(id):
		return
	}
	s.rebuild()
}

// hideFlyout hides the panel that slid out; with toStrip its tab gets the
// keyboard.
func (s *dockState) hideFlyout(toStrip bool) bool {
	c := s.ctrl
	if c.flyout == "" {
		return false
	}
	if toStrip {
		c.focus = dockFocus{kind: dockFocusStripTab, id: c.flyout}
	}
	c.flyout = ""
	s.rebuild()
	return true
}

// pinFlyout docks the panel id back, from the pin button of its flyout.
func (s *dockState) pinFlyout(id string) {
	if s.ctrl.pin(id) {
		s.rebuild()
		s.userChanged()
	}
}

// unpinGroup hides the group gid at its nearest edge, from the pin button of
// its strip.
func (s *dockState) unpinGroup(gid int) {
	c := s.ctrl
	g := c.groupByID(gid)
	if g == nil || len(g.panels) == 0 {
		return
	}
	active := g.active
	if c.hide(slices.Clone(g.panels), DockCenter) {
		c.focus = dockFocus{kind: dockFocusStripTab, id: active}
		s.rebuild()
		s.userChanged()
	}
}

// unpinCurrent hides the selected panel of the current group, or the whole
// group, at its nearest edge, from the keyboard.
func (s *dockState) unpinCurrent(group bool) bool {
	c := s.ctrl
	g := c.currentGroup()
	id := g.active
	if id == "" {
		return false
	}
	ids := []string{id}
	if group {
		ids = slices.Clone(g.panels)
	}
	if !c.hide(ids, DockCenter) {
		return false
	}
	c.focus = dockFocus{kind: dockFocusStripTab, id: id}
	s.rebuild()
	s.userChanged()
	return true
}

// closeFlyout closes the panel that slid out, if it is closable.
func (s *dockState) closeFlyout() {
	c := s.ctrl
	id := c.flyout
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

// flyoutFocus follows the focus in and out of the panel that slid out: it
// hides once the focus goes anywhere but the flyout and the strips.
func (s *dockState) flyoutFocus(within bool) {
	s.flyoutWithin = within
	if !within {
		s.keepFlyoutNear()
	}
}

// keepFlyoutNear hides the panel that slid out unless the focus is near it.
// It runs when the focus leaves the flyout, and when it leaves a menu or a
// dialog opened from within the layout, which the focus may have gone into
// from the flyout.
func (s *dockState) keepFlyoutNear() {
	if s.ctrl.flyout != "" && !s.focusStaysNear() {
		s.hideFlyout(false)
	}
}

// focusStaysNear reports whether the focus is in a part of the layout that
// keeps the flyout out: the flyout itself or a strip of hidden panels, or a
// menu, a popup or a dialog opened from one of them.
func (s *dockState) focusStaysNear() bool {
	r := s.view
	if r == nil {
		return false
	}
	f := kit.FocusFrom(r)
	if f == nil {
		return false
	}
	for _, child := range r.children {
		if sl, ok := child.(*renderDockSlot); ok && sl.keep && overlay.Contains(sl, f) {
			return true
		}
	}
	return false
}

// resizeFlyout gives the panel that slid out the size length, within the
// bounds of the last layout. It only lays out again, and does not allocate.
func (s *dockState) resizeFlyout(length float32) {
	r := s.view
	h := s.ctrl.hiddenEntry(s.ctrl.flyout)
	if r == nil || h == nil {
		return
	}
	length = geom.Clamp32(length, r.flyLo, r.flyHi)
	if length == h.size {
		return
	}
	h.size = length
	s.relayout()
}

// The drag of a tab off a strip of hidden panels.

// stripDragStart starts the drag of the tab of the hidden panel id.
func (s *dockState) stripDragStart(id string) {
	s.stripDrag = id
	if s.ctrl.flyout != "" {
		s.hideFlyout(false)
	}
}

// stripDragged follows the drag of the tab of the hidden panel id at global.
func (s *dockState) stripDragged(id string, global geom.Offset) {
	if s.stripDrag != id || s.view == nil {
		return
	}
	d := s.dropAt(id, global)
	s.view.setPreview(d.preview, d.ok)
}

// stripDropped ends the drag of the tab of the hidden panel id at global.
func (s *dockState) stripDropped(id string, global geom.Offset) {
	live := s.stripDrag == id
	s.stripDragEnded()
	if !live {
		return
	}
	if d := s.dropAt(id, global); d.ok {
		s.applyDrop(d)
	}
}

// stripDragEnded ends any drag off a strip. It only paints.
func (s *dockState) stripDragEnded() {
	s.stripDrag = ""
	if r := s.view; r != nil {
		r.setPreview(geom.Rect{}, false)
	}
}

// applyDrop moves the panel of d where it was let go.
func (s *dockState) applyDrop(d dockDrop) {
	c := s.ctrl
	if d.hide {
		c.hide([]string{d.id}, d.side)
	} else {
		c.move(d.id, d.group, d.zone, -1)
	}
	s.rebuild()
	s.userChanged()
}
