package field

import (
	"testing"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
)

var (
	jaSymbols = numberSymbols{decimal: '.', group: ','}
	deSymbols = numberSymbols{decimal: ',', group: '.'}
	frSymbols = numberSymbols{decimal: ',', group: ' '}
	chSymbols = numberSymbols{decimal: '.', group: '’'}
)

func TestParseNumber(t *testing.T) {
	for _, tc := range []struct {
		name string
		sym  numberSymbols
		in   string
		want float64
		ok   bool
	}{
		{"ja: 小数", jaSymbols, "1234.5", 1234.5, true},
		{"ja: 桁区切り", jaSymbols, "1,234.5", 1234.5, true},
		{"ja: 全角", jaSymbols, "１，２３４．５", 1234.5, true},
		{"ja: 全角マイナス", jaSymbols, "－５", -5, true},
		{"ja: 長音のマイナス", jaSymbols, "ー5", -5, true},
		{"ja: 前後の空白", jaSymbols, "　 12 ", 12, true},
		{"ja: プラス", jaSymbols, "+3", 3, true},
		{"ja: 先頭の小数点", jaSymbols, ".5", 0.5, true},
		{"ja: 末尾の小数点", jaSymbols, "5.", 5, true},
		{"ja: 区切りの後が3桁でないなら小数点", jaSymbols, "1,5", 1.5, true},
		{"ja: 区切りがずれている", jaSymbols, "1,23,4", 0, false},
		{"ja: 小数部の区切り", jaSymbols, "1.234,5", 0, false},
		{"ja: 小数点が2つ", jaSymbols, "1.2.3", 0, false},
		{"ja: 文字", jaSymbols, "abc", 0, false},
		{"ja: 空", jaSymbols, "", 0, false},
		{"ja: 符号だけ", jaSymbols, "-", 0, false},
		{"ja: 途中の符号", jaSymbols, "1-2", 0, false},
		{"ja: 指数", jaSymbols, "1e5", 0, false},
		{"ja: アラビア・インド数字", jaSymbols, "١٢", 12, true},
		{"de: 桁区切りと小数点", deSymbols, "1.234,5", 1234.5, true},
		{"de: 小数点だけ", deSymbols, "1234,5", 1234.5, true},
		{"de: 点1つで3桁が続かないなら小数点", deSymbols, "1.5", 1.5, true},
		{"de: 点の後が3桁なら桁区切り", deSymbols, "1.500", 1500, true},
		{"de: 英語の書き方", deSymbols, "1,234.5", 0, false},
		{"de: 全角", deSymbols, "１．２３４，５", 1234.5, true},
		{"fr: 空白の区切り", frSymbols, "1 234,5", 1234.5, true},
		{"fr: 改行しない空白", frSymbols, "1 234,5", 1234.5, true},
		{"fr: 狭い空白", frSymbols, "1 234 567,25", 1234567.25, true},
		{"fr: 点", frSymbols, "1.5", 1.5, true},
		{"ch: アポストロフィ", chSymbols, "1'234.5", 1234.5, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseNumber(tc.in, tc.sym)
			if ok != tc.ok || ok && got != tc.want {
				t.Errorf("parseNumber(%q) = %v, %v; want %v, %v", tc.in, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestFormatNumber(t *testing.T) {
	for _, tc := range []struct {
		v        float64
		digits   int
		sym      numberSymbols
		grouping bool
		want     string
	}{
		{1234567.891, 2, jaSymbols, true, "1,234,567.89"},
		{1234567.891, 2, deSymbols, true, "1.234.567,89"},
		{1234.5, 1, frSymbols, true, "1 234,5"},
		{1234.5, 1, deSymbols, false, "1234,5"},
		{-0.0001, 2, jaSymbols, false, "0.00"},
		{-12, 0, jaSymbols, true, "-12"},
		{123, 0, jaSymbols, true, "123"},
		{2.5, -1, jaSymbols, false, "2"},
	} {
		if got := formatNumber(tc.v, tc.digits, tc.sym, tc.grouping); got != tc.want {
			t.Errorf("formatNumber(%v, %d) = %q, want %q", tc.v, tc.digits, got, tc.want)
		}
	}
}

func TestDecimalsOf(t *testing.T) {
	for step, want := range map[float64]int{1: 0, 0.5: 1, 0.25: 2, 0.1: 1, 0.001: 3, 10: 0} {
		if got := decimalsOf(step); got != want {
			t.Errorf("decimalsOf(%v) = %d, want %d", step, got, want)
		}
	}
}

// numberPick records what a NumberField asks for.
type numberPick struct {
	changes []float64
	// refuse makes the parent keep its value.
	refuse bool
}

func numberRig(t *testing.T, locale language.Tag, n NumberField, p *numberPick) *fieldRig {
	t.Helper()
	if n.Label == "" {
		n.Label = "数量"
	}
	r := newFieldRig(t, basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1, Locale: locale}, Child: basic.SizedBox{Width: 200, Child: formHolder[float64]{
		Initial: n.Value,
		Build: func(v float64, set func(float64)) core.Widget {
			w := n
			w.Value = v
			w.OnChange = func(x float64) {
				p.changes = append(p.changes, x)
				if p.refuse {
					// The parent builds again with the value it keeps.
					set(v)
				} else {
					set(x)
				}
			}
			return w
		},
	}}})
	return r
}

// replaceAll selects the text of the field with the keyboard and types s.
func (r *fieldRig) replaceAll(s string) {
	r.Key(input.KeyA, input.ModControl)
	r.typeText(s)
}

func (p *numberPick) want(t *testing.T, want ...float64) {
	t.Helper()
	if len(p.changes) != len(want) {
		t.Fatalf("changes %v, want %v", p.changes, want)
	}
	for i := range want {
		if p.changes[i] != want[i] {
			t.Fatalf("changes %v, want %v", p.changes, want)
		}
	}
}

func (r *fieldRig) wantText(want string) {
	r.T.Helper()
	if got := r.editing().Text(); got != want {
		r.T.Errorf("text %q, want %q", got, want)
	}
}

func TestNumberField_TypesAndCommitsInJapanese(t *testing.T) {
	var p numberPick
	r := numberRig(t, language.Japanese, NumberField{Value: 1, Step: 0.5}, &p)
	r.focusField("数量")
	r.wantText("1.0")
	// A Japanese IME types full-width digits.
	r.replaceAll("１２．５")
	p.want(t)
	r.Key(input.KeyEnter, 0)
	p.want(t, 12.5)
	r.wantText("12.5")
	if n := r.must(semantics.RoleStepper, "数量"); n.Value != "12.5" || n.Range.Value != 12.5 || n.Range.Step != 0.5 {
		t.Errorf("got %+v", n)
	}
}

func TestNumberField_FollowsGermanAndFrenchSeparators(t *testing.T) {
	var p numberPick
	r := numberRig(t, language.German, NumberField{Value: 1234.5, Precision: 2, Grouping: true}, &p)
	r.focusField("数量")
	r.wantText("1.234,50")
	r.replaceAll("3,25")
	r.Key(input.KeyEnter, 0)
	p.want(t, 3.25)
	r.wantText("3,25")
	r.replaceAll("2.000,5")
	r.Key(input.KeyEnter, 0)
	p.want(t, 3.25, 2000.5)
	r.wantText("2.000,50")

	var q numberPick
	r = numberRig(t, language.French, NumberField{Value: 1234.5, Step: 0.1, Grouping: true}, &q)
	r.focusField("数量")
	r.wantText("1 234,5")
	r.replaceAll("12 345,7")
	r.Key(input.KeyEnter, 0)
	q.want(t, 12345.7)
	r.wantText("12 345,7")
}

func TestNumberField_ClampsAndStopsAtTheLimits(t *testing.T) {
	var p numberPick
	r := numberRig(t, language.Japanese, NumberField{Value: 9, Min: 0, Max: 10}, &p)
	r.focusField("数量")
	r.Key(input.KeyUp, 0)
	p.want(t, 10)
	r.Key(input.KeyUp, 0)
	p.want(t, 10)
	r.wantText("10")
	r.replaceAll("25")
	r.Key(input.KeyEnter, 0)
	r.wantText("10")
	p.want(t, 10)
	r.replaceAll("-3")
	r.Key(input.KeyEnter, 0)
	p.want(t, 10, 0)
	r.Key(input.KeyDown, 0)
	p.want(t, 10, 0)
	r.wantText("0")
}

func TestNumberField_StepsWithModifiersAndPages(t *testing.T) {
	var p numberPick
	r := numberRig(t, language.English, NumberField{Value: 5, Precision: 1}, &p)
	r.focusField("数量")
	r.Key(input.KeyUp, 0)
	r.Key(input.KeyUp, input.ModShift)
	r.Key(input.KeyDown, input.ModControl)
	r.Key(input.KeyPageDown, 0)
	r.Key(input.KeyPageUp, 0)
	p.want(t, 6, 16, 15.9, 5.9, 15.9)
	r.wantText("15.9")
	// Stepping starts from what was typed, if it is a number.
	r.replaceAll("2")
	r.Key(input.KeyUp, 0)
	p.want(t, 6, 16, 15.9, 5.9, 15.9, 3)
	r.wantText("3.0")
}

func TestNumberField_InvalidTextShowsAnErrorAndGoesBack(t *testing.T) {
	var p numberPick
	r := numberRig(t, language.Japanese, NumberField{Value: 4}, &p)
	r.focusField("数量")
	r.replaceAll("abc")
	n := r.must(semantics.RoleTextField, "数量")
	if n.State&semantics.Invalid == 0 || n.Hint != "数値を入力してください" {
		t.Errorf("got %+v", n)
	}
	r.Key(input.KeyEnter, 0)
	p.want(t)
	r.wantText("4")
	if n := r.must(semantics.RoleTextField, "数量"); n.State&semantics.Invalid != 0 {
		t.Errorf("the error must go with the text, got %+v", n)
	}
	// Escape puts the value back too.
	r.replaceAll("9")
	r.Key(input.KeyEscape, 0)
	r.wantText("4")
	p.want(t)
	// Leaving the field commits.
	r.replaceAll("7")
	r.tree.RequestFocus(nil)
	r.frame()
	p.want(t, 7)
}

func TestNumberField_EnterDuringCompositionIsTheIMEs(t *testing.T) {
	var p numberPick
	r := numberRig(t, language.Japanese, NumberField{Value: 0}, &p)
	r.focusField("数量")
	r.Key(input.KeyA, input.ModControl)
	r.compose(&input.Composition{Text: "１２", Cursor: len("１２")})
	r.Key(input.KeyEnter, 0)
	r.Key(input.KeyUp, 0)
	p.want(t)
	r.compose(nil)
	r.typeText("１２")
	r.Key(input.KeyEnter, 0)
	p.want(t, 12)
}

func TestNumberField_WheelStepsOnlyWithTheKeyboard(t *testing.T) {
	var p numberPick
	r := numberRig(t, language.Japanese, NumberField{Value: 3}, &p)
	field := r.must(semantics.RoleTextField, "数量").Rect
	wheel := func(notches float32) {
		r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: geom.Offset{X: field.X + 20, Y: field.Y + field.H/2}, Scroll: geom.Offset{Y: notches}})
		r.frame()
	}
	wheel(1)
	p.want(t)
	r.focusField("数量")
	wheel(1)
	wheel(-0.5)
	wheel(-0.5)
	wheel(-2)
	p.want(t, 4, 3, 1)
}

func TestNumberField_ScrubHandleDrags(t *testing.T) {
	var p numberPick
	r := numberRig(t, language.Japanese, NumberField{Value: 10}, &p)
	field := r.must(semantics.RoleTextField, "数量").Rect
	// The handle sits at the end of the box, inside its border.
	box := r.must(semantics.RoleStepper, "数量").Rect
	at := geom.Offset{X: box.X + box.W - numberGripInset - 8, Y: field.Y + field.H/2}
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at})
	r.frame()
	r.wantFocus(semantics.RoleTextField, "数量")
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: at.Add(geom.Offset{X: 9})})
	r.frame()
	p.want(t, 12)
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: at.Add(geom.Offset{X: 13}), Modifiers: input.ModShift})
	r.frame()
	p.want(t, 12, 22)
	// The steps carry what a move leaves over: 1 + 1 - 16 pixels is three
	// steps down.
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: at.Add(geom.Offset{X: -3})})
	r.frame()
	p.want(t, 12, 22, 19)
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: at})
	r.frame()
	r.wantText("19")
}

func TestNumberField_SemanticsActions(t *testing.T) {
	var p numberPick
	r := numberRig(t, language.English, NumberField{Value: 2, Min: 0, Max: 5}, &p)
	n := r.must(semantics.RoleStepper, "数量")
	if n.Range != (semantics.Range{Min: 0, Max: 5, Value: 2, Step: 1}) {
		t.Errorf("range %+v", n.Range)
	}
	r.act(n, semantics.ActionIncrease)
	p.want(t, 3)
	if !r.tree.PerformSemanticsAction(r.must(semantics.RoleStepper, "数量").ID, semantics.ActionSetValue, core.SemanticsArg{Number: 9}) {
		t.Fatal("set value failed")
	}
	r.frame()
	p.want(t, 3, 5)
	if n := r.must(semantics.RoleStepper, "数量"); n.Value != "5" {
		t.Errorf("got %+v", n)
	}
}

func TestNumberField_ParentWins(t *testing.T) {
	p := numberPick{refuse: true}
	r := numberRig(t, language.English, NumberField{Value: 2}, &p)
	r.focusField("数量")
	r.Key(input.KeyUp, 0)
	p.want(t, 3)
	// The parent kept 2 and built again: the field shows its value.
	r.wantText("2")
	r.Key(input.KeyUp, 0)
	p.want(t, 3, 3)
}

func TestNumberField_ExternalValueReplacesTheText(t *testing.T) {
	var swap func(core.Widget)
	r := newFieldRig(t, basic.SizedBox{Width: 200, Child: swapper{set: &swap, first: NumberField{Label: "数量", Value: 1, OnChange: func(float64) {}}}})
	r.focusField("数量")
	r.wantText("1")
	swap(NumberField{Label: "数量", Value: 42, OnChange: func(float64) {}})
	r.frame()
	r.wantText("42")
}

// TestNumberField_NewValueDropsTheDraft gives the field a new Value while text
// is typed and not committed: the new value replaces the text, and Enter
// reports nothing stale. A build with the same Value keeps the typed text.
func TestNumberField_NewValueDropsTheDraft(t *testing.T) {
	var changes []float64
	onChange := func(v float64) { changes = append(changes, v) }
	var swap func(core.Widget)
	r := newFieldRig(t, basic.SizedBox{Width: 200, Child: swapper{set: &swap, first: NumberField{Label: "数量", Value: 1, OnChange: onChange}}})
	r.focusField("数量")
	r.replaceAll("12")
	// A build for another reason, with the same value, keeps what was typed.
	swap(NumberField{Label: "数量", Value: 1, Placeholder: "個数", OnChange: onChange})
	r.frame()
	r.wantText("12")
	swap(NumberField{Label: "数量", Value: 42, OnChange: onChange})
	r.frame()
	r.wantText("42")
	r.Key(input.KeyEnter, 0)
	if len(changes) != 0 {
		t.Errorf("changes %v after Enter on the parent's value, want none", changes)
	}
	r.wantText("42")
	// The same when the new value spells as the last one did: 42.2 in whole
	// numbers shows as 42, so the text given to the field stays the same.
	r.replaceAll("7")
	swap(NumberField{Label: "数量", Value: 42.2, Precision: -1, OnChange: onChange})
	r.frame()
	r.wantText("42")
	r.Key(input.KeyEnter, 0)
	if len(changes) != 0 {
		t.Errorf("changes %v after Enter on the parent's value, want none", changes)
	}
}

func TestNumberField_ZeroValueAndDisabled(t *testing.T) {
	r := newFieldRig(t, basic.SizedBox{Width: 200, Child: NumberField{}})
	n := r.must(semantics.RoleStepper, "")
	if n.State&semantics.Disabled == 0 || n.Value != "0" {
		t.Errorf("got %+v", n)
	}
	r.Key(input.KeyTab, 0)
	if r.tree.Focused() != nil {
		t.Error("a disabled field takes no focus")
	}
}

// swapper shows first until set replaces it.
type swapper struct {
	core.StatefulBase
	set   *func(core.Widget)
	first core.Widget
}

type swapperState struct {
	core.StateOf[swapper]
	child core.Widget
}

func (swapper) CreateState() core.State { return &swapperState{} }

func (s *swapperState) InitState() {
	s.child = s.Widget().first
	*s.Widget().set = func(w core.Widget) { s.SetState(func() { s.child = w }) }
}

func (s *swapperState) Build(core.BuildContext) core.Widget { return s.child }
