package collection

import (
	"math"

	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
)

// ReorderableList is a List whose items the user puts in another order: by
// dragging an item, while the others slide out of its way, or from the keyboard.
// Only the items near the view are built. Like List, it takes all the room it
// is given and panics where that is unbounded.
//
// The list does not own the items. OnReorder asks to move the item at from so
// that it ends at the index to, and the caller changes its data and rebuilds:
//
//	OnReorder: func(from, to int) {
//		s.SetState(func() {
//			item := s.items[from]
//			s.items = slices.Insert(slices.Delete(s.items, from, from+1), to, item)
//		})
//	},
//
// A press anywhere on an item that does not take the press itself, as a
// checkbox inside it does, lifts the item once the pointer moves; near the
// top and the bottom of the view the list scrolls. Escape puts it back.
// Content that takes the press, such as a Pressable, a GestureDetector with
// OnTap or a SidebarItem with OnPress, keeps the item from being lifted
// there. For rows that are clicked as well as dragged, give the content no
// press of its own and set OnTap.
//
// The list is one stop of Tab. Up and Down move the focus, Home and End go to
// the first and last item, Page Up and Page Down by a view; Ctrl+Shift with Up
// or Down moves the item in focus by one place, and with Home or End to the
// first or last place, as Ctrl+Shift with an arrow moves a tab of a TabBar.
// A screen reader hears the new place of a moved item.
//
// The items have the same height, ItemExtent, unless ItemExtentOf is set, as
// in List. See docs/ADR/0091-reorderable-list.md.
type ReorderableList struct {
	core.StatefulBase
	ItemCount int
	// ItemExtent is the height of an item. Zero selects 48.
	ItemExtent float32
	// ItemExtentOf, when not nil, returns the height of item i in place of
	// ItemExtent, as in List. It is called for every item when the list first
	// appears and each time a parent rebuild gives it a new value.
	ItemExtentOf func(i int) float32
	// Label names the list to a screen reader.
	Label string
	// LabelOf, when not nil, names item i to a screen reader, which also
	// hears it when the item moves. Nil names an item by the text it shows.
	LabelOf func(i int) string
	// OnReorder asks to move the item at from to the index to, counted as the
	// items are after the move. Without it nothing moves.
	OnReorder func(from, to int)
	// OnTap runs for item i when a press on it comes up without lifting it,
	// over the same item, and for Enter or Space on the item in focus.
	OnTap func(i int)
	// ItemBuilder returns the widget of item i.
	ItemBuilder func(i int) core.Widget
}

func (l ReorderableList) extent() float32 {
	if l.ItemExtent > 0 {
		return l.ItemExtent
	}
	return defaultItemExtent
}

const (
	// reorderDragSlop is how far a press moves before it lifts the item.
	reorderDragSlop = 4
	// reorderScrollZone is the most of the band at each edge of the view
	// where a drag scrolls, at up to reorderScrollSpeed pixels a second.
	reorderScrollZone  = 48
	reorderScrollSpeed = 900
	// reorderSlideTime is the time constant of an item that slides to its
	// place: it covers 63% of the way in that time.
	reorderSlideTime = 60 * time.Millisecond
	reorderRing      = 2
	// reorderShadowBlur and reorderShadowDrop shape the shadow under a lifted
	// item.
	reorderShadowBlur = 12
	reorderShadowDrop = 4
)

type reorderState struct {
	basic.ScrollState
	// count is the number of items at the last build.
	count int
	// extents holds the heights from ItemExtentOf, read again when stale.
	extents layout.ExtentTable
	stale   bool
	// view is the height of the view, and first and last the items built.
	view        float32
	first, last int
	// focus is the item in focus.
	focus int
	body  *renderReorderBody
	// active means the keyboard is in the list; claim asks the item of the
	// focus to take the keyboard when it is built.
	active, claim bool
	drag          reorderDrag
	// settle is a move just asked for, whose items slide to their new places
	// at the next layout.
	settle reorderSettle
	// clock runs the slides and the scrolling of a drag near an edge; lastTick
	// is when it last ran.
	clock    *core.Ticker
	lastTick time.Duration
	// announce is what a screen reader hears after a move.
	announce string
	// still and words are ReduceMotion and the strings at the last build.
	still bool
	words *basic.Strings
}

// reorderDrag is a press on an item, and the drag it may become. Positions
// are relative to the view.
type reorderDrag struct {
	pressed, dragging bool
	// index is the item pressed, and target where it would go.
	index, target int
	start, pos    geom.Offset
	// grab is where the pointer holds the item, from its top.
	grab float32
}

// reorderSettle is a move from one index to another. A drop slides the moved
// item from top, where the pointer left it, in the content.
type reorderSettle struct {
	pending, dropped bool
	from, to         int
	top              float64
}

// CreateState implements core.StatefulWidget.
func (ReorderableList) CreateState() core.State { return &reorderState{stale: true} }

// InitState implements core.State.
func (s *reorderState) InitState() {
	s.ScrollState.InitState()
	s.OnMove = s.scrolled
	s.clock = s.NewTicker(s.tick)
}

// DidUpdateWidget implements core.State. The heights are read again, as List
// does, because functions cannot be compared.
func (s *reorderState) DidUpdateWidget(core.Widget) { s.stale = true }

// Dispose implements core.State.
func (s *reorderState) Dispose() {
	s.ScrollState.Dispose()
	s.clock.Stop()
}

// table returns the heights of the items, or nil for items of one height.
func (s *reorderState) table() *layout.ExtentTable {
	w := s.Widget().(ReorderableList)
	if w.ItemExtentOf == nil {
		return nil
	}
	if s.stale {
		s.extents.Reset(max(w.ItemCount, 0), w.ItemExtentOf)
		s.stale = false
	}
	return &s.extents
}

// top returns the top of item i, at rest, in the content. i may be count.
func (s *reorderState) top(i int) float64 {
	if t := s.table(); t != nil {
		return t.Offset(min(max(i, 0), t.Len()))
	}
	return float64(i) * float64(s.Widget().(ReorderableList).extent())
}

// extentOf returns the height of item i.
func (s *reorderState) extentOf(i int) float32 {
	if t := s.table(); t != nil {
		if i < 0 || i >= t.Len() {
			return 0
		}
		return t.Extent(i)
	}
	return s.Widget().(ReorderableList).extent()
}

// indexAt returns the item that covers y in the content, clamped to the items.
func (s *reorderState) indexAt(y float64) int {
	if t := s.table(); t != nil {
		return t.IndexAt(y)
	}
	e := float64(s.Widget().(ReorderableList).extent())
	return min(max(int(math.Floor(y/e)), 0), max(s.count-1, 0))
}

// content returns the height of all the items.
func (s *reorderState) content() float64 { return s.top(s.count) }

// visible returns the items to build at the current offset.
func (s *reorderState) visible() (first, last int) {
	off := s.Offset.Value()
	last = max(min(s.indexAt(off+float64(s.view))+1+cachedItems, s.count), 0)
	return min(max(s.indexAt(off)-cachedItems, 0), last), last
}

// scrolled follows a move of the view.
func (s *reorderState) scrolled() {
	if s.body != nil {
		s.body.scrolled()
	}
	if s.drag.dragging {
		s.dragOver()
	}
	if first, last := s.visible(); first != s.first || last != s.last {
		kit.Rebuild(&s.StateBase)
	}
}

// measured follows the height of the view at a layout.
func (s *reorderState) measured(view float32) {
	s.Metrics = layout.ScrollMetrics{View: float64(view), Content: s.content()}
	s.Scroller.SetLimit(s.Metrics.Content - float64(view))
	if view != s.view {
		s.view = view
		kit.Rebuild(&s.StateBase)
	} else if first, last := s.visible(); first != s.first || last != s.last {
		kit.Rebuild(&s.StateBase)
	}
}

// reveal scrolls item i into view, at once.
func (s *reorderState) reveal(i int) {
	if i < 0 || i >= s.count || s.view <= 0 {
		return
	}
	top, h := s.top(i), float64(s.extentOf(i))
	off, view := s.Offset.Value(), float64(s.view)
	switch {
	case top < off:
		off = top
	case top+h > off+view:
		off = top + h - view
	default:
		return
	}
	s.Scroller.Jump(min(max(off, 0), max(s.content()-view, 0)))
	s.Ticker.Stop()
	if s.body != nil {
		s.body.scrolled()
	}
	s.ShowBar()
	if first, last := s.visible(); first != s.first || last != s.last {
		kit.Rebuild(&s.StateBase)
	}
}

// Build implements core.State.
func (s *reorderState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget().(ReorderableList)
	n := max(w.ItemCount, 0)
	s.count = n
	if s.drag.pressed && s.drag.index >= n {
		// The item under the pointer left the list.
		s.endDrag()
	}
	s.drag.target = min(s.drag.target, max(n-1, 0))
	if s.settle.pending && (s.settle.from >= n || s.settle.to >= n) {
		s.settle.pending = false
	}
	s.focus = min(max(s.focus, 0), max(n-1, 0))
	s.first, s.last = s.visible()
	s.still = basic.ReduceMotionOf(ctx)
	s.words = basic.StringsOf(ctx)
	th := basic.ThemeOf(ctx)
	look := reorderLook{
		ring: th.Color.AccentInk.WithAlpha(treeRingOpacity), ground: th.Color.Raised,
		shadow: th.Color.Shadow, radius: th.Radius.Control,
	}

	focus := -1
	if n > 0 {
		focus = s.focus
	}
	count := s.last - s.first
	items := make([]core.Widget, 0, count+1)
	slots := make([]int, 0, count+1)
	add := func(i int) {
		v := reorderItemView{host: s, index: i, count: n, look: &look, focusable: i == focus, claim: i == focus && s.claim}
		if w.LabelOf != nil {
			v.label = w.LabelOf(i)
		}
		if w.ItemBuilder != nil {
			if child := w.ItemBuilder(i); child != nil {
				v.child = basic.RepaintBoundary{Child: child}
			}
		}
		items = append(items, core.Keyed(i, v))
		slots = append(slots, i)
	}
	if focus >= 0 && focus < s.first {
		add(focus)
	}
	for i := s.first; i < s.last; i++ {
		add(i)
	}
	if focus >= s.last {
		add(focus)
	}
	s.claim = false

	body := reorderBody{host: s, slots: slots, items: items, look: look, ref: &s.body}
	list := interact.Semantics{Role: semantics.RoleList, Label: w.Label, Child: basic.ScrollFrame{
		Axis: layout.Vertical, State: &s.ScrollState, View: basic.RepaintBoundary{Child: body},
	}}
	return basic.Stack{Children: []core.Widget{
		list,
		interact.Semantics{Role: semantics.RoleText, Label: s.announce, Live: semantics.LivePolite, Child: basic.SizedBox{}},
	}}
}

// lostFocus follows the item of the focus leaving the list with the keyboard,
// as when the items shrink: the item of the new focus takes it back.
func (s *reorderState) lostFocus() {
	if s.active && !s.claim && s.count > 0 {
		s.claim = true
		kit.Rebuild(&s.StateBase)
	}
}

// setActive follows the keyboard entering and leaving the list.
func (s *reorderState) setActive(active bool) {
	if s.active != active {
		s.active = active
		kit.Rebuild(&s.StateBase)
	}
}

// reorderLook is what the items take from the theme.
type reorderLook struct {
	ring, ground, shadow geom.Color
	radius               float32
}
