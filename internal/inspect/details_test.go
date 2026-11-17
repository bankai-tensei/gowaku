//go:build gowakudebug

package inspect

import (
	"reflect"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// probeWidget has a field of each kind the details spell.
type probeWidget struct {
	core.StatelessBase
	Title    string
	Count    int
	Ratio    float32
	Axis     layout.Axis
	Ink      geom.Color
	Insets   geom.Insets
	OnPress  func()
	OnChange func(string)
	Child    core.Widget
	Children []core.Widget
	Words    map[string]int
	Deep     struct{ A struct{ B struct{ C int } } }
	Ptr      *geom.Size
	hidden   int
}

func (p probeWidget) Build(core.BuildContext) core.Widget { return basic.SizedBox{} }

func rowsByName(rows []detailRow) map[string]detailRow {
	m := map[string]detailRow{}
	for _, r := range rows {
		m[r.name] = r
	}
	return m
}

func TestWidgetRows_SpellFieldsCompactly(t *testing.T) {
	w := probeWidget{
		Title: "Save", Count: 3, Ratio: 0.5, Axis: layout.Vertical, Ink: geom.Hex(0x2f5fd0),
		Insets: geom.Insets{Left: 8, Top: 4}, OnPress: func() {},
		Child: basic.Text{Content: "x"}, Children: []core.Widget{basic.SizedBox{}, basic.SizedBox{}},
		Words: map[string]int{"a": 1}, Ptr: &geom.Size{W: 1}, hidden: 1,
	}
	w.Deep.A.B.C = 7
	rows := widgetRows(w)
	got := rowsByName(rows)
	want := map[string]string{
		"Type":     "inspect.probeWidget",
		"Key":      "—",
		"Title":    `"Save"`,
		"Count":    "3",
		"Ratio":    "0.5",
		"Axis":     "1",
		"Ink":      "#2F5FD0",
		"Insets":   "{Left: 8, Top: 4}",
		"OnPress":  "func",
		"OnChange": "nil",
		"Child":    "basic.Text",
		"Children": "[]core.Widget len 2",
		"Words":    "map[string]int len 1",
		"Deep":     "{A: {B: struct { C int }{…}}}",
		"Ptr":      "*geom.Size",
	}
	for name, v := range want {
		if got[name].value != v {
			t.Errorf("%s: got %q, want %q", name, got[name].value, v)
		}
	}
	if !got["Ink"].isColor || got["Ink"].color != w.Ink {
		t.Error("a color shows a swatch")
	}
	for _, name := range []string{"hidden", "StatelessBase"} {
		if _, ok := got[name]; ok {
			t.Errorf("%s shows", name)
		}
	}
	if rows[0].name != "Type" || rows[1].name != "Key" || rows[2].name != "Title" {
		t.Error("the fields keep their order after the type and the key")
	}
}

func TestFormatValue_Limits(t *testing.T) {
	long := strings.Repeat("あ", 200)
	if got := formatValue(reflect.ValueOf(long), 0); len([]rune(got)) > maxStringLen+2 {
		t.Errorf("a long string runs on: %d runes", len([]rune(got)))
	}
	var nilWidget core.Widget
	if got := formatValue(reflect.ValueOf(&nilWidget).Elem(), 0); got != "nil" {
		t.Errorf("nil interface: got %q", got)
	}
	if got := formatValue(reflect.ValueOf(geom.Color{R: 1, A: 0.5}), 0); got != "#FF000080" {
		t.Errorf("translucent color: got %q", got)
	}
	if got := formatValue(reflect.ValueOf(geom.Insets{}), 0); got != "{}" {
		t.Errorf("zero struct: got %q", got)
	}
}

func TestElementAndObjectRows(t *testing.T) {
	ft, _ := newSample(t)
	el := findElement(t, ft.tree.RootElement(), "control.Button")
	rows := rowsByName(elementRows(el))
	if rows["Kind"].value != "Stateful" || rows["Mounted"].value != "true" || !strings.Contains(rows["State"].value, "buttonState") {
		t.Errorf("got %+v", rows)
	}
	ro := el.NearestRenderObject()
	rows = rowsByName(objectRows(ro, true, &stringsEN))
	if rows["Type"].value != TypeName(ro) || rows["Needs layout"].value != "false" || rows["Constraints"].value == "" {
		t.Errorf("got %+v", rows)
	}
	if got := rows["Global rect"].value; !strings.HasPrefix(got, "0, ") {
		t.Errorf("global rect: got %q", got)
	}
	rows = rowsByName(objectRows(ro, false, &stringsEN))
	if rows["Status"].value != stringsEN.detached || len(rows) != 2 {
		t.Errorf("detached: got %+v", rows)
	}
}

func TestConstraintsAndFlags(t *testing.T) {
	if got := constraints(geom.Constraints{MaxW: 460, MinH: 32, MaxH: 32}); got != "w 0…460  h 32" {
		t.Errorf("got %q", got)
	}
	if got := constraints(geom.Loose(geom.Size{W: geom.Inf, H: geom.Inf})); !strings.Contains(got, "∞") {
		t.Errorf("unbounded: got %q", got)
	}
	if got := flags(uint64(semantics.Focusable|semantics.Checked), stateNames[:]); got != "Checked Focusable" {
		t.Errorf("state: got %q", got)
	}
	if got := flags(uint64(semantics.Required), stateNames[:]); got != "Required" {
		t.Errorf("the names of State are behind package semantics: got %q", got)
	}
	if got := flags(uint64(semantics.ActionScrollIntoView), actionNames[:]); got != "ScrollIntoView" {
		t.Errorf("the names of Actions are behind package semantics: got %q", got)
	}
}

func TestSemanticsRows(t *testing.T) {
	n := &semantics.Node{ID: 4, Role: semantics.RoleSlider, Label: "Volume", State: semantics.Focusable,
		Range: semantics.Range{Max: 100, Value: 40}, Rect: geom.Rect{X: 1, Y: 2, W: 30, H: 10}}
	rows := rowsByName(semanticsRows(n, true, nil, &stringsEN))
	if rows["Role"].value != "slider" || rows["Label"].value != `"Volume"` || rows["Range"].value != "40 in 0…100" {
		t.Errorf("got %+v", rows)
	}
	if _, ok := rows["Hint"]; ok {
		t.Error("an empty hint shows")
	}
	if _, ok := rows["Node"]; ok {
		t.Error("the node of the object itself says it is an ancestor's")
	}
	rows = rowsByName(semanticsRows(n, false, widgetsRO(t), &stringsEN))
	if !strings.Contains(rows["Node"].value, "ancestor") {
		t.Errorf("got %q", rows["Node"].value)
	}
}

func widgetsRO(t *testing.T) core.RenderObject {
	ft, _ := newSample(t)
	return ft.tree.RootRenderObject()
}

func TestDumpOf_ShowsThePicture(t *testing.T) {
	ft, _ := newSample(t)
	// The column stretches the box to the width of the window.
	boundary := findElement(t, ft.tree.RootElement(), "basic.RepaintBoundary").NearestRenderObject()
	box := boundary.ChildAt(0)
	got := dumpOf(ft, box, &stringsEN)
	if !strings.HasPrefix(got, describe(box)+"\n\n") || !strings.Contains(got, "rrect x=0 y=0 w=400 h=20 color=#336699") {
		t.Errorf("got\n%s", got)
	}
	// A repaint boundary shows its layer, and the commands in it.
	got = dumpOf(ft, boundary, &stringsEN)
	if !strings.Contains(got, "\nlayer ") || !strings.Contains(got, "\n  rrect x=0 y=0 w=400 h=20 color=#336699") {
		t.Errorf("got\n%s", got)
	}
	// The whole window has the text and the button too.
	got = dumpOf(ft, ft.tree.RootRenderObject(), &stringsEN)
	if !strings.Contains(got, "glyphs") {
		t.Errorf("got\n%s", got)
	}
	if got := dumpOf(ft, nil, &stringsEN); got != "" {
		t.Errorf("nothing selected: got %q", got)
	}
}
