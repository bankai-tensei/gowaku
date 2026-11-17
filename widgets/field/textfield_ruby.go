package field

import (
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/text"
)

// A Input with Ruby shows its text through a projection: the notation of
// every mark the selection does not reach into is hidden, and its base carries
// the ruby. The layout is of the shown text. Offsets of the display, the text
// with the composition, map to offsets of the shown text and back. See
// Input.Ruby.

// rubyView is the projection of the ruby notation of a field.
type rubyView struct {
	// marks are the marks of the text, and fresh says they are of the text as
	// it is now.
	marks []text.RubyMark
	fresh bool
	// found is room for the marks of the lines of an edit.
	found []text.RubyMark
	// active are the marks that an end of the selection is inside, which show
	// their notation, or -1.
	active [2]int
	// hidden are the stretches of the display left out of the shown text, in
	// order.
	hidden []hiddenRun
	spans  []text.Span
}

// hiddenRun is notation left out of the shown text: the ｜ before a base, or
// the ruby with its brackets after it.
type hiddenRun struct {
	// start and end are offsets of the display.
	start, end int
	// at is where the run would be in the shown text, and removed how much of
	// the display the runs before it leave out.
	at, removed int
	// lead is set for the run before a base. A shown offset at it stands for
	// its start, before the notation; at any other run it stands for the end.
	lead bool
}

// toShown returns the shown offset of display offset d. An offset inside
// hidden notation goes to where the notation was.
func (v *rubyView) toShown(d int) int {
	h := v.hidden
	k := sort.Search(len(h), func(i int) bool { return h[i].end > d })
	switch {
	case k < len(h) && h[k].start < d:
		return h[k].at
	case k < len(h):
		return d - h[k].removed
	case k > 0:
		last := &h[k-1]
		return d - last.removed - (last.end - last.start)
	}
	return d
}

// fromShown returns the display offset of shown offset s. Where notation was
// left out, s stands for the outside of the mark: before its ｜, or after its
// 》, so that a caret put there leaves the mark hidden.
func (v *rubyView) fromShown(s int) int {
	h := v.hidden
	k := sort.Search(len(h), func(i int) bool { return h[i].at >= s })
	removed := 0
	switch {
	case k < len(h):
		removed = h[k].removed
	case k > 0:
		removed = h[k-1].removed + h[k-1].end - h[k-1].start
	}
	for ; k < len(h) && h[k].at == s && !h[k].lead; k++ {
		removed += h[k].end - h[k].start
	}
	return s + removed
}

// activeMarks returns the marks that the anchor or the caret is inside.
func (r *RenderEditable) activeMarks() [2]int {
	a := r.markInside(r.Anchor)
	c := r.markInside(r.Caret)
	if c == a {
		c = -1
	}
	return [2]int{a, c}
}

// markInside returns the mark that offset is strictly inside, or -1.
func (r *RenderEditable) markInside(offset int) int {
	m := r.ruby.marks
	k := sort.Search(len(m), func(i int) bool { return m[i].End > offset })
	if k < len(m) && m[k].Start < offset {
		return k
	}
	return -1
}

// hiddenMark returns the mark whose notation is hidden and that starts at
// offset, or ends there if end is set.
func (r *RenderEditable) hiddenMark(offset int, end bool) (text.RubyMark, bool) {
	if r.field.Ruby == nil || r.Composition != nil {
		return text.RubyMark{}, false
	}
	m := r.ruby.marks
	k := sort.Search(len(m), func(i int) bool { return m[i].End >= offset })
	if k < len(m) && end && m[k].End == offset && k != r.ruby.active[0] && k != r.ruby.active[1] {
		return m[k], true
	}
	if k < len(m) && m[k].End == offset {
		k++ // the next mark may start where this one ends
	}
	if k < len(m) && !end && m[k].Start == offset && k != r.ruby.active[0] && k != r.ruby.active[1] {
		return m[k], true
	}
	return text.RubyMark{}, false
}

// findMarks brings the marks up to date with the text.
func (r *RenderEditable) findMarks() {
	v := &r.ruby
	if v.fresh {
		return
	}
	v.marks = r.field.Ruby(r.Content, v.marks[:0])
	checkMarks(v.marks, r.Content)
	v.fresh = true
}

// spliceMarks follows an edit that replaced removed bytes at a with inserted
// bytes. Only the lines the edit touched are searched again.
func (r *RenderEditable) spliceMarks(a, removed, inserted int) {
	v := &r.ruby
	if r.field.Ruby == nil || !v.fresh {
		v.fresh = false
		return
	}
	s := r.Content
	start := strings.LastIndexByte(s[:a], '\n') + 1
	end := len(s)
	if i := strings.IndexByte(s[a+inserted:], '\n'); i >= 0 {
		end = a + inserted + i
	}
	delta := inserted - removed
	m := v.marks
	// The marks of the old lines, from i to j, give way to those found now.
	i := sort.Search(len(m), func(k int) bool { return m[k].End > start })
	j := sort.Search(len(m), func(k int) bool { return m[k].Start >= end-delta })
	found := r.field.Ruby(s[start:end], v.found[:0])
	checkMarks(found, s[start:end])
	for k := range found {
		found[k] = shiftMark(found[k], start)
	}
	for k := j; k < len(m); k++ {
		m[k] = shiftMark(m[k], delta)
	}
	v.marks = slices.Replace(m, i, j, found...)
	v.found = found
}

func shiftMark(m text.RubyMark, by int) text.RubyMark {
	return text.RubyMark{
		Start: m.Start + by, End: m.End + by,
		BaseStart: m.BaseStart + by, BaseEnd: m.BaseEnd + by,
		RubyStart: m.RubyStart + by, RubyEnd: m.RubyEnd + by,
		Emphasis: m.Emphasis,
	}
}

// checkMarks panics if marks are not valid marks of s, in order.
func checkMarks(marks []text.RubyMark, s string) {
	prev := 0
	for _, m := range marks {
		if !validMark(m, prev, len(s)) {
			panic("gowaku: field.Input.Ruby must return valid marks in order, each with a base")
		}
		prev = m.End
	}
}

// validMark reports whether m lies in order between from and to.
func validMark(m text.RubyMark, from, to int) bool {
	return from <= m.Start && m.Start <= m.BaseStart && m.BaseStart < m.BaseEnd && m.BaseEnd <= m.RubyStart &&
		m.RubyStart <= m.RubyEnd && m.RubyEnd <= m.End && m.End <= to
}

// project returns the spans of the shown text: the display with the notation
// of the marks that are not active left out, and their ruby set on their base.
func (r *RenderEditable) project() []text.Span {
	r.findMarks()
	v := &r.ruby
	v.active = r.activeMarks()
	display := r.display()
	// The composition takes the place of the selection, and the marks it
	// reaches show as they are.
	a, b := r.Selection()
	shift := 0
	if r.Composition != nil {
		shift = len(r.Composition.Text) - (b - a)
	}
	v.hidden, v.spans = v.hidden[:0], v.spans[:0]
	at, removed := 0, 0
	for i, m := range v.marks {
		if i == v.active[0] || i == v.active[1] {
			continue
		}
		if r.Composition != nil {
			switch {
			case m.End <= a:
			case m.Start >= b:
				m.Start, m.BaseStart, m.BaseEnd = m.Start+shift, m.BaseStart+shift, m.BaseEnd+shift
				m.RubyStart, m.RubyEnd, m.End = m.RubyStart+shift, m.RubyEnd+shift, m.End+shift
			default:
				continue
			}
		}
		if m.Start > at {
			v.spans = append(v.spans, text.Span{Text: display[at:m.Start]})
		}
		if m.BaseStart > m.Start {
			v.hidden = append(v.hidden, hiddenRun{start: m.Start, end: m.BaseStart, at: m.Start - removed, removed: removed, lead: true})
			removed += m.BaseStart - m.Start
		}
		v.spans = append(v.spans, text.Span{Text: display[m.BaseStart:m.BaseEnd], Ruby: display[m.RubyStart:m.RubyEnd], Emphasis: m.Emphasis})
		if m.End > m.BaseEnd {
			v.hidden = append(v.hidden, hiddenRun{start: m.BaseEnd, end: m.End, at: m.BaseEnd - removed, removed: removed})
			removed += m.End - m.BaseEnd
		}
		at = m.End
	}
	if at < len(display) || len(v.spans) == 0 {
		v.spans = append(v.spans, text.Span{Text: display[at:]})
	}
	return v.spans
}

// moveRuby follows a move of the selection: a mark that an end entered or
// left changes the shown text.
func (r *RenderEditable) moveRuby() {
	if r.field.Ruby != nil && !r.stale && r.ruby.active != r.activeMarks() {
		r.invalidate()
	}
}

// enterMark turns a step of the caret from offset to target that would jump
// over the base of a hidden mark into a step into its notation, which shows it.
func (r *RenderEditable) enterMark(offset, target int) int {
	if target > offset {
		if m, ok := r.hiddenMark(offset, false); ok && target >= m.End {
			return textStep(r.Content, offset, 1)
		}
	} else if target < offset {
		if m, ok := r.hiddenMark(offset, true); ok && target <= m.Start {
			return textStep(r.Content, offset, -1)
		}
	}
	return target
}

// textStep returns the offset one character from i in s, forward if dir is
// positive, with the marks that combine with the character.
func textStep(s string, i, dir int) int {
	if dir > 0 {
		if i >= len(s) {
			return len(s)
		}
		_, n := utf8.DecodeRuneInString(s[i:])
		i += n
		for i < len(s) {
			c, n := utf8.DecodeRuneInString(s[i:])
			if !combines(c) {
				break
			}
			i += n
		}
		return i
	}
	for i > 0 {
		c, n := utf8.DecodeLastRuneInString(s[:i])
		i -= n
		if !combines(c) {
			break
		}
	}
	return i
}

// combines reports whether c belongs to the character before it.
func combines(c rune) bool {
	return unicode.Is(unicode.Mn, c) || unicode.Is(unicode.Me, c) ||
		(c >= 0xFE00 && c <= 0xFE0F) || (c >= 0xE0100 && c <= 0xE01EF) || c == 0x200D
}

// shownLayout is the layout of a field in offsets of the display. Without
// hidden notation, the offsets are those of the layout.
type shownLayout struct {
	l *text.Layout
	v *rubyView
	// o, if set, maps the display to the bullets of an obscured field.
	o *obscureView
}

// shown returns the layout of the field in offsets of the display.
func (r *RenderEditable) shown() shownLayout {
	if r.field.Obscure {
		return shownLayout{l: r.laidOut, o: &r.obscure}
	}
	if r.field.Ruby == nil || len(r.ruby.hidden) == 0 {
		return shownLayout{l: r.laidOut}
	}
	return shownLayout{l: r.laidOut, v: &r.ruby}
}

func (s shownLayout) in(d int) int {
	if s.o != nil {
		return s.o.toShown(d)
	}
	if s.v == nil {
		return d
	}
	return s.v.toShown(d)
}

func (s shownLayout) out(o int) int {
	if s.o != nil {
		return s.o.fromShown(o)
	}
	if s.v == nil {
		return o
	}
	return s.v.fromShown(o)
}

// CaretRect is text.Layout.CaretRect.
func (s shownLayout) CaretRect(d int) geom.Rect { return s.l.CaretRect(s.in(d)) }

// CaretPosition is text.Layout.CaretPosition.
func (s shownLayout) CaretPosition(d int) float32 { return s.l.CaretPosition(s.in(d)) }

// HitTest is text.Layout.HitTest.
func (s shownLayout) HitTest(pos geom.Offset) int { return s.out(s.l.HitTest(pos)) }

// PrevCaret is text.Layout.PrevCaret.
func (s shownLayout) PrevCaret(d int) int { return s.out(s.l.PrevCaret(s.in(d))) }

// NextCaret is text.Layout.NextCaret.
func (s shownLayout) NextCaret(d int) int { return s.out(s.l.NextCaret(s.in(d))) }

// PrevWord is text.Layout.PrevWord. An obscured field has one word.
func (s shownLayout) PrevWord(d int) int {
	if s.o != nil {
		return 0
	}
	return s.out(s.l.PrevWord(s.in(d)))
}

// NextWord is text.Layout.NextWord.
func (s shownLayout) NextWord(d int) int {
	if s.o != nil {
		return s.o.end()
	}
	return s.out(s.l.NextWord(s.in(d)))
}

// LineStart is text.Layout.LineStart.
func (s shownLayout) LineStart(d int) int { return s.out(s.l.LineStart(s.in(d))) }

// LineEnd is text.Layout.LineEnd.
func (s shownLayout) LineEnd(d int) int { return s.out(s.l.LineEnd(s.in(d))) }

// VisualMove is text.Layout.VisualMove.
func (s shownLayout) VisualMove(d, dir int) int { return s.out(s.l.VisualMove(s.in(d), dir)) }

// LineCount is text.Layout.LineCount.
func (s shownLayout) LineCount() int { return s.l.LineCount() }

// VerticalMove is text.Layout.VerticalMove.
func (s shownLayout) VerticalMove(d, rows int, x float32) int {
	return s.out(s.l.VerticalMove(s.in(d), rows, x))
}

// SelectionRects is text.Layout.SelectionRects.
func (s shownLayout) SelectionRects(a, b int, dst []geom.Rect) []geom.Rect {
	return s.l.SelectionRects(s.in(a), s.in(b), dst)
}

// WordRange is text.Layout.WordRange.
func (s shownLayout) WordRange(d int) (start, end int) {
	if s.o != nil {
		return 0, s.o.end()
	}
	a, b := s.l.WordRange(s.in(d))
	return s.out(a), s.out(b)
}

// LineRange is text.Layout.LineRange.
func (s shownLayout) LineRange(i int) (start, end int) {
	a, b := s.l.LineRange(i)
	return s.out(a), s.out(b)
}
