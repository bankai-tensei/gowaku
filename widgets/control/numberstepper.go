package control

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

const numberStepperValueWidth = 40

// NumberStepper changes a whole number by Step with a minus and a plus button,
// as NSStepper does. Min is the lowest value; Max is the highest when it is
// greater than Min, and there is no upper limit otherwise. A button that would
// cross a limit is disabled, and without OnChange both are. A screen reader
// finds a spin button. For the numbered steps of a wizard, sometimes also
// called a stepper, see nav.Steps.
type NumberStepper struct {
	core.StatelessBase
	Value, Min, Max int
	// Step is the amount a press moves the value by. Zero selects 1.
	Step int
	// Format spells the value. Nil prints the number alone, grouped as the
	// locale of the window does (FormatInt).
	Format func(int) string
	// Label describes the stepper to assistive technology.
	Label    string
	OnChange func(int)
}

func (s NumberStepper) step() int { return max(s.Step, 1) }

// clamp keeps v within the limits.
func (s NumberStepper) clamp(v int) int {
	v = max(v, s.Min)
	if s.Max > s.Min {
		v = min(v, s.Max)
	}
	return v
}

// Build implements core.StatelessWidget.
func (s NumberStepper) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	style := th.Text.Control
	style.Color = th.Color.Ink
	var label string
	if s.Format != nil {
		label = s.Format(s.Value)
	} else {
		label = basic.FormatInt(ctx, s.Value)
	}
	var down, up func()
	if s.OnChange != nil {
		if next := s.clamp(s.Value - s.step()); next != s.Value {
			down = func() { s.OnChange(next) }
		}
		if next := s.clamp(s.Value + s.step()); next != s.Value {
			up = func() { s.OnChange(next) }
		}
	}
	r := semantics.Range{Min: float64(s.Min), Max: float64(s.Value + s.step()), Value: float64(s.Value), Step: float64(s.step())}
	if s.Max > s.Min {
		r.Max = float64(s.Max)
	}
	var state semantics.State
	if s.OnChange == nil {
		state = semantics.Disabled
	}
	return interact.Semantics{Role: semantics.RoleStepper, Label: s.Label, Value: label, Range: r, State: state, OnIncrease: up, OnDecrease: down, Child: basic.Row{
		Size: layout.MainMin, Spacing: 4, Cross: layout.CrossCenter, Children: []core.Widget{
			Button{Label: "−", Size: ButtonSmall, OnPress: down},
			basic.ConstrainedBox{MinWidth: numberStepperValueWidth, Child: basic.Center{Child: basic.Text{Content: label, Style: style}}},
			Button{Label: "+", Size: ButtonSmall, OnPress: up},
		},
	}}
}
