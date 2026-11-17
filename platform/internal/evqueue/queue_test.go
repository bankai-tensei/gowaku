package evqueue

import (
	"testing"

	"github.com/bankai-tensei/gowaku/platform"
)

func TestEventQueue_FIFO(t *testing.T) {
	var q Queue
	q.Push(platform.Event{Kind: platform.EventClose, Window: 1})
	q.Push(platform.Event{Kind: platform.EventWake})
	for _, want := range []platform.EventKind{platform.EventClose, platform.EventWake} {
		ev, ok := q.Pop()
		if !ok || ev.Kind != want {
			t.Fatalf("got %v %v, want %v", ev.Kind, ok, want)
		}
	}
	if _, ok := q.Pop(); ok {
		t.Error("queue must be empty")
	}
}

func TestEventQueue_MergesConsecutiveResizeOfSameWindow(t *testing.T) {
	var q Queue
	q.Push(platform.Event{Kind: platform.EventResize, Window: 1, PhysicalWidth: 10})
	q.Push(platform.Event{Kind: platform.EventResize, Window: 1, PhysicalWidth: 20})
	ev, _ := q.Pop()
	if ev.PhysicalWidth != 20 {
		t.Errorf("got width %d, want the latest 20", ev.PhysicalWidth)
	}
	if _, ok := q.Pop(); ok {
		t.Error("the two resizes must have merged")
	}
}

func TestEventQueue_KeepsResizeOfDifferentWindows(t *testing.T) {
	var q Queue
	q.Push(platform.Event{Kind: platform.EventResize, Window: 1})
	q.Push(platform.Event{Kind: platform.EventResize, Window: 2})
	q.Pop()
	if _, ok := q.Pop(); !ok {
		t.Error("resize of another window must not merge")
	}
}

func TestEventQueue_NeverMergesClose(t *testing.T) {
	var q Queue
	q.Push(platform.Event{Kind: platform.EventClose, Window: 1})
	q.Push(platform.Event{Kind: platform.EventClose, Window: 1})
	q.Pop()
	if _, ok := q.Pop(); !ok {
		t.Error("close must not merge")
	}
}

func TestEventQueue_DoesNotMergeWithConsumedEvent(t *testing.T) {
	var q Queue
	q.Push(platform.Event{Kind: platform.EventResize, Window: 1, PhysicalWidth: 1})
	q.Push(platform.Event{Kind: platform.EventWake})
	q.Pop()
	q.Pop()
	q.Push(platform.Event{Kind: platform.EventResize, Window: 1, PhysicalWidth: 2})
	if ev, ok := q.Pop(); !ok || ev.PhysicalWidth != 2 {
		t.Errorf("got %v %v, want the new resize", ev, ok)
	}
}

func TestEventQueue_SteadyStateDoesNotAllocate(t *testing.T) {
	var q Queue
	q.Push(platform.Event{Kind: platform.EventWake})
	q.Pop()
	allocs := testing.AllocsPerRun(100, func() {
		q.Push(platform.Event{Kind: platform.EventRedraw, Window: 1})
		q.Pop()
	})
	if allocs != 0 {
		t.Errorf("got %v allocs, want 0", allocs)
	}
}
