package core

import (
	"testing"
	"time"
)

func TestTicker_StoppedByAnEarlierOneDoesNotRunThatFrame(t *testing.T) {
	h := newHarness(leaf{})
	tree := h.tree
	defer tree.Dispose()
	var later *Ticker
	calls := 0
	first := h.state.NewTicker(func(time.Duration) { later.Stop() })
	later = h.state.NewTicker(func(time.Duration) { calls++ })
	first.Start()
	later.Start()
	tree.Tick(0)
	if calls != 0 {
		t.Errorf("a stopped ticker ran %d times", calls)
	}
	later.Start()
	tree.Tick(time.Millisecond)
	if calls != 0 {
		t.Errorf("stopped again, yet it ran %d times", calls)
	}
	first.Stop()
	later.Start()
	tree.Tick(2 * time.Millisecond)
	if calls != 1 {
		t.Errorf("a restarted ticker ran %d times, want 1", calls)
	}
}
