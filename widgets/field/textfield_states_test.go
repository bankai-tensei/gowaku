package field_test

import (
	"strings"
	"testing"

	fieldw "github.com/bankai-tensei/gowaku/widgets/field"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

// mountInput puts in at the top left of a window, 240 wide.
func mountInput(t *testing.T, in fieldw.Input) *gowakutest.Tester {
	t.Helper()
	return gowakutest.New(t, basic.Align{Alignment: layout.AlignTopLeft, Child: basic.SizedBox{Width: 240, Child: in}}, pressableFonts)
}

func field(t *testing.T, tt *gowakutest.Tester, label string) *gowakutest.Node {
	t.Helper()
	n, ok := tt.FindRole(semantics.RoleTextField, label)
	if !ok {
		t.Fatalf("no text field %q:\n%s", label, tt.Dump())
	}
	return n
}

func TestInput_DisabledTakesNothing(t *testing.T) {
	changed := 0
	tt := mountInput(t, fieldw.Input{Label: "Name", Value: "Ada", Disabled: true, OnChange: func(string) { changed++ }})
	n := field(t, tt, "Name")
	if n.State&semantics.Disabled == 0 {
		t.Errorf("state %v: want Disabled", n.State)
	}
	if n.Actions != 0 {
		t.Errorf("actions %v: a disabled field offers none", n.Actions)
	}
	tt.Tap("Name")
	tt.Type("x")
	tt.Key(input.KeyBackspace, 0)
	if n := field(t, tt, "Name"); n.Value != "Ada" || n.State&semantics.Focused != 0 {
		t.Errorf("value %q, state %v: a disabled field takes neither the keyboard nor text", n.Value, n.State)
	}
	if changed != 0 {
		t.Errorf("OnChange ran %d times", changed)
	}
	for _, w := range tt.Warnings() {
		if w.Kind == gowakutest.WarnContrast {
			t.Errorf("the gray text of a disabled field is exempt: %v", w)
		}
	}
}

func TestInput_DisablingDropsTheFocus(t *testing.T) {
	tt := gowakutest.New(t, toggledInput{}, pressableFonts)
	tt.Tap("Name")
	if n := field(t, tt, "Name"); n.State&semantics.Focused == 0 {
		t.Fatalf("the field should have the keyboard:\n%s", tt.Dump())
	}
	tt.Tap("Lock")
	tt.Type("x")
	if n := field(t, tt, "Name"); n.State&semantics.Focused != 0 || n.Value != "" {
		t.Errorf("value %q, state %v: a field that is disabled gives the keyboard away", n.Value, n.State)
	}
}

// toggledInput is a field and a button that disables it.
type toggledInput struct{ core.StatefulBase }

func (toggledInput) CreateState() core.State { return &toggledInputState{} }

type toggledInputState struct {
	core.StateOf[toggledInput]
	off bool
}

func (s *toggledInputState) Build(core.BuildContext) core.Widget {
	return basic.Align{Alignment: layout.AlignTopLeft, Child: basic.SizedBox{Width: 240, Child: basic.Column{Children: []core.Widget{
		fieldw.Input{Label: "Name", Disabled: s.off},
		interact.Semantics{Role: semantics.RoleButton, Label: "Lock", OnTap: func() { s.SetState(func() { s.off = true }) },
			Child: basic.SizedBox{Width: 40, Height: 20}},
	}}}}
}

func TestInput_ReadOnlySelectsAndCopiesButKeepsItsText(t *testing.T) {
	changed, submitted := 0, ""
	tt := mountInput(t, fieldw.Input{Label: "Key", Value: "abc-123", ReadOnly: true,
		OnChange: func(string) { changed++ }, OnSubmit: func(s string) { submitted = s }})
	n := field(t, tt, "Key")
	if n.State&semantics.ReadOnly == 0 || n.State&semantics.Disabled != 0 {
		t.Errorf("state %v: want ReadOnly and not Disabled", n.State)
	}
	if n.Actions&semantics.ActionSetValue != 0 || n.Actions&semantics.ActionSetSelection == 0 {
		t.Errorf("actions %v: a reader may select but not set the value", n.Actions)
	}
	tt.Tap("Key")
	if n := field(t, tt, "Key"); n.State&semantics.Focused == 0 {
		t.Fatalf("a read-only field takes the keyboard:\n%s", tt.Dump())
	}
	tt.Type("x")
	tt.Tree().DispatchComposition(&input.Composition{Text: "か"})
	tt.Pump()
	tt.Key(input.KeyBackspace, 0)
	tt.Key(input.KeyDelete, 0)
	tt.SetClipboard("pasted")
	tt.Key(input.KeyV, input.ModControl)
	tt.Key(input.KeyA, input.ModControl)
	tt.Key(input.KeyX, input.ModControl)
	tt.Key(input.KeyZ, input.ModControl)
	if n := field(t, tt, "Key"); n.Value != "abc-123" {
		t.Errorf("value %q: a read-only field keeps its text", n.Value)
	}
	if changed != 0 {
		t.Errorf("OnChange ran %d times", changed)
	}
	// The selection of Ctrl+A is there to copy.
	tt.SetClipboard("")
	tt.Key(input.KeyC, input.ModControl)
	if got := tt.Clipboard(); got != "abc-123" {
		t.Errorf("clipboard %q, want the text", got)
	}
	n = field(t, tt, "Key")
	if n.Text == nil || n.Text.SelectionStart != 0 || n.Text.SelectionEnd != len("abc-123") {
		t.Errorf("selection %+v, want all", n.Text)
	}
	// The caret moves.
	tt.Key(input.KeyHome, 0)
	tt.Key(input.KeyRight, input.ModShift)
	tt.Key(input.KeyC, input.ModControl)
	if got := tt.Clipboard(); got != "a" {
		t.Errorf("clipboard %q, want a", got)
	}
	tt.Key(input.KeyEnter, 0)
	if submitted != "abc-123" {
		t.Errorf("submitted %q: Enter still submits", submitted)
	}
}

func TestInput_MaxLinesStopsTheGrowth(t *testing.T) {
	height := func(t *testing.T, in fieldw.Input) float32 {
		t.Helper()
		tt := mountInput(t, in)
		return field(t, tt, "Note").Rect.H
	}
	lines := func(n int) string { return strings.Repeat("line\n", n-1) + "line" }
	base := fieldw.Input{Label: "Note", HideLabel: true, Multiline: true}
	one := height(t, base)
	in := base
	in.Value = lines(2)
	two := height(t, in)
	step := two - one
	if step <= 0 {
		t.Fatalf("a multiline field grows: %v then %v", one, two)
	}
	in.MaxLines = 3
	for _, c := range []struct {
		n    int
		want float32
	}{{1, one}, {3, one + 2*step}, {4, one + 2*step}, {9, one + 2*step}} {
		in.Value = lines(c.n)
		if got := height(t, in); got != c.want {
			t.Errorf("%d lines with MaxLines 3: height %v, want %v", c.n, got, c.want)
		}
	}
	in = base
	in.MinLines, in.MaxLines = 3, 5
	for _, c := range []struct {
		n    int
		want float32
	}{{0, one + 2*step}, {1, one + 2*step}, {4, one + 3*step}, {7, one + 4*step}} {
		if c.n > 0 {
			in.Value = lines(c.n)
		}
		if got := height(t, in); got != c.want {
			t.Errorf("%d lines with MinLines 3, MaxLines 5: height %v, want %v", c.n, got, c.want)
		}
	}
	// A single-line field ignores both.
	if got := height(t, fieldw.Input{Label: "Note", HideLabel: true, MinLines: 3}); got != one {
		t.Errorf("single line with MinLines: height %v, want %v", got, one)
	}
}

func TestInput_MaxLinesScrollsToTheCaret(t *testing.T) {
	tt := mountInput(t, fieldw.Input{Label: "Note", HideLabel: true, Multiline: true, MaxLines: 2})
	tt.Tap("Note")
	tall := field(t, tt, "Note").Rect.H
	for range 6 {
		tt.Type("word")
		tt.Key(input.KeyEnter, 0)
	}
	tt.Type("last")
	n := field(t, tt, "Note")
	if !strings.HasSuffix(n.Value, "last") {
		t.Fatalf("value %q", n.Value)
	}
	if n.Rect.H <= tall || n.Text == nil || len(n.Text.LineRects) != 7 {
		t.Fatalf("height %v after %v, lines %+v", n.Rect.H, tall, n.Text)
	}
	// The last line, where the caret is, shows inside the field.
	last := n.Text.LineRects[6]
	if last.H <= 0 || last.Y+last.H > n.Rect.Y+n.Rect.H+0.5 || last.Y < n.Rect.Y {
		t.Errorf("the line of the caret %v is outside the field %v", last, n.Rect)
	}
	if first := n.Text.LineRects[0]; first.H > 0 {
		t.Errorf("the first line %v should have scrolled out of the field %v", first, n.Rect)
	}
}
