package editor

import (
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

const (
	// editorPadX is the room between the gutter and the text and right of
	// the text, and editorPadY the room above the first line and below the
	// last.
	editorPadX = theme.Space8
	editorPadY = theme.Space4
	// editorGutterPad is the room on each side of the line numbers.
	editorGutterPad = theme.Space8
	// editorBarRoom keeps wrapped text clear of the scroll bar.
	editorBarRoom = 12
	// editorDefaultWidth and editorDefaultLines size an editor whose room is
	// unbounded.
	editorDefaultWidth = 480
	editorDefaultLines = 12
	// editorTabSpaces is how many spaces a tab stop is apart, unless the style
	// sets TabStop.
	editorTabSpaces = 4
	// editorNewline is the width of the mark of a selected line break, in ems.
	editorNewline = 0.35
)

// editorLook is what an editor takes from the theme.
type editorLook struct {
	caret, selection, match, currentMatch, currentLine geom.Color
	// number is the color of the line numbers, and current that of the
	// number of the line of the caret.
	number, current geom.Color
	// target marks the clause the IME is converting.
	target geom.Color
}

// renderEditor shows the lines of a Editor that are in view, and takes the
// keyboard and the pointer. Only the lines near the view are laid out, into a
// cache that painting reads without allocating; rowTable places the rest by
// their row counts, estimated until a line is laid out.
type renderEditor struct {
	core.RenderBase
	state  *textEditorState
	shaper *text.Shaper
	editor Editor
	ctl    *Controller
	// view holds the selection and the scroll position that this editor
	// shows.
	view *View
	look editorLook

	// style is the style of the editor with its tab stops, and styleGen counts
	// its changes. lineHeight is the height of a row, baseline the position of
	// its baseline, digit the advance of a digit and byteWidth the average
	// advance of a byte of text, for estimates.
	style                 text.Style
	styleOK               bool
	styleGen              uint64
	lineHeight, baseline  float32
	digit                 float32
	byteWidth             float32
	sampledBytes, sampled float32

	size geom.Size
	// numbers is the width of the line numbers, gutter that of the whole
	// gutter, and textLeft the start of the text before hscroll.
	numbers, gutter, textLeft float32
	// wrap is the length of a row: the width of the text, or Inf.
	wrap      float32
	rowsValid bool
	rows      rowTable
	cache     lineCache
	// temp lays out a line outside the cache, for the keyboard.
	temp lineEntry
	// widest is the widest line laid out, which bounds the sideways scroll.
	widest float32
	// anchorPos is where the view goes on the next layout after an edit
	// above it, so that it stays on the same text, if reanchor is set.
	anchorPos ScrollPosition
	reanchor  bool
	// notify tells OnScroll of a change that a layout made, on the next frame.
	notify *core.Ticker
	// first and last are the lines in view.
	first, last int

	composition *input.Composition
	// compLine is the line of the composition, compStart and compEnd the
	// part of the line it replaces, and compText the line with it.
	compLine, compStart, compEnd int
	compText                     string

	focused, dragging bool
	// goalX is the position along a row that Up and Down try to keep.
	goalX  float32
	goalOK bool
	// reveal asks the next layout to scroll the caret into view.
	reveal bool
	// findRoom is the room the find bar takes over the top of the view, kept
	// free above the first line while the bar is open.
	findRoom float32
	imeRect  geom.Rect
	rects    []geom.Rect

	blink                   *core.Ticker
	caretHidden, blinkReset bool

	// a11y holds what the semantics last built, for reuse.
	a11y editorSemantics
}

// textReset follows SetText, a new controller or a new view.
func (r *renderEditor) textReset() {
	r.composition = nil
	r.a11y.valid = false
	r.cache.reset()
	r.rowsValid = false
	r.widest = 0
	r.reanchor = false
	r.goalOK = false
	if r.state != nil && r.state.Ticker != nil {
		// Not ScrollState.Jump, whose OnMove would tell OnScroll in the
		// middle of a build. The layout tells it, on the next frame.
		r.state.Scroller.Jump(0)
		r.state.Ticker.Stop()
		r.state.horz.Scroller.Jump(0)
		r.state.horz.Ticker.Stop()
		r.state.ShowBar()
	}
	r.MarkNeedsLayout()
}

// detach lets go of the view, which keeps its scroll position for the next
// editor that shows it.
func (r *renderEditor) detach() {
	v := r.view
	if v == nil || v.r != r {
		return
	}
	r.keepPlace()
	v.r = nil
	if v.ctl != nil {
		v.ctl.detach(v)
	}
}

// keepPlace saves the scroll position into the view, which goes back to it
// in the next editor that shows it.
func (r *renderEditor) keepPlace() {
	if v := r.view; r.rowsValid && !v.hasPending {
		v.pending, v.hasPending, v.kept = r.scrollPosition(), true, true
	}
}

// checkView panics if another editor took the view of this one while this
// one stays in the tree.
func (r *renderEditor) checkView() {
	if r.view != nil && r.view.r != r {
		panic("gowaku: an editor.View shows in two Editors at once; give each Editor its own View")
	}
}

// activate makes the view of this editor the primary view of the controller,
// as the user works in it.
func (r *renderEditor) activate() { r.ctl.activate(r.view) }

// sel returns the selection of the view of this editor.
func (r *renderEditor) sel() *selection { return &r.view.sel }

// styleChanged follows a change of the style, the wrapping or the gutter.
func (r *renderEditor) styleChanged() {
	r.styleOK = false
	r.cache.reset()
	r.rowsValid = false
	r.MarkNeedsLayout()
}

// linesChanged follows an edit of the lines. An edit above the view keeps the
// view on the same text.
func (r *renderEditor) linesChanged(ch lineChange) {
	if r.rowsValid {
		if !r.reanchor {
			// The rows still count the lines before the edit.
			r.anchorPos = r.scrollPosition()
		}
		if p := &r.anchorPos; ch.first < p.Line {
			r.reanchor = true
			if ch.first+ch.removed <= p.Line {
				p.Line += ch.added - ch.removed
			} else {
				p.Line, p.Offset = ch.first, 0
			}
		}
	}
	r.cache.splice(ch)
	if r.rowsValid {
		r.rows.splice(ch.first, ch.removed, ch.added, r.estimate)
	}
	r.MarkNeedsLayout()
}

// textChanged follows a complete edit. The primary view, where the edit
// happened, follows its caret; the others stay where they are.
func (r *renderEditor) textChanged(primary bool) {
	r.goalOK = false
	if primary {
		r.reveal = true
		r.showCaret()
	}
	r.MarkNeedsLayout()
	r.refreshFindBar()
	if f := r.editor.OnChange; f != nil {
		f()
	}
}

// refreshFindBar rebuilds an open find bar, whose count follows the edits and
// the selection.
func (r *renderEditor) refreshFindBar() {
	if r.state != nil && r.state.finding {
		kit.Rebuild(&r.state.StateBase)
	}
}

// selectionChanged follows a move of the selection.
func (r *renderEditor) selectionChanged() {
	r.reveal = true
	r.showCaret()
	r.MarkNeedsLayout()
	r.MarkNeedsPaint()
	r.refreshFindBar()
}

func (r *renderEditor) findChanged() {
	r.MarkNeedsPaint()
	if r.state != nil {
		r.state.followFind(r.ctl)
	}
}

func (r *renderEditor) redecorate(first, last int) {
	r.cache.invalidate(first, last)
	r.MarkNeedsLayout()
}

// scrolled follows a move of either scroll state, by the wheel, the scroll bar
// or a key, and tells OnScroll at once.
func (r *renderEditor) scrolled() {
	r.MarkNeedsLayout()
	r.MarkNeedsPaint()
	r.reportScroll()
}

// padTop returns the room above the first line: the padding, and the find bar
// while it is open, so that the first lines can come out from under it.
func (r *renderEditor) padTop() float32 { return editorPadY + r.findRoom }

// setFindRoom follows the find bar opening, closing or growing a row. Scrolled
// down, the text stays where it is on screen; at the top it moves down, out
// from under the bar.
func (r *renderEditor) setFindRoom(room float32) {
	if room == r.findRoom {
		return
	}
	if top := r.top(); top > 0 && r.state.Ticker != nil {
		r.state.Scroller.Jump(max(top+float64(room-r.findRoom), 0))
		r.state.Ticker.Stop()
	}
	r.findRoom = room
	r.MarkNeedsLayout()
}

// top returns how far the content has scrolled up. It is a float64, as the
// scroll offset is, so that the lines millions of rows down still move a pixel
// at a time; positions in the view are float32.
func (r *renderEditor) top() float64 { return r.state.Offset.Value() }

// viewTop returns the top of line i from the top of the view.
func (r *renderEditor) viewTop(i int) float32 {
	return float32(float64(r.padTop()) + r.lineTop(i) - r.top())
}

// contentHeight returns the height of all the rows, with the padding.
func (r *renderEditor) contentHeight() float64 {
	return float64(r.rows.total())*float64(r.lineHeight) + float64(r.padTop()+editorPadY)
}

// scrollLimit returns the largest offset for a view of height view.
func (r *renderEditor) scrollLimit(view float32) float64 {
	return max(r.contentHeight()-float64(view), 0)
}

// hscroll returns how far the text has scrolled left. No line is wide enough
// to need more than a float32.
func (r *renderEditor) hscroll() float32 { return float32(r.state.horz.Offset.Value()) }

// innerWidth is the room of the text when it does not wrap.
func (r *renderEditor) innerWidth() float32 {
	return r.size.W - r.textLeft - editorPadX - editorBarRoom
}

// scrollPosition returns the scroll position of the view, from the rows as
// they are. A view pulled past an end counts as at the end.
func (r *renderEditor) scrollPosition() ScrollPosition {
	lh := float64(r.lineHeight)
	m := &r.state.Metrics
	top := min(max(r.top(), 0), max(m.Content-m.View, 0))
	line, row := r.rows.find(int((top - float64(r.padTop())) / lh))
	box := float64(0)
	if line > 0 {
		box = float64(r.padTop()) + float64(row)*lh
	}
	p := ScrollPosition{Line: line, Offset: float32(max(top-box, 0))}
	if r.wrap >= geom.Inf {
		h := &r.state.horz.Metrics
		p.X = float32(min(max(float64(r.hscroll()), 0), max(h.Content-h.View, 0)))
	}
	return p
}

// jumpTo scrolls the view to the line and offset of p, within the content.
func (r *renderEditor) jumpTo(p ScrollPosition) {
	line := min(max(p.Line, 0), r.ctl.buf.lineCount()-1)
	top := float64(p.Offset)
	if line > 0 {
		top += float64(r.padTop()) + r.lineTop(line)
	}
	r.state.Scroller.Jump(min(max(top, 0), r.scrollLimit(r.size.H)))
	r.state.Ticker.Stop()
}

// jumpSideways scrolls the text left by x, within the widest line.
func (r *renderEditor) jumpSideways(x float32) {
	h := &r.state.horz
	h.Scroller.Jump(min(max(float64(x), 0), max(h.Metrics.Content-h.Metrics.View, 0)))
	h.Ticker.Stop()
}

// reportScroll tells OnScroll of a new scroll position.
func (r *renderEditor) reportScroll() {
	v := r.view
	if v == nil || v.r != r || v.hasPending || !r.rowsValid {
		return
	}
	p := r.scrollPosition()
	if p == v.reported {
		return
	}
	v.reported = p
	if v.OnScroll != nil {
		v.OnScroll(p)
	}
}

// scrollLater tells OnScroll of a move that a layout made, on the next frame:
// OnScroll may scroll another editor, which must not be marked for layout
// while the tree is laid out.
func (r *renderEditor) scrollLater() {
	if r.view.OnScroll == nil {
		r.view.reported = r.scrollPosition()
		return
	}
	r.tellLater()
}

// tellLater tells OnScroll and OnSelectionChange of what changed, at the start
// of the next frame. Each tells only a change since it last told.
func (r *renderEditor) tellLater() {
	if r.notify == nil {
		r.notify = r.NewTicker(func(time.Duration) {
			r.notify.Stop()
			if r.Attached() && r.view != nil && r.view.r == r {
				r.reportScroll()
				r.view.tellSelection()
			}
		})
	}
	r.notify.Start()
}

// prepare measures the style once.
func (r *renderEditor) prepare() {
	if r.styleOK {
		return
	}
	r.styleOK = true
	r.styleGen++
	st := r.editor.Style
	probe := r.shaper.Shape("0000000000", st).Layout(geom.Inf)
	r.lineHeight = probe.Size().H
	if runs := probe.Runs(); len(runs) > 0 {
		r.baseline = runs[0].Origin.Y
	}
	r.digit = probe.CaretPosition(10) / 10
	if st.TabStop == 0 {
		spaces := r.shaper.Shape("    x", st).Layout(geom.Inf)
		st.TabStop = spaces.CaretPosition(4) / 4 * editorTabSpaces
	}
	r.style = st
	if r.byteWidth == 0 {
		r.byteWidth = r.digit
	}
}

// Layout implements core.RenderObject. The editor takes the room it is given.
func (r *renderEditor) Layout(c geom.Constraints) geom.Size {
	r.checkView()
	r.prepare()
	w, h := c.MaxW, c.MaxH
	if w >= geom.Inf {
		w = editorDefaultWidth
	}
	if h >= geom.Inf {
		h = r.lineHeight*editorDefaultLines + 2*editorPadY
	}
	r.size = c.Constrain(geom.Size{W: w, H: h})
	r.placeColumns()
	if !r.rowsValid {
		r.rows.reset(r.ctl.buf.lineCount(), r.wrap >= geom.Inf, r.estimate)
		r.rowsValid = true
	}
	v := r.view
	if r.reanchor {
		r.reanchor = false
		r.jumpTo(r.anchorPos)
	}
	if v.reveal {
		// A selection set while no editor showed the view.
		v.reveal = false
		r.reveal = true
	}
	if v.hasPending {
		// SetScroll wins over the caret.
		r.jumpTo(v.pending)
		r.reveal = false
	}
	r.layoutView()
	if r.reveal {
		r.reveal = false
		if r.revealCaret() {
			r.layoutView()
		}
	}
	r.measureSideways()
	switch {
	case v.hasPending:
		v.hasPending = false
		r.jumpSideways(v.pending.X)
		v.reported = r.scrollPosition()
	case r.scrollPosition() != v.reported:
		r.scrollLater()
	}
	if v.focus {
		// Focus asked before the editor was laid out.
		v.focus = false
		r.RequestFocus()
	}
	return r.size
}

// measureSideways feeds the sideways scroll state: the width of the text and
// of the widest line laid out. With Wrap there is nothing to scroll.
func (r *renderEditor) measureSideways() {
	h := &r.state.horz
	inner := max(r.innerWidth(), 0)
	content := inner
	if r.wrap >= geom.Inf {
		content = max(r.widest+kit.CaretWidth, inner)
	}
	h.Metrics.View, h.Metrics.Content = float64(inner), float64(content)
	h.Scroller.SetLimit(float64(content - inner))
}

// placeColumns places the gutter and the text, and decides the wrap width.
func (r *renderEditor) placeColumns() {
	r.numbers = 0
	if r.editor.LineNumbers {
		digits := max(digitCount(r.ctl.buf.lineCount()), 2)
		r.numbers = geom.Ceil32(float32(digits)*r.digit) + 2*editorGutterPad
	}
	r.gutter = r.numbers + r.editor.GutterWidth
	r.textLeft = r.gutter + editorPadX
	wrap := geom.Inf
	if r.editor.Wrap {
		wrap = max(r.size.W-r.textLeft-editorPadX-editorBarRoom, r.lineHeight)
	}
	if wrap != r.wrap {
		r.wrap = wrap
		r.rowsValid = false
		r.state.horz.Scroller.Jump(0)
		r.state.horz.Ticker.Stop()
	}
}

// estimate returns the rows that line i likely takes before it is laid out.
func (r *renderEditor) estimate(i int) int32 {
	if r.wrap >= geom.Inf {
		return 1
	}
	n := float32(len(r.ctl.buf.line(i))) * r.byteWidth / r.wrap
	return max(int32(geom.Ceil32(n)), 1)
}

// layoutView lays out the lines in view and a margin around them, and keeps
// the line at the top of the view in place while estimates above it become
// exact.
func (r *renderEditor) layoutView() {
	n := r.ctl.buf.lineCount()
	lh := r.lineHeight
	view := r.size.H
	top := max(r.top(), 0)
	first, firstRow := r.rows.find(int((top - float64(r.padTop())) / float64(lh)))
	anchor := top - (float64(r.padTop()) + float64(firstRow)*float64(lh))
	page := int(view/lh) + 1
	margin := max(page/2, 8)
	lastGuess, _ := r.rows.find(firstRow + page)
	r.cache.cover(max(first-margin, 0), min(lastGuess+margin, n-1),
		max(first-3*margin-page, 0), min(lastGuess+3*margin+page, n-1))
	for i := max(first-margin, 0); i < first; i++ {
		r.layoutEntry(r.cache.get(i), i)
	}
	// The anchor line stays where it was on screen.
	if top > 0 {
		if want := float64(r.padTop()) + r.lineTop(first) + anchor; want != top {
			r.state.Offset.Set(want)
			r.state.Scroller.ShiftGlide(want - top)
			top = want
		}
	}
	bottom := top + float64(view-r.padTop())
	i := first
	for y := r.lineTop(first); i < n && y < bottom; i++ {
		e := r.cache.get(i)
		if e == nil {
			r.cache.cover(i, min(i+margin, n-1), max(first-3*margin-page, 0), min(i+3*margin+page, n-1))
			e = r.cache.get(i)
		}
		r.layoutEntry(e, i)
		y += float64(r.rows.at(i)) * float64(lh)
	}
	r.first, r.last = first, max(i-1, first)
	for k := r.last + 1; k <= min(r.last+margin, n-1); k++ {
		if e := r.cache.get(k); e != nil {
			r.layoutEntry(e, k)
		}
	}
	content := r.contentHeight()
	r.state.Metrics.View, r.state.Metrics.Content = float64(view), content
	r.state.Scroller.SetLimit(content - float64(view))
}

// display returns line i as the editor shows it: with the composition.
func (r *renderEditor) display(i int) string {
	if r.composition != nil && i == r.compLine {
		return r.compText
	}
	return r.ctl.buf.line(i)
}

// layoutEntry brings e up to date with line i.
func (r *renderEditor) layoutEntry(e *lineEntry, i int) {
	src := r.display(i)
	same := e.valid && e.src == src && e.styleGen == r.styleGen && e.decoGen == r.ctl.decoGen
	if same && e.wrap == r.wrap {
		return
	}
	if !same {
		e.decos = e.decos[:0]
		if f := r.editor.Decorations; f != nil && (r.composition == nil || i != r.compLine) {
			e.decos = f(i, src, e.decos)
		}
		e.line = LineStyle{}
		if f := r.editor.LineStyle; f != nil {
			// The line keeps its colors while a composition runs in it,
			// from the text without the composition.
			e.line = f(i, r.ctl.buf.line(i))
		}
		e.para = r.shapeLine(e, src)
	}
	e.layout = e.para.Layout(r.wrap)
	e.src, e.wrap, e.valid, e.styleGen, e.decoGen = src, r.wrap, true, r.styleGen, r.ctl.decoGen
	r.makeSquiggles(e)
	r.rows.set(i, int32(e.layout.LineCount()), true)
	if r.wrap >= geom.Inf {
		r.widest = max(r.widest, e.layout.Size().W)
	}
	if len(src) >= 8 && e.layout.LineCount() == 1 {
		r.sampledBytes += float32(len(src))
		r.sampled += e.layout.Size().W
		r.byteWidth = r.sampled / r.sampledBytes
	}
}

// lineLayout returns the layout of line i, laying it out outside the cache if
// it is not in it.
func (r *renderEditor) lineLayout(i int) *text.Layout {
	if r.ctl.buf.lineCount() != r.rows.count() || !r.styleOK {
		// Before the first layout, or between an edit and the next.
		r.prepare()
		if !r.rowsValid || r.ctl.buf.lineCount() != r.rows.count() {
			r.rows.reset(r.ctl.buf.lineCount(), r.wrap >= geom.Inf, r.estimate)
			r.rowsValid = true
		}
	}
	if e := r.cache.get(i); e != nil {
		r.layoutEntry(e, i)
		return e.layout
	}
	r.temp.valid = false
	r.layoutEntry(&r.temp, i)
	return r.temp.layout
}

// lineTop returns the top of line i in the content, below the padding.
func (r *renderEditor) lineTop(i int) float64 {
	return float64(r.rows.before(i)) * float64(r.lineHeight)
}

// revealCaret scrolls so that the caret is in view, and reports whether the
// view moved.
func (r *renderEditor) revealCaret() bool {
	caret := r.sel().caret
	l := r.lineLayout(caret.Line)
	rect := l.CaretRect(r.displayColumn(caret))
	view := float64(r.size.H)
	top := r.top()
	y := float64(r.padTop()+rect.Y) + r.lineTop(caret.Line)
	below := float64(rect.H + editorPadY)
	want := top
	switch {
	case y+below > top+view:
		want = y + below - view
	case y-float64(r.padTop()) < top:
		want = y - float64(r.padTop())
	}
	want = max(min(want, r.scrollLimit(r.size.H)), 0)
	moved := false
	if want != top {
		r.state.Scroller.Jump(want)
		r.state.Ticker.Stop()
		moved = true
	}
	if r.wrap >= geom.Inf {
		inner := r.innerWidth()
		x, at := rect.X, r.hscroll()
		to := at
		switch {
		case x-at > inner-kit.CaretWidth:
			to = x - inner + kit.CaretWidth
		case x < at:
			to = max(x-inner/4, 0)
		}
		if to != at {
			h := &r.state.horz
			h.Scroller.Jump(float64(to))
			h.Ticker.Stop()
		}
	}
	return moved
}

// displayColumn returns where p is in the line as the editor shows it.
func (r *renderEditor) displayColumn(p TextPosition) int {
	if r.composition != nil && p.Line == r.compLine {
		return r.compStart + r.composition.Caret()
	}
	return p.Column
}

// showCaret puts the caret on for a full phase, as Input does.
func (r *renderEditor) showCaret() {
	if !r.focused {
		return
	}
	if r.blink == nil {
		r.blink = r.NewTicker(r.tickCaret)
	}
	r.caretHidden, r.blinkReset = false, true
	r.blink.Start()
	r.MarkNeedsPaint()
}

func (r *renderEditor) tickCaret(now time.Duration) {
	if !r.focused || !r.Attached() {
		r.blink.Stop()
		return
	}
	if r.blinkReset {
		r.blinkReset = false
	} else {
		r.caretHidden = !r.caretHidden
		r.MarkNeedsPaint()
	}
	r.blink.SleepUntil(now + kit.CaretBlink)
}

// digitCount returns the number of decimal digits of n.
func digitCount(n int) int {
	d := 1
	for ; n >= 10; n /= 10 {
		d++
	}
	return d
}
