package store

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/width"

	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/text/aozora"
)

// snippetRunes is how many runes of the line a hit keeps on each side of the
// match.
const snippetRunes = 30

// Search returns where query appears, the most recently modified notes
// first, at most limit hits. It ignores case and the difference between
// full-width and half-width forms, and matches the base text of ruby, so
// that 漢字 finds ｜漢字《かんじ》.
//
// The hits of a note come in the order of the text. The text is matched as a
// reader sees it, without the notation, and the Offset of a hit is mapped
// back to the file's text, LF line ends and no byte order mark, as ReadDoc
// returns it. Before, Match and After are without the notation too. Files
// that cannot be read or are not UTF-8 are left out. An empty query or a
// limit below one finds nothing. Search is safe for concurrent use.
func (l Library) Search(query string, limit int) ([]Hit, error) {
	q := fold(query)
	if q == "" || limit < 1 {
		return nil, nil
	}
	ents, err := l.scan()
	if err != nil {
		return nil, err
	}
	var hits []Hit
	for _, e := range ents {
		if !e.ok {
			continue
		}
		hits = e.idx.search(e.note, q, hits, limit)
		if len(hits) >= limit {
			break
		}
	}
	return hits, nil
}

// index is the text of a note made ready for search: the plain text, as a
// reader sees it without the notation, and the same text folded, with
// spans that map an offset in the folded text back to the plain text and to
// the text of the file.
//
// The folded text has a rune for each rune of the plain text, so that a
// match found in it is a run of runes of the plain text too.
type index struct {
	plain, folded string
	spans         []span
}

// span says that from the offset f of the folded text on, until the next
// span, an offset x of the folded text is p+x-f in the plain text and r+x-f
// in the raw text.
type span struct {
	f, p, r int32
}

// makeIndex makes the index of raw, a text with LF line ends.
func makeIndex(raw string) index {
	return indexMarks(raw, aozora.Marks(raw, nil))
}

// indexMarks makes the index of raw, whose ruby is marks, in order.
func indexMarks(raw string, marks []text.RubyMark) index {
	plain := make([]byte, 0, len(raw))
	folded := make([]byte, 0, len(raw))
	var spans []span
	var dp, dr int32 = -1, -1 // the deltas of the last span
	mark := 0
	at := func(r int) {
		f, p := int32(len(folded)), int32(len(plain))
		if p-f != dp || int32(r)-f != dr {
			spans = append(spans, span{f: f, p: p, r: int32(r)})
			dp, dr = p-f, int32(r)-f
		}
	}
	for i := 0; i < len(raw); {
		c, n := utf8.DecodeRuneInString(raw[i:])
		for mark < len(marks) && marks[mark].End <= i {
			mark++
		}
		if mark < len(marks) && skipped(marks[mark], i) {
			i += n
			continue
		}
		// ※《 writes a 《 that opens no ruby.
		if c == '※' && strings.HasPrefix(raw[i+n:], "《") && (mark >= len(marks) || i < marks[mark].Start) {
			i += n
			continue
		}
		at(i)
		plain = append(plain, raw[i:i+n]...)
		folded = utf8.AppendRune(folded, foldRune(c))
		i += n
	}
	at(len(raw))
	return index{plain: string(plain), folded: string(folded), spans: spans}
}

// skipped reports whether the byte at i is notation of m, rather than its
// base text or text outside it.
func skipped(m text.RubyMark, i int) bool {
	if i < m.Start || i >= m.End {
		return false
	}
	return i < m.BaseStart || i >= m.BaseEnd
}

// at returns the offsets in the plain text and the raw text of the offset f
// of the folded text.
func (x *index) at(f int) (p, r int) {
	i := sort.Search(len(x.spans), func(i int) bool { return int(x.spans[i].f) > f }) - 1
	s := x.spans[i]
	return int(s.p) + f - int(s.f), int(s.r) + f - int(s.f)
}

// search appends to hits the matches of q, a folded query, in the text of n,
// until there are limit hits.
func (x *index) search(n Note, q string, hits []Hit, limit int) []Hit {
	line, lineAt := 0, 0 // the line of the plain text at the offset lineAt
	for from := 0; len(hits) < limit; {
		i := strings.Index(x.folded[from:], q)
		if i < 0 {
			break
		}
		fs := from + i
		fe := fs + len(q)
		from = fe
		ps, rs := x.at(fs)
		pe, _ := x.at(fe)

		line += strings.Count(x.plain[lineAt:ps], "\n")
		lineAt = ps
		start := strings.LastIndexByte(x.plain[:ps], '\n') + 1
		end := len(x.plain)
		if j := strings.IndexByte(x.plain[pe:], '\n'); j >= 0 {
			end = pe + j
		}
		hits = append(hits, Hit{
			Note:   n,
			Offset: rs,
			Line:   line,
			Before: lastRunes(x.plain[start:ps], snippetRunes),
			Match:  x.plain[ps:pe],
			After:  cutRunes(x.plain[pe:end], snippetRunes),
		})
	}
	return hits
}

// lastRunes returns the last n runes of s.
func lastRunes(s string, n int) string {
	i := len(s)
	for ; i > 0 && n > 0; n-- {
		_, size := utf8.DecodeLastRuneInString(s[:i])
		i -= size
	}
	return s[i:]
}

// fold returns s with each rune folded by foldRune.
func fold(s string) string {
	b := make([]byte, 0, len(s))
	for _, c := range s {
		b = utf8.AppendRune(b, foldRune(c))
	}
	return string(b)
}

// foldRune returns c in the form search compares: half-width for full-width
// letters, digits and signs, full-width for half-width katakana, and lower
// case. It maps one rune to one rune, so that the folded text keeps the runes
// of the text it folds.
func foldRune(c rune) rune {
	if c < utf8.RuneSelf {
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		return c
	}
	if f := width.LookupRune(c).Folded(); f != 0 {
		c = f
	}
	return unicode.ToLower(c)
}
