package field

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

func TestSearchField_SaysSearchWhileEmpty(t *testing.T) {
	cases := []struct {
		name   string
		locale language.Tag
		w      Search
		want   string
	}{
		{"英語", language.English, Search{}, "Search"},
		{"日本語", language.Japanese, Search{}, "検索"},
		{"Placeholder が勝つ", language.Japanese, Search{Placeholder: "メモを検索"}, "メモを検索"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newFieldRig(t, basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1, Locale: tc.locale}, Child: tc.w})
			r.must(semantics.RoleTextField, tc.want)
			if n := r.Find(semantics.RoleButton, ""); n != nil {
				t.Errorf("an empty field must not offer to clear: %+v", n)
			}
		})
	}
}

func TestSearchField_ClearButtonClearsAndKeepsTheKeyboard(t *testing.T) {
	var changes []string
	r := newFieldRig(t, Search{OnChange: func(s string) { changes = append(changes, s) }})
	r.focusField("Search")
	r.typeText("go")
	clear := r.must(semantics.RoleButton, "Clear")
	if clear.State&semantics.Focusable != 0 {
		t.Error("Tab must pass the clear button, which Escape stands for")
	}
	r.click(clear)
	if got := r.editing().Text(); got != "" {
		t.Errorf("got %q", got)
	}
	r.wantFocus(semantics.RoleTextField, "Search")
	if len(changes) != 2 || changes[1] != "" {
		t.Errorf("the clearing must be reported: %q", changes)
	}
	if r.Find(semantics.RoleButton, "Clear") != nil {
		t.Error("the button must go with the text")
	}
	r.Key(input.KeyZ, input.ModControl)
	if got := r.editing().Text(); got != "go" {
		t.Errorf("Ctrl+Z must bring the text back, got %q", got)
	}
}

func TestSearchField_EscapeClearsThenPassesOn(t *testing.T) {
	escapes := 0
	r := newFieldRig(t, interact.Focus{
		OnKey: func(e *core.KeyEvent) bool {
			if e.Key == input.KeyEscape {
				escapes++
			}
			return true
		},
		Child: Search{Value: "query"},
	})
	r.focusField("Search")
	r.Key(input.KeyEscape, 0)
	if got := r.editing().Text(); got != "" || escapes != 0 {
		t.Fatalf("the first Escape must clear: %q, %d", got, escapes)
	}
	r.Key(input.KeyEscape, 0)
	if escapes != 1 {
		t.Error("an Escape in an empty field must reach the dialog or panel around it")
	}
}

func TestSearchField_EscapeLeavesTheIMEAlone(t *testing.T) {
	r := newFieldRig(t, Search{Value: "東京"})
	r.focusField("Search")
	r.compose(&input.Composition{Text: "と", Cursor: 3})
	r.Key(input.KeyEscape, 0)
	if got := r.editing().Text(); got != "東京" {
		t.Errorf("the Escape belongs to the IME, got %q", got)
	}
}

func TestSearchField_SubmitAndOutsideValue(t *testing.T) {
	var submitted string
	r := newFieldRig(t, formHolder[string]{Initial: "a", Build: func(v string, set func(string)) core.Widget {
		return basic.Column{Children: []core.Widget{
			Search{Value: v, OnSubmit: func(s string) { submitted = s }},
			control.Button{Label: "reset", OnPress: func() { set("") }},
		}}
	}})
	r.focusField("Search")
	r.Key(input.KeyEnd, 0)
	r.typeText("b")
	r.Key(input.KeyEnter, 0)
	if submitted != "ab" {
		t.Errorf("got %q", submitted)
	}
	r.must(semantics.RoleButton, "Clear")
	r.click(r.must(semantics.RoleButton, "reset"))
	if r.Find(semantics.RoleButton, "Clear") != nil {
		t.Error("a Value of \"\" from outside must hide the clear button")
	}
}

func TestSearchField_LooksLikeAField(t *testing.T) {
	th := theme.Default().Light
	tree := wtest.TextTree(t, basic.Align{Child: basic.SizedBox{Width: 200, Child: Search{Value: "x"}}})
	list := wtest.Ops(tree, wtest.Screen)
	var box, glyphs int
	for _, op := range list {
		switch {
		case op.Color == th.Color.Field && op.RRect.Rect.H == theme.ControlHeight:
			box++
			if op.RRect.TopLeft != th.Radius.Control {
				t.Errorf("the field must be rounded like one: %+v", op.RRect)
			}
		case op.Color == th.Color.Ink3:
			glyphs++
		}
	}
	if box != 1 || glyphs < 2 {
		t.Errorf("want one field box and the two glyphs, got %d and %d", box, glyphs)
	}
}
