package control_test

import (
	"fmt"
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

// percent spells a fraction as a percentage.
func percent(v float32) string { return fmt.Sprintf("%.0f%%", v*100) }

func TestSlider_FormatSpellsTheValue(t *testing.T) {
	var value float32 = 0.5
	var swap func(core.Widget)
	build := func() core.Widget {
		return basic.Column{Children: []core.Widget{
			control.Slider{Label: "Volume", Value: value, Format: percent, OnChange: func(v float32) {
				value = v
				swap(nil)
			}},
			control.Slider{Label: "Balance", Value: 0.25},
		}}
	}
	tt := gowakutest.New(t, rebuilt{build: build, swap: &swap}, gowakutest.Options{})
	volume, ok := tt.FindRole(semantics.RoleSlider, "Volume")
	if !ok || volume.Value != "50%" {
		t.Fatalf("Volume reads %q, want 50%%\n%s", volume.Value, tt.Dump())
	}
	if n, _ := tt.FindRole(semantics.RoleSlider, "Balance"); n.Value != "0.25" {
		t.Errorf("Balance without Format reads %q, want 0.25", n.Value)
	}
	tt.Tap("Volume")
	tt.Key(input.KeyRight, 0)
	if n, _ := tt.FindRole(semantics.RoleSlider, "Volume"); n.Value != "55%" {
		t.Errorf("after Right, Volume reads %q, want 55%%", n.Value)
	}
}

func TestRangeSlider_FormatSpellsEachThumb(t *testing.T) {
	price := func(v float32) string { return fmt.Sprintf("$%.0f", v) }
	tt := gowakutest.New(t, control.RangeSlider{
		Label: "Price", Low: 20, High: 80, Max: 100, Format: price, OnChange: func(float32, float32) {},
	}, gowakutest.Options{})
	low, _ := tt.FindRole(semantics.RoleSlider, "Minimum")
	high, _ := tt.FindRole(semantics.RoleSlider, "Maximum")
	if low == nil || high == nil || low.Value != "$20" || high.Value != "$80" {
		t.Errorf("the thumbs do not read $20 and $80\n%s", tt.Dump())
	}
}

// rebuilt builds its child again whenever swap is called, the way a parent
// that keeps the value does.
type rebuilt struct {
	core.StatefulBase
	build func() core.Widget
	swap  *func(core.Widget)
}

type rebuiltState struct{ core.StateBase }

func (rebuilt) CreateState() core.State { return &rebuiltState{} }

func (s *rebuiltState) InitState() {
	*s.Widget().(rebuilt).swap = func(core.Widget) { s.SetState(func() {}) }
}

func (s *rebuiltState) Build(core.BuildContext) core.Widget { return s.Widget().(rebuilt).build() }
