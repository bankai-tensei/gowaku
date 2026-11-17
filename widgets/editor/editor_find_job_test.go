package editor

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

// bigLog returns n lines of a log, an error on every 97th, well over the
// 4 MB a search on the UI goroutine takes for n of 60,000 and more.
func bigLog(n int) string {
	var b strings.Builder
	for i := range n {
		level := "INFO "
		if i%97 == 0 {
			level = "ERROR"
		}
		fmt.Fprintf(&b, "2026-11-17T10:00:%02d.123Z %s request %d handled path=/api/v1/items\n", i%60, level, i)
	}
	return b.String()
}

// wantMatches searches the text of c on the UI goroutine, as a short text is.
func wantMatches(t *testing.T, c *Controller, query string, opts FindOptions) []TextRange {
	t.Helper()
	var f finder
	if err := f.set(query, opts); err != nil {
		t.Fatal(err)
	}
	f.all(&c.buf)
	return f.matches
}

// waitFind waits for the search goroutine of c, taking its reports as the UI
// goroutine would.
func waitFind(t *testing.T, c *Controller) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for c.Searching() {
		if time.Now().After(deadline) {
			t.Fatal("the search never ended")
		}
		time.Sleep(time.Millisecond)
		c.applyFind()
	}
}

func sameMatches(t *testing.T, got, want []TextRange) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%d matches, want %d", len(got), len(want))
	}
	for k := range want {
		if got[k] != want[k] {
			t.Fatalf("match %d = %v, want %v", k, got[k], want[k])
		}
	}
}

// postNow makes the posts of c run at once on the goroutine that posts, which
// is enough for a test that only waits with waitFind.
func postNow(c *Controller) {
	none := func(func()) {}
	c.post.Store(&none)
}

func TestFind_ALongTextIsSearchedOnAGoroutine(t *testing.T) {
	var ctl Controller
	postNow(&ctl)
	ctl.SetText(bigLog(80_000))
	if ctl.Len() <= bigSearch {
		t.Fatalf("the text is only %d bytes", ctl.Len())
	}
	n, err := ctl.Find("error", FindOptions{})
	if err != nil || n != 0 || !ctl.Searching() {
		t.Fatalf("Find = %d, %v, searching %v; want 0 at once and a search under way", n, err, ctl.Searching())
	}
	waitFind(t, &ctl)
	sameMatches(t, ctl.Matches(), wantMatches(t, &ctl, "error", FindOptions{}))
	if len(ctl.Matches()) != 825 {
		t.Errorf("%d matches, want 825", len(ctl.Matches()))
	}
}

// TestFind_ASourceWhileItGrows searches a source that learns its lines in
// steps, as a file being read does: the search goes on into each step.
func TestFind_ASourceWhileItGrows(t *testing.T) {
	var ctl Controller
	postNow(&ctl)
	src := newMemSource(strings.ReplaceAll(bigLog(3000), "\n", "\r\n"))
	src.n = 100
	ctl.SetSource(src)
	if _, err := ctl.Find(`request \d+5 `, FindOptions{Regexp: true}); err != nil {
		t.Fatal(err)
	}
	for src.n < len(src.lines) {
		waitFind(t, &ctl)
		from := src.n
		src.n = min(src.n+437, len(src.lines))
		ctl.SourceGrew(from)
	}
	waitFind(t, &ctl)
	sameMatches(t, ctl.Matches(), wantMatches(t, &ctl, `request \d+5 `, FindOptions{Regexp: true}))
	// The request numbers of two digits and more that end in 5.
	if len(ctl.Matches()) != 299 {
		t.Errorf("%d matches, want 299", len(ctl.Matches()))
	}
}

// TestFind_EditsWhileSearching holds the search goroutine, edits the text
// before, inside and after the lines it is to search, and lets it go: its
// matches move through the edits and join those of the edited lines.
func TestFind_EditsWhileSearching(t *testing.T) {
	gate := make(chan struct{})
	findHold = func() { <-gate }
	t.Cleanup(func() { findHold = nil })
	var ctl Controller
	postNow(&ctl)
	ctl.SetText(bigLog(80_000))
	if _, err := ctl.Find("ERROR", FindOptions{MatchCase: true}); err != nil {
		t.Fatal(err)
	}
	edits := []struct {
		at   TextPosition
		to   TextPosition
		with string
	}{
		{TextPosition{0, 0}, TextPosition{0, 0}, "ERROR first\nand a line\n"},
		{TextPosition{40_000, 0}, TextPosition{40_003, 0}, "ERROR in the middle\n"},
		{TextPosition{79_000, 5}, TextPosition{79_000, 5}, " ERROR "},
		{TextPosition{97 * 3, 0}, TextPosition{97*3 + 1, 0}, ""},
	}
	for _, e := range edits {
		ctl.Replace(TextRange{Start: e.at, End: e.to}, e.with)
	}
	close(gate)
	waitFind(t, &ctl)
	sameMatches(t, ctl.Matches(), wantMatches(t, &ctl, "ERROR", FindOptions{MatchCase: true}))
}

func TestFind_ANewQueryDropsTheOldSearch(t *testing.T) {
	gate := make(chan struct{})
	findHold = func() { <-gate }
	t.Cleanup(func() { findHold = nil })
	var ctl Controller
	postNow(&ctl)
	ctl.SetText(bigLog(80_000))
	if _, err := ctl.Find("INFO", FindOptions{}); err != nil {
		t.Fatal(err)
	}
	old := ctl.find.job
	if _, err := ctl.Find("handled path", FindOptions{}); err != nil {
		t.Fatal(err)
	}
	close(gate)
	waitFind(t, &ctl)
	if !old.stop.Load() {
		t.Error("the old search goes on")
	}
	sameMatches(t, ctl.Matches(), wantMatches(t, &ctl, "handled path", FindOptions{}))
}

func TestFind_TheFindBarCountsWhileSearching(t *testing.T) {
	gate := make(chan struct{})
	findHold = func() { <-gate }
	t.Cleanup(func() { findHold = nil })
	var ctl Controller
	ctl.SetText(bigLog(80_000))
	h := newEditor(t, &ctl, Editor{})
	r := h.view()
	r.state.openFind(false)
	h.frame()
	r.state.queryChanged("ERROR")
	h.frame()
	if !hasLabel(h.tree.Semantics(), "Searching…") {
		t.Error("the bar must say it searches")
	}
	close(gate)
	// The matches reach the editor through the Poster of its State.
	wtest.WaitUntil(t, h.tree, h.size, func() bool { return !ctl.Searching() })
	h.frame()
	if !hasLabel(h.tree.Semantics(), "825") {
		t.Error("the bar must count the 825 matches")
	}
}

// hasLabel reports whether a node under n has a label that contains s.
func hasLabel(n *semantics.Node, s string) bool {
	if n == nil {
		return false
	}
	if strings.Contains(n.Label, s) || strings.Contains(n.Value, s) {
		return true
	}
	for _, c := range n.Children {
		if hasLabel(c, s) {
			return true
		}
	}
	return false
}
