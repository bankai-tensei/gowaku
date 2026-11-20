package gowakutest_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

// openApp asks for files from a button and says what came back.
type openApp struct{ core.StatefulBase }

func (openApp) CreateState() core.State { return &openAppState{status: "idle"} }

type openAppState struct {
	core.StateBase
	status string
}

func (s *openAppState) open() {
	d := gowaku.FileDialog{Title: "Open", Multiple: true, Filters: []gowaku.FileFilter{{Name: "Text", Patterns: []string{"*.txt"}}}}
	gowaku.ShowFileDialog(s.Context(), d, func(paths []string, err error) {
		s.SetState(func() {
			switch {
			case err != nil:
				s.status = "error: " + err.Error()
			case paths == nil:
				s.status = "canceled"
			default:
				s.status = "opened " + strings.Join(paths, ",")
			}
		})
	})
}

func (s *openAppState) Build(core.BuildContext) core.Widget {
	return basic.Column{Children: []core.Widget{
		control.Button{Label: "Open", OnPress: s.open},
		basic.Text{Content: s.status},
	}}
}

func TestFileDialog_TheTestAnswers(t *testing.T) {
	tt := gowakutest.New(t, openApp{}, gowakutest.Options{})
	tt.Tap("Open")
	d := tt.FileDialog()
	if o := d.Options(); o.Title != "Open" || !o.Multiple || len(o.Filters) != 1 {
		t.Errorf("options %+v", o)
	}
	d.Choose("a.txt", "b.txt")
	if _, ok := tt.Find("opened a.txt,b.txt"); !ok {
		t.Error("the paths must reach done")
	}
	if len(tt.FileDialogs()) != 0 {
		t.Error("an answered dialog must leave")
	}

	tt.Tap("Open")
	tt.FileDialog().Cancel()
	if _, ok := tt.Find("canceled"); !ok {
		t.Error("a cancel must give nil paths")
	}

	tt.Tap("Open")
	tt.FileDialog().Fail(errors.New("no portal"))
	if _, ok := tt.Find("error: no portal"); !ok {
		t.Error("the error must reach done")
	}
}

// themeApp switches to a red dark theme and shows the canvas color it reads.
type themeApp struct{ core.StatelessBase }

var red = theme.FromAccent(geom.Hex(0xc0392b))

func (themeApp) Build(ctx core.BuildContext) core.Widget {
	_, mode := gowaku.CurrentTheme(ctx)
	label := "light"
	if basic.ThemeOf(ctx).Brightness == theme.BrightnessDark {
		label = "dark"
	}
	return basic.Column{Children: []core.Widget{
		control.Button{Label: "Dark", OnPress: func() { gowaku.SetTheme(ctx, red, gowaku.ThemeDark) }},
		control.Button{Label: "System", OnPress: func() { gowaku.SetTheme(ctx, red, gowaku.ThemeSystem) }},
		basic.Text{Content: label},
		basic.Text{Content: map[gowaku.ThemeMode]string{gowaku.ThemeSystem: "system", gowaku.ThemeDark: "fixed dark", gowaku.ThemeLight: "fixed light"}[mode]},
	}}
}

func TestSetTheme_TheTreeTakesIt(t *testing.T) {
	tt := gowakutest.New(t, themeApp{}, gowakutest.Options{})
	if _, ok := tt.Find("light"); !ok {
		t.Fatal("the tree must start light")
	}
	tt.Tap("Dark")
	if _, ok := tt.Find("dark"); !ok {
		t.Error("ThemeDark must make the tree dark")
	}
	if _, ok := tt.Find("fixed dark"); !ok {
		t.Error("CurrentTheme must report the mode")
	}
	// ThemeSystem follows Options.Dark, false here.
	tt.Tap("System")
	if _, ok := tt.Find("light"); !ok {
		t.Error("ThemeSystem must follow the OS of the Tester")
	}
}
