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
	overlay.ShowDialog(ctx, func(func()) core.Widget {
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

func TestShowDialog_ReturnsTheDismissOfBuild(t *testing.T) {
	tt, ctx := newPlace(t)
	var inner func()
	dismiss := overlay.ShowDialog(ctx, func(d func()) core.Widget {
		inner = d
		return overlay.Dialog{Title: "Draft", OnDismissRequest: func() bool { return false }}
	})
	tt.Settle()
	if !open(tt) {
		t.Fatal("the dialog did not open")
	}
	// It closes without asking OnDismissRequest.
	dismiss()
	tt.Advance(300 * time.Millisecond)
	if open(tt) {
		t.Fatal("the dismiss ShowDialog returned did not close the dialog")
	}
	// Both are harmless once the dialog is gone.
	dismiss()
	inner()
	tt.Settle()
	if _, ok := tt.Find("Page"); !ok {
		t.Errorf("the page is lost after a late dismiss:\n%s", tt.Dump())
	}
}

func TestShowDialog_DismissTwiceWhileClosing(t *testing.T) {
	tt, ctx := newPlace(t)
	dismiss := overlay.ShowDialog(ctx, func(func()) core.Widget { return overlay.Dialog{Title: "Draft"} })
	tt.Settle()
	dismiss()
	tt.Advance(50 * time.Millisecond)
	dismiss()
	tt.Advance(300 * time.Millisecond)
	if open(tt) {
		t.Error("the dialog stays open")
	}
	// A dialog dismissed before its first frame never shows.
	dismiss = overlay.ShowDialog(ctx, func(func()) core.Widget { return overlay.Dialog{Title: "Draft"} })
	dismiss()
	dismiss()
	tt.Settle()
	if open(tt) {
		t.Error("a dialog dismissed at once still shows")
	}
}

func TestShowSheet_ReturnsTheDismissOfBuild(t *testing.T) {
	tt, ctx := newPlace(t)
	var inner func()
	dismiss := overlay.ShowSheet(ctx, overlay.Sheet{Label: "Draft", Persistent: true}, func(d func()) core.Widget {
		inner = d
		return basic.Text{Content: "Body"}
	})
	tt.Settle()
	if !open(tt) {
		t.Fatal("the sheet did not open")
	}
	inner()
	tt.Advance(50 * time.Millisecond)
	dismiss()
	tt.Advance(300 * time.Millisecond)
	if open(tt) {
		t.Fatal("the sheet stays open")
	}
	dismiss()
	inner()
	tt.Settle()
	if _, ok := tt.Find("Page"); !ok {
		t.Errorf("the page is lost after a late dismiss:\n%s", tt.Dump())
	}
}
