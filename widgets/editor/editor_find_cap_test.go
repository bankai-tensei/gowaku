package editor

import (
	"strings"
	"sync/atomic"
	"testing"
)

// countReader is a LineReader over lines in memory that counts the lines it
// hands out after stop was set, and sets it itself after stopAt lines.
type countReader struct {
	lines  []string
	stop   *atomic.Bool
	stopAt int
	after  int
}

func (r *countReader) ReadLines(from, to int, fn func(i int, line string) bool) error {
	for i := from; i < to; i++ {
		if r.stop.Load() {
			r.after++
		}
		if i-from == r.stopAt {
			r.stop.Store(true)
		}
		if !fn(i, r.lines[i]) {
			return nil
		}
	}
	return nil
}

func (*countReader) Close() error { return nil }

func newJob(t *testing.T, query string) *findJob {
	t.Helper()
	j := &findJob{wake: func() {}}
	if err := j.q.set(query, FindOptions{}); err != nil {
		t.Fatal(err)
	}
	return j
}

// reported returns how many matches the reports of j hold.
func reported(j *findJob) int {
	n := 0
	for _, r := range j.take() {
		n += len(r.matches)
	}
	return n
}

// TestFindJob_KeepsToMaxMatchesWithinABatch searches long lines full of the
// query, far fewer lines than a batch: the search must stop at maxMatches
// instead of gathering every match of the batch first.
func TestFindJob_KeepsToMaxMatchesWithinABatch(t *testing.T) {
	line := strings.Repeat("e", 2000)
	lines := make([]string, 1000) // 2,000,000 matches, all in one batch
	for i := range lines {
		lines[i] = line
	}
	j := newJob(t, "e")
	j.search(findSeg{lines: lines})
	if n := reported(j); n != maxMatches {
		t.Errorf("%d matches reported, want %d", n, maxMatches)
	}
	if !j.stop.Load() {
		t.Error("a search that reached maxMatches must stop")
	}
}

// TestFindJob_KeepsToMaxMatchesInOneLine searches one line with more matches
// than maxMatches.
func TestFindJob_KeepsToMaxMatchesInOneLine(t *testing.T) {
	j := newJob(t, "e")
	j.search(findSeg{lines: []string{strings.Repeat("e", 3*maxMatches)}})
	if n := reported(j); n != maxMatches {
		t.Errorf("%d matches reported, want %d", n, maxMatches)
	}
}

// TestFindJob_KeepsToMaxMatchesAcrossSegments counts the matches reported by
// earlier segments against the cap.
func TestFindJob_KeepsToMaxMatchesAcrossSegments(t *testing.T) {
	j := newJob(t, "e")
	half := []string{strings.Repeat("e", maxMatches/2+1)}
	j.search(findSeg{lines: half})
	j.search(findSeg{line: 1, lines: half})
	if n := reported(j); n != maxMatches {
		t.Errorf("%d matches reported, want %d", n, maxMatches)
	}
}

// TestFindJob_StopsWithinABatch stops the search after a few lines of a
// segment shorter than a batch: it must read no further.
func TestFindJob_StopsWithinABatch(t *testing.T) {
	lines := make([]string, findBatchLines-1)
	for i := range lines {
		lines[i] = "a line without the query"
	}
	j := newJob(t, "zzz")
	rd := &countReader{lines: lines, stop: &j.stop, stopAt: 10}
	j.search(findSeg{rd: rd, from: 0, to: len(lines)})
	if rd.after > 0 {
		t.Errorf("%d lines read after the search was stopped", rd.after)
	}
}

// TestFind_ALongSourceOfFrequentMatches searches a file of long lines for a
// letter on every byte, through the controller.
func TestFind_ALongSourceOfFrequentMatches(t *testing.T) {
	var ctl Controller
	postNow(&ctl)
	ctl.SetSource(newMemSource(strings.Repeat(strings.Repeat("e", 1000)+"\n", 500)))
	if _, err := ctl.Find("e", FindOptions{}); err != nil {
		t.Fatal(err)
	}
	waitFind(t, &ctl)
	if n := len(ctl.Matches()); n != maxMatches {
		t.Errorf("%d matches, want %d", n, maxMatches)
	}
	if last := ctl.Matches()[maxMatches-1]; last != (TextRange{TextPosition{99, 999}, TextPosition{99, 1000}}) {
		t.Errorf("last match %v", last)
	}
}
