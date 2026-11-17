package collection_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
)

// logView is a list of count rows of 20 pixels in a view 100 high, whose
// count the test changes as an application appends lines.
type logView struct {
	core.StatefulBase
	list collection.List
	// state is set to the State when it is created.
	state **logViewState
}

type logViewState struct {
	core.StateBase
	count int
}

func (logView) CreateState() core.State { return &logViewState{} }

func (s *logViewState) InitState() {
	w := s.Widget().(logView)
	s.count = w.list.ItemCount
	*w.state = s
}

// add appends n rows, as the SetState of an application does.
func (s *logViewState) add(n int) { s.SetState(func() { s.count += n }) }

func (s *logViewState) Build(core.BuildContext) core.Widget {
	l := s.Widget().(logView).list
	l.ItemCount = s.count
	l.ItemBuilder = func(i int) core.Widget { return basic.Text{Content: rowLabel(i)} }
	return basic.SizedBox{Width: 200, Height: 100, Child: l}
}

// rowLabel is the text of row i: every third row wraps to several lines where
// the list measures its rows.
func rowLabel(i int) string {
	if i%3 == 0 {
		return fmt.Sprintf("row %d", i) + strings.Repeat(" long", 30)
	}
	return fmt.Sprintf("row %d", i)
}

func newLog(t *testing.T, list collection.List) (*gowakutest.Tester, *logViewState) {
	t.Helper()
	if list.ItemExtent == 0 && !list.MeasureItems {
		list.ItemExtent = 20
	}
	var s *logViewState
	tt := gowakutest.New(t, basic.Column{Children: []core.Widget{logView{list: list, state: &s}}}, gowakutest.Options{Width: 300, Height: 200})
	return tt, s
}

// rowBottom returns the bottom edge of row i, failing the test if it is not
// there.
func rowBottom(t *testing.T, tt *gowakutest.Tester, i int) float32 {
	t.Helper()
	n, ok := tt.Find(rowLabel(i))
	if !ok {
		t.Fatalf("row %d is not built\n%s", i, tt.Dump())
	}
	return n.Rect.Y + n.Rect.H
}

// wheelTowardTop turns the wheel over the list by notches toward the top of
// the content. It sends the event itself, in the convention of
// core.PointerEvent.Scroll, where a positive Y turns away from the user and
// moves the view toward the top, so that it does not depend on the sign of
// Tester.Scroll.
func wheelTowardTop(tt *gowakutest.Tester, notches float32) {
	tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 50, Y: 50}})
	tt.Tree().DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: geom.Offset{X: 50, Y: 50}, Scroll: geom.Offset{Y: notches}})
	tt.Settle()
}

func TestList_JumpToEndRightAfterAppending(t *testing.T) {
	var c collection.ListController
	tt, s := newLog(t, collection.List{ItemCount: 10, Controller: &c})
	s.add(5)
	// The list has not rebuilt yet: it still has 10 rows.
	c.JumpToEnd()
	tt.Settle()
	if got := rowBottom(t, tt, 14); got != 100 {
		t.Errorf("the bottom of the last row is at %v, want 100", got)
	}
	if !c.AtEnd() || c.MaxOffset() != 200 || c.Offset() != 200 {
		t.Errorf("position %+v, want at the end, 200 of 200", c.Position())
	}
}

func TestList_JumpToEndBeforeTheFirstRow(t *testing.T) {
	var c collection.ListController
	tt, s := newLog(t, collection.List{Controller: &c})
	c.JumpToEnd()
	s.add(8)
	tt.Settle()
	if got := rowBottom(t, tt, 7); got != 100 {
		t.Errorf("the bottom of the last row is at %v, want 100", got)
	}
}

func TestList_StickToEnd(t *testing.T) {
	var c collection.ListController
	var told []basic.ScrollPosition
	tt, s := newLog(t, collection.List{ItemCount: 3, StickToEnd: true, Controller: &c,
		OnScroll: func(p basic.ScrollPosition) { told = append(told, p) }})
	if len(told) != 1 || told[0] != (basic.ScrollPosition{}) {
		t.Fatalf("after the first layout OnScroll was told %v, want one zero position", told)
	}
	// Short, then longer than the view: the view follows the new rows.
	for _, n := range []int{1, 4, 30} {
		s.add(n)
		tt.Settle()
		if last := s.count - 1; rowBottom(t, tt, last) != min(100, 20*float32(s.count)) || !c.AtEnd() {
			t.Fatalf("after %d rows the view is at %+v, not at the end", s.count, c.Position())
		}
	}
	if p := told[len(told)-1]; p.Offset != 660 || p.MaxOffset != 660 || !p.AtEnd() {
		t.Errorf("OnScroll was last told %+v, want 660 of 660", p)
	}
	// The user scrolls up: the view stays where it is as rows come.
	wheelTowardTop(tt, 2)
	away := c.Offset()
	if c.AtEnd() || away >= 660 {
		t.Fatalf("the wheel did not move the view up: %+v", c.Position())
	}
	if last := told[len(told)-1]; last.AtEnd() {
		t.Errorf("OnScroll was last told %+v, still at the end", last)
	}
	s.add(5)
	tt.Settle()
	if c.Offset() != away || c.AtEnd() {
		t.Errorf("with the view away from the end, new rows moved it: %+v, want offset %v", c.Position(), away)
	}
	// Back at the end, it sticks again.
	c.JumpToEnd()
	tt.Settle()
	s.add(2)
	tt.Settle()
	if last := s.count - 1; rowBottom(t, tt, last) != 100 {
		t.Errorf("back at the end, the view did not follow: %+v", c.Position())
	}
}

func TestList_StickToEndOpensAtTheEnd(t *testing.T) {
	var c collection.ListController
	tt, _ := newLog(t, collection.List{ItemCount: 50, StickToEnd: true, Controller: &c})
	if rowBottom(t, tt, 49) != 100 || !c.AtEnd() {
		t.Errorf("the list opened at %+v, not at its end", c.Position())
	}
}

func TestList_StickToEndMeasuredRows(t *testing.T) {
	var c collection.ListController
	tt, s := newLog(t, collection.List{ItemCount: 2, MeasureItems: true, StickToEnd: true, Controller: &c})
	for range 6 {
		s.add(7)
		tt.Settle()
		if last := s.count - 1; rowBottom(t, tt, last) != 100 || !c.AtEnd() {
			t.Fatalf("with %d measured rows the view is at %+v, not at the end\n%s", s.count, c.Position(), tt.Dump())
		}
	}
}
