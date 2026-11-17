package field_test

import (
	"testing"

	fieldw "github.com/bankai-tensei/gowaku/widgets/field"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// mountField puts w at the top left of a window, 240 wide.
func mountField(t *testing.T, w core.Widget) *gowakutest.Tester {
	t.Helper()
	return gowakutest.New(t, basic.Align{Alignment: layout.AlignTopLeft, Child: basic.SizedBox{Width: 240, Child: w}}, pressableFonts)
}

func TestSearch_DisabledAndReadOnly(t *testing.T) {
	for _, tc := range []struct {
		name  string
		w     fieldw.Search
		state semantics.State
	}{
		{"disabled", fieldw.Search{Label: "Find", Value: "ada", Disabled: true}, semantics.Disabled},
		{"read-only", fieldw.Search{Label: "Find", Value: "ada", ReadOnly: true}, semantics.ReadOnly},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := 0
			tc.w.OnChange = func(string) { changed++ }
			tt := mountField(t, tc.w)
			if n := field(t, tt, "Find"); n.State&tc.state == 0 {
				t.Errorf("state %v: want %v", n.State, tc.state)
			}
			if _, ok := tt.Find("Clear"); ok {
				t.Errorf("a field that cannot change shows no clear button:\n%s", tt.Dump())
			}
			tt.Tap("Find")
			tt.Type("x")
			tt.Key(input.KeyEscape, 0)
			if n := field(t, tt, "Find"); n.Value != "ada" || changed != 0 {
				t.Errorf("value %q after typing and Escape, OnChange %d times: want it kept", n.Value, changed)
			}
		})
	}
}

func TestNumberField_ReadOnly(t *testing.T) {
	changed := 0
	tt := mountField(t, fieldw.NumberField{Label: "Count", Value: 3, ReadOnly: true, OnChange: func(float64) { changed++ }})
	n, ok := tt.FindRole(semantics.RoleStepper, "Count")
	if !ok {
		t.Fatalf("no stepper:\n%s", tt.Dump())
	}
	if n.State&semantics.ReadOnly == 0 || n.State&semantics.Disabled != 0 {
		t.Errorf("state %v: want ReadOnly and not Disabled", n.State)
	}
	if n.Actions&(semantics.ActionIncrease|semantics.ActionDecrease) != 0 {
		t.Errorf("actions %v: a read-only field offers no steps", n.Actions)
	}
	tt.Tap("Count")
	tt.Key(input.KeyUp, 0)
	tt.Type("9")
	tt.Key(input.KeyEnter, 0)
	if changed != 0 {
		t.Errorf("OnChange ran %d times on a read-only field", changed)
	}
	if n, _ := tt.FindRole(semantics.RoleStepper, "Count"); n.Value != "3" {
		t.Errorf("value %q, want 3", n.Value)
	}
}

func TestAutocomplete_ReadOnlyOpensNoList(t *testing.T) {
	tt := mountField(t, fieldw.Autocomplete[string]{
		Label: "City", Value: "Osaka", ReadOnly: true,
		Options: []string{"Osaka", "Otaru", "Tokyo"},
	})
	if n := field(t, tt, "City"); n.State&semantics.ReadOnly == 0 {
		t.Errorf("state %v: want ReadOnly", n.State)
	}
	tt.Tap("City")
	tt.Key(input.KeyDown, 0)
	if _, ok := tt.Find("Tokyo"); ok {
		t.Errorf("a read-only field opened its suggestions:\n%s", tt.Dump())
	}
}

func TestTokenField_ReadOnly(t *testing.T) {
	changed := 0
	tt := mountField(t, fieldw.TokenField{Label: "To", Tokens: []string{"ada", "lin"}, ReadOnly: true, OnChange: func([]string) { changed++ }})
	n, ok := tt.FindRole(semantics.RoleList, "To")
	if !ok {
		t.Fatalf("no list:\n%s", tt.Dump())
	}
	if n.State&semantics.ReadOnly == 0 || n.State&semantics.Disabled != 0 {
		t.Errorf("state %v: want ReadOnly and not Disabled", n.State)
	}
	if _, ok := tt.FindRole(semantics.RoleTextField, "To"); ok {
		t.Errorf("a read-only token field has no text to type in:\n%s", tt.Dump())
	}
	if _, ok := tt.Find("Remove ada"); ok {
		t.Errorf("a read-only token field has no × on its chips:\n%s", tt.Dump())
	}
	if changed != 0 {
		t.Errorf("OnChange ran %d times", changed)
	}
}
