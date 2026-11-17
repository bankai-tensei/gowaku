// Package evqueue is the event queue that the platforms share.
package evqueue

import "github.com/bankai-tensei/gowaku/platform"

// Queue is a FIFO that reuses its storage, so that steady-state event
// delivery does not allocate.
type Queue struct {
	events []platform.Event
	head   int
}

// Push appends ev. A redraw, resize or pointer move replaces an identical-kind
// event for the same window at the tail: a drag produces far more of them than
// can be drawn, and only the latest matters.
func (q *Queue) Push(ev platform.Event) {
	if n := len(q.events); n > q.head {
		last := &q.events[n-1]
		if last.Kind == ev.Kind && last.Window == ev.Window &&
			(ev.Kind == platform.EventRedraw || ev.Kind == platform.EventResize || ev.Kind == platform.EventPointerMove) {
			*last = ev
			return
		}
	}
	q.events = append(q.events, ev)
}

// Pop removes the oldest event and reports whether there was one.
func (q *Queue) Pop() (platform.Event, bool) {
	if q.head == len(q.events) {
		return platform.Event{}, false
	}
	ev := q.events[q.head]
	q.head++
	if q.head == len(q.events) {
		q.events = q.events[:0]
		q.head = 0
	}
	return ev, true
}
