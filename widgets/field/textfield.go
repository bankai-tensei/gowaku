package field

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

const (
	fieldPaddingX     = theme.Space10
	fieldHeight       = theme.ControlHeight
	fieldDefaultWidth = 240
	fieldLabelGap     = theme.Space4
	// fieldWheelDistance is how far a notch of the wheel moves the lines.
	fieldWheelDistance = 60

	selectionOpacity = 0.25
)

// fieldDecorGap is the room between the text and what a variant of the field
// puts beside it.
const fieldDecorGap = theme.Space4

// fieldLook is what a field takes from the theme.
type fieldLook struct {
	caret, placeholder, selection geom.Color
	// target marks the clause the IME is converting.
	target geom.Color
}

// Input edits text, with selection, the clipboard, undo and IME composition.
//
// It keeps its own text, so that the zero value works. Set Value to change that
// text from outside: the field follows whenever Value differs from the last
// Value it was given. Use OnChange to learn about edits.
//
// Label, if set, names the field above it and to a screen reader. HideLabel
// keeps the name for the screen reader and draws no caption, for a field whose
// row already says what it is, such as a row of settings or the editor of a
// cell. A field that sits in a GroupRow has its name there, and needs none.
//
// A tap or a click puts the caret where it lands and selects nothing; a
// double click selects a word and a triple click the whole text. To replace
// the whole text, as a test does, select it first with Ctrl+A (Cmd+A on
// macOS) and then type: tt.Key(input.KeyA, input.ModControl), then tt.Type.
//
// A single-line field is one control high. A Multiline field grows with its
// text when nothing sets its height, as in a Column, and scrolls its lines
// once a height is set: it fills the height that a parent such as
// basic.Expanded or basic.SizedBox gives it, with or without a Label.
// MaxLines stops the growth at that many lines, after which the lines scroll,
// as in the box of a chat; MinLines keeps room for that many while the text
// is shorter. A multiline Input suits a field in a form or a short note; for a long
// document, with line numbers, decorations, a find bar and a view that shapes
// only the lines on screen, use editor.Editor.
type Input struct {
	core.StatefulBase
	Value string
	Label string
	// HideLabel draws no caption for Label, which still names the field to a
	// screen reader.
	HideLabel bool
	// Placeholder shows while the field is empty.
	Placeholder string
	// Disabled grays the field out: it takes no keyboard and no pointer, and a
	// screen reader hears it as disabled. Input keeps its own text, so a nil
	// OnChange cannot mean disabled as a nil OnPress does for a Button; the
	// name breaks the rule of positive names (CODING_STYLE §4.1) so that the
	// zero value is a field that works.
	Disabled bool
	// ReadOnly shows the text and keeps the user from changing it: typing,
	// the IME, paste, cut, undo and the edits of a screen reader do nothing,
	// while the caret moves, the text can be selected and copied, and Enter
	// still runs OnSubmit. A screen reader hears it as read-only. Value and
	// the Controller still change the text from code.
	ReadOnly bool
	// Multiline lets Enter and Shift+Enter start a new line, and wraps long
	// lines.
	Multiline bool
	// MinLines and MaxLines bound the height of a Multiline field that grows
	// with its text: it is at least MinLines lines high and at most MaxLines,
	// after which its lines scroll. Zero leaves that end open. A height set
	// by the parent wins over both, and a single-line field ignores them.
	MinLines, MaxLines int
	// SubmitOnEnter makes Enter in a Multiline field run OnSubmit, or submit
	// the Form the field is in, while Shift+Enter starts a new line, as in the
	// box of a chat. Enter that confirms what an IME is converting confirms it
	// and submits nothing, and text an IME commits keeps its line breaks.
	// A single-line field submits on Enter without it.
	SubmitOnEnter bool
	// Autofocus gives the field the keyboard when it is first laid out, as for
	// the field of a dialog. It wins over the autofocus of the dialog or sheet
	// the field is in. InputController.Focus does the same from code.
	Autofocus bool
	Style     text.Style
	// OnChange runs after every edit. Text an IME is still converting does not
	// count as an edit.
	OnChange func(string)
	// OnSubmit runs when Enter is pressed in a single-line field, or in a
	// Multiline one with SubmitOnEnter. Without it, Enter submits the Form the
	// field is in, if any. Shift+Enter submits a single-line field as well, as
	// in the fields of browsers and of the platforms. The field uses only
	// Enter and Shift+Enter: Enter with Ctrl or Alt (Command on macOS) goes on
	// to the interact.Actions above it, so that a Ctrl+Enter shortcut runs
	// while the field has the keyboard.
	OnSubmit func(string)
	// Ruby, if set, finds ruby written into the text with a notation, such as
	// aozora.Marks for ｜漢字《かんじ》. The field then draws the ruby over its
	// base, or to its right in vertical text, and hides the notation, except
	// for the marks that an end of the selection is inside: those show as
	// written, to be edited. The arrow keys step into a mark, which shows it,
	// and a mark being typed shows until its closing character. Everything else
	// works on the text as written: the selection, copy and undo, OnChange and
	// what a screen reader gets. A mark with Emphasis draws emphasis marks
	// beside its base, so aozora.Marks also hides the note of
	// 対象［＃「対象」に傍点］ and dots 対象, the same way. Marks over ruby make
	// the lines taller than Style.RubyRoom does, so the lines move when the
	// first such mark shows or the last hides.
	//
	// Ruby appends the marks of value to dst in order and returns the result.
	// A mark may not span a line break: after an edit, value is only the lines
	// the edit touched, without the line feed after them.
	Ruby func(value string, dst []text.RubyMark) []text.RubyMark
	// Controller, if set, reads and sets the selection, edits and gives the
	// field the keyboard from code. See InputController.
	Controller *InputController
	// OnSelectionChange runs when the anchor or the caret moves: by a key, the
	// pointer, an edit, undo, a screen reader or the Controller. The offsets
	// are bytes of the text, as in InputController.Selection, and come
	// after OnChange for an edit. Text an IME is still converting does not
	// move the caret until it is committed.
	OnSelectionChange func(anchor, caret int)
	// Bare leaves out the box: no border, no ground and no focus ring, for a
	// field that is the page, as the text of an editor. The caret alone shows
	// the focus, as in the editors of the platforms. The error of a FormItem
	// around it shows only in the message.
	Bare bool
	// Padding, if not zero, is the room between the edge of the field and its
	// text. Zero gives the room of a control to a field with a box, and none
	// to a Bare one. A Bare field that fills a page wants some, so that the
	// text scrolls under it rather than being cut at its edge.
	Padding geom.Insets
	// Obscure makes the field a password field. It draws a bullet for each
	// character, keeps the IME off, so that no input method composes or
	// learns the text, and asks the window for secure input while it has the
	// keyboard. Copy and cut do nothing, while paste works; undo keeps no
	// history; a screen reader gets the bullets and hears a password field.
	// The caret, the selection, the pointer and deletion still work on the
	// characters, and moves by word go to the ends. An obscured field has one
	// line and no ruby: Multiline and Ruby are ignored. Vertical text stays.
	//
	// Obscure can change at any time, as for a button that shows the
	// password; the undo history is cleared then. See
	// docs/ADR/0088-input-obscure.md.
	Obscure bool

	// Extras is what the variants of the field, such as Search, add.
	Extras Extras
}

// Extras is what a variant of Text puts inside its box, and what it learns
// from the field. Outside this package only OnFocus can be set.
type Extras struct {
	// leading and trailing sit before and after the text, inside the border.
	leading, trailing core.Widget
	// OnFocus reports gaining and losing the keyboard.
	OnFocus func(focused bool)
	// err is the error of the FormItem around the field, for its semantics.
	err string
	// state holds the Invalid and Required states the FormItem gives.
	state semantics.State
	// onKey sees the keys before the field does, and returns true for those
	// it takes, as a TokenField does at the start of the text.
	onKey func(e *core.KeyEvent) bool
}

type textFieldState struct {
	core.StateBase
	focused, hovered bool
	tint             motion.Implicit
	lit, hot         *motion.Value[float32]
}

// CreateState implements core.StatefulWidget.
func (Input) CreateState() core.State { return &textFieldState{} }

// InitState implements core.State.
func (s *textFieldState) InitState() {
	s.tint.Attach(&s.StateBase, func() { kit.Rebuild(&s.StateBase) })
	s.lit = motion.Track(&s.tint, 0, motion.LerpFloat)
	s.hot = motion.Track(&s.tint, 0, motion.LerpFloat)
}

// Dispose implements core.State.
func (s *textFieldState) Dispose() { s.tint.Dispose() }

func (s *textFieldState) sync() {
	s.lit.Set(kit.OnOff(s.focused))
	s.hot.Set(kit.OnOff(s.hovered))
	s.tint.Commit(kit.HoverDuration, motion.Linear)
}

func (s *textFieldState) setFocused(focused bool) {
	s.focused = focused
	s.sync()
	kit.Rebuild(&s.StateBase)
}

func (s *textFieldState) setHovered(hovered bool) {
	s.hovered = hovered
	s.sync()
}

// fieldStyle fills in what the style of a field leaves open.
func fieldStyle(th *theme.Theme, s text.Style) text.Style {
	if s.Color == (geom.Color{}) {
		s.Color = th.Color.Ink
	}
	if s.Size == 0 {
		s.Size, s.LineHeight = th.Text.Control.Size, th.Text.Control.LineHeight
	}
	return s
}

// Build implements core.State.
func (s *textFieldState) Build(ctx core.BuildContext) core.Widget {
	f := s.Widget().(Input)
	shaper, ok := core.DependOn[*text.Shaper](ctx)
	if !ok {
		panic("gowaku: field.Input needs a text.Shaper above it; wrap the root in core.Inherited[*text.Shaper]")
	}
	th := basic.ThemeOf(ctx)
	c := &th.Color
	f.Style = fieldStyle(th, f.Style)
	if f.Obscure {
		f.Multiline, f.Ruby = false, nil
	}
	lineHeight := f.Style.LineHeight
	if f.Ruby != nil {
		// Lines keep room for ruby whether or not it shows.
		f.Style.RubyRoom = true
		lineHeight = max(lineHeight, text.RubyLineHeight)
	}
	line := f.Style.Size * lineHeight
	lit := s.lit.Get()
	border := motion.LerpColor(motion.LerpColor(c.Hair, c.Ink3, s.hot.Get()), c.AccentInk, lit)
	ring := c.AccentInk
	ground := c.Field
	if f.Disabled {
		// As every disabled control: Fill and Ink3, not translucency.
		ground, border, lit = c.Fill, geom.Color{}, 0
		f.Style.Color = c.Ink3
	}
	// A FormItem around the field shows its error in red, and takes its focus
	// and Enter for validating and submitting.
	scope := fieldScopeOf(ctx)
	if scope.err != "" {
		if !f.Disabled {
			border, ring = c.Danger, c.Danger
		}
		f.Extras.err = scope.err
	}
	f.Extras.state = scope.state()
	if f.OnSubmit == nil && (!f.Multiline || f.SubmitOnEnter) && scope.form != nil {
		f.OnSubmit = func(string) { scope.submit() }
	}
	onFocus := s.setFocused
	if extras := f.Extras; scope.field != nil || extras.OnFocus != nil {
		onFocus = func(focused bool) {
			s.setFocused(focused)
			scope.focusChanged(focused)
			if extras.OnFocus != nil {
				extras.OnFocus(focused)
			}
		}
	}
	insets := fieldInsets(f.Style.Orientation == text.Vertical, line)
	switch {
	case f.Padding != (geom.Insets{}):
		insets = f.Padding
	case f.Bare:
		insets = geom.Insets{}
	}
	if f.Extras.leading != nil {
		insets.Left = fieldDecorGap
	}
	if f.Extras.trailing != nil {
		insets.Right = fieldDecorGap
	}
	var content core.Widget = editable{
		shaper:  shaper,
		field:   f,
		insets:  insets,
		onFocus: onFocus,
		look: fieldLook{
			caret:       c.AccentInk,
			placeholder: c.Ink3,
			selection:   c.Accent.WithAlpha(selectionOpacity),
			target:      c.Accent.WithAlpha(kit.TargetOpacity),
		},
	}
	if f.Extras.leading != nil || f.Extras.trailing != nil {
		parts := make([]core.Widget, 0, 3)
		if f.Extras.leading != nil {
			parts = append(parts, f.Extras.leading)
		}
		parts = append(parts, basic.Expanded{Child: content})
		if f.Extras.trailing != nil {
			parts = append(parts, f.Extras.trailing)
		}
		content = basic.Row{Cross: layout.CrossCenter, Children: parts}
	}
	var body core.Widget
	if f.Bare {
		body = content
	} else {
		body = interact.MouseRegion{
			OnEnter: func() { s.setHovered(true) },
			OnExit:  func() { s.setHovered(false) },
			Child: kit.Ring{Radius: th.Radius.Control, Color: ring.WithAlpha(kit.RingOpacity * lit), Child: basic.Container{
				Color:  ground,
				Radius: th.Radius.Control,
				Border: paint.Stroke{Width: 1, Color: border},
				Child:  content,
			}},
		}
	}
	if f.Label == "" || f.HideLabel {
		return body
	}
	name := basic.Text{Content: f.Label, Style: kit.Colored(th.Text.Caption, c.Ink2)}
	if f.Multiline {
		return basic.CustomLayout{Delegate: labeledLayout{}, Children: []core.Widget{name, body}}
	}
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: fieldLabelGap, Children: []core.Widget{
		name,
		body,
	}}
}

// labeledLayout stacks the label of a multiline Input over its box and hands
// the box what is left of the height the field is given, so that the box
// fills an Expanded as it does without a label. A Column would let the box
// stay as short as its text.
type labeledLayout struct{}

// Layout implements layout.Delegate.
func (labeledLayout) Layout(c layout.Constraints, children []core.RenderObject) geom.Size {
	label, box := children[0], children[1]
	across := geom.Constraints{MaxW: c.MaxW, MaxH: geom.Inf}
	if c.MaxW < geom.Inf {
		across.MinW = c.MaxW
	}
	name := core.LayoutChild(label, across)
	top := name.H + fieldLabelGap
	rest := across
	rest.MinH = max(c.MinH-top, 0)
	if c.MaxH < geom.Inf {
		rest.MaxH = max(c.MaxH-top, rest.MinH)
	}
	sz := core.LayoutChild(box, rest)
	label.SetOffset(geom.Offset{})
	box.SetOffset(geom.Offset{Y: top})
	return geom.Size{W: max(sz.W, name.W), H: top + sz.H}
}

// editable is Input with its Shaper found.
type editable struct {
	shaper *text.Shaper
	field  Input
	look   fieldLook
	insets geom.Insets
	// onFocus reports gaining and losing the keyboard.
	onFocus func(bool)
}

// Key implements core.Widget.
func (editable) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (e editable) CreateRenderObject() core.RenderObject {
	r := &RenderEditable{shaper: e.shaper, field: e.field, look: e.look, insets: e.insets, onFocus: e.onFocus, value: e.field.Value}
	r.setText(e.field.Value)
	if c := e.field.Controller; c != nil {
		c.attach(r)
	}
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (e editable) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*RenderEditable)
	old := r.field
	r.field, r.onFocus = e.field, e.onFocus
	if old.Controller != e.field.Controller {
		if old.Controller != nil {
			old.Controller.remember(r)
			old.Controller.detach(r)
		}
		if e.field.Controller != nil {
			e.field.Controller.attach(r)
			r.revealCaret()
			r.moveRuby()
			r.MarkNeedsPaint()
		}
	}
	if !old.Style.SameLayout(e.field.Style) || old.Multiline != e.field.Multiline ||
		old.Placeholder != e.field.Placeholder || r.insets != e.insets || (old.Ruby == nil) != (e.field.Ruby == nil) {
		r.insets = e.insets
		r.ruby.fresh = false
		r.invalidate()
	}
	if old.Obscure != e.field.Obscure {
		r.obscureChanged()
	}
	if old.MinLines != e.field.MinLines || old.MaxLines != e.field.MaxLines {
		r.MarkNeedsLayout()
	}
	if old.Disabled != e.field.Disabled || old.ReadOnly != e.field.ReadOnly {
		r.editableChanged()
	}
	if old.Style.PaintColor() != e.field.Style.PaintColor() || r.look != e.look {
		r.look = e.look
		r.MarkNeedsPaint()
	}
	if e.field.Value != r.value {
		r.value = e.field.Value
		if e.field.Value != r.Content {
			r.setText(e.field.Value)
			r.history.Clear()
			r.invalidate()
			// The parent made the change, and Build is no time for callbacks.
			r.reported = [2]int{r.Anchor, r.Caret}
			if c := r.field.Controller; c != nil {
				c.remember(r)
			}
		}
	}
}

// RenderEditable is the render object of Input. Offsets are bytes of text,
// which leaves out the composition, unless their name says display.
type RenderEditable struct {
	core.RenderBase
	shaper *text.Shaper
	field  Input
	look   fieldLook
	// insets is the room around the text, and onFocus reports the keyboard.
	insets  geom.Insets
	onFocus func(bool)
	// value is the last Value of the widget.
	value string

	Content string
	// Anchor is where the selection started, and Caret the end that moves,
	// as byte offsets into Content.
	Anchor, Caret int
	// reported is the selection OnSelectionChange last told of.
	reported [2]int
	// Composition is the text the IME is composing, or nil.
	Composition *input.Composition
	history     kit.History
	focused     bool
	dragging    bool
	// autofocused says that Input.Autofocus has had its turn.
	autofocused bool
	// goalX is the position along the line that moves between lines try to
	// keep: Up and Down, or Left and Right in vertical text.
	goalX  float32
	goalOK bool

	shaped  *text.Paragraph
	laidOut *text.Layout
	// stale means that the text changed since laidOut was made.
	stale       bool
	placeholder *text.Layout
	// length is the outer length of the lines the layouts were made for: the
	// width of the field, or its height for vertical text. size is the size
	// of the field.
	length float32
	size   geom.Size
	// scroll is how far a single line has moved back along itself to show the
	// caret, and scrollY how far the lines of a full box have moved back
	// across them: up, or right in vertical text.
	scroll, scrollY float32
	// view is the extent of the text area across the lines.
	view float32
	// imeRect is the last rectangle the host got.
	imeRect geom.Rect
	rects   []geom.Rect

	// ruby is the projection of Input.Ruby.
	ruby rubyView
	// obscure is the projection of an obscured field onto its bullets.
	obscure obscureView

	// blink sleeps between changes, so that a field with the focus costs two
	// frames a second and not a frame per vertical blank.
	blink *core.Ticker
	// caretHidden is the off phase, and blinkReset asks the next wake to start a
	// full on phase rather than to change anything.
	caretHidden, blinkReset bool
}

// Text returns the text of the field, for tests and tools.
func (r *RenderEditable) Text() string { return r.Content }

// Selection returns the ends of the selection, in order.
func (r *RenderEditable) Selection() (start, end int) {
	return min(r.Anchor, r.Caret), max(r.Anchor, r.Caret)
}

func (r *RenderEditable) setText(s string) {
	r.Content = r.sanitize(s)
	r.ruby.fresh = false
	r.Anchor = min(r.Anchor, len(r.Content))
	r.Caret = min(r.Caret, len(r.Content))
}

// sanitize keeps line breaks out of a single-line field.
func (r *RenderEditable) sanitize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if r.field.Multiline {
		return s
	}
	return strings.ReplaceAll(s, "\n", " ")
}

func (r *RenderEditable) invalidate() {
	// The old layout stays for Relayout to build on.
	r.stale, r.placeholder = true, nil
	r.MarkNeedsLayout()
}

// display returns what the field shows: the text with the composition in place
// of the selection.
func (r *RenderEditable) display() string {
	if r.Composition == nil {
		return r.Content
	}
	a, b := r.Selection()
	return r.Content[:a] + r.Composition.Text + r.Content[b:]
}

// displayCaret returns the caret as an offset into display.
func (r *RenderEditable) displayCaret() int {
	if r.Composition == nil {
		return r.Caret
	}
	a, _ := r.Selection()
	return a + r.Composition.Caret()
}

// Layout implements core.RenderObject. A horizontal field takes the width it is
// given and grows down with its lines; a vertical one takes the height and grows
// leftwards.
func (r *RenderEditable) Layout(c geom.Constraints) geom.Size {
	vertical := r.vertical()
	length := c.MaxW
	if vertical {
		length = c.MaxH
	}
	if length >= geom.Inf {
		length = fieldDefaultWidth
	}
	if r.laidOut == nil || r.stale || r.length != length {
		r.length = length
		r.shape()
	}
	p := r.insets
	cross := r.linesCross()
	sz := geom.Size{W: length, H: cross + p.Top + p.Bottom}
	if vertical {
		sz = geom.Size{W: cross + p.Left + p.Right, H: length}
	}
	sz = c.Constrain(sz)
	r.size = sz
	r.view = r.innerCross(sz)
	r.revealCaret()
	if c := r.field.Controller; c != nil && c.focus && c.view == r {
		// Focus asked before the field was laid out; now it can see.
		c.focus = false
		r.RequestFocus()
	}
	if r.field.Autofocus && !r.autofocused {
		r.autofocused = true
		r.RequestFocus()
	}
	return sz
}

// linesCross returns the extent across the lines that the field asks for: that
// of its text, kept between MinLines and MaxLines lines in a Multiline field.
// The lines of a field share one height, as its style sets it.
func (r *RenderEditable) linesCross() float32 {
	cross := r.textCross()
	if !r.field.Multiline || r.field.MinLines <= 0 && r.field.MaxLines <= 0 {
		return cross
	}
	n := r.laidOut.LineCount()
	if n <= 0 {
		return cross
	}
	line := cross / float32(n)
	if m := r.field.MinLines; m > 0 {
		cross = max(cross, float32(m)*line)
	}
	if m := r.field.MaxLines; m > 0 {
		cross = min(cross, float32(m)*line)
	}
	return cross
}

// shape brings the layouts up to date with the text. Besides Layout, the event
// handlers use it when several events arrive between two frames.
func (r *RenderEditable) shape() {
	wrap := geom.Inf
	if r.field.Multiline {
		wrap = r.innerLength(geom.Size{W: r.length, H: r.length})
	}
	// Only the lines around an edit are shaped again, so that a long text
	// stays quick to type in.
	r.stale = false
	switch {
	case r.field.Obscure:
		r.shaped = r.shaper.Reshape(r.shaped, r.obscure.update(r.shaper, r.display()), r.field.Style)
	case r.field.Ruby != nil:
		r.shaped = r.shaper.ReshapeSpans(r.shaped, r.project(), r.field.Style)
	default:
		r.shaped = r.shaper.Reshape(r.shaped, r.display(), r.field.Style)
	}
	r.laidOut = r.shaped.Relayout(r.laidOut, wrap)
	r.placeholder = nil
	if r.Content == "" && r.Composition == nil && r.field.Placeholder != "" {
		r.placeholder = r.shaper.Shape(r.field.Placeholder, r.field.Style).Layout(wrap)
	}
}

// revealCaret scrolls a single line so that the caret is in the field.
func (r *RenderEditable) revealCaret() {
	if r.laidOut == nil || r.stale {
		return
	}
	l := r.shown()
	if r.field.Multiline {
		start, end := r.crossRange(l.CaretRect(r.displayCaret()))
		switch {
		case end-r.scrollY > r.view:
			r.scrollY = end - r.view
		case start < r.scrollY:
			r.scrollY = start
		}
		r.clampScrollY()
		return
	}
	inner := r.innerLength(geom.Size{W: r.length, H: r.length})
	x := l.CaretPosition(r.displayCaret())
	switch {
	case x-r.scroll > inner-kit.CaretWidth:
		r.scroll = x - inner + kit.CaretWidth
	case x < r.scroll:
		r.scroll = x
	}
	r.scroll = max(min(r.scroll, r.textLength()-inner+kit.CaretWidth), 0)
}

func (r *RenderEditable) clampScrollY() {
	r.scrollY = max(min(r.scrollY, r.textCross()-r.view), 0)
}

// textOrigin returns the top left of the text inside the field. Vertical text
// keeps its first column at the right edge.
func (r *RenderEditable) textOrigin() geom.Offset {
	p := r.insets
	if r.vertical() {
		return geom.Offset{X: r.size.W - p.Right - r.laidOut.Size().W + r.scrollY, Y: p.Top - r.scroll}
	}
	x := p.Left - r.scroll
	if !r.field.Multiline && r.laidOut.Direction() == text.DirectionRTL {
		// A single right-to-left line starts at the right edge.
		x += max(r.innerLength(geom.Size{W: r.length})-r.textLength(), 0)
	}
	return geom.Offset{X: x, Y: p.Top - r.scrollY}
}

// Paint implements core.RenderObject.
func (r *RenderEditable) Paint(cv *paint.Canvas, off geom.Offset) {
	sz := r.Size()
	cv.Save()
	if r.field.Bare {
		cv.ClipRect(geom.Rect{X: off.X, Y: off.Y, W: sz.W, H: sz.H})
	} else {
		// Inside the border.
		cv.ClipRect(geom.Rect{X: off.X + 2, Y: off.Y + 2, W: max(sz.W-4, 0), H: max(sz.H-4, 0)})
	}
	origin := off.Add(r.textOrigin())
	if r.Composition == nil {
		if a, b := r.Selection(); a != b {
			r.fillRange(cv, origin, a, b, r.look.selection)
		}
	} else {
		r.paintComposition(cv, origin)
	}
	color := r.field.Style.PaintColor()
	shown := r.laidOut
	if r.placeholder != nil {
		shown, color = r.placeholder, r.look.placeholder
	}
	runs := shown.Runs()
	for i := range runs {
		cv.DrawGlyphs(&runs[i], origin, color)
	}
	if r.focused && !r.caretHidden {
		c := r.caretBar(r.shown().CaretRect(r.displayCaret()))
		cv.DrawRRect(geom.RRectAll(c.Translate(origin), 0), paint.Paint{Color: r.look.caret})
	}
	cv.Restore()
	r.tellIME()
}

// fillRange fills the display range from a to b, line by line.
func (r *RenderEditable) fillRange(cv *paint.Canvas, origin geom.Offset, a, b int, color geom.Color) {
	r.rects = r.shown().SelectionRects(a, b, r.rects[:0])
	for _, rect := range r.rects {
		cv.DrawRRect(geom.RRectAll(rect.Translate(origin), 0), paint.Paint{Color: color})
	}
}

// paintComposition marks the clauses. The clause being converted gets a
// background and a thick underline, the others a thin one.
func (r *RenderEditable) paintComposition(cv *paint.Canvas, origin geom.Offset) {
	base, _ := r.Selection()
	color := r.field.Style.PaintColor()
	for _, cl := range r.Composition.Clauses {
		a, b := base+cl.Start, base+cl.End
		thickness := float32(1)
		if cl.Kind == input.ClauseTarget {
			thickness = 2
			r.fillRange(cv, origin, a, b, r.look.target)
		}
		r.rects = r.shown().SelectionRects(a, b, r.rects[:0])
		for _, rect := range r.rects {
			line := r.underline(rect, thickness)
			cv.DrawRRect(geom.RRectAll(line.Translate(origin), 0), paint.Paint{Color: color})
		}
	}
}

// tellIME moves the candidate list to the clause being converted, or else the
// caret. Paint is the first moment the position in the window is known.
func (r *RenderEditable) tellIME() {
	if !r.focused {
		return
	}
	at := r.displayCaret()
	if r.Composition != nil {
		base, _ := r.Selection()
		for _, cl := range r.Composition.Clauses {
			if cl.Kind == input.ClauseTarget {
				at = base + cl.Start
				break
			}
		}
	}
	// Under a Transform, the IME gets the upright box around the turned caret.
	rect := core.LocalRectToGlobal(r, r.shown().CaretRect(at).Translate(r.textOrigin()))
	if rect != r.imeRect {
		r.imeRect = rect
		if h, ok := r.Host().(core.VerticalIMEHost); ok {
			h.SetIMEVertical(r.vertical())
		}
		r.Host().SetIMECursorRect(rect)
	}
}

// showCaret puts the caret on for a full phase. Any edit or move of the caret
// does this, because a caret that vanishes while typing is lost.
func (r *RenderEditable) showCaret() {
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

func (r *RenderEditable) tickCaret(now time.Duration) {
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

// MouseCursor implements core.CursorSource.
func (r *RenderEditable) MouseCursor() input.Cursor {
	if r.field.Disabled {
		return input.CursorArrow
	}
	return input.CursorIBeam
}

// CanFocus implements core.FocusTarget.
func (r *RenderEditable) CanFocus() bool { return !r.field.Disabled }

// editable reports whether the user may change the text.
func (r *RenderEditable) editable() bool { return !r.field.Disabled && !r.field.ReadOnly }

// editableChanged follows Disabled or ReadOnly turning on or off. It runs
// during Build, so it reports nothing: what an IME was converting is dropped
// rather than committed. A field that is now disabled no longer takes the
// keyboard, and the tree takes it away after the frame.
func (r *RenderEditable) editableChanged() {
	if !r.editable() && r.Composition != nil {
		r.Composition = nil
		r.Host().CancelComposition()
		r.invalidate()
	}
	r.dragging = r.dragging && !r.field.Disabled
	r.MarkNeedsPaint()
	if r.focused && !r.field.Disabled {
		r.Host().SetIMEEnabled(!r.field.Obscure && r.editable())
	}
}

// FocusChanged implements core.FocusTarget.
func (r *RenderEditable) FocusChanged(focused bool) {
	if r.focused == focused {
		return
	}
	if !focused {
		r.commitComposition()
	}
	r.focused = focused
	r.imeRect = geom.Rect{}
	r.focusIME(focused)
	r.showCaret()
	r.MarkNeedsPaint()
	if r.onFocus != nil {
		r.onFocus(focused)
	}
}

// commitComposition keeps what the IME was converting. Asking the IME to commit
// would deliver the text after the focus has gone.
func (r *RenderEditable) commitComposition() {
	if r.Composition == nil {
		return
	}
	s := r.Composition.Text
	r.Composition = nil
	r.Host().CancelComposition()
	r.insert(s, kit.EditOther)
}

// HandleText implements core.FocusTarget.
func (r *RenderEditable) HandleText(s string) {
	if !r.editable() {
		return
	}
	r.Composition = nil
	kind := kit.EditOther
	if utf8.RuneCountInString(s) == 1 {
		kind = kit.EditTyping
	}
	r.insert(s, kind)
}

// HandleComposition implements core.FocusTarget.
func (r *RenderEditable) HandleComposition(c *input.Composition) {
	if c == nil && r.Composition == nil || c != nil && !r.editable() {
		return
	}
	r.Composition = c
	r.invalidate()
	r.showCaret()
}

// insert replaces the selection with s.
func (r *RenderEditable) insert(s string, kind kit.EditKind) {
	s = r.sanitize(s)
	a, b := r.Selection()
	r.replace(a, b, s, kind)
}

// replace is the one place where the user changes the text.
func (r *RenderEditable) replace(a, b int, s string, kind kit.EditKind) {
	if a == b && s == "" {
		return
	}
	// An obscured field keeps no history, which would hold the parts of the
	// password that were deleted.
	if !r.field.Obscure {
		r.history.Record(kit.Edit{
			Kind: kind, At: a, Removed: r.Content[a:b], Inserted: s, Anchor: r.Anchor, Caret: r.Caret,
		})
	}
	r.splice(a, b, s)
	r.Anchor = a + len(s)
	r.Caret = r.Anchor
	r.changed()
}

// splice changes the text without touching the history or the selection.
func (r *RenderEditable) splice(a, b int, s string) {
	r.Content = r.Content[:a] + s + r.Content[b:]
	r.spliceMarks(a, b-a, len(s))
}

// changed reports an edit that is complete, selection included.
func (r *RenderEditable) changed() {
	r.goalOK = false
	r.invalidate()
	r.showCaret()
	if c := r.field.Controller; c != nil {
		c.remember(r)
	}
	if r.field.OnChange != nil {
		r.field.OnChange(r.Content)
	}
	r.selectionMoved()
}

// selectionMoved runs OnSelectionChange if the selection is not the one it
// last told of.
func (r *RenderEditable) selectionMoved() {
	sel := [2]int{r.Anchor, r.Caret}
	if sel == r.reported {
		return
	}
	r.reported = sel
	if c := r.field.Controller; c != nil {
		c.remember(r)
	}
	if f := r.field.OnSelectionChange; f != nil {
		f(r.Anchor, r.Caret)
	}
}

// undo takes back the last step and restores the selection before it.
func (r *RenderEditable) undo() {
	e, ok := r.history.TakeUndo()
	if !ok {
		return
	}
	r.splice(e.At, e.At+len(e.Inserted), e.Removed)
	r.Anchor, r.Caret = e.Anchor, e.Caret
	r.changed()
}

// redo repeats the last step that undo took back.
func (r *RenderEditable) redo() {
	e, ok := r.history.TakeRedo()
	if !ok {
		return
	}
	r.splice(e.At, e.At+len(e.Removed), e.Inserted)
	r.Anchor = e.At + len(e.Inserted)
	r.Caret = r.Anchor
	r.changed()
}

// MoveTo places the caret, extending the selection if extend is set.
func (r *RenderEditable) MoveTo(offset int, extend bool) {
	r.history.Seal()
	r.Caret = offset
	if !extend {
		r.Anchor = offset
	}
	r.moveRuby()
	r.revealCaret()
	r.MarkNeedsPaint()
	r.selectionMoved()
}

// HandlePointer implements core.PointerTarget.
func (r *RenderEditable) HandlePointer(e *core.PointerEvent) {
	if r.laidOut == nil {
		return
	}
	if r.stale {
		r.shape()
	}
	if e.Kind == core.PointerScroll {
		// The wheel moves across the lines: down the page, or on to the
		// columns to the left.
		if r.textCross() > r.view {
			r.scrollY -= e.Scroll.Y * fieldWheelDistance
			r.clampScrollY()
			r.MarkNeedsPaint()
			e.Handled = true
		}
		return
	}
	if e.Button != input.ButtonPrimary || r.field.Disabled {
		return
	}
	switch e.Kind {
	case core.PointerDown:
		e.Handled = true
		r.commitComposition()
		r.RequestFocus()
		if r.stale {
			r.shape()
		}
		r.press(e)
	case core.PointerMove:
		if r.dragging {
			r.MoveTo(r.offsetAt(e.Pos), true)
			r.goalOK = false
		}
	case core.PointerUp, core.PointerCancel:
		r.dragging = false
	}
}

func (r *RenderEditable) press(e *core.PointerEvent) {
	r.goalOK = false
	offset := r.offsetAt(e.Pos)
	switch {
	case e.Clicks == 2:
		a, b := r.shown().WordRange(offset)
		r.Anchor = a
		r.MoveTo(b, true)
	case e.Clicks >= 3:
		r.Anchor = 0
		r.MoveTo(len(r.Content), true)
	default:
		r.MoveTo(offset, e.Modifiers&input.ModShift != 0)
		r.dragging = true
	}
}

func (r *RenderEditable) offsetAt(pos geom.Offset) int {
	return r.shown().HitTest(pos.Sub(r.textOrigin()))
}

// HandleKey implements core.FocusTarget.
func (r *RenderEditable) HandleKey(e *core.KeyEvent) bool {
	if r.laidOut == nil || r.stale {
		r.shape()
	}
	if f := r.field.Extras.onKey; f != nil && f(e) {
		return true
	}
	if kit.ShortcutEnter(e, r.Composition != nil) {
		return false
	}
	control := e.Modifiers&input.ModControl != 0
	shift := e.Modifiers&input.ModShift != 0
	if control && r.shortcut(e.Key, shift) {
		return true
	}
	if r.move(e.Key, control, shift) {
		return true
	}
	if r.edit(e.Key, control, shift) {
		return true
	}
	// The character of a typing key arrives as text of its own; the press is
	// the field's, so that a bare-key shortcut does not run as well.
	return kit.TypingKey(e, r.Composition != nil)
}

// shortcut runs the Ctrl combinations of the clipboard, selection and history.
func (r *RenderEditable) shortcut(key input.KeyCode, shift bool) bool {
	a, b := r.Selection()
	switch key {
	case input.KeyA:
		r.Anchor = 0
		r.MoveTo(len(r.Content), true)
	case input.KeyC:
		// An obscured field takes the keys and copies nothing.
		if a != b && !r.field.Obscure {
			r.Host().SetClipboardText(r.Content[a:b])
		}
	case input.KeyX:
		// A read-only field takes the keys and changes nothing.
		if a != b && !r.field.Obscure && r.editable() {
			r.Host().SetClipboardText(r.Content[a:b])
			r.replace(a, b, "", kit.EditOther)
		}
	case input.KeyV:
		if r.editable() {
			r.insert(r.Host().ClipboardText(), kit.EditOther)
		}
	case input.KeyZ:
		switch {
		case !r.editable():
		case shift:
			r.redo()
		default:
			r.undo()
		}
	case input.KeyY:
		if r.editable() {
			r.redo()
		}
	default:
		return false
	}
	return true
}

// move handles the keys that move the caret.
func (r *RenderEditable) move(key input.KeyCode, control, shift bool) bool {
	l := r.shown()
	a, b := r.Selection()
	key = r.logicalKey(key)
	vertical := false
	var target int
	switch key {
	case input.KeyLeft:
		switch {
		case control:
			target = l.PrevWord(r.Caret)
		case a != b && !shift:
			target = a
		default:
			// Visual order: Left goes left on screen, in Arabic too.
			target = l.VisualMove(r.Caret, -1)
			if !shift {
				target = r.enterMark(r.Caret, target)
			}
		}
	case input.KeyRight:
		switch {
		case control:
			target = l.NextWord(r.Caret)
		case a != b && !shift:
			target = b
		default:
			target = l.VisualMove(r.Caret, 1)
			if !shift {
				target = r.enterMark(r.Caret, target)
			}
		}
	case input.KeyHome:
		target = l.LineStart(r.Caret)
		if control {
			target = 0
		}
	case input.KeyEnd:
		target = l.LineEnd(r.Caret)
		if control {
			target = len(r.Content)
		}
	case input.KeyUp, input.KeyDown:
		if !r.field.Multiline {
			return false
		}
		vertical = true
		if !r.goalOK {
			r.goalX, r.goalOK = l.CaretPosition(r.Caret), true
		}
		rows := 1
		if key == input.KeyUp {
			rows = -1
		}
		target = l.VerticalMove(r.Caret, rows, r.goalX)
	default:
		return false
	}
	if !vertical {
		r.goalOK = false
	}
	r.MoveTo(target, shift)
	return true
}

// edit handles the keys that change the text.
func (r *RenderEditable) edit(key input.KeyCode, control, shift bool) bool {
	a, b := r.Selection()
	l := r.shown()
	if !r.editable() {
		// A read-only field takes the keys of editing and changes nothing;
		// Enter still submits.
		switch key {
		case input.KeyBackspace, input.KeyDelete:
			return true
		case input.KeyEnter:
			if r.field.OnSubmit != nil && (!r.field.Multiline || r.field.SubmitOnEnter && !shift) {
				r.field.OnSubmit(r.Content)
			}
			return true
		}
		return false
	}
	switch key {
	case input.KeyBackspace:
		switch {
		case a != b:
			r.replace(a, b, "", kit.EditOther)
		case control:
			r.replace(l.PrevWord(a), a, "", kit.EditOther)
		default:
			// Right after hidden ruby, the closing 》 goes and the notation
			// shows, rather than the whole base.
			r.replace(r.enterMark(a, l.PrevCaret(a)), a, "", kit.EditBackspace)
		}
	case input.KeyDelete:
		switch {
		case a != b:
			r.replace(a, b, "", kit.EditOther)
		case control:
			r.replace(a, l.NextWord(a), "", kit.EditOther)
		default:
			r.replace(a, r.enterMark(a, l.NextCaret(a)), "", kit.EditForward)
		}
	case input.KeyEnter:
		switch {
		case r.Composition != nil:
			// The platforms keep the keys an IME works on, but should one
			// arrive, it confirms the conversion, which the IME commits.
		case r.field.Multiline && (!r.field.SubmitOnEnter || shift):
			r.insert("\n", kit.EditTyping)
		case r.field.OnSubmit != nil:
			r.field.OnSubmit(r.Content)
		}
	default:
		return false
	}
	return true
}
