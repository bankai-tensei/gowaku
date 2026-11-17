package main

import (
	"strings"
	"testing"

	"golang.org/x/text/language"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
)

// testerDriver drives a gowakutest.Tester as bringTo drives the window of -png.
type testerDriver struct{ tt *gowakutest.Tester }

func (d testerDriver) tree() *core.Tree { return d.tt.Tree() }

func (d testerDriver) settle() error {
	d.tt.Settle()
	return nil
}

func (d testerDriver) frames(n int) error {
	for i := range n {
		tree := d.tt.Tree()
		if i > 0 && !tree.Ticking() && !tree.NeedsFrame() {
			break
		}
		d.tt.Advance(gowakutest.FrameTime)
	}
	return nil
}

// stateWant is what a state of -state must end with: the page chosen in the
// sidebar, the control with the keyboard and its value, a node on show.
type stateWant struct {
	page              string
	focusRole         semantics.Role
	focusLabel, value string
	shows             semantics.Role
	showsLabel        string
	// ring means the focus ring shows, as after Tab; invalid are fields that
	// show their errors.
	ring    bool
	invalid []string
}

// TestStates drives the gallery to every state of -state, as -png does but
// without a GPU, and checks that it ends where its name says, so that rows
// added to the sidebar or to a page cannot send a state elsewhere unnoticed.
func TestStates(t *testing.T) {
	wants := map[string]stateWant{
		"hover":  {page: "一般"},
		"focus":  {page: "一般", focusRole: semantics.RoleButton, focusLabel: "保存", ring: true},
		"fields": {page: "一般", focusRole: semantics.RoleTextField, focusLabel: "名前", value: "山田 太郎さん", ring: true},
		// The dialog hides the page, sidebar and all, from assistive technology.
		"dialog": {page: "", shows: semantics.RoleDialog, showsLabel: "保存しますか"},

		"parts":             {page: "部品"},
		"display":           {page: "表示"},
		"forms":             {page: "フォーム"},
		"date":              {page: "フォーム", shows: semantics.RoleDialog, showsLabel: "日付を選択"},
		"submit":            {page: "フォーム", invalid: []string{"ニックネーム", "郵便番号"}},
		"desktop":           {page: "デスクトップ"},
		"desktop-memo":      {page: "デスクトップ", focusRole: semantics.RoleTextField, focusLabel: "検索", value: "TODO"},
		"others":            {page: "そのほか"},
		"navigation":        {page: "ナビゲーション"},
		"navigation-detail": {page: "ナビゲーション", shows: semantics.RoleButton, showsLabel: "メモ"},
	}
	for _, p := range extraPages() {
		wants[p.state] = stateWant{page: p.label}
		wants[p.state+"-end"] = stateWant{page: p.label}
	}
	obscure := wants["obscure"]
	obscure.focusRole, obscure.focusLabel, obscure.value = semantics.RoleTextField, "パスワード", "•••••••"
	obscure.ring = true
	wants["obscure"] = obscure
	lines := wants["lines"]
	lines.shows, lines.showsLabel = semantics.RoleMenuItem, "名前を変更"
	wants["lines"] = lines
	for _, name := range stateNames {
		if _, ok := wants[name]; !ok {
			t.Errorf("state %q has no expectation here", name)
		}
	}
	for name, want := range wants {
		t.Run(name, func(t *testing.T) {
			w, h := windowSize(name)
			tt := gowakutest.New(t, gallery{locale: language.Japanese}, gowakutest.Options{Width: w, Height: h, Theme: theme.Default()})
			tt.Settle()
			if err := bringTo(testerDriver{tt}, name); err != nil {
				t.Fatal(err)
			}
			root, focus := tt.Tree().SemanticsTree()
			if row := selectedRow(root); row != want.page {
				t.Errorf("the sidebar has %q chosen, want %q", row, want.page)
			}
			if want.focusLabel != "" {
				f := root.Find(focus)
				switch {
				case focus == 0 || f == nil:
					t.Errorf("nothing has the keyboard, want %v %q", want.focusRole, want.focusLabel)
				case f.Role != want.focusRole || f.Label != want.focusLabel:
					t.Errorf("%v %q has the keyboard, want %v %q", f.Role, f.Label, want.focusRole, want.focusLabel)
				case want.value != "" && f.Value != want.value:
					t.Errorf("%v %q holds %q, want %q", f.Role, f.Label, f.Value, want.value)
				}
			}
			if want.ring && !tt.Tree().FocusVisible() {
				t.Error("the focus ring is hidden")
			}
			for _, label := range want.invalid {
				if n := findNode(root, semantics.RoleTextField, label); n == nil || n.State&semantics.Invalid == 0 {
					t.Errorf("the field %q shows no error", label)
				}
			}
			if want.showsLabel != "" && findNode(root, want.shows, want.showsLabel) == nil {
				t.Errorf("no %v %q:\n%s", want.shows, want.showsLabel, tt.Dump())
			}
		})
	}
}

// selectedRow returns the label of the chosen row of the sidebar.
func selectedRow(root *semantics.Node) string {
	var row string
	root.Walk(func(n *semantics.Node) bool {
		if n.Role == semantics.RoleListItem && n.State&semantics.Selected != 0 {
			row = n.Label
		}
		return row == ""
	})
	return row
}

// The window of the gallery speaks Japanese, as its pages do, whatever the
// OS: the words of the library in the date picker are Japanese.
func TestGallery_SpeaksJapaneseOnAnEnglishOS(t *testing.T) {
	tt := gowakutest.New(t, englishOS{child: gallery{locale: language.Japanese}}, gowakutest.Options{Width: 860, Height: 720, Theme: theme.Default()})
	if err := bringTo(testerDriver{tt}, "date"); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"前の月", "次の月", "今日"} {
		if _, ok := tt.Find(label); !ok {
			t.Errorf("no %q in the date picker:\n%s", label, tt.Dump())
		}
	}
	if strings.Contains(tt.Dump(), "Today") {
		t.Error("the date picker speaks English")
	}
}

// englishOS stands for the root that Run gives a window on an English OS.
type englishOS struct {
	core.StatelessBase
	child core.Widget
}

func (e englishOS) Build(ctx core.BuildContext) core.Widget {
	md := basic.MediaQueryOf(ctx)
	md.Locale = language.AmericanEnglish
	return basic.MediaQuery{Data: md, Child: basic.Localizations{Child: e.child}}
}
