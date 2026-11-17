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

var pickerScreen = geom.Size{W: 600, H: 500}

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// localeTree renders w in a window of the locale tag.
func localeTree(t testing.TB, tag language.Tag, w core.Widget) *core.Tree {
	t.Helper()
	tree := wtest.TextTree(t, overlay.Overlay{Child: basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1, Locale: tag}, Child: basic.Align{
		Alignment: layout.AlignTopLeft, Child: w,
	}}})
	wtest.Ops(tree, pickerScreen)
	return tree
}

// press sends a key and renders.
func press(tree *core.Tree, k input.KeyCode, m input.Modifiers) {
	tree.DispatchKey(core.KeyEvent{Key: k, Modifiers: m})
	wtest.Ops(tree, pickerScreen)
}

func TestCalendar_WeekStartComesFromTheLocale(t *testing.T) {
	cases := []struct {
		name  string
		tag   language.Tag
		first string
		want  string
	}{
		{"英語は日曜から", language.English, "Sunday", "Sunday, November 1, 2026"},
		{"英国は月曜から", language.BritishEnglish, "Monday", "Monday, October 26, 2026"},
		{"日本語は日曜から", language.Japanese, "日曜日", "2026年11月1日 日曜日"},
		{"エジプトは土曜から", language.MustParse("ar-EG"), "Saturday", "Saturday, October 31, 2026"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := localeTree(t, tc.tag, Calendar{Value: day(2026, 11, 9), OnChange: func(time.Time) {}})
			root := tree.Semantics()
			heads := wtest.RolesOf(root, semantics.RoleColumnHeader)
			if len(heads) != 7 || heads[0].Label != tc.first {
				t.Fatalf("got %d heads, first %q", len(heads), heads[0].Label)
			}
			cells := wtest.RolesOf(root, semantics.RoleGridCell)
			if len(cells) != 42 || cells[0].Label != tc.want {
				t.Fatalf("got %d cells, first %q, want %q", len(cells), cells[0].Label, tc.want)
			}
			if rows := wtest.RolesOf(root, semantics.RoleRow); len(rows) != 7 {
				t.Errorf("got %d rows", len(rows))
			}
		})
	}
}

func TestCalendar_Semantics(t *testing.T) {
	tree := localeTree(t, language.Japanese, Calendar{Value: day(2026, 11, 9), Today: day(2026, 11, 3), OnChange: func(time.Time) {}})
	root := tree.Semantics()
	grid := wtest.FindRole(root, semantics.RoleGrid)
	if grid == nil || grid.Label != "2026年11月" {
		t.Fatalf("got grid %+v", grid)
	}
	sel := wtest.ByLabel(t, root, "2026年11月9日 月曜日")
	if sel.Role != semantics.RoleGridCell || sel.State&semantics.Selected == 0 || sel.Actions&semantics.ActionTap == 0 {
		t.Errorf("got %+v", sel)
	}
	if other := wtest.ByLabel(t, root, "2026年11月10日 火曜日"); other.State&semantics.Selected != 0 {
		t.Error("only the value is selected")
	}
	for _, label := range []string{"前の月", "次の月"} {
		if n := wtest.ByLabel(t, root, label); n.Role != semantics.RoleButton {
			t.Errorf("%s: got %v", label, n.Role)
		}
	}
	// The cells say the whole date and nothing else: no text node of "9".
	for _, c := range wtest.RolesOf(grid, semantics.RoleGridCell) {
		if len(c.Children) != 0 {
			t.Fatalf("a cell has children: %+v", c.Children[0])
		}
	}
}

func TestCalendar_KeysMoveTheFocusAndTurnTheMonth(t *testing.T) {
	cases := []struct {
		name  string
		from  time.Time
		key   input.KeyCode
		mod   input.Modifiers
		to    string
		title string
	}{
		{"右で翌日、年をまたぐ", day(2026, 12, 31), input.KeyRight, 0, "Friday, January 1, 2027", "January 2027"},
		{"左で前日、年をまたぐ", day(2027, 1, 1), input.KeyLeft, 0, "Thursday, December 31, 2026", "December 2026"},
		{"下で翌週", day(2026, 11, 9), input.KeyDown, 0, "Monday, November 16, 2026", "November 2026"},
		{"上で前週", day(2026, 11, 2), input.KeyUp, 0, "Monday, October 26, 2026", "October 2026"},
		{"PageUp は月末に寄せる", day(2026, 3, 31), input.KeyPageUp, 0, "Saturday, February 28, 2026", "February 2026"},
		{"PageDown で年をまたぐ", day(2026, 12, 15), input.KeyPageDown, 0, "Friday, January 15, 2027", "January 2027"},
		{"閏日から Shift+PageDown", day(2024, 2, 29), input.KeyPageDown, input.ModShift, "Friday, February 28, 2025", "February 2025"},
		{"閏年の Shift+PageUp", day(2025, 2, 28), input.KeyPageUp, input.ModShift, "Wednesday, February 28, 2024", "February 2024"},
		{"Home は週の始め", day(2026, 11, 12), input.KeyHome, 0, "Sunday, November 8, 2026", "November 2026"},
		{"End は週の終わり", day(2026, 11, 12), input.KeyEnd, 0, "Saturday, November 14, 2026", "November 2026"},
		{"End で月をまたぐ", day(2026, 11, 30), input.KeyEnd, 0, "Saturday, December 5, 2026", "December 2026"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := localeTree(t, language.English, Calendar{Value: tc.from, OnChange: func(time.Time) {}, autofocus: true})
			en := basic.EnglishStrings()
			if got := wtest.FocusedLabel(tree); got != en.SpokenDate(tc.from) {
				t.Fatalf("the value must take the focus, got %q", got)
			}
			press(tree, tc.key, tc.mod)
			if got := wtest.FocusedLabel(tree); got != tc.to {
				t.Errorf("focus on %q, want %q", got, tc.to)
			}
			if g := wtest.FindRole(tree.Semantics(), semantics.RoleGrid); g.Label != tc.title {
				t.Errorf("month %q, want %q", g.Label, tc.title)
			}
		})
	}
}

func TestCalendar_MondayWeekHomeAndEnd(t *testing.T) {
	tree := localeTree(t, language.BritishEnglish, Calendar{Value: day(2026, 11, 8), OnChange: func(time.Time) {}, autofocus: true})
	press(tree, input.KeyHome, 0)
	if got := wtest.FocusedLabel(tree); got != "Monday, November 2, 2026" {
		t.Errorf("got %q", got)
	}
	press(tree, input.KeyEnd, 0)
	if got := wtest.FocusedLabel(tree); got != "Sunday, November 8, 2026" {
		t.Errorf("got %q", got)
	}
}

func TestCalendar_EnterAndClickChoose(t *testing.T) {
	var got time.Time
	zone := time.FixedZone("JST", 9*3600)
	tree := localeTree(t, language.English, Calendar{
		Value: time.Date(2026, 11, 9, 15, 30, 0, 0, zone), OnChange: func(d time.Time) { got = d }, autofocus: true,
	})
	press(tree, input.KeyRight, 0)
	press(tree, input.KeyEnter, 0)
	if want := time.Date(2026, 11, 10, 0, 0, 0, 0, zone); !got.Equal(want) || got.Location() != zone {
		t.Errorf("got %v, want %v", got, want)
	}
	n := wtest.ByLabel(t, tree.Semantics(), "Wednesday, November 25, 2026")
	wtest.Tap(tree, wtest.Center(n.Rect))
	wtest.Ops(tree, pickerScreen)
	if got.Day() != 25 {
		t.Errorf("a click must choose, got %v", got)
	}
	if wtest.FocusedLabel(tree) != "Wednesday, November 25, 2026" {
		t.Errorf("a click must move the focus, got %q", wtest.FocusedLabel(tree))
	}
}

func TestCalendar_ClickOnAnotherMonthTurns(t *testing.T) {
	var got time.Time
	tree := localeTree(t, language.English, Calendar{Value: day(2026, 11, 9), OnChange: func(d time.Time) { got = d }})
	n := wtest.ByLabel(t, tree.Semantics(), "Tuesday, December 1, 2026")
	wtest.Tap(tree, wtest.Center(n.Rect))
	wtest.Ops(tree, pickerScreen)
	if got != day(2026, 12, 1) {
		t.Errorf("got %v", got)
	}
	if g := wtest.FindRole(tree.Semantics(), semantics.RoleGrid); g.Label != "December 2026" {
		t.Errorf("got %q", g.Label)
	}
}

func TestCalendar_MinMaxAndSelectable(t *testing.T) {
	calls := 0
	weekdays := func(d time.Time) bool { return d.Weekday() != time.Saturday && d.Weekday() != time.Sunday }
	tree := localeTree(t, language.English, Calendar{
		Value: day(2026, 11, 9), Min: day(2026, 11, 5), Max: day(2026, 11, 20), Selectable: weekdays,
		OnChange: func(time.Time) { calls++ }, autofocus: true,
	})
	root := tree.Semantics()
	for label, want := range map[string]bool{
		"Wednesday, November 4, 2026": false, // before Min
		"Thursday, November 5, 2026":  true,
		"Saturday, November 7, 2026":  false, // not selectable
		"Friday, November 20, 2026":   true,
		"Monday, November 23, 2026":   false, // after Max
	} {
		n := wtest.ByLabel(t, root, label)
		if got := n.State&semantics.Disabled == 0; got != want {
			t.Errorf("%s: enabled %v, want %v", label, got, want)
		}
	}
	if wtest.ByLabel(t, root, "Previous month").State&semantics.Disabled == 0 {
		t.Error("the month before Min must be out of reach")
	}
	if wtest.ByLabel(t, root, "Next month").State&semantics.Disabled == 0 {
		t.Error("the month after Max must be out of reach")
	}
	// The arrows stop at the ends.
	press(tree, input.KeyPageUp, 0)
	if got := wtest.FocusedLabel(tree); got != "Thursday, November 5, 2026" {
		t.Errorf("got %q", got)
	}
	press(tree, input.KeyRight, 0)
	press(tree, input.KeyRight, 0) // Saturday 7
	press(tree, input.KeyEnter, 0)
	if calls != 0 {
		t.Error("a day that is not selectable must not be chosen")
	}
	press(tree, input.KeyPageDown, input.ModShift)
	if got := wtest.FocusedLabel(tree); got != "Friday, November 20, 2026" {
		t.Errorf("got %q", got)
	}
}

func TestCalendar_ButtonsTurnTheMonth(t *testing.T) {
	tree := localeTree(t, language.English, Calendar{Value: day(2026, 1, 31), OnChange: func(time.Time) {}})
	wtest.Tap(tree, wtest.Center(wtest.ByLabel(t, tree.Semantics(), "Previous month").Rect))
	wtest.Ops(tree, pickerScreen)
	if g := wtest.FindRole(tree.Semantics(), semantics.RoleGrid); g.Label != "December 2025" {
		t.Fatalf("got %q", g.Label)
	}
	for range 3 {
		wtest.Tap(tree, wtest.Center(wtest.ByLabel(t, tree.Semantics(), "Next month").Rect))
		wtest.Ops(tree, pickerScreen)
	}
	if g := wtest.FindRole(tree.Semantics(), semantics.RoleGrid); g.Label != "March 2026" {
		t.Errorf("got %q", g.Label)
	}
}

func TestCalendar_FollowsTheValue(t *testing.T) {
	tree, swap := wtest.SwapTree(t, Calendar{Value: day(2026, 11, 9), OnChange: func(time.Time) {}})
	wtest.Ops(tree, pickerScreen)
	swap(Calendar{Value: day(2027, 3, 2), OnChange: func(time.Time) {}})
	wtest.Ops(tree, pickerScreen)
	if g := wtest.FindRole(tree.Semantics(), semantics.RoleGrid); g.Label != "March 2027" {
		t.Errorf("got %q", g.Label)
	}
}

func TestCalendar_TabReachesOneDay(t *testing.T) {
	tree := localeTree(t, language.English, Calendar{Value: day(2026, 11, 9), OnChange: func(time.Time) {}})
	var seen []string
	for range 5 {
		press(tree, input.KeyTab, 0)
		seen = append(seen, wtest.FocusedLabel(tree))
	}
	want := []string{"November 2026", "Previous month", "Next month", "Monday, November 9, 2026", "November 2026"}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("got %q, want %q", seen, want)
		}
	}
}

func TestCalendar_LooksFollowTheDesign(t *testing.T) {
	th := theme.Default().Light
	tree := localeTree(t, language.English, Calendar{Value: day(2026, 11, 9), Today: day(2026, 11, 3), OnChange: func(time.Time) {}})
	var accent, todayInk bool
	for _, op := range wtest.Ops(tree, pickerScreen) {
		if op.Kind == paint.OpRRect && op.Color == th.Color.Accent && op.RRect.Rect.W == calendarCell {
			accent = true
		}
		if op.Kind == paint.OpGlyphs && op.Color == th.Color.AccentInk {
			todayInk = true
		}
	}
	if !accent {
		t.Error("the value must be filled with the accent")
	}
	if !todayInk {
		t.Error("today must be in the accent ink")
	}
	// Hover fills the day under the pointer.
	n := wtest.ByLabel(t, tree.Semantics(), "Tuesday, November 17, 2026")
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: wtest.Center(n.Rect)})
	var fill bool
	for _, op := range wtest.Ops(tree, pickerScreen) {
		if op.Kind == paint.OpRRect && op.Color == th.Color.Fill && op.RRect.Rect.W == calendarCell {
			fill = true
		}
	}
	if !fill {
		t.Error("hover must fill the day")
	}
}

func TestCalendar_DisabledUsesTokens(t *testing.T) {
	th := theme.Default().Light
	tree := localeTree(t, language.English, Calendar{Value: day(2026, 11, 9)})
	if wtest.FindRole(tree.Semantics(), semantics.RoleGrid).State&semantics.Disabled == 0 {
		t.Error("without OnChange the calendar is disabled")
	}
	for _, op := range wtest.Ops(tree, pickerScreen) {
		if op.Kind == paint.OpGlyphs && op.Color == th.Color.Ink {
			t.Fatal("a disabled calendar must draw its days in Ink3")
		}
	}
}

// markedCalendar marks four days of November 2026: the 5th with four colors,
// the 6th with a clear one and red, the 9th (the value) and December 2nd
// (outside the month) with red. calls counts the calls of Marks.
func markedCalendar(calls *int) Calendar {
	red, green, blue, gray := geom.Hex(0xd03030), geom.Hex(0x30a050), geom.Hex(0x3050d0), geom.Hex(0x808080)
	return Calendar{Value: day(2026, 11, 9), Today: day(2026, 11, 3), OnChange: func(time.Time) {},
		Marks: func(d time.Time) []geom.Color {
			if calls != nil {
				*calls++
			}
			switch d {
			case day(2026, 11, 5):
				return []geom.Color{red, green, blue, gray}
			case day(2026, 11, 6):
				return []geom.Color{{}, red}
			case day(2026, 11, 9), day(2026, 12, 2):
				return []geom.Color{red}
			}
			return nil
		}}
}

// dotsIn returns the colors of the dots drawn inside r, from the left.
func dotsIn(all []paint.Op, r geom.Rect) []geom.Color {
	var out []geom.Color
	for _, op := range all {
		b := op.RRect.Rect
		if op.Kind != paint.OpRRect || b.W != calendarDot || b.H != calendarDot {
			continue
		}
		if b.X >= r.X && b.Y >= r.Y && b.X+b.W <= r.X+r.W && b.Y+b.H <= r.Y+r.H {
			out = append(out, op.Color)
		}
	}
	return out
}

func TestCalendar_MarksDrawDots(t *testing.T) {
	for _, mode := range []struct {
		name string
		dark bool
	}{{"ライト", false}, {"ダーク", true}} {
		t.Run(mode.name, func(t *testing.T) {
			set := theme.Default()
			th := set.Light
			if mode.dark {
				th = set.Dark
			}
			tree := localeTree(t, language.English, basic.Theme{Data: th, Child: markedCalendar(nil)})
			all := wtest.Ops(tree, pickerScreen)
			root := tree.Semantics()
			cell := func(label string) geom.Rect { return wtest.ByLabel(t, root, label).Rect }
			red, green, blue := geom.Hex(0xd03030), geom.Hex(0x30a050), geom.Hex(0x3050d0)
			cases := []struct {
				name, label string
				want        []geom.Color
			}{
				{"4 色は先頭の 3 色だけ", "Thursday, November 5, 2026, has events", []geom.Color{red, green, blue}},
				{"透明な色は飛ばす", "Friday, November 6, 2026, has events", []geom.Color{red}},
				{"選んだ日は OnAccent", "Monday, November 9, 2026, has events", []geom.Color{th.Color.OnAccent}},
				{"月の外は Ink3", "Wednesday, December 2, 2026, has events", []geom.Color{th.Color.Ink3}},
				{"印の無い日", "Saturday, November 7, 2026", nil},
			}
			for _, tc := range cases {
				got := dotsIn(all, cell(tc.label))
				if len(got) != len(tc.want) {
					t.Errorf("%s: got %d dots, want %d", tc.name, len(got), len(tc.want))
					continue
				}
				for i := range got {
					if got[i] != tc.want[i] {
						t.Errorf("%s: dot %d is %v, want %v", tc.name, i, got[i], tc.want[i])
					}
				}
			}
			// The dots of the chosen day stay inside its circle.
			r := cell("Monday, November 9, 2026, has events")
			for _, op := range all {
				b := op.RRect.Rect
				if op.Kind != paint.OpRRect || b.W != calendarDot || b.X < r.X || b.X > r.X+r.W || b.Y < r.Y || b.Y > r.Y+r.H {
					continue
				}
				for _, p := range []geom.Offset{{X: b.X, Y: b.Y + b.H}, {X: b.X + b.W, Y: b.Y + b.H}} {
					dx, dy := p.X-(r.X+r.W/2), p.Y-(r.Y+r.H/2)
					if dx*dx+dy*dy > calendarCell*calendarCell/4 {
						t.Errorf("a dot at %v leaves the circle of the day", b)
					}
				}
			}
		})
	}
}

func TestCalendar_MarksSayTheDayHasThem(t *testing.T) {
	tree := localeTree(t, language.Japanese, markedCalendar(nil))
	root := tree.Semantics()
	wtest.ByLabel(t, root, "2026年11月5日 木曜日, 予定あり")
	wtest.ByLabel(t, root, "2026年11月7日 土曜日")
	// The marks reach a calendar of another table through the fallback.
	es := basic.Strings{Language: language.Spanish}
	tree = localeTree(t, language.Spanish, basic.Localizations{Strings: []*basic.Strings{&es}, Child: markedCalendar(nil)})
	wtest.ByLabel(t, tree.Semantics(), "Thursday, November 5, 2026, has events")
}

func TestCalendar_MarksAreAskedWhenBuildingNotPainting(t *testing.T) {
	calls := 0
	tree := localeTree(t, language.English, markedCalendar(&calls))
	if calls != calendarDays {
		t.Errorf("a build asks for the %d days once each, got %d", calendarDays, calls)
	}
	calls = 0
	for range 3 {
		var cv paint.Canvas
		cv.Reset()
		tree.Paint(&cv)
	}
	if calls != 0 {
		t.Errorf("painting must not ask for the marks, got %d calls", calls)
	}
}

func TestDatePicker_PassesTheMarks(t *testing.T) {
	m := markedCalendar(nil)
	a := newDateApp(t, language.English, layout.AlignTopLeft, DatePicker{Value: day(2026, 11, 9), Marks: m.Marks})
	a.clickField(t)
	if !a.open(t) {
		t.Fatal("a click must open the calendar")
	}
	if got := wtest.FocusedLabel(a.tree); got != "Monday, November 9, 2026, has events" {
		t.Errorf("got %q", got)
	}
	n := wtest.ByLabel(t, a.tree.Semantics(), "Thursday, November 5, 2026, has events")
	if got := dotsIn(wtest.Ops(a.tree, pickerScreen), n.Rect); len(got) != calendarMarks {
		t.Errorf("got %d dots", len(got))
	}
}

func BenchmarkCalendar_PaintMarks(b *testing.B) {
	tree := core.NewTree(testutil.WithShaper(testutil.NewShaper(b), basic.Align{Child: markedCalendar(nil)}))
	defer tree.Dispose()
	testutil.RenderTree(tree, pickerScreen)
	var cv paint.Canvas
	cv.Reset()
	tree.Paint(&cv)
	b.ReportAllocs()
	for b.Loop() {
		cv.Reset()
		tree.Paint(&cv)
	}
}

func BenchmarkCalendar_Paint(b *testing.B) {
	tree := core.NewTree(testutil.WithShaper(testutil.NewShaper(b), basic.Align{Child: Calendar{
		Value: day(2026, 11, 9), Today: day(2026, 11, 3), OnChange: func(time.Time) {},
	}}))
	defer tree.Dispose()
	testutil.RenderTree(tree, pickerScreen)
	var cv paint.Canvas
	cv.Reset()
	tree.Paint(&cv)
	b.ReportAllocs()
	for b.Loop() {
		cv.Reset()
		tree.Paint(&cv)
	}
}

func BenchmarkCalendar_Layout(b *testing.B) {
	tree := core.NewTree(testutil.WithShaper(testutil.NewShaper(b), basic.Align{Child: Calendar{
		Value: day(2026, 11, 9), OnChange: func(time.Time) {},
	}}))
	defer tree.Dispose()
	testutil.RenderTree(tree, pickerScreen)
	c1, c2 := geom.Tight(pickerScreen), geom.Tight(geom.Size{W: 601, H: 500})
	b.ReportAllocs()
	flip := false
	for b.Loop() {
		flip = !flip
		if flip {
			tree.Layout(c2)
		} else {
			tree.Layout(c1)
		}
	}
}
