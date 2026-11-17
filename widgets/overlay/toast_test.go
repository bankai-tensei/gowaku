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

// toastRig is a window with a button in the bottom left corner, beside where
// toasts show.
type toastRig struct {
	t       *testing.T
	tree    *core.Tree
	ctx     core.BuildContext
	now     time.Duration
	pressed int
}

func newToastRig(t *testing.T, mq basic.MediaQueryData) *toastRig {
	t.Helper()
	r := &toastRig{t: t, now: time.Second}
	r.tree = wtest.TextTree(t, basic.MediaQuery{Data: mq, Child: Overlay{Child: wtest.Capture{Got: &r.ctx, Child: basic.Align{
		Alignment: layout.AlignBottomLeft,
		Child:     control.Button{Label: "下", OnPress: func() { r.pressed++ }},
	}}}})
	wtest.Ops(r.tree, wtest.Screen)
	return r
}

func (r *toastRig) wait(d time.Duration) { wtest.Advance(r.tree, wtest.Screen, &r.now, d) }

// toasts returns the live nodes of the toasts, oldest first.
func (r *toastRig) toasts() []*semantics.Node {
	wtest.Ops(r.tree, wtest.Screen)
	root, _ := r.tree.SemanticsTree()
	var out []*semantics.Node
	root.Walk(func(n *semantics.Node) bool {
		if n.Role == semantics.RoleGroup && n.Live == semantics.LivePolite {
			out = append(out, n)
		}
		return true
	})
	return out
}

func (r *toastRig) labels() []string {
	var out []string
	for _, n := range r.toasts() {
		out = append(out, n.Label)
	}
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestToast_StacksAndLeavesOnTime(t *testing.T) {
	r := newToastRig(t, basic.MediaQueryData{Scale: 1})
	ShowToast(r.ctx, Toast{Message: "一つ目", Duration: time.Second})
	ShowToast(r.ctx, Toast{Message: "二つ目"})
	r.wait(500 * time.Millisecond)
	nodes := r.toasts()
	if got := r.labels(); !sameStrings(got, []string{"一つ目", "二つ目"}) {
		t.Fatalf("got %q", got)
	}
	if !(nodes[0].Rect.Y < nodes[1].Rect.Y) {
		t.Error("the newest toast goes at the bottom")
	}
	if b := nodes[1].Rect.Y + nodes[1].Rect.H; geom.Abs32(b-(wtest.Screen.H-toastBottom)) > 0.5 {
		t.Errorf("the newest toast ends at %v, want %v", b, wtest.Screen.H-toastBottom)
	}
	if mid := nodes[1].Rect.X + nodes[1].Rect.W/2; geom.Abs32(mid-wtest.Screen.W/2) > 1 {
		t.Errorf("the toasts center at %v", mid)
	}
	r.wait(800 * time.Millisecond)
	if got := r.labels(); !sameStrings(got, []string{"二つ目"}) {
		t.Fatalf("after 1.3s got %q", got)
	}
	r.wait(3 * time.Second)
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	if got := r.labels(); len(got) != 0 {
		t.Errorf("after 4.3s got %q", got)
	}
	if r.tree.Ticking() {
		t.Error("an empty stack must not ask for frames")
	}
}

func TestToast_WaitsWithoutFrames(t *testing.T) {
	r := newToastRig(t, basic.MediaQueryData{Scale: 1})
	ShowToast(r.ctx, Toast{Message: "m"})
	wtest.Ops(r.tree, wtest.Screen)
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	if wake, ok := r.tree.NextWake(); !ok || wake <= r.now {
		t.Errorf("the count must sleep until it ends: %v %v", wake, ok)
	}
}

func TestToast_HoverHoldsTheCount(t *testing.T) {
	r := newToastRig(t, basic.MediaQueryData{Scale: 1})
	ShowToast(r.ctx, Toast{Message: "待つ", Duration: time.Second})
	r.wait(500 * time.Millisecond)
	over := wtest.Center(r.toasts()[0].Rect)
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: over})
	r.wait(3 * time.Second)
	if got := r.labels(); len(got) != 1 {
		t.Fatalf("a hovered toast left: %q", got)
	}
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 5, Y: 5}})
	// Half a second had passed before the pointer came.
	r.wait(400 * time.Millisecond)
	if got := r.labels(); len(got) != 1 {
		t.Fatalf("the count must go on from where it stopped: %q", got)
	}
	r.wait(400 * time.Millisecond)
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	if got := r.labels(); len(got) != 0 {
		t.Errorf("the toast must leave once the count ends: %q", got)
	}
}

func TestToast_ActionKeyboardAndEscape(t *testing.T) {
	r := newToastRig(t, basic.MediaQueryData{Scale: 1})
	undone := 0
	ShowToast(r.ctx, Toast{Message: "削除しました", Action: "元に戻す", OnAction: func() { undone++ }, Duration: time.Second})
	r.wait(300 * time.Millisecond)
	// The button of the window comes first, then the action.
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	wtest.Ops(r.tree, wtest.Screen)
	root, _ := r.tree.SemanticsTree()
	if wtest.ByLabel(t, root, "元に戻す").State&semantics.Focused == 0 {
		t.Fatal("Tab must reach the action")
	}
	r.wait(2 * time.Second)
	if len(r.toasts()) != 1 {
		t.Fatal("the keyboard in the toast must hold the count")
	}
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyEscape})
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	if len(r.toasts()) != 0 || undone != 0 {
		t.Errorf("Escape must close the toast without the action: %d left, %d undone", len(r.toasts()), undone)
	}

	ShowToast(r.ctx, Toast{Message: "削除しました", Action: "元に戻す", OnAction: func() { undone++ }})
	r.wait(300 * time.Millisecond)
	root, _ = r.tree.SemanticsTree()
	wtest.Tap(r.tree, wtest.Center(wtest.ByLabel(t, root, "元に戻す").Rect))
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	if undone != 1 || len(r.toasts()) != 0 {
		t.Errorf("the action ran %d times and left %d toasts", undone, len(r.toasts()))
	}
}

func TestToast_LimitAndDismiss(t *testing.T) {
	r := newToastRig(t, basic.MediaQueryData{Scale: 1})
	for _, m := range []string{"1", "2", "3"} {
		ShowToast(r.ctx, Toast{Message: m})
	}
	r.wait(100 * time.Millisecond)
	dismiss := ShowToast(r.ctx, Toast{Message: "4"})
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	if got := r.labels(); !sameStrings(got, []string{"2", "3", "4"}) {
		t.Fatalf("a fourth toast must send the first away: %q", got)
	}
	dismiss()
	dismiss()
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	if got := r.labels(); !sameStrings(got, []string{"2", "3"}) {
		t.Errorf("dismiss left %q", got)
	}
	// Closed before it was ever built.
	ShowToast(r.ctx, Toast{Message: "5"})()
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	if got := r.labels(); !sameStrings(got, []string{"2", "3"}) {
		t.Errorf("a toast closed at once left %q", got)
	}
}

func TestToast_LetsThePointerThrough(t *testing.T) {
	r := newToastRig(t, basic.MediaQueryData{Scale: 1})
	ShowToast(r.ctx, Toast{Message: "通知"})
	r.wait(300 * time.Millisecond)
	root, _ := r.tree.SemanticsTree()
	wtest.Tap(r.tree, wtest.Center(wtest.ByLabel(t, root, "下").Rect))
	if r.pressed != 1 {
		t.Error("the window beside a toast must stay usable")
	}
}

func TestToast_Semantics(t *testing.T) {
	r := newToastRig(t, basic.MediaQueryData{Scale: 1})
	ShowToast(r.ctx, Toast{Message: "保存しました", Action: "開く"})
	nodes := r.toasts()
	if len(nodes) != 1 || nodes[0].Label != "" {
		t.Fatalf("the live node must appear empty first: %+v", nodes)
	}
	r.wait(16 * time.Millisecond)
	n := r.toasts()[0]
	if n.Label != "保存しました" || n.Live != semantics.LivePolite || n.Role != semantics.RoleGroup {
		t.Fatalf("got %+v", n)
	}
	if len(n.Children) != 1 || n.Children[0].Role != semantics.RoleButton || n.Children[0].Label != "開く" {
		t.Errorf("the message must not be read twice, and the action must be there: %+v", n.Children)
	}
	if _, focus := r.tree.SemanticsTree(); focus != 0 {
		t.Error("a toast must not take the focus")
	}
}

func TestToast_ReduceMotionFadesInPlace(t *testing.T) {
	r := newToastRig(t, basic.MediaQueryData{Scale: 1, ReduceMotion: true})
	ShowToast(r.ctx, Toast{Message: "m"})
	r.wait(24 * time.Millisecond)
	n := r.toasts()[0]
	if b := n.Rect.Y + n.Rect.H; geom.Abs32(b-(wtest.Screen.H-toastBottom)) > 0.5 || n.Rect.H < toastHeight {
		t.Errorf("the toast must stand in place at once: %v", n.Rect)
	}
}
