package core

import (
	"sync"
	"testing"
	"time"
)

func TestPoster_RunsOnTheNextTickInOrder(t *testing.T) {
	h := newHarness(leaf{})
	defer h.tree.Dispose()
	post := h.state.Poster()
	woken := 0
	h.tree.SetWake(func() { woken++ })
	var got []int
	for i := range 3 {
		post(func() { got = append(got, i) })
	}
	if len(got) != 0 {
		t.Fatal("posted work ran before the tick")
	}
	if woken != 3 {
		t.Errorf("woken %d times, want 3", woken)
	}
	if !h.tree.Ticking() || !h.tree.Due(0) {
		t.Error("posted work must ask for a tick")
	}
	h.tree.Tick(0)
	if len(got) != 3 || got[0] != 0 || got[2] != 2 {
		t.Errorf("ran %v, want [0 1 2]", got)
	}
	if h.tree.Ticking() {
		t.Error("nothing is left to run")
	}
}

func TestPoster_FromOtherGoroutines(t *testing.T) {
	h := newHarness(leaf{})
	defer h.tree.Dispose()
	post := h.state.Poster()
	var wg sync.WaitGroup
	n := 0
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				post(func() { n++ })
			}
		}()
	}
	wg.Wait()
	h.tree.Tick(time.Millisecond)
	if n != 800 {
		t.Errorf("ran %d, want 800", n)
	}
}

func TestPoster_DropsWorkForAStateThatLeft(t *testing.T) {
	h := newHarness(statefulLeaf("a", nil))
	defer h.tree.Dispose()
	s := h.tree.root.child.state.(*loggingState)
	post := s.Poster()
	ran := false
	post(func() { ran = true })
	h.set(leaf{})
	h.tree.Tick(0)
	if ran {
		t.Error("work posted for a disposed State ran")
	}
}

func TestPoster_WorkPostedWhileRunningWaitsForTheNextTick(t *testing.T) {
	h := newHarness(leaf{})
	defer h.tree.Dispose()
	post := h.state.Poster()
	second := false
	post(func() { post(func() { second = true }) })
	h.tree.Tick(0)
	if second {
		t.Error("work posted by posted work ran in the same tick")
	}
	h.tree.Tick(time.Millisecond)
	if !second {
		t.Error("it must run at the next tick")
	}
}
