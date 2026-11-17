package field

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

const (
	// searchGlyphInset is the room left of the magnifier.
	searchGlyphInset = theme.Space8
	// searchClearBox is the square that the clear button takes, and
	// searchClearInset the room right of it.
	searchClearBox   = 22
	searchClearInset = theme.Space4
)

// Search is a Input for a search: a magnifier before the text, the
// word for Search while it is empty, and a button that clears it while it is
// not. Escape clears it as well. OnChange runs on every edit, the clearing
// included, so a list can follow the query as it is typed; OnSubmit runs on
// Enter.
//
// Value works as for Input. Label, if set, names the field above it, and
// HideLabel keeps it as the name for a screen reader only. Placeholder
// replaces the word for Search.
type Search struct {
	core.StatefulBase
	Value string
	Label string
	// HideLabel draws no caption for Label, which still names the field to a
	// screen reader.
	HideLabel   bool
	Placeholder string
	// Disabled and ReadOnly work as they do on Input. Neither shows the clear
	// button, and Escape leaves the text alone.
	Disabled, ReadOnly bool
	OnChange           func(string)
	OnSubmit           func(string)
}

type searchFieldState struct {
	core.StateOf[Search]
	// text is what the field holds, which decides the clear button.
	text             string
	magnifier, cross *paint.Path
}

// CreateState implements core.StatefulWidget.
func (Search) CreateState() core.State { return &searchFieldState{} }

// InitState implements core.State.
func (s *searchFieldState) InitState() {
	s.text = s.Widget().Value
	s.magnifier, s.cross = kit.MagnifierGlyph(), kit.ClearGlyph()
}

// DidUpdateWidget implements core.State.
func (s *searchFieldState) DidUpdateWidget(old core.Widget) {
	if v := s.Widget().Value; v != old.(Search).Value {
		s.text = v
	}
}

// Build implements core.State.
func (s *searchFieldState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	th := basic.ThemeOf(ctx)
	words := basic.StringsOf(ctx)
	placeholder := w.Placeholder
	if placeholder == "" {
		placeholder = words.Search
	}
	extras := Extras{leading: interact.GestureDetector{
		// The magnifier is part of the field: a click on it starts typing.
		OnTap: s.focusField,
		Child: basic.Padding{Insets: geom.Insets{Left: searchGlyphInset}, Child: basic.Icon{
			Path: s.magnifier, Size: kit.GlyphSize, Color: th.Color.Ink3,
		}},
	}}
	if s.text != "" && !w.Disabled && !w.ReadOnly {
		extras.trailing = interact.Pressable{
			Label:   words.Clear,
			OnPress: s.clear,
			// Escape does the same from the field, so Tab passes the button.
			SkipTab: true,
			Build: func(p interact.PressState) core.Widget {
				color := th.Color.Ink3
				if p.Hovered || p.Pressed {
					color = th.Color.Ink2
				}
				return basic.Padding{Insets: geom.Insets{Right: searchClearInset}, Child: basic.SizedBox{
					Width: searchClearBox, Height: searchClearBox,
					Child: basic.Center{Child: basic.Icon{Path: s.cross, Size: kit.GlyphSize, Color: color}},
				}}
			},
		}
	}
	return interact.Focus{OnKey: s.key, Child: Input{
		Value:       w.Value,
		Label:       w.Label,
		HideLabel:   w.HideLabel,
		Placeholder: placeholder,
		Disabled:    w.Disabled,
		ReadOnly:    w.ReadOnly,
		OnChange:    s.changed,
		OnSubmit:    w.OnSubmit,
		Extras:      extras,
	}}
}

func (s *searchFieldState) changed(v string) {
	if (v == "") != (s.text == "") {
		kit.Rebuild(&s.StateBase)
	}
	s.text = v
	if f := s.Widget().OnChange; f != nil {
		f(v)
	}
}

// key clears the field on Escape, unless an IME is converting: the Escape is
// the IME's then.
func (s *searchFieldState) key(e *core.KeyEvent) bool {
	if w := s.Widget(); e.Key != input.KeyEscape || e.Modifiers != 0 || s.text == "" || w.Disabled || w.ReadOnly {
		return false
	}
	if r := FindEditable(s.Context().RenderObject()); r == nil || r.Composition != nil {
		return false
	}
	s.clear()
	return true
}

// clear empties the field as an edit, which Ctrl+Z takes back, and gives it
// the keyboard.
func (s *searchFieldState) clear() {
	r := FindEditable(s.Context().RenderObject())
	if r == nil {
		return
	}
	if r.Composition != nil {
		r.Composition = nil
		r.Host().CancelComposition()
	}
	r.replace(0, len(r.Content), "", kit.EditOther)
	r.RequestFocus()
}

func (s *searchFieldState) focusField() {
	if s.Widget().Disabled {
		return
	}
	if r := FindEditable(s.Context().RenderObject()); r != nil {
		r.RequestFocus()
	}
}

// FindEditable returns the render object of the first text field at or below
// ro, or nil.
func FindEditable(ro core.RenderObject) *RenderEditable {
	if ro == nil {
		return nil
	}
	if r, ok := ro.(*RenderEditable); ok {
		return r
	}
	for i := range ro.ChildCount() {
		if r := FindEditable(ro.ChildAt(i)); r != nil {
			return r
		}
	}
	return nil
}
