package editor

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

// Paint implements core.RenderObject. It draws the lines in view from the
// cache, and allocates nothing.
func (r *renderEditor) Paint(cv *paint.Canvas, off geom.Offset) {
	r.checkView()
	sz := r.size
	cv.Save()
	cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H})
	cv.Save()
	cv.ClipRect(geom.Rect{X: off.X + r.gutter, Y: off.Y, W: max(sz.W-r.gutter, 0), H: sz.H})
	x := off.X + r.textLeft - r.hscroll()
	y := off.Y + r.viewTop(r.first)
	for i := r.first; i <= r.last; i++ {
		e := r.cache.get(i)
		if e == nil || !e.valid {
			continue
		}
		r.paintLine(cv, i, e, geom.Offset{X: x, Y: y})
		y += float32(r.rows.at(i)) * r.lineHeight
	}
	cv.Restore()
	if r.gutter > 0 {
		r.paintGutter(cv, off)
	}
	cv.Restore()
	r.tellIME()
}

// paintLine draws line i at origin: its grounds, the text, the marks under it
// and the caret.
func (r *renderEditor) paintLine(cv *paint.Canvas, i int, e *lineEntry, origin geom.Offset) {
	l := e.layout
	ctl := r.ctl
	composing := r.composition != nil && i == r.compLine
	// whole is the line across the text area, all its rows, wherever the
	// text has scrolled sideways.
	whole := geom.Rect{
		X: origin.X - r.textLeft + r.hscroll() + r.gutter, Y: origin.Y,
		W: r.size.W - r.gutter, H: float32(l.LineCount()) * r.lineHeight,
	}
	if c := e.line.Background; c != (geom.Color{}) {
		r.fill(cv, whole, c)
	}
	if r.editor.HighlightCurrentLine && i == r.sel().caret.Line {
		r.fill(cv, whole, r.look.currentLine)
	}
	if !composing {
		current := ctl.currentMatchOf(r.sel())
		base := ctl.find.firstInLine(i)
		for k, m := range ctl.find.inLines(i, i) {
			r.fillRange(cv, l, origin, m.Start.Column, m.End.Column, r.look.match)
			if base+k == current {
				r.outlineRange(cv, l, origin, m.Start.Column, m.End.Column, r.look.currentMatch)
			}
		}
	}
	for k := range e.decos {
		if d := &e.decos[k]; d.Background != (geom.Color{}) {
			r.fillRange(cv, l, origin, clampCol(d.Start, e.src), clampCol(d.End, e.src), d.Background)
		}
	}
	if composing {
		r.paintComposition(cv, l, origin)
	} else {
		r.paintSelection(cv, i, e, origin)
	}
	color := r.editor.Style.PaintColor()
	runs := l.Runs()
	for k := range runs {
		cv.DrawGlyphs(&runs[k], origin, color)
	}
	for k := range e.decos {
		if d := &e.decos[k]; d.Underline == UnderlineSolid {
			r.rects = l.SelectionRects(clampCol(d.Start, e.src), clampCol(d.End, e.src), r.rects[:0])
			for _, rc := range r.rects {
				line := geom.Rect{X: rc.X, Y: rc.Y + r.baseline + r.style.Size*0.12, W: rc.W, H: 1}
				r.fill(cv, line.Translate(origin), r.underlineColor(d))
			}
		}
	}
	if len(e.squiggles) > 0 {
		cv.Save()
		cv.Translate(origin.X, origin.Y)
		for k := range e.squiggles {
			cv.DrawPath(e.squiggles[k].path, paint.Paint{Color: e.squiggles[k].color})
		}
		cv.Restore()
	}
	if r.focused && !r.caretHidden && i == r.sel().caret.Line {
		c := l.CaretRect(r.displayColumn(r.sel().caret))
		c.W = kit.CaretWidth
		r.fill(cv, c.Translate(origin), r.look.caret)
	}
	if c := e.line.Bar; c != (geom.Color{}) {
		// Over the text, which scrolls sideways under it.
		r.fill(cv, geom.Rect{X: whole.X, Y: whole.Y, W: LineBarWidth, H: whole.H}, c)
	}
}

// paintSelection fills the part of the selection on line i, with a narrow mark
// for its line break when the selection goes on past it.
func (r *renderEditor) paintSelection(cv *paint.Canvas, i int, e *lineEntry, origin geom.Offset) {
	sel := r.sel().textRange().Ordered()
	if sel.Empty() || i < sel.Start.Line || i > sel.End.Line {
		return
	}
	a, z := 0, len(e.src)
	if i == sel.Start.Line {
		a = sel.Start.Column
	}
	if i == sel.End.Line {
		z = sel.End.Column
	}
	r.fillRange(cv, e.layout, origin, a, z, r.look.selection)
	if i < sel.End.Line {
		c := e.layout.CaretRect(len(e.src))
		c.W = r.style.Size * editorNewline
		r.fill(cv, c.Translate(origin), r.look.selection)
	}
}

// paintComposition marks the clauses of the composition as Input does.
func (r *renderEditor) paintComposition(cv *paint.Canvas, l *text.Layout, origin geom.Offset) {
	base := r.compStart
	color := r.editor.Style.PaintColor()
	for _, cl := range r.composition.Clauses {
		a, b := base+cl.Start, base+cl.End
		thickness := float32(1)
		if cl.Kind == input.ClauseTarget {
			thickness = 2
			r.fillRange(cv, l, origin, a, b, r.look.target)
		}
		r.rects = l.SelectionRects(a, b, r.rects[:0])
		for _, rc := range r.rects {
			line := geom.Rect{X: rc.X + 1, Y: rc.Y + rc.H - thickness - 1, W: max(rc.W-2, 1), H: thickness}
			r.fill(cv, line.Translate(origin), color)
		}
	}
}

func (r *renderEditor) fill(cv *paint.Canvas, rc geom.Rect, color geom.Color) {
	cv.DrawRRect(geom.RRectAll(rc, 0), paint.Paint{Color: color})
}

// fillRange fills the columns a to z of a line laid out as l.
func (r *renderEditor) fillRange(cv *paint.Canvas, l *text.Layout, origin geom.Offset, a, z int, color geom.Color) {
	r.rects = l.SelectionRects(a, z, r.rects[:0])
	for _, rc := range r.rects {
		r.fill(cv, rc.Translate(origin), color)
	}
}

// outlineRange draws a hairline around the columns a to z.
func (r *renderEditor) outlineRange(cv *paint.Canvas, l *text.Layout, origin geom.Offset, a, z int, color geom.Color) {
	r.rects = l.SelectionRects(a, z, r.rects[:0])
	for _, rc := range r.rects {
		cv.DrawRRect(geom.RRectAll(rc.Translate(origin), 2), paint.Paint{Stroke: &paint.Stroke{Width: 1, Color: color}})
	}
}

// paintGutter draws the line numbers and calls Gutter for every line in view.
func (r *renderEditor) paintGutter(cv *paint.Canvas, off geom.Offset) {
	cv.Save()
	cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: r.gutter, H: r.size.H})
	y := off.Y + r.viewTop(r.first)
	caretLine := r.sel().caret.Line
	for i := r.first; i <= r.last; i++ {
		e := r.cache.get(i)
		if e == nil || !e.valid {
			continue
		}
		h := float32(r.rows.at(i)) * r.lineHeight
		if c := e.line.Gutter; c != (geom.Color{}) {
			r.fill(cv, geom.Rect{X: off.X, Y: y, W: r.gutter, H: h}, c)
		}
		if r.numbers > 0 {
			num := r.lineNumber(e, i)
			color := r.look.number
			if i == caretLine {
				color = r.look.current
			}
			at := geom.Offset{X: off.X + r.numbers - editorGutterPad - num.Size().W, Y: y}
			runs := num.Runs()
			for k := range runs {
				cv.DrawGlyphs(&runs[k], at, color)
			}
		}
		if f := r.editor.Gutter; f != nil {
			f(cv, GutterLine{
				Line:     i,
				Rect:     geom.Rect{X: off.X + r.numbers, Y: y, W: r.editor.GutterWidth, H: h},
				Baseline: y + r.baseline,
				Current:  i == caretLine,
			})
		}
		y += h
	}
	cv.Restore()
}

// lineNumber returns the number of line i laid out. It shapes only when the
// line shows another number than it did, as after lines above it went.
func (r *renderEditor) lineNumber(e *lineEntry, i int) *text.Layout {
	if e.numbered == nil || e.number != i+1 || e.numberGen != r.styleGen {
		e.number, e.numberGen = i+1, r.styleGen
		e.numbered = r.shaper.Shape(kit.Itoa(i+1), r.style).Layout(geom.Inf)
	}
	return e.numbered
}

// tellIME moves the candidate list to the clause being converted, or else the
// caret, as Input does.
func (r *renderEditor) tellIME() {
	if !r.focused || r.editor.ReadOnly {
		return
	}
	caret := r.sel().caret
	e := r.cache.get(caret.Line)
	if e == nil || !e.valid {
		return
	}
	at := r.displayColumn(caret)
	if r.composition != nil {
		for _, cl := range r.composition.Clauses {
			if cl.Kind == input.ClauseTarget {
				at = r.compStart + cl.Start
				break
			}
		}
	}
	origin := geom.Offset{X: r.textLeft - r.hscroll(), Y: r.viewTop(caret.Line)}
	rect := core.LocalRectToGlobal(r, e.layout.CaretRect(at).Translate(origin))
	if rect != r.imeRect {
		r.imeRect = rect
		r.Host().SetIMECursorRect(rect)
	}
}
