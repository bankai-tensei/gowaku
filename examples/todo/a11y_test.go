package main

import (
	"testing"

	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
)

// TestA11y_Tree is the tree a screen reader gets from todo.
func TestA11y_Tree(t *testing.T) {
	tt := gowakutest.New(t, todo{}, gowakutest.Options{Width: 520, Height: 560})
	t.Log("\n" + tt.Dump())
	for _, w := range tt.Warnings() {
		t.Errorf("warning: %v", w)
	}
	if n, ok := tt.Find("Todo"); !ok || n.Role != semantics.RoleHeading {
		t.Errorf("the title must be a heading: %+v", n)
	}
	if n, ok := tt.Find("残り 2 件"); !ok || n.Live != semantics.LivePolite {
		t.Errorf("the count must be a live region: %+v", n)
	}
	for _, d := range tt.FindAll("削除") {
		if d.Hint == "" {
			t.Errorf("a 削除 button without a hint: %+v", d)
		}
	}
}

// TestA11y_KeyboardOnly runs the filter and a row with the keyboard alone.
func TestA11y_KeyboardOnly(t *testing.T) {
	tt := gowakutest.New(t, todo{}, gowakutest.Options{Width: 520, Height: 560})
	for range 3 {
		tt.Key(input.KeyTab, 0)
	}
	tt.Key(input.KeyRight, 0)
	if _, ok := tt.Find("牛乳を買う"); ok {
		t.Fatal("Right on the filter must show only the open items")
	}
	if n, _ := tt.Find("未完了"); n == nil || n.State&semantics.Selected == 0 {
		t.Errorf("未完了 must be the chosen one: %+v", n)
	}
}
