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

// tooltipRig is a window with two buttons that have tooltips.
type tooltipRig struct {
	t    *testing.T
	tree *core.Tree
	now  time.Duration
}

func newTooltipRig(t *testing.T) *tooltipRig {
	t.Helper()
	tip := func(label, message string) core.Widget {
		return Tooltip{Message: message, Child: control.Button{Label: label, OnPress: func() {}}}
	}
	r := &tooltipRig{t: t, now: time.Second}
	r.tree = wtest.TextTree(t, Overlay{Child: basic.Padding{Insets: geom.All(20), Child: basic.Row{Spacing: 8, Children: []core.Widget{
		tip("太字", "太字 (Ctrl+B)"),
		tip("斜体", "斜体 (Ctrl+I)"),
		control.Button{Label: "他", OnPress: func() {}},
	}}}})
	wtest.Ops(r.tree, wtest.Screen)
	return r
}

// at returns the middle of the control labeled label.
func (r *tooltipRig) at(label string) geom.Offset {
	r.t.Helper()
	root, _ := r.tree.SemanticsTree()
	return wtest.Center(wtest.ByLabel(r.t, root, label).Rect)
}

func (r *tooltipRig) move(label string) {
	r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: r.at(label)})
	wtest.Ops(r.tree, wtest.Screen)
}

func (r *tooltipRig) wait(d time.Duration) { wtest.Advance(r.tree, wtest.Screen, &r.now, d) }

// shown returns the label of the tooltip showing, or "".
func (r *tooltipRig) shown() string {
	wtest.Ops(r.tree, wtest.Screen)
	root, _ := r.tree.SemanticsTree()
	if n := wtest.FindRole(root, semantics.RoleTooltip); n != nil {
		return n.Label
	}
	return ""
}

func TestTooltip_ShowsAfterTheDelay(t *testing.T) {
	r := newTooltipRig(t)
	r.move("太字")
	r.wait(400 * time.Millisecond)
	if got := r.shown(); got != "" {
		t.Fatalf("shown after 400ms: %q", got)
	}
	r.wait(150 * time.Millisecond)
	if got := r.shown(); got != "太字 (Ctrl+B)" {
		t.Fatalf("after 550ms got %q", got)
	}
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	if r.tree.Ticking() {
		t.Error("a tooltip at rest must not ask for frames")
	}
}

func TestTooltip_SitsBelowTheControl(t *testing.T) {
	r := newTooltipRig(t)
	r.move("太字")
	r.wait(600 * time.Millisecond)
	root, _ := r.tree.SemanticsTree()
	button := wtest.ByLabel(t, root, "太字").Rect
	tip := wtest.FindRole(root, semantics.RoleTooltip)
	if tip == nil {
		t.Fatal("no tooltip")
	}
	if got := tip.Rect.Y - (button.Y + button.H); got != tooltipGap {
		t.Errorf("gap %v, want %v", got, tooltipGap)
	}
	if mid := tip.Rect.X + tip.Rect.W/2; geom.Abs32(mid-(button.X+button.W/2)) > 0.5 && tip.Rect.X > tooltipEdge {
		t.Errorf("the tooltip %v must center under %v", tip.Rect, button)
	}
}

func TestTooltip_FlipsAboveNearTheBottom(t *testing.T) {
	now := time.Second
	tree := wtest.TextTree(t, Overlay{Child: basic.Align{Alignment: layout.AlignBottomCenter, Child: Tooltip{
		Message: "下端", Child: control.Button{Label: "b", OnPress: func() {}},
	}}})
	wtest.Ops(tree, wtest.Screen)
	root, _ := tree.SemanticsTree()
	button := wtest.ByLabel(t, root, "b").Rect
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: wtest.Center(button)})
	wtest.Advance(tree, wtest.Screen, &now, 600*time.Millisecond)
	wtest.Ops(tree, wtest.Screen)
	root, _ = tree.SemanticsTree()
	tip := wtest.FindRole(root, semantics.RoleTooltip)
	if tip == nil || tip.Rect.Y+tip.Rect.H > button.Y {
		t.Fatalf("the tooltip must go above the button at the bottom: %+v over %v", tip, button)
	}
}

func TestTooltip_WarmMovesShowAtOnce(t *testing.T) {
	r := newTooltipRig(t)
	r.move("太字")
	r.wait(600 * time.Millisecond)
	r.move("斜体")
	// The first fades out in 80 milliseconds.
	r.wait(100 * time.Millisecond)
	if got := r.shown(); got != "斜体 (Ctrl+I)" {
		t.Fatalf("moving on got %q, want the next tooltip at once", got)
	}
	// Off every tooltip for longer than the warmth, the delay comes back.
	r.move("他")
	r.wait(1500 * time.Millisecond)
	r.move("太字")
	r.wait(100 * time.Millisecond)
	if got := r.shown(); got != "" {
		t.Errorf("after the warmth ended got %q at once", got)
	}
}

func TestTooltip_Hides(t *testing.T) {
	cases := []struct {
		name string
		do   func(r *tooltipRig)
	}{
		{"ポインタが離れる", func(r *tooltipRig) { r.move("他") }},
		{"押す", func(r *tooltipRig) { wtest.Tap(r.tree, r.at("太字")) }},
		{"Escape", func(r *tooltipRig) { r.tree.DispatchKey(core.KeyEvent{Key: input.KeyEscape}) }},
		{"ホイール", func(r *tooltipRig) {
			r.tree.DispatchPointer(core.PointerEvent{Kind: core.PointerScroll, Pos: r.at("太字"), Scroll: geom.Offset{Y: -1}})
		}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			r := newTooltipRig(t)
			r.move("太字")
			r.wait(600 * time.Millisecond)
			if r.shown() == "" {
				t.Fatal("no tooltip to hide")
			}
			tt.do(r)
			wtest.Settled(t, r.tree, wtest.Screen, &r.now)
			if got := r.shown(); got != "" {
				t.Errorf("still shows %q", got)
			}
			// Resting on the control again does not bring it back until the
			// pointer leaves, except after the pointer left.
			r.wait(time.Second)
			if got := r.shown(); got != "" && tt.name != "ポインタが離れる" {
				t.Errorf("came back as %q without leaving", got)
			}
		})
	}
}

func TestTooltip_KeyboardShowsAtOnce(t *testing.T) {
	r := newTooltipRig(t)
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	// The label is fully transparent, and so hidden, until its first step.
	r.wait(32 * time.Millisecond)
	if got := r.shown(); got != "太字 (Ctrl+B)" {
		t.Fatalf("Tab to the control got %q", got)
	}
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	if got := r.shown(); got != "斜体 (Ctrl+I)" {
		t.Fatalf("Tab on got %q", got)
	}
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyEscape})
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	if got := r.shown(); got != "" {
		t.Errorf("Escape left %q", got)
	}
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	wtest.Settled(t, r.tree, wtest.Screen, &r.now)
	if got := r.shown(); got != "" {
		t.Errorf("a control without a tooltip shows %q", got)
	}
}

func TestTooltip_PressFocusShowsNothing(t *testing.T) {
	r := newTooltipRig(t)
	wtest.Tap(r.tree, r.at("太字"))
	r.wait(time.Second)
	if r.tree.Focused() == nil {
		t.Fatal("the press must give the button the focus")
	}
	if got := r.shown(); got != "" {
		t.Errorf("a press showed %q", got)
	}
}

func TestTooltip_Semantics(t *testing.T) {
	r := newTooltipRig(t)
	root, _ := r.tree.SemanticsTree()
	button := wtest.ByLabel(t, root, "太字")
	if button.Role != semantics.RoleButton || button.Hint != "太字 (Ctrl+B)" {
		t.Errorf("the button must carry the message as its hint: %+v", button)
	}
	if other := wtest.ByLabel(t, root, "他"); other.Hint != "" {
		t.Errorf("the hint must not reach the next control: %q", other.Hint)
	}
	if wtest.FindRole(root, semantics.RoleTooltip) != nil {
		t.Error("no tooltip node before it shows")
	}
	r.tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	r.wait(32 * time.Millisecond)
	root, _ = r.tree.SemanticsTree()
	tip := wtest.FindRole(root, semantics.RoleTooltip)
	if tip == nil || tip.Label != "太字 (Ctrl+B)" || len(tip.Children) != 0 {
		t.Fatalf("the tooltip node must speak for its text: %+v", tip)
	}
	if wtest.ByLabel(t, root, "太字").State&semantics.Focused == 0 {
		t.Error("the focus stays on the control")
	}
}

func TestTooltip_LeavesWithItsControl(t *testing.T) {
	now := time.Second
	tree, swap := wtest.SwapTree(t, Overlay{Child: Tooltip{Message: "m", Child: control.Button{Label: "b", OnPress: func() {}}}})
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	wtest.Ops(tree, wtest.Screen)
	wtest.Advance(tree, wtest.Screen, &now, 16*time.Millisecond)
	root, _ := tree.SemanticsTree()
	if wtest.FindRole(root, semantics.RoleTooltip) == nil {
		t.Fatal("no tooltip")
	}
	swap(Overlay{Child: basic.Text{Content: "gone"}})
	wtest.Ops(tree, wtest.Screen)
	wtest.Settled(t, tree, wtest.Screen, &now)
	root, _ = tree.SemanticsTree()
	if wtest.FindRole(root, semantics.RoleTooltip) != nil {
		t.Error("the tooltip must leave with its control")
	}
}

func TestTooltip_EmptyMessageShowsNothing(t *testing.T) {
	now := time.Second
	tree := wtest.TextTree(t, Overlay{Child: Tooltip{Child: control.Button{Label: "b", OnPress: func() {}}}})
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	wtest.Settled(t, tree, wtest.Screen, &now)
	root, _ := tree.SemanticsTree()
	if wtest.FindRole(root, semantics.RoleTooltip) != nil || wtest.ByLabel(t, root, "b").Hint != "" {
		t.Error("a Tooltip without a message must stay out of the way")
	}
}
