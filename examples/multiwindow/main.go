// Command multiwindow opens windows that share one GPU device and one piece of
// state. Each window has a tree of its own, and all of them run on the UI
// goroutine, so that the state needs no lock.
//
//	multiwindow                  opens the window
//	multiwindow -windows 3       starts with three
//	multiwindow -quit-after 5s   quits on its own
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/motion"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/interact"
)

// model is what the windows share.
type model struct {
	count  int
	opened int
	// watchers rebuild the open windows when count changes.
	watchers map[int]func()
}

func (m *model) add(n int) {
	m.count += n
	for _, rebuild := range m.watchers {
		rebuild()
	}
}

// open opens one more window.
func (m *model) open() {
	m.opened++
	id := m.opened
	title := fmt.Sprintf("gowaku: ウィンドウ %d", id)
	handle := new(*gowaku.WindowHandle)
	opts := gowaku.WindowOptions{Title: title, Width: 360, Height: 280}
	h, err := gowaku.OpenWindow(opts, panel{model: m, id: id, handle: handle})
	if err != nil {
		log.Fatal(err)
	}
	*handle = h
}

type panel struct {
	gowaku.StatefulBase
	model *model
	id    int
	// handle is nil in the first window, which Run opened.
	handle **gowaku.WindowHandle
}

type panelState struct {
	gowaku.StateOf[panel]
	wide bool
}

func (panel) CreateState() gowaku.State { return &panelState{} }

// InitState implements gowaku.State.
func (s *panelState) InitState() {
	p := s.Widget()
	p.model.watchers[p.id] = func() { s.SetState(func() {}) }
}

// Dispose implements gowaku.State.
func (s *panelState) Dispose() {
	p := s.Widget()
	delete(p.model.watchers, p.id)
}

func (s *panelState) Build(ctx gowaku.BuildContext) gowaku.Widget {
	p := s.Widget()
	th := basic.ThemeOf(ctx)
	width := float32(120)
	if s.wide {
		width = 280
	}
	buttons := []gowaku.Widget{
		control.Button{Label: "増やす", Style: control.ButtonPrimary, OnPress: func() { p.model.add(1) }},
		control.Button{Label: "新しいウィンドウ", OnPress: p.model.open},
	}
	if p.handle != nil {
		buttons = append(buttons, control.Button{Label: "閉じる", Style: control.ButtonQuiet, OnPress: func() { (*p.handle).Close() }})
	}
	return basic.Pad(theme.Space24, basic.Column{Spacing: theme.Space16, Cross: layout.CrossStart, Children: []gowaku.Widget{
		basic.Text{Content: fmt.Sprintf("ウィンドウ %d", p.id), Style: th.Text.Title},
		basic.Text{Content: fmt.Sprintf("全部のウィンドウで共有: %d", p.model.count), Style: th.Text.Body},
		// Each window animates on its own: one being busy leaves the others alone.
		interact.GestureDetector{
			OnTap: func() { s.SetState(func() { s.wide = !s.wide }) },
			Child: basic.AnimatedContainer{
				Duration: 400 * time.Millisecond,
				Curve:    motion.EaseInOutCubic,
				Width:    width,
				Height:   32,
				Radius:   th.Radius.Control,
				Color:    th.Color.AccentTint,
			},
		},
		basic.Row{Spacing: theme.Space8, Children: buttons},
	}})
}

// starter opens the other windows from the UI goroutine, once Run is going.
type starter struct {
	gowaku.StatefulBase
	model *model
	extra int
	child gowaku.Widget
}

type starterState struct{ gowaku.StateOf[starter] }

func (starter) CreateState() gowaku.State { return &starterState{} }

// InitState implements gowaku.State.
func (s *starterState) InitState() {
	w := s.Widget()
	gowaku.Post(func() {
		for range w.extra {
			w.model.open()
		}
	})
}

func (s *starterState) Build(gowaku.BuildContext) gowaku.Widget { return s.Widget().child }

func main() {
	windows := flag.Int("windows", 1, "how many windows to start with")
	quitAfter := flag.Duration("quit-after", 0, "quit after this long; 0 waits for the windows to close")
	flag.Parse()
	if *quitAfter > 0 {
		time.AfterFunc(*quitAfter, gowaku.Quit)
	}
	m := &model{opened: 1, watchers: make(map[int]func())}
	err := gowaku.Run(gowaku.App{
		Title:  "gowaku: ウィンドウ 1",
		Width:  360,
		Height: 280,
		Home:   starter{model: m, extra: *windows - 1, child: panel{model: m, id: 1}},
	})
	if err != nil {
		log.Fatal(err)
	}
}
