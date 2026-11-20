// Package aozora reads the ruby notation of Aozora Bunko: ｜漢字《かんじ》, or
// 漢字《かんじ》 when the base is the run of kanji just before 《.
// See https://www.aozora.gr.jp/annotation/etc.html#ruby.
//
// A 《 that has no base before it, no 》 after it on the same line, or nothing
// between the two is plain text, and so is a ｜ that no ruby follows.
//
// It also reads the notes of emphasis marks (傍点), which follow the text they
// name on the same line: 対象［＃「対象」に傍点］ sets sesame dots beside 対象,
// and 白ゴマ傍点, 丸傍点, 白丸傍点, 二重丸傍点, 蛇の目傍点, 黒三角傍点, 白三角傍点
// and ばつ傍点 set the other marks of text.Emphasis. The named text is read
// without its ruby notation, so ｜青空《あおぞら》［＃「青空」に傍点］ marks 青空
// and keeps its ruby. A note whose text does not end just before it, or that
// starts or ends inside a base with ruby, is plain text, and so are the other
// notes of Aozora Bunko, the ranges ［＃傍点］…［＃傍点終わり］ and the marks on
// the left (［＃「対象」の左に傍点］) among them.
// See https://www.aozora.gr.jp/annotation/emphasis.html.
package aozora

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/text"
)

const (
	// bar marks the start of the base (U+FF5C FULLWIDTH VERTICAL LINE).
	bar = "｜"
	// openMark and closeMark enclose the ruby (U+300A and U+300B).
	openMark  = "《"
	closeMark = "》"
)

// Marks appends the ruby and the notes of emphasis of s to dst, in order, and
// returns the result. A note makes the text it names a mark with Emphasis set
// and no ruby, whose notation is the note; a base with ruby that it names gets
// Emphasis, and its notation reaches over the note. Text that runs over
// several bases with ruby, or over text between them, becomes one mark per
// stretch, and the last holds the note. Marks does not allocate when dst has
// room.
func Marks(s string, dst []text.RubyMark) []text.RubyMark {
	own := len(dst) // the marks before are the caller's
	from, limit := 0, 0
	ruby, rubyOK := next(s, 0)
	note := index(s, 0, openNote)
	for {
		switch {
		case rubyOK && (note < 0 || ruby.Start <= note):
			dst = append(dst, ruby)
			from = ruby.End
		case note >= 0:
			var ok bool
			if dst, from, ok = emphasize(s, dst, firstAfter(dst, own, limit), limit, note); ok {
				limit = from
			} else {
				from = note + len(openNote)
			}
		default:
			return dst
		}
		if rubyOK && ruby.Start < from {
			ruby, rubyOK = next(s, from)
		}
		if note >= 0 && note < from {
			note = index(s, from, openNote)
		}
	}
}

// next returns the first mark of s that starts at from or later. The base of
// a mark cannot reach back past from, where the last mark ended.
func next(s string, from int) (text.RubyMark, bool) {
	done := from
	for from < len(s) {
		i := strings.Index(s[from:], openMark)
		if i < 0 {
			break
		}
		o := from + i
		rubyStart := o + len(openMark)
		c, ok := closing(s, rubyStart)
		if !ok {
			from = rubyStart
			continue
		}
		if c == rubyStart {
			// Empty ruby is plain text.
			from = c + len(closeMark)
			continue
		}
		start, baseStart, ok := base(s, done, o)
		if !ok {
			from = rubyStart
			continue
		}
		return text.RubyMark{
			Start: start, End: c + len(closeMark),
			BaseStart: baseStart, BaseEnd: o,
			RubyStart: rubyStart, RubyEnd: c,
		}, true
	}
	return text.RubyMark{}, false
}

// Spans returns s as spans for basic.RichText or text.Shaper.ShapeSpans: the
// base of each mark with its ruby and its emphasis, and the text between the
// marks as it is. Every span has style, whose zero value takes the style of
// the paragraph. The spans share the memory of s; only the slice of spans and
// one of marks are allocated.
func Spans(s string, style text.Style) []text.Span {
	marks := Marks(s, make([]text.RubyMark, 0, maxMarks(s)))
	n := 0
	at := 0
	for _, m := range marks {
		if m.Start > at {
			n++
		}
		n++
		at = m.End
	}
	if at < len(s) {
		n++
	}
	spans := make([]text.Span, 0, n)
	at = 0
	for _, m := range marks {
		if m.Start > at {
			spans = append(spans, text.Span{Text: s[at:m.Start], Style: style})
		}
		spans = append(spans, text.Span{
			Text: s[m.BaseStart:m.BaseEnd], Style: style, Ruby: s[m.RubyStart:m.RubyEnd], Emphasis: m.Emphasis,
		})
		at = m.End
	}
	if at < len(s) {
		spans = append(spans, text.Span{Text: s[at:], Style: style})
	}
	return spans
}

// closing returns the 》 that closes ruby starting at from. Ruby ends at the
// line, and another 《 before the 》 leaves the first one unclosed.
func closing(s string, from int) (int, bool) {
	for i := from; i < len(s); {
		switch {
		case s[i] == '\n' || s[i] == '\r':
			return 0, false
		case strings.HasPrefix(s[i:], closeMark):
			return i, true
		case strings.HasPrefix(s[i:], openMark):
			return 0, false
		}
		_, n := utf8.DecodeRuneInString(s[i:])
		i += n
	}
	return 0, false
}

// base finds the base of ruby whose 《 is at o, no earlier than done: from the
// last ｜ on the line, or else the run of kanji before o. It returns the start
// of the notation and of the base.
func base(s string, done, o int) (start, baseStart int, ok bool) {
	// One pass back to the start of the line looks for the last bar.
	lineStart := done
	for i := o - 1; i >= done; i-- {
		c := s[i]
		if c == '\n' || c == '\r' {
			lineStart = i + 1
			break
		}
		if c == bar[2] && i-2 >= done && s[i-2:i+1] == bar {
			baseStart = i + 1
			return i - 2, baseStart, baseStart < o
		}
	}
	baseStart = o
	for baseStart > lineStart {
		r, n := utf8.DecodeLastRuneInString(s[lineStart:baseStart])
		if isVariationSelector(r) {
			// A selector picks the glyph of the kanji before it, as in 葛󠄀,
			// and goes with it.
			k, m := utf8.DecodeLastRuneInString(s[lineStart : baseStart-n])
			if !isKanji(k) {
				break
			}
			baseStart -= n + m
			continue
		}
		if !isKanji(r) {
			break
		}
		baseStart -= n
	}
	return baseStart, baseStart, baseStart < o
}

// isKanji reports whether r can be part of a base without a ｜: a Han
// character, or one of the marks that are written among them.
func isKanji(r rune) bool {
	switch r {
	case '々', '〆', 'ヶ', 'ヵ', '〇':
		return true
	}
	return unicode.Is(unicode.Han, r)
}

// isVariationSelector reports whether r is a standardized (SVS) or
// ideographic (IVS) variation selector.
func isVariationSelector(r rune) bool {
	return r >= 0xFE00 && r <= 0xFE0F || r >= 0xE0100 && r <= 0xE01EF
}

// Plain returns s with the notation taken out and the base text kept, as a
// reader sees it without ruby and emphasis marks.
func Plain(s string) string {
	n := maxMarks(s)
	if n == 0 {
		return s
	}
	marks := Marks(s, make([]text.RubyMark, 0, n))
	if len(marks) == 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	at := 0
	for _, m := range marks {
		b.WriteString(s[at:m.Start])
		b.WriteString(s[m.BaseStart:m.BaseEnd])
		at = m.End
	}
	b.WriteString(s[at:])
	return b.String()
}
