package main

import "testing"

func TestWholeMarks_WidensAHitOnARubyBase(t *testing.T) {
	s := "京都の旅\n金曜の朝、｜嵐山《あらしやま》へ向かう。"
	base := len("京都の旅\n金曜の朝、｜")
	got := wholeMarks(s, match{start: base, end: base + len("嵐山")})
	want := match{start: base - len("｜"), end: base + len("嵐山《あらしやま》")}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	// A hit outside the marks stays as it is.
	plain := match{start: len("京都"), end: len("京都の旅")}
	if got := wholeMarks(s, plain); got != plain {
		t.Errorf("a hit outside a mark moved: %+v", got)
	}
}
