package field

import (
	"slices"
	"strings"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

// The measures of a command palette.
const (
	paletteWidth = 560
	// paletteRows is how many results show at once, paletteRow the height of
	// one, and paletteRowGap the room between two.
	paletteRows   = 8
	paletteRow    = 32
	paletteRowGap = theme.Space2
	// paletteFooter is the height of the line under the results that counts
	// them when they do not all show.
	paletteFooter = 28
	// paletteListHeight holds a full page of rows and the line below them, so
	// that the dialog, centered in the window, does not jump as the query
	// narrows the results.
	paletteListHeight = paletteRows*(paletteRow+paletteRowGap) + paletteFooter
	// paletteRecent is how many commands the palette remembers for
	// RecentFirst.
	paletteRecent = 8
	// paletteDetailPenalty puts the items found by their Detail after those
	// found by their label.
	paletteDetailPenalty = 1 << 16
	// paletteCheck is the side of the check mark of a checked action.
	paletteCheck = 16
)

// PaletteItem is a choice of a CommandPalette besides its Actions, such as a
// recent file or a page to go to.
type PaletteItem struct {
	// Label is what the row shows and the query matches.
	Label string
	// Detail is shown dimmed at the end of the row, such as "最近のファイル".
	// The query matches it too, after the labels.
	Detail string
	// OnInvoke runs the item. Without it the item is disabled.
	OnInvoke func()
}

// CommandPalette lets the user find a command by typing part of its name, as
// in an editor: Ctrl+Shift+P (Cmd+Shift+P on macOS) opens a dialog with a
// search field over the Actions, and typing narrows them by fuzzy matching,
// best match first, with the matched letters marked. Up and Down choose, Enter
// or a click runs the chosen one, and Escape closes the palette, giving the
// keyboard back to where it was. While an IME is converting, Enter, Escape
// and the arrows belong to the IME.
//
// Give it the same actions that the window registers with interact.Actions
// and the menus, around the part of the tree they serve:
//
//	field.CommandPalette{Actions: acts, Child: interact.Actions{Actions: acts, Child: page}}
//
// Each row shows the label of its action, without its mnemonic, and its
// shortcut. Disabled actions show grayed out; the arrows pass them by and
// Enter does nothing on them. A Checkable action that is Checked shows a check
// mark. Extra adds choices that are not actions.
//
// The palette follows its widget while it is open: when the parent rebuilds
// with other actions, the results change with them. Opening it is an action
// too: CommandPaletteAction returns it for a menu, and OpenCommandPalette opens
// the nearest palette from a callback.
//
// The palette is a thin searcher; it has no pages, sub-menus or providers of
// its own. See docs/ADR/0097-command-palette.md.
type CommandPalette struct {
	core.StatefulBase
	// Actions are the commands the palette finds.
	Actions []interact.Action
	// Extra are further choices, listed after the actions of the same score.
	Extra []PaletteItem
	// Shortcut opens the palette. The zero value selects Ctrl+Shift+P.
	Shortcut interact.Shortcut
	// Placeholder is the hint of the search field. Empty selects the words
	// for Search commands.
	Placeholder string
	// RecentFirst lists the choices run last at the top while the query is
	// empty, and before others of the same score while it is not.
	RecentFirst bool
	Child       core.Widget
}

type commandPaletteState struct {
	core.StateOf[CommandPalette]
	// body is the open palette, or nil, and dismiss closes it.
	body    *paletteBodyState
	open    bool
	dismiss func()
	// recent holds the labels of the choices run last, the latest first.
	recent []string
	// actions holds the action that opens the palette, for interact.Actions.
	actions [1]interact.Action
}

// CreateState implements core.StatefulWidget.
func (CommandPalette) CreateState() core.State { return &commandPaletteState{} }

// DidUpdateWidget implements core.State.
func (s *commandPaletteState) DidUpdateWidget(core.Widget) {
	if s.body != nil && s.body.Mounted() {
		kit.Rebuild(&s.body.StateBase)
	}
}

// Dispose implements core.State.
func (s *commandPaletteState) Dispose() {
	if s.open && s.dismiss != nil {
		s.dismiss()
	}
}

// Build implements core.State.
func (s *commandPaletteState) Build(ctx core.BuildContext) core.Widget {
	s.actions[0] = s.openAction(ctx)
	return core.Inherited[*commandPaletteState]{Value: s, Child: interact.Actions{
		Actions: s.actions[:],
		Child:   s.Widget().Child,
	}}
}

// openAction returns the action that opens the palette.
func (s *commandPaletteState) openAction(ctx core.BuildContext) interact.Action {
	sc := s.Widget().Shortcut
	if sc.Key == input.KeyNone {
		sc = interact.Shortcut{Key: input.KeyP, Modifiers: input.ModControl | input.ModShift}
	}
	return interact.Action{Label: basic.StringsOf(ctx).CommandPalette, Shortcut: sc, OnInvoke: s.show}
}

// show opens the palette, unless it is open.
func (s *commandPaletteState) show() {
	if s.open || !s.Mounted() {
		return
	}
	s.open = true
	overlay.ShowDialog(s.Context(), func(dismiss func()) core.Widget {
		s.dismiss = dismiss
		return overlay.Dialog{Glass: true, Width: paletteWidth, Content: paletteBody{owner: s}}
	})
}

// remember puts label at the top of the recent choices.
func (s *commandPaletteState) remember(label string) {
	if i := slices.Index(s.recent, label); i >= 0 {
		s.recent = slices.Delete(s.recent, i, i+1)
	}
	s.recent = slices.Insert(s.recent, 0, label)
	if len(s.recent) > paletteRecent {
		s.recent = s.recent[:paletteRecent]
	}
}

// OpenCommandPalette opens the nearest CommandPalette above ctx, as its
// shortcut does. It must not be called during a Build, and panics if there is
// no CommandPalette above ctx.
func OpenCommandPalette(ctx core.BuildContext) {
	paletteOf(ctx, "OpenCommandPalette").show()
}

// CommandPaletteAction returns the action that opens the nearest
// CommandPalette above ctx, with its shortcut, for a menu or a toolbar. It
// panics if there is no CommandPalette above ctx.
func CommandPaletteAction(ctx core.BuildContext) interact.Action {
	return paletteOf(ctx, "CommandPaletteAction").openAction(ctx)
}

func paletteOf(ctx core.BuildContext, caller string) *commandPaletteState {
	s, ok := core.Lookup[*commandPaletteState](ctx)
	if !ok {
		panic("gowaku: " + caller + " found no CommandPalette above the BuildContext")
	}
	return s
}

// paletteChoice is one row the palette can show.
type paletteChoice struct {
	label, detail    string
	enabled, checked bool
	run              func()
	score            int
	// order is the place of the choice in Actions and Extra, and recent its
	// place among the recent choices, or len(recent) if it is not one.
	order, recent int
	// ranges are the matched byte ranges of label, as pairs, taken from
	// the ranges of the body from rangeAt.
	rangeAt, rangeLen int
}

// paletteBody is the search field and the results of an open palette.
type paletteBody struct {
	core.StatefulBase
	owner *commandPaletteState
}

type paletteBodyState struct {
	core.StateOf[paletteBody]
	query    string
	selected int
	field    InputController
	fuzzy    Fuzzy
	// shown are the results for the query, best first; ranges holds their
	// matched ranges, which shown slices.
	shown  []paletteChoice
	ranges []int
	// focus gives the field the keyboard on the first frame after the
	// dialog took it, which it does at its first layout, after its children.
	focus *core.Ticker
}

// CreateState implements core.StatefulWidget.
func (paletteBody) CreateState() core.State { return &paletteBodyState{} }

// InitState implements core.State.
func (s *paletteBodyState) InitState() {
	s.Widget().owner.body = s
	s.focus = s.NewTicker(func(time.Duration) {
		s.focus.Stop()
		s.field.Focus()
	})
	s.focus.Start()
}

// Dispose implements core.State.
func (s *paletteBodyState) Dispose() {
	o := s.Widget().owner
	if o.body == s {
		o.body, o.open, o.dismiss = nil, false, nil
	}
}

// filter fills shown with the choices that match the query, best first.
func (s *paletteBodyState) filter() {
	o := s.Widget().owner
	w := o.Widget()
	s.fuzzy.SetQuery(s.query)
	s.shown, s.ranges = s.shown[:0], s.ranges[:0]
	recent := o.recent
	if !w.RecentFirst {
		recent = nil
	}
	add := func(label, detail string, enabled, checked bool, run func(), order int) {
		score, ok := s.fuzzy.Match(label)
		var ranges []int
		if ok {
			ranges = s.fuzzy.Ranges()
		} else if detail != "" {
			if score, ok = s.fuzzy.Match(detail); ok {
				score -= paletteDetailPenalty
			}
		}
		if !ok {
			return
		}
		at := len(s.ranges)
		s.ranges = append(s.ranges, ranges...)
		rank := slices.Index(recent, label)
		if rank < 0 {
			rank = len(recent)
		}
		s.shown = append(s.shown, paletteChoice{
			label: label, detail: detail, enabled: enabled, checked: checked, run: run,
			score: score, order: order, recent: rank, rangeAt: at, rangeLen: len(ranges),
		})
	}
	for i := range w.Actions {
		a := &w.Actions[i]
		add(paletteLabel(a.Label), a.Shortcut.String(), a.Enabled(), a.Checkable && a.Checked, a.OnInvoke, i)
	}
	for i := range w.Extra {
		it := &w.Extra[i]
		add(it.Label, it.Detail, it.OnInvoke != nil, false, it.OnInvoke, len(w.Actions)+i)
	}
	slices.SortStableFunc(s.shown, func(a, b paletteChoice) int {
		switch {
		case a.score != b.score:
			return b.score - a.score
		case a.recent != b.recent:
			return a.recent - b.recent
		}
		return a.order - b.order
	})
}

// settle keeps the selection on an enabled choice: the one it is on, or the
// next one after it, or the last one before it.
func (s *paletteBodyState) settle() {
	n := len(s.shown)
	s.selected = min(max(s.selected, 0), max(n-1, 0))
	if n == 0 || s.shown[s.selected].enabled {
		return
	}
	if next := s.step(s.selected, 1); next >= 0 {
		s.selected = next
	} else if prev := s.step(s.selected, -1); prev >= 0 {
		s.selected = prev
	}
}

// step returns the next enabled choice from i in direction dir, or -1.
func (s *paletteBodyState) step(i, dir int) int {
	for j := i + dir; j >= 0 && j < len(s.shown); j += dir {
		if s.shown[j].enabled {
			return j
		}
	}
	return -1
}

// composing reports whether an IME is converting text in the field.
func (s *paletteBodyState) composing() bool {
	r := FindEditable(s.Context().RenderObject())
	return r != nil && r.Composition != nil
}

// key moves the selection with the arrows, which the field leaves alone.
func (s *paletteBodyState) key(e *core.KeyEvent) bool {
	if s.composing() {
		// The keys are the IME's; Escape must not close the palette under it.
		return e.Key == input.KeyEscape
	}
	if e.Modifiers != 0 {
		return false
	}
	switch e.Key {
	case input.KeyDown:
		if next := s.step(s.selected, 1); next >= 0 {
			s.selected = next
			kit.Rebuild(&s.StateBase)
		}
	case input.KeyUp:
		if prev := s.step(s.selected, -1); prev >= 0 {
			s.selected = prev
			kit.Rebuild(&s.StateBase)
		}
	default:
		return false
	}
	return true
}

// submit runs the selected choice on Enter.
func (s *paletteBodyState) submit(string) {
	if s.composing() {
		return
	}
	s.run(s.selected)
}

// run closes the palette and runs choice i, if it is enabled.
func (s *paletteBodyState) run(i int) {
	if i < 0 || i >= len(s.shown) || !s.shown[i].enabled {
		return
	}
	c := s.shown[i]
	o := s.Widget().owner
	o.remember(c.label)
	if o.dismiss != nil {
		o.dismiss()
	}
	c.run()
}

// Build implements core.State.
func (s *paletteBodyState) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	words := basic.StringsOf(ctx)
	o := s.Widget().owner
	s.filter()
	s.settle()

	first := max(0, s.selected-paletteRows+1)
	rows := make([]core.Widget, 0, paletteRows+1)
	for i := first; i < len(s.shown) && i < first+paletteRows; i++ {
		rows = append(rows, s.row(th, i))
	}
	switch {
	case len(s.shown) == 0:
		rows = append(rows, basic.Padding{Insets: geom.Symmetric(theme.Space10, theme.Space8), Child: basic.Text{
			Content: words.NoResults, Style: kit.Colored(th.Text.Body, th.Color.Ink2),
		}})
	case len(s.shown) > paletteRows:
		rows = append(rows, basic.Padding{Insets: geom.Symmetric(theme.Space10, theme.Space4), Child: basic.Text{
			Content: words.MatchCount(s.selected+1, len(s.shown)), Style: kit.Colored(th.Text.Caption, th.Color.Ink2),
		}})
	}
	placeholder := o.Widget().Placeholder
	if placeholder == "" {
		placeholder = words.SearchCommands
	}
	return interact.Semantics{Role: semantics.RoleGroup, Label: words.CommandPalette, Child: interact.Focus{
		OnKey: s.key,
		Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: theme.Space8, Children: []core.Widget{
			Input{
				Value: s.query, Placeholder: placeholder, Controller: &s.field,
				OnChange: func(v string) { s.SetState(func() { s.query, s.selected = v, 0 }) },
				OnSubmit: s.submit,
			},
			basic.SizedBox{Height: paletteListHeight, Child: interact.Semantics{Role: semantics.RoleList, Child: basic.Column{
				Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: paletteRowGap, Children: rows,
			}}},
		}},
	}}
}

// row returns choice i, lit when it is the selected one.
func (s *paletteBodyState) row(th *theme.Theme, i int) core.Widget {
	c := s.shown[i]
	selected := i == s.selected
	ink, dim, mark := th.Color.Ink, th.Color.Ink2, th.Color.AccentInk
	if selected {
		// The selected row lies on the tint, where Ink2 and AccentInk fall
		// below 4.5:1.
		on := th.Color.OnTint()
		dim, mark = on.Ink2, on.AccentInk
	}
	if !c.enabled {
		ink, dim, mark = th.Color.Ink3, th.Color.Ink3, th.Color.Ink3
	}
	body := kit.Colored(th.Text.Body, ink)
	label := FuzzySpans(make([]text.Span, 0, c.rangeLen+1), c.label, s.ranges[c.rangeAt:c.rangeAt+c.rangeLen], body, FuzzyHighlight(body, mark))
	var state semantics.State
	if selected {
		state |= semantics.Selected
	}
	if c.checked {
		state |= semantics.Checked
	}
	var press func()
	if c.enabled {
		press = func() { s.run(i) }
	}
	return interact.Pressable{
		Label: c.label, Value: c.detail, Role: semantics.RoleListItem, State: state,
		OnPress: press,
		// The arrows move through the results; Tab stays with the field.
		SkipTab: true,
		Build: func(ps interact.PressState) core.Widget {
			var ground geom.Color
			switch {
			case selected:
				ground = th.Color.AccentTint
			case ps.Hovered:
				ground = th.Color.Fill
			}
			parts := make([]core.Widget, 0, 3)
			parts = append(parts, basic.Expanded{Child: basic.RichText{Spans: label, Style: kit.Colored(th.Text.Body, ink)}})
			if c.checked {
				parts = append(parts, basic.SizedBox{Width: paletteCheck, Height: paletteCheck, Child: kit.Checkmark{Color: mark, Progress: 1}})
			}
			if c.detail != "" {
				parts = append(parts, basic.Text{Content: c.detail, Style: kit.Colored(th.Text.Caption, dim)})
			}
			return basic.Container{
				Height: paletteRow, Color: ground, Radius: th.Radius.Control, Padding: geom.Symmetric(theme.Space10, 0),
				Child: basic.Row{Cross: layout.CrossCenter, Spacing: theme.Space12, Children: parts},
			}
		},
	}
}

// paletteLabel takes the mnemonic out of a menu label, with the parentheses
// that hold it in a Japanese one, and the ellipsis of a command that asks for
// more: "開く(&O)…" reads 開く.
func paletteLabel(label string) string {
	if i := strings.Index(label, "(&"); i >= 0 && i+4 <= len(label) && label[i+3] == ')' {
		label = label[:i] + label[i+4:]
	}
	label = kit.ParseMnemonic(label).Text
	label = strings.TrimSuffix(label, "…")
	return strings.TrimSuffix(label, "...")
}
