package markdown

import (
	"testing"
	"testing/fstest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/field"
)

const pictureDoc = "## Top\n\n[down](#target)\n\n![tall](tall.png)\n\ntext\n\n![late](late)\n\n## Target\n\n" +
	"more\n\nmore\n\nmore\n\nmore\n\nmore\n\nmore\n\nmore\n\nmore\n\nmore\n\nmore\n\nmore\n\nmore\n\n" +
	"more\n\nmore\n\nmore\n\nmore\n\nmore\n\nmore\n\nmore\n\nmore\n\nmore\n\nmore\n\nmore\n\nmore\n"

// lateDoc shows pictureDoc with a picture from Images, and one from
// ImageProvider whose height the test changes, as a picture that arrives
// after the document was laid out.
type lateDoc struct {
	core.StatefulBase
	self **lateDocState
	tall []byte
}

type lateDocState struct {
	core.StateOf[lateDoc]
	lateHeight float32
	c          Controller
}

func (lateDoc) CreateState() core.State { return &lateDocState{} }

func (s *lateDocState) InitState() { *s.Widget().self = s }

func (s *lateDocState) Build(core.BuildContext) core.Widget {
	h := s.lateHeight
	return basic.Scroll{Child: Markdown{
		Source:     pictureDoc,
		Controller: &s.c,
		Images:     fstest.MapFS{"tall.png": {Data: s.Widget().tall}},
		ImageProvider: func(src, _ string) core.Widget {
			if src != "late" {
				return nil
			}
			return basic.SizedBox{Width: 100, Height: h}
		},
	}}
}

func TestAnchor_LandsBelowPicturesAndFollowsALatePicture(t *testing.T) {
	var s *lateDocState
	tt := gowakutest.New(t, lateDoc{self: &s, tall: pngFile(t, 100, 500)}, gowakutest.Options{Height: 400})
	if y := headingTop(t, tt, "target"); y < 500 {
		t.Fatalf("the heading is at %v, above the 500 pixels of the picture", y)
	}
	tt.Tap("down")
	if y := headingTop(t, tt, "target"); y != headingMargin {
		t.Fatalf("below a picture the heading lands at %v, want %v", y, headingMargin)
	}
	// The picture above it comes in late: the heading comes back to the top
	// on the next frame.
	s.SetState(func() { s.lateHeight = 300 })
	tt.Pump()
	tt.Pump()
	if y := headingTop(t, tt, "target"); y != headingMargin {
		t.Errorf("after a picture above grew the heading is at %v, want %v", y, headingMargin)
	}
	// Once the user scrolls, the view stays where the user put it.
	tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: geom.Offset{X: 100, Y: 100}, Scroll: geom.Offset{Y: -0.5}})
	tt.Pump()
	moved := headingTop(t, tt, "target")
	if moved == headingMargin {
		t.Fatal("the wheel did not move the view")
	}
	s.SetState(func() { s.lateHeight = 100 })
	tt.Pump()
	tt.Pump()
	if y := headingTop(t, tt, "target"); y == headingMargin {
		t.Error("after the user scrolled, a change above still brought the heading back")
	}
}

func TestAnchor_NewerJumpWinsOverAPendingFollow(t *testing.T) {
	var s *lateDocState
	tt := gowakutest.New(t, lateDoc{self: &s, tall: pngFile(t, 100, 500)}, gowakutest.Options{Height: 400})
	tt.Tap("down")
	// The picture above grows: bringing the heading back waits for the next
	// frame. Before that, the app moves to another heading, which happens at
	// once and must stay.
	s.SetState(func() { s.lateHeight = 300 })
	tt.Pump()
	if !s.c.ShowHeading("top") {
		t.Fatal("ShowHeading does not find the heading")
	}
	if s.c.view.ticker.Running() {
		t.Error("the wait still runs after a jump that succeeded at once")
	}
	want := headingTop(t, tt, "top")
	tt.Pump()
	tt.Pump()
	if y := headingTop(t, tt, "top"); y != want {
		t.Errorf("the pending follow-up took the view away: the newer heading went from %v to %v", want, y)
	}
}

func TestAnchor_WheelCancelsAPendingFollow(t *testing.T) {
	var s *lateDocState
	tt := gowakutest.New(t, lateDoc{self: &s, tall: pngFile(t, 100, 500)}, gowakutest.Options{Height: 400})
	tt.Tap("down")
	// The picture above grows, and before the follow-up runs the user turns
	// the wheel: the view stays where the user puts it.
	s.SetState(func() { s.lateHeight = 300 })
	tt.Pump()
	tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: geom.Offset{X: 100, Y: 100}, Scroll: geom.Offset{Y: -0.5}})
	tt.Pump()
	tt.Pump()
	if y := headingTop(t, tt, "target"); y == headingMargin {
		t.Error("the follow-up undid the wheel")
	}
}

func TestSelection_SkipsPictures(t *testing.T) {
	r := newSelRig(t, field.SelectionArea{Child: basic.Scroll{Child: Markdown{
		Source: "before\n\n![a picture](p.png)\n\n![missing](none.png)\n\nafter",
		Images: fstest.MapFS{"p.png": {Data: pngFile(t, 40, 40)}},
	}}}, gowakutest.Options{})
	p := findRich(r.tt, "before")
	r.drag(caret(p, 0), caret(p, 1))
	r.tt.Key(input.KeyA, input.ModControl)
	// The picture is no text; a picture that cannot be read shows its
	// alternative text, which is text like any other.
	if got := r.copied(); got != "before\n\nmissing\n\nafter" {
		t.Errorf("copied %q", got)
	}
}
