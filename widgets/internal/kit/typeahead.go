package kit

import (
	"strings"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/input"
)

// typeAheadPause is how long a pause in typing starts the search anew.
const typeAheadPause = time.Second

// TypeAhead finds a choice by the first letters of its label as they are
// typed, the way native lists do. Letters typed in quick succession build up
// a prefix; the same letter typed again cycles through the labels that start
// with it.
type TypeAhead struct {
	Typed  []byte
	Ticker *core.Ticker
	// armed means the ticker has set deadline for the pause.
	Armed    bool
	deadline time.Duration
}

// Attach belongs in InitState.
func (t *TypeAhead) Attach(base *core.StateBase) { t.Ticker = base.NewTicker(t.tick) }

func (t *TypeAhead) tick(now time.Duration) {
	if !t.Armed {
		t.Armed, t.deadline = true, now+typeAheadPause
	}
	if now >= t.deadline {
		t.Typed = t.Typed[:0]
		t.Ticker.Stop()
		return
	}
	t.Ticker.SleepUntil(t.deadline)
}

// LetterOf returns the lower case letter or digit that e types, or 0. Keys
// with Ctrl or Alt type nothing.
func LetterOf(e *core.KeyEvent) byte {
	if e.Modifiers&(input.ModControl|input.ModAlt) != 0 {
		return 0
	}
	switch k := e.Key; {
	case k >= input.KeyA && k <= input.KeyZ:
		return byte(k-input.KeyA) + 'a'
	case k >= input.Key0 && k <= input.Key9:
		return byte(k-input.Key0) + '0'
	}
	return 0
}

// Find adds c to what was typed and returns the choice to go to from the
// current one, or -1 if none matches. enabled says which labels can be
// chosen.
func (t *TypeAhead) Find(c byte, labels []string, enabled func(i int) bool, current int) int {
	t.Typed = append(t.Typed, c)
	t.Armed = false
	t.Ticker.Start()
	prefix := string(t.Typed)
	start := max(current, 0)
	if same(t.Typed) {
		// One letter again and again steps to the next label with it.
		prefix = prefix[:1]
		start = current + 1
	}
	n := len(labels)
	for k := range n {
		i := (start + k) % n
		if enabled(i) && strings.HasPrefix(strings.ToLower(labels[i]), prefix) {
			return i
		}
	}
	return -1
}

// same reports whether every byte of b is the same.
func same(b []byte) bool {
	for _, c := range b {
		if c != b[0] {
			return false
		}
	}
	return true
}
