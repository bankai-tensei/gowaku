package editor

import (
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
)

// shapeLine shapes src with the colors, weights and slants of the decorations
// of e.
func (r *renderEditor) shapeLine(e *lineEntry, src string) *text.Paragraph {
	styled := false
	for k := range e.decos {
		if d := &e.decos[k]; d.Color != (geom.Color{}) || d.Bold || d.Italic {
			styled = true
		}
	}
	if !styled {
		return r.shaper.Shape(src, r.style)
	}
	e.spans = e.spans[:0]
	for at := 0; at < len(src); {
		next := len(src)
		var st text.Style
		for k := range e.decos {
			d := &e.decos[k]
			a, z := max(d.Start, 0), min(d.End, len(src))
			if a >= z {
				continue
			}
			if a > at {
				next = min(next, a)
				continue
			}
			if z <= at {
				continue
			}
			next = min(next, z)
			if d.Color != (geom.Color{}) {
				st.Color = d.Color
			}
			if d.Bold {
				st.Weight = 700
			}
			if d.Italic {
				st.Italic = true
			}
		}
		// A boundary inside a character would split it.
		for next < len(src) && !startsRune(src[next]) {
			next++
		}
		e.spans = append(e.spans, text.Span{Text: src[at:next], Style: st})
		at = next
	}
	return r.shaper.ShapeSpans(e.spans, r.style)
}

// makeSquiggles builds the wavy lines of e, which stay until e changes.
func (r *renderEditor) makeSquiggles(e *lineEntry) {
	e.squiggles = e.squiggles[:0]
	for k := range e.decos {
		d := &e.decos[k]
		if d.Underline != UnderlineSquiggle {
			continue
		}
		r.rects = e.layout.SelectionRects(clampCol(d.Start, e.src), clampCol(d.End, e.src), r.rects[:0])
		for _, rc := range r.rects {
			n := len(e.squiggles)
			if n == len(e.paths) {
				e.paths = append(e.paths, &paint.Path{})
			}
			p := e.paths[n]
			p.Reset()
			wave(p, rc.X, rc.X+rc.W, rc.Y+r.baseline+r.style.Size*0.12)
			e.squiggles = append(e.squiggles, squiggle{path: p, color: r.underlineColor(d)})
		}
	}
}

func clampCol(c int, s string) int { return max(min(c, len(s)), 0) }

// wave adds a wave from x0 to x1 below y to p, as a filled band, so that it
// needs no stroking.
func wave(p *paint.Path, x0, x1, y float32) {
	const half, amp, thick = 2, 1.1, 1.1
	n := max(int((x1-x0)/half), 1)
	step := (x1 - x0) / float32(n)
	for k := 0; k <= n; k++ {
		yy := y + amp
		if k%2 == 1 {
			yy = y - amp
		}
		if k == 0 {
			p.MoveTo(x0, yy)
		} else {
			p.LineTo(x0+float32(k)*step, yy)
		}
	}
	for k := n; k >= 0; k-- {
		yy := y + amp + thick
		if k%2 == 1 {
			yy = y - amp + thick
		}
		p.LineTo(x0+float32(k)*step, yy)
	}
	p.Close()
}

func (r *renderEditor) underlineColor(d *Decoration) geom.Color {
	switch {
	case d.UnderlineColor != (geom.Color{}):
		return d.UnderlineColor
	case d.Color != (geom.Color{}):
		return d.Color
	}
	return r.editor.Style.PaintColor()
}
