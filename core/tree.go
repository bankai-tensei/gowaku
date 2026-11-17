package core

import (
	"cmp"
	"slices"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

// Tree is the element and render trees of one window, with their scheduling.
// A frame calls Build, Layout and Paint, in that order.
type Tree struct {
	root   *element
	rootRO RenderObject

	dirty       []*element
	layoutDirty []*RenderBase
	needsPaint  bool
	inBuild     bool
	// dirtyLayers holds the repaint boundaries to record again.
	dirtyLayers []*RenderRepaintBoundary
	// paintGeneration counts the paints. It starts at 1, because a render object
	// starts with 0 in RenderBase.markedPaint.
	paintGeneration uint64

	// keyMaps is a stack, because reconciling recurses.
	keyMaps []map[Key]*element

	host Host
	// hits is the storage of pointer hit tests. captured is what gets the
	// events of the press in progress, and hovered what the pointer is over.
	hits          HitTestResult
	captured      []RenderObject
	hovered       []RenderObject
	pointerInside bool
	cursor        input.Cursor

	clicks        int
	lastPress     geom.Offset
	lastPressTime uint32
	lastButton    input.Button

	tickers []*Ticker
	// running is the storage of Tick, because tickers can stop while it runs.
	running []*Ticker
	// posts holds the work that other goroutines sent through a Poster.
	posts postQueue

	// events holds the copies of a pointer event that the targets get, one
	// per depth of delivery, and eventDepth how many are in use. See handOver.
	events     []*PointerEvent
	eventDepth int

	focus FocusTarget
	// focusTold is the object whose last FocusChanged said true, and
	// focusWithin the one whose FocusWithinListeners were last told that the
	// focus is below them. They trail focus while syncFocus runs, which
	// focusSyncing says it does. See syncFocus.
	focusTold    FocusTarget
	focusWithin  FocusTarget
	focusSyncing bool
	// anchor is where Tab goes on from after the focus was taken from an
	// object that became hidden, or nil. anchorAt is its place in order.
	anchor        FocusTarget
	anchorAt      int
	anchorEntry   focusEntry
	order         []focusEntry
	windowFocused bool
	// focusVisible means the last input came from the keyboard, so the focus
	// should show. See FocusVisible.
	focusVisible bool
	// touch is what the tree knows of the fingers on a touch screen.
	touch touches
	// dropTarget is what a drag over the window is over, or nil.
	dropTarget DropTarget
	// altDown means that Alt went down and nothing else happened since, so
	// that its release is a press of input.KeyAlt.
	altDown bool

	// lastSemanticsID is the last ID given to a render object that opened a
	// semantics node. owners maps the nodes of the last SemanticsTree to them.
	lastSemanticsID uint64
	owners          map[semantics.ID]RenderObject
	// blockedPath is the buffer of the path from the root to the
	// SemanticsBlocker that a semantics walk keeps to. See
	// SemanticsBuilder.Visit.
	blockedPath []RenderObject

	// probe hears the frames, with -tags gowakudebug only. See Probe.
	probe Probe
}

// NewTree builds the tree of root.
func NewTree(root Widget) *Tree {
	t := &Tree{windowFocused: true, focusVisible: true, paintGeneration: 1}
	t.root = t.inflate(root, nil)
	t.rootChanged()
	return t
}

// RootRenderObject returns the render object at the root of the tree, for
// tools that walk it, such as gowakutest. It may change after a Build.
func (t *Tree) RootRenderObject() RenderObject { return t.rootRO }

// NeedsFrame reports whether anything changed since the last Paint. When it
// reports false, a frame would draw the same picture, so none should be drawn.
func (t *Tree) NeedsFrame() bool {
	return len(t.dirty) > 0 || len(t.layoutDirty) > 0 || t.needsPaint || len(t.dirtyLayers) > 0
}

// NeedsBuild reports whether a Layout has asked for another Build, as a list
// does when it learns its height. A frame goes around again before painting.
func (t *Tree) NeedsBuild() bool { return len(t.dirty) > 0 }

// Build rebuilds every element marked by SetState, parents first. It first
// takes the focus from an object that became hidden since the last frame.
func (t *Tree) Build() {
	t.dropHiddenFocus()
	sortByDepth(t.dirty)
	for i := 0; i < len(t.dirty); i++ {
		n := len(t.dirty)
		if el := t.dirty[i]; el.dirty && el.mounted {
			el.rebuild()
		}
		// A rebuild can dirty more elements, such as dependents of Inherited.
		if len(t.dirty) > n {
			sortByDepth(t.dirty[i+1:])
		}
	}
	clear(t.dirty)
	t.dirty = t.dirty[:0]
}

// Layout brings every size and position up to date, with c applying to the
// root. Only subtrees below a dirty relayout boundary do any work.
func (t *Tree) Layout(c geom.Constraints) {
	LayoutChild(t.rootRO, c)
	slices.SortFunc(t.layoutDirty, func(a, b *RenderBase) int { return cmp.Compare(a.depth, b.depth) })
	for _, b := range t.layoutDirty {
		// A parent can have laid it out already, or it can have left the tree.
		if b.needsLayout && b.tree != nil {
			LayoutChild(b.self, b.constraints)
		}
	}
	clear(t.layoutDirty)
	t.layoutDirty = t.layoutDirty[:0]
}

// Paint records the picture of the tree, along with the layers that changed. The
// layers that did not change appear in it as they are.
func (t *Tree) Paint(cv *paint.Canvas) {
	t.rootRO.Paint(cv, geom.Offset{})
	t.finishPaint()
}

// Repaint is Paint for a canvas that holds the result of the previous Paint or
// Repaint. It leaves the canvas alone if only layers changed, and otherwise
// resets it. It reports whether it recorded the canvas again, which means that
// anything in the window may have changed.
func (t *Tree) Repaint(cv *paint.Canvas) bool {
	if !t.needsPaint {
		t.finishPaint()
		return false
	}
	cv.Reset()
	t.Paint(cv)
	return true
}

// finishPaint records the layers that nothing above them asked to paint.
func (t *Tree) finishPaint() {
	for i, r := range t.dirtyLayers {
		t.dirtyLayers[i] = nil
		if r.tree == t {
			r.flush()
		}
	}
	t.dirtyLayers = t.dirtyLayers[:0]
	t.needsPaint = false
	t.paintGeneration++
}

func (t *Tree) addDirtyLayer(r *RenderRepaintBoundary) {
	r.stale = true
	t.dirtyLayers = append(t.dirtyLayers, r)
}

// MarkNeedsPaint asks for a frame even though nothing changed, such as when
// the OS has lost the window contents.
func (t *Tree) MarkNeedsPaint() { t.needsPaint = true }

// HitTest returns the render objects at pos, deepest first.
func (t *Tree) HitTest(pos geom.Offset) *HitTestResult {
	r := &HitTestResult{}
	if !isHidden(t.rootRO) {
		t.rootRO.HitTest(pos, r)
	}
	return r
}

// Semantics returns the accessibility tree. Hidden subtrees (see Hider) are
// not in it, and neither is what a SemanticsBlocker covers, such as the page
// under a dialog.
func (t *Tree) Semantics() *semantics.Node {
	sb := newSemanticsBuilder(t)
	sb.build()
	return sb.root
}

// BlockingSemantics returns the SemanticsBlocker that the semantics tree keeps
// to now, such as the modal of an open dialog, or nil when none blocks. What is
// painted before it, on the way from the root to it, is out of the tree; a
// tool that checks what the user can reach, as gowakutest.Tester.Warnings
// does, skips the same.
func (t *Tree) BlockingSemantics() RenderObject {
	if t.rootRO == nil || isHidden(t.rootRO) {
		return nil
	}
	return semanticsBlocker(t.rootRO)
}

// Dispose unmounts the whole tree.
func (t *Tree) Dispose() {
	for _, k := range t.tickers {
		k.active = false
	}
	t.tickers = nil
	t.root.unmount()
	t.root, t.rootRO = nil, nil
}

func (t *Tree) rootChanged() {
	t.rootRO = renderObjectOf(t.root)
	t.rootRO.base().parent = nil
	t.needsPaint = true
}

func (t *Tree) acquireKeyMap() map[Key]*element {
	if n := len(t.keyMaps); n > 0 {
		m := t.keyMaps[n-1]
		t.keyMaps = t.keyMaps[:n-1]
		return m
	}
	return make(map[Key]*element)
}

func (t *Tree) releaseKeyMap(m map[Key]*element) {
	clear(m)
	t.keyMaps = append(t.keyMaps, m)
}

func sortByDepth(s []*element) {
	slices.SortFunc(s, func(a, b *element) int { return cmp.Compare(a.depth, b.depth) })
}
