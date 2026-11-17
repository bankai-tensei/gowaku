package field

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

func hm(h, m int) time.Duration { return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute }

// timeHost keeps the value of a TimePicker, as an application does.
type timeHost struct {
	core.StatefulBase
	picker TimePicker
}

type timeHostState struct {
	core.StateOf[timeHost]
	value time.Duration
}

func (timeHost) CreateState() core.State { return &timeHostState{} }

func (s *timeHostState) InitState() { s.value = s.Widget().picker.Value }

func (s *timeHostState) Build(core.BuildContext) core.Widget {
	p := s.Widget().picker
	p.Value = s.value
	p.OnChange = func(v time.Duration) { s.SetState(func() { s.value = v }) }
	return p
}

// timeTree renders a time picker that keeps its value, in a window of tag,
// and puts the focus on its first part.
func timeTree(t *testing.T, tag language.Tag, p TimePicker) *core.Tree {
	t.Helper()
	tree := localeTree(t, tag, timeHost{picker: p})
	press(tree, input.KeyTab, 0)
	return tree
}

// timeValue returns the time the field shows.
func timeValue(tree *core.Tree) string {
	return wtest.FindRole(tree.Semantics(), semantics.RoleGroup).Value
}

func TestTimePicker_PartsFollowTheLocale(t *testing.T) {
	cases := []struct {
		name   string
		tag    language.Tag
		p      TimePicker
		value  string
		labels []string
	}{
		{"日本語は 24 時間", language.Japanese, TimePicker{Value: hm(9, 5)}, "09:05", []string{"時", "分"}},
		{"英語は 12 時間", language.English, TimePicker{Value: hm(21, 5)}, "9:05 PM", []string{"Hour", "Minute", "AM/PM"}},
		{"日本語の 12 時間は午前が先", language.Japanese, TimePicker{Value: hm(9, 5), HourCycle: HourCycle12}, "午前 9:05", []string{"午前/午後", "時", "分"}},
		{"英語の 24 時間と秒", language.English, TimePicker{Value: hm(21, 5) + 7*time.Second, HourCycle: HourCycle24, Seconds: true}, "21:05:07", []string{"Hour", "Minute", "Second"}},
		{"真夜中は 12 AM", language.English, TimePicker{}, "12:00 AM", []string{"Hour", "Minute", "AM/PM"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.p
			p.Label = "時刻"
			p.OnChange = func(time.Duration) {}
			tree := localeTree(t, tc.tag, p)
			root := tree.Semantics()
			g := wtest.FindRole(root, semantics.RoleGroup)
			if g.Label != "時刻" || g.Value != tc.value {
				t.Errorf("got %q %q, want %q", g.Label, g.Value, tc.value)
			}
			parts := wtest.RolesOf(root, semantics.RoleStepper)
			if len(parts) != len(tc.labels) {
				t.Fatalf("got %d parts", len(parts))
			}
			for i, want := range tc.labels {
				if parts[i].Label != want {
					t.Errorf("part %d: got %q, want %q", i, parts[i].Label, want)
				}
				if len(parts[i].Children) != 0 {
					t.Errorf("part %d says its digits twice", i)
				}
			}
		})
	}
}

func TestTimePicker_PartRange(t *testing.T) {
	tree := localeTree(t, language.English, TimePicker{Value: hm(14, 30), MinuteStep: 15, OnChange: func(time.Duration) {}})
	parts := wtest.RolesOf(tree.Semantics(), semantics.RoleStepper)
	want := []semantics.Range{{Min: 1, Max: 12, Value: 2, Step: 1}, {Min: 0, Max: 59, Value: 30, Step: 15}, {Min: 0, Max: 1, Value: 1, Step: 1}}
	for i := range want {
		if parts[i].Range != want[i] {
			t.Errorf("part %d: got %+v", i, parts[i].Range)
		}
		if parts[i].Actions&(semantics.ActionIncrease|semantics.ActionDecrease) == 0 {
			t.Errorf("part %d cannot be stepped", i)
		}
	}
	if parts[0].Value != "2" || parts[1].Value != "30" || parts[2].Value != "PM" {
		t.Errorf("got %q %q %q", parts[0].Value, parts[1].Value, parts[2].Value)
	}
}

func TestTimePicker_ArrowsStep(t *testing.T) {
	cases := []struct {
		name  string
		tag   language.Tag
		p     TimePicker
		right int
		key   input.KeyCode
		want  string
	}{
		{"11 時 AM の次は 12 時 PM", language.English, TimePicker{Value: hm(11, 0)}, 0, input.KeyUp, "12:00 PM"},
		{"11 時 PM の次は 12 時 AM", language.English, TimePicker{Value: hm(23, 15)}, 0, input.KeyUp, "12:15 AM"},
		{"12 時 AM の前は 11 時 PM", language.English, TimePicker{Value: hm(0, 30)}, 0, input.KeyDown, "11:30 PM"},
		{"24 時間は 23 から 0 へ", language.Japanese, TimePicker{Value: hm(23, 0)}, 0, input.KeyUp, "00:00"},
		{"分は繰り上がらない", language.Japanese, TimePicker{Value: hm(9, 59)}, 1, input.KeyUp, "09:00"},
		{"分を下へ", language.Japanese, TimePicker{Value: hm(9, 0)}, 1, input.KeyDown, "09:59"},
		{"刻み 15 で次の倍数へ", language.Japanese, TimePicker{Value: hm(9, 7), MinuteStep: 15}, 1, input.KeyUp, "09:15"},
		{"刻み 15 で前の倍数へ", language.Japanese, TimePicker{Value: hm(9, 7), MinuteStep: 15}, 1, input.KeyDown, "09:00"},
		{"刻み 15 で一周", language.Japanese, TimePicker{Value: hm(9, 45), MinuteStep: 15}, 1, input.KeyUp, "09:00"},
		{"刻み 15 で 0 の前", language.Japanese, TimePicker{Value: hm(9, 0), MinuteStep: 15}, 1, input.KeyDown, "09:45"},
		{"午前午後を切り替える", language.English, TimePicker{Value: hm(9, 0)}, 2, input.KeyUp, "9:00 PM"},
		{"End で 12 時", language.English, TimePicker{Value: hm(13, 0)}, 0, input.KeyEnd, "12:00 PM"},
		{"Home で 1 時", language.English, TimePicker{Value: hm(13, 0)}, 0, input.KeyHome, "1:00 PM"},
		{"秒は 59 から 0", language.Japanese, TimePicker{Value: hm(9, 0) + 59*time.Second, Seconds: true}, 2, input.KeyUp, "09:00:00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := timeTree(t, tc.tag, tc.p)
			for range tc.right {
				press(tree, input.KeyRight, 0)
			}
			press(tree, tc.key, 0)
			if got := timeValue(tree); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTimePicker_Typing(t *testing.T) {
	cases := []struct {
		name  string
		tag   language.Tag
		p     TimePicker
		keys  []input.KeyCode
		want  string
		focus string
	}{
		{"二桁で次へ", language.Japanese, TimePicker{Value: hm(9, 30)}, []input.KeyCode{input.Key1, input.Key4}, "14:30", "分"},
		{"3 以上は一桁で次へ", language.Japanese, TimePicker{Value: hm(9, 30)}, []input.KeyCode{input.Key7}, "07:30", "分"},
		{"一桁目だけなら留まる", language.Japanese, TimePicker{Value: hm(9, 30)}, []input.KeyCode{input.Key2}, "02:30", "時"},
		{"範囲外の二桁目は打ち直し", language.Japanese, TimePicker{Value: hm(9, 30)}, []input.KeyCode{input.Key2, input.Key5}, "05:30", "分"},
		{"時と分を続けて", language.Japanese, TimePicker{Value: hm(9, 30)}, []input.KeyCode{input.Key0, input.Key8, input.Key4, input.Key5}, "08:45", "分"},
		{"12 時間の 0 9", language.English, TimePicker{Value: hm(14, 30)}, []input.KeyCode{input.Key0, input.Key9}, "9:30 PM", "Minute"},
		{"12 時間の 1 2", language.English, TimePicker{Value: hm(14, 30)}, []input.KeyCode{input.Key1, input.Key2}, "12:30 PM", "Minute"},
		{"P で午後", language.English, TimePicker{Value: hm(9, 0)}, []input.KeyCode{input.KeyRight, input.KeyRight, input.KeyP}, "9:00 PM", "AM/PM"},
		{"A で午前", language.English, TimePicker{Value: hm(21, 0)}, []input.KeyCode{input.KeyRight, input.KeyRight, input.KeyA}, "9:00 AM", "AM/PM"},
		{"分は 6 以上で次へ", language.English, TimePicker{Value: hm(9, 0)}, []input.KeyCode{input.KeyRight, input.Key7}, "9:07 AM", "AM/PM"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := timeTree(t, tc.tag, tc.p)
			for _, k := range tc.keys {
				press(tree, k, 0)
			}
			if got := timeValue(tree); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
			if got := wtest.FocusedLabel(tree); got != tc.focus {
				t.Errorf("focus on %q, want %q", got, tc.focus)
			}
		})
	}
}

func TestTimePicker_MovesBetweenParts(t *testing.T) {
	tree := timeTree(t, language.English, TimePicker{Value: hm(9, 0), Seconds: true})
	var seen []string
	for _, k := range []input.KeyCode{input.KeyRight, input.KeyRight, input.KeyRight, input.KeyRight, input.KeyLeft, input.KeyTab} {
		press(tree, k, 0)
		seen = append(seen, wtest.FocusedLabel(tree))
	}
	want := []string{"Minute", "Second", "AM/PM", "AM/PM", "Second", "AM/PM"}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("got %q, want %q", seen, want)
		}
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab, Modifiers: input.ModShift})
	wtest.Ops(tree, pickerScreen)
	if got := wtest.FocusedLabel(tree); got != "Second" {
		t.Errorf("Shift+Tab: got %q", got)
	}
}

func TestTimePicker_ClickFocusesAPart(t *testing.T) {
	tree := localeTree(t, language.English, timeHost{picker: TimePicker{Value: hm(9, 0)}})
	wtest.Tap(tree, wtest.Center(wtest.ByLabel(t, tree.Semantics(), "Minute").Rect))
	wtest.Ops(tree, pickerScreen)
	if got := wtest.FocusedLabel(tree); got != "Minute" {
		t.Fatalf("got %q", got)
	}
	press(tree, input.KeyUp, 0)
	if got := timeValue(tree); got != "9:01 AM" {
		t.Errorf("got %q", got)
	}
}

func TestTimePicker_ScreenReaderSteps(t *testing.T) {
	tree := localeTree(t, language.Japanese, timeHost{picker: TimePicker{Value: hm(9, 0)}})
	root, _ := tree.SemanticsTree()
	n := wtest.ByLabel(t, root, "時")
	if !tree.PerformSemanticsAction(n.ID, semantics.ActionIncrease, core.SemanticsArg{}) {
		t.Fatal("the action must work")
	}
	wtest.Ops(tree, pickerScreen)
	if got := timeValue(tree); got != "10:00" {
		t.Errorf("got %q", got)
	}
}

func TestTimePicker_Disabled(t *testing.T) {
	th := theme.Default().Light
	tree := localeTree(t, language.English, TimePicker{Value: hm(9, 0)})
	g := wtest.FindRole(tree.Semantics(), semantics.RoleGroup)
	if g.State&semantics.Disabled == 0 {
		t.Error("without OnChange the field is disabled")
	}
	var token bool
	for _, op := range wtest.Ops(tree, pickerScreen) {
		if op.Color == th.Color.Ink3 || op.Color == th.Color.Fill {
			token = true
		}
		if op.Kind == paint.OpGlyphs && op.Color == th.Color.Ink {
			t.Error("a disabled field must not draw in Ink")
		}
	}
	if !token {
		t.Error("a disabled field must draw in the disabled colors")
	}
	press(tree, input.KeyTab, 0)
	if wtest.FocusedLabel(tree) != "" {
		t.Error("a disabled field must not take the focus")
	}
}

func TestTimePicker_FocusShowsOnThePart(t *testing.T) {
	th := theme.Default().Light
	tree := timeTree(t, language.English, TimePicker{Value: hm(9, 0)})
	var accent int
	for _, op := range wtest.Ops(tree, pickerScreen) {
		if op.Kind == paint.OpRRect && op.Color == th.Color.Accent {
			accent++
		}
	}
	if accent != 1 {
		t.Errorf("the part in focus must be filled with the accent, got %d", accent)
	}
}

func TestTimePicker_IsThirtyHighAndHovers(t *testing.T) {
	tree := wtest.TextTree(t, basic.Align{Alignment: layout.AlignTopLeft, Child: TimePicker{OnChange: func(time.Duration) {}}})
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
	rest := testutil.RenderTree(tree, wtest.Screen)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: wtest.Center(wtest.FindRole(tree.Semantics(), semantics.RoleGroup).Rect)})
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

func TestHourCycle_String(t *testing.T) {
	for c, want := range map[HourCycle]string{HourCycleLocale: "locale", HourCycle12: "12h", HourCycle24: "24h"} {
		if c.String() != want {
			t.Errorf("got %q", c.String())
		}
	}
}

func TestTimePicker_ZeroValueWorks(t *testing.T) {
	tree := wtest.TextTree(t, TimePicker{})
	wtest.Ops(tree, wtest.Screen)
}
