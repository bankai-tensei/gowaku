package text

import "github.com/bankai-tensei/gowaku/geom"

// Background is a box that Span.Background fills behind the glyphs of a
// Layout, on one line. Rect is relative to the top left of the layout, as
// the runs are.
type Background struct {
	Rect  geom.Rect
	Color geom.Color
}

// Backgrounds returns the boxes of the spans with a Background, in the order
// of the lines, to be filled before the runs are drawn. Characters side by
// side on a line in the same color and size make one box. The slice belongs to
// the layout.
func (l *Layout) Backgrounds() []Background { return l.backgrounds }

// addBackgrounds finds the boxes of the spans with a Background. It looks at
// every cluster of the lines when a span has one, and at nothing else
// otherwise.
func (l *Layout) addBackgrounds() {
	l.backgrounds = l.backgrounds[:0]
	p := l.paragraph
	if !p.hasBackground() {
		return
	}
	for i := range l.lines {
		ln := &l.lines[i]
		origin := l.lineOrigin(ln)
		first := len(l.backgrounds)
		for c := ln.clusterStart; c < ln.clusterEnd; c++ {
			sec := &p.sections[p.clusters[c].section]
			if sec.background.A == 0 {
				continue
			}
			lo := ln.x + l.xs[c]
			r := l.backgroundRect(origin, lo, lo+l.adv[c], sec.style.Size)
			if n := len(l.backgrounds); n > first && l.extend(&l.backgrounds[n-1], r, sec.background) {
				continue
			}
			l.backgrounds = append(l.backgrounds, Background{Rect: r, Color: sec.background})
		}
	}
}

// backgroundRect returns the box of the characters from inline lo to hi on
// the line at origin, in text of size: across the line, the em box, down to
// the descent of the font in horizontal text.
func (l *Layout) backgroundRect(origin geom.Offset, lo, hi, size float32) geom.Rect {
	p := l.paragraph
	if p.vertical {
		return geom.Rect{X: origin.X - size/2, Y: lo, W: size, H: hi - lo}
	}
	top := origin.Y - (p.emCenter+0.5)*size
	bottom := origin.Y + (0.5-p.emCenter)*size
	if p.style.Size > 0 {
		bottom = max(bottom, origin.Y+p.descent*size/p.style.Size)
	}
	return geom.Rect{X: lo, Y: top, W: hi - lo, H: bottom - top}
}

// extend grows b to take in r, and reports whether it did: r must be of the
// same color and line, and touch b along the line on either side, since right
// to left clusters come in from the right.
func (l *Layout) extend(b *Background, r geom.Rect, color geom.Color) bool {
	if b.Color != color {
		return false
	}
	const near = 0.01
	if l.paragraph.vertical {
		if b.Rect.X != r.X || b.Rect.W != r.W || geom.Abs32(b.Rect.Y+b.Rect.H-r.Y) > near {
			return false
		}
		b.Rect.H = r.Y + r.H - b.Rect.Y
		return true
	}
	if b.Rect.Y != r.Y || b.Rect.H != r.H {
		return false
	}
	switch {
	case geom.Abs32(b.Rect.X+b.Rect.W-r.X) <= near:
		b.Rect.W = r.X + r.W - b.Rect.X
	case geom.Abs32(r.X+r.W-b.Rect.X) <= near:
		b.Rect.W += b.Rect.X - r.X
		b.Rect.X = r.X
	default:
		return false
	}
	return true
}

// hasBackground reports whether a span of p has a Background.
func (p *Paragraph) hasBackground() bool {
	for i := range p.sections {
		if p.sections[i].background.A != 0 {
			return true
		}
	}
	return false
}
