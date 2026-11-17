package gowaku_test

import (
	"fmt"
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

// stepper counts in steps of Step, which its state reads through StateOf.
type stepper struct {
	gowaku.StatefulBase
	Label string
	Step  int
}

type stepperState struct {
	gowaku.StateOf[stepper]
	n int
}

// CreateState implements core.StatefulWidget.
func (stepper) CreateState() gowaku.State { return &stepperState{} }

// Build implements core.State.
func (s *stepperState) Build(gowaku.BuildContext) gowaku.Widget {
	w := s.Widget() // a stepper: no type assertion
	return basic.Center{Child: basic.VStack(
		basic.Text{Content: fmt.Sprintf("%s: %d", w.Label, s.n)},
		control.Button{Label: "Step", OnPress: func() { s.SetState(func() { s.n += s.Widget().Step }) }},
	)}
}

func TestStateOf_ThroughRootPackage(t *testing.T) {
	tt := gowakutest.New(t, stepper{Label: "total", Step: 5}, gowakutest.Options{Fonts: [][]byte{goregular.TTF}})
	tt.Tap("Step")
	tt.Tap("Step")
	if _, ok := tt.Find("total: 10"); !ok {
		t.Errorf("the total did not reach 10:\n%s", tt.Dump())
	}
}

// A State that embeds StateOf gets its widget with its own type.
func ExampleStateOf() {
	run := func() {
		err := gowaku.Run(gowaku.App{Title: "stepper", Width: 400, Height: 300, Home: stepper{Label: "total", Step: 5}})
		if err != nil {
			fmt.Println(err)
		}
	}
	_ = run
}
