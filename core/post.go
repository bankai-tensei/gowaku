package core

import "sync"

// postQueue holds the functions that other goroutines posted to a tree, to
// run on the UI goroutine at the next tick.
type postQueue struct {
	mu   sync.Mutex
	fns  []posted
	wake func()
	// running is the storage of runPosts.
	running []posted
}

type posted struct {
	el *element
	fn func()
}

// Poster returns a function that runs fn on the UI goroutine, at the start of
// the next frame, as long as the State is still in the tree then; fn posted
// for a State that has left it is dropped. The function it returns may be
// called from any goroutine, any number of times, and wakes a sleeping
// window. It is how work done elsewhere, as reading or searching a large file
// on a goroutine of its own, reaches the State that shows it:
//
//	post := s.Poster()
//	go func() {
//		n := count(path)
//		post(func() { s.SetState(func() { s.lines = n }) })
//	}()
//
// Call Poster on the UI goroutine, as from InitState or a callback, and hand
// the function it returns to the goroutine; the State itself is not safe to
// touch from there. Posted functions run in the order they were posted, before
// the tickers of that frame, and may call SetState.
func (b *StateBase) Poster() func(fn func()) {
	el := b.el
	t := el.tree
	return func(fn func()) { t.post(el, fn) }
}

// post queues fn for el, and wakes the loop that runs the tree.
func (t *Tree) post(el *element, fn func()) {
	q := &t.posts
	q.mu.Lock()
	q.fns = append(q.fns, posted{el: el, fn: fn})
	wake := q.wake
	q.mu.Unlock()
	if wake != nil {
		wake()
	}
}

// SetWake sets the function that the tree calls, from the goroutine that posts,
// when a State's Poster queues work while the loop may be asleep: the
// application wakes its event loop with it, which then sees Due and ticks.
func (t *Tree) SetWake(wake func()) {
	t.posts.mu.Lock()
	t.posts.wake = wake
	t.posts.mu.Unlock()
}

// posting reports whether functions wait to be run.
func (t *Tree) posting() bool {
	t.posts.mu.Lock()
	defer t.posts.mu.Unlock()
	return len(t.posts.fns) > 0
}

// runPosts runs the functions posted so far whose State is still in the tree.
// Functions posted while they run wait for the next tick.
func (t *Tree) runPosts() {
	q := &t.posts
	q.mu.Lock()
	if len(q.fns) == 0 {
		q.mu.Unlock()
		return
	}
	q.fns, q.running = q.running[:0], q.fns
	q.mu.Unlock()
	for i, p := range q.running {
		q.running[i] = posted{}
		if p.el.mounted {
			p.fn()
		}
	}
}
