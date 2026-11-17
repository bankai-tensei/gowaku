package nav

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

var stepLabels = []string{"Account", "Shipping", "Payment"}

func TestSteps_ReadsAsAListOfSteps(t *testing.T) {
	tree := wtest.A11yTree(t, basic.Pad(10, Steps{Labels: stepLabels, Current: 1, OnStepPress: func(int) {}}))
	root, _ := tree.SemanticsTree()
	lists := wtest.RolesOf(root, semantics.RoleList)
	if len(lists) != 1 || lists[0].Label != "Steps" {
		t.Fatalf("want one list named Steps, got %v", lists)
	}
	items := wtest.RolesOf(lists[0], semantics.RoleListItem)
	if len(items) != 3 {
		t.Fatalf("%d items, want 3", len(items))
	}
	for i, want := range []struct {
		hint string
		tap  bool
	}{{"Completed", true}, {"Current step", false}, {"", false}} {
		it := items[i]
		if it.Label != stepLabels[i] || it.PosInSet != i+1 || it.SetSize != 3 || it.Hint != want.hint {
			t.Errorf("step %d: %q %d of %d, hint %q; want %q %d of 3, hint %q", i, it.Label, it.PosInSet, it.SetSize, it.Hint, stepLabels[i], i+1, want.hint)
		}
		if got := it.Actions&semantics.ActionTap != 0; got != want.tap {
			t.Errorf("step %d: can be pressed %v, want %v", i, got, want.tap)
		}
		if it.State&semantics.Disabled != 0 {
			t.Errorf("step %d is disabled", i)
		}
		// The number in the circle is the place the item says already.
		if len(it.Children) != 0 {
			t.Errorf("step %d has %d children, want its name read once", i, len(it.Children))
		}
	}
	if h := items[0].Rect.H; h > theme.ControlHeight {
		t.Errorf("a step is %g high, more than a control", h)
	}
}

func TestSteps_GoesBackToADoneStep(t *testing.T) {
	var pressed []int
	tree := wtest.A11yTree(t, basic.Pad(10, Steps{Labels: stepLabels, Current: 2, OnStepPress: func(i int) { pressed = append(pressed, i) }}))
	root, _ := tree.SemanticsTree()
	items := wtest.RolesOf(root, semantics.RoleListItem)
	// A tap on the second done step, and on the current one, which does nothing.
	wtest.Tap(tree, wtest.Center(items[1].Rect))
	wtest.Tap(tree, wtest.Center(items[2].Rect))
	// The keyboard reaches the done steps with Tab, and Enter presses one.
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	wtest.Ops(tree, wtest.Screen)
	if got := wtest.FocusedLabel(tree); got != "Account" {
		t.Errorf("Tab goes to %q, want the first done step", got)
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyEnter})
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeySpace})
	if !slices.Equal(pressed, []int{1, 0, 1}) {
		t.Errorf("pressed %v, want [1 0 1]", pressed)
	}
}

func TestSteps_EveryStepDone(t *testing.T) {
	tree := wtest.A11yTree(t, basic.Pad(10, Steps{Labels: stepLabels, Current: len(stepLabels)}))
	root, _ := tree.SemanticsTree()
	for _, it := range wtest.RolesOf(root, semantics.RoleListItem) {
		if it.Hint != "Completed" || it.Actions&semantics.ActionTap != 0 {
			t.Errorf("%q: hint %q, actions %v; want done, and no press without OnStepPress", it.Label, it.Hint, it.Actions)
		}
	}
	// The lines between done steps take the accent.
	th := theme.Default().Light
	lines := 0
	for _, op := range wtest.Rrects(tree, wtest.Screen) {
		if op.RRect.Rect.H == stepLine && op.Color == th.Color.Accent {
			lines++
		}
	}
	if lines != 2 {
		t.Errorf("%d accent lines, want 2", lines)
	}
}

func TestSteps_ZeroValue(t *testing.T) {
	tree := wtest.A11yTree(t, basic.Pad(10, Steps{}))
	root, _ := tree.SemanticsTree()
	if items := wtest.RolesOf(root, semantics.RoleListItem); len(items) != 0 {
		t.Errorf("an empty Steps has %d items", len(items))
	}
}
