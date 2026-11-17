package basic_test

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// growing is a Scroll 100 high over a Column of lines 20 high, whose number
// the test changes as an application appends lines.
type growing struct {
	core.StatefulBase
	scroll basic.Scroll
	lines  int
	state  **growingState
}

type growingState struct {
	core.StateBase
	lines int
}

func (growing) CreateState() core.State { return &growingState{} }

func (s *growingState) InitState() {
	w := s.Widget().(growing)
	s.lines = w.lines
	*w.state = s
}

func (s *growingState) add(n int) { s.SetState(func() { s.lines += n }) }

func (s *growingState) Build(core.BuildContext) core.Widget {
	lines := make([]core.Widget, s.lines)
	for i := range lines {
		lines[i] = basic.SizedBox{Width: 100, Height: 20}
	}
	sc := s.Widget().(growing).scroll
	sc.Child = basic.Column{Children: lines}
	return basic.SizedBox{Width: 200, Height: 100, Child: sc}
}

func newGrowing(t *testing.T, sc basic.Scroll, lines int) (*gowakutest.Tester, *growingState) {
	t.Helper()
	var s *growingState
	tt := gowakutest.New(t, basic.Column{Children: []core.Widget{growing{scroll: sc, lines: lines, state: &s}}},
		gowakutest.Options{Width: 300, Height: 200})
	return tt, s
}

// wheelTowardTop turns the wheel over the Scroll by notches toward the top of
// the content, in the convention of core.PointerEvent.Scroll: a positive Y
// turns away from the user and moves the view toward the top. It sends the
// event itself so as not to depend on the sign of Tester.Scroll.
func wheelTowardTop(tt *gowakutest.Tester, notches float32) {
	at := geom.Offset{X: 50, Y: 50}
	tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: at})
	tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: at, Scroll: geom.Offset{Y: notches}})
	tt.Settle()
}

func TestScrollController_JumpToEndRightAfterAppending(t *testing.T) {
	var c basic.ScrollController
	tt, s := newGrowing(t, basic.Scroll{Controller: &c}, 10)
	s.add(5)
	// The Scroll has not rebuilt yet: its content is still 200 high.
	c.JumpToEnd()
	tt.Settle()
	if p := c.Position(); p.Offset != 200 || p.MaxOffset != 200 || !c.AtEnd() {
		t.Errorf("position %+v, want 200 of 200", p)
	}
}

func TestScroll_StickToEnd(t *testing.T) {
	var c basic.ScrollController
	var told []basic.ScrollPosition
	tt, s := newGrowing(t, basic.Scroll{StickToEnd: true, Controller: &c,
		OnScroll: func(p basic.ScrollPosition) { told = append(told, p) }}, 2)
	if len(told) != 1 || !told[0].AtEnd() || !told[0].AtStart() {
		t.Fatalf("after the first layout OnScroll was told %v, want one position at both ends", told)
	}
	s.add(10)
	tt.Settle()
	if p := c.Position(); p.Offset != 140 || !p.AtEnd() {
		t.Fatalf("after 12 lines the view is at %+v, want 140, at the end", p)
	}
	if last := told[len(told)-1]; last != c.Position() {
		t.Errorf("OnScroll was last told %+v, want %+v", last, c.Position())
	}
	wheelTowardTop(tt, 1)
	away := c.Offset()
	if c.AtEnd() {
		t.Fatalf("the wheel did not move the view up: %+v", c.Position())
	}
	s.add(3)
	tt.Settle()
	if c.Offset() != away {
		t.Errorf("away from the end, new lines moved the view from %v to %v", away, c.Offset())
	}
	c.JumpToEnd()
	tt.Settle()
	s.add(1)
	tt.Settle()
	if p := c.Position(); p.Offset != 220 || !p.AtEnd() {
		t.Errorf("back at the end, the view did not follow: %+v", p)
	}
}

func TestScroll_ReverseStaysAtTheBottomWithoutStickToEnd(t *testing.T) {
	var c basic.ScrollController
	tt, s := newGrowing(t, basic.Scroll{Reverse: true, Controller: &c}, 10)
	s.add(4)
	tt.Settle()
	// Offset 0 is the bottom of a Reverse Scroll: the newest line.
	if p := c.Position(); p.Offset != 0 || !p.AtStart() || p.MaxOffset != 180 {
		t.Errorf("position %+v, want 0 of 180", p)
	}
}
