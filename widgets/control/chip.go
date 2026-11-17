package control

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/behavior"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

const (
	// chipHeight is the small control height (DESIGN §4), and the corners are
	// half of it: a chip is a pill.
	chipHeight  = 24
	chipPadding = theme.Space10
	chipIcon    = 14
	chipIconGap = theme.Space4
	// chipRemove is the round target of the remove button, and chipRemoveEnd
	// the room after it, which with the target's own margin matches the
	// padding at the start.
	chipRemove    = 16
	chipRemoveEnd = theme.Space4
	chipRemoveGap = theme.Space2
	// chipCross is the length of the arms of the cross, and chipCrossWidth
	// their width, on the 16 unit grid of the remove button.
	chipCross      = 6
	chipCrossWidth = 1.5
)

// Chip is a compact pill for a tag, a filter or a choice. It is 24 high, and
// comes in three kinds by what it is given:
//
//   - With OnChange it is a filter that toggles: Selected shows the accent tint,
//     and a press asks for the opposite through OnChange. Assistive technology
//     sees a checkbox.
//   - With OnPress it is a button, such as a suggested reply.
//   - With neither it is a plain tag that shows Label.
//
// OnRemove adds a × at the end that runs it, as for a tag in a field of
// recipients. Delete and Backspace run it too while the chip has the focus. The
// × of a toggling or pressable chip is for the pointer, since the keys do the
// same; that of a plain tag is a stop of Tab of its own. Either way assistive
// technology sees it as a separate button.
//
// Lay several out with a Wrap:
//
//	Wrap{Spacing: theme.Space8, RunSpacing: theme.Space8, Children: chips}
type Chip struct {
	core.StatefulBase
	Label string
	// Icon is drawn before the label, on a 24 unit grid.
	Icon *paint.Path
	// Selected is the state of a chip with OnChange.
	Selected bool
	// OnChange makes the chip toggle. It receives the state asked for.
	OnChange func(selected bool)
	// OnPress makes the chip a button. OnChange wins when both are set.
	OnPress func()
	// OnRemove shows the remove button.
	OnRemove func()
}

type chipState struct {
	core.StateBase
	behavior.Control
	// cross is the shape of the remove button, made once.
	cross *paint.Path
}

// CreateState implements core.StatefulWidget.
func (Chip) CreateState() core.State { return &chipState{} }

// InitState implements core.State.
func (s *chipState) InitState() {
	s.Attach(&s.StateBase)
	s.cross = kit.CrossPath(chipRemove, chipCross, chipCrossWidth)
}

// Dispose implements core.State.
func (s *chipState) Dispose() { s.Control.Dispose() }

// activation returns what a press of the chip does, or nil for a plain tag.
func (w *Chip) activation() func() {
	switch {
	case w.OnChange != nil:
		return func() { w.OnChange(!w.Selected) }
	case w.OnPress != nil:
		return w.OnPress
	}
	return nil
}

// Build implements core.State.
func (s *chipState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget().(Chip)
	th := basic.ThemeOf(ctx)
	if w.OnChange != nil {
		kit.ReportField(ctx, kit.FieldChoice)
	}
	c := &th.Color
	activate := w.activation()
	interactive := activate != nil
	toggles := w.OnChange != nil
	selected := toggles && w.Selected

	sink := geom.Clamp32(s.Sink.Value(), 0, 1)
	fill, ink, tint := c.Fill, c.Ink, c.Ink2
	if selected {
		// AccentInk falls below 4.5:1 on the tint under a hover; OnTint
		// deepens it by what it lacks.
		on := c.OnTint()
		fill, ink, tint = c.AccentTint, on.AccentInk, on.AccentInk
	}
	if interactive {
		fill = theme.Over(fill, c.Ink, kit.HoverStrength*s.Hot.Get()+(kit.PressStrength-kit.HoverStrength)*sink)
	}
	style := kit.Colored(th.Text.Control, ink)

	row := make([]core.Widget, 0, 3)
	if w.Icon != nil {
		row = append(row, basic.Icon{Path: w.Icon, Size: chipIcon, Color: tint})
	}
	row = append(row, basic.Text{Content: w.Label, Style: style})
	end := float32(chipPadding)
	if w.OnRemove != nil {
		row = append(row, s.removeButton(ctx, th, &w, tint, interactive))
		end = chipRemoveEnd
	}
	gap := float32(chipIconGap)
	if w.Icon == nil {
		gap = chipRemoveGap
	}
	start := float32(chipPadding)
	if w.Icon != nil {
		start = theme.Space8
	}
	body := basic.Container{
		Height: chipHeight, Radius: chipHeight / 2, Color: fill,
		Sink:    kit.ButtonSink * sink,
		Padding: geom.Insets{Left: start, Right: end},
		Child:   basic.Row{Size: layout.MainMin, Cross: layout.CrossCenter, Spacing: gap, Children: row},
	}
	if !interactive {
		if w.OnRemove == nil {
			return body
		}
		// A tag says its label as text; the remove button inside it is the
		// control.
		return interact.Semantics{Role: semantics.RoleGroup, Label: w.Label, Child: body}
	}

	sem := interact.Semantics{Role: semantics.RoleButton, Label: w.Label}
	if toggles {
		sem.Role, sem.Value = semantics.RoleCheckbox, "unchecked"
		if selected {
			sem.State, sem.Value = semantics.Checked, "checked"
		}
	}
	sem.OnTap = activate
	ring := s.Ring(chipHeight/2, c.AccentInk, body)
	sem.Child = interact.Focus{
		Enabled: true,
		OnKey: func(e *core.KeyEvent) bool {
			if removes(e) {
				if f := s.Widget().(Chip).OnRemove; f != nil {
					if !e.Repeat {
						f()
					}
					return true
				}
				return false
			}
			if !kit.Activates(e) {
				return false
			}
			if !e.Repeat {
				s.Sink.Kick(kit.KeyKick)
				activate()
			}
			return true
		},
		OnFocusChange: s.Focus,
		OnShowFocus:   s.ShowFocus,
		Child: interact.MouseRegion{
			OnEnter: func() { s.Hover(true) },
			OnExit:  func() { s.Hover(false) },
			Child: interact.GestureDetector{
				OnTapDown:   s.Press,
				OnTapCancel: s.Release,
				OnTap: func() {
					s.Release()
					activate()
				},
				Child: ring,
			},
		},
	}
	return sem
}

// removes reports whether e is a key that removes the focused chip.
func removes(e *core.KeyEvent) bool {
	return e.Modifiers == 0 && (e.Key == input.KeyDelete || e.Key == input.KeyBackspace)
}

// removeButton returns the × of a chip. It is a stop of Tab only on a plain
// tag, whose body cannot take the focus.
func (s *chipState) removeButton(ctx core.BuildContext, th *theme.Theme, w *Chip, tint geom.Color, interactive bool) core.Widget {
	c := &th.Color
	remove := w.OnRemove
	return removeKeys{remove: remove, child: interact.Pressable{
		Label:   basic.StringsOf(ctx).RemoveNamed(w.Label),
		OnPress: remove,
		SkipTab: interactive,
		Build: func(p interact.PressState) core.Widget {
			var ground geom.Color
			switch {
			case p.Pressed:
				ground = c.FillStrong
			case p.Hovered:
				ground = c.Fill
			}
			color := tint
			if p.Hovered || p.Pressed {
				color = c.Ink
			}
			return interact.FocusRing{Visible: p.FocusVisible, Radius: chipRemove / 2, Child: basic.Container{
				Width: chipRemove, Height: chipRemove, Radius: chipRemove / 2, Color: ground,
				Child: basic.Icon{Path: s.cross, ViewBox: chipRemove, Size: chipRemove, Color: color},
			}}
		},
	}}
}

// removeKeys lets the remove button of a plain tag, which has the focus of its
// own, take Delete and Backspace as well as Space and Enter.
type removeKeys struct {
	core.StatelessBase
	remove func()
	child  core.Widget
}

// Build implements core.StatelessWidget.
func (r removeKeys) Build(core.BuildContext) core.Widget {
	return keyHandler{onKey: func(e *core.KeyEvent) bool {
		if !removes(e) {
			return false
		}
		if !e.Repeat {
			r.remove()
		}
		return true
	}, child: r.child}
}

// keyHandler sees the keys that reach it from the focus below it, without
// taking the focus itself.
type keyHandler struct {
	onKey func(e *core.KeyEvent) bool
	child core.Widget
}

// Key implements core.Widget.
func (keyHandler) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (k keyHandler) CreateRenderObject() core.RenderObject {
	return &layout.RenderFocus{OnKey: k.onKey}
}

// UpdateRenderObject implements core.RenderWidget.
func (k keyHandler) UpdateRenderObject(ro core.RenderObject) {
	ro.(*layout.RenderFocus).OnKey = k.onKey
}

// ChildWidget implements core.SingleChildWidget.
func (k keyHandler) ChildWidget() core.Widget { return k.child }
