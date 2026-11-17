package field

import (
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/width"
)

// The scores of Fuzzy, as its doc comment tells. See
// docs/ADR/0097-command-palette.md.
const (
	fuzzyMatch       = 16
	fuzzyWordStart   = 10
	fuzzyLabelStart  = 6
	fuzzyConsecutive = 8
	fuzzyGapOpen     = 3
	fuzzyGapExtend   = 1
	// fuzzyLead is what each rune before the first match costs, up to
	// fuzzyLeadMax, so that a match nearer the start wins a tie.
	fuzzyLead    = 1
	fuzzyLeadMax = 8
)

// FuzzyMaxQuery and FuzzyMaxLabel bound the work of one Fuzzy.Match: a query
// of more runes than FuzzyMaxQuery matches nothing, and the runes of a label
// past the first FuzzyMaxLabel are not searched.
const (
	FuzzyMaxQuery = 64
	FuzzyMaxLabel = 256
)

// fuzzyNone marks a cell of the table that no match reaches.
const fuzzyNone = -1 << 30

// Fuzzy is the matcher of CommandPalette, for an application that filters
// lists of its own the same way, as a quick open of files does:
//
//	var f field.Fuzzy // kept in the State, so that its buffers are reused
//	f.SetQuery(query)
//	for _, name := range files {
//		if score, ok := f.Match(name); ok {
//			hits = append(hits, hit{name, score, slices.Clone(f.Ranges())})
//		}
//	}
//
// FuzzySpans then shows the matched runes of a label as the palette does.
//
// A label matches when the runes of the query appear in it in order, not
// necessarily side by side; spaces in the query are ignored. Runes compare
// after folding case (unicode.ToLower) and width: full-width letters, digits
// and signs equal their ASCII forms, and half-width katakana the full-width
// ones. Hiragana and katakana stay apart, and no reading is looked up: "ほぞん"
// does not find "保存".
//
// Among the ways to place the query in the label, Match takes the one of the
// highest score. Each matched rune is worth 16, 10 more at the start of a word,
// 6 more again at the start of the label, and 8 more right after the rune
// matched before it. A gap between two matched runes costs 3, and 1 more for
// each rune it skips; each rune before the first match costs 1, up to 8. A word
// starts after a space or a sign, at a capital inside a word (camelCase), and
// where the script changes, as between katakana and kanji, so that "開" starts
// a word in "ファイルを開く". Scores compare the matches of one query: a higher
// score is a better match. An empty query matches every label with 0.
//
// A Fuzzy keeps its buffers between matches: Match allocates only while they
// grow to the longest query and label so far. The zero value is ready to use.
// A Fuzzy is not safe for use by several goroutines at once.
type Fuzzy struct {
	query []rune
	// label holds the folded runes of the label being matched, starts the
	// byte offset of each, and bonus the bonus of matching there. stop is
	// the byte offset where the searched runes end.
	label  []rune
	starts []int
	bonus  []int32
	stop   int
	// score is the table of the best scores, row i for query rune i, and
	// from the column of the previous match on the best path to each cell.
	score []int32
	from  []int16
	// matched holds the byte ranges of the last match, as pairs, and cols
	// the columns of its runes, from the last.
	matched, cols []int
}

// SetQuery sets the query that Match looks for. Spaces, ideographic spaces and
// tabs in it are ignored.
func (f *Fuzzy) SetQuery(q string) {
	f.query = f.query[:0]
	for _, c := range q {
		if c == ' ' || c == '　' || c == '\t' {
			continue
		}
		f.query = append(f.query, foldFuzzy(c))
	}
}

// Empty reports whether the query has no runes, in which case every label
// matches with a score of 0.
func (f *Fuzzy) Empty() bool { return len(f.query) == 0 }

// Match reports whether label matches the query, and how well: a higher score
// is a better match. After a match, Ranges returns where.
func (f *Fuzzy) Match(label string) (score int, ok bool) {
	f.matched = f.matched[:0]
	m := len(f.query)
	if m == 0 {
		return 0, true
	}
	if m > FuzzyMaxQuery {
		return 0, false
	}
	f.load(label)
	n := len(f.label)
	if n < m || !f.subsequence() {
		return 0, false
	}
	if cap(f.score) < m*n {
		f.score = make([]int32, m*n)
		f.from = make([]int16, m*n)
	}
	score32, end := f.fill(m, n)
	f.trace(m, n, end)
	return int(score32), true
}

// Ranges returns where the last successful Match matched, as byte offsets
// into the label it was given (not the folded one): start, end, start, end and
// so on, each pair a half-open range [start, end) on rune boundaries. The
// ranges are in order and do not touch, as runes matched side by side make one
// range. Ranges is empty after an empty query or a failed Match. The slice is
// reused by the next Match; copy it to keep it.
func (f *Fuzzy) Ranges() []int { return f.matched }

// load folds label into the buffers, with the bonus of each rune.
func (f *Fuzzy) load(label string) {
	f.label, f.starts, f.bonus = f.label[:0], f.starts[:0], f.bonus[:0]
	f.stop = len(label)
	prev := rune(0)
	for i, c := range label {
		if len(f.label) == FuzzyMaxLabel {
			f.stop = i
			break
		}
		var b int32
		switch {
		case len(f.label) == 0:
			b = fuzzyWordStart + fuzzyLabelStart
		case wordStart(prev, c):
			b = fuzzyWordStart
		}
		f.label = append(f.label, foldFuzzy(c))
		f.starts = append(f.starts, i)
		f.bonus = append(f.bonus, b)
		prev = c
	}
}

// subsequence reports whether the query is a subsequence of the label at all,
// which most labels fail quickly.
func (f *Fuzzy) subsequence() bool {
	i := 0
	for _, c := range f.label {
		if c == f.query[i] {
			i++
			if i == len(f.query) {
				return true
			}
		}
	}
	return false
}

// fill computes the table and returns the best score and the column where the
// best path ends.
func (f *Fuzzy) fill(m, n int) (int32, int) {
	for i := range m {
		row := f.score[i*n : (i+1)*n]
		from := f.from[i*n : (i+1)*n]
		q := f.query[i]
		if i == 0 {
			for j := range n {
				row[j], from[j] = fuzzyNone, -1
				if f.label[j] == q {
					row[j] = fuzzyMatch + f.bonus[j] - int32(min(j*fuzzyLead, fuzzyLeadMax))
				}
			}
			continue
		}
		prev := f.score[(i-1)*n : i*n]
		// best is the best score of a previous match at least two columns
		// back, less the gap to j-1, and bestAt its column.
		best, bestAt := int32(fuzzyNone), -1
		for j := range n {
			row[j], from[j] = fuzzyNone, -1
			if j >= 2 {
				if best > fuzzyNone {
					best -= fuzzyGapExtend
				}
				if p := prev[j-2]; p > fuzzyNone && p >= best {
					best, bestAt = p, j-2
				}
			}
			// No path reaches column 0 for a query rune after the first.
			if j == 0 || f.label[j] != q {
				continue
			}
			s := int32(fuzzyNone)
			if best > fuzzyNone {
				s, from[j] = best-fuzzyGapOpen, int16(bestAt)
			}
			if p := prev[j-1]; p > fuzzyNone && p+fuzzyConsecutive >= s {
				s, from[j] = p+fuzzyConsecutive, int16(j-1)
			}
			if s > fuzzyNone {
				row[j] = s + fuzzyMatch + f.bonus[j]
			}
		}
	}
	last := f.score[(m-1)*n : m*n]
	best, end := int32(fuzzyNone), -1
	for j, s := range last {
		if s > best {
			best, end = s, j
		}
	}
	return best, end
}

// trace follows the best path back from column end and records the byte
// ranges of its runes.
func (f *Fuzzy) trace(m, n, end int) {
	cols := f.cols[:0]
	j := end
	for i := m - 1; i >= 0; i-- {
		cols = append(cols, j)
		j = int(f.from[i*n+j])
	}
	f.cols = cols
	for k := len(cols) - 1; k >= 0; {
		first := cols[k]
		last := first
		for k > 0 && cols[k-1] == last+1 {
			k--
			last = cols[k]
		}
		k--
		stop := f.stop
		if last+1 < len(f.starts) {
			stop = f.starts[last+1]
		}
		f.matched = append(f.matched, f.starts[first], stop)
	}
}

// foldFuzzy folds c the way Fuzzy compares runes: full-width letters, digits
// and signs to their ASCII forms, half-width katakana to full-width, and to
// lower case.
func foldFuzzy(c rune) rune {
	if c < utf8.RuneSelf {
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		return c
	}
	if folded := width.LookupRune(c).Folded(); folded != 0 {
		c = folded
	}
	return unicode.ToLower(c)
}

// wordStart reports whether c starts a word after prev.
func wordStart(prev, c rune) bool {
	pc, cc := scriptOf(prev), scriptOf(c)
	switch {
	case cc == scriptSeparator:
		return false
	case pc == scriptSeparator:
		return true
	case pc == scriptLatin && cc == scriptLatin:
		// A capital inside a word, as in camelCase.
		return unicode.IsUpper(c) && !unicode.IsUpper(prev)
	}
	return pc != cc
}

// script is the kind of a rune, for finding where words start.
type script uint8

const (
	scriptSeparator script = iota
	scriptLatin
	scriptDigit
	scriptHiragana
	scriptKatakana
	scriptHan
	scriptOther
)

// scriptOf returns the kind of c.
func scriptOf(c rune) script {
	if c < utf8.RuneSelf {
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z':
			return scriptLatin
		case '0' <= c && c <= '9':
			return scriptDigit
		}
		return scriptSeparator
	}
	if folded := width.LookupRune(c).Folded(); folded != 0 {
		c = folded
		if c < utf8.RuneSelf {
			return scriptOf(c)
		}
	}
	switch {
	case c >= 0x3041 && c <= 0x309F:
		return scriptHiragana
	case c >= 0x30A0 && c <= 0x30FF, c >= 0x31F0 && c <= 0x31FF:
		return scriptKatakana
	case unicode.Is(unicode.Han, c), c == '々':
		return scriptHan
	case unicode.IsLetter(c), unicode.IsMark(c):
		if unicode.Is(unicode.Latin, c) {
			return scriptLatin
		}
		return scriptOther
	case unicode.IsDigit(c):
		return scriptDigit
	}
	return scriptSeparator
}
