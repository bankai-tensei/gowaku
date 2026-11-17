package nav

import (
	"fmt"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// The measures of a DockLayout.
const (
	dividerThickness = kit.DividerThickness
	// dockMinGroupWidth and dockMinGroupHeight are the smallest a group gets
	// when a divider is dragged.
	dockMinGroupWidth  = 96
	dockMinGroupHeight = tabBarHeight + 40
	// dockKeyStep is how far an arrow key moves a divider.
	dockKeyStep = theme.Space10
	// dockEdgeBand is how near the edge of the whole layout a dragged tab
	// docks at that edge.
	dockEdgeBand = 24
	// dockEdgeFraction is how near the edge of a group, as a share of its
	// size, a dragged tab docks at that edge instead of joining the group.
	dockEdgeFraction = 0.25
	// dockPreviewRadius and dockPreviewInset shape the preview of a drop.
	dockPreviewRadius = 6
	dockPreviewInset  = 2
	// dockPreviewFill and dockPreviewEdge are the strengths of the accent in
	// the preview.
	dockPreviewFill = 0.14
	dockPreviewEdge = 0.6
	// dockStripThickness is that of a strip of hidden panels, and
	// dockStripTabBreadth the breadth of a tab on it.
	dockStripThickness  = theme.Space28
	dockStripTabBreadth = theme.Space24
	// dockFlyoutMargin is the least of the layout that a flyout leaves
	// uncovered, and dockFlyoutShare the part of it that one takes at first.
	dockFlyoutMargin = 48
	dockFlyoutShare  = 0.3
	// dockFlyoutDuration is how long a hidden panel takes to slide out, and
	// dockFlyoutBack how long it takes to slide back in, which is quicker.
	dockFlyoutDuration = 200 * time.Millisecond
	dockFlyoutBack     = 150 * time.Millisecond
)

// DockPanel is one panel of a DockLayout.
type DockPanel struct {
	// ID tells the panel apart, in the layout and in a saved
	// DockArrangement. It must not be empty, and must be unique among the
	// panels of the layout.
	ID    string
	Title string
	// Icon is drawn before the title on its tab, on the 24 unit grid of Icon.
	Icon *paint.Path
	// Build returns what the panel shows. Nil shows nothing. ctx is that of
	// the place of the panel, so that a menu or a dialog opened with it
	// belongs to the panel: while the panel slides out, the focus going into
	// one does not hide it (see overlay.Contains).
	Build func(ctx core.BuildContext) core.Widget
	// Closable gives the tab of the panel a close button.
	Closable bool
	// Italic slants the title on the tab of the panel, as TabItem.Italic
	// does, such as for a preview that the next document opened takes over.
	Italic bool
}

// DockLayout arranges panels as an IDE does: groups of tabs side by side and
// one above the other, which the user resizes by dragging the dividers between
// them, and rearranges by dragging tabs. It takes all the room it is given,
// which must be bounded.
//
// A tab dragged along its strip moves within its group, and dropped on the
// strip of another group it joins that group. Dropped on the content of a
// group, it joins it in the middle, and docks beside it near an edge, where
// it splits the room of the group; near an edge of the whole layout it docks
// along that edge. A translucent preview shows where the panel would go, and
// Escape cancels the drag. A group whose last panel leaves goes, and its
// neighbors take its room. A double click on the empty part of a strip makes
// the group fill the layout, and another brings the others back.
//
// The panels keep their state as they move between groups: what a panel
// builds stays in the tree from the first time it shows until it closes, out
// of sight while its tab is not selected.
//
// From the keyboard, Ctrl+Alt with an arrow moves the focus to the group on
// that side, Ctrl+Alt+Shift with an arrow moves the selected panel there, or
// docks it on that side when there is no group, and Ctrl+Alt+Enter maximizes
// the group with the focus and restores it. Each strip and divider takes the
// keys of a TabBar and of the divider of a SplitPane. A screen reader finds
// each group as a tab list, each panel as a group named by its title, and each
// divider as a separator whose value is the share of the first side.
//
// A panel can also hide at an edge of the layout: a tab on a strip along that
// edge stands for it, and pressing the tab slides the panel out over the
// layout, at a size the user drags on its inner edge, until Escape, a press or
// the focus elsewhere, or the tab again hides it. The pin button in its header
// docks it back in the group it came from. With AutoHide, the strip of each
// group has a pin button that hides the group, and Ctrl+Alt+PageDown hides the
// selected panel, with Shift the whole group; Ctrl+Alt+PageUp docks back the
// panel that slid out. Tabs drag onto a strip to hide there, and off it to dock.
//
// A panel may hold another DockLayout. Each has its own arrangement, and tabs
// stay within the layout they belong to. The keys act on the innermost layout
// that holds the focus, and those it has no use for, such as Ctrl+Alt with an
// arrow at its edge, go on to the layout around it. See
// docs/ADR/0102-dock-autohide-nesting.md.
//
// The arrangement lives in Controller, which saves and restores it as a
// DockArrangement. The layout does no file I/O itself.
type DockLayout struct {
	core.StatefulBase
	// Panels are those the layout can show. Panels that leave the list leave
	// the layout; new ones go to the current group.
	Panels []DockPanel
	// Controller holds the arrangement. Nil keeps one in the layout, which
	// starts from Default.
	Controller *DockController
	// Default is where the panels start, and where DockController.Reset
	// puts them. The zero value puts them all in one group, in order.
	Default DockArrangement
	// Label names the tab list of each group to assistive technology.
	Label string
	// OnChange reports a change of the arrangement that the user made, as
	// moving, closing or resizing, with the arrangement after it, to save.
	OnChange func(a DockArrangement)
	// OnCloseRequest asks whether the user may close the panel with id,
	// before it closes: by the close button of its tab or of its flyout, the
	// middle button on its tab, or Ctrl+W or Ctrl+F4. False keeps it open,
	// as when it holds unsaved changes: open a dialog, return false, and
	// call DockController.Close once the user agrees. It runs in the
	// callback of the press or the key, and must not block. Nil lets every
	// closable panel close.
	OnCloseRequest func(id string) bool
	// OnClose reports that the user closed the panel with id. It follows
	// the closes that OnCloseRequest let through, but not DockController.Close,
	// after which the caller does what OnClose would and saves the
	// arrangement itself.
	OnClose func(id string)
	// AutoHide lets the user hide panels at the edges: the strip of each group
	// gets a pin button, and Ctrl+Alt+PageDown hides. Without it, panels hide
	// only from DockController.Unpin or a restored arrangement, and the user
	// can still show them and pin them back.
	AutoHide bool
}

type dockState struct {
	core.StateOf[DockLayout]
	// own is the controller without DockLayout.Controller, and ctrl the one
	// in use.
	own  DockController
	ctrl *DockController
	// tabs joins the strips, so that tabs move between them, and lets the
	// layout follow a tab dragged off them.
	tabs TabGroup
	// mounted are the panels that showed, whose content stays in the tree.
	mounted map[string]bool
	// panels are those of the last build, by ID.
	panels map[string]*DockPanel
	view   *renderDock
	// within says the focus is in the layout, and flyoutWithin in the panel
	// that slid out.
	within, flyoutWithin bool
	// slide moves the panel that slid out, from 0 hidden to 1 out, and shown
	// is the one it moves. leaving is a panel that hid and still slides back
	// in: it is drawn until slide comes to 0, but takes neither the pointer,
	// the keyboard nor assistive technology.
	slide          motion.AnimationController
	shown, leaving string
	// stripDrag is the hidden panel whose tab is dragged off its strip, or
	// empty.
	stripDrag string
	// pinned and unpinned are the glyphs of the pin buttons.
	pinned, unpinned, cross *paint.Path
}

// CreateState implements core.StatefulWidget.
func (DockLayout) CreateState() core.State { return &dockState{} }

// InitState implements core.State.
func (s *dockState) InitState() {
	s.mounted = make(map[string]bool)
	s.tabs.dropper = s
	s.attach(s.Widget().Controller)
	s.slide.Attach(&s.StateBase)
	s.slide.Duration, s.slide.ReverseDuration, s.slide.Curve = dockFlyoutDuration, dockFlyoutBack, motion.Standard
	s.slide.OnChange = s.relayout
	s.slide.OnStatus = s.slideStatus
	s.slide.SetValue(1)
	s.pinned, s.unpinned = pinPath(false), pinPath(true)
	s.cross = kit.CrossPath(tabCloseSize, theme.Space8, 1.5)
}

// DidUpdateWidget implements core.State.
func (s *dockState) DidUpdateWidget(old core.Widget) {
	if w := s.Widget(); w.Controller != old.(DockLayout).Controller {
		s.attach(w.Controller)
	}
}

// Dispose implements core.State.
func (s *dockState) Dispose() {
	if s.ctrl != nil && s.ctrl.view == s {
		s.ctrl.view = nil
	}
	s.tabs.dropper = nil
	s.slide.Dispose()
}

// attach shows the controller c, or the layout's own for nil.
func (s *dockState) attach(c *DockController) {
	if c == nil {
		c = &s.own
	}
	if s.ctrl != nil && s.ctrl.view == s {
		s.ctrl.view = nil
	}
	s.ctrl = c
	c.view = s
}

// rebuild shows a change of the arrangement.
func (s *dockState) rebuild() { kit.Rebuild(&s.StateBase) }

// relayout shows a change of the weights, which takes no build.
func (s *dockState) relayout() {
	if s.view != nil {
		s.view.MarkNeedsLayout()
	}
}

// userChanged reports a change the user made.
func (s *dockState) userChanged() {
	if f := s.Widget().OnChange; f != nil {
		f(s.ctrl.Arrangement())
	}
}

// Build implements core.State.
func (s *dockState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	c := s.ctrl
	c.view = s
	ids := make([]string, len(w.Panels))
	s.panels = make(map[string]*DockPanel, len(w.Panels))
	for i := range w.Panels {
		p := &w.Panels[i]
		if p.ID == "" {
			panic("gowaku: DockLayout has a panel without an ID; give every DockPanel a unique ID")
		}
		if s.panels[p.ID] != nil {
			panic(fmt.Sprintf("gowaku: DockLayout has two panels with the ID %q; panel IDs must be unique", p.ID))
		}
		s.panels[p.ID] = p
		ids[i] = p.ID
	}
	c.sync(ids, &w.Default)
	s.startSlide(ctx)
	maxed := c.maximized
	th := basic.ThemeOf(ctx)
	col := &th.Color
	l := basic.StringsOf(ctx)

	var slots []dockSlot
	var children []core.Widget
	for i := range w.Panels {
		p := &w.Panels[i]
		g := c.groupOf(p.ID)
		if g == nil {
			if c.hiddenIndex(p.ID) < 0 {
				delete(s.mounted, p.ID)
			}
			continue
		}
		shown := g.active == p.ID && (maxed == nil || maxed == g)
		if shown {
			s.mounted[p.ID] = true
		}
		if !s.mounted[p.ID] {
			continue
		}
		id := p.ID
		children = append(children, core.Keyed(dockPanelKey(id), dockSlotView{
			hidden:  !shown,
			onFocus: func(within bool) { s.focusIn(id, within) },
			child:   paneContent(p),
		}))
		slots = append(slots, dockSlot{kind: dockSlotPane, node: g, panel: id})
	}
	for id := range s.mounted {
		if s.panels[id] == nil {
			delete(s.mounted, id)
		}
	}
	c.root.eachGroup(func(g *dockNode) {
		children = append(children, core.Keyed(dockGroupKey(g.id), s.strip(th, l, g, maxed)))
		slots = append(slots, dockSlot{kind: dockSlotStrip, node: g})
	})
	c.root.eachSplit(func(split *dockNode) {
		for i := range len(split.children) - 1 {
			children = append(children, core.Keyed(dockDividerKey{split.id, i}, dockSlotView{
				hidden: maxed != nil,
				child:  dockDivider{dock: s, split: split, index: i, label: l.PaneDivider},
			}))
			slots = append(slots, dockSlot{kind: dockSlotDivider, node: split, index: i})
		}
	})
	// The strips of the hidden panels, and over everything the one that slid
	// out, which keeps its element, and so its state, as it hides and docks.
	for side := DockLeft; side <= DockBottom; side++ {
		if strip := s.sideStrip(th, l.HiddenPanels, side); strip != nil {
			children = append(children, core.Keyed(dockSideKey(side), strip))
			slots = append(slots, dockSlot{kind: dockSlotSide, index: int(side)})
		}
	}
	// fly is the panel drawn over the layout: the one out, or the one
	// sliding back in, which is left out of the input.
	fly, leaving := c.flyout, false
	if fly == "" && s.leaving != "" {
		fly, leaving = s.leaving, true
	}
	if fly != "" {
		chrome := s.flyoutChrome(th, l, fly)
		chrome.leaving = leaving
		children = append(children, core.Keyed(dockFlyoutKey{}, chrome))
		slots = append(slots, dockSlot{kind: dockSlotFlyout, panel: fly})
	}
	for _, h := range c.hidden {
		shown := h.id == fly
		if shown {
			s.mounted[h.id] = true
		}
		if !s.mounted[h.id] {
			continue
		}
		children = append(children, core.Keyed(dockPanelKey(h.id), dockSlotView{
			hidden: !shown, leaving: shown && leaving, keep: true, catch: true, onFocus: s.flyoutFocus,
			child: paneContent(s.panels[h.id]),
		}))
		slots = append(slots, dockSlot{kind: dockSlotFlyoutPane, panel: h.id})
	}
	if fly != "" {
		side := c.hiddenEntry(fly).side
		children = append(children, core.Keyed(dockFlyoutHandleKey{}, dockSlotView{
			leaving: leaving, keep: true, child: dockFlyoutHandle{dock: s, side: side, label: l.PaneDivider},
		}))
		slots = append(slots, dockSlot{kind: dockSlotFlyoutHandle})
	}
	children = append(children, core.Keyed(dockKeysKey{}, dockKeys{onShortcut: s.shortcutElsewhere}))
	slots = append(slots, dockSlot{kind: dockSlotKeys})
	// The keys of the layout come first while the focus is in one of its
	// panes, and before those of the strips while it is elsewhere.
	return dockKeys{onShortcut: s.shortcutWithin, onFocus: s.focusWithin, child: dockView{
		ctrl: c, fly: fly, ref: &s.view, slots: slots, children: children, ground: col.Card, canvas: col.Canvas,
		fill: col.AccentInk.WithAlpha(dockPreviewFill), edge: col.AccentInk.WithAlpha(dockPreviewEdge),
		slide: &s.slide, onOutside: s.outsidePress, onEntryLeft: s.keepFlyoutNear,
	}}
}

// paneContent returns what the panel p shows, as a group named by its title.
func paneContent(p *DockPanel) core.Widget {
	return interact.Semantics{Role: semantics.RoleGroup, Label: p.Title, Child: basic.Builder{Content: p.Build}}
}

// startSlide slides out a panel that has just come out, and back in one
// that has just hidden, unless motion is reduced: then the one goes at once.
// A panel shown again while it slides back in turns round where it is;
// another one takes its place at once and slides out from the edge.
func (s *dockState) startSlide(ctx core.BuildContext) {
	c := s.ctrl
	fly, was := c.flyout, s.leaving
	if was != "" && (fly != "" || !s.canLeave(was)) {
		// Another panel came out, or the one sliding in docked or closed.
		s.leaving = ""
	}
	if fly == s.shown {
		return
	}
	prev := s.shown
	s.shown = fly
	reduce := basic.ReduceMotionOf(ctx)
	switch {
	case fly == "":
		if !reduce && prev != "" && s.canLeave(prev) && s.slide.Value() > 0 {
			s.leaving = prev
			s.slide.Reverse()
		}
	case reduce:
		s.slide.SetValue(1)
	case fly == was:
		s.slide.Forward()
	default:
		s.slide.SetValue(0)
		s.slide.Forward()
	}
}

// canLeave reports whether the panel id can slide back in: it is still a
// hidden panel of the layout.
func (s *dockState) canLeave(id string) bool {
	return s.panels[id] != nil && s.ctrl.hiddenIndex(id) >= 0
}

// slideStatus takes the panel that slid back in out of the layout once it is
// in.
func (s *dockState) slideStatus(st motion.Status) {
	if st == motion.Dismissed && s.leaving != "" {
		s.leaving = ""
		s.rebuild()
	}
}

// focusWithin follows the focus in and out of the layout.
func (s *dockState) focusWithin(within bool) { s.within = within }

// outsidePress hides the panel that slid out, at a press outside it and the
// strips.
func (s *dockState) outsidePress() { s.hideFlyout(false) }

// strip returns the tab strip of the group g, with the pin button that hides
// it with AutoHide.
func (s *dockState) strip(th *theme.Theme, l *basic.Strings, g, maxed *dockNode) core.Widget {
	w := s.Widget()
	items := make([]TabItem, len(g.panels))
	for i, id := range g.panels {
		p := s.panels[id]
		items[i] = TabItem{Key: id, Label: p.Title, Icon: p.Icon, Permanent: !p.Closable, Italic: p.Italic}
	}
	gid := g.id
	var bar core.Widget = TabBar{
		Tabs: items, Selected: g.indexOf(g.active), Label: w.Label,
		OnSelect:  func(i int) { s.selectAt(gid, i) },
		OnClose:   func(i int) { s.closeAt(gid, i) },
		OnMove:    func(from, to int) { s.reorderIn(gid, from, to) },
		Group:     &s.tabs,
		OnReceive: func(key core.Key, i int) { s.receive(gid, key, i) },
		// The panel moved when the other strip received it.
		OnDetach: func(int) {},
	}
	if w.AutoHide {
		bar = dockStripRow{line: th.Color.Hair, children: []core.Widget{
			bar,
			basic.Padding{
				Insets: geom.Insets{Left: tabButtonGap, Right: tabButtonGap, Top: tabTop + (theme.ControlHeight-tabButtonSize)/2},
				Child:  dockIconButton(th, l.AutoHide, s.pinned, true, false, func() { s.unpinGroup(gid) }),
			},
		}}
	}
	return dockSlotView{
		hidden:   maxed != nil && maxed != g,
		onFocus:  func(within bool) { s.focusGroup(gid, within) },
		onDouble: func() { s.toggleMaximize(gid) },
		child:    bar,
	}
}
