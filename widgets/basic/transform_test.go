package basic

import (
	"math"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
)

var transformPage = geom.Size{W: 200, H: 200}

func card() core.Widget {
	return SizedBox{Width: 80, Height: 40, Child: Container{Color: geom.Hex(0x2f5fd0), Radius: 6}}
}

func TestTransform_ZeroValuePaintsLikeTheChild(t *testing.T) {
	plain := testutil.Render(Center{Child: card()}, transformPage)
	if got := testutil.Render(Center{Child: Transform{Child: card()}}, transformPage); got != plain {
		t.Errorf("got\n%s\nwant\n%s", got, plain)
	}
}

func TestTransform_ScaleTranslateTakesTheFastPath(t *testing.T) {
	got := testutil.Render(Transform{Transform: geom.Translation(geom.Offset{X: 10}), Child: card()}, transformPage)
	if strings.Contains(got, "transform=") {
		t.Errorf("a move must not reach Op.Transform:\n%s", got)
	}
	if !strings.Contains(got, "x=10 ") {
		t.Errorf("the card must move 10 to the right:\n%s", got)
	}
}

func TestRotate_RecordsTheTurn(t *testing.T) {
	got := testutil.Render(Center{Child: Rotate(math.Pi/2, card())}, transformPage)
	if !strings.Contains(got, "transform=") {
		t.Fatalf("a rotation must reach Op.Transform:\n%s", got)
	}
}

func TestSkew_IsAroundTheCenter(t *testing.T) {
	s := Skew(0.3, 0, card())
	if s.Origin != layout.AlignCenter || s.Transform != geom.Skewing(0.3, 0) {
		t.Errorf("got %+v", s)
	}
}

func TestTransform_LayoutIgnoresIt(t *testing.T) {
	tree := core.NewTree(Center{Child: Rotate(1, card())})
	defer tree.Dispose()
	testutil.RenderTree(tree, transformPage)
	path := tree.HitTest(geom.Offset{X: 100, Y: 100}).Path
	var ro *layout.RenderTransform
	for _, o := range path {
		if r, ok := o.(*layout.RenderTransform); ok {
			ro = r
		}
	}
	if ro == nil {
		t.Fatal("the middle must hit the turned card")
	}
	if ro.Size() != (geom.Size{W: 80, H: 40}) || ro.Offset() != (geom.Offset{X: 60, Y: 80}) {
		t.Errorf("size %v offset %v: layout must not see the turn", ro.Size(), ro.Offset())
	}
}

func TestTransform_PaintDoesNotAllocate(t *testing.T) {
	tree := core.NewTree(Center{Child: Rotate(0.4, card())})
	defer tree.Dispose()
	testutil.RenderTree(tree, transformPage)
	var cv paint.Canvas
	allocs := testing.AllocsPerRun(20, func() {
		cv.Reset()
		tree.Paint(&cv)
	})
	if allocs != 0 {
		t.Errorf("got %v allocs, want 0", allocs)
	}
}
