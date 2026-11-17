package editor

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	controlw "github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

const (
	// findFieldWidth is the width of the fields of the find bar, findButton
	// the square of its small buttons and findCountWidth the room of the
	// count of matches.
	findFieldWidth = 168
	findButton     = 24
	findCountWidth = 52
	// findBarTop is how far below the top of the editor the bar sits, and
	// findBarPad the room around and between its rows, of a control each.
	findBarTop = theme.Space6
	findBarPad = theme.Space6
)

// findRoom returns how far down the open find bar reaches from the top of the
// view, or 0 while it is closed. The view keeps that much room above its first
// line and reveals the current match below it, so that no match is hidden
// under the bar. It follows from the rows of the bar, as laid out in findBar,
// so that the view knows it before the bar is laid out.
func (s *textEditorState) findRoom() float32 {
	if !s.finding {
		return 0
	}
	rows := float32(1)
	if s.replacing && !s.Widget().(Editor).ReadOnly {
		rows = 2
	}
	return findBarTop + 2*findBarPad + rows*theme.ControlHeight + (rows-1)*findBarPad
}

// findGlyphs are the paths of the find bar, made once per editor.
type findGlyphs struct {
	up, down, cross *paint.Path
}

func newFindGlyphs() findGlyphs {
	up := &paint.Path{}
	up.MoveTo(6.3, 15.7)
	up.LineTo(12, 10)
	up.LineTo(17.7, 15.7)
	up.LineTo(19.1, 14.3)
	up.LineTo(12, 7.2)
	up.LineTo(4.9, 14.3)
	up.Close()
	cross := &paint.Path{}
	const a, t = 6.5, 0.75
	cross.MoveTo(12-a-t, 12-a+t)
	cross.LineTo(12-a+t, 12-a-t)
	cross.LineTo(12+a+t, 12+a-t)
	cross.LineTo(12+a-t, 12+a+t)
	cross.Close()
	cross.MoveTo(12+a-t, 12-a-t)
	cross.LineTo(12+a+t, 12-a+t)
	cross.LineTo(12-a+t, 12+a+t)
	cross.LineTo(12-a-t, 12+a-t)
	cross.Close()
	return findGlyphs{up: up, down: kit.ChevronGlyph(), cross: cross}
}

// findBar is the quiet bar over the top right corner of the editor that finds
// and replaces.
func (s *textEditorState) findBar(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	c := &th.Color
	words := basic.StringsOf(ctx)
	if s.glyphs.up == nil {
		s.glyphs = newFindGlyphs()
	}
	ctl := s.controller()
	count := ""
	switch {
	case s.badQuery:
		count = words.BadPattern
	case s.query == "":
	case len(ctl.Matches()) == 0 && ctl.Searching():
		count = words.Searching
	case len(ctl.Matches()) == 0:
		count = words.NoResults
	default:
		count = words.MatchCount(ctl.currentMatchOf(&s.view().sel)+1, len(ctl.Matches()))
		if ctl.Searching() {
			// More may come.
			count += "…"
		}
	}
	countStyle := kit.Colored(th.Text.Caption, c.Ink2)
	if s.badQuery {
		countStyle.Color = c.Danger
	}
	top := basic.Row{Size: layout.MainMin, Cross: layout.CrossCenter, Spacing: theme.Space4, Children: []core.Widget{
		basic.SizedBox{Width: findFieldWidth, Child: field.Input{
			Value: s.query, Placeholder: words.Find, OnChange: s.queryChanged,
			OnSubmit: func(string) { s.activate(); ctl.FindNext() },
		}},
		basic.SizedBox{Width: findCountWidth, Child: basic.Text{Content: count, Style: countStyle}},
		s.toggle(th, "Aa", words.MatchCase, &s.options.MatchCase),
		s.toggle(th, "W", words.WholeWord, &s.options.WholeWord),
		s.toggle(th, ".*", words.RegularExpression, &s.options.Regexp),
		s.iconButton(th, s.glyphs.up, words.PreviousMatch, func() { s.activate(); ctl.FindPrevious() }),
		s.iconButton(th, s.glyphs.down, words.NextMatch, func() { s.activate(); ctl.FindNext() }),
		s.iconButton(th, s.glyphs.cross, words.Close, s.closeFind),
	}}
	rows := []core.Widget{top}
	if s.replacing && !s.Widget().(Editor).ReadOnly {
		rows = append(rows, basic.Row{Size: layout.MainMin, Cross: layout.CrossCenter, Spacing: theme.Space4, Children: []core.Widget{
			basic.SizedBox{Width: findFieldWidth, Child: field.Input{
				Value: s.replacement, Placeholder: words.ReplaceWith,
				OnChange: func(v string) { s.replacement = v },
				OnSubmit: func(string) { s.activate(); ctl.ReplaceMatch(s.replacement) },
			}},
			controlw.Button{Label: words.Replace, Style: controlw.ButtonQuiet, OnPress: func() { s.activate(); ctl.ReplaceMatch(s.replacement) }},
			controlw.Button{Label: words.ReplaceAll, Style: controlw.ButtonQuiet, OnPress: func() { s.activate(); ctl.ReplaceAll(s.replacement) }},
		}})
	}
	return interact.Semantics{Role: semantics.RoleToolbar, Label: words.Find, Child: interact.Focus{OnKey: s.barKey, Child: focusQuery{
		request: s.focusRequest, typed: s.typedAhead, focused: s.queryFocused,
		child: basic.Container{
			Color:   c.Raised,
			Radius:  th.Radius.Control,
			Border:  paint.Stroke{Width: 1, Color: c.Hair},
			Shadows: th.Color.Shadows(theme.LiftFloat),
			Padding: geom.All(findBarPad),
			Child:   basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: findBarPad, Children: rows},
		},
	}}}
}

// queryChanged runs the search as the query is typed.
func (s *textEditorState) queryChanged(v string) {
	s.query = v
	s.search()
	kit.Rebuild(&s.StateBase)
}

// toggle is a small button that switches an option of the search.
func (s *textEditorState) toggle(th *theme.Theme, mark, label string, on *bool) core.Widget {
	c := &th.Color
	checked := *on
	return interact.Pressable{
		Label: label,
		Role:  semantics.RoleCheckbox,
		State: checkedState(checked),
		OnPress: func() {
			*on = !*on
			s.search()
			kit.Rebuild(&s.StateBase)
		},
		Build: func(p interact.PressState) core.Widget {
			var ground geom.Color
			ink := c.Ink2
			switch {
			case checked:
				ground, ink = c.AccentTint, c.OnTint().AccentInk
			case p.Hovered || p.Pressed:
				ground = c.Fill
			}
			return interact.FocusRing{Visible: p.FocusVisible, Radius: th.Radius.Check, Child: basic.Container{
				Width: findButton, Height: findButton, Color: ground, Radius: th.Radius.Check,
				Child: basic.Center{Child: basic.Text{Content: mark, Style: text.Style{
					Size: th.Text.Caption.Size, LineHeight: th.Text.Caption.LineHeight, Weight: 600, Color: ink,
				}}},
			}}
		},
	}
}

func checkedState(on bool) semantics.State {
	if on {
		return semantics.Checked
	}
	return 0
}

// iconButton is a small square button of the find bar.
func (s *textEditorState) iconButton(th *theme.Theme, path *paint.Path, label string, press func()) core.Widget {
	c := &th.Color
	return interact.Pressable{
		Label:   label,
		OnPress: press,
		Build: func(p interact.PressState) core.Widget {
			var ground geom.Color
			if p.Hovered || p.Pressed {
				ground = c.Fill
			}
			return interact.FocusRing{Visible: p.FocusVisible, Radius: th.Radius.Check, Child: basic.Container{
				Width: findButton, Height: findButton, Color: ground, Radius: th.Radius.Check,
				Child: basic.Center{Child: basic.Icon{Path: path, Size: kit.GlyphSize, Color: c.Ink2}},
			}}
		},
	}
}

// barKey handles the keys of the find bar wherever its focus is.
func (s *textEditorState) barKey(e *core.KeyEvent) bool {
	s.activate()
	ctl := s.controller()
	control := e.Modifiers&input.ModControl != 0
	shift := e.Modifiers&input.ModShift != 0
	switch {
	case e.Key == input.KeyEscape:
		s.closeFind()
	case e.Key == input.KeyF3 && shift:
		ctl.FindPrevious()
	case e.Key == input.KeyF3:
		ctl.FindNext()
	case control && e.Key == input.KeyF:
		s.openFind(false)
	case control && e.Key == input.KeyH:
		s.openFind(!s.replacing)
	default:
		return false
	}
	return true
}

// focusQuery gives the first text field below it the keyboard, with its text
// selected, each time request changes. When text was typed into the query
// before, the caret goes after it instead, so that typing goes on.
type focusQuery struct {
	request int
	typed   bool
	focused func()
	child   core.Widget
}

// Key implements core.Widget.
func (focusQuery) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (f focusQuery) CreateRenderObject() core.RenderObject {
	return &renderFocusQuery{request: f.request, done: f.request - 1, typed: f.typed, focused: f.focused}
}

// UpdateRenderObject implements core.RenderWidget.
func (f focusQuery) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderFocusQuery)
	r.typed, r.focused = f.typed, f.focused
	if r.request != f.request {
		r.request = f.request
		r.MarkNeedsLayout()
	}
}

// ChildWidget implements core.SingleChildWidget.
func (f focusQuery) ChildWidget() core.Widget { return f.child }

type renderFocusQuery struct {
	layout.RenderProxy
	request, done int
	typed         bool
	focused       func()
}

// Layout implements core.RenderObject.
func (r *renderFocusQuery) Layout(c layout.Constraints) geom.Size {
	sz := r.RenderProxy.Layout(c)
	if r.done != r.request {
		r.done = r.request
		if f := field.FindEditable(r); f != nil {
			f.RequestFocus()
			f.Anchor = 0
			if r.typed {
				f.Anchor = len(f.Content)
			}
			f.MoveTo(len(f.Content), true)
		}
		if r.focused != nil {
			r.focused()
		}
	}
	return sz
}
