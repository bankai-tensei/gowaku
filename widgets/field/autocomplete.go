package field

import (
	"strings"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
)

// Autocomplete is a Input that suggests choices below itself as text is
// typed. The arrows move through the suggestions, Enter or Tab or a click takes
// the highlighted one into the field, and Escape closes the list. The keyboard
// stays in the field throughout.
//
// The list follows the committed text only: while an IME is converting, the
// suggestions stay as they were and the keys belong to the IME.
//
// The choices come from Suggestions, which gets the text and returns what to
// show, or else from Options, of which those containing the text show, those
// that start with it first, ignoring case. Format spells a choice; it may be
// nil when T is string.
//
// Value works as for Input, and OnChange reports every edit, a taken
// suggestion included. OnSelect reports the choice that was taken. OnSubmit
// runs on Enter when no suggestion is highlighted.
type Autocomplete[T any] struct {
	core.StatefulBase
	Value string
	// Label names the field above it and to a screen reader.
	Label string
	// HideLabel draws no caption for Label, which still names the field to a
	// screen reader.
	HideLabel   bool
	Placeholder string
	// Disabled and ReadOnly work as they do on Input, and neither opens the
	// list of suggestions.
	Disabled, ReadOnly bool
	Options            []T
	Suggestions        func(query string) []T
	Format             func(T) string
	OnChange           func(string)
	OnSelect           func(T)
	OnSubmit           func(string)
	// ShowNoResults keeps the list open with the words for No results when
	// nothing matches, instead of closing it.
	ShowNoResults bool
}

type autocompleteState[T any] struct {
	core.StateOf[Autocomplete[T]]
	text  string
	items []T
	// highlight is the suggestion that Enter takes, or -1 for none.
	highlight int
	scope     fieldScope
	// taking is set while a suggestion goes into the field, whose edit must
	// not ask for suggestions again.
	taking bool
	// entry is the open list, or nil, with its layer, the place it opens
	// from and the words for No results.
	entry     *overlay.Entry
	layer     *kit.PopupLayerState
	anchor    geom.Rect
	rows      int
	noResults string
}

// CreateState implements core.StatefulWidget.
func (Autocomplete[T]) CreateState() core.State { return &autocompleteState[T]{highlight: -1} }

// InitState implements core.State.
func (s *autocompleteState[T]) InitState() { s.text = s.Widget().Value }

// DidUpdateWidget implements core.State.
func (s *autocompleteState[T]) DidUpdateWidget(old core.Widget) {
	if v := s.Widget().Value; v != old.(Autocomplete[T]).Value {
		s.text = v
	}
	s.refresh()
}

// Dispose implements core.State.
func (s *autocompleteState[T]) Dispose() {
	if s.entry != nil {
		s.entry.Remove()
		s.entry = nil
	}
}

// Build implements core.State.
func (s *autocompleteState[T]) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	s.scope = fieldScopeOf(ctx)
	label := w.Label
	if label == "" {
		label = w.Placeholder
	}
	sem := interact.Semantics{
		Role: semantics.RoleComboBox, Label: label, Value: s.text, Hint: s.scope.err, State: s.scope.state(),
		OnExpand: func() { s.show(true) }, OnCollapse: s.close,
	}
	if s.entry != nil {
		sem.State |= semantics.Expanded
	} else {
		sem.State |= semantics.Collapsed
	}
	sem.Child = interact.Focus{OnKey: s.key, Child: Input{
		Value:       w.Value,
		Label:       w.Label,
		HideLabel:   w.HideLabel,
		Placeholder: w.Placeholder,
		Disabled:    w.Disabled,
		ReadOnly:    w.ReadOnly,
		OnChange:    s.changed,
		OnSubmit:    s.submit,
		OnFocus:     s.fieldFocus,
	}}
	return sem
}

// format spells a choice.
func (s *autocompleteState[T]) format(v T) string {
	if f := s.Widget().Format; f != nil {
		return f(v)
	}
	if str, ok := any(v).(string); ok {
		return str
	}
	panic("gowaku: Autocomplete needs Format to spell a choice that is not a string")
}

// suggest returns the choices for query.
func (s *autocompleteState[T]) suggest(query string) []T {
	w := s.Widget()
	if w.Suggestions != nil {
		return w.Suggestions(query)
	}
	q := strings.ToLower(query)
	var starts, contains []T
	for _, o := range w.Options {
		l := strings.ToLower(s.format(o))
		switch {
		case strings.HasPrefix(l, q):
			starts = append(starts, o)
		case strings.Contains(l, q):
			contains = append(contains, o)
		}
	}
	return append(starts, contains...)
}

// changed follows an edit, which an IME's composition never is.
func (s *autocompleteState[T]) changed(v string) {
	s.text = v
	// The combo box tells a screen reader the text.
	kit.Rebuild(&s.StateBase)
	if f := s.Widget().OnChange; f != nil {
		f(v)
	}
	if s.taking {
		return
	}
	if v == "" {
		s.close()
		return
	}
	s.show(false)
}

// show lists the suggestions for the text. all lists them for an empty text
// as well, as Down and a screen reader ask.
func (s *autocompleteState[T]) show(all bool) {
	if s.text == "" && !all {
		s.close()
		return
	}
	s.items = s.suggest(s.text)
	s.highlight = -1
	if len(s.items) == 0 && !s.Widget().ShowNoResults {
		s.close()
		return
	}
	field := FindEditable(s.Context().RenderObject())
	if field == nil || !field.Attached() {
		return
	}
	o, ok := core.Lookup[*overlay.Host](s.Context())
	if !ok {
		return
	}
	s.anchor = kit.OverlayRect(o.Context().RenderObject(), kit.GlobalRect(field))
	s.rows = kit.PopupRows(o.Context().RenderObject(), s.anchor)
	s.noResults = basic.LookupStrings(s.Context()).NoResults
	if s.entry == nil {
		s.entry = o.InsertFrom(s.Context(), func() core.Widget {
			return kit.PopupLayer{Build: s.list, State: &s.layer}
		})
		kit.Rebuild(&s.StateBase)
		return
	}
	s.refresh()
}

func (s *autocompleteState[T]) close() {
	if s.entry == nil {
		return
	}
	s.entry.Remove()
	s.entry, s.layer = nil, nil
	s.highlight = -1
	kit.Rebuild(&s.StateBase)
}

// refresh rebuilds the open list.
func (s *autocompleteState[T]) refresh() {
	if s.layer != nil && s.layer.Mounted() {
		kit.Rebuild(&s.layer.StateBase)
	}
}

// list builds the suggestions below the field.
func (s *autocompleteState[T]) list() core.Widget {
	labels := make([]string, len(s.items))
	for i, it := range s.items {
		labels[i] = s.format(it)
	}
	return kit.Popup{Anchor: s.anchor, Side: layout.PopupBelow, Child: overlay.PopupList{
		Label: s.Widget().Label, Labels: labels, Hot: s.highlight, Checked: -1,
		Width: s.anchor.W, Rows: s.rows, Empty: s.noResults,
		OnHover: s.hover, OnTap: s.take,
	}}
}

func (s *autocompleteState[T]) hover(i int) {
	if s.highlight != i {
		s.highlight = i
		s.refresh()
	}
}

// composing reports whether an IME is converting text in the field.
func (s *autocompleteState[T]) composing() bool {
	r := FindEditable(s.Context().RenderObject())
	return r != nil && r.Composition != nil
}

// key handles the keys that the field leaves: the arrows, Escape and Tab.
func (s *autocompleteState[T]) key(e *core.KeyEvent) bool {
	if w := s.Widget(); w.Disabled || w.ReadOnly || s.composing() {
		return false
	}
	open := s.entry != nil
	switch {
	case e.Key == input.KeyDown && e.Modifiers&^input.ModAlt == 0:
		if !open {
			s.show(true)
			if s.entry != nil && e.Modifiers == 0 && len(s.items) > 0 {
				s.highlight = 0
			}
		} else if s.highlight < len(s.items)-1 {
			s.highlight++
		}
		s.refresh()
		return true
	case e.Key == input.KeyUp && e.Modifiers == 0 && open:
		if s.highlight >= 0 {
			s.highlight--
			s.refresh()
		}
		return true
	case e.Key == input.KeyEscape && e.Modifiers == 0 && open:
		s.close()
		return true
	case e.Key == input.KeyTab && open:
		// The focus moves on after the choice is taken.
		if s.highlight >= 0 && e.Modifiers == 0 {
			s.take(s.highlight)
		}
		s.close()
	}
	return false
}

// submit takes the highlighted suggestion on Enter, or else submits the text.
func (s *autocompleteState[T]) submit(v string) {
	if s.composing() {
		return
	}
	if s.entry != nil && s.highlight >= 0 {
		s.take(s.highlight)
		return
	}
	s.close()
	if f := s.Widget().OnSubmit; f != nil {
		f(v)
	} else {
		s.scope.submit()
	}
}

// take puts suggestion i into the field, as an edit that Ctrl+Z takes back.
func (s *autocompleteState[T]) take(i int) {
	if i < 0 || i >= len(s.items) {
		return
	}
	item := s.items[i]
	if r := FindEditable(s.Context().RenderObject()); r != nil {
		s.taking = true
		r.replace(0, len(r.Content), r.sanitize(s.format(item)), kit.EditOther)
		s.taking = false
	}
	s.close()
	if f := s.Widget().OnSelect; f != nil {
		f(item)
	}
}

// fieldFocus closes the list when the field loses the keyboard.
func (s *autocompleteState[T]) fieldFocus(focused bool) {
	if !focused {
		s.close()
	}
}
