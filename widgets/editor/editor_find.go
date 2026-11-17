package editor

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxMatches bounds the matches a search keeps, so that a search for "e" in a
// large document stays quick to draw and to update.
const maxMatches = 100_000

// FindOptions says how editor.Controller.Find compares.
type FindOptions struct {
	// MatchCase tells upper and lower case apart.
	MatchCase bool
	// WholeWord takes only matches that neither start nor end inside a word.
	WholeWord bool
	// Regexp reads the query as a regular expression of package regexp. A
	// match never spans a line break.
	Regexp bool
}

// finder is the search of a editor.Controller: the query and its matches,
// which follow every edit.
type finder struct {
	query string
	opts  FindOptions
	re    *regexp.Regexp
	// lower is the query in lower case, for plain search without MatchCase.
	lower string
	// matches are in order and each lies in one line.
	matches []TextRange
	// job is the search of a large text running on a goroutine of its own,
	// and log the edits made since it began, through which its matches move.
	job *findJob
	log []lineChange
}

// active reports whether there is a query.
func (f *finder) active() bool { return f.query != "" }

// set changes the query. A regular expression that does not compile leaves no
// query and returns the error.
func (f *finder) set(query string, opts FindOptions) error {
	f.query, f.opts, f.re, f.matches = query, opts, nil, f.matches[:0]
	if query == "" {
		return nil
	}
	if opts.Regexp || !opts.MatchCase && !isASCII(query) {
		expr := query
		if !opts.Regexp {
			expr = regexp.QuoteMeta(query)
		}
		if !opts.MatchCase {
			expr = "(?i)" + expr
		}
		re, err := regexp.Compile(expr)
		if err != nil {
			f.query = ""
			return err
		}
		f.re = re
	}
	f.lower = strings.ToLower(query)
	return nil
}

func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// all searches the whole text.
func (f *finder) all(b *lineBuffer) {
	f.matches = f.matches[:0]
	if !f.active() {
		return
	}
	for i := range b.lineCount() {
		f.matches = f.searchLine(i, b.line(i), f.matches)
		if len(f.matches) >= maxMatches {
			f.matches = f.matches[:maxMatches]
			return
		}
	}
}

// splice follows an edit: the matches of the replaced lines go, those of the
// new lines come, and those after move.
func (f *finder) splice(b *lineBuffer, ch lineChange) {
	if !f.active() {
		return
	}
	if f.job != nil {
		f.log = append(f.log, ch)
	}
	lo := f.firstInLine(ch.first)
	hi := f.firstInLine(ch.first + ch.removed)
	var found []TextRange
	for i := ch.first; i < ch.first+ch.added; i++ {
		found = f.searchLine(i, b.line(i), found)
	}
	shift := ch.added - ch.removed
	if shift != 0 {
		for k := hi; k < len(f.matches); k++ {
			f.matches[k].Start.Line += shift
			f.matches[k].End.Line += shift
		}
	}
	f.matches = append(f.matches[:lo], append(found, f.matches[hi:]...)...)
	if len(f.matches) > maxMatches {
		f.matches = f.matches[:maxMatches]
	}
}

// searchLine appends the matches in line i, whose text is s, to dst.
func (f *finder) searchLine(i int, s string, dst []TextRange) []TextRange {
	if f.re != nil {
		for _, m := range f.re.FindAllStringIndex(s, -1) {
			if m[0] == m[1] || f.opts.WholeWord && !wholeWord(s, m[0], m[1]) {
				continue
			}
			dst = append(dst, TextRange{Start: TextPosition{i, m[0]}, End: TextPosition{i, m[1]}})
		}
		return dst
	}
	needle := f.query
	index := strings.Index
	if !f.opts.MatchCase {
		// The query is ASCII here, so an ASCII line is searched folding ASCII
		// case in place, without a lowered copy of each line; any other line
		// folds Unicode case.
		if !isASCII(s) {
			return f.searchFolded(i, s, dst)
		}
		needle, index = f.lower, indexASCIIFold
	}
	for at := 0; at <= len(s)-len(needle); {
		k := index(s[at:], needle)
		if k < 0 {
			break
		}
		a, z := at+k, at+k+len(needle)
		if !f.opts.WholeWord || wholeWord(s, a, z) {
			dst = append(dst, TextRange{Start: TextPosition{i, a}, End: TextPosition{i, z}})
		}
		at = z
	}
	return dst
}

// indexASCIIFold returns where q, ASCII in lower case, first occurs in s, an
// ASCII string, folding ASCII case, or -1. It looks for the first byte of q in
// both cases with strings.IndexByte, which scans fast, and keeps where each
// was found so that neither is scanned twice; a lowered copy of each line cost
// an allocation a line, seconds for a log of millions of lines.
func indexASCIIFold(s, q string) int {
	if q == "" {
		return 0
	}
	lo, hi := q[0], q[0]
	if 'a' <= lo && lo <= 'z' {
		hi = lo - 'a' + 'A'
	}
	// nextLo and nextHi are the next places of either case at or after at,
	// or len(s) for none.
	nextLo, nextHi := -1, -1
	find := func(c byte, from int) int {
		if k := strings.IndexByte(s[from:], c); k >= 0 {
			return from + k
		}
		return len(s)
	}
	for at := 0; at <= len(s)-len(q); {
		if nextLo < at {
			nextLo = find(lo, at)
		}
		if hi == lo {
			nextHi = nextLo
		} else if nextHi < at {
			nextHi = find(hi, at)
		}
		k := min(nextLo, nextHi)
		if k > len(s)-len(q) {
			return -1
		}
		if equalASCIIFold(s[k+1:k+len(q)], q[1:]) {
			return k
		}
		at = k + 1
	}
	return -1
}

// equalASCIIFold reports whether s, ASCII, equals q, ASCII in lower case,
// folding ASCII case.
func equalASCIIFold(s, q string) bool {
	for i := range len(q) {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != q[i] {
			return false
		}
	}
	return true
}

// searchFolded searches a line that is not ASCII without regard to case.
func (f *finder) searchFolded(i int, s string, dst []TextRange) []TextRange {
	for at := 0; at < len(s); {
		k := indexFold(s[at:], f.query)
		if k < 0 {
			break
		}
		a := at + k
		z := a + foldedLength(s[a:], f.query)
		if !f.opts.WholeWord || wholeWord(s, a, z) {
			dst = append(dst, TextRange{Start: TextPosition{i, a}, End: TextPosition{i, z}})
		}
		at = max(z, a+1)
	}
	return dst
}

// indexFold returns where q first occurs in s without regard to case, or -1.
func indexFold(s, q string) int {
	for i := range s {
		if !startsRune(s[i]) {
			continue
		}
		if foldedLength(s[i:], q) >= 0 {
			return i
		}
	}
	return -1
}

// foldedLength returns how many bytes at the start of s match q without regard
// to case, or -1 if they do not.
func foldedLength(s, q string) int {
	n := 0
	for _, qc := range q {
		if n >= len(s) {
			return -1
		}
		sc, w := utf8.DecodeRuneInString(s[n:])
		if sc != qc && unicode.SimpleFold(sc) != qc && !foldsTo(sc, qc) {
			return -1
		}
		n += w
	}
	return n
}

func foldsTo(a, b rune) bool {
	for f := unicode.SimpleFold(a); f != a; f = unicode.SimpleFold(f) {
		if f == b {
			return true
		}
	}
	return false
}

// wholeWord reports whether s[a:z] neither starts nor ends inside a word.
func wholeWord(s string, a, z int) bool {
	if a > 0 {
		r, _ := utf8.DecodeLastRuneInString(s[:a])
		first, _ := utf8.DecodeRuneInString(s[a:])
		if isWordRune(r) && isWordRune(first) {
			return false
		}
	}
	if z < len(s) {
		r, _ := utf8.DecodeRuneInString(s[z:])
		last, _ := utf8.DecodeLastRuneInString(s[:z])
		if isWordRune(r) && isWordRune(last) {
			return false
		}
	}
	return true
}

func isWordRune(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

// firstInLine returns the index of the first match on line i or after it.
func (f *finder) firstInLine(i int) int {
	return sort.Search(len(f.matches), func(k int) bool { return f.matches[k].Start.Line >= i })
}

// inLines returns the matches on lines first to last.
func (f *finder) inLines(first, last int) []TextRange {
	return f.matches[f.firstInLine(first):f.firstInLine(last+1)]
}

// next returns the index of the first match that starts at p or after it,
// wrapping around to the first, or -1 without matches.
func (f *finder) next(p TextPosition) int {
	if len(f.matches) == 0 {
		return -1
	}
	k := sort.Search(len(f.matches), func(k int) bool { return !f.matches[k].Start.Before(p) })
	if k == len(f.matches) {
		return 0
	}
	return k
}

// previous returns the index of the last match that starts before p, wrapping
// around to the last, or -1 without matches.
func (f *finder) previous(p TextPosition) int {
	if len(f.matches) == 0 {
		return -1
	}
	k := sort.Search(len(f.matches), func(k int) bool { return !f.matches[k].Start.Before(p) }) - 1
	if k < 0 {
		return len(f.matches) - 1
	}
	return k
}

// index returns the index of the match r, or -1.
func (f *finder) index(r TextRange) int {
	r = r.Ordered()
	k := sort.Search(len(f.matches), func(k int) bool { return !f.matches[k].Start.Before(r.Start) })
	if k < len(f.matches) && f.matches[k] == r {
		return k
	}
	return -1
}

// expand returns what replaces each of matches, which are in order, with the
// replacement s: nil if it is s itself for all of them, or for a regular
// expression s with $1 and the like filled in. The groups are taken from the
// match as it stands in its line, since what is around a match decides what
// \b, ^ or $ in it find. It must be called before any of matches is replaced.
func (f *finder) expand(b *lineBuffer, matches []TextRange, s string) []string {
	if f.re == nil || !f.opts.Regexp {
		return nil
	}
	out := make([]string, 0, len(matches))
	var buf []byte
	for len(matches) > 0 {
		i := matches[0].Start.Line
		n := 1
		for n < len(matches) && matches[n].Start.Line == i {
			n++
		}
		line := b.line(i)
		// The same search as searchLine, so that the same matches come out.
		locs := f.re.FindAllStringSubmatchIndex(line, -1)
		k := 0
		for _, m := range matches[:n] {
			for k < len(locs) && locs[k][0] < m.Start.Column {
				k++
			}
			if k < len(locs) && locs[k][0] == m.Start.Column && locs[k][1] == m.End.Column {
				buf = f.re.ExpandString(buf[:0], s, line, locs[k])
				out = append(out, string(buf))
				continue
			}
			// The matches follow every edit, so this is not reached; the match
			// by itself is the closest there is.
			out = append(out, f.re.ReplaceAllString(line[m.Start.Column:m.End.Column], s))
		}
		matches = matches[n:]
	}
	return out
}
