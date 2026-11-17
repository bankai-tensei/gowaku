package editor

import (
	"slices"
	"sync"
	"sync/atomic"
)

// bigSearch is how long a text in memory may be for a search on the UI
// goroutine. A longer one, or a file, is searched on a goroutine of its own.
const bigSearch = 4 << 20

// findBatchLines is how many lines the search goroutine searches between
// reports.
const findBatchLines = 1 << 16

// findJob is a search running on a goroutine of its own, over a snapshot of
// the text taken when it began and the lines a file grew by since. It hands
// its matches over through pending, numbered as the lines were when each
// segment was queued; the UI goroutine moves them through the edits made
// since (finder.log).
type findJob struct {
	q    finder
	stop atomic.Bool
	wake func()

	mu sync.Mutex
	// segs are the segments not searched yet, and running says the goroutine
	// has not ended.
	segs    []findSeg
	running bool
	pending []findResult
}

// findSeg is a run of lines to search: lines in memory, or lines from to to-1
// of a source, read by rd. line is the number of its first line when it was
// queued, which is entry logAt of the log of edits.
type findSeg struct {
	line, logAt int
	lines       []string
	rd          LineReader
	from, to    int
}

// findResult is a report of the search goroutine. done says it has ended.
type findResult struct {
	logAt   int
	matches []TextRange
	done    bool
}

// queue adds segments, and starts the goroutine if it is not running. It
// runs on the UI goroutine.
func (j *findJob) queue(s ...findSeg) {
	j.mu.Lock()
	j.segs = append(j.segs, s...)
	start := !j.running
	j.running = true
	j.mu.Unlock()
	if start {
		go j.run()
	}
}

// findHold, when not nil, runs on the search goroutine before each segment,
// so that a test can hold the search while it edits.
var findHold func()

// run searches the segments until none is left.
func (j *findJob) run() {
	hold := findHold
	for {
		if hold != nil {
			hold()
		}
		j.mu.Lock()
		if len(j.segs) == 0 || j.stop.Load() {
			// The readers of the segments left go with them.
			for _, s := range j.segs {
				if s.rd != nil {
					_ = s.rd.Close() //nolint:errcheck // only read
				}
			}
			j.segs = nil
			j.running = false
			j.pending = append(j.pending, findResult{done: true})
			j.mu.Unlock()
			j.wake()
			return
		}
		s := j.segs[0]
		j.segs = j.segs[1:]
		j.mu.Unlock()
		j.search(s)
	}
}

// report hands matches over.
func (j *findJob) report(logAt int, m []TextRange) {
	if len(m) == 0 {
		return
	}
	j.mu.Lock()
	j.pending = append(j.pending, findResult{logAt: logAt, matches: m})
	j.mu.Unlock()
	j.wake()
}

// take returns the reports handed over.
func (j *findJob) take() []findResult {
	j.mu.Lock()
	defer j.mu.Unlock()
	r := j.pending
	j.pending = nil
	return r
}

// search searches one segment, reporting every findBatchLines lines.
func (j *findJob) search(s findSeg) {
	var found []TextRange
	count := 0
	line := func(i int, l string) bool {
		found = j.q.searchLine(s.line+i, l, found)
		count++
		if count%findBatchLines == 0 {
			if j.stop.Load() {
				return false
			}
			j.report(s.logAt, found)
			found = nil
		}
		return true
	}
	if s.rd == nil {
		for i, l := range s.lines {
			if !line(i, l) {
				return
			}
		}
		j.report(s.logAt, found)
		return
	}
	defer s.rd.Close() //nolint:errcheck // only read
	stopped := false
	if err := s.rd.ReadLines(s.from, s.to, func(i int, l string) bool {
		stopped = !line(i-s.from, l)
		return !stopped
	}); err != nil || stopped {
		return
	}
	j.report(s.logAt, found)
}

// async reports whether a search of b runs on a goroutine of its own.
func async(b *lineBuffer) bool { return b.src != nil || b.length() > bigSearch }

// segments returns the text of b as segments to search.
func (b *lineBuffer) segments(logAt int) []findSeg {
	var segs []findSeg
	line := 0
	for _, p := range b.pieces {
		if p.source {
			segs = append(segs, b.sourceSeg(p.start, p.n, line, logAt))
		} else {
			segs = append(segs, findSeg{line: line, logAt: logAt, lines: slices.Clone(p.lines)})
		}
		line += p.n
	}
	return segs
}

// sourceSeg returns lines start to start+n of the source, which is line line
// of the text, as a segment.
func (b *lineBuffer) sourceSeg(start, n, line, logAt int) findSeg {
	return findSeg{line: line, logAt: logAt, rd: b.src.Reader(), from: start, to: start + n}
}

// moveThrough moves the matches m, numbered as the lines were at entry from
// of log, through the edits after it. A match on a line an edit replaced
// goes, as the search of the edit found that line anew.
func moveThrough(m []TextRange, log []lineChange) []TextRange {
	if len(log) == 0 {
		return m
	}
	out := m[:0]
	for _, r := range m {
		l := r.Start.Line
		keep := true
		for _, ch := range log {
			switch {
			case l < ch.first:
			case l >= ch.first+ch.removed:
				l += ch.added - ch.removed
			default:
				keep = false
			}
			if !keep {
				break
			}
		}
		if keep {
			d := l - r.Start.Line
			r.Start.Line += d
			r.End.Line += d
			out = append(out, r)
		}
	}
	return out
}

// merge returns the matches of a and b, both in order, in order.
func merge(a, b []TextRange) []TextRange {
	if len(b) == 0 {
		return a
	}
	if len(a) == 0 || a[len(a)-1].Start.Before(b[0].Start) {
		return append(a, b...)
	}
	out := make([]TextRange, 0, len(a)+len(b))
	i, k := 0, 0
	for i < len(a) && k < len(b) {
		if b[k].Start.Before(a[i].Start) {
			out = append(out, b[k])
			k++
		} else {
			out = append(out, a[i])
			i++
		}
	}
	out = append(out, a[i:]...)
	return append(out, b[k:]...)
}

// busy reports whether the goroutine still searches or has reports to hand
// over.
func (j *findJob) busy() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.running || len(j.pending) > 0
}
