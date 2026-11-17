package gowaku

import (
	"testing"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/platform"
	"github.com/bankai-tensei/gowaku/semantics"
)

func TestTitleBar_ButtonsAreNamedButtons(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	win := func() *fakeWindow { return p.windows[1] }
	p, b = newFakes(t,
		func() {
			win().csd = true
			win().a11y.listening = true
			p.send(platform.Event{Kind: platform.EventResize, Window: 1, PhysicalWidth: 400, PhysicalHeight: 200, Scale: 2})
		},
		func() {
			root := win().a11y.root
			for _, label := range []string{"Minimize", "Maximize", "Close"} {
				if n := findLabel(root, label); n == nil || n.Role != semantics.RoleButton {
					t.Errorf("no button %s: %+v", label, n)
				}
			}
			if findLabel(root, "Document") == nil {
				t.Error("the content of the bar is missing")
			}
			n := findLabel(root, "Minimize")
			if n == nil {
				t.Fatal("no Minimize")
			}
			p.send(platform.Event{Kind: platform.EventAccessibilityAction, Window: 1, Node: n.ID, Action: semantics.ActionTap})
		},
		func() {
			if win().minimizes != 1 {
				t.Errorf("Tap minimized %d times, want once", win().minimizes)
			}
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
	)
	p.a11y = &fakeA11y{}
	home := basic.Column{Children: []core.Widget{
		TitleBar{Child: basic.Text{Content: "Document"}},
		basic.Expanded{Child: basic.SizedBox{}},
	}}
	if err := run(p, b, testutil.NewShaper(t), App{Title: "t", Home: home}); err != nil {
		t.Fatal(err)
	}
}

func TestTitleBar_ButtonsSpeakTheLocaleOfTheApp(t *testing.T) {
	var p *fakePlatform
	var b *fakeBackend
	win := func() *fakeWindow { return p.windows[1] }
	p, b = newFakes(t,
		func() {
			win().csd = true
			win().a11y.listening = true
			p.send(platform.Event{Kind: platform.EventResize, Window: 1, PhysicalWidth: 400, PhysicalHeight: 200, Scale: 2})
		},
		func() {
			root := win().a11y.root
			for _, label := range []string{"最小化", "最大化", "閉じる"} {
				if n := findLabel(root, label); n == nil || n.Role != semantics.RoleButton {
					t.Errorf("no button %s: %+v", label, n)
				}
			}
			if findLabel(root, "Minimize") != nil {
				t.Error("the buttons must not stay English")
			}
			win().maximized = true
			p.send(platform.Event{Kind: platform.EventResize, Window: 1, PhysicalWidth: 400, PhysicalHeight: 200, Scale: 2})
		},
		func() {
			if findLabel(win().a11y.root, "元に戻す") == nil {
				t.Error("a maximized window offers to restore it")
			}
			p.send(platform.Event{Kind: platform.EventClose, Window: 1})
		},
	)
	p.a11y = &fakeA11y{}
	home := basic.Column{Children: []core.Widget{
		TitleBar{Child: basic.Text{Content: "Document"}},
		basic.Expanded{Child: basic.SizedBox{}},
	}}
	if err := run(p, b, testutil.NewShaper(t), App{Title: "t", Home: home, Locale: language.Japanese}); err != nil {
		t.Fatal(err)
	}
}
