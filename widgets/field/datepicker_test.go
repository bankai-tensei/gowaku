package field

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// dateHost keeps the value of a DatePicker, as an application does.
type dateHost struct {
	core.StatefulBase
	picker DatePicker
	log    *[]time.Time
}

type dateHostState struct {
	core.StateOf[dateHost]
	value time.Time
}

func (dateHost) CreateState() core.State { return &dateHostState{} }

func (s *dateHostState) InitState() { s.value = s.Widget().picker.Value }

func (s *dateHostState) Build(core.BuildContext) core.Widget {
	p := s.Widget().picker
	p.Value = s.value
	p.OnChange = func(d time.Time) {
		*s.Widget().log = append(*s.Widget().log, d)
		s.SetState(func() { s.value = d })
	}
	return p
}

// dateApp is a window with a date picker at align, and what it reported.
type dateApp struct {
	tree *core.Tree
	log  []time.Time
}

func newDateApp(t *testing.T, tag language.Tag, align layout.Alignment, p DatePicker) *dateApp {
	t.Helper()
	a := &dateApp{}
	a.tree = wtest.TextTree(t, overlay.Overlay{Child: basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1, Locale: tag}, Child: basic.Align{
		Alignment: align, Child: dateHost{picker: p, log: &a.log},
	}}})
	wtest.Ops(a.tree, pickerScreen)
	return a
}

func (a *dateApp) combo(t *testing.T) *semantics.Node {
	t.Helper()
	n := wtest.FindRole(a.tree.Semantics(), semantics.RoleComboBox)
	if n == nil {
		t.Fatal("no combo box")
	}
	return n
}

func (a *dateApp) open(t *testing.T) bool {
	t.Helper()
	return wtest.FindRole(a.tree.Semantics(), semantics.RoleDialog) != nil
}

func (a *dateApp) click(t *testing.T, label string) {
	t.Helper()
	wtest.Tap(a.tree, wtest.Center(wtest.ByLabel(t, a.tree.Semantics(), label).Rect))
	wtest.Ops(a.tree, pickerScreen)
}

func (a *dateApp) clickField(t *testing.T) {
	t.Helper()
	wtest.Tap(a.tree, wtest.Center(a.combo(t).Rect))
	wtest.Ops(a.tree, pickerScreen)
}

func TestDatePicker_ShowsTheDateOfTheLocale(t *testing.T) {
	cases := []struct {
		name  string
		tag   language.Tag
		value time.Time
		want  string
		label string
	}{
		{"英語", language.English, day(2026, 11, 9), "Nov 9, 2026", "Choose date"},
		{"日本語", language.Japanese, day(2026, 11, 9), "2026/11/09", "日付を選択"},
		{"空", language.English, time.Time{}, "", "Choose date"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newDateApp(t, tc.tag, layout.AlignTopLeft, DatePicker{Value: tc.value, Placeholder: "—"})
			n := a.combo(t)
			if n.Value != tc.want || n.Label != tc.label {
				t.Errorf("got %q %q", n.Label, n.Value)
			}
			if n.State&semantics.Collapsed == 0 || n.State&semantics.Expanded != 0 {
				t.Errorf("a closed field is collapsed, got %v", n.State)
			}
		})
	}
}

func TestDatePicker_ClickOpensAndADayChooses(t *testing.T) {
	a := newDateApp(t, language.English, layout.AlignTopLeft, DatePicker{Label: "Due", Value: day(2026, 11, 9)})
	a.clickField(t)
	if !a.open(t) {
		t.Fatal("a click must open the calendar")
	}
	if n := a.combo(t); n.State&semantics.Expanded == 0 {
		t.Errorf("an open field is expanded, got %v", n.State)
	}
	if got := wtest.FocusedLabel(a.tree); got != "Monday, November 9, 2026" {
		t.Errorf("the value must take the focus, got %q", got)
	}
	a.click(t, "Friday, November 20, 2026")
	if a.open(t) {
		t.Error("choosing must close the calendar")
	}
	if len(a.log) != 1 || a.log[0] != day(2026, 11, 20) {
		t.Fatalf("got %v", a.log)
	}
	if n := a.combo(t); n.Value != "Nov 20, 2026" {
		t.Errorf("got %q", n.Value)
	}
	if got := wtest.FocusedLabel(a.tree); got != "Due" {
		t.Errorf("the field must get the focus back, got %q", got)
	}
}

func TestDatePicker_Keys(t *testing.T) {
	for _, k := range []struct {
		name string
		key  input.KeyCode
		mod  input.Modifiers
	}{
		{"Enter", input.KeyEnter, 0},
		{"Space", input.KeySpace, 0},
		{"Alt+Down", input.KeyDown, input.ModAlt},
		{"F4", input.KeyF4, 0},
	} {
		t.Run(k.name, func(t *testing.T) {
			a := newDateApp(t, language.English, layout.AlignTopLeft, DatePicker{Label: "Due", Value: day(2026, 12, 31)})
			press(a.tree, input.KeyTab, 0)
			press(a.tree, k.key, k.mod)
			if !a.open(t) {
				t.Fatal("the key must open the calendar")
			}
			press(a.tree, input.KeyRight, 0)
			if got := wtest.FocusedLabel(a.tree); got != "Friday, January 1, 2027" {
				t.Errorf("got %q", got)
			}
			press(a.tree, input.KeyEnter, 0)
			if a.open(t) || len(a.log) != 1 || a.log[0] != day(2027, 1, 1) {
				t.Errorf("Enter must choose and close, got %v", a.log)
			}
			if got := wtest.FocusedLabel(a.tree); got != "Due" {
				t.Errorf("got %q", got)
			}
		})
	}
}

func TestDatePicker_EscapeAndClickBesideClose(t *testing.T) {
	a := newDateApp(t, language.English, layout.AlignTopLeft, DatePicker{Label: "Due", Value: day(2026, 11, 9)})
	press(a.tree, input.KeyTab, 0)
	press(a.tree, input.KeyEnter, 0)
	press(a.tree, input.KeyDown, 0)
	press(a.tree, input.KeyEscape, 0)
	if a.open(t) || len(a.log) != 0 {
		t.Fatalf("Escape must close without a change, got %v", a.log)
	}
	if got := wtest.FocusedLabel(a.tree); got != "Due" {
		t.Errorf("got %q", got)
	}
	a.clickField(t)
	if !a.open(t) {
		t.Fatal("must open again")
	}
	wtest.Tap(a.tree, geom.Offset{X: pickerScreen.W - 10, Y: pickerScreen.H - 10})
	wtest.Ops(a.tree, pickerScreen)
	if a.open(t) || len(a.log) != 0 {
		t.Errorf("a click beside must close without a change, got %v", a.log)
	}
}

func TestDatePicker_TabStaysInTheCalendar(t *testing.T) {
	a := newDateApp(t, language.English, layout.AlignTopLeft, DatePicker{Label: "Due", Value: day(2026, 11, 9), ShowToday: true, Today: day(2026, 11, 3)})
	a.clickField(t)
	seen := map[string]bool{}
	for range 6 {
		press(a.tree, input.KeyTab, 0)
		seen[wtest.FocusedLabel(a.tree)] = true
	}
	if seen["Due"] {
		t.Error("Tab must stay in the open calendar")
	}
	for _, want := range []string{"Previous month", "Next month", "Monday, November 9, 2026", "Today"} {
		if !seen[want] {
			t.Errorf("Tab must reach %q, got %v", want, seen)
		}
	}
}

func TestDatePicker_TodayAndClear(t *testing.T) {
	a := newDateApp(t, language.Japanese, layout.AlignTopLeft, DatePicker{
		Label: "期日", Value: day(2026, 10, 1), Today: day(2026, 11, 9), ShowToday: true, ShowClear: true,
	})
	a.clickField(t)
	a.click(t, "今日")
	if len(a.log) != 1 || a.log[0] != day(2026, 11, 9) || a.open(t) {
		t.Fatalf("Today must choose today and close, got %v", a.log)
	}
	a.clickField(t)
	a.click(t, "クリア")
	if len(a.log) != 2 || !a.log[1].IsZero() {
		t.Fatalf("Clear must choose the zero time, got %v", a.log)
	}
	if n := a.combo(t); n.Value != "" {
		t.Errorf("got %q", n.Value)
	}
	a.clickField(t)
	if n := wtest.ByLabel(t, a.tree.Semantics(), "クリア"); n.State&semantics.Disabled == 0 {
		t.Error("Clear has nothing to clear")
	}
}

func TestDatePicker_TodayOutOfRangeIsDisabled(t *testing.T) {
	a := newDateApp(t, language.English, layout.AlignTopLeft, DatePicker{
		Label: "Due", Value: day(2026, 12, 1), Today: day(2026, 11, 9), Min: day(2026, 12, 1), ShowToday: true,
	})
	a.clickField(t)
	if n := wtest.ByLabel(t, a.tree.Semantics(), "Today"); n.State&semantics.Disabled == 0 {
		t.Error("Today before Min cannot be chosen")
	}
}

func TestDatePicker_TurnsAboveAtTheBottom(t *testing.T) {
	a := newDateApp(t, language.English, layout.AlignBottomLeft, DatePicker{Label: "Due", Value: day(2026, 11, 9)})
	field := a.combo(t).Rect
	a.clickField(t)
	dialog := wtest.FindRole(a.tree.Semantics(), semantics.RoleDialog)
	if dialog == nil {
		t.Fatal("not open")
	}
	if dialog.Rect.Y+dialog.Rect.H > field.Y {
		t.Errorf("the calendar must open above the field: %+v over %+v", dialog.Rect, field)
	}
	b := newDateApp(t, language.English, layout.AlignTopLeft, DatePicker{Label: "Due", Value: day(2026, 11, 9)})
	field = b.combo(t).Rect
	b.clickField(t)
	if d := wtest.FindRole(b.tree.Semantics(), semantics.RoleDialog); d.Rect.Y < field.Y+field.H {
		t.Errorf("the calendar must open below the field: %+v under %+v", d.Rect, field)
	}
}

func TestDatePicker_ThemeAndLocaleReachTheCalendar(t *testing.T) {
	var log []time.Time
	tree := wtest.TextTree(t, overlay.Overlay{Child: basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1, Locale: language.Japanese}, Child: basic.Theme{
		Data: theme.Default().Dark, Child: basic.Align{Alignment: layout.AlignTopLeft, Child: dateHost{
			picker: DatePicker{Label: "期日", Value: day(2026, 11, 9)}, log: &log,
		}},
	}}})
	wtest.Ops(tree, pickerScreen)
	wtest.Tap(tree, wtest.Center(wtest.FindRole(tree.Semantics(), semantics.RoleComboBox).Rect))
	list := wtest.Ops(tree, pickerScreen)
	if g := wtest.FindRole(tree.Semantics(), semantics.RoleGrid); g == nil || g.Label != "2026年11月" {
		t.Fatalf("the calendar must speak Japanese, got %+v", g)
	}
	var raised bool
	for _, op := range list {
		if op.Kind == paint.OpRRect && op.Color == theme.Default().Dark.Color.Raised {
			raised = true
		}
	}
	if !raised {
		t.Error("the card must take the theme of the field")
	}
}

func TestDatePicker_Disabled(t *testing.T) {
	th := theme.Default().Light
	tree := wtest.TextTree(t, overlay.Overlay{Child: basic.Align{Child: DatePicker{Value: day(2026, 11, 9)}}})
	list := wtest.Ops(tree, wtest.Screen)
	n := wtest.FindRole(tree.Semantics(), semantics.RoleComboBox)
	if n.State&semantics.Disabled == 0 || n.Actions&semantics.ActionTap != 0 {
		t.Errorf("got %+v", n)
	}
	var token bool
	for _, op := range list {
		if op.Color == th.Color.Ink3 || op.Color == th.Color.Fill {
			token = true
		}
	}
	if !token {
		t.Error("a disabled field must draw in the disabled colors")
	}
	wtest.Tap(tree, wtest.Center(n.Rect))
	wtest.Ops(tree, wtest.Screen)
	if wtest.FindRole(tree.Semantics(), semantics.RoleDialog) != nil {
		t.Error("a disabled field must not open")
	}
}

func TestDatePicker_IsThirtyHighAndHovers(t *testing.T) {
	tree := wtest.TextTree(t, overlay.Overlay{Child: basic.Align{Child: DatePicker{OnChange: func(time.Time) {}}}})
	var h float32
	for _, op := range wtest.Ops(tree, wtest.Screen) {
		if op.Kind == paint.OpRRect {
			h = op.RRect.Rect.H
			break
		}
	}
	if h != theme.ControlHeight {
		t.Errorf("got height %v", h)
	}
	// Align centers the field; move onto it.
	n := wtest.FindRole(tree.Semantics(), semantics.RoleComboBox)
	rest := testutil.RenderTree(tree, wtest.Screen)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: wtest.Center(n.Rect)})
	testutil.RenderTree(tree, wtest.Screen)
	if !tree.Ticking() {
		t.Fatal("hover must animate")
	}
	now := time.Second
	wtest.Settled(t, tree, wtest.Screen, &now)
	if testutil.RenderTree(tree, wtest.Screen) == rest {
		t.Error("hover must show")
	}
}

func TestDatePicker_GoingAwayClosesTheCalendar(t *testing.T) {
	var state *wtest.HolderState
	tree := wtest.TextTree(t, overlay.Overlay{Child: wtest.Holder{First: basic.Align{Child: DatePicker{Value: day(2026, 11, 9), OnChange: func(time.Time) {}}}, State: &state}})
	wtest.Ops(tree, wtest.Screen)
	wtest.Tap(tree, wtest.Center(wtest.FindRole(tree.Semantics(), semantics.RoleComboBox).Rect))
	wtest.Ops(tree, wtest.Screen)
	if wtest.FindRole(tree.Semantics(), semantics.RoleDialog) == nil {
		t.Fatal("not open")
	}
	state.SetState(func() { state.Child = basic.SizedBox{} })
	wtest.Ops(tree, wtest.Screen)
	if wtest.FindRole(tree.Semantics(), semantics.RoleDialog) != nil {
		t.Error("the calendar must go with its field")
	}
}

func TestDatePicker_ZeroValueWorks(t *testing.T) {
	tree := wtest.TextTree(t, overlay.Overlay{Child: DatePicker{}})
	wtest.Ops(tree, wtest.Screen)
}
