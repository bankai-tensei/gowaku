package collection

import (
	"fmt"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
)

// Tree shows nodes that open to show the nodes below them, as the folders
// of a file explorer do. Only the rows near the view are built, so a tree of a
// hundred thousand open nodes scrolls as lightly as a short one. Like List
// it takes all the room it is given, and panics where that is unbounded.
//
// The application owns the data and describes it with functions of a key K,
// which identifies a node for as long as the node exists: a path, an ID, or a
// pointer.
//
//	Tree[string]{
//		Roots:      []string{"/"},
//		ChildrenOf: fs.entries,
//		HasChildren: fs.isDir,
//		LabelOf:    path.Base,
//		OnActivate: s.open,
//	}
//
// What is open, selected and in focus lives in a TreeController, which the
// tree makes for itself unless Controller gives one to read and change from
// outside. The keys follow the tree pattern of ARIA: Up and Down move, Right
// opens a node and then enters it, Left closes it and then goes to the parent,
// Home, End, Page Up and Page Down jump, typing the first letters of a label
// finds it, * opens the siblings, Enter activates, F2 renames, and the menu key
// or Shift+F10 asks OnContextMenu for a menu. Tab reaches one row only, the
// one the arrows move. A click of the secondary button selects the row under
// it, unless it is in the selection already, before it asks for a menu.
//
// A node whose children are not loaded yet says so with HasChildren while
// ChildrenOf returns none: opening it shows a loading row and calls OnLoad,
// and the row stays until ChildrenOf returns the children after a rebuild. If
// the load finds none, make HasChildren false; to retry a failed load, close
// the node.
//
// With OnMove, rows drag to another place: before, after or into a node. The
// tree reports the move and the caller changes its data. See
// docs/ADR/0059-tree-view.md.
type Tree[K comparable] struct {
	core.StatefulBase
	// Roots are the nodes at the top level.
	Roots []K
	// ChildrenOf returns the children of a node, in order. It is called for
	// every open node each time the tree reads its data, which is when the
	// tree first appears, when a parent rebuild gives it a new Tree, and
	// when a node opens or closes; scrolling does not call it. Keep it cheap.
	ChildrenOf func(node K) []K
	// HasChildren reports whether a node can open. Nil asks ChildrenOf, so
	// that a node without children shows no chevron; set it for children that
	// load later, and for empty folders that should still take drops.
	HasChildren func(node K) bool
	// LabelOf returns the name a row shows. Nil shows a string key itself, or
	// the String of a fmt.Stringer.
	LabelOf func(node K) string
	// IconOf returns the widget before the label, such as a folder icon, in a
	// box of 16 by 16: basic.Icon{Path: folder, Size: 16}, where folder is
	// icons.Folder() from package icons, built once. Nil or a nil widget
	// shows none.
	IconOf func(node K) core.Widget
	// Label names the whole tree to assistive technology.
	Label string
	// Controller holds what is open, selected and in focus. Nil keeps that
	// inside the tree. A controller serves one tree at a time.
	Controller *TreeController[K]
	// Multiselect lets Ctrl and Shift select several rows, and Ctrl+A all of
	// them. Without it one row is selected at most.
	Multiselect bool
	// OnSelectionChange reports the selected nodes after the user changes
	// them, in the order they were selected.
	OnSelectionChange func(selected []K)
	// OnActivate runs for Enter and a double click. Without it those open and
	// close the node instead.
	OnActivate func(node K)
	// OnContextMenu asks for the context menu of node: after a click of the
	// secondary button on its row, or the menu key or Shift+F10 on the row in
	// focus. pos is where to open the menu, from the top left of the tree: the
	// release of the button, or below the label of the row for a key. Open the
	// menu with overlay.ShowPopupMenu from a context at the place of the tree,
	// as that of a basic.Builder around it. Before it runs, the secondary press
	// has selected the row, unless the row was in the selection already, and
	// given it the focus. Without OnContextMenu the press still selects and
	// focuses the row, and goes on to an overlay.ContextMenu around the tree.
	OnContextMenu func(node K, pos geom.Offset)
	// OnLoad asks for the children of a node that HasChildren says has some,
	// and ChildrenOf does not return yet. It runs once each time such a node
	// opens; rebuild the tree with the children when they arrive.
	OnLoad func(node K)
	// OnRename, when set, lets F2 or Controller.Rename edit a label in place.
	// Enter and leaving the field commit the new name to it, Escape cancels.
	OnRename func(node K, name string)
	// OnMove, when set, lets rows be dragged. It receives the move when they
	// are dropped; the tree changes nothing itself.
	OnMove func(m TreeMove[K])
	// CanDrop, when set, refuses moves it returns false for. The tree already
	// refuses a node into itself or into its own descendants, and a drop into
	// a node that cannot open.
	CanDrop func(m TreeMove[K]) bool
}

// TreeDrop says where dropped rows go relative to the node under the pointer.
//
// It is TreeDropBefore (the zero value), TreeDropAfter or TreeDropInto.
type TreeDrop uint8

// The places of a drop.
const (
	// TreeDropBefore puts the rows just above the node, among its siblings.
	TreeDropBefore TreeDrop = iota
	// TreeDropAfter puts the rows just below the node. Below an open node
	// with children, they become its first children.
	TreeDropAfter
	// TreeDropInto makes the rows the last children of the node.
	TreeDropInto
)

// String returns the name of the place.
func (d TreeDrop) String() string {
	switch d {
	case TreeDropBefore:
		return "before"
	case TreeDropAfter:
		return "after"
	case TreeDropInto:
		return "into"
	}
	return "unknown"
}

// TreeMove is the move of a drag: Nodes go to Index among the children of
// Parent. The caller carries it out.
type TreeMove[K comparable] struct {
	// Nodes are the moved nodes in the order they show. A node whose
	// ancestor moves too is left out: it moves with that ancestor.
	Nodes []K
	// Target is the node under the pointer and Drop the place relative to it.
	Target K
	Drop   TreeDrop
	// Parent is the node the moved nodes go under. AtRoot means they go among
	// the roots instead, and Parent is then the zero K.
	Parent K
	AtRoot bool
	// Index is the place among the children of Parent, or the roots, counted
	// as they are before the move: 0 puts the nodes first, the number of
	// children last. Moved nodes above Index in the same parent still count.
	Index int
}

// treeRow is a row of the flattened tree: a node that shows, or the loading
// row under a node whose children are coming.
type treeRow[K comparable] struct {
	key K
	// parent is the index of the row of the parent, or -1 for a root.
	parent int32
	// pos and size place the node among its siblings, from 1.
	pos, size int32
	depth     int16
	// expandable says the node can open, and expanded that it is.
	expandable, expanded bool
	// loading marks the loading row; key is then that of the parent.
	loading bool
}

// treeKey keys the row of a node, or the loading row below it.
type treeKey[K comparable] struct {
	key     K
	loading bool
}

type treeState[K comparable] struct {
	basic.ScrollState
	own  TreeController[K]
	ctrl *TreeController[K]
	rows []treeRow[K]
	// seen is the generation of the controller the rows were made from, and
	// stale says the data must be read again.
	seen  uint64
	stale bool
	// view is the height of the view, and first and last the rows built.
	view        float32
	first, last int
	// focusAt caches the row of the focus.
	focusAt int
	body    *renderTreeBody
	// active means the keyboard is in the tree, which colors the selection.
	active bool
	// claim asks the row of the focus to take the keyboard when it is built.
	claim bool
	ahead kit.TypeAhead
	// requested holds the nodes whose load is under way, and pending those
	// to ask for on the next frame.
	requested map[K]struct{}
	pending   []K
	loader    *core.Ticker
	// spinner turns the loading rows, at spinAngle.
	spinner   *core.Ticker
	spinAngle float32
	// turn rotates the chevron of the node that opened or closed last.
	turn      motion.SpringValue
	turnKey   K
	turning   bool
	turnPath  paint.Path
	right     *paint.Path
	down      *paint.Path
	arc       *paint.Path
	drag      treeDrag[K]
	dragClock *core.Ticker
	// renaming is the node whose label is edited.
	renameKey K
	renaming  bool
}

// CreateState implements core.StatefulWidget.
func (Tree[K]) CreateState() core.State { return &treeState[K]{stale: true, focusAt: -1} }

// InitState implements core.State.
func (s *treeState[K]) InitState() {
	s.ScrollState.InitState()
	s.OnMove = s.scrolled
	s.right, s.down = kit.ChevronPath(true), turnedChevron(nil, 90)
	s.arc = &paint.Path{}
	layout.RingPath(s.arc, treeSpinnerSize, treeSpinnerThickness, treeSpinnerShare)
	s.ahead.Attach(&s.StateBase)
	s.loader = s.NewTicker(s.loadPending)
	s.spinner = s.NewTicker(s.spin)
	s.dragClock = s.NewTicker(s.dragTick)
	s.turn.Spring = motion.SnappySpring()
	s.turn.OnChange = s.turned
	s.turn.Attach(&s.StateBase, 0)
	s.attach(s.Widget().(Tree[K]).Controller)
}

// attach makes c, or the tree's own controller for nil, the one it shows.
func (s *treeState[K]) attach(c *TreeController[K]) {
	if c == nil {
		c = &s.own
	}
	if s.ctrl == c {
		return
	}
	if s.ctrl != nil {
		s.ctrl.listener = nil
	}
	s.ctrl = c
	c.listener = s.controllerChanged
	s.stale = true
}

// DidUpdateWidget implements core.State. The data is read again, because the
// functions may read new data and cannot be compared.
func (s *treeState[K]) DidUpdateWidget(core.Widget) {
	s.attach(s.Widget().(Tree[K]).Controller)
	s.stale = true
}

// Dispose implements core.State.
func (s *treeState[K]) Dispose() {
	s.ScrollState.Dispose()
	s.turn.Dispose()
	s.loader.Stop()
	s.spinner.Stop()
	s.dragClock.Stop()
	if s.ctrl != nil {
		s.ctrl.listener = nil
	}
}

func (s *treeState[K]) controllerChanged() { kit.Rebuild(&s.StateBase) }

// expandable reports whether node can open.
func (s *treeState[K]) expandable(w *Tree[K], node K) bool {
	if w.HasChildren != nil {
		return w.HasChildren(node)
	}
	return w.ChildrenOf != nil && len(w.ChildrenOf(node)) > 0
}

// flatten lists the rows that show, in order.
func (s *treeState[K]) flatten() {
	w := s.Widget().(Tree[K])
	s.rows = s.rows[:0]
	s.addLevel(&w, w.Roots, -1, 0)
	s.seen = s.ctrl.generation
	s.stale = false
	s.focusAt = -1
}

func (s *treeState[K]) addLevel(w *Tree[K], keys []K, parent int32, depth int) {
	for i, k := range keys {
		at := int32(len(s.rows))
		can := s.expandable(w, k)
		open := can && s.ctrl.Expanded(k)
		s.rows = append(s.rows, treeRow[K]{
			key: k, parent: parent, pos: int32(i + 1), size: int32(len(keys)),
			depth: int16(depth), expandable: can, expanded: open,
		})
		if !open {
			continue
		}
		var children []K
		if w.ChildrenOf != nil {
			children = w.ChildrenOf(k)
		}
		if len(children) == 0 {
			s.rows = append(s.rows, treeRow[K]{key: k, parent: at, depth: int16(depth + 1), loading: true})
			s.askLoad(w, k)
			continue
		}
		if len(s.requested) > 0 {
			delete(s.requested, k)
		}
		s.addLevel(w, children, at, depth+1)
	}
}

// askLoad asks for the children of node on the next frame, unless they were
// asked for already: the flattening runs in Build, where OnLoad may not
// rebuild anything.
func (s *treeState[K]) askLoad(w *Tree[K], node K) {
	if w.OnLoad == nil {
		return
	}
	if _, ok := s.requested[node]; ok {
		return
	}
	if s.requested == nil {
		s.requested = make(map[K]struct{})
	}
	s.requested[node] = struct{}{}
	s.pending = append(s.pending, node)
	s.loader.Start()
}

func (s *treeState[K]) loadPending(_ time.Duration) {
	s.loader.Stop()
	pending := s.pending
	s.pending = nil
	if load := s.Widget().(Tree[K]).OnLoad; load != nil {
		for _, k := range pending {
			load(k)
		}
	}
}

// labelOf returns the label of node.
func (s *treeState[K]) labelOf(w *Tree[K], node K) string {
	if w.LabelOf != nil {
		return w.LabelOf(node)
	}
	switch v := any(node).(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	}
	return ""
}

// focusIndex returns the row of the focus, or -1 for an empty tree. Without
// a focus, or with one that does not show, it is the first selected row that
// shows, or else the first row.
func (s *treeState[K]) focusIndex() int {
	c := s.ctrl
	if i := s.focusAt; i >= 0 && i < len(s.rows) && c.hasFocus && !s.rows[i].loading && s.rows[i].key == c.focus {
		return i
	}
	s.focusAt = -1
	if c.hasFocus {
		s.focusAt = s.indexOf(c.focus)
	}
	if s.focusAt < 0 && len(c.order) > 0 {
		for i := range s.rows {
			if !s.rows[i].loading && c.Selected(s.rows[i].key) {
				s.focusAt = i
				break
			}
		}
	}
	if s.focusAt < 0 && len(s.rows) > 0 {
		s.focusAt = 0
	}
	return s.focusAt
}

// indexOf returns the row of node, or -1.
func (s *treeState[K]) indexOf(node K) int {
	for i := range s.rows {
		if !s.rows[i].loading && s.rows[i].key == node {
			return i
		}
	}
	return -1
}

// visible returns the rows to build at the current offset.
func (s *treeState[K]) visible() (first, last int) {
	top := s.Offset.Value()
	first = int(top/treeRowHeight) - cachedItems
	last = max(min(int((top+float64(s.view))/treeRowHeight)+1+cachedItems, len(s.rows)), 0)
	// The offset may lie past the rows until the next layout clamps it, as
	// when a node above the view closes.
	return min(max(first, 0), last), last
}

// scrolled follows a move of the view: the rows move at once, and other rows
// are built when they come into range.
func (s *treeState[K]) scrolled() {
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

func (s *treeState[K]) measured(view float32) {
	s.Metrics = layout.ScrollMetrics{View: float64(view), Content: float64(len(s.rows)) * treeRowHeight}
	s.Scroller.SetLimit(s.Metrics.Content - float64(view))
	if view != s.view {
		s.view = view
		// A reveal asked for before the first layout waits for the height.
		if first, last := s.visible(); first != s.first || last != s.last || s.ctrl.reveal {
			kit.Rebuild(&s.StateBase)
		}
	}
}

// reveal scrolls row i into view, at once.
func (s *treeState[K]) reveal(i int) {
	if i < 0 || s.view <= 0 {
		return
	}
	top := float64(i) * treeRowHeight
	off, view := s.Offset.Value(), float64(s.view)
	switch {
	case top < off:
		off = top
	case top+treeRowHeight > off+view:
		off = top + treeRowHeight - view
	default:
		return
	}
	limit := max(float64(len(s.rows))*treeRowHeight-view, 0)
	s.Scroller.Jump(min(max(off, 0), limit))
	s.Ticker.Stop()
	if s.body != nil {
		s.body.scrolled()
	}
	s.ShowBar()
}

// Build implements core.State.
func (s *treeState[K]) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget().(Tree[K])
	shaper, ok := core.DependOn[*text.Shaper](ctx)
	if !ok {
		panic("gowaku: collection.Tree needs a text.Shaper above it; wrap the root in core.Inherited[*text.Shaper]")
	}
	if s.stale || s.seen != s.ctrl.generation {
		s.flatten()
	}
	c := s.ctrl
	if c.renaming {
		c.renaming = false
		if w.OnRename != nil && s.indexOf(c.rename) >= 0 {
			s.renameKey, s.renaming, s.claim = c.rename, true, false
		}
	}
	focus := s.focusIndex()
	// Before the first layout the tree has no height to scroll within, so
	// the reveal stays asked for until measured knows it.
	if c.reveal && s.view > 0 {
		c.reveal = false
		s.reveal(focus)
	}
	if s.renaming && s.indexOf(s.renameKey) < 0 {
		s.renaming = false
	}
	s.first, s.last = s.visible()
	look := newTreeLook(basic.ThemeOf(ctx), s.active)
	still := basic.ReduceMotionOf(ctx)
	words := basic.StringsOf(ctx)

	n := s.last - s.first
	rows := make([]core.Widget, 0, n+1)
	slots := make([]int, 0, n+1)
	loading := false
	add := func(i int) {
		rows = append(rows, s.row(&w, i, i == focus, shaper, &look, words))
		slots = append(slots, i)
		loading = loading || s.rows[i].loading
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
	switch {
	case loading && !still:
		s.spinner.Start()
	default:
		s.spinner.Stop()
		s.spinAngle = 0
	}
	s.claim = false

	var state semantics.State
	if w.Multiselect {
		state = semantics.Multiselectable
	}
	body := treeBody{state: s, position: &s.Offset, slots: slots, rows: rows, bar: look.bar, ref: &s.body}
	return interact.Semantics{Role: semantics.RoleTree, Label: w.Label, State: state, Child: basic.ScrollFrame{
		Axis: layout.Vertical, State: &s.ScrollState, View: basic.RepaintBoundary{Child: body},
	}}
}

// row builds row i.
func (s *treeState[K]) row(w *Tree[K], i int, focus bool, shaper *text.Shaper, look *treeLook, words *basic.Strings) core.Widget {
	r := &s.rows[i]
	v := treeRowView{
		host: s, index: i, depth: int(r.depth), shaper: shaper, look: look,
		chevrons: [2]*paint.Path{s.right, s.down},
	}
	if r.loading {
		v.label, v.loading, v.arc = words.Loading, true, s.arc
		v.level = int(r.depth) + 1
		v.iconRoom = w.IconOf != nil
		return core.Keyed(treeKey[K]{key: r.key, loading: true}, v)
	}
	v.label = s.labelOf(w, r.key)
	v.level, v.pos, v.size = int(r.depth)+1, int(r.pos), int(r.size)
	v.expandable, v.expanded = r.expandable, r.expanded
	v.selected = s.ctrl.Selected(r.key)
	v.focusable = focus
	v.claim = focus && s.claim
	v.angle = 0
	if r.expanded {
		v.angle = 90
	}
	if s.turning && r.key == s.turnKey {
		v.angle = s.turn.Value()
		v.turning = &s.turnPath
	}
	if w.IconOf != nil {
		v.icon = w.IconOf(r.key)
	}
	if s.renaming && r.key == s.renameKey {
		key := r.key
		v.editor = treeRenamer{
			initial:  v.label,
			onCommit: func(name string, blurred bool) { s.commitRename(key, name, blurred) },
			onCancel: s.cancelRename,
		}
	}
	return core.Keyed(treeKey[K]{key: r.key}, v)
}

// turned follows the chevron that turns.
func (s *treeState[K]) turned() {
	if !s.turn.Moving() {
		s.turning = false
	}
	kit.Rebuild(&s.StateBase)
}

// spin turns the loading rows.
func (s *treeState[K]) spin(now time.Duration) {
	s.spinAngle = float32(now.Seconds()*treeSpinTurns-float64(int64(now.Seconds()*treeSpinTurns))) * 360
	if s.body == nil {
		return
	}
	for _, c := range s.body.children {
		if r, ok := c.(*renderTreeRow); ok && r.loading {
			r.MarkNeedsPaint()
		}
	}
}

func (s *treeState[K]) spinnerAngle() float32 { return s.spinAngle }

// lostFocus implements treeHost. A row that had the keyboard went with its
// node, which the tree is not told of: the row of the focus takes it back.
func (s *treeState[K]) lostFocus() {
	if s.active && !s.claim && len(s.rows) > 0 {
		s.claim = true
		kit.Rebuild(&s.StateBase)
	}
}

// setActive follows the keyboard entering and leaving the tree.
func (s *treeState[K]) setActive(active bool) {
	if s.active != active {
		s.active = active
		kit.Rebuild(&s.StateBase)
	}
}
