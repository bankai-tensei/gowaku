package field

import (
	"strings"
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// selPoint is an end of a selection: a byte offset into the text of a
// paragraph, any basic.SelectableText.
type selPoint struct {
	p   basic.SelectableText
	off int
}

// selection is what a SelectionArea has selected. It lives in the state, so
// that a new render object keeps it, and does its work at event time: nothing
// here runs during a frame.
type selection struct {
	// region is the render object of the area, or nil.
	region *renderSelectionRegion
	color  geom.Color
	// anchor is where the selection started and focus the end that moves.
	// anchor.p is nil without a selection.
	anchor, focus selPoint
	// marked are the paragraphs that show a part of the selection.
	marked []basic.SelectableText
	// paras is scratch for the paragraphs of the area in document order.
	paras []basic.SelectableText
	// dragging is set between a press that put the anchor down and its
	// release.
	dragging bool
	// menuOpen keeps the selection while the menu of the area has the focus.
	menuOpen bool
	// goal is where Up and Down try to stay along the lines, relative to the
	// area: x in horizontal text, y in vertical text.
	goal   float32
	goalOK bool
}

// collect lists the paragraphs of the area in the order they are painted. It
// leaves out hidden subtrees, the text of controls and that of areas inside.
func (s *selection) collect() []basic.SelectableText {
	s.paras = s.paras[:0]
	if r := s.region; r != nil && r.Attached() {
		s.paras = appendParagraphs(s.paras, r, r)
	}
	return s.paras
}

func appendParagraphs(dst []basic.SelectableText, ro, root core.RenderObject) []basic.SelectableText {
	if h, ok := ro.(core.Hider); ok && h.Hidden() {
		return dst
	}
	if ro != root {
		if p, ok := ro.(basic.SelectableText); ok {
			// A nil layout is a text not laid out yet, or one that is not
			// to be selected.
			if p.TextLayout() != nil {
				dst = append(dst, p)
			}
			return dst
		}
		// A control, or another area, owns the text below it.
		if _, ok := ro.(core.FocusTarget); ok {
			return dst
		}
	}
	for i := range ro.ChildCount() {
		dst = appendParagraphs(dst, ro.ChildAt(i), root)
	}
	return dst
}

// indexOf returns the place of p in paras, or -1.
func indexOf(paras []basic.SelectableText, p basic.SelectableText) int {
	for i, q := range paras {
		if q == p {
			return i
		}
	}
	return -1
}

// clampOffset keeps off inside the text of p, on the start of a character:
// the text may have changed since the offset was taken.
func clampOffset(p basic.SelectableText, off int) int {
	t := p.Text()
	off = min(max(off, 0), len(t))
	for off > 0 && off < len(t) && !utf8.RuneStart(t[off]) {
		off--
	}
	return off
}

// span returns the ends of the selection in document order, as indices into
// the paragraphs collect last found and offsets. ok is false without a
// selection, and when an end has left the area, which drops the selection.
func (s *selection) span() (i0, o0, i1, o1 int, ok bool) {
	if s.anchor.p == nil {
		return 0, 0, 0, 0, false
	}
	paras := s.collect()
	ia, ifo := indexOf(paras, s.anchor.p), indexOf(paras, s.focus.p)
	if ia < 0 || ifo < 0 {
		s.clear()
		return 0, 0, 0, 0, false
	}
	s.anchor.off = clampOffset(s.anchor.p, s.anchor.off)
	s.focus.off = clampOffset(s.focus.p, s.focus.off)
	i0, o0, i1, o1 = ia, s.anchor.off, ifo, s.focus.off
	if i1 < i0 || i1 == i0 && o1 < o0 {
		i0, o0, i1, o1 = i1, o1, i0, o0
	}
	return i0, o0, i1, o1, true
}

// hasRange reports whether anything is selected.
func (s *selection) hasRange() bool {
	i0, o0, i1, o1, ok := s.span()
	return ok && (i0 != i1 || o0 != o1)
}

// apply shows the selection on the paragraphs.
func (s *selection) apply() {
	for _, p := range s.marked {
		p.ClearSelection()
	}
	clear(s.marked)
	s.marked = s.marked[:0]
	i0, o0, i1, o1, ok := s.span()
	if !ok {
		return
	}
	for k := i0; k <= i1; k++ {
		p := s.paras[k]
		start, end := 0, len(p.Text())
		if k == i0 {
			start = o0
		}
		if k == i1 {
			end = o1
		}
		p.SetSelection(start, end, s.color)
		s.marked = append(s.marked, p)
	}
}

// setColor changes the color of the highlight, as a new theme does.
func (s *selection) setColor(c geom.Color) {
	if s.color == c {
		return
	}
	s.color = c
	for _, p := range s.marked {
		start, end := p.Selection()
		p.SetSelection(start, end, c)
	}
}

// clear drops the selection.
func (s *selection) clear() {
	s.anchor, s.focus = selPoint{}, selPoint{}
	s.goalOK = false
	for _, p := range s.marked {
		p.ClearSelection()
	}
	clear(s.marked)
	s.marked = s.marked[:0]
}

// text returns the selected text: the parts of the paragraphs, each after the
// SelectionBreak of its paragraph ("\n" for Text and RichText). Ruby is not
// part of the text of a paragraph, so only the base is copied.
func (s *selection) text() string {
	i0, o0, i1, o1, ok := s.span()
	if !ok {
		return ""
	}
	var b strings.Builder
	for k := i0; k <= i1; k++ {
		t := s.paras[k].Text()
		start, end := 0, len(t)
		if k == i0 {
			start = o0
		}
		if k == i1 {
			end = o1
		}
		if k > i0 {
			b.WriteString(s.paras[k].SelectionBreak())
		}
		b.WriteString(t[start:end])
	}
	return b.String()
}

// copy puts the selected text on the clipboard.
func (s *selection) copy() {
	if s.region == nil {
		return
	}
	if t := s.text(); t != "" {
		s.region.Host().SetClipboardText(t)
	}
}

// selectAll selects from the start of the first paragraph to the end of the
// last.
func (s *selection) selectAll() {
	paras := s.collect()
	if len(paras) == 0 {
		return
	}
	last := paras[len(paras)-1]
	s.anchor = selPoint{paras[0], 0}
	s.focus = selPoint{last, len(last.Text())}
	s.goalOK = false
	s.apply()
}

// pointAt returns the place in the text nearest to pos, relative to the area:
// in the paragraph under pos, or else in the closest one. Past the lines of
// that paragraph, it is its start or its end.
func (s *selection) pointAt(pos geom.Offset) (selPoint, bool) {
	paras := s.collect()
	global := core.LocalToGlobal(s.region, pos)
	best, bestDist := -1, float32(0)
	var bestLocal geom.Offset
	for i, p := range paras {
		local, ok := core.GlobalToLocal(p, global)
		if !ok {
			continue
		}
		d := distanceTo(local, p.Size(), p.TextLayout().Orientation() == text.Vertical)
		if best < 0 || d < bestDist {
			best, bestDist, bestLocal = i, d, local
			if d == 0 {
				break
			}
		}
	}
	if best < 0 {
		return selPoint{}, false
	}
	return hitParagraph(paras[best], bestLocal.Sub(paras[best].TextOrigin())), true
}

// crossWeight makes a distance across the lines count more than one along
// them, so that a pointer beside the texts picks the one at its height (its
// column in vertical text) rather than a shorter one further down.
const crossWeight = 4

// distanceTo returns how far pos is from a box of size sz at the origin, zero
// inside: the square of the distance, with the part across the lines
// weighted.
func distanceTo(pos geom.Offset, sz geom.Size, vertical bool) float32 {
	dx := max(-pos.X, 0, pos.X-sz.W)
	dy := max(-pos.Y, 0, pos.Y-sz.H)
	if vertical {
		dx *= crossWeight
	} else {
		dy *= crossWeight
	}
	return dx*dx + dy*dy
}

// hitParagraph returns the place in p nearest to pos, relative to the origin
// of its layout. Before
// the first line it is the start, past the last line the end: below the text,
// or left of its last column.
func hitParagraph(p basic.SelectableText, pos geom.Offset) selPoint {
	l := p.TextLayout()
	sz := l.Size()
	if l.Orientation() == text.Vertical {
		switch {
		case pos.X >= sz.W:
			return selPoint{p, 0}
		case pos.X < 0:
			return selPoint{p, len(p.Text())}
		}
	} else {
		switch {
		case pos.Y < 0:
			return selPoint{p, 0}
		case pos.Y >= sz.H:
			return selPoint{p, len(p.Text())}
		}
	}
	return selPoint{p, l.HitTest(pos)}
}

// press starts a selection at pt: clicks 2 takes the word, 3 the line between
// hard breaks, and extend keeps the anchor, as Shift+click does.
func (s *selection) press(pt selPoint, clicks int, extend bool) {
	s.goalOK = false
	s.dragging = false
	switch {
	case clicks == 2:
		a, b := pt.p.TextLayout().WordRange(pt.off)
		s.anchor, s.focus = selPoint{pt.p, a}, selPoint{pt.p, b}
	case clicks >= 3:
		a, b := blockRange(pt.p.Text(), pt.off)
		s.anchor, s.focus = selPoint{pt.p, a}, selPoint{pt.p, b}
	default:
		if !extend || !s.valid() {
			s.anchor = pt
		}
		s.focus = pt
		s.dragging = true
	}
	s.apply()
}

// valid reports whether both ends are still in the area.
func (s *selection) valid() bool {
	_, _, _, _, ok := s.span()
	return ok
}

// dragTo moves the free end to pos.
func (s *selection) dragTo(pos geom.Offset) {
	if !s.dragging {
		return
	}
	if !s.valid() {
		s.dragging = false
		return
	}
	if pt, ok := s.pointAt(pos); ok && pt != s.focus {
		s.focus = pt
		s.apply()
	}
}

// blockRange returns the stretch of t around off between hard line breaks.
func blockRange(t string, off int) (start, end int) {
	off = min(max(off, 0), len(t))
	start = strings.LastIndexByte(t[:off], '\n') + 1
	end = strings.IndexByte(t[off:], '\n')
	if end < 0 {
		return start, len(t)
	}
	end += off
	if end > start && t[end-1] == '\r' {
		end--
	}
	return start, end
}
