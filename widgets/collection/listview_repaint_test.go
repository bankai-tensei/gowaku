package collection

import (
	"fmt"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/control"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/paint"
)

// listRow has what a row of a real list has: a background, text and a press.
func listRow(i int) core.Widget {
	return basic.Container{
		Color: geom.Hex(0xf5f5f7),
		Child: basic.Row{Children: []core.Widget{
			basic.Text{Content: fmt.Sprintf("row %d", i)},
			control.Button{Label: "open", OnPress: func() {}},
		}},
	}
}

// versions returns the layers of the frame in cv with their versions.
func versions(cv *paint.Canvas) map[*paint.Layer]uint64 {
	seen := map[*paint.Layer]uint64{}
	for _, op := range cv.DisplayList().Ops {
		if op.Kind == paint.OpLayer {
			seen[op.Layer] = op.Layer.Version()
		}
	}
	return seen
}

func TestListView_ScrollRecordsOnlyNewRows(t *testing.T) {
	tree := wtest.TextTree(t, List{ItemCount: 10000, ItemExtent: 20, ItemBuilder: listRow})
	size := geom.Size{W: 400, H: 200}
	testutil.RenderTree(tree, size)
	var cv paint.Canvas
	frame := func() {
		tree.Build()
		tree.Layout(geom.Tight(size))
		for tree.NeedsBuild() {
			tree.Build()
			tree.Layout(geom.Tight(size))
		}
		tree.Repaint(&cv)
	}
	frame()
	cv.Reset()
	tree.Paint(&cv)
	seen := versions(&cv)
	if len(seen) < 10 {
		t.Fatalf("got %d layers for 10 rows", len(seen))
	}

	wtest.Wheel(tree, -2)
	frames, kept := 0, 0
	runUntilIdleWith(t, tree, frame, func() {
		frames++
		next := versions(&cv)
		for l, v := range next {
			old, ok := seen[l]
			if !ok {
				continue
			}
			kept++
			if v != old {
				t.Fatalf("frame %d: a row that stayed recorded again", frames)
			}
		}
		for l, v := range next {
			seen[l] = v
		}
	})
	if frames == 0 || kept == 0 {
		t.Fatalf("%d frames kept %d rows", frames, kept)
	}
}

func TestListView_ChangeInARowRecordsThatRow(t *testing.T) {
	tree := wtest.TextTree(t, List{ItemCount: 100, ItemExtent: 20, ItemBuilder: listRow})
	size := geom.Size{W: 400, H: 200}
	testutil.RenderTree(tree, size)
	var cv paint.Canvas
	cv.Reset()
	tree.Paint(&cv)
	before := versions(&cv)

	// Hovering the first row changes it and nothing else.
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 390, Y: 10}})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 60, Y: 10}})
	tree.Build()
	tree.Layout(geom.Tight(size))
	tree.Repaint(&cv)
	changed := 0
	for l, v := range before {
		if l.Version() != v {
			changed++
		}
	}
	if changed > 1 {
		t.Errorf("%d rows recorded again", changed)
	}
}
func runUntilIdleWith(t testing.TB, tree *core.Tree, frame, visit func()) {
	t.Helper()
	frames := 0
	for now := time.Second; tree.Ticking(); now += 8 * time.Millisecond {
		tree.Tick(now)
		frame()
		visit()
		if frames++; frames > 2000 {
			t.Fatal("the motion never ends")
		}
	}
}
