package field

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/behavior"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

const (
	// dropdownGlyphGap is the room between the label and the chevron.
	dropdownGlyphGap = theme.Space8
	// dropdownPaddingRight is the room right of the chevron, which has room
	// of its own inside its square.
	dropdownPaddingRight = theme.Space6
)

// DropdownOption is one choice of a DropdownMenu.
type DropdownOption[T comparable] struct {
	Value T
	Label string
	// Disabled shows the option grayed out and keeps it from being chosen. It
	// breaks the rule of positive names (CODING_STYLE §4.1) so that the zero
	// value is an option that works.
	Disabled bool
}

// DropdownMenu chooses one of Options, as a select does: a field that shows the
// current choice with a chevron, and opens the list of choices below itself,
// or above where there is no room. Without OnChange it is disabled.
//
// The list opens with a click, Space, Enter, Alt+Down or F4, with the current
// choice highlighted and in view. In the list, the arrows move, letters jump to
// the choices that start with them, Enter or a click chooses, and Escape closes.
// On the closed field, the arrows and letters change the choice at once, as in
// a Windows combo box.
//
// Value is the current choice, which the caller keeps. A Value that matches no
// option shows Placeholder. Label, if set, names the field above it and to a
// screen reader; HideLabel keeps only the name for the screen reader, for a
// field in a row that already says what it is.
//
// Like Input and DatePicker, the field takes all the width its parent gives
// it, so that the fields of a form line up; in a Row, which leaves the width
// open, it is as wide as its longest option.
type DropdownMenu[T comparable] struct {
	core.StatefulBase
	Value       T
	Options     []DropdownOption[T]
	OnChange    func(T)
	Placeholder string
	Label       string
	// HideLabel draws no caption for Label, which still names the field to a
	// screen reader.
	HideLabel bool
}

type dropdownState[T comparable] struct {
	core.StateOf[DropdownMenu[T]]
	behavior.Control
	chevron *paint.Path
	ahead   kit.TypeAhead
	scope   fieldScope
	// box is the context of the field without its label, which the list
	// opens from.
	box core.BuildContext

	// entry is the open list, or nil, with its layer, its rows, the row that
	// takes the focus first, the highlighted row, where it opens, how wide it
	// is and how many rows it shows.
	entry     *overlay.Entry
	layer     *kit.PopupLayerState
	refs      []*layout.RenderFocus
	first     int
	highlight int
	anchor    geom.Rect
	width     float32
	rows      int
}

// CreateState implements core.StatefulWidget.
func (DropdownMenu[T]) CreateState() core.State { return &dropdownState[T]{} }

// InitState implements core.State.
func (s *dropdownState[T]) InitState() {
	s.Attach(&s.StateBase)
	s.ahead.Attach(&s.StateBase)
	s.chevron = kit.ChevronGlyph()
}

// DidUpdateWidget implements core.State.
func (s *dropdownState[T]) DidUpdateWidget(core.Widget) {
	w := s.Widget()
	switch {
	case s.entry == nil:
	case w.OnChange == nil || len(w.Options) != len(s.refs):
		s.close(true)
	default:
		s.refresh()
	}
}

// Dispose implements core.State.
func (s *dropdownState[T]) Dispose() {
	s.Control.Dispose()
	if s.entry != nil {
		s.entry.Remove()
		s.entry = nil
	}
}

// index returns the option of Value, or -1.
func (s *dropdownState[T]) index() int {
	w := s.Widget()
	for i := range w.Options {
		if w.Options[i].Value == w.Value {
			return i
		}
	}
	return -1
}

func (s *dropdownState[T]) labels() []string {
	opts := s.Widget().Options
	out := make([]string, len(opts))
	for i := range opts {
		out[i] = opts[i].Label
	}
	return out
}

func (s *dropdownState[T]) enabled(i int) bool { return !s.Widget().Options[i].Disabled }

// step returns the enabled option dir steps from i, stopping at the ends, or i
// if there is none.
func (s *dropdownState[T]) step(i, dir int) int {
	n := len(s.Widget().Options)
	for j := i + dir; j >= 0 && j < n; j += dir {
		if s.enabled(j) {
			return j
		}
	}
	return i
}

// end returns the first enabled option from the start, or from the end for a
// dir of -1, or -1 if there is none.
func (s *dropdownState[T]) end(dir int) int {
	n := len(s.Widget().Options)
	if dir > 0 {
		return s.step(-1, 1)
	}
	if j := s.step(n, -1); j < n {
		return j
	}
	return -1
}

// choose makes option i the value.
func (s *dropdownState[T]) choose(i int) {
	w := s.Widget()
	if i < 0 || i >= len(w.Options) || !s.enabled(i) || w.OnChange == nil {
		return
	}
	if v := w.Options[i].Value; v != w.Value {
		w.OnChange(v)
	}
}

// Build implements core.State.
func (s *dropdownState[T]) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	th := basic.ThemeOf(ctx)
	c := &th.Color
	s.scope = fieldScopeOf(ctx)
	kit.ReportField(ctx, kit.FieldChoice)
	enabled := w.OnChange != nil
	open := s.entry != nil
	shown, ink := w.Placeholder, c.Ink3
	var value string
	if i := s.index(); i >= 0 {
		value = w.Options[i].Label
		shown, ink = value, c.Ink
	}
	shaper, _ := core.DependOn[*text.Shaper](ctx)
	sink := geom.Clamp32(s.Sink.Value(), 0, 1)
	ground := theme.Over(c.Field, c.Ink, (kit.PressStrength-kit.HoverStrength)*sink)
	border := motion.LerpColor(c.Hair, c.Ink3, s.Hot.Get())
	glyph := c.Ink2
	ring := c.AccentInk
	switch {
	case !enabled:
		ground, border, ink, glyph = c.Fill, geom.Color{}, c.Ink3, c.Ink3
	case s.scope.err != "":
		border, ring = c.Danger, c.Danger
	case open:
		border = c.AccentInk
	}
	style := kit.Colored(th.Text.Control, ink)
	box := s.Ring(th.Radius.Control, ring, basic.Container{
		Height:  fieldHeight,
		Color:   ground,
		Radius:  th.Radius.Control,
		Border:  paint.Stroke{Width: 1, Color: border},
		Sink:    kit.ButtonSink * sink,
		Padding: geom.Insets{Left: fieldPaddingX, Right: dropdownPaddingRight},
		Child: basic.Row{Cross: layout.CrossCenter, Spacing: dropdownGlyphGap, Children: []core.Widget{
			basic.Expanded{Child: s.shown(shaper, shown, value != "", style)},
			basic.Icon{Path: s.chevron, Size: kit.GlyphSize, Color: glyph},
		}},
	})
	// The placeholder names the field or hints at it, as that of Input does.
	sem := interact.Semantics{Role: semantics.RoleComboBox, Label: w.Label, Value: value, Hint: s.scope.err, State: s.scope.state()}
	switch {
	case w.Label == "":
		sem.Label = w.Placeholder
	case sem.Hint == "":
		sem.Hint = w.Placeholder
	}
	if open {
		sem.State |= semantics.Expanded
	} else {
		sem.State |= semantics.Collapsed
	}
	var body core.Widget
	if enabled {
		sem.OnTap, sem.OnExpand, sem.OnCollapse = s.toggle, s.open, func() { s.close(true) }
		body = basic.Builder{Content: func(bctx core.BuildContext) core.Widget {
			s.box = bctx
			return s.focusable(box)
		}}
	} else {
		sem.State |= semantics.Disabled
		body = box
	}
	sem.Child = body
	// The field takes the width it is given, as Input does, and is as wide as
	// its longest label where the width is open.
	width := s.naturalWidth(shaper, style)
	if w.Label == "" || w.HideLabel {
		return fieldWidth{width: width, child: sem}
	}
	return fieldWidth{width: width, child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: fieldLabelGap, Children: []core.Widget{
		basic.Text{Content: w.Label, Style: kit.Colored(th.Text.Caption, c.Ink2)},
		sem,
	}}}
}

// shown is the text in the field. The placeholder is drawn as Input and
// DatePicker draw theirs: as a face that says nothing to assistive technology
// itself, since the semantics of the field carry it, and that the contrast
// warnings of gowakutest treat as the look of an empty field, not as text.
func (s *dropdownState[T]) shown(shaper *text.Shaper, content string, chosen bool, style text.Style) core.Widget {
	if chosen || shaper == nil {
		return basic.Text{Content: content, Style: style}
	}
	return face{shaper: shaper, content: content, style: style, start: true}
}

// naturalWidth returns the width that fits the longest label.
func (s *dropdownState[T]) naturalWidth(shaper *text.Shaper, style text.Style) float32 {
	if shaper == nil {
		return fieldDefaultWidth
	}
	w := s.Widget()
	widest := kit.TextWidth(shaper, w.Placeholder, style)
	for i := range w.Options {
		widest = max(widest, kit.TextWidth(shaper, w.Options[i].Label, style))
	}
	return geom.Ceil32(fieldPaddingX + widest + dropdownGlyphGap + kit.GlyphSize + dropdownPaddingRight)
}

// focusable puts the field inside the focus, hover and press handling of a
// control, with the keys of a closed dropdown.
func (s *dropdownState[T]) focusable(child core.Widget) core.Widget {
	return interact.Focus{
		Enabled:       true,
		OnKey:         s.fieldKey,
		OnFocusChange: s.fieldFocus,
		OnShowFocus:   s.ShowFocus,
		Child: interact.MouseRegion{
			OnEnter: func() { s.Hover(true) },
			OnExit:  func() { s.Hover(false) },
			Child: interact.GestureDetector{
				OnTapDown:   s.Press,
				OnTapCancel: s.Release,
				OnTap: func() {
					s.Release()
					s.toggle()
				},
				Child: child,
			},
		},
	}
}

// fieldFocus follows the keyboard of the field. The list taking it is not a
// blur for the FormItem around.
func (s *dropdownState[T]) fieldFocus(focused bool) {
	s.Focus(focused)
	if focused || s.entry == nil {
		s.scope.focusChanged(focused)
	}
}

// fieldKey handles the keys of the closed field.
func (s *dropdownState[T]) fieldKey(e *core.KeyEvent) bool {
	alt := e.Modifiers == input.ModAlt
	plain := e.Modifiers&^input.ModShift == 0
	cur := s.index()
	switch {
	case kit.Activates(e) && plain, e.Key == input.KeyF4 && e.Modifiers == 0,
		alt && (e.Key == input.KeyDown || e.Key == input.KeyUp):
		if !e.Repeat {
			s.Sink.Kick(kit.KeyKick)
			s.toggle()
		}
	case e.Key == input.KeyDown && plain:
		if cur < 0 {
			s.choose(s.end(1))
		} else {
			s.choose(s.step(cur, 1))
		}
	case e.Key == input.KeyUp && plain:
		if cur < 0 {
			s.choose(s.end(1))
		} else {
			s.choose(s.step(cur, -1))
		}
	case e.Key == input.KeyHome && plain:
		s.choose(s.end(1))
	case e.Key == input.KeyEnd && plain:
		s.choose(s.end(-1))
	default:
		ch := kit.LetterOf(e)
		if ch == 0 {
			return false
		}
		s.choose(s.ahead.Find(ch, s.labels(), s.enabled, cur))
	}
	return true
}

func (s *dropdownState[T]) toggle() {
	if s.entry != nil {
		s.close(true)
	} else {
		s.open()
	}
}

// open shows the list below the field.
func (s *dropdownState[T]) open() {
	w := s.Widget()
	if s.entry != nil || w.OnChange == nil || len(w.Options) == 0 || !s.Mounted() {
		return
	}
	ro := s.box.RenderObject()
	if ro == nil {
		return
	}
	ctx := s.Context()
	// Run always gives an Overlay; a bare tree, as in a test, has none.
	o, ok := core.Lookup[*overlay.Host](ctx)
	if !ok {
		return
	}
	r := kit.GlobalRect(ro)
	s.anchor = kit.OverlayRect(o.Context().RenderObject(), r)
	s.width = overlay.PopupWidth(ctx, s.labels(), true, r.W)
	s.rows = kit.PopupRows(o.Context().RenderObject(), s.anchor)
	s.highlight = s.index()
	if s.highlight < 0 || !s.enabled(s.highlight) {
		s.highlight = s.end(1)
	}
	s.first = s.highlight
	s.refs = make([]*layout.RenderFocus, len(w.Options))
	s.entry = o.InsertFrom(ctx, func() core.Widget { return kit.PopupLayer{Build: s.list, State: &s.layer} })
	kit.Rebuild(&s.StateBase)
}

// close takes the list away. restore gives the keyboard back to the field.
func (s *dropdownState[T]) close(restore bool) {
	if s.entry == nil {
		return
	}
	s.entry.Remove()
	s.entry, s.layer = nil, nil
	if field := kit.FirstFocusable(s.box.RenderObject()); restore && field != nil {
		core.RestoreFocus(field)
	} else {
		s.scope.focusChanged(false)
	}
	kit.Rebuild(&s.StateBase)
}

// refresh rebuilds the open list.
func (s *dropdownState[T]) refresh() {
	if s.layer != nil && s.layer.Mounted() {
		kit.Rebuild(&s.layer.StateBase)
	}
}

// list builds the open list over the window.
func (s *dropdownState[T]) list() core.Widget {
	w := s.Widget()
	disabled := make([]bool, len(w.Options))
	for i := range w.Options {
		disabled[i] = w.Options[i].Disabled
	}
	return kit.ModalScope{Active: true, Child: basic.Stack{Fit: layout.StackExpand, Children: []core.Widget{
		kit.Barrier{OnPress: func() { s.close(true) }},
		kit.Popup{Anchor: s.anchor, Side: layout.PopupBelow, Child: overlay.PopupList{
			Label: w.Label, Labels: s.labels(), Disabled: disabled,
			Hot: s.highlight, Checked: s.index(), Checks: true, Width: s.width, Rows: s.rows,
			Refs: s.refs, Autofocus: s.first,
			OnHover:    s.focusRow,
			OnTap:      s.pick,
			OnKey:      s.listKey,
			OnRowFocus: s.rowFocus,
		}},
	}}}
}

// pick chooses row i and closes the list.
func (s *dropdownState[T]) pick(i int) {
	if !s.enabled(i) {
		return
	}
	s.close(true)
	s.choose(i)
}

// focusRow gives the keyboard to row i.
func (s *dropdownState[T]) focusRow(i int) {
	if s.entry == nil || i < 0 || i >= len(s.refs) || !s.enabled(i) {
		return
	}
	if r := s.refs[i]; r != nil && r.Attached() {
		r.RequestFocus()
	}
}

// rowFocus follows the keyboard through the rows, and closes the list when it
// leaves them.
func (s *dropdownState[T]) rowFocus(i int, focused bool) {
	if s.entry == nil {
		return
	}
	if focused {
		if s.highlight != i {
			s.highlight = i
			s.refresh()
		}
		return
	}
	row := s.refs[i]
	now := kit.FocusFrom(row)
	switch {
	case now == core.RenderObject(row):
		// The window lost the keyboard.
		s.close(true)
	case !s.isRow(now):
		s.close(false)
	}
}

func (s *dropdownState[T]) isRow(ro core.RenderObject) bool {
	for _, r := range s.refs {
		if r != nil && core.RenderObject(r) == ro {
			return true
		}
	}
	return false
}

// listKey handles a key while a row has the focus. Every key stops here, as
// in a native list.
func (s *dropdownState[T]) listKey(e *core.KeyEvent) bool {
	plain := e.Modifiers&^input.ModShift == 0
	switch {
	case e.Key == input.KeyEscape, e.Key == input.KeyF4, e.Key == input.KeyTab,
		e.Modifiers == input.ModAlt && (e.Key == input.KeyUp || e.Key == input.KeyDown):
		s.close(true)
	case kit.Activates(e):
		if !e.Repeat && s.highlight >= 0 {
			s.pick(s.highlight)
		}
	case e.Key == input.KeyDown:
		s.focusRow(s.step(s.highlight, 1))
	case e.Key == input.KeyUp:
		s.focusRow(s.step(s.highlight, -1))
	case e.Key == input.KeyHome:
		s.focusRow(s.end(1))
	case e.Key == input.KeyEnd:
		s.focusRow(s.end(-1))
	case e.Key == input.KeyPageDown:
		s.focusRow(s.jump(s.highlight, kit.PopupMaxRows-1))
	case e.Key == input.KeyPageUp:
		s.focusRow(s.jump(s.highlight, -(kit.PopupMaxRows - 1)))
	case plain:
		if ch := kit.LetterOf(e); ch != 0 {
			if i := s.ahead.Find(ch, s.labels(), s.enabled, s.highlight); i >= 0 {
				s.focusRow(i)
			}
		}
	}
	return true
}

// jump returns the enabled option nearest to i+by within the list.
func (s *dropdownState[T]) jump(i, by int) int {
	n := len(s.Widget().Options)
	j := min(max(i+by, 0), n-1)
	if s.enabled(j) {
		return j
	}
	if by > 0 {
		if k := s.step(j, -1); k != j {
			return k
		}
		return s.step(j, 1)
	}
	if k := s.step(j, 1); k != j {
		return k
	}
	return s.step(j, -1)
}
