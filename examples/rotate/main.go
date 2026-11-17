// Command rotate turns widgets with Transform: a card that spins with an
// AnimationController, and a turned button that is hit only inside its turned
// shape. See docs/ADR/0044-drawing-extensions.md.
//
// With -png it writes one frame, the card at -angle, and exits.
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/shot"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

type spinner struct {
	gowaku.StatefulBase
	// still, if set, fixes the card at angle instead of spinning it.
	still bool
	angle float32
}

type spinnerState struct {
	gowaku.StateBase
	controller motion.AnimationController
	presses    int
}

func (spinner) CreateState() gowaku.State { return &spinnerState{} }

func (s *spinnerState) InitState() {
	if s.Widget().(spinner).still {
		return
	}
	s.controller.Duration = 6 * time.Second
	s.controller.OnChange = func() { s.SetState(func() {}) }
	s.controller.Attach(&s.StateBase)
	s.controller.Repeat(false)
}

func (s *spinnerState) Dispose() {
	if !s.Widget().(spinner).still {
		s.controller.Dispose()
	}
}

func (s *spinnerState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	th := basic.ThemeOf(ctx)
	w := s.Widget().(spinner)
	angle := w.angle
	if !w.still {
		angle = 2 * math.Pi * s.controller.Value()
	}
	card := basic.Container{
		Width: 180, Height: 110, Radius: th.Radius.Group,
		Gradient: &paint.Gradient{
			End:        geom.Offset{X: 180, Y: 110},
			StartColor: geom.Hex(0x7b2ff7), EndColor: geom.Hex(0xff8a00),
		},
		Shadows: []paint.BoxShadow{{Color: geom.Hex(0x000000).WithAlpha(0.25), Blur: 18, Offset: geom.Offset{Y: 6}}},
		Padding: geom.All(16),
		Child:   basic.Text{Content: "回るカード", Style: th.Text.Title.WithColor(geom.Hex(0xffffff))},
	}
	return basic.Center{Child: basic.HStack(
		basic.Pad(40, basic.Rotate(angle, card)),
		basic.VStack(
			basic.Rotate(-0.25, control.Button{
				Label: "押せる範囲も回る", Style: control.ButtonPrimary,
				OnPress: func() { s.SetState(func() { s.presses++ }) },
			}),
			basic.Pad(16, basic.Text{Content: fmt.Sprintf("押した回数: %d", s.presses)}),
		),
	)}
}

func main() {
	pngPath := flag.String("png", "", "write one frame to this file and exit")
	angle := flag.Float64("angle", 0.5, "the angle of the card in radians, with -png")
	flag.Parse()
	var err error
	if *pngPath != "" {
		err = screenshot(*pngPath, float32(*angle))
	} else {
		err = gowaku.Run(gowaku.App{Title: "gowaku: rotate", Width: 640, Height: 400, Home: spinner{}})
	}
	if err != nil {
		log.Fatal(err)
	}
}

func screenshot(out string, angle float32) error {
	h, err := shot.Open(spinner{still: true, angle: angle}, shot.Options{Width: 640, Height: 400, Theme: shot.DefaultTheme()})
	if err != nil {
		return err
	}
	defer h.Close()
	if err := h.Settle(); err != nil {
		return err
	}
	return h.Screenshot(out)
}
