package field

import (
	"math"
	"strconv"
	"strings"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// The measures and steps of a color picker.
const (
	// colorPickerWidth is the width of a picker that sets none.
	colorPickerWidth = 240
	// colorPlaneShare is the height of the plane as a share of its width.
	colorPlaneShare = 0.66
	// colorPickerGap is the room between the parts.
	colorPickerGap = theme.Space12
	// colorPreviewWidth is the width of the preview of the old and the new
	// color, beside the strips; its height is that of the two strips.
	colorPreviewWidth = 44
	// colorSwatchSide is the side of a color of the palette.
	colorSwatchSide = 20
	// colorStep and colorBigStep are how far an arrow key moves a share, and
	// with Shift.
	colorStep    = 0.01
	colorBigStep = 0.1
	// hueStep and hueBigStep are the same for the hue, in degrees.
	hueStep    = 1
	hueBigStep = 10
)

// The parts of a picker that take the keyboard.
const (
	pickPlane = iota
	pickHue
	pickAlpha
	pickParts
)

// ColorPicker chooses a color: a square of saturation and brightness for one
// hue, a strip of hue, a strip of opacity over a checkerboard, a field for the
// hex code, and a preview that shows the color it started with beside the new
// one. Numeric adds number fields for red, green and blue, and for hue,
// saturation and brightness; Swatches adds a palette to pick from.
//
// The pointer drags on the square and the strips. Tab moves through the parts;
// on the square the arrow keys move saturation (left, right) and brightness
// (up, down) by 1%, and with Shift by 10%; on a strip they move the hue by 1°
// or 10°, or the opacity by 1% or 10%, and Home and End go to the ends. The hex
// field takes #RGB, #RGBA, #RRGGBB and #RRGGBBAA, and forgives what a paste
// brings along (see theme.ParseHex). A press on the old color goes back to it.
//
// The picker works in HSV and keeps its hue and saturation while they cannot
// be seen, so dragging to black or to gray and back does not lose the hue. It
// calls OnChange with every new color and follows a Value that comes from
// elsewhere. Without OnChange it only shows the color.
//
// A screen reader sees a group named Label (Choose color without one); the
// square is a slider whose value says both saturation and brightness, and
// the strips are sliders of degrees and percent.
//
// The picker is as wide as Width, which defaults to 240, and fits inline as
// well as in a popup; PropertyColor of package collection opens one.
type ColorPicker struct {
	core.StatefulBase
	Value geom.Color
	// Opaque leaves out the strip of opacity, and makes every color it
	// reports opaque.
	Opaque bool
	// Numeric adds number fields for the channels.
	Numeric bool
	// Swatches is a palette of colors to pick with one press.
	Swatches []geom.Color
	// Width is the width of the picker. Zero selects 240.
	Width float32
	// Autofocus gives the square the keyboard when the picker appears, as in
	// a popup.
	Autofocus bool
	// Label names the picker to assistive technology.
	Label    string
	OnChange func(geom.Color)
}

type colorPickerState struct {
	core.StateOf[ColorPicker]
	// hsv is the color shown. It is the truth, and Value only its result:
	// going through RGB would lose the hue of a gray.
	hsv theme.HSV
	// emitted is the last color reported, which a parent hands back as Value.
	emitted geom.Color
	// original is the color the picker started with.
	original geom.Color
	// hexDraft is the text of the hex field while it has the keyboard.
	hexDraft   string
	hexEditing bool
	// lit says which part shows the focus ring.
	lit [pickParts]bool
}

// CreateState implements core.StatefulWidget.
func (ColorPicker) CreateState() core.State { return &colorPickerState{} }

// InitState implements core.State.
func (s *colorPickerState) InitState() {
	w := s.Widget()
	s.hsv = theme.HSVFromColor(w.Value)
	s.emitted, s.original = w.Value, w.Value
}

// DidUpdateWidget implements core.State.
func (s *colorPickerState) DidUpdateWidget(core.Widget) {
	if v := s.Widget().Value; v != s.emitted {
		s.hsv = theme.HSVFromColorLike(v, s.hsv)
		s.emitted = v
	}
}

// color returns the color of the HSV state, as the picker reports it.
func (s *colorPickerState) color() geom.Color {
	c := s.hsv.Color()
	if s.Widget().Opaque {
		c.A = 1
	}
	return c
}

// set makes h the color and reports it.
func (s *colorPickerState) set(h theme.HSV) {
	h.H = geom.Clamp32(h.H, 0, 360)
	h.S, h.V, h.A = geom.Clamp32(h.S, 0, 1), geom.Clamp32(h.V, 0, 1), geom.Clamp32(h.A, 0, 1)
	if s.Widget().Opaque {
		h.A = 1
	}
	if h == s.hsv {
		return
	}
	s.hsv = h
	s.emitted = s.color()
	kit.Rebuild(&s.StateBase)
	if f := s.Widget().OnChange; f != nil {
		f(s.emitted)
	}
}

// setColor makes c the color, keeping the hue and saturation it leaves open.
func (s *colorPickerState) setColor(c geom.Color) {
	s.set(theme.HSVFromColorLike(c, s.hsv))
}

// step returns the step of an arrow key with the modifiers of e.
func step(e *core.KeyEvent, small, big float32) float32 {
	if e.Modifiers&input.ModShift != 0 {
		return big
	}
	return small
}

func (s *colorPickerState) planeKey(e *core.KeyEvent) bool {
	h := s.hsv
	d := step(e, colorStep, colorBigStep)
	switch e.Key {
	case input.KeyLeft:
		h.S -= d
	case input.KeyRight:
		h.S += d
	case input.KeyUp:
		h.V += d
	case input.KeyDown:
		h.V -= d
	case input.KeyPageUp:
		h.V += colorBigStep
	case input.KeyPageDown:
		h.V -= colorBigStep
	case input.KeyHome:
		h.S = 0
	case input.KeyEnd:
		h.S = 1
	default:
		return false
	}
	s.set(h)
	return true
}

func (s *colorPickerState) hueKey(e *core.KeyEvent) bool {
	h := s.hsv
	switch e.Key {
	case input.KeyLeft, input.KeyDown:
		h.H -= step(e, hueStep, hueBigStep)
	case input.KeyRight, input.KeyUp:
		h.H += step(e, hueStep, hueBigStep)
	case input.KeyHome:
		h.H = 0
	case input.KeyEnd:
		h.H = 360
	default:
		return false
	}
	s.set(h)
	return true
}

func (s *colorPickerState) alphaKey(e *core.KeyEvent) bool {
	h := s.hsv
	switch e.Key {
	case input.KeyLeft, input.KeyDown:
		h.A -= step(e, colorStep, colorBigStep)
	case input.KeyRight, input.KeyUp:
		h.A += step(e, colorStep, colorBigStep)
	case input.KeyHome:
		h.A = 0
	case input.KeyEnd:
		h.A = 1
	default:
		return false
	}
	s.set(h)
	return true
}

// light shows or hides the focus ring of part.
func (s *colorPickerState) light(part int) func(bool) {
	return func(show bool) {
		if s.lit[part] != show {
			s.lit[part] = show
			kit.Rebuild(&s.StateBase)
		}
	}
}

// hexChange follows the text of the hex field, taking the color as soon as it
// reads as one.
func (s *colorPickerState) hexChange(text string) {
	s.hexDraft, s.hexEditing = text, true
	if c, ok := theme.ParseHex(text); ok {
		s.setColor(c)
	}
	kit.Rebuild(&s.StateBase)
}

// hexDone ends an edit of the hex field: the field shows the code of the
// color again, which puts back the color over text that is no code.
func (s *colorPickerState) hexDone() {
	if !s.hexEditing {
		return
	}
	s.hexEditing = false
	if c, ok := theme.ParseHex(s.hexDraft); ok {
		s.setColor(c)
	}
	kit.Rebuild(&s.StateBase)
}

// Build implements core.State.
func (s *colorPickerState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	th := basic.ThemeOf(ctx)
	l := basic.StringsOf(ctx)
	c := &th.Color
	enabled := w.OnChange != nil
	width := w.Width
	if width <= 0 {
		width = colorPickerWidth
	}
	current := s.color()
	// In a FormItem, the square and the hex field show the error, and the
	// group says it; the fields inside neither check nor submit the form.
	kit.ReportField(ctx, kit.FieldChoice)
	scope := fieldScopeOf(ctx)
	look := func(part int) thumbLook {
		t := thumbLook{shadows: th.Color.Shadows(theme.LiftKnob), edge: paint.Stroke{Width: 0.5, Color: c.Hair}}
		if s.lit[part] {
			t.ring = paint.Stroke{Width: pickerRingWidth, Color: c.AccentInk.WithAlpha(kit.RingOpacity)}
		}
		return t
	}
	border := paint.Stroke{Width: 1, Color: c.Hair}
	planeBorder := border
	if scope.err != "" {
		planeBorder.Color = c.Danger
	}
	plane := colorPlaneView{hue: s.hsv.H, sat: s.hsv.S, val: s.hsv.V, height: geom.Floor32(width * colorPlaneShare),
		radius: th.Radius.Control, border: planeBorder, look: look(pickPlane)}
	hue := colorStripView{value: s.hsv.H / 360, color: current, border: border, look: look(pickHue)}
	alpha := colorStripView{alpha: true, value: s.hsv.A, color: current, checker: checkerOf(th), border: border, look: look(pickAlpha)}
	if enabled {
		plane.onDrag = func(sat, val float32) { h := s.hsv; h.S, h.V = sat, val; s.set(h) }
		hue.onDrag = func(f float32) { h := s.hsv; h.H = f * 360; s.set(h) }
		alpha.onDrag = func(f float32) { h := s.hsv; h.A = f; s.set(h) }
	}

	pct := func(v float32) string { return strconv.Itoa(int(geom.Clamp32(v, 0, 1)*100+0.5)) + "%" }
	planeSem := interact.Semantics{
		Role: semantics.RoleSlider, Label: l.ColorPlane,
		Value: l.Saturation + " " + pct(s.hsv.S) + ", " + l.Brightness + " " + pct(s.hsv.V),
	}
	hueSem := interact.Semantics{
		Role: semantics.RoleSlider, Label: l.Hue, Value: strconv.Itoa(int(s.hsv.H+0.5)) + "°",
		Range: semantics.Range{Max: 360, Value: math.Round(float64(s.hsv.H)), Step: hueStep},
	}
	alphaSem := interact.Semantics{Role: semantics.RoleSlider, Label: l.Opacity, Value: pct(s.hsv.A), Range: kit.PercentRange(s.hsv.A)}
	alphaSem.Range.Step = colorStep * 100
	var parts [pickParts]core.Widget
	parts[pickPlane], parts[pickHue], parts[pickAlpha] = plane, hue, alpha
	sems := [pickParts]*interact.Semantics{&planeSem, &hueSem, &alphaSem}
	keys := [pickParts]func(*core.KeyEvent) bool{s.planeKey, s.hueKey, s.alphaKey}
	for i, sem := range sems {
		if !enabled {
			sem.State |= semantics.Disabled
			sem.Child = parts[i]
			continue
		}
		key := keys[i]
		sem.OnIncrease = func() { key(&core.KeyEvent{Key: input.KeyRight}) }
		sem.OnDecrease = func() { key(&core.KeyEvent{Key: input.KeyLeft}) }
		sem.Child = interact.Focus{
			Enabled: true, Autofocus: i == pickPlane && w.Autofocus,
			OnKey: key, OnShowFocus: s.light(i),
			Child: interact.MouseRegion{Cursor: input.CursorHand, Child: parts[i]},
		}
	}
	if enabled {
		hueSem.OnSetValue = func(_ string, v float64) { h := s.hsv; h.H = float32(v); s.set(h) }
		alphaSem.OnSetValue = func(_ string, v float64) { h := s.hsv; h.A = float32(v / 100); s.set(h) }
	}

	strips := []core.Widget{hueSem}
	if !w.Opaque {
		strips = append(strips, alphaSem)
	}
	stripsHeight := float32(len(strips))*pickerStrip + float32(len(strips)-1)*theme.Space6
	// The preview comes after the strips, so that Tab goes from the square
	// through the strips first.
	row := basic.Row{Cross: layout.CrossCenter, Spacing: colorPickerGap, Children: []core.Widget{
		basic.Expanded{Child: basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: theme.Space6, Children: strips}},
		s.preview(th, l, current, stripsHeight, enabled),
	}}
	children := []core.Widget{planeSem, row, core.Inherited[fieldScope]{Value: scope.errorOnly(), Child: s.hexField(th, l, current, enabled)}}
	if w.Numeric {
		children = append(children, s.numbers(l, current, enabled)...)
	}
	if len(w.Swatches) > 0 {
		children = append(children, s.palette(th, current, enabled))
	}
	label := w.Label
	if label == "" {
		label = l.ChooseColor
	}
	return interact.Semantics{Role: semantics.RoleGroup, Label: label, Hint: scope.err, State: scope.state(), Child: basic.SizedBox{
		Width: width, Child: core.Inherited[fieldScope]{Value: fieldScope{}, Child: basic.Column{
			Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: colorPickerGap, Children: children,
		}},
	}}
}

// preview shows the color the picker started with beside the new one. A
// press on the old one goes back to it.
func (s *colorPickerState) preview(th *theme.Theme, l *basic.Strings, current geom.Color, height float32, enabled bool) core.Widget {
	r := th.Radius.Control
	half := float32(colorPreviewWidth / 2)
	border := paint.Stroke{Width: 1, Color: th.Color.Hair}
	before := colorSwatchView{color: s.original, width: half, height: height, corners: [4]float32{r, 0, 0, r}, checker: checkerOf(th), border: border}
	after := colorSwatchView{color: current, width: half, height: height, corners: [4]float32{0, r, r, 0}, checker: checkerOf(th), border: border}
	var old core.Widget = before
	if enabled {
		original := s.original
		old = interact.Pressable{
			Label: l.OriginalColor, Value: theme.FormatHex(original),
			OnPress: func() { s.setColor(original) },
			Build: func(p interact.PressState) core.Widget {
				return interact.FocusRing{Visible: p.FocusVisible, Radius: r, Child: before}
			},
		}
	}
	return basic.Row{Size: layout.MainMin, Children: []core.Widget{old, silentBox{child: after}}}
}

// hexField is the field of the hex code. While it has the keyboard it keeps
// what is typed; afterwards it shows the code of the color.
func (s *colorPickerState) hexField(th *theme.Theme, l *basic.Strings, current geom.Color, enabled bool) core.Widget {
	code := theme.FormatHex(current)
	if !enabled {
		return interact.Semantics{Role: semantics.RoleText, Label: l.HexCode, Value: code, State: semantics.ReadOnly, Child: silentBox{child: basic.Text{
			Content: code, Style: kit.Colored(th.Text.Body, th.Color.Ink),
		}}}
	}
	value := code
	if s.hexEditing {
		value = s.hexDraft
	}
	return nameField{name: l.HexCode, child: Input{
		Value:    value,
		OnChange: s.hexChange,
		OnSubmit: func(string) { s.hexDone() },
		Extras: Extras{
			leading: fieldTag("HEX", th),
			OnFocus: func(focused bool) {
				if focused {
					s.hexDraft, s.hexEditing = value, true
					return
				}
				s.hexDone()
			},
		},
	}}
}

// numbers returns the rows of number fields: red, green and blue from 0 to
// 255, and hue, saturation and brightness in degrees and percent.
func (s *colorPickerState) numbers(l *basic.Strings, current geom.Color, enabled bool) []core.Widget {
	byte8 := func(v float32) int { return int(geom.Clamp32(v, 0, 1)*255 + 0.5) }
	rgb := func(set func(c *geom.Color, v float32)) func(int) {
		return func(n int) {
			c := s.color()
			set(&c, float32(min(max(n, 0), 255))/255)
			s.setColor(c)
		}
	}
	hsv := func(set func(h *theme.HSV, v float32)) func(int) {
		return func(n int) {
			h := s.hsv
			set(&h, float32(n))
			s.set(h)
		}
	}
	field := func(short, name string, value, top int, commit func(int)) core.Widget {
		if !enabled {
			commit = nil
		}
		return basic.Expanded{Child: colorNumber{short: short, name: name, value: value, top: top, onCommit: commit}}
	}
	return []core.Widget{
		basic.Row{Spacing: theme.Space8, Children: []core.Widget{
			field("R", l.Red, byte8(current.R), 255, rgb(func(c *geom.Color, v float32) { c.R = v })),
			field("G", l.Green, byte8(current.G), 255, rgb(func(c *geom.Color, v float32) { c.G = v })),
			field("B", l.Blue, byte8(current.B), 255, rgb(func(c *geom.Color, v float32) { c.B = v })),
		}},
		basic.Row{Spacing: theme.Space8, Children: []core.Widget{
			field("H", l.Hue, int(s.hsv.H+0.5), 360, hsv(func(h *theme.HSV, v float32) { h.H = v })),
			field("S", l.Saturation, int(s.hsv.S*100+0.5), 100, hsv(func(h *theme.HSV, v float32) { h.S = v / 100 })),
			field("B", l.Brightness, int(s.hsv.V*100+0.5), 100, hsv(func(h *theme.HSV, v float32) { h.V = v / 100 })),
		}},
	}
}

// palette returns the swatches to pick from. The one of the current color is
// selected.
func (s *colorPickerState) palette(th *theme.Theme, current geom.Color, enabled bool) core.Widget {
	w := s.Widget()
	r := th.Radius.Check + 1
	chips := make([]core.Widget, len(w.Swatches))
	for i, sw := range w.Swatches {
		box := colorSwatchView{
			color: sw, width: colorSwatchSide, height: colorSwatchSide, corners: [4]float32{r, r, r, r},
			checker: checkerOf(th), border: paint.Stroke{Width: 1, Color: th.Color.Hair},
		}
		var press func()
		if enabled {
			press = func() { s.setColor(sw) }
		}
		var state semantics.State
		chosen := theme.FormatHex(sw) == theme.FormatHex(current)
		if chosen {
			state = semantics.Selected
		}
		chips[i] = interact.Pressable{
			Label: theme.FormatHex(sw), State: state, OnPress: press,
			Build: func(p interact.PressState) core.Widget {
				return interact.FocusRing{Visible: p.FocusVisible || chosen, Radius: r, Child: box}
			},
		}
	}
	return basic.Wrap{Spacing: theme.Space8, RunSpacing: theme.Space8, Children: chips}
}

// colorNumber is a number field of a channel, which takes the number when
// Enter is pressed or the field is left.
type colorNumber struct {
	core.StatefulBase
	// short is the letter shown in the field, and name what a screen reader
	// hears.
	short, name string
	value, top  int
	onCommit    func(int)
}

type colorNumberState struct {
	core.StateOf[colorNumber]
	draft   string
	editing bool
}

// CreateState implements core.StatefulWidget.
func (colorNumber) CreateState() core.State { return &colorNumberState{} }

func (s *colorNumberState) commit() {
	if !s.editing {
		return
	}
	s.editing = false
	w := s.Widget()
	if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimRight(strings.TrimSpace(s.draft), "%°"))); err == nil && w.onCommit != nil {
		w.onCommit(min(max(n, 0), w.top))
	}
	kit.Rebuild(&s.StateBase)
}

// Build implements core.State.
func (s *colorNumberState) Build(ctx core.BuildContext) core.Widget {
	w := s.Widget()
	th := basic.ThemeOf(ctx)
	shown := strconv.Itoa(w.value)
	if w.onCommit == nil {
		return interact.Semantics{Role: semantics.RoleText, Label: w.name, Value: shown, State: semantics.ReadOnly, Child: silentBox{child: basic.Text{
			Content: w.short + " " + shown, Style: kit.Colored(th.Text.Body, th.Color.Ink2),
		}}}
	}
	value := shown
	if s.editing {
		value = s.draft
	}
	return nameField{name: w.name, child: Input{
		Value: value,
		OnChange: func(t string) {
			s.draft, s.editing = t, true
			// The field must know its Value is the draft, to follow when it
			// changes back.
			kit.Rebuild(&s.StateBase)
		},
		OnSubmit: func(string) { s.commit() },
		Extras: Extras{
			leading: fieldTag(w.short, th),
			OnFocus: func(focused bool) {
				if focused {
					s.draft, s.editing = value, true
					return
				}
				s.commit()
			},
		},
	}}
}

// fieldTag is the short caption at the start of a field of the picker, which
// a screen reader skips: the field is named already. It is in Ink2, which
// reads at 4.5:1 on the field: it is text that says what the field holds.
func fieldTag(tag string, th *theme.Theme) core.Widget {
	return silentBox{child: basic.Padding{Insets: geom.Insets{Left: fieldPaddingX}, Child: basic.Text{
		Content: tag, Style: kit.Colored(th.Text.Caption, th.Color.Ink2),
	}}}
}

// nameField names the field inside it to assistive technology, as a label
// beside it would, without showing the name.
type nameField struct {
	name  string
	child core.Widget
}

// Key implements core.Widget.
func (nameField) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (n nameField) CreateRenderObject() core.RenderObject { return &renderNameField{name: n.name} }

// UpdateRenderObject implements core.RenderWidget.
func (n nameField) UpdateRenderObject(ro core.RenderObject) { ro.(*renderNameField).name = n.name }

// ChildWidget implements core.SingleChildWidget.
func (n nameField) ChildWidget() core.Widget { return n.child }

type renderNameField struct {
	layout.RenderProxy
	name string
}

// Semantics implements core.RenderObject.
func (r *renderNameField) Semantics(sb *core.SemanticsBuilder) {
	sb.Name(r.name)
	r.RenderProxy.Semantics(sb)
	sb.Name("")
}

// silentBox hides its child from assistive technology, for what a node
// around it already says.
type silentBox struct {
	child core.Widget
}

// Key implements core.Widget.
func (silentBox) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (silentBox) CreateRenderObject() core.RenderObject { return &renderSilentBox{} }

// UpdateRenderObject implements core.RenderWidget.
func (silentBox) UpdateRenderObject(core.RenderObject) {}

// ChildWidget implements core.SingleChildWidget.
func (s silentBox) ChildWidget() core.Widget { return s.child }

type renderSilentBox struct{ layout.RenderProxy }

// Semantics implements core.RenderObject.
func (*renderSilentBox) Semantics(*core.SemanticsBuilder) {}

// colorPlaneView is the render widget of the square of a picker.
type colorPlaneView struct {
	hue, sat, val  float32
	height, radius float32
	border         paint.Stroke
	look           thumbLook
	onDrag         func(sat, val float32)
}

// Key implements core.Widget.
func (colorPlaneView) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (v colorPlaneView) CreateRenderObject() core.RenderObject {
	r := &renderColorPlane{}
	v.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (v colorPlaneView) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderColorPlane)
	if r.height != v.height {
		r.height = v.height
		r.MarkNeedsLayout()
	}
	r.hue, r.sat, r.val, r.radius, r.border, r.look, r.onDrag = v.hue, v.sat, v.val, v.radius, v.border, v.look, v.onDrag
	r.MarkNeedsPaint()
}

// colorStripView is the render widget of a strip of a picker.
type colorStripView struct {
	alpha   bool
	value   float32
	color   geom.Color
	checker checker
	border  paint.Stroke
	look    thumbLook
	onDrag  func(float32)
}

// Key implements core.Widget.
func (colorStripView) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (v colorStripView) CreateRenderObject() core.RenderObject {
	r := &renderColorStrip{}
	v.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (v colorStripView) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderColorStrip)
	r.alpha, r.value, r.color, r.checker, r.border, r.look, r.onDrag = v.alpha, v.value, v.color, v.checker, v.border, v.look, v.onDrag
	r.MarkNeedsPaint()
}

// colorSwatchView is the render widget of a color over a checkerboard.
type colorSwatchView struct {
	color         geom.Color
	width, height float32
	corners       [4]float32
	checker       checker
	border        paint.Stroke
}

// Key implements core.Widget.
func (colorSwatchView) Key() core.Key { return nil }

// CreateRenderObject implements core.RenderWidget.
func (v colorSwatchView) CreateRenderObject() core.RenderObject {
	r := &renderColorSwatch{}
	v.UpdateRenderObject(r)
	return r
}

// UpdateRenderObject implements core.RenderWidget.
func (v colorSwatchView) UpdateRenderObject(ro core.RenderObject) {
	r := ro.(*renderColorSwatch)
	if r.width != v.width || r.height != v.height {
		r.width, r.height = v.width, v.height
		r.MarkNeedsLayout()
	}
	r.color, r.corners, r.checker, r.border = v.color, v.corners, v.checker, v.border
	r.MarkNeedsPaint()
}
