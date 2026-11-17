package main

import (
	"os"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

// TestPageObscure signs in on the page. GALLERY_OBSCURE_PNG, if set, is a
// directory to save screenshots of it in, for review.
func TestPageObscure(t *testing.T) {
	tt := gowakutest.New(t, basic.Padding{Insets: geom.All(24), Child: pageObscure()},
		gowakutest.Options{Width: 520, Height: 560, Scale: 2, Theme: theme.Default(), Settle: 500e6})
	tt.Tap("ユーザー名")
	tt.Type("taro")
	tt.Tap("パスワード")
	tt.Type("hunter2")
	n, ok := tt.Find("パスワード")
	if !ok || n.Value != "•••••••" || n.State&semantics.Obscured == 0 {
		t.Fatalf("the password field gives %+v", n)
	}
	shot := func(name string) {
		if dir := os.Getenv("GALLERY_OBSCURE_PNG"); dir != "" {
			tt.SaveScreenshot(dir + "/" + name + ".png")
		}
	}
	shot("hidden")
	tt.Tap("パスワードを表示")
	if n, _ := tt.Find("パスワード"); n.Value != "hunter2" {
		t.Errorf("shown, the field gives %q", n.Value)
	}
	shot("shown")
	tt.Tap("パスワードを表示")
	tt.Tap("パスワード")
	tt.Key(input.KeyEnter, 0)
	if _, ok := tt.Find("taro でサインインしました（パスワード 7 文字）"); !ok {
		t.Errorf("Enter must sign in:\n%s", tt.Dump())
	}
	shot("signed-in")
}
