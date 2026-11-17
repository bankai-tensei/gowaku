package collection

import (
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

// reorderScreen is the window; the list of a reorderRig is 300 by 200 at its
// top left, five items of 40 in view.
var reorderScreen = geom.Size{W: 400, H: 300}

// reorderRig drives a ReorderableList over items it reorders itself, as an
// application would.
type reorderRig struct {
	t       *testing.T
	tree    *core.Tree
	swap    func(core.Widget)
	items   []string
	moves   [][2]int
	still   bool
	extra   func(*ReorderableList)
	heights func(items []string) func(i int) float32
	now     time.Duration
	clock   uint32
	refuses bool
}

func newReorderRig(t *testing.T, n int, extra func(*ReorderableList)) *reorderRig {
	t.Helper()
	r := &reorderRig{t: t, extra: extra, now: time.Second}
	for i := range n {
		r.items = append(r.items, "item"+strconv.Itoa(i))
	}
	r.tree, r.swap = wtest.SwapTree(t, r.widget())
	r.frame()
	return r
}

func (r *reorderRig) widget() core.Widget {
	items := slices.Clone(r.items)
	l := ReorderableList{
		ItemCount: len(items), ItemExtent: 40, Label: "Todo",
		LabelOf: func(i int) string { return items[i] },
		OnReorder: func(from, to int) {
			r.moves = append(r.moves, [2]int{from, to})
			if r.refuses {
				return
			}
			item := r.items[from]
			r.items = slices.Insert(slices.Delete(r.items, from, from+1), to, item)
			r.swap(r.widget())
		},
		ItemBuilder: func(i int) core.Widget { return basic.Text{Content: items[i]} },
	}
	if r.heights != nil {
		l.ItemExtentOf = r.heights(items)
	}
	if r.extra != nil {
		r.extra(&l)
	}
	var w core.Widget = basic.Align{Alignment: layout.AlignTopLeft, Child: basic.SizedBox{Width: 300, Height: 200, Child: l}}
	if r.still {
		w = basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1, ReduceMotion: true}, Child: w}
	}
	return w
}

func (r *reorderRig) refresh() {
	r.swap(r.widget())
	r.frame()
}

func (r *reorderRig) frame() { wtest.Ops(r.tree, reorderScreen) }

// settle plays frames until the items rest.
func (r *reorderRig) settle() { wtest.Settled(r.t, r.tree, reorderScreen, &r.now) }

func (r *reorderRig) Key(k input.KeyCode, m input.Modifiers) {
	r.tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: m})
	r.frame()
}

func (r *reorderRig) nodes() []*semantics.Node {
	return wtest.RolesOf(r.tree.Semantics(), semantics.RoleListItem)
}

func (r *reorderRig) node(label string) *semantics.Node {
	r.t.Helper()
	for _, n := range r.nodes() {
		if n.Label == label {
			return n
		}
	}
	r.t.Fatalf("no item %q", label)
	return nil
}

// y returns the top of the item with label in the window.
func (r *reorderRig) y(label string) float32 { return r.node(label).Rect.Y }

func (r *reorderRig) pointer(kind core.PointerKind, pos geom.Offset) {
	r.clock += 1000
	r.tree.DispatchPointer(core.PointerEvent{Kind: kind, Pos: pos, Time: r.clock})
}

// lift presses the item with label and drags it by dy, without dropping it.
func (r *reorderRig) lift(label string, dy float32) geom.Offset {
	r.t.Helper()
	start := geom.Offset{X: 150, Y: r.y(label) + 20}
	r.pointer(core.PointerDown, start)
	r.pointer(core.PointerMove, start.Add(geom.Offset{Y: 6}))
	r.frame()
	end := start.Add(geom.Offset{Y: dy})
	r.pointer(core.PointerMove, end)
	r.frame()
	return end
}

func (r *reorderRig) drop(at geom.Offset) {
	r.pointer(core.PointerUp, at)
	r.frame()
}

func (r *reorderRig) wantItems(want ...string) {
	r.t.Helper()
	if !slices.Equal(r.items, want) {
		r.t.Errorf("items %q, want %q", r.items, want)
	}
}

func (r *reorderRig) wantMoves(want ...[2]int) {
	r.t.Helper()
	if !slices.Equal(r.moves, want) {
		r.t.Errorf("moves %v, want %v", r.moves, want)
	}
}

func (r *reorderRig) announced() string {
	for _, n := range wtest.RolesOf(r.tree.Semantics(), semantics.RoleText) {
		if n.Live == semantics.LivePolite {
			return n.Label
		}
	}
	return ""
}

func reorderBodyOf(t testing.TB, tree *core.Tree) *renderReorderBody {
	t.Helper()
	for _, ro := range tree.HitTest(geom.Offset{X: 2, Y: 2}).Path {
		if b, ok := ro.(*renderReorderBody); ok {
			return b
		}
	}
	t.Fatal("no reorderable body")
	return nil
}

func TestReorderableList_ZeroValueAndEmpty(t *testing.T) {
	tree := wtest.TextTree(t, basic.SizedBox{Width: 200, Height: 100, Child: ReorderableList{}})
	wtest.Ops(tree, reorderScreen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyDown, Modifiers: input.ModControl | input.ModShift})
	wtest.Ops(tree, reorderScreen)
	if n := len(wtest.RolesOf(tree.Semantics(), semantics.RoleListItem)); n != 0 {
		t.Errorf("got %d items", n)
	}
	r := newReorderRig(t, 0, nil)
	r.Key(input.KeyTab, 0)
	r.Key(input.KeyEnd, input.ModControl|input.ModShift)
	r.wantMoves()
}

func TestReorderableList_InColumnPanics(t *testing.T) {
	wtest.ExpectPanic(t, "collection.ReorderableList has an unbounded height", func() {
		wtest.Rects(t, basic.Column{Children: []core.Widget{ReorderableList{ItemCount: 2}}}, wtest.Page)
	})
}

func TestReorderableList_Semantics(t *testing.T) {
	r := newReorderRig(t, 10, nil)
	lists := wtest.RolesOf(r.tree.Semantics(), semantics.RoleList)
	if len(lists) != 1 || lists[0].Label != "Todo" {
		t.Fatalf("lists %+v", lists)
	}
	n := r.node("item2")
	if n.PosInSet != 3 || n.SetSize != 10 || n.Rect.Y != 80 {
		t.Errorf("item2 %+v", n)
	}
	focusable := 0
	for _, n := range r.nodes() {
		if n.State&semantics.Focusable != 0 {
			focusable++
		}
	}
	if focusable != 1 {
		t.Errorf("one item must take Tab, got %d", focusable)
	}
	if got := len(r.nodes()); got > 5 {
		t.Errorf("%d items speak, want those in view", got)
	}
}

func TestReorderableList_KeyboardFocus(t *testing.T) {
	r := newReorderRig(t, 20, nil)
	r.Key(input.KeyTab, 0)
	steps := []struct {
		name  string
		key   input.KeyCode
		focus string
	}{
		{"上端の上", input.KeyUp, "item0"},
		{"下", input.KeyDown, "item1"},
		{"PageDown", input.KeyPageDown, "item5"},
		{"End", input.KeyEnd, "item19"},
		{"下端の下", input.KeyDown, "item19"},
		{"PageUp", input.KeyPageUp, "item15"},
		{"Home", input.KeyHome, "item0"},
	}
	for _, s := range steps {
		r.Key(s.key, 0)
		if got := wtest.FocusedLabel(r.tree); got != s.focus {
			t.Errorf("%s: focus on %q, want %q", s.name, got, s.focus)
		}
	}
}

func TestReorderableList_KeyboardMove(t *testing.T) {
	r := newReorderRig(t, 10, nil)
	r.Key(input.KeyTab, 0)
	r.Key(input.KeyDown, 0)
	r.Key(input.KeyDown, input.ModControl|input.ModShift)
	r.wantMoves([2]int{1, 2})
	r.wantItems("item0", "item2", "item1", "item3", "item4", "item5", "item6", "item7", "item8", "item9")
	if got := wtest.FocusedLabel(r.tree); got != "item1" {
		t.Errorf("the moved item must keep the focus, got %q", got)
	}
	if got := r.announced(); got != "Moved item1 to position 3 of 10" {
		t.Errorf("announced %q", got)
	}
	// It slides from its old place to the new one.
	if y := r.y("item1"); y != 40 {
		t.Errorf("item1 starts at %v, want its old place 40", y)
	}
	r.settle()
	if y := r.y("item1"); y != 80 {
		t.Errorf("item1 rests at %v, want 80", y)
	}
	if y := r.y("item2"); y != 40 {
		t.Errorf("item2 rests at %v, want 40", y)
	}
	r.Key(input.KeyUp, input.ModControl|input.ModShift)
	r.Key(input.KeyUp, input.ModControl|input.ModShift)
	r.Key(input.KeyUp, input.ModControl|input.ModShift)
	r.wantMoves([2]int{1, 2}, [2]int{2, 1}, [2]int{1, 0})
	r.Key(input.KeyEnd, input.ModControl|input.ModShift)
	r.wantMoves([2]int{1, 2}, [2]int{2, 1}, [2]int{1, 0}, [2]int{0, 9})
	r.wantItems("item0", "item2", "item3", "item4", "item5", "item6", "item7", "item8", "item9", "item1")
	r.settle()
	if got := wtest.FocusedLabel(r.tree); got != "item1" {
		t.Errorf("the moved item must keep the focus at the end, got %q", got)
	}
	r.Key(input.KeyHome, input.ModControl|input.ModShift)
	r.wantItems("item1", "item0", "item2", "item3", "item4", "item5", "item6", "item7", "item8", "item9")
	r.settle()
	if y := r.y("item1"); y != 0 {
		t.Errorf("the list must show the top again, item1 at %v", y)
	}
}

func TestReorderableList_PointerMove(t *testing.T) {
	r := newReorderRig(t, 10, nil)
	end := r.lift("item0", 90)
	// The middle of item0 is at 110, over item2: item1 and item2 make room.
	r.settle()
	if y1, y2, y3 := r.y("item1"), r.y("item2"), r.y("item3"); y1 != 0 || y2 != 40 || y3 != 120 {
		t.Errorf("the items between must slide up: %v %v %v", y1, y2, y3)
	}
	if y := r.y("item0"); y != 90 {
		t.Errorf("the dragged item follows the pointer, at %v", y)
	}
	r.drop(end)
	r.wantMoves([2]int{0, 2})
	r.wantItems("item1", "item2", "item0", "item3", "item4", "item5", "item6", "item7", "item8", "item9")
	// The dropped item slides from where it was let go.
	if y := r.y("item0"); y != 90 {
		t.Errorf("item0 starts at %v, where it was dropped", y)
	}
	r.settle()
	if y := r.y("item0"); y != 80 {
		t.Errorf("item0 rests at %v, want 80", y)
	}
	if got := wtest.FocusedLabel(r.tree); got != "item0" {
		t.Errorf("focus on %q", got)
	}

	// Upward too: the middle of item4 at 70 is over item2, in the second
	// place.
	end = r.lift("item4", -110)
	r.drop(end)
	r.wantMoves([2]int{0, 2}, [2]int{4, 1})
	r.wantItems("item1", "item4", "item2", "item0", "item3", "item5", "item6", "item7", "item8", "item9")
	r.settle()
	// A drop on its own place moves nothing, and the item goes back.
	end = r.lift("item3", 10)
	r.drop(end)
	r.wantMoves([2]int{0, 2}, [2]int{4, 1})
	r.settle()
	if y := r.y("item3"); y != 160 {
		t.Errorf("item3 rests at %v", y)
	}
}

func TestReorderableList_PressWithoutDragOnlyFocuses(t *testing.T) {
	r := newReorderRig(t, 10, nil)
	pos := geom.Offset{X: 150, Y: r.y("item3") + 20}
	r.pointer(core.PointerDown, pos)
	r.pointer(core.PointerMove, pos.Add(geom.Offset{Y: 2}))
	r.pointer(core.PointerUp, pos)
	r.frame()
	r.wantMoves()
	if got := wtest.FocusedLabel(r.tree); got != "item3" {
		t.Errorf("focus on %q", got)
	}
}

func TestReorderableList_OnTap(t *testing.T) {
	var taps []int
	r := newReorderRig(t, 10, func(l *ReorderableList) { l.OnTap = func(i int) { taps = append(taps, i) } })
	pos := geom.Offset{X: 150, Y: r.y("item3") + 20}
	r.pointer(core.PointerDown, pos)
	r.pointer(core.PointerMove, pos.Add(geom.Offset{Y: 2}))
	r.pointer(core.PointerUp, pos)
	r.frame()
	if !slices.Equal(taps, []int{3}) {
		t.Fatalf("taps %v after a press on item3", taps)
	}
	r.drop(r.lift("item1", 90))
	if len(taps) != 1 || len(r.moves) != 1 {
		t.Errorf("a drag tapped: taps %v, moves %v", taps, r.moves)
	}
	r.settle()
	r.Key(input.KeyEnter, 0)
	r.Key(input.KeySpace, 0)
	// The dropped item1 is now at index 3 and keeps the focus.
	if got := wtest.FocusedLabel(r.tree); !slices.Equal(taps, []int{3, 3, 3}) || got != "item1" {
		t.Errorf("Enter and Space on %q: taps %v", got, taps)
	}
	if n := r.node("item0"); n.Actions&semantics.ActionTap == 0 {
		t.Error("an item of a list with OnTap must be pressable by assistive technology")
	}
}

func TestReorderableList_OnTapNeedsTheReleaseOnTheItem(t *testing.T) {
	var taps []int
	r := newReorderRig(t, 10, func(l *ReorderableList) {
		l.OnReorder = nil
		l.OnTap = func(i int) { taps = append(taps, i) }
	})
	pos := geom.Offset{X: 150, Y: r.y("item3") + 20}
	r.pointer(core.PointerDown, pos)
	r.pointer(core.PointerMove, pos.Add(geom.Offset{Y: 40}))
	r.pointer(core.PointerUp, pos.Add(geom.Offset{Y: 40}))
	r.frame()
	if len(taps) != 0 {
		t.Errorf("a release over item4 tapped %v", taps)
	}
}

func TestReorderableList_EscapeCancels(t *testing.T) {
	r := newReorderRig(t, 10, nil)
	end := r.lift("item1", 100)
	r.Key(input.KeyEscape, 0)
	r.drop(end)
	r.wantMoves()
	r.settle()
	for i := range 5 {
		if y := r.y("item" + strconv.Itoa(i)); y != float32(i*40) {
			t.Errorf("item%d back at %v", i, y)
		}
	}
	// A cancel of the pointer puts it back too.
	r.lift("item1", 100)
	r.pointer(core.PointerCancel, geom.Offset{})
	r.frame()
	r.settle()
	r.wantMoves()
	if y := r.y("item2"); y != 80 {
		t.Errorf("item2 back at %v", y)
	}
}

func TestReorderableList_RebuildWithFewerItemsWhileDragging(t *testing.T) {
	r := newReorderRig(t, 10, nil)
	end := r.lift("item4", 60)
	r.items = r.items[:3]
	r.refresh()
	r.drop(end)
	r.wantMoves()
	r.settle()
	if n := len(r.nodes()); n != 3 {
		t.Errorf("%d items, want 3", n)
	}
	// The item dragged stays, its target past the end comes back.
	r = newReorderRig(t, 10, nil)
	end = r.lift("item1", 150)
	r.items = r.items[:3]
	r.refresh()
	r.drop(end)
	r.wantMoves([2]int{1, 2})
	r.settle()
	r.Key(input.KeyUp, 0)
	r.Key(input.KeyDown, input.ModControl|input.ModShift)
}

func TestReorderableList_ShrinkKeepsTheFocus(t *testing.T) {
	r := newReorderRig(t, 30, nil)
	r.Key(input.KeyTab, 0)
	r.Key(input.KeyEnd, 0)
	r.items = r.items[:4]
	r.refresh()
	r.frame()
	if got := wtest.FocusedLabel(r.tree); got != "item3" {
		t.Errorf("focus on %q, want the last item left", got)
	}
	r.Key(input.KeyUp, input.ModControl|input.ModShift)
	r.wantMoves([2]int{3, 2})
}

func TestReorderableList_DragScrollsNearTheEdge(t *testing.T) {
	r := newReorderRig(t, 50, nil)
	start := geom.Offset{X: 150, Y: r.y("item1") + 20}
	r.pointer(core.PointerDown, start)
	r.pointer(core.PointerMove, start.Add(geom.Offset{Y: 6}))
	r.frame()
	edge := geom.Offset{X: 150, Y: 196}
	r.pointer(core.PointerMove, edge)
	r.frame()
	wtest.Advance(r.tree, reorderScreen, &r.now, 500*time.Millisecond)
	body := reorderBodyOf(t, r.tree)
	if off := body.host.Offset.Value(); off < 100 {
		t.Errorf("the view must scroll toward the pointer, offset %v", off)
	}
	r.drop(edge)
	if len(r.moves) != 1 || r.moves[0][0] != 1 || r.moves[0][1] < 6 {
		t.Errorf("moves %v", r.moves)
	}
	// The clock stops once the drag is over and the items rest.
	r.settle()
	if r.tree.Ticking() {
		t.Error("nothing must tick at rest")
	}
}

func TestReorderableList_ReduceMotion(t *testing.T) {
	r := &reorderRig{t: t, still: true, now: time.Second}
	for i := range 10 {
		r.items = append(r.items, "item"+strconv.Itoa(i))
	}
	r.tree, r.swap = wtest.SwapTree(t, r.widget())
	r.frame()
	r.lift("item0", 90)
	if y := r.y("item1"); y != 0 {
		t.Errorf("with ReduceMotion item1 makes room at once, at %v", y)
	}
	r.Key(input.KeyEscape, 0)
	if y := r.y("item1"); y != 40 {
		t.Errorf("with ReduceMotion item1 goes back at once, at %v", y)
	}
	r.Key(input.KeyDown, input.ModControl|input.ModShift)
	if y := r.y("item0"); y != 40 {
		t.Errorf("with ReduceMotion the moved item is at its place at once, at %v", y)
	}
	if r.tree.Ticking() {
		t.Error("nothing must slide with ReduceMotion")
	}
}

func TestReorderableList_WithoutOnReorder(t *testing.T) {
	r := newReorderRig(t, 10, func(l *ReorderableList) { l.OnReorder = nil })
	end := r.lift("item0", 90)
	if y := r.y("item1"); y != 40 {
		t.Errorf("nothing lifts without OnReorder, item1 at %v", y)
	}
	r.drop(end)
	r.Key(input.KeyDown, input.ModControl|input.ModShift)
	r.wantMoves()
}

func TestReorderableList_VaryingHeights(t *testing.T) {
	heights := func(items []string) func(i int) float32 {
		return func(i int) float32 {
			if items[i] == "item1" {
				return 60
			}
			return 30
		}
	}
	r := &reorderRig{t: t, heights: heights, now: time.Second}
	for i := range 6 {
		r.items = append(r.items, "item"+strconv.Itoa(i))
	}
	r.tree, r.swap = wtest.SwapTree(t, r.widget())
	r.frame()
	if y := r.y("item2"); y != 90 {
		t.Fatalf("item2 at %v, want 90", y)
	}
	r.Key(input.KeyTab, 0)
	r.Key(input.KeyDown, 0)
	r.Key(input.KeyDown, input.ModControl|input.ModShift)
	r.wantItems("item0", "item2", "item1", "item3", "item4", "item5")
	// item1 starts where it was, item2 where it was.
	if y1, y2 := r.y("item1"), r.y("item2"); y1 != 30 || y2 != 90 {
		t.Errorf("start at %v and %v, want 30 and 90", y1, y2)
	}
	r.settle()
	if y1, y2, y3 := r.y("item1"), r.y("item2"), r.y("item3"); y1 != 60 || y2 != 30 || y3 != 120 {
		t.Errorf("rest at %v %v %v, want 60 30 120", y1, y2, y3)
	}
	// Up again: the passed item starts below.
	r.Key(input.KeyUp, input.ModControl|input.ModShift)
	if y1, y2 := r.y("item1"), r.y("item2"); y1 != 60 || y2 != 30 {
		t.Errorf("start at %v and %v, want 60 and 30", y1, y2)
	}
	r.settle()
	if y1, y2 := r.y("item1"), r.y("item2"); y1 != 30 || y2 != 90 {
		t.Errorf("rest at %v and %v, want 30 and 90", y1, y2)
	}
}

func TestReorderableList_MovedToStrings(t *testing.T) {
	en, ja := basic.EnglishStrings(), basic.JapaneseStrings()
	cases := []struct{ got, want string }{
		{en.MovedTo("Milk", 2, 5), "Moved Milk to position 2 of 5"},
		{en.MovedTo("", 2, 5), "Moved to position 2 of 5"},
		{ja.MovedTo("牛乳", 2, 5), "牛乳を 5 件中 2 番目に移動しました"},
		{ja.MovedTo("", 2, 5), "5 件中 2 番目に移動しました"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func TestReorderableList_DragFramesDoNotAllocate(t *testing.T) {
	r := newReorderRig(t, 10, nil)
	end := r.lift("item0", 90)
	s := reorderBodyOf(t, r.tree).host
	var cv paint.Canvas
	k := 0
	frame := func() {
		k++
		dy := float32(k%40) - 20
		// The move goes to the list as the item passes it on; core's delivery
		// of the event is not part of the list.
		s.pointer(core.PointerMove, end.Add(geom.Offset{Y: dy}))
		r.now += 8 * time.Millisecond
		r.tree.Tick(r.now)
		r.tree.Build()
		r.tree.Layout(geom.Tight(reorderScreen))
		cv.Reset()
		r.tree.Paint(&cv)
	}
	for range 100 {
		frame()
	}
	// A run is one whole swing of the pointer, 40 frames, so an allocation
	// anywhere in the swing shows as one per run. Several runs are averaged,
	// as the runtime allocates now and then on its own: one stray allocation
	// in 200 frames failed this test under load.
	allocs := testing.AllocsPerRun(5, func() {
		for range 40 {
			frame()
		}
	})
	if allocs != 0 {
		t.Errorf("got %v allocs per swing of a drag", allocs)
	}
}

// reorderFrames returns a function that plays one frame of a small wheel
// motion back and forth over a list of 100,000 items, warmed up.
func reorderFrames(t testing.TB) func() {
	l := ReorderableList{ItemCount: 100000, ItemExtent: 40, OnReorder: func(int, int) {}, ItemBuilder: func(int) core.Widget {
		return basic.Container{Color: wtest.Red}
	}}
	tree := wtest.TextTree(t, l)
	size := geom.Size{W: 400, H: 600}
	wtest.Ops(tree, size)
	s := reorderBodyOf(t, tree).host
	s.Jump(50000*40 + 20)
	wtest.Ops(tree, size)
	var cv paint.Canvas
	now, dir := time.Hour, float32(1)
	frame := func() {
		if !tree.Ticking() {
			dir = -dir
			s.Wheel(dir * 0.03)
		}
		now += 8 * time.Millisecond
		tree.Tick(now)
		tree.Build()
		tree.Layout(geom.Tight(size))
		cv.Reset()
		tree.Paint(&cv)
	}
	for range 200 {
		frame()
	}
	return frame
}

// BenchmarkReorderableList_ScrollFrame_100k is one frame of scrolling a
// reorderable list of 100,000 items while the same items stay in range.
func BenchmarkReorderableList_ScrollFrame_100k(b *testing.B) {
	frame := reorderFrames(b)
	b.ReportAllocs()
	for b.Loop() {
		frame()
	}
}

func TestReorderableList_ScrollingDoesNotAllocate(t *testing.T) {
	frame := reorderFrames(t)
	allocs := testing.AllocsPerRun(1, func() {
		for range 1000 {
			frame()
		}
	})
	if allocs != 0 {
		t.Errorf("got %v allocs in 1000 frames", allocs)
	}
}
