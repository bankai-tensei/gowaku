package overlay

import (
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
)

// openDialog shows a Dialog with a Close button from the sheet rig. request
// is its OnDismissRequest; the dismiss function it returns closes it from code.
func (r *sheetRig) openDialog(request func() bool) *func() {
	var closeIt func()
	ShowDialog(r.ctx, func(dismiss func()) core.Widget {
		closeIt = dismiss
		return Dialog{Title: "下書き", OnDismissRequest: request, Actions: []core.Widget{
			control.Button{Label: "閉じる", OnPress: dismiss},
		}}
	})
	wtest.Ops(r.tree, wtest.Screen)
	wtest.Settled(r.t, r.tree, wtest.Screen, &r.now)
	return &closeIt
}

func TestDialog_ClosesOnEscapeAndScrimWithoutARequest(t *testing.T) {
	for _, how := range []string{"escape", "scrim"} {
		r := newSheetRig(t, basic.MediaQueryData{Scale: 1})
		r.openDialog(nil)
		if r.sheet() == nil {
			t.Fatal("the dialog did not open")
		}
		if how == "escape" {
			r.tree.DispatchKey(core.KeyEvent{Key: input.KeyEscape})
		} else {
			wtest.Tap(r.tree, geom.Offset{X: 10, Y: 10})
		}
		wtest.Settled(t, r.tree, wtest.Screen, &r.now)
		if r.sheet() != nil {
			t.Errorf("%s: the dialog stays open", how)
		}
	}
}

func TestDialog_OnDismissRequestDecides(t *testing.T) {
	r := newSheetRig(t, basic.MediaQueryData{Scale: 1})
	asked := 0
	closeIt := r.openDialog(func() bool { asked++; return false })
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyEscape})
	// A held Escape asks once.
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyEscape, Repeat: true})
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	wtest.Tap(r.tree, geom.Offset{X: 10, Y: 10})
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	if r.sheet() == nil {
		t.Fatal("the dialog closed although OnDismissRequest kept it")
	}
	if asked != 2 {
		t.Errorf("OnDismissRequest ran %d times, want 2 (Escape and the scrim)", asked)
	}
	if r.pressed != 0 {
		t.Error("the scrim must hold back the window")
	}
	// A press on the card asks nothing.
	inside := wtest.Center(r.sheet().Rect)
	wtest.Tap(r.tree, geom.Offset{X: inside.X, Y: inside.Y - 20})
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	if asked != 2 {
		t.Errorf("a press on the dialog asked to close it")
	}
	// The app decides to close it.
	(*closeIt)()
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	if r.sheet() != nil {
		t.Error("the dismiss function must close the dialog")
	}
}

func TestDialog_DismissButtonClosesDespiteARequest(t *testing.T) {
	r := newSheetRig(t, basic.MediaQueryData{Scale: 1})
	asked := 0
	r.openDialog(func() bool { asked++; return false })
	root, _ := r.tree.SemanticsTree()
	wtest.Tap(r.tree, wtest.Center(wtest.ByLabel(t, root, "閉じる").Rect))
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	if r.sheet() != nil || asked != 0 {
		t.Errorf("open %v, asked %d: the dismiss function closes without asking", r.sheet() != nil, asked)
	}
}

func TestSheet_OnDismissRequestDecides(t *testing.T) {
	for _, persistent := range []bool{false, true} {
		r := newSheetRig(t, basic.MediaQueryData{Scale: 1})
		asked := 0
		r.open(Sheet{Persistent: persistent, OnDismissRequest: func() bool { asked++; return false }})
		wtest.Settled(t, r.tree, wtest.Screen, &r.now)
		r.tree.DispatchKey(core.KeyEvent{Key: input.KeyEscape})
		wtest.Tap(r.tree, geom.Offset{X: 10, Y: 10})
		wtest.Settled(t, r.tree, wtest.Screen, &r.now)
		if r.sheet() == nil {
			t.Fatalf("Persistent %v: the sheet closed although OnDismissRequest kept it", persistent)
		}
		want := 2
		if persistent {
			// A Persistent sheet ignores the click.
			want = 1
		}
		if asked != want {
			t.Errorf("Persistent %v: OnDismissRequest ran %d times, want %d", persistent, asked, want)
		}
	}
}
