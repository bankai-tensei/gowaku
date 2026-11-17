package overlay

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
)

// sheetRig is a window with a button, from which sheets open.
type sheetRig struct {
	t       *testing.T
	tree    *core.Tree
	ctx     core.BuildContext
	now     time.Duration
	pressed int
}

func newSheetRig(t *testing.T, mq basic.MediaQueryData) *sheetRig {
	t.Helper()
	r := &sheetRig{t: t, now: time.Second}
	r.tree = wtest.TextTree(t, basic.MediaQuery{Data: mq, Child: Overlay{Child: wtest.Capture{Got: &r.ctx, Child: basic.Align{
		Alignment: layout.AlignTopLeft,
		Child:     control.Button{Label: "開く", OnPress: func() { r.pressed++ }},
	}}}})
	wtest.Ops(r.tree, wtest.Screen)
	return r
}

func (r *sheetRig) open(s Sheet) {
	ShowSheet(r.ctx, s, func(dismiss func()) core.Widget {
		return basic.Column{Size: layout.MainMin, Spacing: 8, Children: []core.Widget{
			basic.Text{Content: "本文"},
			control.Button{Label: "閉じる", OnPress: dismiss},
			control.Button{Label: "保存", OnPress: dismiss},
		}}
	})
	wtest.Ops(r.tree, wtest.Screen)
}

// sheet returns the node of the open sheet, or nil.
func (r *sheetRig) sheet() *semantics.Node {
	wtest.Ops(r.tree, wtest.Screen)
	root, _ := r.tree.SemanticsTree()
	return wtest.FindRole(root, semantics.RoleDialog)
}

func TestSheet_RestsAgainstItsEdge(t *testing.T) {
	over := float32(16)
	cases := []struct {
		name string
		s    Sheet
		want geom.Rect
	}{
		{"下", Sheet{}, geom.Rect{X: 0, W: 400}},
		{"下で高さ指定", Sheet{Size: 120}, geom.Rect{X: 0, Y: 300 - 120, W: 400, H: 120 + over}},
		{"右", Sheet{Edge: SheetTrailing}, geom.Rect{X: 400 - sheetWidth, Y: 0, W: sheetWidth + over, H: 300}},
		{"右で幅指定は 9 割まで", Sheet{Edge: SheetTrailing, Size: 1000}, geom.Rect{X: 40, Y: 0, W: 360 + over, H: 300}},
		{"左", Sheet{Edge: SheetLeading}, geom.Rect{X: -over, Y: 0, W: sheetWidth + over, H: 300}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			r := newSheetRig(t, basic.MediaQueryData{Scale: 1})
			r.open(tt.s)
			wtest.Advance(r.tree, wtest.Screen, &r.now, 16*time.Millisecond)
			early := r.sheet().Rect
			wtest.Settled(t, r.tree, wtest.Screen, &r.now)
			got := r.sheet().Rect
			if tt.want.H == 0 {
				// A bottom sheet fits its content, and its lower corners hang
				// below the window.
				tt.want.H = got.H
				tt.want.Y = 300 + over - got.H
				if got.H <= over || got.H > 300*sheetMaxShare+over {
					t.Errorf("height %v", got.H)
				}
			}
			if !wtest.NearRect(got, tt.want) {
				t.Errorf("rests at %v, want %v", got, tt.want)
			}
			if early == got {
				t.Error("the sheet must slide in")
			}
		})
	}
}

func TestSheet_KeepsTabAndClosesOnEscape(t *testing.T) {
	r := newSheetRig(t, basic.MediaQueryData{Scale: 1})
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	opener := r.tree.Focused()
	if opener == nil {
		t.Fatal("the button must take focus")
	}
	r.open(Sheet{Edge: SheetTrailing, Label: "詳細"})
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	seen := map[core.FocusTarget]bool{}
	for range 6 {
		r.tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
		wtest.Ops(r.tree, wtest.Screen)
		seen[r.tree.Focused()] = true
	}
	if seen[opener] {
		t.Error("Tab must stay in the sheet")
	}
	if len(seen) != 3 {
		t.Errorf("the scope and two buttons must take turns, got %d", len(seen))
	}
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyEscape})
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	if r.sheet() != nil {
		t.Fatal("Escape must close the sheet")
	}
	if r.tree.Focused() != opener {
		t.Error("the focus must go back to the opener")
	}
}

func TestSheet_ScrimTap(t *testing.T) {
	for _, persistent := range []bool{false, true} {
		r := newSheetRig(t, basic.MediaQueryData{Scale: 1})
		r.open(Sheet{Persistent: persistent})
		wtest.Settled(t, r.tree, wtest.Screen, &r.now)
		inside := wtest.Center(r.sheet().Rect)
		wtest.Tap(r.tree, geom.Offset{X: inside.X, Y: inside.Y - 40})
		wtest.Tap(r.tree, geom.Offset{X: 200, Y: 300 - r.sheet().Rect.H + 20})
		wtest.Settled(t, r.tree, wtest.Screen, &r.now)
		if r.sheet() == nil {
			t.Fatalf("Persistent %v: a press on the sheet closed it", persistent)
		}
		wtest.Tap(r.tree, geom.Offset{X: 10, Y: 10})
		wtest.Settled(t, r.tree, wtest.Screen, &r.now)
		if open := r.sheet() != nil; open != persistent {
			t.Errorf("Persistent %v: open after a press on the scrim = %v", persistent, open)
		}
		if r.pressed != 0 {
			t.Error("the scrim must hold back the window")
		}
	}
}

func TestSheet_DismissButton(t *testing.T) {
	r := newSheetRig(t, basic.MediaQueryData{Scale: 1})
	r.open(Sheet{})
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	root, _ := r.tree.SemanticsTree()
	wtest.Tap(r.tree, wtest.Center(wtest.ByLabel(t, root, "閉じる").Rect))
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	if r.sheet() != nil {
		t.Error("the dismiss function must close the sheet")
	}
}

func TestSheet_Semantics(t *testing.T) {
	r := newSheetRig(t, basic.MediaQueryData{Scale: 1})
	r.open(Sheet{Edge: SheetLeading, Label: "フィルター"})
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	n := r.sheet()
	if n == nil || n.Label != "フィルター" {
		t.Fatalf("got %+v", n)
	}
	if wtest.ByLabel(t, n, "本文").Role != semantics.RoleText || wtest.ByLabel(t, n, "保存").Role != semantics.RoleButton {
		t.Error("the content belongs to the sheet")
	}
}

func TestSheet_ReduceMotionFadesInPlace(t *testing.T) {
	r := newSheetRig(t, basic.MediaQueryData{Scale: 1, ReduceMotion: true})
	r.open(Sheet{Edge: SheetTrailing})
	wtest.Advance(r.tree, wtest.Screen, &r.now, 24*time.Millisecond)
	if got := r.sheet().Rect.X; got != 400-sheetWidth {
		t.Errorf("the sheet must not slide: x %v", got)
	}
}

func TestSheetEdge_String(t *testing.T) {
	for e, want := range map[SheetEdge]string{SheetBottom: "SheetBottom", SheetTrailing: "SheetTrailing", SheetLeading: "SheetLeading", 9: "unknown"} {
		if got := e.String(); got != want {
			t.Errorf("%d: got %q", e, got)
		}
	}
}

// labeled returns the first node under n labeled label, or nil.
func labeled(n *semantics.Node, label string) *semantics.Node {
	var got *semantics.Node
	n.Walk(func(m *semantics.Node) bool {
		if got == nil && m.Label == label {
			got = m
		}
		return got == nil
	})
	return got
}

func TestSheet_HidesThePageFromAssistiveTechnology(t *testing.T) {
	r := newSheetRig(t, basic.MediaQueryData{Scale: 1})
	root, _ := r.tree.SemanticsTree()
	page := labeled(root, "開く")
	if page == nil {
		t.Fatal("no page button")
	}
	id := page.ID
	r.open(Sheet{Label: "フィルター"})
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	root, _ = r.tree.SemanticsTree()
	if labeled(root, "開く") != nil {
		t.Error("the page under the sheet is in the tree")
	}
	if labeled(root, "保存") == nil {
		t.Error("the sheet is not in the tree")
	}
	if r.tree.PerformSemanticsAction(id, semantics.ActionTap, core.SemanticsArg{}) || r.pressed != 0 {
		t.Error("a screen reader pressed the page under the sheet")
	}
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyEscape})
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	root, _ = r.tree.SemanticsTree()
	if labeled(root, "開く") == nil {
		t.Error("the page must come back when the sheet closes")
	}
}
