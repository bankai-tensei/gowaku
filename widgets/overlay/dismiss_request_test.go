package overlay_test

import (
	"testing"
	"time"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

var fonts = gowakutest.Options{Fonts: [][]byte{goregular.TTF}}

// place keeps the BuildContext of the page, from which the tests open dialogs
// and sheets as a callback would.
type place struct {
	core.StatelessBase
	ctx *core.BuildContext
}

func (p place) Build(ctx core.BuildContext) core.Widget {
	*p.ctx = ctx
	return basic.Text{Content: "Page"}
}

func newPlace(t *testing.T) (*gowakutest.Tester, core.BuildContext) {
	var ctx core.BuildContext
	tt := gowakutest.New(t, place{ctx: &ctx}, fonts)
	return tt, ctx
}

// draft is the content of the dialogs and sheets of the tests.
func draft(func()) core.Widget { return basic.Text{Content: "Draft"} }

// escape presses Escape and lets the dialog or sheet fade out if it closes.
func escape(tt *gowakutest.Tester) {
	tt.Key(input.KeyEscape, 0)
	tt.Advance(300 * time.Millisecond)
}

func open(tt *gowakutest.Tester) bool {
	_, ok := tt.Find("Draft")
	return ok
}

func TestDialog_OnDismissRequestReturnsWhetherToClose(t *testing.T) {
	tt, ctx := newPlace(t)
	allow, asked := false, 0
	overlay.ShowDialog(ctx, func(dismiss func()) core.Widget {
		return overlay.Dialog{Title: "Draft", OnDismissRequest: func() bool {
			asked++
			return allow
		}}
	})
	tt.Settle()
	escape(tt)
	if !open(tt) {
		t.Fatal("the dialog closed although OnDismissRequest returned false")
	}
	allow = true
	escape(tt)
	if open(tt) {
		t.Error("the dialog stays open although OnDismissRequest returned true")
	}
	if asked != 2 {
		t.Errorf("OnDismissRequest ran %d times, want 2", asked)
	}
}

func TestSheet_OnDismissRequestReturnsWhetherToClose(t *testing.T) {
	for _, persistent := range []bool{false, true} {
		tt, ctx := newPlace(t)
		allow := false
		overlay.ShowSheet(ctx, overlay.Sheet{Label: "Draft", Persistent: persistent, OnDismissRequest: func() bool {
			return allow
		}}, draft)
		tt.Settle()
		escape(tt)
		if !open(tt) {
			t.Fatalf("Persistent %v: the sheet closed although OnDismissRequest returned false", persistent)
		}
		allow = true
		escape(tt)
		if open(tt) {
			t.Errorf("Persistent %v: the sheet stays open although OnDismissRequest returned true", persistent)
		}
	}
}
