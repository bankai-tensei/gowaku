package gowaku_test

import (
	"fmt"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

type hello struct{ gowaku.StatelessBase }

// Build implements core.StatelessWidget.
func (hello) Build(gowaku.BuildContext) gowaku.Widget {
	return basic.Center{Child: basic.Text{Content: "こんにちは、gowaku"}}
}

// Run opens the window and blocks until it is closed.
func ExampleRun() {
	run := func() {
		err := gowaku.Run(gowaku.App{Title: "hello", Width: 400, Height: 300, Home: hello{}})
		if err != nil {
			fmt.Println(err)
		}
	}
	_ = run
}

type counter struct{ gowaku.StatefulBase }

type counterState struct {
	gowaku.StateBase
	n int
}

// CreateState implements core.StatefulWidget.
func (counter) CreateState() gowaku.State { return &counterState{} }

// Build implements core.State.
func (s *counterState) Build(gowaku.BuildContext) gowaku.Widget {
	return control.Button{
		Label:   fmt.Sprintf("%d 回押した", s.n),
		OnPress: func() { s.SetState(func() { s.n++ }) },
	}
}

// A StatefulWidget keeps its State across builds. SetState asks for a new one.
func ExampleStatefulBase() {
	var _ gowaku.Widget = counter{}
}
