package field

import (
	"strconv"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// colorHost keeps the color of a picker, as an application does, and logs
// what it reports.
type colorHost struct {
	core.StatefulBase
	picker ColorPicker
	log    *[]geom.Color
	state  **colorHostState
}

type colorHostState struct {
	core.StateOf[colorHost]
	value geom.Color
}

func (colorHost) CreateState() core.State { return &colorHostState{} }

func (s *colorHostState) InitState() {
	s.value = s.Widget().picker.Value
	if p := s.Widget().state; p != nil {
		*p = s
	}
}

func (s *colorHostState) Build(core.BuildContext) core.Widget {
	p := s.Widget().picker
	p.Value = s.value
	if p.OnChange != nil {
		p.OnChange = func(c geom.Color) {
			*s.Widget().log = append(*s.Widget().log, c)
			s.SetState(func() { s.value = c })
		}
	}
	return p
}

// colorRig is a window with a picker and what it reported.
type colorRig struct {
	*fieldRig
	log  []geom.Color
	host *colorHostState
}

func newColorRig(t *testing.T, p ColorPicker) *colorRig {
	t.Helper()
	r := &colorRig{}
	if p.OnChange == nil {
		p.OnChange = func(geom.Color) {}
	}
	r.fieldRig = newFieldRig(t, colorHost{picker: p, log: &r.log, state: &r.host})
	r.Size = geom.Size{W: 500, H: 600}
	r.frame()
	return r
}

// last returns the last color reported.
func (r *colorRig) last() geom.Color {
	r.T.Helper()
	if len(r.log) == 0 {
		r.T.Fatal("nothing reported")
	}
	return r.log[len(r.log)-1]
}

// hsv returns the HSV state of the picker.
func (r *colorRig) hsv() theme.HSV {
	r.T.Helper()
	var found *colorPickerState
	var walk func(v core.ElementView)
	walk = func(v core.ElementView) {
		if s, ok := v.State().(*colorPickerState); ok {
			found = s
		}
		for i := range v.ChildCount() {
			walk(v.ChildAt(i))
		}
	}
	walk(r.tree.RootElement())
	if found == nil {
		r.T.Fatal("no picker")
	}
	return found.hsv
}

// press puts the pointer down at at, relative to the node n.
func (r *colorRig) drag(n *semantics.Node, from geom.Offset, to ...geom.Offset) {
	r.T.Helper()
	at := func(p geom.Offset) geom.Offset { return geom.Offset{X: n.Rect.X + p.X, Y: n.Rect.Y + p.Y} }
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: at(from)})
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at(from), Button: input.ButtonPrimary})
	r.frame()
	for _, p := range to {
		r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: at(p)})
		r.frame()
	}
	end := from
	if len(to) > 0 {
		end = to[len(to)-1]
	}
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: at(end), Button: input.ButtonPrimary})
	r.frame()
}

func near(a, b float32) bool { return geom.Abs32(a-b) < 0.02 }

func nearHSV(t *testing.T, got, want theme.HSV) {
	t.Helper()
	if geom.Abs32(got.H-want.H) > 0.5 || !near(got.S, want.S) || !near(got.V, want.V) || !near(got.A, want.A) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

var pickerBlue = theme.HSV{H: 210, S: 0.6, V: 0.8, A: 1}

func TestColorPicker_Semantics(t *testing.T) {
	r := newColorRig(t, ColorPicker{Value: geom.Hex(0x3a7bf0)})
	group := r.must(semantics.RoleGroup, "Choose color")
	if len(group.Children) == 0 {
		t.Fatal("the group holds the parts")
	}
	plane := r.must(semantics.RoleSlider, "Saturation and brightness")
	h := theme.HSVFromColor(geom.Hex(0x3a7bf0))
	want := "Saturation " + itoaPct(h.S) + ", Brightness " + itoaPct(h.V)
	if plane.Value != want {
		t.Errorf("plane says %q, want %q", plane.Value, want)
	}
	hue := r.must(semantics.RoleSlider, "Hue")
	if hue.Range.Max != 360 || hue.Value != "219°" || hue.Actions&semantics.ActionIncrease == 0 {
		t.Errorf("hue %+v", hue)
	}
	alpha := r.must(semantics.RoleSlider, "Opacity")
	if alpha.Value != "100%" || alpha.Range.Max != 100 {
		t.Errorf("opacity %+v", alpha)
	}
	hex := r.must(semantics.RoleTextField, "Hex")
	if hex.Value != "#3A7BF0" {
		t.Errorf("hex %q", hex.Value)
	}
	if n := r.must(semantics.RoleButton, "Original color"); n.Value != "#3A7BF0" {
		t.Errorf("original %+v", n)
	}
}

func itoaPct(v float32) string { return strconv.Itoa(int(v*100+0.5)) + "%" }

func TestColorPicker_DragThePlane(t *testing.T) {
	r := newColorRig(t, ColorPicker{Value: pickerBlue.Color()})
	plane := r.must(semantics.RoleSlider, "Saturation and brightness")
	w, h := plane.Rect.W, plane.Rect.H
	r.drag(plane, geom.Offset{X: w / 2, Y: h / 2})
	nearHSV(t, r.hsv(), theme.HSV{H: 210, S: 0.5, V: 0.5, A: 1})
	nearHSV(t, theme.HSVFromColor(r.last()), theme.HSV{H: 210, S: 0.5, V: 0.5, A: 1})
	r.drag(plane, geom.Offset{X: w / 4, Y: h / 4}, geom.Offset{X: w * 2, Y: -50})
	nearHSV(t, r.hsv(), theme.HSV{H: 210, S: 1, V: 1, A: 1})
	if got := r.focused(); got == nil || got.Label != "Saturation and brightness" {
		t.Errorf("a press gives the plane the keyboard, focus on %+v", got)
	}
}

func TestColorPicker_DragTheStrips(t *testing.T) {
	r := newColorRig(t, ColorPicker{Value: pickerBlue.Color()})
	hue := r.must(semantics.RoleSlider, "Hue")
	r.drag(hue, geom.Offset{X: 0, Y: hue.Rect.H / 2})
	nearHSV(t, r.hsv(), theme.HSV{H: 0, S: 0.6, V: 0.8, A: 1})
	r.drag(hue, geom.Offset{X: 0, Y: hue.Rect.H / 2}, geom.Offset{X: hue.Rect.W / 2, Y: 0}, geom.Offset{X: hue.Rect.W + 40, Y: 0})
	nearHSV(t, r.hsv(), theme.HSV{H: 360, S: 0.6, V: 0.8, A: 1})
	if c := r.last(); !near(c.R, 0.8) || !near(c.G, 0.32) {
		t.Errorf("hue 360 is red again, got %+v", c)
	}

	alpha := r.must(semantics.RoleSlider, "Opacity")
	r.drag(alpha, geom.Offset{X: alpha.Rect.W / 2, Y: alpha.Rect.H / 2})
	if a := r.last().A; !near(a, 0.5) {
		t.Errorf("the middle of the opacity strip is half, got %v", a)
	}
	r.drag(alpha, geom.Offset{X: alpha.Rect.W / 2, Y: 0}, geom.Offset{X: -30, Y: 0})
	if a := r.last().A; a != 0 {
		t.Errorf("the left end is clear, got %v", a)
	}
	if hex := r.must(semantics.RoleTextField, "Hex"); !strings.HasSuffix(hex.Value, "00") || len(hex.Value) != 9 {
		t.Errorf("a clear color has its alpha in the code, got %q", hex.Value)
	}
}

func TestColorPicker_Keys(t *testing.T) {
	r := newColorRig(t, ColorPicker{Value: pickerBlue.Color()})
	r.act(r.must(semantics.RoleSlider, "Saturation and brightness"), semantics.ActionFocus)
	steps := []struct {
		key  input.KeyCode
		mods input.Modifiers
		want theme.HSV
	}{
		{input.KeyRight, 0, theme.HSV{H: 210, S: 0.61, V: 0.8, A: 1}},
		{input.KeyRight, input.ModShift, theme.HSV{H: 210, S: 0.71, V: 0.8, A: 1}},
		{input.KeyLeft, 0, theme.HSV{H: 210, S: 0.70, V: 0.8, A: 1}},
		{input.KeyUp, 0, theme.HSV{H: 210, S: 0.70, V: 0.81, A: 1}},
		{input.KeyDown, input.ModShift, theme.HSV{H: 210, S: 0.70, V: 0.71, A: 1}},
		{input.KeyPageUp, 0, theme.HSV{H: 210, S: 0.70, V: 0.81, A: 1}},
		{input.KeyEnd, 0, theme.HSV{H: 210, S: 1, V: 0.81, A: 1}},
		{input.KeyHome, 0, theme.HSV{H: 210, S: 0, V: 0.81, A: 1}},
	}
	for _, st := range steps {
		r.Key(st.key, st.mods)
		nearHSV(t, r.hsv(), st.want)
	}

	r.Key(input.KeyTab, 0)
	if got := r.focused(); got == nil || got.Label != "Hue" {
		t.Fatalf("Tab goes to the hue, focus on %+v", got)
	}
	for _, st := range []struct {
		key  input.KeyCode
		mods input.Modifiers
		want float32
	}{
		{input.KeyRight, 0, 211}, {input.KeyUp, input.ModShift, 221}, {input.KeyDown, 0, 220},
		{input.KeyLeft, input.ModShift, 210}, {input.KeyEnd, 0, 360}, {input.KeyHome, 0, 0}, {input.KeyLeft, 0, 0},
	} {
		r.Key(st.key, st.mods)
		if got := r.hsv().H; geom.Abs32(got-st.want) > 1e-3 {
			t.Errorf("hue after %v: got %v, want %v", st.key, got, st.want)
		}
	}

	r.Key(input.KeyTab, 0)
	if got := r.focused(); got == nil || got.Label != "Opacity" {
		t.Fatalf("Tab goes to the opacity, focus on %+v", got)
	}
	for _, st := range []struct {
		key  input.KeyCode
		mods input.Modifiers
		want float32
	}{
		{input.KeyLeft, 0, 0.99}, {input.KeyLeft, input.ModShift, 0.89}, {input.KeyRight, 0, 0.9},
		{input.KeyHome, 0, 0}, {input.KeyDown, 0, 0}, {input.KeyEnd, 0, 1}, {input.KeyUp, 0, 1},
	} {
		r.Key(st.key, st.mods)
		if got := r.hsv().A; !near(got, st.want) {
			t.Errorf("opacity after %v: got %v, want %v", st.key, got, st.want)
		}
	}
	r.Key(input.KeyTab, 0)
	if got := r.focused(); got == nil || got.Label != "Original color" {
		t.Fatalf("Tab goes to the original color, focus on %+v", got)
	}
	r.Key(input.KeyTab, 0)
	if got := r.focused(); got == nil || got.Role != semantics.RoleTextField || got.Label != "Hex" {
		t.Fatalf("Tab goes to the hex field, focus on %+v", got)
	}
	r.Key(input.KeyRight, 0)
	if got := r.hsv().A; got != 1 {
		t.Errorf("keys in the field do not move the strips, got %v", got)
	}
}

func TestColorPicker_HueStaysAtTheEdges(t *testing.T) {
	r := newColorRig(t, ColorPicker{Value: pickerBlue.Color()})
	plane := r.must(semantics.RoleSlider, "Saturation and brightness")
	w, h := plane.Rect.W, plane.Rect.H

	// Down to black: the color has no hue, the picker keeps it.
	r.drag(plane, geom.Offset{X: w * 0.6, Y: h * 0.2}, geom.Offset{X: w * 0.6, Y: h + 20})
	if c := r.last(); c.R != 0 || c.G != 0 || c.B != 0 {
		t.Fatalf("the bottom is black, got %+v", c)
	}
	nearHSV(t, r.hsv(), theme.HSV{H: 210, S: 0.6, V: 0, A: 1})
	r.drag(plane, geom.Offset{X: w * 0.6, Y: h - 1}, geom.Offset{X: w * 0.6, Y: h * 0.2})
	nearHSV(t, r.hsv(), pickerBlue)
	nearHSV(t, theme.HSVFromColor(r.last()), pickerBlue)

	// To the left edge: gray, and the hue again stays.
	r.drag(plane, geom.Offset{X: 1, Y: h * 0.2}, geom.Offset{X: -10, Y: h * 0.2})
	if c := r.last(); c.R != c.G || c.G != c.B {
		t.Fatalf("the left edge is gray, got %+v", c)
	}
	r.act(plane, semantics.ActionFocus)
	r.Key(input.KeyEnd, 0)
	nearHSV(t, r.hsv(), theme.HSV{H: 210, S: 1, V: 0.8, A: 1})

	// A gray handed back as Value from outside keeps the hue too.
	r.host.SetState(func() { r.host.value = geom.Hex(0x808080) })
	r.frame()
	if got := r.hsv(); got.H != 210 || got.S != 0 {
		t.Errorf("an outside gray keeps the hue, got %+v", got)
	}
	r.host.SetState(func() { r.host.value = geom.Hex(0x000000) })
	r.frame()
	if got := r.hsv(); got.H != 210 || got.S != 0 || got.V != 0 {
		t.Errorf("an outside black keeps the hue, got %+v", got)
	}
	r.host.SetState(func() { r.host.value = geom.Hex(0x00ff00) })
	r.frame()
	nearHSV(t, r.hsv(), theme.HSV{H: 120, S: 1, V: 1, A: 1})
}

func TestColorPicker_HexField(t *testing.T) {
	r := newColorRig(t, ColorPicker{Value: pickerBlue.Color()})
	hex := r.must(semantics.RoleTextField, "Hex")
	r.act(hex, semantics.ActionFocus)
	replace := func(text string) {
		r.Key(input.KeyA, input.ModControl)
		r.typeText(text)
	}

	replace("#abc")
	if got := theme.FormatHex(r.last()); got != "#AABBCC" {
		t.Errorf("a short code takes at once, got %s", got)
	}
	if got := r.must(semantics.RoleTextField, "Hex").Value; got != "#abc" {
		t.Errorf("the field keeps what is typed while it has the keyboard, got %q", got)
	}
	r.Key(input.KeyEnter, 0)
	if got := r.must(semantics.RoleTextField, "Hex").Value; got != "#AABBCC" {
		t.Errorf("Enter shows the code of the color, got %q", got)
	}

	cases := []struct {
		name, text, want string
	}{
		{"貼り付けの空白と記号", "  \"#3a7bf0\"; ", "#3A7BF0"},
		{"# なし", "ff8800", "#FF8800"},
		{"0x", "0x00FF00", "#00FF00"},
		{"透明度つき", "#FF000080", "#FF000080"},
		{"短い透明度つき", "#f008", "#FF000088"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			replace(tc.text)
			r.Key(input.KeyEnter, 0)
			if got := theme.FormatHex(r.last()); got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}

	n := len(r.log)
	before := r.last()
	replace("#12")
	replace("not a color")
	if len(r.log) != n {
		t.Errorf("text that is no code changes nothing, got %v", r.log[n:])
	}
	r.Key(input.KeyEnter, 0)
	if got := r.must(semantics.RoleTextField, "Hex").Value; got != theme.FormatHex(before) {
		t.Errorf("Enter puts back the code of the color, got %q", got)
	}

	// Leaving the field ends the edit as Enter does.
	replace("#0000ff")
	r.act(r.must(semantics.RoleSlider, "Hue"), semantics.ActionFocus)
	if got := r.must(semantics.RoleTextField, "Hex").Value; got != "#0000FF" {
		t.Errorf("leaving shows the code, got %q", got)
	}
	if got := r.hsv(); got.H != 240 {
		t.Errorf("got %+v", got)
	}
}

func TestColorPicker_Opaque(t *testing.T) {
	r := newColorRig(t, ColorPicker{Value: geom.Color{R: 1, A: 0.5}, Opaque: true})
	if r.Find(semantics.RoleSlider, "Opacity") != nil {
		t.Error("an opaque picker has no opacity strip")
	}
	if got := r.must(semantics.RoleTextField, "Hex").Value; got != "#FF0000" {
		t.Errorf("an opaque picker shows an opaque code, got %q", got)
	}
	r.act(r.must(semantics.RoleTextField, "Hex"), semantics.ActionFocus)
	r.Key(input.KeyA, input.ModControl)
	r.typeText("#00ff0080")
	r.Key(input.KeyEnter, 0)
	if got := r.last(); got.A != 1 || got.G != 1 {
		t.Errorf("the alpha of a code is dropped, got %+v", got)
	}
}

func TestColorPicker_OriginalAndSwatches(t *testing.T) {
	swatches := []geom.Color{geom.Hex(0xff3b30), geom.Hex(0x34c759), {B: 1, A: 0.5}}
	r := newColorRig(t, ColorPicker{Value: pickerBlue.Color(), Swatches: swatches})
	r.click(r.must(semantics.RoleButton, "#34C759"))
	if got := theme.FormatHex(r.last()); got != "#34C759" {
		t.Errorf("a swatch picks its color, got %s", got)
	}
	if n := r.must(semantics.RoleButton, "#34C759"); n.State&semantics.Selected == 0 {
		t.Error("the swatch of the color is selected")
	}
	if n := r.must(semantics.RoleButton, "#FF3B30"); n.State&semantics.Selected != 0 {
		t.Error("the others are not")
	}
	r.click(r.must(semantics.RoleButton, "#0000FF80"))
	if got := r.last(); got.A != 0.5 || got.B != 1 {
		t.Errorf("a translucent swatch keeps its alpha, got %+v", got)
	}
	r.click(r.must(semantics.RoleButton, "Original color"))
	nearHSV(t, r.hsv(), pickerBlue)
	nearHSV(t, theme.HSVFromColor(r.last()), pickerBlue)
}

func TestColorPicker_NumberFields(t *testing.T) {
	r := newColorRig(t, ColorPicker{Value: geom.Hex(0x102030), Numeric: true})
	red := r.must(semantics.RoleTextField, "Red")
	if red.Value != "16" {
		t.Errorf("red shows %q", red.Value)
	}
	set := func(label, text string) {
		r.act(r.must(semantics.RoleTextField, label), semantics.ActionFocus)
		r.Key(input.KeyA, input.ModControl)
		r.typeText(text)
		r.Key(input.KeyEnter, 0)
	}
	set("Red", "255")
	if got := theme.FormatHex(r.last()); got != "#FF2030" {
		t.Errorf("got %s", got)
	}
	set("Green", "999")
	if got := theme.FormatHex(r.last()); got != "#FFFF30" {
		t.Errorf("a number past the top is clamped, got %s", got)
	}
	n := len(r.log)
	set("Blue", "x")
	if len(r.log) != n {
		t.Error("text that is no number changes nothing")
	}
	if got := r.must(semantics.RoleTextField, "Blue").Value; got != "48" {
		t.Errorf("the field shows the number again, got %q", got)
	}
	set("Hue", "120")
	set("Saturation", "100")
	set("Brightness", "50")
	nearHSV(t, r.hsv(), theme.HSV{H: 120, S: 1, V: 0.5, A: 1})
}

func TestColorPicker_ScreenReaderActions(t *testing.T) {
	r := newColorRig(t, ColorPicker{Value: pickerBlue.Color()})
	r.act(r.must(semantics.RoleSlider, "Hue"), semantics.ActionIncrease)
	if got := r.hsv().H; got != 211 {
		t.Errorf("increase steps the hue, got %v", got)
	}
	hue := r.must(semantics.RoleSlider, "Hue")
	if !r.tree.PerformSemanticsAction(hue.ID, semantics.ActionSetValue, core.SemanticsArg{Number: 30}) {
		t.Fatal("the hue takes a value")
	}
	r.frame()
	if got := r.hsv().H; got != 30 {
		t.Errorf("got %v", got)
	}
	alpha := r.must(semantics.RoleSlider, "Opacity")
	r.tree.PerformSemanticsAction(alpha.ID, semantics.ActionSetValue, core.SemanticsArg{Number: 25})
	r.frame()
	if got := r.hsv().A; got != 0.25 {
		t.Errorf("got %v", got)
	}
	r.act(r.must(semantics.RoleSlider, "Saturation and brightness"), semantics.ActionDecrease)
	if got := r.hsv().S; !near(got, 0.59) {
		t.Errorf("decrease steps the saturation, got %v", got)
	}
}

func TestColorPicker_WithoutOnChangeOnlyShows(t *testing.T) {
	var log []geom.Color
	rig := newFieldRig(t, colorHost{picker: ColorPicker{Value: pickerBlue.Color()}, log: &log})
	rig.Size = geom.Size{W: 500, H: 600}
	rig.frame()
	plane := rig.must(semantics.RoleSlider, "Saturation and brightness")
	if plane.State&semantics.Disabled == 0 {
		t.Error("a picker without OnChange is disabled")
	}
	wtest.Tap(rig.tree, wtest.Center(plane.Rect))
	rig.frame()
	if rig.Find(semantics.RoleTextField, "") != nil {
		t.Error("it has no field to type into")
	}
	if n := rig.must(semantics.RoleText, "Hex"); n.Value != theme.FormatHex(pickerBlue.Color()) {
		t.Errorf("the code still shows, got %+v", n)
	}
	if len(log) != 0 {
		t.Errorf("got %v", log)
	}
}

func TestColorPicker_ZeroValueAndWidth(t *testing.T) {
	r := newFieldRig(t, ColorPicker{OnChange: func(geom.Color) {}})
	if n := r.must(semantics.RoleTextField, "Hex"); n.Value != "#00000000" {
		t.Errorf("the zero color is clear black, got %q", n.Value)
	}
	if g := r.must(semantics.RoleGroup, "Choose color"); g.Rect.W != colorPickerWidth {
		t.Errorf("got %+v", g.Rect)
	}
	if bare := newFieldRig(t, ColorPicker{}); bare.must(semantics.RoleText, "Hex").Value != "#00000000" {
		t.Error("the zero value shows clear black")
	}
	wide := newFieldRig(t, ColorPicker{Width: 320, Label: "背景色"})
	if g := wide.must(semantics.RoleGroup, "背景色"); g.Rect.W != 320 {
		t.Errorf("got %+v", g.Rect)
	}
	plane := wide.must(semantics.RoleSlider, "Saturation and brightness")
	if plane.Rect.W != 320 || plane.Rect.H != geom.Floor32(320*colorPlaneShare) {
		t.Errorf("the plane follows the width, got %+v", plane.Rect)
	}
}

func TestColorPicker_PaintsWithGradients(t *testing.T) {
	r := newColorRig(t, ColorPicker{Value: pickerBlue.Color()})
	ops := wtest.Ops(r.tree, r.Size)
	var plane, hue, alpha int
	for i := range ops {
		op := &ops[i]
		if !op.HasGradient {
			continue
		}
		g := &op.Gradient
		switch {
		case g.MoreStops != nil:
			hue++
			if n := len(*g.MoreStops); n != 7 {
				t.Errorf("the hue strip has %d stops", n)
			}
		case g.StartColor == geom.Hex(0xffffff):
			plane++
			if got := theme.HSVFromColor(g.EndColor); got.H != 210 || got.S != 1 || got.V != 1 {
				t.Errorf("the plane runs to the pure hue, got %+v", got)
			}
		case g.EndColor == geom.Hex(0):
			plane++
		case g.StartColor.A == 0 && g.EndColor.A == 1:
			alpha++
		}
	}
	if plane != 2 || hue != 1 || alpha != 1 {
		t.Errorf("got plane %d, hue %d, alpha %d gradients", plane, hue, alpha)
	}
}

func TestColorPicker_PaintDoesNotAllocate(t *testing.T) {
	r := newColorRig(t, ColorPicker{Value: geom.Color{R: 0.2, G: 0.4, B: 0.9, A: 0.6}, Swatches: []geom.Color{geom.Hex(0xff0000)}})
	var cv paint.Canvas
	frame := func() {
		r.tree.RootRenderObject().MarkNeedsPaint()
		cv.Reset()
		r.tree.Paint(&cv)
	}
	for range 5 {
		frame()
	}
	if got := testing.AllocsPerRun(20, frame); got != 0 {
		t.Errorf("a paint of the picker allocates %v times", got)
	}
}
