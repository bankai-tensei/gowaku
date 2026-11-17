package core

import (
	"slices"
	"time"
)

// Ticker calls a function once per frame while it runs. It is the only way to
// ask for frames: as long as no ticker runs, an idle window draws nothing.
//
// A ticker that has nothing to do for a while, such as a blinking caret, sleeps
// with SleepUntil instead of asking for frames it does not need. A ticker made
// by NewPeriodicTicker does so on its own, between calls.
//
// A ticker made by a State stops when the State leaves the tree, right after
// its Dispose.
//
// The time a ticker's function gets counts from when the App started, and in
// gowakutest from zero, so the first ticks may well get 0 or a few
// milliseconds. Only differences between times mean anything. A time is never
// negative, but one computed from it, such as now minus a position, can be, so
// neither zero nor a negative value works as a mark for "not set yet": keep a
// separate bool.
type Ticker struct {
	tree *Tree
	fn   func(now time.Duration)
	// owner is the element of the State that made the ticker, if a State did.
	owner *element
	// wake is when a sleeping ticker runs again, if sleeping is set.
	wake     time.Duration
	sleeping bool
	// period is the time between calls of a periodic ticker, and zero for one
	// that runs every frame. anchored says whether wake holds the time of the
	// last call, which the next call counts from.
	period   time.Duration
	anchored bool
	// active says whether the ticker is among the tickers of its tree. Tick
	// reads it, as a ticker can stop another one after Tick copied the list.
	active bool
}

// NewTicker returns a stopped Ticker that calls fn. The time fn gets only has
// meaning relative to other ticks, and may start at zero (see Ticker). It
// stops when the State leaves the tree.
func (b *StateBase) NewTicker(fn func(now time.Duration)) *Ticker {
	return &Ticker{tree: b.el.tree, fn: fn, owner: b.el}
}

// NewPeriodicTicker returns a stopped Ticker that calls fn once per period
// instead of once per frame, and lets the window sleep in between. The first
// call comes right away, on the frame after Start, not one period later; the
// next ones keep to the grid of that call and skip what a stalled loop missed,
// so a clock driven by it does not drift. It stops when the State leaves the
// tree. A period of zero or less panics. For a single call after a delay, use
// After.
func (b *StateBase) NewPeriodicTicker(period time.Duration, fn func(now time.Duration)) *Ticker {
	if period <= 0 {
		panic("gowaku: NewPeriodicTicker needs a positive period")
	}
	return &Ticker{tree: b.el.tree, fn: fn, owner: b.el, period: period}
}

// After runs fn once, on the UI goroutine, when d has passed on the clock of
// the tickers, so that gowakutest.Tester.Advance drives it as it drives
// animations. The delay counts from the next tick of the tickers, which comes
// right after the callback that calls After, and at the next frame for a call
// from an InitState that a frame runs. The window sleeps while it waits: no
// frames are drawn for it. A delay of zero or less runs fn at that tick.
//
// The call is canceled when the State leaves the tree, so fn never sees a
// State that is gone. cancel stops it before then; calling cancel after fn ran,
// or more than once, does nothing. Call After from InitState or a callback,
// not from Build. See docs/ADR/0126-dialog-focus-submit-on-enter-after.md.
//
//	s.After(1200*time.Millisecond, func() {
//		s.SetState(func() { s.saved = false })
//	})
func (b *StateBase) After(d time.Duration, fn func()) (cancel func()) {
	var k *Ticker
	armed := false
	var due time.Duration
	k = b.NewTicker(func(now time.Duration) {
		if !armed {
			armed, due = true, now+d
		}
		if now < due {
			k.SleepUntil(due)
			return
		}
		k.Stop()
		fn()
	})
	k.Start()
	return k.Stop
}

// NewTicker returns a stopped Ticker that calls fn, for render objects, which
// must be in the tree. Stop it when the object has no more use for it.
func (b *RenderBase) NewTicker(fn func(now time.Duration)) *Ticker {
	return &Ticker{tree: b.tree, fn: fn}
}

// Start makes the ticker run from the next frame on, also if it was sleeping. A
// periodic ticker starts its grid anew.
func (k *Ticker) Start() {
	k.sleeping = false
	k.anchored = false
	if !k.Running() {
		k.tree.tickers = append(k.tree.tickers, k)
	}
	k.active = true
}

// SleepUntil pauses a running ticker until the time of the tickers reaches wake.
// The window then sleeps as well, instead of drawing frames.
func (k *Ticker) SleepUntil(wake time.Duration) { k.wake, k.sleeping = wake, true }

// Stop ends the calls. A ticker can stop itself from its function.
func (k *Ticker) Stop() {
	k.active = false
	if i := slices.Index(k.tree.tickers, k); i >= 0 {
		k.tree.tickers = slices.Delete(k.tree.tickers, i, i+1)
	}
}

// Running reports whether the ticker runs.
func (k *Ticker) Running() bool { return slices.Contains(k.tree.tickers, k) }

// Ticking reports whether any ticker wants the next frame, or work posted
// through a Poster waits to run.
func (t *Tree) Ticking() bool {
	if t.posting() {
		return true
	}
	for _, k := range t.tickers {
		if !k.sleeping {
			return true
		}
	}
	return false
}

// NextWake returns the earliest time at which a sleeping ticker runs again.
func (t *Tree) NextWake() (wake time.Duration, ok bool) {
	for _, k := range t.tickers {
		if k.sleeping && (!ok || k.wake < wake) {
			wake, ok = k.wake, true
		}
	}
	return wake, ok
}

// Due reports whether a Tick at now would call anything.
func (t *Tree) Due(now time.Duration) bool {
	if t.posting() {
		return true
	}
	for _, k := range t.tickers {
		if !k.sleeping || k.wake <= now {
			return true
		}
	}
	return false
}

// Tick runs the work posted through a Poster, then the tickers that are due.
// Call it at the start of a frame. However many there are, they share that
// one frame.
func (t *Tree) Tick(now time.Duration) {
	t.runPosts()
	t.running = append(t.running[:0], t.tickers...)
	for _, k := range t.running {
		// A ticker that ran earlier in this frame may have stopped this one.
		if !k.active || k.sleeping && k.wake > now {
			continue
		}
		k.sleeping = false
		k.fn(now)
		if k.period > 0 && !k.sleeping {
			k.rest(now)
		}
	}
	clear(t.running)
}

// rest puts a periodic ticker to sleep until its next call. It does nothing to
// a ticker that stopped itself, or chose its own wake with SleepUntil.
func (k *Ticker) rest(now time.Duration) {
	if !k.Running() {
		return
	}
	next := now + k.period
	if k.anchored && k.wake <= now {
		// wake is the call that was due. The next one is the first point of its
		// grid after now, however many a stall skipped.
		next = k.wake + ((now-k.wake)/k.period+1)*k.period
	}
	k.wake, k.sleeping, k.anchored = next, true, true
}

// stopTickersOf stops the tickers that the State of el made.
func (t *Tree) stopTickersOf(el *element) {
	// A plain loop: a closure here would allocate on every unmount, and a list
	// unmounts rows on every frame of a scroll.
	kept := t.tickers[:0]
	for _, k := range t.tickers {
		if k.owner != el {
			kept = append(kept, k)
		} else {
			k.active = false
		}
	}
	clear(t.tickers[len(kept):])
	t.tickers = kept
}
