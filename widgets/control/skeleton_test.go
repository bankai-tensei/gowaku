package control

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

var skeletonScreen = geom.Size{W: 400, H: 300}

// skeletonCard is a card of placeholders, as a list shows while it loads.
func skeletonCard() core.Widget {
	return SkeletonGroup{Child: basic.SizedBox{Width: 300, Child: basic.Row{Cross: layout.CrossStart, Spacing: 12, Children: []core.Widget{
		Skeleton{Shape: SkeletonCircle},
		basic.Expanded{Child: SkeletonText{Lines: 3}},
	}}}}
}

func skeletonTree(t *testing.T, reduce bool, w core.Widget) *core.Tree {
	t.Helper()
	tree := core.NewTree(testutil.WithShaper(testutil.NewShaper(t), basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1, ReduceMotion: reduce, Locale: language.Japanese}, Child: basic.Localizations{Child: basic.Align{
		Alignment: layout.AlignTopLeft, Child: w,
	}}}))
	t.Cleanup(tree.Dispose)
	return tree
}

func TestSkeleton_Shapes(t *testing.T) {
	cases := []struct {
		name string
		w    core.Widget
		want geom.RRect
	}{
		{"ゼロ値は幅いっぱいの線", Skeleton{}, geom.RRectAll(geom.Rect{W: 200, H: 12}, 6)},
		{"ブロック", Skeleton{Shape: SkeletonBlock, Height: 80}, geom.RRectAll(geom.Rect{W: 200, H: 80}, 8)},
		{"円", Skeleton{Shape: SkeletonCircle}, geom.RRectAll(geom.Rect{W: 32, H: 32}, 16)},
		{"大きさ指定の円", Skeleton{Shape: SkeletonCircle, Width: 48}, geom.RRectAll(geom.Rect{W: 48, H: 48}, 24)},
		{"角の指定", Skeleton{Width: 50, Height: 10, Radius: 2}, geom.RRectAll(geom.Rect{W: 50, H: 10}, 2)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := skeletonTree(t, true, basic.SizedBox{Width: 200, Child: basic.Align{Alignment: layout.AlignTopLeft, Child: tc.w}})
			ops := wtest.Ops(tree, skeletonScreen)
			if len(ops) != 1 || ops[0].Kind != paint.OpRRect {
				t.Fatalf("got %d commands", len(ops))
			}
			if ops[0].RRect != tc.want {
				t.Errorf("got %+v, want %+v", ops[0].RRect, tc.want)
			}
			if ops[0].HasGradient {
				t.Error("a still skeleton has no light")
			}
		})
	}
}

func TestSkeletonText_VariedLines(t *testing.T) {
	tree := skeletonTree(t, true, basic.SizedBox{Width: 200, Child: SkeletonText{Lines: 4}})
	ops := wtest.Ops(tree, skeletonScreen)
	if len(ops) != 4 {
		t.Fatalf("got %d lines", len(ops))
	}
	widths := map[float32]bool{}
	for i := range ops {
		widths[ops[i].RRect.Rect.W] = true
		if i > 0 && ops[i].RRect.Rect.Y != ops[i-1].RRect.Rect.Y+12+8 {
			t.Errorf("line %d at %v", i, ops[i].RRect.Rect.Y)
		}
	}
	if len(widths) != 4 {
		t.Errorf("the widths vary, got %v", widths)
	}
	if last := ops[3].RRect.Rect.W; geom.Abs32(last-200*skeletonLastShare) > 1e-3 {
		t.Errorf("the last line is short, got %v", last)
	}
	if one := wtest.Ops(skeletonTree(t, true, basic.SizedBox{Width: 200, Child: SkeletonText{Lines: 1}}), skeletonScreen); len(one) != 1 || one[0].RRect.Rect.W != 200 {
		t.Errorf("a single line is full, got %+v", one)
	}
	if three := wtest.Ops(skeletonTree(t, true, basic.SizedBox{Width: 200, Child: SkeletonText{}}), skeletonScreen); len(three) != 3 {
		t.Errorf("the zero value has three lines, got %d", len(three))
	}
}

func TestSkeletonGroup_IsBusyAndHidesItsChild(t *testing.T) {
	tree := skeletonTree(t, true, SkeletonGroup{Child: basic.Column{Size: layout.MainMin, Children: []core.Widget{
		basic.Text{Content: "隠れる"}, Skeleton{},
	}}})
	testutil.RenderTree(tree, skeletonScreen)
	root := tree.Semantics()
	n := wtest.FindRole(root, semantics.RoleProgress)
	if n == nil || n.Label != "読み込み中…" || len(n.Children) != 0 {
		t.Fatalf("got %+v", n)
	}
	if n.Range.Max > n.Range.Min {
		t.Error("loading has no amount")
	}
	if wtest.FindRole(root, semantics.RoleText) != nil {
		t.Error("what the skeletons stand in front of is hidden")
	}
	named := skeletonTree(t, true, SkeletonGroup{Label: "受信箱を読み込み中", Child: Skeleton{}})
	testutil.RenderTree(named, skeletonScreen)
	if n := wtest.FindRole(named.Semantics(), semantics.RoleProgress); n == nil || n.Label != "受信箱を読み込み中" {
		t.Errorf("got %+v", n)
	}
	if n := wtest.FindRole(skeletonTree(t, true, Skeleton{}).Semantics(), semantics.RoleImage); n != nil {
		t.Error("a bare skeleton says nothing")
	}
}

func TestSkeleton_ShimmerRunsInSync(t *testing.T) {
	tree := skeletonTree(t, false, skeletonCard())
	testutil.RenderTree(tree, skeletonScreen)
	if !tree.Ticking() {
		t.Fatal("a seen skeleton shimmers")
	}
	now := time.Second
	wtest.Advance(tree, skeletonScreen, &now, 400*time.Millisecond)
	ops := wtest.Ops(tree, skeletonScreen)
	if len(ops) != 4 {
		t.Fatalf("got %d commands", len(ops))
	}
	// One band crosses the window: in window coordinates every shape has
	// the same gradient.
	var band float32
	for i := range ops {
		op := &ops[i]
		if !op.HasGradient || op.Gradient.StopCount != 3 {
			t.Fatalf("shape %d has no light", i)
		}
		start := op.Gradient.Start.X + op.Origin.X
		if i == 0 {
			band = start
		} else if geom.Abs32(start-band) > 1e-3 {
			t.Errorf("shape %d has its own band at %v, want %v", i, start, band)
		}
	}
	before := band
	wtest.Advance(tree, skeletonScreen, &now, 200*time.Millisecond)
	ops = wtest.Ops(tree, skeletonScreen)
	if after := ops[0].Gradient.Start.X + ops[0].Origin.X; after <= before {
		t.Errorf("the band moves right, from %v to %v", before, after)
	}
}

func TestSkeleton_ReduceMotionStopsTheTicker(t *testing.T) {
	tree := skeletonTree(t, true, skeletonCard())
	testutil.RenderTree(tree, skeletonScreen)
	if tree.Ticking() {
		t.Error("less motion asks for no frames")
	}

	// A window that asks for less motion while the light runs stops it.
	reduce := false
	var state *wtest.SwapState
	moving := core.NewTree(wtest.Swap{State: &state, Build: func() core.Widget {
		return basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1, ReduceMotion: reduce}, Child: basic.Align{Alignment: layout.AlignTopLeft, Child: skeletonCard()}}
	}})
	defer moving.Dispose()
	now := time.Second
	testutil.RenderTree(moving, skeletonScreen)
	wtest.Advance(moving, skeletonScreen, &now, 100*time.Millisecond)
	if !moving.Ticking() {
		t.Fatal("the light runs")
	}
	state.SetState(func() { reduce = true })
	testutil.RenderTree(moving, skeletonScreen)
	if moving.Ticking() {
		t.Error("less motion stops the light at once")
	}
	for _, op := range wtest.Ops(moving, skeletonScreen) {
		if op.HasGradient {
			t.Error("a still skeleton is flat")
		}
	}
}

func TestSkeleton_StopsWhenUnmountedOrUnseen(t *testing.T) {
	t.Run("木から外れる", func(t *testing.T) {
		tree, swap := wtest.SwapTree(t, basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1}, Child: skeletonCard()})
		now := time.Second
		testutil.RenderTree(tree, skeletonScreen)
		wtest.Advance(tree, skeletonScreen, &now, 50*time.Millisecond)
		if !tree.Ticking() {
			t.Fatal("the light runs")
		}
		swap(basic.Text{Content: "読み込み完了"})
		testutil.RenderTree(tree, skeletonScreen)
		wtest.Advance(tree, skeletonScreen, &now, 50*time.Millisecond)
		if tree.Ticking() {
			t.Error("a skeleton out of the tree asks for no frames")
		}
	})
	t.Run("スクロールの外", func(t *testing.T) {
		rows := []core.Widget{basic.SizedBox{Height: 1000}, Skeleton{Width: 100}}
		tree := skeletonTree(t, false, basic.SizedBox{Height: 200, Child: basic.Scroll{Child: basic.Column{Size: layout.MainMin, Children: rows}}})
		now := time.Second
		testutil.RenderTree(tree, skeletonScreen)
		wtest.Advance(tree, skeletonScreen, &now, 50*time.Millisecond)
		if tree.Ticking() {
			t.Error("a skeleton scrolled out of view asks for no frames")
		}
	})
	t.Run("透明", func(t *testing.T) {
		opacity := float32(1)
		var state *wtest.SwapState
		tree := core.NewTree(wtest.Swap{State: &state, Build: func() core.Widget {
			return basic.Align{Alignment: layout.AlignTopLeft, Child: basic.Opacity{Opacity: opacity, Child: Skeleton{Width: 100}}}
		}})
		defer tree.Dispose()
		now := time.Second
		testutil.RenderTree(tree, skeletonScreen)
		wtest.Advance(tree, skeletonScreen, &now, 50*time.Millisecond)
		if !tree.Ticking() {
			t.Fatal("the light runs")
		}
		state.SetState(func() { opacity = 0 })
		wtest.Advance(tree, skeletonScreen, &now, 50*time.Millisecond)
		if tree.Ticking() {
			t.Error("a hidden skeleton asks for no frames")
		}
		state.SetState(func() { opacity = 1 })
		testutil.RenderTree(tree, skeletonScreen)
		if !tree.Ticking() {
			t.Error("shown again, the light runs again")
		}
	})
	t.Run("大きさゼロ", func(t *testing.T) {
		tree := core.NewTree(basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1}, Child: Skeleton{Shape: SkeletonBlock}})
		defer tree.Dispose()
		testutil.RenderTree(tree, geom.Size{})
		if tree.Ticking() {
			t.Error("a skeleton of no size asks for no frames")
		}
	})
}

func TestSkeleton_FrameDoesNotAllocate(t *testing.T) {
	tree := skeletonTree(t, false, skeletonCard())
	now := time.Second
	testutil.RenderTree(tree, skeletonScreen)
	var cv paint.Canvas
	frame := func() {
		now += 8 * time.Millisecond
		tree.Tick(now)
		tree.Layout(geom.Tight(skeletonScreen))
		cv.Reset()
		tree.Paint(&cv)
	}
	for range 10 {
		frame()
	}
	if got := testing.AllocsPerRun(50, frame); got != 0 {
		t.Errorf("a frame of shimmer allocates %v times", got)
	}
}

// BenchmarkSkeleton_ShimmerFrame is one frame of a card of skeletons while the
// light runs: the tick, the layout and the paint.
func BenchmarkSkeleton_ShimmerFrame(b *testing.B) {
	tree := core.NewTree(basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1}, Child: basic.Align{Alignment: layout.AlignTopLeft, Child: skeletonCard()}})
	defer tree.Dispose()
	now := time.Second
	testutil.RenderTree(tree, skeletonScreen)
	var cv paint.Canvas
	b.ReportAllocs()
	for b.Loop() {
		now += 8 * time.Millisecond
		tree.Tick(now)
		tree.Layout(geom.Tight(skeletonScreen))
		cv.Reset()
		tree.Paint(&cv)
	}
}
