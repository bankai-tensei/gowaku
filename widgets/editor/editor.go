package editor

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

const (
	// matchOpacity is the accent behind every match of a search, and
	// currentMatchOpacity the outline of the one the selection is on.
	matchOpacity        = 0.22
	currentMatchOpacity = 0.7
)

// Underline is the line a Decoration draws under its text.
//
// It is UnderlineNone (the zero value), UnderlineSolid or UnderlineSquiggle.
type Underline uint8

const (
	// UnderlineNone draws no line.
	UnderlineNone Underline = iota
	// UnderlineSolid draws a straight line, as for a link.
	UnderlineSolid
	// UnderlineSquiggle draws a wavy line, as for a misspelled word.
	UnderlineSquiggle
)

// Decoration styles a stretch of one line of a Editor. Zero fields
// change nothing, so a decoration can set only a background or only a color.
// Where decorations overlap, the later one wins for Color; Bold and Italic
// add up, and backgrounds and underlines all show.
type Decoration struct {
	// Start and End are byte offsets into the line.
	Start, End int
	// Color is the color of the text.
	Color geom.Color
	// Background fills behind the text.
	Background geom.Color
	// Underline draws a line under the text, in UnderlineColor or else in
	// the color of the text.
	Underline      Underline
	UnderlineColor geom.Color
	// Bold sets the text in weight 700.
	Bold bool
	// Italic sets the text in the italic face of the font, or slants it where
	// the font has none. The text keeps its advance, so the caret and the
	// columns stay where they were.
	Italic bool
}

// LineBarWidth is the width of the bar of LineStyle.Bar.
const LineBarWidth = 3

// LineStyle colors a whole line of an Editor, as a diff colors the lines it
// added and removed. Zero fields draw nothing.
type LineStyle struct {
	// Background fills the whole width of the text area behind the line:
	// from the gutter to the right edge, wherever the view has scrolled
	// sideways, and all the rows of a line that wraps, an empty line
	// included. The current line, the matches of a search, the backgrounds of
	// Decorations and the selection show on top of it.
	Background geom.Color
	// Gutter fills the gutter beside the line, behind its number.
	Gutter geom.Color
	// Bar draws a strip LineBarWidth wide at the left edge of the text area,
	// beside all the rows of the line, as the change bars of a diff.
	Bar geom.Color
}

// GutterLine is what Editor.Gutter gets for each line it shows.
type GutterLine struct {
	// Line is the index of the line, from 0.
	Line int
	// Rect is the part of the gutter beside the line, all its rows, on the
	// canvas, right of the line numbers if they show.
	Rect geom.Rect
	// Baseline is the y of the baseline of the first row, on the canvas.
	Baseline float32
	// Current means the caret is on the line.
	Current bool
}

// Editor edits a large plain text: a document of 100,000 lines scrolls and
// takes typing without delay, because only the lines in view are shaped. It has
// the selection, the clipboard, undo, IME composition and a find bar (Ctrl+F,
// with Ctrl+H for replace and F3 or Shift+F3 to move between matches).
//
// The text lives in Controller, which also selects, edits and searches from
// code. Without one the editor keeps its own, which suits an editor whose text
// only the user changes:
//
//	var doc editor.Controller // kept in the State
//	doc.SetText(source)
//	editor.Editor{Controller: &doc, LineNumbers: true}
//
// The text is monospaced unless Style names another Family: the zero Style
// takes the Code text of the theme (theme.TextTheme.Code). Decorations put a
// color, a background, an underline or bold on stretches of a line, for syntax
// colors or the squiggles of a spell checker. LineStyle colors whole lines,
// across the width of the editor, as a diff does. Gutter draws beside each line.
//
// Several Editors can show one Controller, as the panes of a split view do:
// they share the text, the history and the search, and each keeps its own
// selection and scroll position in a View (see View).
//
// Like Scroll, an Editor takes all the room it is given, and it shows a
// default size where that room is unbounded.
//
// An Editor suits a long document, with line numbers, decorations and a find
// bar; for a field in a form or a short note, with a visible label, ruby or
// vertical text, use a field.Input with Multiline, which grows with its text.
type Editor struct {
	core.StatefulBase
	// Controller holds the text. Nil makes the editor keep its own.
	Controller *Controller
	// View holds the selection and the scroll position of this editor. Nil
	// makes the editor keep its own. A View shows in one Editor at a time.
	View *View
	// Label names the editor to assistive technology.
	Label string
	// Style sets the font. Zero fields take the Code text of the theme, which
	// is monospaced; set Family for a proportional one.
	Style text.Style
	// Wrap breaks long lines at the width of the editor. Without it they run
	// on, and the view scrolls sideways: to the caret, with Shift and the wheel,
	// a sideways wheel or touchpad, or the scroll bar along the bottom.
	Wrap bool
	// ReadOnly lets the user select, copy and search, but not edit.
	ReadOnly bool
	// TabMovesFocus makes Tab move the focus on, as in a form, instead of
	// typing a tab. Shift+Tab always moves the focus back.
	TabMovesFocus bool
	// LineNumbers shows the number of each line in the gutter.
	LineNumbers bool
	// HighlightCurrentLine tints the line of the caret.
	HighlightCurrentLine bool
	// Border draws a hairline around the editor, with rounded corners, as
	// for an editor in a form. Without it the editor is a plain surface for a
	// pane.
	Border bool
	// GutterWidth is the room of Gutter, after the line numbers.
	GutterWidth float32
	// Gutter, if set, draws in the gutter beside each line in view, as a
	// breakpoint or a change bar. It runs on every paint and must not
	// allocate.
	Gutter func(cv *paint.Canvas, line GutterLine)
	// Decorations, if set, returns the decorations of line i, whose text is
	// s, appended to dst. The editor calls it when it lays the line out: when
	// the line comes into view or changes. Call Redecorate on the controller
	// when the decorations change for another reason.
	Decorations func(i int, s string, dst []Decoration) []Decoration
	// LineStyle, if set, returns how line i, whose text is s, is colored as a
	// whole: a background across the text area, a gutter and a bar. Like
	// Decorations, the editor calls it when it lays the line out, and
	// Redecorate and RedecorateLines on the controller call it again.
	LineStyle func(i int, s string) LineStyle
	// OnChange runs after every change of the text but SetText, by the user or
	// through the controller.
	OnChange func()
}

type textEditorState struct {
	basic.ScrollState
	// horz scrolls the text sideways, when it does not wrap.
	horz    basic.ScrollState
	own     Controller
	ownView View
	render  *renderEditor
	// The find bar: whether it shows, with the replace row, and what it holds.
	finding, replacing bool
	query, replacement string
	options            FindOptions
	badQuery           bool
	// focusRequest changes to give the query the keyboard.
	focusRequest int
	// queryPending is set from openFind until the query has the keyboard, and
	// typedAhead once text typed in that time went into the query instead of
	// the document.
	queryPending, typedAhead bool
	glyphs                   findGlyphs
}

// CreateState implements core.StatefulWidget.
func (Editor) CreateState() core.State { return &textEditorState{} }

// InitState implements core.State.
func (s *textEditorState) InitState() {
	s.ScrollState.InitState()
	// The two scroll states share the element of the editor, which their
	// tickers need: StateBase is only a pointer to it.
	s.horz.StateBase = s.StateBase
	s.horz.InitState()
	moved := func() {
		if s.render != nil {
			s.render.scrolled()
		}
	}
	s.OnMove, s.horz.OnMove = moved, moved
}

func (s *textEditorState) controller() *Controller {
	if c := s.Widget().(Editor).Controller; c != nil {
		return c
	}
	return &s.own
}

func (s *textEditorState) view() *View {
	if v := s.Widget().(Editor).View; v != nil {
		return v
	}
	return &s.ownView
}

// activate makes the view of this editor the one the selection methods of the
// controller act on, before the find bar moves the selection.
func (s *textEditorState) activate() { s.controller().activate(s.view()) }

// editorStyle fills in what the style of an editor leaves open.
func editorStyle(th *theme.Theme, st text.Style) text.Style {
	if st.Color == (geom.Color{}) {
		st.Color = th.Color.Ink
	}
	if st.Family == "" {
		st.Family = th.Text.Code.Family
	}
	if st.Size == 0 {
		st.Size, st.LineHeight = th.Text.Code.Size, th.Text.Code.LineHeight
	}
	return st
}

// Build implements core.State.
func (s *textEditorState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget().(Editor)
	shaper, ok := core.DependOn[*text.Shaper](ctx)
	if !ok {
		panic("gowaku: editor.Editor needs a text.Shaper above it; wrap the root in core.Inherited[*text.Shaper]")
	}
	th := basic.ThemeOf(ctx)
	c := &th.Color
	w.Style = editorStyle(th, w.Style)
	view := editorView{
		state:  s,
		shaper: shaper,
		editor: w,
		ctl:    s.controller(),
		view:   s.view(),
		look: editorLook{
			caret:        c.AccentInk,
			selection:    c.AccentTint,
			match:        c.Accent.WithAlpha(matchOpacity),
			currentMatch: c.AccentInk.WithAlpha(currentMatchOpacity),
			currentLine:  c.Fill,
			number:       c.Ink3,
			current:      c.Ink2,
			target:       c.Accent.WithAlpha(kit.TargetOpacity),
		},
		findRoom: s.findRoom(),
	}
	// The stack stays when the find bar goes, so that the view keeps its
	// render object and the lines it laid out. The sideways bar is there with
	// Wrap too, where it has nothing to scroll and does not show.
	frame := basic.ScrollFrame{Axis: layout.Vertical, State: &s.ScrollState, View: basic.ScrollFrame{
		Axis: layout.Horizontal, State: &s.horz, View: view,
	}}
	layers := []core.Widget{frame}
	if s.finding {
		layers = append(layers, basic.Positioned{Top: findBarTop, Right: theme.Space16, AnchorRight: true, Child: s.findBar(ctx)})
	}
	content := basic.Stack{Children: layers}
	box := basic.Container{Color: c.Card, Child: content}
	if w.Border {
		box.Border = paint.Stroke{Width: 1, Color: c.Hair}
		box.Radius = th.Radius.Control
		box.Padding = geom.All(1)
	}
	return box
}

// openFind shows the find bar with the query in it, taken from the selection
// when that is part of one line, and gives the query the keyboard.
func (s *textEditorState) openFind(replace bool) {
	s.activate()
	ctl := s.controller()
	if sel := ctl.Selection().Ordered(); !sel.Empty() && sel.Start.Line == sel.End.Line {
		s.query = ctl.SelectedText()
	}
	s.finding = true
	s.replacing = replace
	s.focusRequest++
	s.queryPending, s.typedAhead = true, false
	s.search()
	kit.Rebuild(&s.StateBase)
}

// typeAhead takes text typed after Ctrl+F but before the query has the
// keyboard, which comes on the next layout, and reports whether it did.
func (s *textEditorState) typeAhead(typed string) bool {
	if !s.queryPending || !s.finding {
		return false
	}
	if !s.typedAhead {
		s.query, s.typedAhead = "", true
	}
	s.query += typed
	s.search()
	kit.Rebuild(&s.StateBase)
	return true
}

// queryFocused ends the time in which typing goes to the query.
func (s *textEditorState) queryFocused() { s.queryPending = false }

// closeFind hides the find bar, ends the search and gives the editor back the
// keyboard.
func (s *textEditorState) closeFind() {
	if !s.finding {
		return
	}
	s.finding, s.replacing = false, false
	s.controller().endFind()
	if s.render != nil {
		s.render.RequestFocus()
	}
	kit.Rebuild(&s.StateBase)
}

// search runs the query of the find bar.
func (s *textEditorState) search() {
	s.activate()
	_, err := s.controller().Find(s.query, s.options)
	s.badQuery = err != nil
}

// findProgress follows matches that a search on a goroutine of its own
// found: an open find bar counts them anew.
func (s *textEditorState) findProgress() {
	if s.finding {
		kit.Rebuild(&s.StateBase)
	}
}

// followFind shows the search of c in the find bar, if it is open: another
// editor of the document, or the code, may have changed it, and a bar must
// never act on a query it does not show. A closed bar keeps its own query for
// the next Ctrl+F.
func (s *textEditorState) followFind(c *Controller) {
	if !s.finding || s.query == c.typed && s.options == c.find.opts && s.badQuery == c.badQuery {
		return
	}
	s.query, s.options, s.badQuery = c.typed, c.find.opts, c.badQuery
	kit.Rebuild(&s.StateBase)
}

// Dispose implements core.State.
func (s *textEditorState) Dispose() {
	s.ScrollState.Dispose()
	s.horz.Dispose()
	if s.render != nil {
		s.render.detach()
	}
}

// editorView is the render widget of the text of a Editor.
type editorView struct {
	state  *textEditorState
	shaper *text.Shaper
	editor Editor
	ctl    *Controller
	view   *View
	look   editorLook
	// findRoom is the room the open find bar takes over the top of the view.
	findRoom float32
}

// Key implements core.Widget.
func (editorView) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (v editorView) CreateRenderObject() core.RenderObject {
	r := &renderEditor{state: v.state, shaper: v.shaper, wrap: geom.Inf}
	v.state.render = r
	r.cache.reset()
	v.apply(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (v editorView) UpdateRenderObject(ro core.RenderObject) { v.apply(ro.(*renderEditor)) }

func (v editorView) apply(r *renderEditor) {
	old := r.editor
	r.editor, r.state, r.shaper = v.editor, v.state, v.shaper
	r.setFindRoom(v.findRoom)
	if r.ctl != v.ctl || r.view != v.view {
		r.detach()
		r.ctl, r.view = v.ctl, v.view
		// A view that another editor still holds moves here; that editor
		// panics if it stays in the tree (see checkView). It may well not
		// stay, as when two editors swap their views or a new keyed editor
		// takes the view before the old one goes, so the view keeps its place
		// from there first.
		if from := v.view.r; from != nil && from != r {
			from.keepPlace()
		}
		v.view.r = r
		v.ctl.attach(v.view)
		// Work for the document from other goroutines, as the lines of a file
		// still being read, reaches it through this editor.
		v.ctl.follow(v.state.Poster())
		r.textReset()
	}
	if !old.Style.SameLayout(v.editor.Style) || old.Wrap != v.editor.Wrap ||
		old.LineNumbers != v.editor.LineNumbers || old.GutterWidth != v.editor.GutterWidth {
		r.styleChanged()
	}
	if old.Style.PaintColor() != v.editor.Style.PaintColor() || r.look != v.look ||
		old.HighlightCurrentLine != v.editor.HighlightCurrentLine {
		r.look = v.look
		r.MarkNeedsPaint()
	}
	if old.ReadOnly != v.editor.ReadOnly && r.focused {
		r.Host().SetIMEEnabled(!v.editor.ReadOnly)
	}
	// A new Gutter, Decorations or LineStyle may draw otherwise; functions cannot be
	// compared, so the paint follows every build and Redecorate the rest.
	r.MarkNeedsPaint()
}
