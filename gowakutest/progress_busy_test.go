package gowakutest_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
)

// busy shows an indeterminate ring and bar, the bar named by the app and the
// ring by the window's language.
func busy() gowaku.Widget {
	return basic.Center{Child: basic.SizedBox{Width: 200, Child: basic.Column{Spacing: 16, Children: []gowaku.Widget{
		control.ProgressRing{Indeterminate: true, Value: 0.5},
		control.ProgressBar{Indeterminate: true, Value: 0.5, Label: "Uploading"},
	}}}}
}

// TestProgress_IndeterminateSpins checks that an indeterminate ring and bar
// say busy with no value, and move while they are seen.
func TestProgress_IndeterminateSpins(t *testing.T) {
	tt := gowakutest.New(t, busy(), gowakutest.Options{Width: 300, Height: 200, Settle: time.Second})
	for _, label := range []string{"Loading…", "Uploading"} {
		n, ok := tt.FindRole(semantics.RoleProgress, label)
		if !ok {
			t.Fatalf("no busy indicator %q:\n%s", label, tt.Dump())
		}
		if n.Value != "" || n.Range != (semantics.Range{}) {
			t.Errorf("%q says value %q, range %+v; want none", label, n.Value, n.Range)
		}
	}
	tt.Pump()
	if !tt.Tree().Ticking() {
		t.Fatal("an indeterminate indicator asks for no frames")
	}
	before := tt.Screenshot().Pix
	tt.Advance(300 * time.Millisecond)
	if bytes.Equal(before, tt.Screenshot().Pix) {
		t.Error("the indicators did not move in 300ms")
	}
}

// TestProgress_IndeterminateStandsStillForLessMotion checks that with less
// motion asked for the indicators stand still and ask for no frames.
func TestProgress_IndeterminateStandsStillForLessMotion(t *testing.T) {
	tt := gowakutest.New(t, busy(), gowakutest.Options{Width: 300, Height: 200, ReduceMotion: true})
	tt.Settle()
	if tt.Tree().Ticking() {
		t.Fatal("a still indicator asks for frames")
	}
	before := tt.Screenshot().Pix
	tt.Advance(300 * time.Millisecond)
	if !bytes.Equal(before, tt.Screenshot().Pix) {
		t.Error("the indicators moved although the window asks for less motion")
	}
}

// TestProgress_DeterminateByDefault checks that the zero value of the field
// keeps the share, which a screen reader hears, and asks for no frames.
func TestProgress_DeterminateByDefault(t *testing.T) {
	tt := gowakutest.New(t, basic.Center{Child: control.ProgressRing{Value: 0.25, Label: "Copy"}}, gowakutest.Options{})
	tt.Settle()
	n, ok := tt.FindRole(semantics.RoleProgress, "Copy")
	if !ok || n.Value != "25%" || n.Range.Max != 100 || n.Range.Value != 25 {
		t.Errorf("got %+v", n)
	}
	if tt.Tree().Ticking() {
		t.Error("a determinate ring at rest asks for frames")
	}
}

// TestProgress_IndeterminateRestsWhenHidden checks that a spinner that
// nobody sees, under a fully transparent Opacity, asks for no frames.
func TestProgress_IndeterminateRestsWhenHidden(t *testing.T) {
	tt := gowakutest.New(t, basic.Opacity{Opacity: 0, Child: busy()}, gowakutest.Options{Width: 300, Height: 200})
	tt.Settle()
	if tt.Tree().Ticking() {
		t.Error("a hidden indicator asks for frames")
	}
}
