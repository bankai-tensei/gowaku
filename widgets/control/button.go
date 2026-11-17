package control

import (
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/behavior"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
)

const buttonIconGap = theme.Space6

// ButtonStyle is the emphasis of a button.
//
// It is ButtonNormal (the zero value), ButtonPrimary, ButtonQuiet or
// ButtonDanger.
type ButtonStyle uint8

// The button styles.
const (
	// ButtonNormal is the default: a raised plain button.
	ButtonNormal ButtonStyle = iota
	// ButtonPrimary is filled with the accent. Use one per screen.
	ButtonPrimary
	// ButtonQuiet has no ground until the pointer is near.
	ButtonQuiet
	// ButtonDanger is a quiet button for what destroys something.
	ButtonDanger
)

// ButtonSize is the height of a button.
//
// It is ButtonRegular (the zero value, 30 high), ButtonSmall (24) or
// ButtonLarge (38).
type ButtonSize uint8

// The button sizes.
const (
	ButtonRegular ButtonSize = iota
	ButtonSmall
	ButtonLarge
)

func (s ButtonSize) metrics() (height, padding float32) {
	switch s {
	case ButtonSmall:
		return 24, theme.Space10
	case ButtonLarge:
		return 38, theme.Space20
	}
	return theme.ControlHeight, theme.Space16
}

// iconAlone is the size of the icon of a button that shows it alone: 16 on a
// regular button, as on the buttons of nav.Toolbar.
func (s ButtonSize) iconAlone() float32 {
	switch s {
	case ButtonSmall:
		return 14
	case ButtonLarge:
		return 20
	}
	return 16
}

// Button runs OnPress when clicked, or when Space or Enter is pressed while it has
// the focus. Without OnPress it is disabled. Icon, if set, is drawn before the
// label on a 24 unit grid.
//
// HideLabel makes an icon button: a square as high as the button with Icon
// alone in it, while Label still names the button to a screen reader. A
// ButtonQuiet one has the look of the buttons of nav.Toolbar: no ground until
// the pointer comes, and the icon in Ink2. Since the icon alone may not say
// enough to everyone, put the button in an overlay.Tooltip with the label:
//
//	overlay.Tooltip{Message: Copy, Child: control.Button{Label: Copy, Icon: icons.Copy(), HideLabel: true, Style: control.ButtonQuiet, OnPress: copyIt}}
type Button struct {
	core.StatefulBase
	Label string
	Icon  *paint.Path
	// HideLabel draws Icon alone, in a square; Label stays the name for a
	// screen reader. Without an Icon the label shows.
	HideLabel bool
	Style     ButtonStyle
	Size      ButtonSize
	OnPress   func()
	// Hint is what a screen reader says after the label, for a button whose
	// label alone is not enough, such as one 削除 on every row of a list.
	Hint string
}

type buttonState struct {
	core.StateBase
	behavior.Control
}

// CreateState implements core.StatefulWidget.
func (Button) CreateState() core.State { return &buttonState{} }

// InitState implements core.State.
func (s *buttonState) InitState() { s.Attach(&s.StateBase) }

// Dispose implements core.State.
func (s *buttonState) Dispose() { s.Control.Dispose() }

// buttonLook is what changes between the styles of a button.
type buttonLook struct {
	fill, content geom.Color
	// wash is what hover and press mix into the ground, and hover, if set,
	// the content under the pointer.
	wash, hover geom.Color
	border      paint.Stroke
	lift        theme.Lift
}

func lookOf(style ButtonStyle, enabled, iconAlone bool, c *theme.Colors) buttonLook {
	switch {
	case !enabled && (style == ButtonQuiet || style == ButtonDanger):
		return buttonLook{content: c.Ink3}
	case !enabled:
		return buttonLook{fill: c.Fill, content: c.Ink3}
	case style == ButtonQuiet && iconAlone:
		// As the buttons of a toolbar: a quiet icon that a hover darkens.
		return buttonLook{content: c.Ink2, wash: c.Ink, hover: c.Ink}
	case style == ButtonPrimary:
		return buttonLook{fill: c.Accent, content: c.OnAccent, wash: c.OnAccent, lift: theme.LiftControl}
	case style == ButtonQuiet:
		return buttonLook{content: c.AccentInk, wash: c.Ink}
	case style == ButtonDanger:
		return buttonLook{content: c.Danger, wash: c.Danger}
	}
	return buttonLook{
		fill: c.Control, content: c.Ink, wash: c.Ink, lift: theme.LiftControl,
		border: paint.Stroke{Width: 1, Color: c.Hair},
	}
}

// Build implements core.State.
func (s *buttonState) Build(ctx core.BuildContext) core.Widget {
	b := s.Widget().(Button)
	th := basic.ThemeOf(ctx)
	enabled := b.OnPress != nil
	alone := b.HideLabel && b.Icon != nil
	look := lookOf(b.Style, enabled, alone, &th.Color)
	sink := geom.Clamp32(s.Sink.Value(), 0, 1)

	fill := look.fill
	if amount := kit.HoverStrength*s.Hot.Get() + (kit.PressStrength-kit.HoverStrength)*sink; amount > 0 && look.wash.A > 0 {
		fill = theme.Over(fill, look.wash, amount)
	}
	ink := look.content
	if look.hover.A > 0 {
		ink = theme.Over(ink, look.hover, s.Hot.Get())
	}
	label := kit.Colored(th.Text.Control, ink)
	content := core.Widget(basic.Text{Content: b.Label, Style: label})
	height, padding := b.Size.metrics()
	if b.Style == ButtonQuiet || b.Style == ButtonDanger {
		// With no ground to show, the padding would read as a wider gap to whatever
		// stands next to the button.
		padding = theme.Space8
	}
	box := basic.Container{
		Color:   fill,
		Radius:  th.Radius.Control,
		Border:  look.border,
		Shadows: th.Color.Shadows(look.lift),
		Sink:    kit.ButtonSink * sink,
		Padding: geom.Symmetric(padding, max((height-kit.LabelHeight(&label))/2, 0)),
		Child:   content,
	}
	switch {
	case alone:
		box.Width, box.Height, box.Padding = height, height, geom.Insets{}
		box.Child = basic.Center{Child: basic.Icon{Path: b.Icon, Size: b.Size.iconAlone(), Color: ink}}
	case b.Icon != nil:
		box.Child = basic.Row{Size: layout.MainMin, Cross: layout.CrossCenter, Spacing: buttonIconGap, Children: []core.Widget{
			basic.Icon{Path: b.Icon, Size: label.Size + 2, Color: ink},
			content,
		}}
	}
	body := s.Ring(th.Radius.Control, th.Color.AccentInk, box)
	return s.Wrap(interact.Semantics{Role: semantics.RoleButton, Label: b.Label, Hint: b.Hint}, enabled, b.OnPress, body)
}

// Checkbox shows Value and asks for the opposite through OnChange. Without
// OnChange it is disabled.
type Checkbox struct {
	core.StatefulBase
	Value    bool
	Label    string
	OnChange func(bool)
}

const checkboxGap = theme.Space8

type checkboxState struct {
	core.StateBase
	behavior.Control
	check   motion.Implicit
	checked *motion.Value[float32]
}

// CreateState implements core.StatefulWidget.
func (Checkbox) CreateState() core.State { return &checkboxState{} }

// InitState implements core.State.
func (s *checkboxState) InitState() {
	s.Attach(&s.StateBase)
	s.check.Attach(&s.StateBase, func() { kit.Rebuild(&s.StateBase) })
	s.checked = motion.Track(&s.check, kit.OnOff(s.Widget().(Checkbox).Value), motion.LerpFloat)
}

// DidUpdateWidget implements core.State.
func (s *checkboxState) DidUpdateWidget(core.Widget) {
	s.checked.Set(kit.OnOff(s.Widget().(Checkbox).Value))
	s.check.Commit(kit.ToggleDuration, motion.Standard)
}

// Dispose implements core.State.
func (s *checkboxState) Dispose() {
	s.Control.Dispose()
	s.check.Dispose()
}

// Build implements core.State.
func (s *checkboxState) Build(ctx core.BuildContext) core.Widget {
	c := s.Widget().(Checkbox)
	th := basic.ThemeOf(ctx)
	kit.ReportField(ctx, kit.FieldCheck)
	enabled := c.OnChange != nil
	on := s.checked.Get()

	fill, line, ink := th.Color.Accent, th.Color.Ink3, th.Color.Ink
	if !enabled {
		fill, line, ink = th.Color.Ink3, th.Color.Hair, th.Color.Ink3
	}
	sink := geom.Clamp32(s.Sink.Value(), 0, 1)
	line = motion.LerpColor(line, th.Color.Ink2, s.Hot.Get())
	fill = theme.Over(fill, th.Color.OnAccent, kit.HoverStrength*s.Hot.Get()+(kit.PressStrength-kit.HoverStrength)*sink)
	box := basic.Container{
		Width: kit.CheckboxBox, Height: kit.CheckboxBox, Radius: th.Radius.Check,
		Color: fill.WithAlpha(fill.A * on),
		Sink:  kit.ButtonSink * 2 * sink,
		Child: kit.Checkmark{Color: th.Color.OnAccent, Progress: on},
	}
	if on < 1 {
		box.Border = paint.Stroke{Width: 1, Color: motion.LerpColor(line, fill, on)}
	}
	row := s.Ring(th.Radius.Check, th.Color.AccentInk, box)
	if c.Label != "" {
		row = basic.Row{Size: layout.MainMin, Spacing: checkboxGap, Cross: layout.CrossCenter, Children: []core.Widget{
			row, basic.Text{Content: c.Label, Style: kit.Colored(th.Text.Body, ink)},
		}}
	}
	value := "unchecked"
	if c.Value {
		value = "checked"
	}
	var state semantics.State
	if c.Value {
		state = semantics.Checked
	}
	press := func() { c.OnChange(!c.Value) }
	return s.Wrap(interact.Semantics{Role: semantics.RoleCheckbox, Label: c.Label, Value: value, State: state}, enabled, press, row)
}
