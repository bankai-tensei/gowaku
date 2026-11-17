package core

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

// The widgets below stand in for the layout package, which core cannot import.

// leaf has a fixed size and counts its render object's work.
type leaf struct {
	name string
	w, h float32
}

type renderLeaf struct {
	RenderBase
	name    string
	w, h    float32
	layouts int
}

func (leaf) Key() Key { return nil }

func (l leaf) CreateRenderObject() RenderObject {
	return &renderLeaf{name: l.name, w: l.w, h: l.h}
}

func (l leaf) UpdateRenderObject(ro RenderObject) {
	r := ro.(*renderLeaf)
	r.name = l.name
	if r.w != l.w || r.h != l.h {
		r.w, r.h = l.w, l.h
		r.MarkNeedsLayout()
	}
}

func (r *renderLeaf) Layout(c geom.Constraints) geom.Size {
	r.layouts++
	return c.Constrain(geom.Size{W: r.w, H: r.h})
}

func (r *renderLeaf) Paint(*paint.Canvas, geom.Offset) {}

// list stacks its children vertically under loose constraints.
type list struct {
	children []Widget
}

type renderList struct {
	RenderBase
	children []RenderObject
	layouts  int
}

func (list) Key() Key                                  { return nil }
func (list) CreateRenderObject() RenderObject          { return &renderList{} }
func (list) UpdateRenderObject(RenderObject)           {}
func (l list) ChildWidgets() []Widget                  { return l.children }
func (r *renderList) SetChildren(c []RenderObject)     { r.children = c }
func (r *renderList) ChildCount() int                  { return len(r.children) }
func (r *renderList) ChildAt(i int) RenderObject       { return r.children[i] }
func (r *renderList) Paint(*paint.Canvas, geom.Offset) {}

func (r *renderList) Layout(c geom.Constraints) geom.Size {
	r.layouts++
	var y, w float32
	for _, child := range r.children {
		sz := LayoutChild(child, c.Loosen())
		child.SetOffset(geom.Offset{Y: y})
		y += sz.H
		w = max(w, sz.W)
	}
	return c.Constrain(geom.Size{W: w, H: y})
}

// box passes tight or loose constraints to its optional child.
type box struct {
	tight bool
	size  geom.Size
	child Widget
}

type renderBox struct {
	RenderBase
	tight   bool
	size    geom.Size
	child   RenderObject
	layouts int
}

func (box) Key() Key { return nil }

func (b box) CreateRenderObject() RenderObject {
	return &renderBox{tight: b.tight, size: b.size}
}

func (b box) UpdateRenderObject(ro RenderObject) {
	r := ro.(*renderBox)
	if r.tight != b.tight || r.size != b.size {
		r.tight, r.size = b.tight, b.size
		r.MarkNeedsLayout()
	}
}

func (b box) ChildWidget() Widget                     { return b.child }
func (r *renderBox) SetChild(c RenderObject)          { r.child = c }
func (r *renderBox) Paint(*paint.Canvas, geom.Offset) {}

func (r *renderBox) ChildCount() int {
	if r.child == nil {
		return 0
	}
	return 1
}

func (r *renderBox) ChildAt(int) RenderObject { return r.child }

func (r *renderBox) Layout(c geom.Constraints) geom.Size {
	r.layouts++
	if r.child == nil {
		return c.Constrain(r.size)
	}
	if r.tight {
		LayoutChild(r.child, geom.Tight(r.size))
		return c.Constrain(r.size)
	}
	return c.Constrain(LayoutChild(r.child, geom.Loose(c.Biggest())))
}

// stateful is a widget whose content is decided from outside the tree.
type stateful struct {
	StatefulBase
	id    string
	log   *[]string
	build func(s *testState, ctx BuildContext) Widget
}

type testState struct {
	StateBase
}

func (s stateful) CreateState() State { return &loggingState{} }

type loggingState struct {
	testState
}

func (s *loggingState) config() stateful { return s.Widget().(stateful) }

func (s *loggingState) note(event string) {
	if l := s.config().log; l != nil {
		*l = append(*l, s.config().id+":"+event)
	}
}

func (s *loggingState) InitState() { s.note("init") }
func (s *loggingState) Dispose()   { s.note("dispose") }

func (s *loggingState) DidUpdateWidget(Widget) { s.note("update") }

func (s *loggingState) Build(ctx BuildContext) Widget {
	s.note("build")
	return s.config().build(&s.testState, ctx)
}

func statefulLeaf(id string, log *[]string) stateful {
	return stateful{id: id, log: log, build: func(*testState, BuildContext) Widget { return leaf{w: 1, h: 1} }}
}

// host holds a widget that the test replaces, the way SetState would.
type host struct {
	StatefulBase
	current *Widget
	state   **hostState
}

type hostState struct {
	StateBase
}

func (h host) CreateState() State {
	s := &hostState{}
	*h.state = s
	return s
}

func (s *hostState) Build(BuildContext) Widget { return *s.Widget().(host).current }

// harness runs a tree whose root the test can swap.
type harness struct {
	tree    *Tree
	current Widget
	state   *hostState
}

func newHarness(w Widget) *harness {
	h := &harness{current: w}
	h.tree = NewTree(host{current: &h.current, state: &h.state})
	return h
}

func (h *harness) set(w Widget) {
	h.state.SetState(func() { h.current = w })
	h.tree.Build()
}

func (h *harness) layout() { h.tree.Layout(geom.Tight(geom.Size{W: 100, H: 100})) }

func (h *harness) rootRO() RenderObject { return h.tree.rootRO }

func expectLog(t *testing.T, got *[]string, want ...string) {
	t.Helper()
	if strings.Join(*got, " ") != strings.Join(want, " ") {
		t.Errorf("got  %v\nwant %v", *got, want)
	}
	*got = nil
}

func TestTree_SameTypeKeepsRenderObject(t *testing.T) {
	h := newHarness(leaf{name: "a", w: 1, h: 1})
	before := h.rootRO()
	h.set(leaf{name: "b", w: 1, h: 1})
	if h.rootRO() != before {
		t.Error("render object was replaced")
	}
	if got := before.(*renderLeaf).name; got != "b" {
		t.Errorf("name: got %q, want b", got)
	}
}

func TestTree_DifferentTypeReplacesRenderObject(t *testing.T) {
	h := newHarness(leaf{w: 1, h: 1})
	before := h.rootRO()
	h.set(box{})
	if h.rootRO() == before {
		t.Error("render object was kept across types")
	}
	if before.base().tree != nil {
		t.Error("the old render object must be detached")
	}
}

func TestTree_NewRenderObjectBelowComponentReachesParent(t *testing.T) {
	var inner *loggingState
	var swap bool
	child := stateful{build: func(*testState, BuildContext) Widget {
		if swap {
			return box{}
		}
		return leaf{w: 1, h: 1}
	}}
	h := newHarness(list{children: []Widget{child}})
	h.layout()
	parent := h.rootRO().(*renderList)
	if _, ok := parent.children[0].(*renderLeaf); !ok {
		t.Fatalf("got %T", parent.children[0])
	}
	el := h.tree.root.child.children[0]
	inner = el.state.(*loggingState)
	inner.SetState(func() { swap = true })
	h.tree.Build()
	if _, ok := parent.children[0].(*renderBox); !ok {
		t.Fatalf("parent still has %T", parent.children[0])
	}
	if parent.children[0].Parent() != RenderObject(parent) {
		t.Error("new child has the wrong parent")
	}
	if !parent.needsLayout {
		t.Error("parent must be laid out again")
	}
}

func TestState_Lifecycle(t *testing.T) {
	var log []string
	h := newHarness(statefulLeaf("a", &log))
	expectLog(t, &log, "a:init", "a:build")

	h.set(statefulLeaf("a", &log))
	expectLog(t, &log, "a:update", "a:build")

	h.set(leaf{w: 1, h: 1})
	expectLog(t, &log, "a:dispose")
}

func TestState_SetStateRebuildsOnlyThatState(t *testing.T) {
	var log []string
	h := newHarness(list{children: []Widget{statefulLeaf("a", &log), statefulLeaf("b", &log)}})
	log = nil
	h.tree.root.child.children[1].state.(*loggingState).SetState(func() {})
	if !h.tree.NeedsFrame() {
		t.Error("SetState must ask for a frame")
	}
	h.tree.Build()
	expectLog(t, &log, "b:build")
}

func TestState_DirtyParentRebuildsChildOnce(t *testing.T) {
	var log []string
	h := newHarness(stateful{id: "parent", log: &log, build: func(*testState, BuildContext) Widget {
		return statefulLeaf("child", &log)
	}})
	parent := h.tree.root.child
	log = nil
	parent.child.state.(*loggingState).SetState(func() {})
	parent.state.(*loggingState).SetState(func() {})
	h.tree.Build()
	expectLog(t, &log, "parent:build", "child:update", "child:build")
}

func expectPanic(t *testing.T, want string, f func()) {
	t.Helper()
	defer func() {
		t.Helper()
		got, _ := recover().(string)
		if !strings.HasPrefix(got, "gowaku: ") || !strings.Contains(got, want) {
			t.Errorf("got panic %q, want one containing %q", got, want)
		}
	}()
	f()
}

func TestState_SetStateDuringBuildPanics(t *testing.T) {
	expectPanic(t, "during Build", func() {
		newHarness(stateful{build: func(s *testState, _ BuildContext) Widget {
			s.SetState(func() {})
			return leaf{}
		}})
	})
}

func TestState_SetStateInInitStateIsAllowed(*testing.T) {
	NewTree(initSetter{})
}

type initSetter struct{ StatefulBase }

type initSetterState struct{ StateBase }

func (initSetter) CreateState() State { return &initSetterState{} }

func (s *initSetterState) InitState()                { s.SetState(func() {}) }
func (s *initSetterState) Build(BuildContext) Widget { return leaf{} }

func TestState_SetStateAfterDisposePanics(t *testing.T) {
	h := newHarness(statefulLeaf("a", nil))
	s := h.tree.root.child.state.(*loggingState)
	h.set(leaf{})
	if s.Mounted() {
		t.Error("Mounted must report false after disposal")
	}
	expectPanic(t, "not in the tree", func() { s.SetState(func() {}) })
}

func TestTree_BuildReturningNilPanics(t *testing.T) {
	expectPanic(t, "nil Widget", func() {
		NewTree(stateful{build: func(*testState, BuildContext) Widget { return nil }})
	})
}

func TestTree_UnknownWidgetKindPanics(t *testing.T) {
	expectPanic(t, "core.notAWidget", func() { NewTree(notAWidget{}) })
}

type notAWidget struct{}

func (notAWidget) Key() Key { return nil }

// keyedList builds a list from names. A name with a leading "-" has no key.
func keyedList(log *[]string, names ...string) list {
	l := list{}
	for _, n := range names {
		if id, unkeyed := strings.CutPrefix(n, "-"); unkeyed {
			l.children = append(l.children, statefulLeaf(id, log))
		} else {
			l.children = append(l.children, Keyed(n, statefulLeaf(n, log)))
		}
	}
	return l
}

// states returns the identity of each child state, by position.
func states(h *harness) []State {
	var out []State
	for _, c := range h.tree.root.child.children {
		e := c
		for e.state == nil {
			e = e.child
		}
		out = append(out, e.state)
	}
	return out
}

func TestReconcile_Keyed(t *testing.T) {
	tests := []struct {
		name     string
		from, to []string
		// kept maps a position in to to a position in from, or -1 for new.
		kept []int
		// disposed lists the ids that must leave.
		disposed []string
	}{
		{"変化なし", []string{"a", "b", "c"}, []string{"a", "b", "c"}, []int{0, 1, 2}, nil},
		{"逆順", []string{"a", "b", "c"}, []string{"c", "b", "a"}, []int{2, 1, 0}, nil},
		{"先頭に追加", []string{"a", "b"}, []string{"x", "a", "b"}, []int{-1, 0, 1}, nil},
		{"末尾に追加", []string{"a", "b"}, []string{"a", "b", "x"}, []int{0, 1, -1}, nil},
		{"中央に追加", []string{"a", "b"}, []string{"a", "x", "b"}, []int{0, -1, 1}, nil},
		{"先頭を削除", []string{"a", "b", "c"}, []string{"b", "c"}, []int{1, 2}, []string{"a"}},
		{"末尾を削除", []string{"a", "b", "c"}, []string{"a", "b"}, []int{0, 1}, []string{"c"}},
		{"中央を削除", []string{"a", "b", "c"}, []string{"a", "c"}, []int{0, 2}, []string{"b"}},
		{"全部入れ替え", []string{"a", "b"}, []string{"x", "y"}, []int{-1, -1}, []string{"a", "b"}},
		{"空にする", []string{"a", "b"}, nil, nil, []string{"a", "b"}},
		{"空から作る", nil, []string{"a", "b"}, []int{-1, -1}, nil},
		{"中央を入れ替え", []string{"a", "b", "c", "d"}, []string{"a", "c", "b", "d"}, []int{0, 2, 1, 3}, nil},
		{"移動と追加と削除", []string{"a", "b", "c", "d"}, []string{"d", "x", "a"}, []int{3, -1, 0}, []string{"b", "c"}},
		{"キーなしの末尾は位置で一致", []string{"-a", "-b"}, []string{"-a", "-b", "-c"}, []int{0, 1, -1}, nil},
		{"キーなしの中央は作り直し", []string{"k", "-a", "m"}, []string{"m", "-a", "k"}, []int{2, -1, 0}, []string{"a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var log []string
			h := newHarness(keyedList(&log, tt.from...))
			before := states(h)
			log = nil
			h.set(keyedList(&log, tt.to...))

			after := states(h)
			if len(after) != len(tt.kept) {
				t.Fatalf("got %d children, want %d", len(after), len(tt.kept))
			}
			for i, from := range tt.kept {
				if from >= 0 && after[i] != before[from] {
					t.Errorf("position %d: state of %d was not kept", i, from)
				}
				if from < 0 {
					for _, b := range before {
						if after[i] == b {
							t.Errorf("position %d: must be new", i)
						}
					}
				}
			}
			var disposed []string
			for _, l := range log {
				if id, ok := strings.CutSuffix(l, ":dispose"); ok {
					disposed = append(disposed, id)
				}
			}
			if fmt.Sprint(sortedCopy(disposed)) != fmt.Sprint(sortedCopy(tt.disposed)) {
				t.Errorf("disposed: got %v, want %v", disposed, tt.disposed)
			}
			// The render objects follow the elements.
			ro := h.rootRO().(*renderList)
			if ro.ChildCount() != len(tt.to) {
				t.Errorf("render children: got %d, want %d", ro.ChildCount(), len(tt.to))
			}
			for i, c := range h.tree.root.child.children {
				if ro.ChildAt(i) != renderObjectOf(c) {
					t.Errorf("render child %d out of order", i)
				}
			}
		})
	}
}

func sortedCopy(s []string) []string {
	out := append([]string(nil), s...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func TestReconcile_SameKeyDifferentTypeIsReplaced(t *testing.T) {
	h := newHarness(list{children: []Widget{Keyed("k", leaf{}), leaf{}}})
	before := h.tree.root.child.children[0]
	h.set(list{children: []Widget{leaf{}, Keyed("k", box{})}})
	if h.tree.root.child.children[1] != before {
		// The keyed wrapper survives; its child changed type below it.
		t.Error("keyed wrapper must move")
	}
	if _, ok := h.rootRO().(*renderList).children[1].(*renderBox); !ok {
		t.Errorf("got %T", h.rootRO().(*renderList).children[1])
	}
}

func TestReconcile_DuplicateKeysPanic(t *testing.T) {
	h := newHarness(list{children: []Widget{leaf{}, Keyed("k", leaf{}), Keyed("k", leaf{})}})
	expectPanic(t, "share the key", func() { h.set(list{children: []Widget{box{}}}) })
}

func TestKeyed_RejectsBadKeys(t *testing.T) {
	expectPanic(t, "not comparable", func() { Keyed([]int{1}, leaf{}) })
	expectPanic(t, "nil key", func() { Keyed(nil, leaf{}) })
}

func TestInherited_DependentSeesNewValue(t *testing.T) {
	var log []string
	var seen []int
	reader := stateful{id: "reader", log: &log, build: func(_ *testState, ctx BuildContext) Widget {
		v, ok := DependOn[int](ctx)
		if !ok {
			t.Error("value not found")
		}
		seen = append(seen, v)
		return leaf{}
	}}
	// The reader sits below a component that does not rebuild.
	middle := stateful{build: func(*testState, BuildContext) Widget { return reader }}
	shell := func(v int) Widget { return Inherited[int]{Value: v, Child: middle} }

	h := newHarness(shell(1))
	h.set(shell(2))
	if got := seen[len(seen)-1]; got != 2 {
		t.Errorf("got %d, want 2", got)
	}
}

func TestInherited_MissingReportsFalse(t *testing.T) {
	NewTree(stateful{build: func(_ *testState, ctx2 BuildContext) Widget {
		if _, ok := DependOn[string](ctx2); ok {
			t.Error("got true, want false")
		}
		return leaf{}
	}})
}

func TestInherited_NearestOfTheRightTypeWins(t *testing.T) {
	var got string
	reader := stateful{build: func(_ *testState, ctx BuildContext) Widget {
		got, _ = DependOn[string](ctx)
		return leaf{}
	}}
	NewTree(Inherited[string]{Value: "far", Child: Inherited[int]{Value: 1, Child: Inherited[string]{Value: "near", Child: reader}}})
	if got != "near" {
		t.Errorf("got %q", got)
	}
}

func TestInherited_DependentInSeparateBranchRebuildsWithoutParent(t *testing.T) {
	var builds int
	reader := stateful{build: func(_ *testState, ctx BuildContext) Widget {
		builds++
		DependOn[int](ctx)
		return leaf{}
	}}
	// A const-like subtree is still rebuilt by reconciling, so the dependent
	// must not be built a second time by its own dirty flag.
	h := newHarness(Inherited[int]{Value: 1, Child: reader})
	builds = 0
	h.set(Inherited[int]{Value: 2, Child: reader})
	if builds != 1 {
		t.Errorf("got %d builds, want 1", builds)
	}
}

func TestInherited_DisposedDependentIsForgotten(t *testing.T) {
	reader := stateful{build: func(_ *testState, ctx BuildContext) Widget {
		DependOn[int](ctx)
		return leaf{}
	}}
	h := newHarness(Inherited[int]{Value: 1, Child: reader})
	inherited := h.tree.root.child
	h.set(Inherited[int]{Value: 1, Child: leaf{}})
	if len(inherited.dependents) != 0 {
		t.Errorf("got %d dependents, want 0", len(inherited.dependents))
	}
}

func TestLayout_StopsAtRelayoutBoundary(t *testing.T) {
	h := newHarness(box{tight: true, size: geom.Size{W: 50, H: 50}, child: box{child: leaf{w: 10, h: 10}}})
	h.layout()
	outer := h.rootRO().(*renderBox)
	inner := outer.child.(*renderBox)
	target := inner.child.(*renderLeaf)

	h.set(box{tight: true, size: geom.Size{W: 50, H: 50}, child: box{child: leaf{w: 20, h: 20}}})
	if !h.tree.NeedsFrame() {
		t.Fatal("a size change must ask for a frame")
	}
	h.layout()
	if outer.layouts != 1 {
		t.Errorf("outer laid out %d times, want 1: its child has tight constraints", outer.layouts)
	}
	if inner.layouts != 2 || target.layouts != 2 {
		t.Errorf("inner %d, target %d, want 2 each", inner.layouts, target.layouts)
	}
	if got, want := target.Size(), (geom.Size{W: 20, H: 20}); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLayout_CleanChildIsNotLaidOutAgain(t *testing.T) {
	h := newHarness(list{children: []Widget{leaf{w: 1, h: 1}, leaf{w: 2, h: 2}}})
	h.layout()
	parent := h.rootRO().(*renderList)
	first := parent.children[0].(*renderLeaf)
	second := parent.children[1].(*renderLeaf)

	second.MarkNeedsLayout()
	h.layout()
	if parent.layouts != 2 || second.layouts != 2 {
		t.Errorf("parent %d, second %d, want 2 each", parent.layouts, second.layouts)
	}
	if first.layouts != 1 {
		t.Errorf("first laid out %d times, want 1", first.layouts)
	}
}

func TestLayout_NewConstraintsReachTheRoot(t *testing.T) {
	h := newHarness(leaf{w: 500, h: 500})
	h.tree.Layout(geom.Tight(geom.Size{W: 100, H: 100}))
	h.tree.Layout(geom.Tight(geom.Size{W: 200, H: 150}))
	if got, want := h.rootRO().Size(), (geom.Size{W: 200, H: 150}); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLayout_RootSwapIsLaidOut(t *testing.T) {
	h := newHarness(leaf{w: 1, h: 1})
	h.layout()
	h.set(box{})
	h.layout()
	if got := h.rootRO().(*renderBox).layouts; got != 1 {
		t.Errorf("got %d layouts, want 1", got)
	}
}

func TestLayout_NoWorkMeansNoFrame(t *testing.T) {
	h := newHarness(leaf{w: 1, h: 1})
	h.tree.Build()
	h.layout()
	var cv paint.Canvas
	cv.Reset()
	h.tree.Paint(&cv)
	if h.tree.NeedsFrame() {
		t.Error("a settled tree must not ask for a frame")
	}
}

func TestTree_MarkNeedsPaintAsksForAFrameWithoutLayout(t *testing.T) {
	h := newHarness(leaf{w: 1, h: 1})
	h.layout()
	var cv paint.Canvas
	cv.Reset()
	h.tree.Paint(&cv)
	r := h.rootRO().(*renderLeaf)
	r.MarkNeedsPaint()
	if !h.tree.NeedsFrame() {
		t.Error("MarkNeedsPaint must ask for a frame")
	}
	h.layout()
	if r.layouts != 1 {
		t.Errorf("got %d layouts, want 1", r.layouts)
	}
}

func TestDetached_MarksAreIgnored(*testing.T) {
	h := newHarness(leaf{w: 1, h: 1})
	h.layout()
	old := h.rootRO()
	h.set(leaf{w: 2, h: 2})
	h.set(box{})
	old.MarkNeedsLayout() // must not panic on the cleared parent
	old.MarkNeedsPaint()
}

func TestHitTest_DeepestFirstAndTopmostSibling(t *testing.T) {
	h := newHarness(list{children: []Widget{leaf{name: "a", w: 10, h: 10}, leaf{name: "b", w: 10, h: 10}}})
	h.layout()
	path := h.tree.HitTest(geom.Offset{X: 5, Y: 15}).Path
	if len(path) != 2 {
		t.Fatalf("got %d objects, want 2", len(path))
	}
	if got := path[0].(*renderLeaf).name; got != "b" {
		t.Errorf("deepest: got %q, want b", got)
	}
	if path[1] != h.rootRO() {
		t.Error("root must come last")
	}
}

func TestHitTest_OutsideIsEmpty(t *testing.T) {
	h := newHarness(leaf{w: 10, h: 10})
	h.layout()
	if got := h.tree.HitTest(geom.Offset{X: 500, Y: 500}).Path; len(got) != 0 {
		t.Errorf("got %d objects, want 0", len(got))
	}
}

// labeled is a render object that speaks for itself.
type labeled struct{ leaf }

type renderLabeled struct{ renderLeaf }

func (l labeled) CreateRenderObject() RenderObject {
	return &renderLabeled{renderLeaf{name: l.name, w: l.w, h: l.h}}
}

func (l labeled) UpdateRenderObject(RenderObject) {}

func (r *renderLabeled) Semantics(sb *SemanticsBuilder) {
	sb.Open(semantics.Node{Role: semantics.RoleButton, Label: r.name}, r.Size())
	sb.Close()
}

func TestSemantics_FindsNodesThroughSilentParents(t *testing.T) {
	h := newHarness(list{children: []Widget{leaf{w: 1, h: 7}, labeled{leaf{name: "ok", w: 30, h: 10}}}})
	h.layout()
	root := h.tree.Semantics()
	if len(root.Children) != 1 {
		t.Fatalf("got %d nodes, want 1", len(root.Children))
	}
	n := root.Children[0]
	if n.Role != semantics.RoleButton || n.Label != "ok" {
		t.Errorf("got %+v", n)
	}
	if want := (geom.Rect{Y: 7, W: 30, H: 10}); n.Rect != want {
		t.Errorf("rect: got %v, want %v", n.Rect, want)
	}
}

func TestDispose_RunsChildrenFirst(t *testing.T) {
	var log []string
	root := stateful{id: "parent", log: &log, build: func(*testState, BuildContext) Widget {
		return statefulLeaf("child", &log)
	}}
	tree := NewTree(root)
	log = nil
	tree.Dispose()
	expectLog(t, &log, "child:dispose", "parent:dispose")
}

func wideTree(n int) Widget {
	children := make([]Widget, n)
	for i := range children {
		children[i] = leaf{w: 10, h: 10}
	}
	return list{children: children}
}

// deepTree nests boxes in lists, so that reconciling recurses.
func deepTree(n int) Widget {
	children := make([]Widget, n/10)
	for i := range children {
		var w Widget = leaf{w: 10, h: 10}
		for range 9 {
			w = box{child: w}
		}
		children[i] = w
	}
	return list{children: children}
}

func TestReconcile_DoesNotAllocate(t *testing.T) {
	for name, w := range map[string]Widget{"wide": wideTree(1000), "deep": deepTree(1000)} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(w)
			allocs := testing.AllocsPerRun(20, func() {
				h.state.SetState(func() {})
				h.tree.Build()
			})
			// The closure of SetState is the test's own and does not escape.
			if allocs != 0 {
				t.Errorf("got %v allocs, want 0", allocs)
			}
		})
	}
}

func TestReconcile_KeyedReorderDoesNotAllocate(t *testing.T) {
	var a, b []Widget
	for i := range 100 {
		a = append(a, Keyed(i, leaf{}))
	}
	for i := range 100 {
		b = append(b, a[99-i])
	}
	h := newHarness(list{children: a})
	la, lb := Widget(list{children: a}), Widget(list{children: b})
	flip := false
	allocs := testing.AllocsPerRun(20, func() {
		flip = !flip
		if flip {
			h.current = lb
		} else {
			h.current = la
		}
		h.state.SetState(func() {})
		h.tree.Build()
	})
	if allocs != 0 {
		t.Errorf("got %v allocs, want 0", allocs)
	}
}

func benchmarkRebuild(b *testing.B, w Widget) {
	h := newHarness(w)
	h.layout()
	b.ReportAllocs()
	for b.Loop() {
		h.state.SetState(func() {})
		h.tree.Build()
	}
}

func BenchmarkReconcile_10kNodes(b *testing.B)     { benchmarkRebuild(b, wideTree(10000)) }
func BenchmarkReconcile_10kNodesDeep(b *testing.B) { benchmarkRebuild(b, deepTree(10000)) }

func BenchmarkLayout_10kNodes(b *testing.B) {
	h := newHarness(wideTree(10000))
	h.layout()
	ro := h.rootRO().(*renderList)
	b.ReportAllocs()
	for b.Loop() {
		for _, c := range ro.children {
			c.base().needsLayout = true
		}
		ro.needsLayout = true
		h.layout()
	}
}
