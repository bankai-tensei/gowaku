package text

import (
	"sort"

	"github.com/bankai-tensei/gowaku/text/jlreq"
)

// wordKind groups characters that a double click selects together.
type wordKind uint8

const (
	wordOther wordKind = iota
	wordSpace
	wordWestern
	wordKanji
	wordHiragana
	wordKatakana
)

func kindOf(r rune) wordKind {
	switch jlreq.Classify(r) {
	case jlreq.ClassSpace:
		return wordSpace
	case jlreq.ClassWestern:
		return wordWestern
	case jlreq.ClassIdeograph, jlreq.ClassIteration:
		return wordKanji
	case jlreq.ClassKana, jlreq.ClassSmallKana, jlreq.ClassProlonged:
		if r >= 0x30A0 && r <= 0x30FF {
			return wordKatakana
		}
		return wordHiragana
	}
	return wordOther
}

// clusterAt returns the index of the first cluster that ends after offset.
func (p *Paragraph) clusterAt(offset int) int {
	return sort.Search(len(p.clusters), func(i int) bool { return p.clusters[i].end > offset })
}

// NextCaret returns the caret position after offset. It steps over one grapheme
// cluster or line break, and stays at the end of the text.
func (l *Layout) NextCaret(offset int) int {
	p := l.paragraph
	if offset >= len(p.text) {
		return len(p.text)
	}
	if n := lineBreakAt(p.text, offset); n > 0 {
		return offset + n
	}
	if i := p.clusterAt(offset); i < len(p.clusters) {
		return p.clusters[i].end
	}
	return len(p.text)
}

// PrevCaret returns the caret position before offset. It steps over one
// grapheme cluster or line break, and stays at the start of the text.
func (l *Layout) PrevCaret(offset int) int {
	p := l.paragraph
	if offset <= 0 {
		return 0
	}
	if p.text[offset-1] == '\n' {
		if offset >= 2 && p.text[offset-2] == '\r' {
			return offset - 2
		}
		return offset - 1
	}
	i := p.clusterAt(offset - 1)
	if i < len(p.clusters) {
		return p.clusters[i].start
	}
	return offset - 1
}

// lineBreakAt returns the length of the line break at offset, or 0.
func lineBreakAt(s string, offset int) int {
	switch {
	case s[offset] == '\n':
		return 1
	case s[offset] == '\r' && offset+1 < len(s) && s[offset+1] == '\n':
		return 2
	}
	return 0
}

// WordRange returns the word around offset: a run of clusters of one kind, such
// as kanji, hiragana or Western letters. A click past the last word of a line
// gets that word.
func (l *Layout) WordRange(offset int) (start, end int) {
	p := l.paragraph
	if len(p.clusters) == 0 {
		return offset, offset
	}
	i := min(p.clusterAt(offset), len(p.clusters)-1)
	kind := p.wordKindOf(i)
	first, last := i, i
	for first > 0 && p.wordKindOf(first-1) == kind && p.clusters[first-1].end == p.clusters[first].start {
		first--
	}
	for last+1 < len(p.clusters) && p.wordKindOf(last+1) == kind && p.clusters[last].end == p.clusters[last+1].start {
		last++
	}
	return p.clusters[first].start, p.clusters[last].end
}

func (p *Paragraph) wordKindOf(i int) wordKind { return kindOf(p.units[i].Rune) }

// NextWord returns the start of the word after offset, for Ctrl+Right.
func (l *Layout) NextWord(offset int) int {
	p := l.paragraph
	if offset >= len(p.text) {
		return len(p.text)
	}
	_, end := l.WordRange(offset)
	end = max(end, l.NextCaret(offset))
	for i := p.clusterAt(end); i < len(p.clusters) && p.wordKindOf(i) == wordSpace; i++ {
		end = p.clusters[i].end
	}
	return end
}

// PrevWord returns the start of the word before offset, for Ctrl+Left.
func (l *Layout) PrevWord(offset int) int {
	p := l.paragraph
	if offset <= 0 {
		return 0
	}
	at := l.PrevCaret(offset)
	for i := p.clusterAt(at); i >= 0 && i < len(p.clusters) && p.wordKindOf(i) == wordSpace && at > 0; i-- {
		at = l.PrevCaret(at)
	}
	start, _ := l.WordRange(at)
	return min(start, at)
}

// LineStart returns the start of the line holding the caret at offset.
func (l *Layout) LineStart(offset int) int { return l.lines[l.lineAt(offset)].start }

// LineEnd returns the end of the line holding the caret at offset, before
// trailing spaces that a wrapped line keeps.
func (l *Layout) LineEnd(offset int) int {
	i := l.lineAt(offset)
	end := l.lines[i].end
	if i+1 < len(l.lines) && l.lines[i+1].start == end {
		// A soft wrap: the offset at the end belongs to the next line.
		return l.PrevCaret(end)
	}
	return end
}

// VerticalMove returns the caret position lines rows from offset, closest to
// the position x along the line, as CaretPosition gives it. It stays on the
// first or last line. In vertical text a row is a column, and a positive lines
// moves left.
func (l *Layout) VerticalMove(offset, lines int, x float32) int {
	i := min(max(l.lineAt(offset)+lines, 0), len(l.lines)-1)
	return l.hitLine(i, x)
}
