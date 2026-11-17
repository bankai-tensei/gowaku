// Command counter is the smallest sample with state: a State, a theme, and an
// implicit animation. Between clicks nothing runs: no build, no layout, no frame.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

type counter struct{ gowaku.StatefulBase }

type counterState struct {
	gowaku.StateBase
	n int
}

func (counter) CreateState() gowaku.State { return &counterState{} }

func (s *counterState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	return basic.Center{Child: basic.VStack(
		basic.Text{Content: fmt.Sprintf("count: %d", s.n), Style: th.Text.LargeTitle},
		// An implicit animation: when the value changes, it gets there on its own.
		basic.AnimatedContainer{
			Duration: 300 * time.Millisecond,
			Curve:    motion.EaseOutCubic,
			Color:    th.Color.AccentTint.WithAlpha(float32(s.n%10) / 10),
			Padding:  geom.All(16),
			Radius:   th.Radius.Group,
			Child: control.Button{
				Label:   "増やす",
				Style:   control.ButtonPrimary,
				OnPress: func() { s.SetState(func() { s.n++ }) },
			},
		},
	)}
}

func main() {
	err := gowaku.Run(gowaku.App{
		Title:  "gowaku: counter",
		Width:  400,
		Height: 300,
		Home:   counter{},
	})
	if err != nil {
		log.Fatal(err)
	}
}
