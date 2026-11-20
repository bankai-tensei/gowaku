package collection

import (
	"strconv"
	"strings"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// propertySwatch is the side of the square of a color property.
const propertySwatch = 16

// PropertyEditor is the value of a Property together with the control that
// edits it. It is one of PropertyText, PropertyNumber, PropertyInt,
// PropertyBool, PropertyChoice, PropertyColor and PropertyCustom; the last holds
// a control of your own. Each takes its value and a callback of its own type, so
// the compiler checks what the caller hands over, without reflection.
type PropertyEditor interface {
	// editor returns the control. It is not exported, so that the kinds stay
	// those of this package; PropertyCustom is the way out.
	editor() core.Widget
}

// PropertyText is a property of text, in a text field. Without OnChange it is
// read-only text.
type PropertyText struct {
	Value    string
	OnChange func(string)
}

func (p PropertyText) editor() core.Widget {
	if p.OnChange == nil {
		return propertyReadOnly{text: p.Value}
	}
	return field.Input{Value: p.Value, OnChange: p.OnChange}
}

// PropertyNumber is a property of a number with a fraction, in a text field. The
// value changes when Enter is pressed or the field is left, and what is not a
// number then goes back to the value. Min and Max limit it when Max is greater
// than Min. Without OnChange it is read-only text.
type PropertyNumber struct {
	Value, Min, Max float64
	// Format spells the value. Nil writes the shortest form that reads back the
	// same.
	Format   func(float64) string
	OnChange func(float64)
}

func (p PropertyNumber) format() string {
	if p.Format != nil {
		return p.Format(p.Value)
	}
	return strconv.FormatFloat(p.Value, 'g', -1, 64)
}

func (p PropertyNumber) editor() core.Widget {
	if p.OnChange == nil {
		return propertyReadOnly{text: p.format()}
	}
	return numberField{number: p}
}

// PropertyInt is a property of a whole number, in a control.NumberStepper.
// Without OnChange the stepper is disabled.
type PropertyInt struct {
	Value, Min, Max int
	// Step is the amount a press moves the value by. Zero selects 1.
	Step     int
	Format   func(int) string
	OnChange func(int)
}

func (p PropertyInt) editor() core.Widget {
	return control.NumberStepper{Value: p.Value, Min: p.Min, Max: p.Max, Step: p.Step, Format: p.Format, OnChange: p.OnChange}
}

// PropertyBool is a property that is on or off, in a Checkbox. Without OnChange
// the checkbox is disabled.
type PropertyBool struct {
	Value    bool
	OnChange func(bool)
}

func (p PropertyBool) editor() core.Widget {
	return control.Checkbox{Value: p.Value, OnChange: p.OnChange}
}

// PropertyChoice is a property with a value from a list, in a DropdownMenu.
// Without OnChange the menu is disabled.
type PropertyChoice[T comparable] struct {
	Value    T
	Options  []field.DropdownOption[T]
	OnChange func(T)
}

func (p PropertyChoice[T]) editor() core.Widget {
	return field.DropdownMenu[T]{Value: p.Value, Options: p.Options, OnChange: p.OnChange}
}

// PropertyColor is a property of a color, shown as a square of it and its hex
// code. OnPress, when set, makes them a button, which opens a color picker of
// your own.
//
// Without OnPress, OnChange makes them a button that opens a field.ColorPicker
// in a popup below the row. The picker reports every change at once; Escape
// goes back to the color it opened with and closes it, and a click beside it
// or Enter keeps the new color. Neither set leaves the property read-only.
type PropertyColor struct {
	Value   geom.Color
	OnPress func()
	// OnChange, without OnPress, opens the standard picker.
	OnChange func(geom.Color)
}

func (p PropertyColor) editor() core.Widget {
	if p.OnPress == nil && p.OnChange != nil {
		return colorProperty{color: p}
	}
	return colorSwatch(p)
}

// PropertyCustom holds a control of your own. Like the others, it is named to
// assistive technology by the name of its property, unless it names itself.
type PropertyCustom struct {
	Child core.Widget
}

func (p PropertyCustom) editor() core.Widget {
	if p.Child == nil {
		return basic.SizedBox{}
	}
	return p.Child
}

// hexOf spells c as #rrggbb, with the alpha after it when it is not opaque.
func hexOf(c geom.Color) string {
	const digits = "0123456789ABCDEF"
	b := []byte{'#'}
	put := func(v float32) {
		n := int(geom.Clamp32(v, 0, 1)*255 + 0.5)
		b = append(b, digits[n>>4], digits[n&15])
	}
	put(c.R)
	put(c.G)
	put(c.B)
	if c.A < 1 {
		put(c.A)
	}
	return string(b)
}

// propertyReadOnly is the text of a read-only property, which a screen reader
// reads as text with the name of the property.
type propertyReadOnly struct {
	core.StatelessBase
	text string
}

// Build implements core.StatelessWidget.
func (p propertyReadOnly) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	return interact.Semantics{Role: semantics.RoleText, Value: p.text, State: semantics.ReadOnly, Child: silent{child: basic.Padding{
		Insets: geom.Insets{Left: theme.Space8}, Child: basic.Text{Content: p.text, Style: kit.Colored(th.Text.Body, th.Color.Ink)},
	}}}
}

// colorSwatch shows a color property.
type colorSwatch PropertyColor

// Key implements core.Widget.
func (colorSwatch) Key() core.Key { return nil }

// Build implements core.StatelessWidget.
func (p colorSwatch) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	hex := hexOf(p.Value)
	face := func(ink geom.Color) core.Widget {
		return basic.Row{Size: layout.MainMin, Cross: layout.CrossCenter, Spacing: theme.Space8, Children: []core.Widget{
			basic.Container{
				Width: propertySwatch, Height: propertySwatch, Radius: th.Radius.Check, Color: p.Value,
				Border: paint.Stroke{Width: 1, Color: th.Color.Hair},
			},
			basic.Text{Content: hex, Style: kit.Colored(th.Text.Body, ink)},
		}}
	}
	if p.OnPress == nil {
		return interact.Semantics{Role: semantics.RoleText, Value: hex, State: semantics.ReadOnly, Child: silent{child: basic.Padding{
			Insets: geom.Insets{Left: theme.Space8}, Child: face(th.Color.Ink),
		}}}
	}
	return interact.Pressable{Value: hex, OnPress: p.OnPress, Build: func(s interact.PressState) core.Widget {
		c := &th.Color
		fill := c.Fill.WithAlpha(c.Fill.A * kit.OnOff(s.Hovered))
		if s.Pressed {
			fill = c.FillStrong
		}
		return interact.FocusRing{Visible: s.FocusVisible, Radius: th.Radius.Control, Child: silent{child: basic.Container{
			Height: theme.ControlHeight, Radius: th.Radius.Control, Color: fill,
			Padding: geom.Symmetric(theme.Space8, 0), Child: face(c.Ink),
		}}}
	}}
}

// numberField is the text field of a PropertyNumber, which changes the value
// only when Enter is pressed or the field is left.
type numberField struct {
	core.StatefulBase
	number PropertyNumber
}

type numberFieldState struct {
	core.StateOf[numberField]
	// draft is the text typed since the value last changed, and dirty says
	// there is one.
	draft string
	dirty bool
	// rev starts the field again, to put back the value over text that is not
	// a number.
	rev int
}

// CreateState implements core.StatefulWidget.
func (numberField) CreateState() core.State { return &numberFieldState{} }

// commit turns the draft into the value. Text that is no number goes back to
// the value when the field is left.
func (s *numberFieldState) commit(leaving bool) {
	if !s.dirty {
		return
	}
	p := s.Widget().number
	v, err := strconv.ParseFloat(strings.TrimSpace(s.draft), 64)
	if err != nil {
		if leaving {
			s.dirty = false
			s.rev++
			kit.Rebuild(&s.StateBase)
		}
		return
	}
	s.dirty = false
	if p.Max > p.Min {
		v = min(max(v, p.Min), p.Max)
	}
	if v != p.Value {
		p.OnChange(v)
	}
	if leaving {
		// The field shows the value as Format spells it. It starts again
		// only when it is left, since that takes the focus.
		s.rev++
		kit.Rebuild(&s.StateBase)
	}
}

// Build implements core.State.
func (s *numberFieldState) Build(core.BuildContext) core.Widget {
	p := s.Widget().number
	return core.Keyed(s.rev, field.Input{
		Value: p.format(),
		OnChange: func(t string) {
			s.draft, s.dirty = t, true
		},
		OnSubmit: func(string) { s.commit(false) },
		OnFocus: func(focused bool) {
			if !focused {
				s.commit(true)
			}
		},
	})
}

// silent hides its child from assistive technology, for what a node around it
// already says.
type silent struct {
	child core.Widget
}

// Key implements core.Widget.
func (silent) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (silent) CreateRenderObject() core.RenderObject { return &renderSilent{} }

// UpdateRenderObject implements core.RenderWidget.
func (silent) UpdateRenderObject(core.RenderObject) {}

// ChildWidget implements core.SingleChildWidget.
func (s silent) ChildWidget() core.Widget { return s.child }

type renderSilent struct{ layout.RenderProxy }

// Semantics implements core.RenderObject.
func (*renderSilent) Semantics(*core.SemanticsBuilder) {}
