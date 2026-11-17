package field

import (
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
)

// suggestions returns the labels of the open list, in order.
func (r *fieldRig) suggestions() []string {
	var out []string
	for _, n := range r.all(semantics.RoleListItem) {
		out = append(out, n.Label)
	}
	return out
}

func (r *fieldRig) wantSuggestions(want ...string) {
	r.T.Helper()
	if got := r.suggestions(); strings.Join(got, ",") != strings.Join(want, ",") {
		r.T.Errorf("suggestions %q, want %q", got, want)
	}
}

// highlighted returns the label of the highlighted suggestion, or "".
func (r *fieldRig) highlighted() string {
	for _, n := range r.all(semantics.RoleListItem) {
		if n.State&semantics.Selected != 0 {
			return n.Label
		}
	}
	return ""
}

type cityPick struct {
	changes []string
	picked  []string
	submit  []string
}

func cityRig(t *testing.T, p *cityPick, extra func(*Autocomplete[string])) *fieldRig {
	t.Helper()
	a := Autocomplete[string]{
		Label: "City", Options: wtest.CityNames(),
		OnChange: func(s string) { p.changes = append(p.changes, s) },
		OnSelect: func(s string) { p.picked = append(p.picked, s) },
		OnSubmit: func(s string) { p.submit = append(p.submit, s) },
	}
	if extra != nil {
		extra(&a)
	}
	r := newFieldRig(t, basic.SizedBox{Width: 240, Child: a})
	r.focusField("City")
	return r
}

func TestAutocomplete_TypingSuggestsStartsFirst(t *testing.T) {
	var p cityPick
	r := cityRig(t, &p, nil)
	r.wantSuggestions()
	r.typeText("o")
	r.wantSuggestions("Otaru", "Kyoto", "Tokyo", "Toyama", "Kobe")
	r.typeText("T")
	r.wantSuggestions("Otaru", "Kyoto")
	if n := r.must(semantics.RoleComboBox, "City"); n.State&semantics.Expanded == 0 || n.Value != "oT" {
		t.Errorf("got %+v", n)
	}
	r.typeText("x")
	r.wantSuggestions()
	if n := r.must(semantics.RoleComboBox, "City"); n.State&semantics.Collapsed == 0 {
		t.Error("nothing to suggest must close the list")
	}
	r.wantFocus(semantics.RoleTextField, "City")
}

func TestAutocomplete_ArrowsAndEnterTakeASuggestion(t *testing.T) {
	var p cityPick
	r := cityRig(t, &p, nil)
	r.typeText("to")
	r.wantSuggestions("Tokyo", "Toyama", "Kyoto")
	if r.highlighted() != "" {
		t.Error("nothing is highlighted until an arrow moves")
	}
	r.Key(input.KeyDown, 0)
	r.Key(input.KeyDown, 0)
	r.Key(input.KeyUp, 0)
	if got := r.highlighted(); got != "Tokyo" {
		t.Fatalf("highlighted %q", got)
	}
	r.Key(input.KeyEnter, 0)
	if got := r.editing().Text(); got != "Tokyo" {
		t.Errorf("got %q", got)
	}
	if a, b := r.editing().Selection(); a != 5 || b != 5 {
		t.Errorf("the caret must go to the end, got %d-%d", a, b)
	}
	r.wantSuggestions()
	if len(p.picked) != 1 || p.picked[0] != "Tokyo" || len(p.submit) != 0 {
		t.Errorf("got %+v", p)
	}
	if p.changes[len(p.changes)-1] != "Tokyo" {
		t.Errorf("the taken text must be reported: %q", p.changes)
	}
	r.Key(input.KeyZ, input.ModControl)
	if got := r.editing().Text(); got != "to" {
		t.Errorf("Ctrl+Z must take the suggestion back, got %q", got)
	}
	r.Key(input.KeyEnter, 0)
	if len(p.submit) != 1 || p.submit[0] != "to" {
		t.Errorf("Enter without a highlight must submit: %q", p.submit)
	}
}

func TestAutocomplete_TabTakesAndEscapeCloses(t *testing.T) {
	var p cityPick
	r := newFieldRig(t, basic.Column{Children: []core.Widget{
		basic.SizedBox{Width: 240, Child: Autocomplete[string]{
			Label: "City", Options: wtest.CityNames(), OnSelect: func(s string) { p.picked = append(p.picked, s) },
		}},
		Input{Label: "Next"},
	}})
	r.focusField("City")
	r.typeText("k")
	r.Key(input.KeyEscape, 0)
	r.wantSuggestions()
	r.wantFocus(semantics.RoleTextField, "City")
	// Down opens the list again, on the first suggestion.
	r.Key(input.KeyDown, 0)
	if got := r.highlighted(); got != "Kyoto" {
		t.Fatalf("highlighted %q", got)
	}
	r.Key(input.KeyTab, 0)
	if len(p.picked) != 1 || p.picked[0] != "Kyoto" {
		t.Errorf("got %q", p.picked)
	}
	r.wantFocus(semantics.RoleTextField, "Next")
	r.wantSuggestions()
}

func TestAutocomplete_ClickTakesAndKeepsTheKeyboard(t *testing.T) {
	var p cityPick
	r := cityRig(t, &p, nil)
	r.typeText("ya")
	r.click(r.must(semantics.RoleListItem, "Toyama"))
	if got := r.editing().Text(); got != "Toyama" {
		t.Errorf("got %q", got)
	}
	r.wantFocus(semantics.RoleTextField, "City")
	r.wantSuggestions()
}

func TestAutocomplete_IMECompositionIsLeftAlone(t *testing.T) {
	var p cityPick
	r := cityRig(t, &p, func(a *Autocomplete[string]) {
		a.Options = []string{"東京", "東北", "京都"}
	})
	r.compose(&input.Composition{Text: "とう", Cursor: 6})
	r.wantSuggestions()
	r.Key(input.KeyDown, 0)
	r.wantSuggestions()
	r.typeText("東")
	r.wantSuggestions("東京", "東北")
	// While the next word is converted, the list stays and the keys are the
	// IME's.
	r.compose(&input.Composition{Text: "きょう", Cursor: 9})
	r.wantSuggestions("東京", "東北")
	r.Key(input.KeyDown, 0)
	r.Key(input.KeyEnter, 0)
	if r.highlighted() != "" || len(p.picked) != 0 || len(p.submit) != 0 {
		t.Errorf("the keys must reach the IME: %q %+v", r.highlighted(), p)
	}
	r.compose(nil)
	r.typeText("京")
	r.wantSuggestions("東京")
	if strings.Join(p.changes, ",") != "東,東京" {
		t.Errorf("only committed text counts: %q", p.changes)
	}
}

func TestAutocomplete_SuggestionsFuncAndNoResults(t *testing.T) {
	type city struct {
		name string
		pop  int
	}
	var asked []string
	var picked []city
	r := newFieldRig(t, basic.SizedBox{Width: 240, Child: Autocomplete[city]{
		Label: "City",
		Suggestions: func(q string) []city {
			asked = append(asked, q)
			if q == "o" {
				return []city{{"Osaka", 2700}, {"Okayama", 700}}
			}
			return nil
		},
		Format:        func(c city) string { return c.name },
		OnSelect:      func(c city) { picked = append(picked, c) },
		ShowNoResults: true,
	}})
	r.focusField("City")
	r.typeText("o")
	r.wantSuggestions("Osaka", "Okayama")
	r.Key(input.KeyDown, 0)
	r.Key(input.KeyDown, 0)
	r.Key(input.KeyEnter, 0)
	if len(picked) != 1 || picked[0].pop != 700 {
		t.Errorf("got %+v", picked)
	}
	r.typeText("z")
	r.must(semantics.RoleText, "No results")
	if n := r.must(semantics.RoleComboBox, "City"); n.State&semantics.Expanded == 0 {
		t.Error("ShowNoResults must keep the list open")
	}
	if strings.Join(asked, ",") != "o,Okayamaz" {
		t.Errorf("got %q", asked)
	}
}

func TestAutocomplete_ScreenReaderExpands(t *testing.T) {
	var p cityPick
	r := cityRig(t, &p, nil)
	n := r.must(semantics.RoleComboBox, "City")
	if n.State&semantics.Collapsed == 0 || n.Actions&semantics.ActionExpand == 0 {
		t.Fatalf("got %+v", n)
	}
	r.act(n, semantics.ActionExpand)
	r.wantSuggestions(wtest.CityNames()...)
	r.act(r.must(semantics.RoleComboBox, "City"), semantics.ActionCollapse)
	r.wantSuggestions()
}

func TestAutocomplete_BlurCloses(t *testing.T) {
	r := newFieldRig(t, basic.Column{Children: []core.Widget{
		basic.SizedBox{Width: 240, Child: Autocomplete[string]{Label: "City", Options: wtest.CityNames()}},
		Input{Label: "Next"},
	}})
	r.focusField("City")
	r.typeText("o")
	r.focusField("Next")
	r.wantSuggestions()
}
