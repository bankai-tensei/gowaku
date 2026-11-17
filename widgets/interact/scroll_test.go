package interact

import (
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

func TestScroll_ClipsAndHitsThroughTheOffset(t *testing.T) {
	tapped := 0
	tree := wtest.TextTree(t, basic.Scroll{Child: basic.Column{Children: []core.Widget{
		basic.SizedBox{Height: 200, Child: tallChild(200)},
		GestureDetector{OnTap: func() { tapped++ }, Child: basic.SizedBox{Width: 100, Height: 50, Child: tallChild(50)}},
		basic.SizedBox{Height: 400, Child: tallChild(400)},
	}}})
	size := geom.Size{W: 200, H: 100}
	testutil.RenderTree(tree, size)
	press := func() {
		tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: geom.Offset{X: 20, Y: 20}})
		tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: geom.Offset{X: 20, Y: 20}})
	}
	press()
	if tapped != 0 {
		t.Fatal("the target is below the view")
	}
	wtest.Wheel(tree, -2)
	wtest.RunUntilIdle(t, tree, size, nil)
	press()
	if tapped != 1 {
		t.Error("the target has scrolled under the pointer")
	}
	if !strings.Contains(testutil.RenderTree(tree, size), "clip=0,0,200,100") {
		t.Error("the content must be clipped to the view")
	}
}

func tallChild(height float32) core.Widget {
	return basic.Container{Height: height, Color: geom.Hex(0x123456)}
}
