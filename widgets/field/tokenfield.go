package field

import (
	"slices"
	"strings"
	"unicode"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

const (
	// tokenGap is the room between the chips, and between their lines.
	tokenGap = theme.Space4
	// tokenPadding is the room between the border and the chips.
	tokenPadding = theme.Space4
	// tokenInputMin is the least width of the text after the chips, and
	// tokenInputSlack the room it keeps after its text for the caret.
	tokenInputMin   = 48
	tokenInputSlack = theme.Space16
	// tokenChipHeight is the height of a control.Chip, whose pill the ring of
	// a selected token follows.
	tokenChipHeight = 24
)

// tokenSeparators are the characters that end a token without
// TokenField.Separators: the comma, its full-width form and the Japanese
// comma 、, which a Japanese IME types for the comma key.
const tokenSeparators = ",，、"

// TokenField edits a list of short texts, such as tags or addresses, each of
// which shows as a control.Chip, with the text being typed after them. The
// chips wrap onto as many lines as they need.
//
// Enter, or one of Separators, makes the typed text a token. Enter while an
// IME converts text belongs to the IME, which commits the text; it does not make
// a token. Pasted text with separators or line breaks in it becomes several
// tokens at once, and the text after the last separator stays to be typed on.
// Leaving the field makes a token of what was typed.
//
// Backspace at the start of the text selects the last token, and a second
// Backspace removes it, as in a mail client. Left at the start of the text
// selects the last token too; Left and Right then move through the tokens,
// and Right past the last one returns to the text. Backspace or Delete removes
// the selected token, and typing or Escape leaves it. The × of each chip
// removes it with the pointer. The keyboard stays in the text throughout.
//
// The parent keeps Tokens; OnChange asks for a new list, which is always a
// new slice. Without OnChange the field shows the tokens and is disabled.
// Assistive technology sees a list named Label, with an item for each token,
// followed by the text field. Label also shows above the field, unless
// HideLabel is set.
type TokenField struct {
	core.StatefulBase
	Tokens []string
	Label  string
	// HideLabel draws no caption for Label, which still names the field to a
	// screen reader.
	HideLabel bool
	// Placeholder shows while there is neither a token nor text.
	Placeholder string
	// Separators are the characters that end a token as they are typed or
	// pasted, besides Enter and line breaks. Empty selects the comma, its
	// full-width form and the Japanese comma 、.
	Separators string
	// Validate, if set, checks a token before it is added, and returns why it
	// is wrong, or "" when it is right. A wrong token stays in the text, and
	// the reason shows below the field.
	Validate func(token string) string
	// AllowDuplicates adds a token that is already in the list. Without it a
	// token that is already there is dropped, and its text goes.
	AllowDuplicates bool
	// MaxTokens, above zero, is the most tokens the field takes; text typed
	// after that stays in the field.
	MaxTokens int
	// ReadOnly shows the tokens and keeps the user from changing them: the
	// chips have no ×, and there is no text to type in. Without OnChange the
	// field is disabled instead.
	ReadOnly bool
	OnChange func(tokens []string)
}

func (w TokenField) separators() string {
	if w.Separators != "" {
		return w.Separators
	}
	return tokenSeparators
}

// isSeparator reports whether r ends a token.
func (w TokenField) isSeparator(r rune) bool {
	return r == '\n' || r == '\r' || strings.ContainsRune(w.separators(), r)
}

type tokenFieldState struct {
	core.StateOf[TokenField]
	ctrl InputController
	// text is what is being typed. selected is the selected token, or -1,
	// and selectedText its text, which finds it again in a new list.
	text         string
	selected     int
	selectedText string
	// err is why the last token was turned down.
	err              string
	focused, hovered bool
	// syncing is set while the field takes text from here rather than from
	// typing.
	syncing bool
}

// CreateState implements core.StatefulWidget.
func (TokenField) CreateState() core.State { return &tokenFieldState{selected: -1} }

// DidUpdateWidget implements core.State. The selection follows its token into
// the new list, or goes when the token has gone.
func (s *tokenFieldState) DidUpdateWidget(core.Widget) {
	if s.selected < 0 {
		return
	}
	tokens := s.Widget().Tokens
	if s.selected < len(tokens) && tokens[s.selected] == s.selectedText {
		return
	}
	s.selected = slices.Index(tokens, s.selectedText)
}

// composing reports whether an IME is converting text in the field.
func (s *tokenFieldState) composing() bool {
	r := s.ctrl.field()
	return r != nil && r.Composition != nil
}

func (s *tokenFieldState) selectToken(i int) {
	tokens := s.Widget().Tokens
	if i < 0 || i >= len(tokens) {
		i = -1
	}
	s.selected = i
	s.selectedText = ""
	if i >= 0 {
		s.selectedText = tokens[i]
	}
	kit.Rebuild(&s.StateBase)
}

// setText puts text in the field, as the remainder after tokens were taken.
func (s *tokenFieldState) setText(t string) {
	s.text = t
	if r := s.ctrl.field(); r != nil {
		if r.Content != t {
			s.syncing = true
			s.ctrl.Replace(0, len(r.Content), t)
			s.syncing = false
		}
		// The field holds the text it is about to be given as Value.
		r.value = t
	}
	kit.Rebuild(&s.StateBase)
}

// take adds the candidates in order, and returns how many it used up: it
// stops at the first that Validate turns down or that MaxTokens leaves no room
// for. Blank and, without AllowDuplicates, repeated candidates are used up
// without a token.
func (s *tokenFieldState) take(candidates []string) int {
	w := s.Widget()
	tokens := slices.Clip(w.Tokens)
	added := false
	used := 0
	s.err = ""
	for _, c := range candidates {
		c = strings.TrimFunc(c, unicode.IsSpace)
		switch {
		case c == "":
		case !w.AllowDuplicates && slices.Contains(tokens, c):
		case w.MaxTokens > 0 && len(tokens) >= w.MaxTokens:
			s.finish(tokens, added)
			return used
		default:
			if w.Validate != nil {
				if err := w.Validate(c); err != "" {
					s.err = err
					kit.Rebuild(&s.StateBase)
					s.finish(tokens, added)
					return used
				}
			}
			tokens = append(tokens, c)
			added = true
		}
		used++
	}
	s.finish(tokens, added)
	return used
}

// finish reports the new tokens, if any were added.
func (s *tokenFieldState) finish(tokens []string, added bool) {
	if added {
		s.Widget().OnChange(tokens)
	}
}

// changed follows typing, which an IME's composition never is. Separators in
// the text end tokens.
func (s *tokenFieldState) changed(t string) {
	if s.syncing {
		return
	}
	if s.selected >= 0 {
		s.selectToken(-1)
	}
	w := s.Widget()
	if strings.IndexFunc(t, w.isSeparator) < 0 {
		if s.text != t || s.err != "" {
			s.text, s.err = t, ""
			// The text field grows with its text.
			kit.Rebuild(&s.StateBase)
		}
		return
	}
	parts := strings.FieldsFunc(t+"\x00", w.isSeparator)
	// The text after the last separator is still being typed.
	last := parts[len(parts)-1]
	last = last[:len(last)-1]
	parts = parts[:len(parts)-1]
	used := s.take(parts)
	if used < len(parts) {
		// What was not taken stays in the field, a token to a separator.
		rest := parts[used:]
		if last != "" {
			rest = append(rest, last)
		}
		for i := range rest {
			rest[i] = strings.TrimFunc(rest[i], unicode.IsSpace)
		}
		s.setText(strings.Join(rest, string(firstRune(w.separators()))))
		return
	}
	s.setText(strings.TrimLeftFunc(last, unicode.IsSpace))
}

func firstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return ','
}

// submit makes a token of the text on Enter, unless an IME is converting:
// that Enter commits the IME's text. With no text, Enter submits the Form
// around the field, if any.
func (s *tokenFieldState) submit(t string) {
	if s.composing() || s.selected >= 0 {
		return
	}
	if strings.TrimFunc(t, unicode.IsSpace) == "" {
		fieldScopeOf(s.Context()).submit()
		return
	}
	if s.take([]string{t}) == 1 {
		s.setText("")
	}
}

func (s *tokenFieldState) fieldFocus(focused bool) {
	s.focused = focused
	if !focused {
		s.selected = -1
		if strings.TrimFunc(s.text, unicode.IsSpace) != "" && s.take([]string{s.text}) == 1 {
			s.setText("")
		}
	}
	kit.Rebuild(&s.StateBase)
}

// remove takes token i out of the list.
func (s *tokenFieldState) remove(i int) {
	w := s.Widget()
	if i < 0 || i >= len(w.Tokens) {
		return
	}
	s.selected = -1
	w.OnChange(slices.Delete(slices.Clone(w.Tokens), i, i+1))
	kit.Rebuild(&s.StateBase)
}

// key sees the keys before the text does: those at the start of the text
// move into the tokens, and those on a selected token act on it.
func (s *tokenFieldState) key(e *core.KeyEvent) bool {
	w := s.Widget()
	if s.composing() || w.OnChange == nil || w.ReadOnly {
		return false
	}
	plain := e.Modifiers == 0
	if s.selected >= 0 {
		switch {
		case e.Key == input.KeyBackspace || e.Key == input.KeyDelete:
			s.remove(s.selected)
		case e.Key == input.KeyLeft && plain:
			s.selectToken(max(s.selected-1, 0))
		case e.Key == input.KeyRight && plain:
			s.selectToken(s.selected + 1)
		case e.Key == input.KeyHome && plain:
			s.selectToken(0)
		case e.Key == input.KeyEscape || e.Key == input.KeyEnd && plain:
			s.selectToken(-1)
		case e.Key == input.KeyEnter && !kit.ShortcutEnter(e, false):
		default:
			// Any other key returns to the text first.
			s.selectToken(-1)
			return false
		}
		return true
	}
	if e.Key == input.KeyV && e.Modifiers == input.ModControl {
		s.paste()
		return true
	}
	anchor, caret := s.ctrl.Selection()
	if anchor != 0 || caret != 0 || len(w.Tokens) == 0 {
		return false
	}
	switch {
	case e.Key == input.KeyBackspace && e.Modifiers&^input.ModControl == 0,
		e.Key == input.KeyLeft && plain:
		s.selectToken(len(w.Tokens) - 1)
		return true
	}
	return false
}

// paste puts the text of the clipboard in place of the selection with its
// line breaks as separators, which a single line field would make spaces.
func (s *tokenFieldState) paste() {
	r := s.ctrl.field()
	if r == nil {
		return
	}
	sep := string(firstRune(s.Widget().separators()))
	clip := strings.NewReplacer("\r\n", sep, "\n", sep, "\r", sep).Replace(r.Host().ClipboardText())
	s.ctrl.Insert(clip)
}

// Build implements core.State.
func (s *tokenFieldState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	th := basic.ThemeOf(ctx)
	c := &th.Color
	enabled := w.OnChange != nil
	editable := enabled && !w.ReadOnly
	scope := fieldScopeOf(ctx)
	if s.err != "" && scope.err == "" {
		scope.err = s.err
	}

	children := make([]core.Widget, 0, len(w.Tokens)+1)
	for i, tok := range w.Tokens {
		chip := control.Chip{Label: tok}
		if editable {
			chip.OnRemove = func() {
				s.remove(i)
				s.ctrl.Focus()
			}
		}
		item := interact.Semantics{Role: semantics.RoleListItem, Label: tok}
		var body core.Widget = chip
		if i == s.selected {
			item.State = semantics.Selected
			body = kit.Ring{Radius: tokenChipHeight / 2, Color: c.AccentInk.WithAlpha(kit.RingOpacity), Child: chip}
		}
		item.Child = body
		// A chip keeps its state with its token, the nth of that text.
		nth := 0
		for _, before := range w.Tokens[:i] {
			if before == tok {
				nth++
			}
		}
		children = append(children, core.Keyed(tokenKey{tok, nth}, item))
	}
	placeholder := ""
	if len(w.Tokens) == 0 {
		placeholder = w.Placeholder
	}
	if editable {
		children = append(children, s.input(ctx, th, placeholder, scope))
	}

	border := c.Hair
	switch {
	case scope.err != "":
		border = c.Danger
	case s.focused:
		border = c.AccentInk
	case s.hovered && editable:
		border = c.Ink3
	}
	ground := c.Field
	if !enabled {
		ground = c.Fill
	}
	var ring geom.Color
	if s.focused {
		ring = border.WithAlpha(kit.RingOpacity)
	}
	var box core.Widget = kit.Ring{Radius: th.Radius.Control, Color: ring, Child: basic.Container{
		Color: ground, Radius: th.Radius.Control, Border: paint.Stroke{Width: 1, Color: border},
		Padding: geom.All(tokenPadding),
		Child: basic.ConstrainedBox{MinHeight: fieldHeight - 2*tokenPadding, Child: basic.Wrap{
			Spacing: tokenGap, RunSpacing: tokenGap, Cross: layout.CrossCenter, RunAlignment: layout.MainCenter, Children: children,
		}},
	}}
	if editable {
		box = interact.MouseRegion{
			OnEnter: func() { s.hovered = true; kit.Rebuild(&s.StateBase) },
			OnExit:  func() { s.hovered = false; kit.Rebuild(&s.StateBase) },
			Cursor:  input.CursorIBeam,
			// A click beside the chips starts typing.
			Child: interact.GestureDetector{OnTap: s.ctrl.Focus, Child: box},
		}
	}
	label := w.Label
	if label == "" {
		label = w.Placeholder
	}
	list := interact.Semantics{Role: semantics.RoleList, Label: label, Hint: scope.err, State: scope.state()}
	switch {
	case !enabled:
		list.State |= semantics.Disabled
	case w.ReadOnly:
		list.State |= semantics.ReadOnly
	}
	parts := make([]core.Widget, 0, 3)
	if w.Label != "" && !w.HideLabel {
		parts = append(parts, basic.Text{Content: w.Label, Style: kit.Colored(th.Text.Caption, c.Ink2)})
	}
	parts = append(parts, box)
	if s.err != "" {
		parts = append(parts, basic.Text{Content: s.err, Style: kit.Colored(th.Text.Caption, c.Danger)})
	}
	if len(parts) == 1 {
		list.Child = box
		return list
	}
	list.Child = basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: fieldLabelGap, Children: parts}
	return list
}

// input returns the text field after the chips, as wide as its text.
func (s *tokenFieldState) input(ctx core.BuildContext, th *theme.Theme, placeholder string, scope fieldScope) core.Widget {
	style := fieldStyle(th, text.Style{})
	width := float32(tokenInputMin)
	if shaper, ok := core.DependOn[*text.Shaper](ctx); ok {
		measure := s.text
		if measure == "" {
			measure = placeholder
		}
		if measure != "" {
			width = max(width, kit.TextWidth(shaper, measure, style)+tokenInputSlack)
		}
	}
	return basic.SizedBox{Width: width, Child: core.Inherited[fieldScope]{Value: scope, Child: Input{
		Value: s.text,
		// The text field is named as the list is, so that it is not left
		// nameless when only Label is set; the caption is the list's.
		Label:       s.Widget().Label,
		HideLabel:   true,
		Placeholder: placeholder,
		Bare:        true,
		Padding:     geom.Insets{Left: theme.Space2, Right: theme.Space2},
		Controller:  &s.ctrl,
		OnChange:    s.changed,
		OnSubmit:    s.submit,
		OnFocus:     s.fieldFocus,
		extras:      fieldExtras{onKey: s.key},
	}}}
}

// tokenKey keeps the state of a chip with its token as tokens come and go:
// the nth token of that text.
type tokenKey struct {
	token string
	nth   int
}
