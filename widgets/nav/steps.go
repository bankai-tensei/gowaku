package nav

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/icons"
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
	// stepCircle is the size of the circle of a step, and stepCheck that of
	// the check in a done one.
	stepCircle = 24
	stepCheck  = 14
	// stepGap is the gap between the circle and the label, and around the
	// line to the next step.
	stepGap = theme.Space8
	// stepLine is the thickness of the line between steps, and stepMinLine
	// the shortest it gets.
	stepLine    = 2
	stepMinLine = theme.Space16
	// stepPillPad is the room at the ends of a step, inside its pill.
	stepPillPad = theme.Space4
)

// Steps shows where a flow of several steps is, such as a wizard or a
// checkout: a numbered circle and a label for each step, in a row, with lines
// between them. The steps before Current are done and show a check, the
// current one is filled with the accent and its label is bold, and the steps
// after it are quiet. It is sometimes called a stepper or a step indicator;
// control.NumberStepper is the minus and plus buttons of a number instead.
//
//	nav.Steps{Labels: []string{"Account", "Shipping", "Payment"}, Current: 1,
//		OnStepPress: func(i int) { s.goTo(i) }}
//
// A Current of len(Labels) shows every step done. OnStepPress, when set,
// makes each done step a button that goes back to it: a stop of Tab that Enter
// or Space presses. The steps ahead do nothing, since a flow is usually
// checked one step at a time.
//
// A screen reader finds a list named Label with an item for each step, which
// it hears as the label, its place ("2 of 3") and, for the current and the
// done steps, "Current step" or "Completed" in the language of the window.
// The row is 30 high and takes the whole width it is given, which must be
// bounded: the lines share what the steps leave.
type Steps struct {
	core.StatelessBase
	// Labels are the names of the steps, in order.
	Labels []string
	// Current is the step under way, from 0.
	Current int
	// OnStepPress gets the step, from 0, that a press on a done step goes
	// back to. Nil leaves the done steps as they are.
	OnStepPress func(step int)
	// Label names the list to assistive technology. Empty selects "Steps" in
	// the language of the window.
	Label string
}

// Build implements core.StatelessWidget.
func (s Steps) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	words := basic.StringsOf(ctx)
	label := s.Label
	if label == "" {
		label = words.Steps
	}
	n := len(s.Labels)
	children := make([]core.Widget, 0, max(2*n-1, 0))
	for i, l := range s.Labels {
		if i > 0 {
			color := th.Color.FillStrong
			if i <= s.Current {
				color = th.Color.Accent
			}
			children = append(children, basic.Expanded{Child: basic.Padding{
				Insets: geom.Symmetric(stepGap, 0),
				Child: basic.ConstrainedBox{MinWidth: stepMinLine, MaxWidth: geom.Inf, Child: basic.Container{
					Height: stepLine, Radius: stepLine / 2, Color: color,
				}},
			}})
		}
		item := stepItem{index: i, count: n, label: l, current: s.Current}
		if i < s.Current && s.OnStepPress != nil {
			press := s.OnStepPress
			item.onPress = func() { press(i) }
		}
		children = append(children, item)
	}
	return interact.Semantics{Role: semantics.RoleList, Label: label, Child: basic.ConstrainedBox{
		MinHeight: theme.ControlHeight, MaxWidth: geom.Inf, MaxHeight: geom.Inf,
		Child: basic.Row{Cross: layout.CrossCenter, Children: children},
	}}
}

// stepItem is one step of Steps: its circle and its label.
type stepItem struct {
	core.StatefulBase
	index, count, current int
	label                 string
	onPress               func()
}

type stepItemState struct {
	core.StateBase
	behavior.Control
	check *paint.Path
}

// CreateState implements core.StatefulWidget.
func (stepItem) CreateState() core.State { return &stepItemState{} }

// InitState implements core.State.
func (s *stepItemState) InitState() {
	s.Attach(&s.StateBase)
	s.check = icons.Check()
}

// Dispose implements core.State.
func (s *stepItemState) Dispose() { s.Control.Dispose() }

// Build implements core.State.
func (s *stepItemState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget().(stepItem)
	th := basic.ThemeOf(ctx)
	words := basic.StringsOf(ctx)
	c := &th.Color
	done, current := w.index < w.current, w.index == w.current
	ground, numberInk := c.Fill, c.Ink2
	style := kit.Colored(th.Text.Body, c.Ink2)
	var hint string
	switch {
	case done:
		ground, style.Color, hint = c.Accent, c.Ink, words.StepCompleted
	case current:
		ground, numberInk, hint = c.Accent, c.OnAccent, words.CurrentStep
		style = kit.Colored(th.Text.Headline, c.Ink)
	}
	var mark core.Widget
	if done {
		mark = basic.Icon{Path: s.check, Size: stepCheck, Color: c.OnAccent}
	} else {
		// The number is the place the item already says.
		mark = interact.ExcludeSemantics{Child: basic.Text{Content: kit.Itoa(w.index + 1), Style: kit.Colored(th.Text.Caption, numberInk).WithWeight(600)}}
	}
	face := basic.Row{Size: layout.MainMin, Cross: layout.CrossCenter, Spacing: stepGap, Children: []core.Widget{
		basic.Container{Width: stepCircle, Height: stepCircle, Radius: stepCircle / 2, Color: ground, Child: basic.Center{Child: mark}},
		basic.Text{Content: w.label, Style: style},
	}}
	// Every step has the room of the pill that a done step that goes back
	// shows under the pointer, so that the steps line up either way.
	pill := basic.Container{Radius: th.Radius.Control, Padding: geom.Symmetric(stepPillPad, theme.Space2), Child: face}
	sem := interact.Semantics{Role: semantics.RoleListItem, Label: w.label, Hint: hint, PosInSet: w.index + 1, SetSize: w.count}
	if w.onPress == nil {
		sem.Child = pill
		return sem
	}
	sink := geom.Clamp32(s.Sink.Value(), 0, 1)
	fill := c.Fill.WithAlpha(c.Fill.A * s.Hot.Get())
	pill.Color = theme.Over(fill, c.Ink, (kit.PressStrength-kit.HoverStrength)*sink)
	pill.Sink = kit.ButtonSink * sink
	sem.OnTap = w.onPress
	sem.Child = s.Focusable(w.onPress, interact.MouseRegion{
		Cursor: input.CursorHand,
		Child:  s.Ring(th.Radius.Control, c.AccentInk, pill),
	})
	return sem
}
