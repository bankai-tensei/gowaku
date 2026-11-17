package gowakutest_test

import (
	"testing"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

// album is a tile with a play button that fades in while the pointer is over
// it, above a player bar with a play button of the same label: the case of the
// music player that found the bug.
type album struct {
	gowaku.StatefulBase
	c *albumCounts
}

// albumCounts records what reached the album.
type albumCounts struct {
	enters, exits           int
	tileTaps, overlay, play int
}

type albumState struct {
	gowaku.StateBase
	hint float32
}

func (album) CreateState() gowaku.State { return &albumState{} }

func (s *albumState) Build(gowaku.BuildContext) gowaku.Widget {
	c := s.Widget().(album).c
	tile := interact.MouseRegion{
		OnEnter: func() { s.SetState(func() { c.enters++; s.hint = 1 }) },
		OnExit:  func() { s.SetState(func() { c.exits++; s.hint = 0 }) },
		Child: interact.GestureDetector{
			OnTap: func() { c.tileTaps++ },
			Child: interact.Semantics{Label: "tile", Child: basic.SizedBox{Width: 200, Height: 200, Child: basic.Stack{
				Fit: layout.StackExpand,
				Children: []gowaku.Widget{
					basic.Container{Color: geom.RGBA(0.2, 0.4, 0.6, 1)},
					basic.AnimatedOpacity{Opacity: s.hint, Child: basic.Center{Child: control.Button{
						Label:   "再生",
						OnPress: func() { c.overlay++ },
					}}},
				},
			}}},
		},
	}
	return basic.VStack(tile, control.Button{Label: "再生", OnPress: func() { c.play++ }})
}

func newAlbum(t *testing.T) (*gowakutest.Tester, *albumCounts) {
	c := &albumCounts{}
	return gowakutest.New(t, album{c: c}, goFonts), c
}

func middle(n *gowakutest.Node) geom.Offset {
	return geom.Offset{X: n.Rect.X + n.Rect.W/2, Y: n.Rect.Y + n.Rect.H/2}
}

// playButtons counts the buttons labeled 再生; their text has the label too.
func playButtons(tt *gowakutest.Tester) int {
	n := 0
	for _, node := range tt.FindAll("再生") {
		if node.Role == semantics.RoleButton {
			n++
		}
	}
	return n
}

func TestHidden_HitTestDoesNotAllocate(t *testing.T) {
	tt := gowakutest.New(t, basic.Opacity{Child: control.Button{Label: "hidden"}}, goFonts)
	if _, ok := tt.Find("hidden"); ok {
		t.Fatalf("a button under Opacity 0 is in the tree\n%s", tt.Dump())
	}
	e := core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 400, Y: 300}}
	tt.Tree().DispatchPointer(e)
	if n := testing.AllocsPerRun(100, func() { tt.Tree().DispatchPointer(e) }); n != 0 {
		t.Errorf("a pointer move over a hidden subtree allocates %v times", n)
	}
}

func move(tt *gowakutest.Tester, pos geom.Offset) {
	tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: pos})
	tt.Pump()
}

func TestHidden_TransparentButtonIsNotFound(t *testing.T) {
	tt, c := newAlbum(t)
	if n := playButtons(tt); n != 1 {
		t.Fatalf("want only the play button of the bar, got %d\n%s", n, tt.Dump())
	}
	tt.Tap("再生")
	if c.play != 1 || c.overlay != 0 {
		t.Fatalf("Tap reached play %d times and the hidden overlay %d times", c.play, c.overlay)
	}
}

func TestHidden_ButtonComesBackWithOpacity(t *testing.T) {
	tt, c := newAlbum(t)
	tile, ok := tt.Find("tile")
	if !ok {
		t.Fatalf("no tile\n%s", tt.Dump())
	}

	// The MouseRegion around the hidden overlay still sees the pointer.
	move(tt, middle(tile))
	if c.enters != 1 {
		t.Fatalf("the region around the hidden overlay got %d enters", c.enters)
	}
	// Partway into the fade, long before it ends, the button is there again.
	tt.Advance(3 * gowakutest.FrameTime)
	if n := playButtons(tt); n != 2 {
		t.Fatalf("partway into the fade, want 2 play buttons, got %d\n%s", n, tt.Dump())
	}
	tt.Settle()
	tt.Tap("再生") // the overlay comes first in reading order
	if c.overlay != 1 || c.play != 0 || c.tileTaps != 0 {
		t.Fatalf("overlay %d, play %d, tile %d after tapping the shown overlay", c.overlay, c.play, c.tileTaps)
	}

	// Leaving fades the overlay out; at rest it is gone again.
	move(tt, geom.Offset{X: 700, Y: 500})
	tt.Settle()
	if c.exits != 1 {
		t.Fatalf("got %d exits", c.exits)
	}
	if n := playButtons(tt); n != 1 {
		t.Fatalf("after the fade out, want 1 play button, got %d\n%s", n, tt.Dump())
	}
}

func TestHidden_PointerPassesThroughToWhatIsBelow(t *testing.T) {
	tt, c := newAlbum(t)
	tile, _ := tt.Find("tile")
	// A press where the invisible button sits reaches the tile, and the
	// region around both sees the pointer come in.
	tt.TapAt(middle(tile))
	if c.tileTaps != 1 || c.overlay != 0 {
		t.Fatalf("tile %d, hidden overlay %d after a press on the hidden button", c.tileTaps, c.overlay)
	}
	if c.enters != 1 {
		t.Fatalf("the region got %d enters", c.enters)
	}
}

func TestHidden_TabSkipsTransparentButton(t *testing.T) {
	tt, c := newAlbum(t)
	tt.Key(input.KeyTab, 0)
	tt.Key(input.KeyEnter, 0)
	if c.play != 1 || c.overlay != 0 {
		t.Fatalf("Tab then Enter pressed play %d times and the hidden overlay %d times", c.play, c.overlay)
	}
	// Tab wraps around to the same button: the hidden one is not in the order.
	tt.Key(input.KeyTab, 0)
	tt.Key(input.KeyEnter, 0)
	if c.play != 2 || c.overlay != 0 {
		t.Fatalf("second Tab: play %d, hidden overlay %d", c.play, c.overlay)
	}
}
