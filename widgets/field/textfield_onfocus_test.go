package field_test

import (
	"slices"
	"testing"

	fieldw "github.com/bankai-tensei/gowaku/widgets/field"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

func TestInput_OnFocusReportsGainingAndLosingTheKeyboard(t *testing.T) {
	var got []bool
	tt := gowakutest.New(t, basic.Column{Children: []core.Widget{
		fieldw.Input{Label: "Name", OnFocus: func(focused bool) { got = append(got, focused) }},
		fieldw.Input{Label: "Mail"},
	}}, pressableFonts)
	if len(got) != 0 {
		t.Fatalf("OnFocus ran %v before the field had the keyboard", got)
	}
	tt.Tap("Name")
	if want := []bool{true}; !slices.Equal(got, want) {
		t.Fatalf("a tap gave %v, want %v", got, want)
	}
	tt.Key(input.KeyTab, 0)
	if want := []bool{true, false}; !slices.Equal(got, want) {
		t.Fatalf("Tab to the next field gave %v, want %v", got, want)
	}
	// The other field reports nothing, and the first hears it get the
	// keyboard back.
	tt.Tap("Name")
	if want := []bool{true, false, true}; !slices.Equal(got, want) {
		t.Errorf("a tap back gave %v, want %v", got, want)
	}
}
