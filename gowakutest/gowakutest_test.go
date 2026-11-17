package gowakutest_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

// goFonts keeps the tests independent of the fonts of the machine.
var goFonts = gowakutest.Options{Fonts: [][]byte{goregular.TTF}}

type counter struct{ gowaku.StatefulBase }

type counterState struct {
	gowaku.StateBase
	n int
}

func (counter) CreateState() gowaku.State { return &counterState{} }

func (s *counterState) Build(gowaku.BuildContext) gowaku.Widget {
	return basic.Center{Child: basic.VStack(
		basic.Text{Content: fmt.Sprintf("count: %d", s.n)},
		control.Button{Label: "Add one", OnPress: func() { s.SetState(func() { s.n++ }) }},
	)}
}

func TestTester_TapChangesTheTree(t *testing.T) {
	tt := gowakutest.New(t, counter{}, goFonts)
	tt.Tap("Add one")
	tt.Tap("Add one")
	if _, ok := tt.Find("count: 2"); !ok {
		t.Error("the count did not reach 2")
	}
	if _, ok := tt.Find("count: 1"); ok {
		t.Error("the old count is still there")
	}
}

// clock counts seconds with a periodic ticker.
type clock struct{ gowaku.StatefulBase }

type clockState struct {
	gowaku.StateBase
	seconds int
}

func (clock) CreateState() gowaku.State { return &clockState{} }

func (s *clockState) InitState() {
	first := true
	k := s.NewPeriodicTicker(time.Second, func(time.Duration) {
		if first {
			first = false
			return
		}
		s.SetState(func() { s.seconds++ })
	})
	k.Start()
}

func (s *clockState) Build(gowaku.BuildContext) gowaku.Widget {
	return basic.Text{Content: fmt.Sprintf("%d s", s.seconds)}
}

func TestTester_AdvanceDrivesPeriodicTickers(t *testing.T) {
	tt := gowakutest.New(t, clock{}, goFonts)
	tt.Advance(3500 * time.Millisecond)
	if _, ok := tt.Find("3 s"); !ok {
		t.Errorf("want 3 s at %v, got\n%s", tt.Now(), tt.Dump())
	}
	tt.Settle() // a sleeping ticker does not keep Settle going
	if tt.Now() != 3500*time.Millisecond {
		t.Errorf("Settle moved the clock to %v", tt.Now())
	}
}

type form struct{ gowaku.StatefulBase }

type formState struct {
	gowaku.StateBase
	name, sent string
}

func (form) CreateState() gowaku.State { return &formState{} }

func (s *formState) Build(gowaku.BuildContext) gowaku.Widget {
	return basic.Pad(16, basic.Column{Spacing: 8, Children: []gowaku.Widget{
		field.Input{
			Label: "Name", Value: s.name,
			OnChange: func(v string) { s.SetState(func() { s.name = v }) },
			OnSubmit: func(v string) { s.SetState(func() { s.sent = v }) },
		},
		basic.Text{Content: "sent: " + s.sent},
	}})
}

func TestTester_TypeAndKey(t *testing.T) {
	tt := gowakutest.New(t, form{}, goFonts)
	tt.Tap("Name")
	tt.Type("gowaku")
	tt.Key(input.KeyEnter, 0)
	if _, ok := tt.Find("sent: gowaku"); !ok {
		t.Errorf("got\n%s", tt.Dump())
	}
}

type confirm struct{ gowaku.StatefulBase }

type confirmState struct {
	gowaku.StateBase
	answer string
}

func (confirm) CreateState() gowaku.State { return &confirmState{} }

func (s *confirmState) Build(gowaku.BuildContext) gowaku.Widget {
	return basic.Center{Child: basic.VStack(
		basic.Text{Content: "answer: " + s.answer},
		control.Button{Label: "Delete", OnPress: func() {
			overlay.ShowDialog(s.Context(), func(dismiss func()) gowaku.Widget {
				return overlay.Dialog{Title: "Sure?", Actions: []gowaku.Widget{
					control.Button{Label: "Yes", OnPress: func() {
						s.SetState(func() { s.answer = "yes" })
						dismiss()
					}},
				}}
			})
		}},
	)}
}

func TestTester_DialogsWork(t *testing.T) {
	tt := gowakutest.New(t, confirm{}, goFonts)
	tt.Tap("Delete")
	if _, ok := tt.Find("Sure?"); !ok {
		t.Fatal("no dialog")
	}
	tt.Tap("Yes")
	if _, ok := tt.Find("answer: yes"); !ok {
		t.Error("the answer did not arrive")
	}
	if _, ok := tt.Find("Sure?"); ok {
		t.Error("the dialog is still open")
	}
}

func TestTester_Screenshot(t *testing.T) {
	opts := goFonts
	opts.Width, opts.Height, opts.Scale = 200, 100, 2
	tt := gowakutest.New(t, basic.Container{Color: geom.Hex(0xff0000)}, opts)
	img := tt.Screenshot()
	if got := img.Bounds().Size(); got.X != 400 || got.Y != 200 {
		t.Fatalf("size %v", got)
	}
	if c := img.NRGBAAt(200, 100); c.R != 255 || c.G != 0 || c.B != 0 {
		t.Errorf("middle pixel %v", c)
	}
	path := filepath.Join(t.TempDir(), "shot", "red.png")
	tt.SaveScreenshot(path)
}

func TestTester_DarkScreenshotUsesTheDarkCanvas(t *testing.T) {
	opts := goFonts
	opts.Width, opts.Height, opts.Dark = 40, 40, true
	tt := gowakutest.New(t, basic.SizedBox{}, opts)
	if c := tt.Screenshot().NRGBAAt(20, 20); c.R > 60 {
		t.Errorf("canvas %v is not dark", c)
	}
}
