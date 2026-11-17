package main

import (
	"strconv"
	"unicode/utf8"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/layout"
)

// The page of Input.Obscure: a sign-in form whose password field draws
// bullets, keeps the IME off and refuses copy and cut. See
// docs/ADR/0088-input-obscure.md.

// pageObscure returns the page of the password field.
func pageObscure() core.Widget { return obscurePage{} }

type obscurePage struct{ core.StatefulBase }

type obscurePageState struct {
	core.StateOf[obscurePage]
	user, password string
	shown          bool
	form           field.FormController
	status         string
}

func (obscurePage) CreateState() core.State { return &obscurePageState{} }

func (s *obscurePageState) signIn() {
	// The status tells the length, never the password.
	n := utf8.RuneCountInString(s.password)
	s.SetState(func() {
		s.status = s.user + " でサインインしました（パスワード " + strconv.Itoa(n) + " 文字）"
	})
}

func (s *obscurePageState) Build(ctx core.BuildContext) core.Widget {
	th := basic.ThemeOf(ctx)
	note := th.Text.Caption
	note.Color = th.Color.Ink2
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStart, Spacing: 16, Children: []core.Widget{
		heading(th, "パスワード欄", "Input.Obscure は伏せ字で描き、IME を切り、コピーと切り取りを止めます"),
		basic.SizedBox{Width: 320, Child: field.Form{Controller: &s.form, OnSubmit: s.signIn, Child: basic.Column{
			Size: layout.MainMin, Cross: layout.CrossStretch, Spacing: 12, Children: []core.Widget{
				field.FormItem[string]{Value: s.user, Required: true, Child: field.Input{
					Label: "ユーザー名", Placeholder: "you@example.com", Value: s.user,
					OnChange: func(v string) { s.SetState(func() { s.user = v }) },
				}},
				field.FormItem[string]{Value: s.password, Required: true, Child: field.Input{
					Label: "パスワード", Placeholder: "8 文字以上", Value: s.password, Obscure: !s.shown,
					OnChange: func(v string) { s.SetState(func() { s.password = v }) },
				}},
				control.Checkbox{Label: "パスワードを表示", Value: s.shown, OnChange: func(v bool) {
					s.SetState(func() { s.shown = v })
				}},
				control.Button{Label: "サインイン", Style: control.ButtonPrimary, OnPress: func() { s.form.Submit() }},
			},
		}}},
		basic.Text{Content: "日本語入力をオンにしたままパスワード欄に移っても、かなに変わりません。", Style: note},
		basic.Text{Content: s.status, Style: note},
	}}
}
