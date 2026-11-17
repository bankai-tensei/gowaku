package editor

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
)

// selectionLog records what OnSelectionChange of a view told.
type selectionLog struct {
	told []TextRange
	// changes counts the calls of OnChange, and atTell what it was at each
	// call.
	changes int
	atTell  []int
}

func (l *selectionLog) watch(v *View) {
	v.OnSelectionChange = func(r TextRange) {
		l.told = append(l.told, r)
		l.atTell = append(l.atTell, l.changes)
	}
}

func (l *selectionLog) take() []TextRange {
	t := l.told
	l.told, l.atTell = nil, nil
	return t
}

func TestView_OnSelectionChangeFollowsEveryMove(t *testing.T) {
	var ctl Controller
	var v View
	var log selectionLog
	log.watch(&v)
	ctl.SetText("one two\nthree\nfour")
	h := newEditor(t, &ctl, Editor{View: &v, OnChange: func() { log.changes++ }})
	if got := log.take(); len(got) != 0 {
		t.Errorf("showing the view told %v", got)
	}

	check := func(what string, want TextRange) {
		t.Helper()
		got := log.take()
		if len(got) != 1 || got[0] != want || v.Selection() != want {
			t.Errorf("%s told %v, want once %v (selection %v)", what, got, want, v.Selection())
		}
	}
	h.Key(input.KeyRight, 0)
	check("Right", caretRange(pos(0, 1)))
	h.Key(input.KeyEnd, input.ModShift)
	check("Shift+End", TextRange{Start: pos(0, 1), End: pos(0, 7)})
	h.click(geom.Offset{X: 5, Y: 5}, 1, 0)
	if got := log.take(); len(got) == 0 || got[len(got)-1] != caretRange(v.Selection().End) || !v.Selection().Empty() {
		t.Errorf("a click told %v, selection %v", got, v.Selection())
	}
	// A key that does not move the caret tells nothing.
	ctl.SetSelection(caretRange(pos(0, 0)))
	log.take()
	h.Key(input.KeyLeft, 0)
	if got := log.take(); len(got) != 0 {
		t.Errorf("Left at the start told %v", got)
	}

	// An edit tells after OnChange, so that the text is new.
	h.typeText("x")
	check("typing", caretRange(pos(0, 1)))
	if log.changes != 1 {
		t.Errorf("%d changes", log.changes)
	}
	h.typeText("y")
	if got := log.atTell; len(got) != 1 || got[0] != 2 {
		t.Errorf("told before OnChange: %v", got)
	}
	log.take()
	ctl.Undo()
	check("Undo", caretRange(pos(0, 0)))
	ctl.Redo()
	check("Redo", caretRange(pos(0, 2)))

	// Code tells too, through the controller and the view.
	ctl.SetSelection(TextRange{Start: pos(1, 0), End: pos(1, 2)})
	check("Controller.SetSelection", TextRange{Start: pos(1, 0), End: pos(1, 2)})
	v.SetSelection(caretRange(pos(2, 1)))
	check("View.SetSelection", caretRange(pos(2, 1)))
	v.SetSelection(caretRange(pos(2, 1)))
	if got := log.take(); len(got) != 0 {
		t.Errorf("the same selection told %v", got)
	}
	if _, err := ctl.Find("t", FindOptions{}); err != nil {
		t.Fatal(err)
	}
	ctl.FindNext()
	check("FindNext", TextRange{Start: pos(0, 6), End: pos(0, 7)})
	h.Key(input.KeyF3, 0)
	check("F3", TextRange{Start: pos(1, 0), End: pos(1, 1)})
	h.Key(input.KeyA, input.ModControl)
	check("Ctrl+A", TextRange{End: pos(2, 4)})

	// SetText starts over without telling, as OnChange does not run for it.
	ctl.SetText("new")
	h.frame()
	if got := log.take(); len(got) != 0 {
		t.Errorf("SetText told %v", got)
	}
	h.Key(input.KeyEnd, 0)
	check("End after SetText", caretRange(pos(0, 3)))
}

func TestViews_OnSelectionChangeOfTheOtherView(t *testing.T) {
	var ctl Controller
	var left, right View
	var ll, rl selectionLog
	ll.watch(&left)
	rl.watch(&right)
	ctl.SetText("one\ntwo\nthree")
	right.SetSelection(caretRange(pos(2, 2)))
	if got := rl.take(); len(got) != 1 || got[0] != caretRange(pos(2, 2)) {
		t.Errorf("SetSelection of a view not shown told %v", got)
	}
	h := newSplit(t, &ctl, &left, &right, Editor{})
	h.focus(&left)
	ll.take()
	// Enter in the left view moves the right selection with its text: both
	// tell.
	h.key(input.KeyEnter, 0)
	if got := ll.take(); len(got) != 1 || got[0] != caretRange(pos(1, 0)) {
		t.Errorf("left told %v", got)
	}
	if got := rl.take(); len(got) != 1 || got[0] != caretRange(pos(3, 2)) {
		t.Errorf("right told %v", got)
	}
	// Selecting in one view leaves the other quiet.
	left.SetSelection(caretRange(pos(0, 1)))
	if got := rl.take(); len(got) != 0 {
		t.Errorf("right told %v for a move in the left", got)
	}
}

func TestView_OnSelectionChangeDuringLayoutWaitsForTheNextFrame(t *testing.T) {
	var ctl Controller
	var v View
	var log selectionLog
	log.watch(&v)
	ctl.SetText("text")
	h := newEditor(t, &ctl, Editor{View: &v})
	h.tree.DispatchComposition(&input.Composition{Text: "ab", Cursor: 2})
	h.frame()
	log.take()
	// Losing the focus commits the composition. The focus can move during a
	// layout, as when a find bar takes it, so the change waits for a tick.
	r := v.r
	r.FocusChanged(false)
	if ctl.Text() != "abtext" {
		t.Fatalf("text %q", ctl.Text())
	}
	if got := log.take(); len(got) != 0 {
		t.Errorf("told at once: %v", got)
	}
	h.tree.Tick(time.Second)
	if got := log.take(); len(got) != 1 || got[0] != caretRange(pos(0, 2)) {
		t.Errorf("the next frame told %v", got)
	}
}

func TestView_OnSelectionChangeAllocatesNothing(t *testing.T) {
	var ctl Controller
	var v View
	n := 0
	v.OnSelectionChange = func(TextRange) { n++ }
	ctl.SetText(numbered(2000))
	h := newEditor(t, &ctl, Editor{View: &v, LineNumbers: true})
	c := geom.Tight(h.size)
	var cv paint.Canvas
	step := 0
	run := func() {
		step++
		// Keys allocate on their own way in, with or without the callback.
		ctl.SetSelection(caretRange(pos(step%40, step%3)))
		h.tree.Layout(c)
		cv.Reset()
		h.tree.Paint(&cv)
	}
	// Once over every line it visits, so that they are laid out.
	for range 120 {
		run()
	}
	if allocs := testing.AllocsPerRun(20, run); allocs != 0 {
		t.Errorf("%v allocations per move", allocs)
	}
	if n == 0 {
		t.Error("nothing told")
	}
}
