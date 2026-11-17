package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/bankai-tensei/gowaku/widgets/editor"
)

// groupLines is how many lines share an entry of the index: a line is found
// by reading its group and cutting it at the breaks, so the index of a file
// of a hundred million lines takes 25 MB.
const groupLines = 64

// fileSource is an editor.LineSource over a file that it reads where its lines
// are asked for. It only reads: the edits stay in the editor. An application
// that saves, follows a growing log or notices a file changed on disk builds
// those on top of a source like this one.
//
// Its fields change on the UI goroutine only. The goroutine that indexes the
// file hands what it finds over through pending, which apply takes.
type fileSource struct {
	f *os.File
	// raw[g] and norm[g] are where line g*groupLines starts in the file and in
	// the text, where a \r\n counts as one byte.
	raw, norm []int64
	// n is the number of lines known, rawEnd where they end in the file, and
	// end where they end in the text, with a break after each.
	n           int
	rawEnd, end int64
	cache       map[int][]string

	mu      sync.Mutex
	pending []step
}

// step is what the indexing goroutine found since its last step.
type step struct {
	raw, norm   []int64
	n           int
	rawEnd, end int64
}

// open opens path and indexes it on a goroutine of its own, which calls found
// after each step it hands over, the last time with done set.
func open(path string, found func(done bool, err error)) (*fileSource, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	s := &fileSource{f: f, raw: []int64{0}, norm: []int64{0}, cache: map[int][]string{}}
	go s.index(found)
	return s, nil
}

// index reads the file from the start to the end, finding its line breaks.
func (s *fileSource) index(found func(done bool, err error)) {
	buf := make([]byte, 1<<20)
	var at, cr, next int64 // next is where the line after the last break starts
	lines, prevCR := 0, false
	var cur step
	hand := func(done bool, err error) {
		cur.n, cur.rawEnd, cur.end = lines, next, next-cr
		if done {
			// The last line runs to the end of the file, without a break.
			cur.n, cur.rawEnd, cur.end = lines+1, at, next-cr+(at-next)+1
		}
		s.mu.Lock()
		s.pending = append(s.pending, cur)
		s.mu.Unlock()
		cur = step{}
		found(done, err)
	}
	for since := 0; ; {
		k, err := s.f.ReadAt(buf, at)
		b := buf[:k]
		for i := 0; i < len(b); i++ {
			e := bytes.IndexByte(b[i:], '\n')
			if e < 0 {
				break
			}
			i += e
			if i > 0 && b[i-1] == '\r' || i == 0 && prevCR {
				cr++
			}
			lines++
			next = at + int64(i) + 1
			if lines%groupLines == 0 {
				cur.raw = append(cur.raw, next)
				cur.norm = append(cur.norm, next-cr)
			}
		}
		if k > 0 {
			prevCR = b[k-1] == '\r'
		}
		at += int64(k)
		switch {
		case errors.Is(err, io.EOF) || err == nil && k == 0:
			hand(true, nil)
			return
		case err != nil:
			hand(true, err)
			return
		}
		if since += k; since >= 32<<20 {
			since = 0
			hand(false, nil)
		}
	}
}

// apply takes the steps of the indexing goroutine. It returns the number of
// lines known before, to tell editor.Controller.SourceGrew.
func (s *fileSource) apply() int {
	s.mu.Lock()
	steps := s.pending
	s.pending = nil
	s.mu.Unlock()
	before := s.n
	for _, st := range steps {
		s.raw = append(s.raw, st.raw...)
		s.norm = append(s.norm, st.norm...)
		s.n, s.rawEnd, s.end = st.n, st.rawEnd, st.end
	}
	// The last group read may have had only some of its lines.
	delete(s.cache, before/groupLines)
	return before
}

// readGroup reads group g of the n lines indexed in raw, which end at rawEnd,
// from f, and cuts it into lines.
func readGroup(f *os.File, raw []int64, n int, rawEnd int64, g int) ([]string, error) {
	count := min(groupLines, n-g*groupLines)
	end := rawEnd
	if g+1 < len(raw) && (g+1)*groupLines <= n {
		end = raw[g+1]
	}
	data := make([]byte, end-raw[g])
	if _, err := f.ReadAt(data, raw[g]); err != nil && !errors.Is(err, io.EOF) {
		return make([]string, count), err
	}
	lines := make([]string, count)
	text := string(data)
	for i := range count {
		k := strings.IndexByte(text, '\n')
		if k < 0 {
			lines[i] = text
			break
		}
		lines[i] = strings.TrimSuffix(text[:k], "\r")
		text = text[k+1:]
	}
	return lines, nil
}

// group returns the lines of group g, read from the file once.
func (s *fileSource) group(g int) []string {
	if lines, ok := s.cache[g]; ok {
		return lines
	}
	if len(s.cache) >= 1024 {
		clear(s.cache)
	}
	lines, _ := readGroup(s.f, s.raw, s.n, s.rawEnd, g) //nolint:errcheck // a group that cannot be read shows empty lines
	s.cache[g] = lines
	return lines
}

// Len implements editor.LineSource.
func (s *fileSource) Len() int { return max(s.n, 1) }

// Line implements editor.LineSource.
func (s *fileSource) Line(i int) string {
	if s.n == 0 {
		return ""
	}
	return s.group(i / groupLines)[i%groupLines]
}

// Offset implements editor.LineSource.
func (s *fileSource) Offset(i int) int64 {
	if i >= s.n {
		return s.end
	}
	g := i / groupLines
	off := s.norm[g]
	for _, l := range s.group(g)[:i%groupLines] {
		off += int64(len(l)) + 1
	}
	return off
}

// LineAt implements editor.LineSource.
func (s *fileSource) LineAt(off int64) int {
	if off >= s.end {
		return max(s.n-1, 0)
	}
	groups := min(len(s.norm), (s.n+groupLines-1)/groupLines)
	g := max(sort.Search(groups, func(g int) bool { return s.norm[g] > off })-1, 0)
	at := s.norm[g]
	lines := s.group(g)
	for k, l := range lines {
		if at += int64(len(l)) + 1; at > off {
			return g*groupLines + k
		}
	}
	return min(g*groupLines+len(lines)-1, s.n-1)
}

// Reader implements editor.LineSource. The reader reads the groups itself,
// on the goroutine of the search, by the index as it is now.
func (s *fileSource) Reader() editor.LineReader {
	return &reader{f: s.f, raw: s.raw, n: s.n, rawEnd: s.rawEnd}
}

// reader reads the lines of a fileSource on another goroutine.
type reader struct {
	f      *os.File
	raw    []int64
	n      int
	rawEnd int64
}

// ReadLines implements editor.LineReader.
func (r *reader) ReadLines(from, to int, fn func(i int, line string) bool) error {
	for g := from / groupLines; g*groupLines < min(to, r.n); g++ {
		lines, err := readGroup(r.f, r.raw, r.n, r.rawEnd, g)
		if err != nil {
			return err
		}
		for k, l := range lines {
			if i := g*groupLines + k; i >= from && i < to && !fn(i, l) {
				return nil
			}
		}
	}
	return nil
}

// Close implements editor.LineReader.
func (r *reader) Close() error { return nil }
