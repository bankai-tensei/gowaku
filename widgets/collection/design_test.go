package collection

import (
	"strings"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/control"
	"github.com/bankai-tensei/gowaku/widgets/field"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/nav"
	"github.com/bankai-tensei/gowaku/widgets/overlay"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/theme"
)

func TestAnimatedContainer_ColorMovesWithoutLayout(t *testing.T) {
	box := func(c geom.Color) core.Widget {
		return basic.AnimatedContainer{Duration: 100 * time.Millisecond, Color: c, Width: 50, Height: 50}
	}
	tree, swap := wtest.SwapTree(t, basic.Align{Child: box(wtest.Red)})
	if got := wtest.Ops(tree, wtest.Screen)[0].Color; got != wtest.Red {
		t.Fatalf("got %+v", got)
	}
	swap(basic.Align{Child: box(wtest.Blue)})
	if got := wtest.Ops(tree, wtest.Screen)[0].Color; got != wtest.Red {
		t.Errorf("the change must not jump, got %+v", got)
	}
	if !tree.Ticking() {
		t.Fatal("the change must start a motion")
	}
	now := time.Second
	wtest.Advance(tree, wtest.Screen, &now, 24*time.Millisecond)
	mid := wtest.Ops(tree, wtest.Screen)[0].Color
	if mid.R <= 0 || mid.R >= 1 || mid.B <= 0 || mid.B >= 1 {
		t.Errorf("got %+v in the middle", mid)
	}
	wtest.Settled(t, tree, wtest.Screen, &now)
	if got := wtest.Ops(tree, wtest.Screen)[0].Color; got != wtest.Blue {
		t.Errorf("got %+v at the end", got)
	}
}

func TestAnimatedContainer_SizeAndShadow(t *testing.T) {
	shadow := []paint.BoxShadow{{Color: geom.Hex(0).WithAlpha(0.5), Blur: 8}}
	first := basic.AnimatedContainer{Duration: 80 * time.Millisecond, Width: 10, Height: 10, Color: wtest.Red}
	tree, swap := wtest.SwapTree(t, basic.Align{Child: first})
	wtest.Ops(tree, wtest.Screen)
	swap(basic.Align{Child: basic.AnimatedContainer{Duration: 80 * time.Millisecond, Width: 110, Height: 10, Color: wtest.Red, Shadows: shadow}})
	wtest.Ops(tree, wtest.Screen)
	now := time.Second
	wtest.Advance(tree, wtest.Screen, &now, 24*time.Millisecond)
	list := wtest.Ops(tree, wtest.Screen)
	last := list[len(list)-1]
	if last.RRect.Rect.W <= 10 || last.RRect.Rect.W >= 110 {
		t.Errorf("got width %v in the middle", last.RRect.Rect.W)
	}
	if list[0].Kind != paint.OpShadow {
		t.Errorf("the shadow must fade in, got %v", list[0].Kind)
	}
	wtest.Settled(t, tree, wtest.Screen, &now)
	if got := wtest.Ops(tree, wtest.Screen); got[len(got)-1].RRect.Rect.W != 110 {
		t.Errorf("got %+v", got[len(got)-1].RRect.Rect)
	}
}

func TestAnimatedOpacity_Fades(t *testing.T) {
	tree, swap := wtest.SwapTree(t, basic.AnimatedOpacity{Duration: 80 * time.Millisecond, Opacity: 1, Child: basic.Container{Color: wtest.Red}})
	wtest.Ops(tree, wtest.Screen)
	swap(basic.AnimatedOpacity{Duration: 80 * time.Millisecond, Opacity: 0, Child: basic.Container{Color: wtest.Red}})
	wtest.Ops(tree, wtest.Screen)
	now := time.Second
	wtest.Advance(tree, wtest.Screen, &now, 24*time.Millisecond)
	if got := wtest.Ops(tree, wtest.Screen)[0].Color.A; got <= 0 || got >= 1 {
		t.Errorf("got alpha %v", got)
	}
	wtest.Settled(t, tree, wtest.Screen, &now)
	if got := wtest.Ops(tree, wtest.Screen); len(got) != 0 {
		t.Errorf("nothing must paint at 0, got %d commands", len(got))
	}
}

func TestAnimatedAlign_Moves(t *testing.T) {
	child := basic.Container{Width: 10, Height: 10, Color: wtest.Red}
	tree, swap := wtest.SwapTree(t, basic.AnimatedAlign{Duration: 80 * time.Millisecond, Child: child})
	wtest.Ops(tree, wtest.Screen)
	swap(basic.AnimatedAlign{Duration: 80 * time.Millisecond, Alignment: layout.Alignment{X: 1}, Child: child})
	wtest.Ops(tree, wtest.Screen)
	now := time.Second
	wtest.Settled(t, tree, wtest.Screen, &now)
	if got := wtest.Ops(tree, wtest.Screen)[0].RRect.Rect.X; got != wtest.Screen.W-10 {
		t.Errorf("got %v", got)
	}
}

func TestAnimatedPositioned_MovesInAStack(t *testing.T) {
	at := func(left float32) core.Widget {
		return basic.Stack{Fit: layout.StackExpand, Children: []core.Widget{
			basic.AnimatedPositioned{Duration: 80 * time.Millisecond, Left: left, Width: 10, Height: 10, Child: basic.Container{Color: wtest.Red}},
		}}
	}
	tree, swap := wtest.SwapTree(t, at(0))
	wtest.Ops(tree, wtest.Screen)
	swap(at(100))
	wtest.Ops(tree, wtest.Screen)
	now := time.Second
	wtest.Advance(tree, wtest.Screen, &now, 24*time.Millisecond)
	if got := wtest.Ops(tree, wtest.Screen)[0].RRect.Rect.X; got <= 0 || got >= 100 {
		t.Errorf("got %v in the middle", got)
	}
	wtest.Settled(t, tree, wtest.Screen, &now)
	if got := wtest.Ops(tree, wtest.Screen)[0].RRect.Rect.X; got != 100 {
		t.Errorf("got %v", got)
	}
}

func TestAnimatedSwitcher_Crossfades(t *testing.T) {
	one := func(key string, c geom.Color) core.Widget {
		return basic.AnimatedSwitcher{Duration: 80 * time.Millisecond, Child: core.Keyed(key, box(50, 50, c))}
	}
	tree, swap := wtest.SwapTree(t, one("a", wtest.Red))
	if got := len(wtest.Ops(tree, wtest.Screen)); got != 1 {
		t.Fatalf("got %d", got)
	}
	swap(one("a", wtest.Blue))
	wtest.Ops(tree, wtest.Screen)
	if tree.Ticking() {
		t.Fatal("the same child must update in place")
	}
	swap(one("b", wtest.Green))
	wtest.Ops(tree, wtest.Screen)
	now := time.Second
	wtest.Advance(tree, wtest.Screen, &now, 24*time.Millisecond)
	// Each child fades as a group of its own.
	if got := wtest.Ops(tree, wtest.Screen); len(got) != 6 || got[0].Kind != paint.OpGroupPush || got[0].Color.A >= 1 ||
		got[3].Kind != paint.OpGroupPush || got[3].Color.A >= 1 {
		t.Errorf("both must show faded: %+v", got)
	}
	wtest.Settled(t, tree, wtest.Screen, &now)
	got := wtest.Ops(tree, wtest.Screen)
	if len(got) != 1 || got[0].Color != wtest.Green {
		t.Errorf("got %+v", got)
	}
}

// fillOf returns the color of the first box tree paints.
func fillOf(tree *core.Tree) (geom.Color, bool) {
	for _, op := range wtest.Ops(tree, wtest.Screen) {
		if op.Kind == paint.OpRRect {
			return op.Color, true
		}
	}
	return geom.Color{}, false
}

func TestButton_StylesFollowTheTheme(t *testing.T) {
	th := theme.Default().Light
	press := func() {}
	tests := []struct {
		name string
		w    core.Widget
		fill geom.Color
	}{
		{"通常", control.Button{Label: "x", OnPress: press}, th.Color.Control},
		{"主要", control.Button{Label: "x", Style: control.ButtonPrimary, OnPress: press}, th.Color.Accent},
		{"控えめ", control.Button{Label: "x", Style: control.ButtonQuiet, OnPress: press}, geom.Color{}},
		{"破壊的", control.Button{Label: "x", Style: control.ButtonDanger, OnPress: press}, geom.Color{}},
		{"無効", control.Button{Label: "x", Style: control.ButtonPrimary}, th.Color.Fill},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := wtest.TextTree(t, basic.Theme{Data: th, Child: basic.Align{Child: tt.w}})
			got, ok := fillOf(tree)
			if tt.fill.A == 0 {
				if ok && got.A != 0 {
					t.Errorf("got fill %+v", got)
				}
				return
			}
			if got != tt.fill {
				t.Errorf("got %+v, want %+v", got, tt.fill)
			}
		})
	}
}

func TestButton_IsThirtyHigh(t *testing.T) {
	tree := wtest.TextTree(t, basic.Align{Child: control.Button{Label: "x", OnPress: func() {}}})
	for _, op := range wtest.Ops(tree, wtest.Screen) {
		if op.Kind == paint.OpRRect {
			if op.RRect.Rect.H != 30 {
				t.Errorf("got %v", op.RRect.Rect.H)
			}
			return
		}
	}
}

func TestButton_HoverIsInterpolated(t *testing.T) {
	tree := wtest.TextTree(t, basic.Align{Child: control.Button{Label: "OK", Style: control.ButtonPrimary, OnPress: func() {}}})
	rest, _ := fillOf(tree)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 10, Y: 10}})
	wtest.Ops(tree, wtest.Screen)
	if !tree.Ticking() {
		t.Fatal("hover must animate")
	}
	now := time.Second
	wtest.Advance(tree, wtest.Screen, &now, 40*time.Millisecond)
	mid, _ := fillOf(tree)
	wtest.Settled(t, tree, wtest.Screen, &now)
	end, _ := fillOf(tree)
	if !(rest.G < mid.G && mid.G < end.G) {
		t.Errorf("got %v, %v, %v", rest.G, mid.G, end.G)
	}
}

func TestButton_PressSinksTheBoxButNotTheLabel(t *testing.T) {
	tree := wtest.TextTree(t, basic.Align{Child: control.Button{Label: "OK", Style: control.ButtonPrimary, OnPress: func() {}}})
	shape := func() (box geom.Rect, label geom.Offset) {
		for _, op := range wtest.Ops(tree, wtest.Screen) {
			switch op.Kind {
			case paint.OpRRect:
				box = op.RRect.Rect
			case paint.OpGlyphs:
				label = op.Origin
			}
		}
		return box, label
	}
	rest, label := shape()
	pos := geom.Offset{X: 10, Y: 10}
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos})
	now := time.Second
	wtest.Advance(tree, wtest.Screen, &now, 200*time.Millisecond)
	got, gotLabel := shape()
	if want := rest.W * (1 - kit.ButtonSink); got.W > want+0.1 || got.W < want-0.1 {
		t.Errorf("got width %v, want %v", got.W, want)
	}
	if gotLabel != label {
		t.Errorf("the label must stay on its pixels: %+v, was %+v", gotLabel, label)
	}
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos})
	wtest.Settled(t, tree, wtest.Screen, &now)
	if got, _ := shape(); got != rest {
		t.Errorf("got %+v, want %+v", got, rest)
	}
}

func TestSwitch_TogglesAndAnimates(t *testing.T) {
	var value bool
	var swap func(core.Widget)
	var build func() core.Widget
	build = func() core.Widget {
		return basic.Align{Child: control.Switch{Value: value, Label: "wifi", OnChange: func(v bool) {
			value = v
			swap(build())
		}}}
	}
	tree, s := wtest.SwapTree(t, build())
	swap = s
	wtest.Ops(tree, wtest.Screen)
	pos := geom.Offset{X: 10, Y: 10}
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos})
	if !value {
		t.Fatal("a click must turn it on")
	}
	wtest.Ops(tree, wtest.Screen)
	now := time.Second
	var peak float32
	for i := 0; tree.Ticking() && i < 2000; i++ {
		tree.Tick(now)
		list := wtest.Ops(tree, wtest.Screen)
		peak = max(peak, list[len(list)-1].RRect.Rect.X)
		now += 8 * time.Millisecond
	}
	list := wtest.Ops(tree, wtest.Screen)
	knob := list[len(list)-1].RRect.Rect
	if want := float32(kit.SwitchWidth - kit.SwitchEdge - kit.SwitchThumb); knob.X != want || knob.W != kit.SwitchThumb {
		t.Errorf("got %+v, want x=%v", knob, want)
	}
	if peak <= knob.X {
		t.Errorf("the spring must overshoot a little, peak %v", peak)
	}
	if n := tree.Semantics().Children[0]; n.Value != "on" {
		t.Errorf("got %+v", n)
	}
}

func TestSlider_PointerAndKeys(t *testing.T) {
	value := float32(0.5)
	var swap func(core.Widget)
	var build func() core.Widget
	build = func() core.Widget {
		return basic.Align{Child: basic.SizedBox{Width: 218, Child: control.Slider{Value: value, Step: 0.25, Snap: true, OnChange: func(v float32) {
			value = v
			swap(build())
		}}}}
	}
	tree, s := wtest.SwapTree(t, build())
	swap = s
	wtest.Ops(tree, wtest.Screen)
	// The thumb's center travels from 9 to 209.
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: geom.Offset{X: 159, Y: 10}})
	if value != 0.75 {
		t.Errorf("got %v", value)
	}
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 500, Y: 10}})
	if value != 1 {
		t.Errorf("must stop at the end, got %v", value)
	}
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: geom.Offset{X: 500, Y: 10}})
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyLeft})
	if value != 0.75 {
		t.Errorf("got %v", value)
	}
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyHome})
	if value != 0 {
		t.Errorf("got %v", value)
	}
}

func TestSlider_ZeroValueWorks(t *testing.T) {
	tree := wtest.TextTree(t, control.Slider{})
	if got := wtest.Ops(tree, wtest.Screen); len(got) == 0 {
		t.Error("a disabled slider must still show")
	}
}

func TestSegmentedControl_SelectsAndSlides(t *testing.T) {
	selected := 0
	var swap func(core.Widget)
	var build func() core.Widget
	build = func() core.Widget {
		return basic.Align{Child: control.SegmentedControl{Options: []string{"a", "b", "c"}, Selected: selected, OnChange: func(i int) {
			selected = i
			swap(build())
		}}}
	}
	tree, s := wtest.SwapTree(t, build())
	swap = s
	plateX := func() float32 {
		rrects := 0
		for _, op := range wtest.Ops(tree, wtest.Screen) {
			if op.Kind == paint.OpRRect {
				if rrects++; rrects == 2 {
					return op.RRect.Rect.X
				}
			}
		}
		t.Fatal("no plate")
		return 0
	}
	start := plateX()
	// Each of the same-width segments holds a letter and 24 of padding.
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: geom.Offset{X: 75, Y: 8}})
	if selected != 2 {
		t.Fatalf("got %d", selected)
	}
	wtest.Ops(tree, wtest.Screen)
	now := time.Second
	wtest.Advance(tree, wtest.Screen, &now, 24*time.Millisecond)
	mid := plateX()
	wtest.Settled(t, tree, wtest.Screen, &now)
	end := plateX()
	if !(start < mid && mid != end) {
		t.Errorf("got %v, %v, %v", start, mid, end)
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyLeft})
	if selected != 1 {
		t.Errorf("got %d", selected)
	}
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyRight})
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyRight})
	if selected != 2 {
		t.Errorf("must stop at the end, got %d", selected)
	}
}

func TestGroup_LinesStopShortOfTheEdge(t *testing.T) {
	tree := wtest.TextTree(t, Group{Title: "t", Children: []core.Widget{
		GroupRow{Label: "a", Child: box(10, 10, wtest.Red)},
		GroupRow{Label: "b", Child: box(10, 10, wtest.Red)},
	}})
	var line *geom.Rect
	list := wtest.Ops(tree, wtest.Screen)
	for i := range list {
		if r := list[i].RRect.Rect; r.H == 1 {
			line = &r
		}
	}
	if line == nil || line.X != groupInset || line.W != wtest.Screen.W-groupInset {
		t.Errorf("got %+v", line)
	}
}

func TestGroupRow_IsAtLeast44High(t *testing.T) {
	tree := wtest.TextTree(t, basic.Align{Child: basic.SizedBox{Width: 200, Child: basic.Container{Color: wtest.Red, Child: GroupRow{Label: "a"}}}})
	if got := wtest.Ops(tree, wtest.Screen)[0].RRect.Rect.H; got != groupRowHeight {
		t.Errorf("got %v", got)
	}
}

func TestSidebar_SelectionAndPress(t *testing.T) {
	th := theme.Default().Light
	pressed := 0
	tree := wtest.TextTree(t, basic.Row{Cross: layout.CrossStretch, Children: []core.Widget{Sidebar{Children: []core.Widget{
		SidebarItem{Label: "a", Selected: true, OnPress: func() {}},
		SidebarItem{Label: "b", OnPress: func() { pressed++ }},
	}}}})
	var tint bool
	for _, op := range wtest.Ops(tree, wtest.Screen) {
		if op.Color == th.Color.AccentTint {
			tint = true
		}
	}
	if !tint {
		t.Error("the selected row must have the tint")
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	tree.DispatchKey(core.KeyEvent{Key: input.KeyEnter})
	if pressed != 1 {
		t.Errorf("got %d", pressed)
	}
}

func TestScrollBar_AppearsDragsAndGoes(t *testing.T) {
	lines := make([]core.Widget, 40)
	for i := range lines {
		lines[i] = box(100, 50, wtest.Red)
	}
	tree := wtest.TextTree(t, basic.Scroll{Child: basic.Column{Children: lines}})
	view := geom.Size{W: 200, H: 200}
	before := testutil.RenderTree(tree, view)

	// The bar is 12 wide at the right edge.
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 195, Y: 10}})
	now := time.Second
	wtest.Settled(t, tree, view, &now)
	shown := testutil.RenderTree(tree, view)
	if shown == before {
		t.Fatal("the pointer must bring the bar")
	}
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: geom.Offset{X: 195, Y: 5}})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 195, Y: 200}})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: geom.Offset{X: 195, Y: 200}})
	if got := wtest.Ops(tree, view); got[len(got)-1].RRect.Rect.Y+got[len(got)-1].RRect.Rect.H < 199 {
		t.Errorf("the thumb must follow to the end: %+v", got[len(got)-1].RRect.Rect)
	}
	tree.PointerLeft()
	wtest.Settled(t, tree, view, &now)
	if tree.Ticking() {
		t.Fatal("the stay must sleep, not draw")
	}
	wake, ok := tree.NextWake()
	if !ok {
		t.Fatal("the bar must plan to leave")
	}
	now = wake
	tree.Tick(now)
	wtest.Settled(t, tree, view, &now)
	if _, ok := tree.NextWake(); ok {
		t.Error("nothing must remain planned")
	}
}

func TestScrollBar_NotThereWithoutOverflow(t *testing.T) {
	tree := wtest.TextTree(t, basic.Scroll{Child: box(100, 50, wtest.Red)})
	view := geom.Size{W: 200, H: 200}
	before := testutil.RenderTree(tree, view)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 195, Y: 10}})
	if got := testutil.RenderTree(tree, view); got != before {
		t.Errorf("got:\n%s", got)
	}
	if tree.Ticking() {
		t.Error("nothing must animate")
	}
}

// dialogTree is an Overlay with a button, and a way to open a dialog with two.
func dialogTree(t testing.TB) (tree *core.Tree, open func()) {
	t.Helper()
	var ctx core.BuildContext
	tree = wtest.TextTree(t, overlay.Overlay{Child: basic.Align{Child: wtest.Capture{Got: &ctx, Child: control.Button{Label: "open", OnPress: func() {}}}}})
	wtest.Ops(tree, wtest.Screen)
	return tree, func() {
		overlay.ShowDialog(ctx, func(dismiss func()) core.Widget {
			return overlay.Dialog{Title: "t", Message: "m", Actions: []core.Widget{
				control.Button{Label: "no", OnPress: dismiss},
				control.Button{Label: "yes", Style: control.ButtonPrimary, OnPress: dismiss},
			}}
		})
	}
}

// Build implements core.StatelessWidget.

func TestDialog_OpensKeepsTabAndCloses(t *testing.T) {
	tree, open := dialogTree(t)
	idle := testutil.RenderTree(tree, wtest.Screen)

	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	opener := tree.Focused()
	if opener == nil {
		t.Fatal("the button must take focus")
	}
	open()
	wtest.Ops(tree, wtest.Screen)
	now := time.Second
	wtest.Advance(tree, wtest.Screen, &now, 16*time.Millisecond)
	first := wtest.Ops(tree, wtest.Screen)
	var scaled bool
	for i := range first {
		if first[i].Kind == paint.OpGlyphs && first[i].Scale < 1 {
			scaled = true
		}
	}
	if !scaled {
		t.Error("the dialog must start small")
	}
	wtest.Settled(t, tree, wtest.Screen, &now)

	seen := map[core.FocusTarget]bool{}
	for range 6 {
		tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
		wtest.Ops(tree, wtest.Screen)
		seen[tree.Focused()] = true
	}
	if seen[opener] {
		t.Error("Tab must stay in the dialog")
	}
	if len(seen) != 3 {
		t.Errorf("the scope and two buttons must take turns, got %d", len(seen))
	}

	tree.DispatchKey(core.KeyEvent{Key: input.KeyEscape})
	wtest.Settled(t, tree, wtest.Screen, &now)
	if got := testutil.RenderTree(tree, wtest.Screen); strings.Count(got, "\n") >= strings.Count(idle, "\n")+6 {
		t.Errorf("the dialog must be gone:\n%s", got)
	}
	if tree.Focused() != opener {
		t.Error("the focus must return")
	}
}

func TestDialog_ClickBesideCloses(t *testing.T) {
	tree, open := dialogTree(t)
	open()
	wtest.Ops(tree, wtest.Screen)
	now := time.Second
	wtest.Settled(t, tree, wtest.Screen, &now)
	pos := geom.Offset{X: 2, Y: 2}
	inside := geom.Offset{X: 200, Y: 130}
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: inside})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: inside})
	wtest.Ops(tree, wtest.Screen)
	if tree.Ticking() {
		t.Fatal("a click on the dialog must not close it")
	}
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos})
	wtest.Settled(t, tree, wtest.Screen, &now)
	if got := testutil.RenderTree(tree, wtest.Screen); strings.Count(got, "\n") > 6 {
		t.Errorf("the dialog must be gone:\n%s", got)
	}
}

func TestOverlay_InsertAndRemove(t *testing.T) {
	var ctx core.BuildContext
	tree := wtest.TextTree(t, overlay.Overlay{Child: wtest.Capture{Got: &ctx, Child: box(10, 10, wtest.Red)}})
	wtest.Ops(tree, wtest.Screen)
	e := overlay.Of(ctx).Insert(func() core.Widget { return basic.Align{Child: box(20, 20, wtest.Blue)} })
	if got := wtest.Ops(tree, wtest.Screen); len(got) != 2 || got[1].Color != wtest.Blue {
		t.Fatalf("got %+v", got)
	}
	e.Remove()
	e.Remove()
	if got := wtest.Ops(tree, wtest.Screen); len(got) != 1 {
		t.Errorf("got %+v", got)
	}
}

func TestTextField_LabelAboveAndRingOnFocus(t *testing.T) {
	th := theme.Default().Light
	tree := wtest.TextTree(t, basic.Align{Child: basic.SizedBox{Width: 200, Child: field.Input{Label: "name"}}})
	ring := func() bool {
		for _, op := range wtest.Ops(tree, wtest.Screen) {
			if op.BorderWidth == 3 {
				return true
			}
		}
		return false
	}
	if ring() {
		t.Error("no ring without the focus")
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	wtest.Ops(tree, wtest.Screen)
	now := time.Second
	wtest.Settled(t, tree, wtest.Screen, &now)
	if !ring() {
		t.Error("the focus must bring the ring")
	}
	var accented bool
	for _, op := range wtest.Ops(tree, wtest.Screen) {
		if op.BorderWidth == 1 && op.BorderColor == th.Color.AccentInk {
			accented = true
		}
	}
	if !accented {
		t.Error("the line must turn to the accent")
	}
}

func TestTextField_ClickAnywhereFocuses(t *testing.T) {
	tree := wtest.TextTree(t, basic.Align{Child: basic.SizedBox{Width: 200, Child: field.Input{}}})
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: geom.Offset{X: 3, Y: 3}})
	if tree.Focused() == nil {
		t.Error("the padding of the box must count")
	}
}

func TestText_TakesItsLookFromTheTheme(t *testing.T) {
	dark := theme.Default().Dark
	tree := wtest.TextTree(t, basic.Theme{Data: dark, Child: basic.Text{Content: "x"}})
	got := wtest.Ops(tree, wtest.Screen)[0]
	if got.Color != dark.Color.Ink || got.Glyphs.Size != dark.Text.Body.Size {
		t.Errorf("got %+v at size %v", got.Color, got.Glyphs.Size)
	}
}

func TestScale_KeepsRadiiInStep(t *testing.T) {
	tree := wtest.TextTree(t, basic.Scale{Scale: 0.5, Origin: layout.Alignment{X: 0.5, Y: 0.5}, Child: basic.Container{Color: wtest.Red, Radius: 8}})
	got := wtest.Ops(tree, wtest.Screen)[0]
	want := geom.Rect{X: 100, Y: 75, W: 200, H: 150}
	if got.RRect.Rect != want || got.RRect.TopLeft != 4 {
		t.Errorf("got %+v", got.RRect)
	}
}

func BenchmarkAnimatedContainer_100(b *testing.B) {
	boxes := func(c geom.Color) core.Widget {
		children := make([]core.Widget, 100)
		for i := range children {
			children[i] = basic.AnimatedContainer{Duration: time.Hour, Width: 10, Height: 10, Color: c}
		}
		return basic.Column{Children: children}
	}
	tree, swap := wtest.SwapTree(b, boxes(wtest.Red))
	size := geom.Size{W: 400, H: 2000}
	wtest.Ops(tree, size)
	swap(boxes(wtest.Blue))
	wtest.Ops(tree, size)
	b.ReportAllocs()
	var cv paint.Canvas
	var now time.Duration
	// Build makes new widgets on every step by design, so this one allocates;
	// what it guards is the time per frame.
	for b.Loop() {
		now += time.Millisecond
		tree.Tick(now)
		tree.Build()
		tree.Layout(geom.Tight(size))
		cv.Reset()
		tree.Paint(&cv)
	}
}

func TestHover_MovesEveryControl(t *testing.T) {
	press := func() {}
	tests := []struct {
		name string
		w    core.Widget
	}{
		{"Button", control.Button{Label: "x", OnPress: press}},
		{"Checkbox", control.Checkbox{Label: "x", OnChange: func(bool) {}}},
		{"チェック済みの Checkbox", control.Checkbox{Value: true, Label: "x", OnChange: func(bool) {}}},
		{"Switch", control.Switch{OnChange: func(bool) {}}},
		{"オンの Switch", control.Switch{Value: true, OnChange: func(bool) {}}},
		{"Slider", basic.SizedBox{Width: 100, Child: control.Slider{OnChange: func(float32) {}}}},
		{"SegmentedControl", control.SegmentedControl{Options: []string{"a", "b"}, OnChange: func(int) {}}},
		{"SidebarItem", basic.SizedBox{Width: 100, Child: SidebarItem{Label: "x", OnPress: press}}},
		{"field.Input", basic.SizedBox{Width: 100, Child: field.Input{}}},
		{"Radio", control.Radio{Label: "x", OnSelect: press}},
		{"選択済みの Radio", control.Radio{Selected: true, Label: "x", OnSelect: press}},
		{"RadioGroup", control.RadioGroup[int]{Options: []control.RadioOption[int]{{Value: 1, Label: "x"}}, OnChange: func(int) {}}},
		{"Tabs", basic.SizedBox{Width: 200, Height: 100, Child: control.Tabs{Pages: []control.TabPage{{Label: "x"}, {Label: "y"}}, Selected: 1, OnChange: func(int) {}}}},
		{"切り替えの Chip", control.Chip{Label: "x", OnChange: func(bool) {}}},
		{"押す Chip", control.Chip{Label: "x", OnPress: press}},
		{"押せる Card", control.Card{OnPress: press, Child: box(100, 40, wtest.Red)}},
		{"Toolbar", basic.SizedBox{Width: 200, Child: nav.Toolbar{Items: []nav.ToolbarItem{{Action: interact.Action{Label: "x", OnInvoke: press}}}}}},
		{"DatePicker", basic.SizedBox{Width: 200, Child: field.DatePicker{OnChange: func(time.Time) {}}}},
		{"TimePicker", field.TimePicker{OnChange: func(time.Duration) {}}},
		{"field.Search", basic.SizedBox{Width: 200, Child: field.Search{}}},
		{"DropdownMenu", field.DropdownMenu[string]{Options: fruits(), OnChange: func(string) {}}},
		{"Autocomplete", basic.SizedBox{Width: 200, Child: field.Autocomplete[string]{Options: wtest.CityNames()}}},
		{"TabBar", basic.SizedBox{Width: 300, Child: nav.TabBar{Tabs: []nav.TabItem{{Key: 1, Label: "x"}, {Key: 2, Label: "y"}}, Selected: 1, OnSelect: func(int) {}}}},
		{"Breadcrumb", basic.SizedBox{Width: 300, Child: nav.Breadcrumb{Items: []nav.BreadcrumbItem{{Label: "x"}, {Label: "y"}}, OnSelect: func(int) {}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := wtest.TextTree(t, overlay.Overlay{Child: basic.Align{Child: tt.w}})
			rest := testutil.RenderTree(tree, wtest.Screen)
			tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 8, Y: 8}})
			testutil.RenderTree(tree, wtest.Screen)
			if !tree.Ticking() {
				t.Fatal("hover must animate")
			}
			now := time.Second
			wtest.Settled(t, tree, wtest.Screen, &now)
			if testutil.RenderTree(tree, wtest.Screen) == rest {
				t.Error("hover must show")
			}
		})
	}
}

func TestPress_MovesEveryControl(t *testing.T) {
	press := func() {}
	tests := []struct {
		name string
		w    core.Widget
	}{
		{"Button", control.Button{Label: "x", OnPress: press}},
		{"Checkbox", control.Checkbox{Label: "x", OnChange: func(bool) {}}},
		{"Switch", control.Switch{OnChange: func(bool) {}}},
		{"Slider", basic.SizedBox{Width: 100, Child: control.Slider{OnChange: func(float32) {}}}},
		{"SegmentedControl", control.SegmentedControl{Options: []string{"a", "b"}, OnChange: func(int) {}}},
		{"SidebarItem", basic.SizedBox{Width: 100, Child: SidebarItem{Label: "x", OnPress: press}}},
		{"Radio", control.Radio{Label: "x", OnSelect: press}},
		{"RadioGroup", control.RadioGroup[int]{Options: []control.RadioOption[int]{{Value: 1, Label: "x"}}, OnChange: func(int) {}}},
		{"Tabs", basic.SizedBox{Width: 200, Height: 100, Child: control.Tabs{Pages: []control.TabPage{{Label: "x"}, {Label: "y"}}, OnChange: func(int) {}}}},
		{"切り替えの Chip", control.Chip{Label: "x", OnChange: func(bool) {}}},
		{"押す Chip", control.Chip{Label: "x", OnPress: press}},
		{"押せる Card", control.Card{OnPress: press, Child: box(100, 40, wtest.Red)}},
		{"Toolbar", basic.SizedBox{Width: 200, Child: nav.Toolbar{Items: []nav.ToolbarItem{{Action: interact.Action{Label: "x", OnInvoke: press}}}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := wtest.TextTree(t, overlay.Overlay{Child: basic.Align{Child: tt.w}})
			testutil.RenderTree(tree, wtest.Screen)
			pos := geom.Offset{X: 8, Y: 8}
			tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: pos})
			testutil.RenderTree(tree, wtest.Screen)
			now := time.Second
			wtest.Settled(t, tree, wtest.Screen, &now)
			rest := testutil.RenderTree(tree, wtest.Screen)
			tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos})
			testutil.RenderTree(tree, wtest.Screen)
			wtest.Settled(t, tree, wtest.Screen, &now)
			if testutil.RenderTree(tree, wtest.Screen) == rest {
				t.Error("a press must show")
			}
			tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos})
			testutil.RenderTree(tree, wtest.Screen)
			wtest.Settled(t, tree, wtest.Screen, &now)
			if got := testutil.RenderTree(tree, wtest.Screen); got != rest {
				t.Errorf("the press must end:\n%s\n%s", got, rest)
			}
		})
	}
}

func TestDisabled_UsesTokensNotOpacity(t *testing.T) {
	th := theme.Default().Light
	for name, w := range map[string]core.Widget{
		"Switch":           control.Switch{Value: true},
		"SegmentedControl": control.SegmentedControl{Options: []string{"a", "b"}},
		"Radio":            control.Radio{Selected: true, Label: "x"},
		"RadioGroup":       control.RadioGroup[int]{Value: 1, Options: []control.RadioOption[int]{{Value: 1, Label: "x"}}},
		"Tabs":             basic.SizedBox{Width: 200, Height: 100, Child: control.Tabs{Pages: []control.TabPage{{Label: "x"}}}},
		"Toolbar":          basic.SizedBox{Width: 200, Child: nav.Toolbar{Items: []nav.ToolbarItem{{Action: interact.Action{Label: "x"}}}}},
		"DatePicker":       basic.SizedBox{Width: 200, Child: field.DatePicker{Value: time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)}},
		"TimePicker":       field.TimePicker{},
		"Calendar":         field.Calendar{Value: time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)},
		"DropdownMenu":     field.DropdownMenu[string]{Value: "Apple", Options: fruits()},
		"TabBar":           basic.SizedBox{Width: 300, Child: nav.TabBar{Tabs: []nav.TabItem{{Key: 1, Label: "x"}}}},
	} {
		t.Run(name, func(t *testing.T) {
			tree := wtest.TextTree(t, overlay.Overlay{Child: basic.Align{Child: w}})
			var token bool
			for _, op := range wtest.Ops(tree, wtest.Screen) {
				if op.Color == th.Color.Ink3 || op.Color == th.Color.Fill {
					token = true
				}
			}
			if !token {
				t.Error("a disabled control must draw in the disabled colors")
			}
		})
	}
}

func TestControls_ShareOneHeight(t *testing.T) {
	heights := func(name string, w core.Widget) float32 {
		tree := wtest.TextTree(t, overlay.Overlay{Child: basic.Align{Child: basic.SizedBox{Width: 200, Child: w}}})
		for _, op := range wtest.Ops(tree, wtest.Screen) {
			if op.Kind == paint.OpRRect {
				return op.RRect.Rect.H
			}
		}
		t.Errorf("%s painted nothing", name)
		return theme.ControlHeight
	}
	for name, w := range map[string]core.Widget{
		"Button":           basic.Row{Size: layout.MainMin, Children: []core.Widget{control.Button{Label: "x", OnPress: func() {}}}},
		"field.Input":      field.Input{},
		"SegmentedControl": basic.Row{Size: layout.MainMin, Children: []core.Widget{control.SegmentedControl{Options: []string{"a"}, OnChange: func(int) {}}}},
		"SidebarItem":      SidebarItem{Label: "x", Selected: true, OnPress: func() {}},
		"Tabs":             basic.SizedBox{Height: 100, Child: control.Tabs{Pages: []control.TabPage{{Label: "x"}}, OnChange: func(int) {}}},
		"DatePicker":       field.DatePicker{OnChange: func(time.Time) {}},
		"TimePicker":       basic.Row{Size: layout.MainMin, Children: []core.Widget{field.TimePicker{OnChange: func(time.Duration) {}}}},
		"field.Search":     field.Search{},
		"DropdownMenu":     field.DropdownMenu[string]{Options: fruits(), OnChange: func(string) {}},
		"Autocomplete":     field.Autocomplete[string]{Options: wtest.CityNames()},
	} {
		if got := heights(name, w); got != theme.ControlHeight {
			t.Errorf("%s is %v high", name, got)
		}
	}
}

func TestStacking_StaysOnTheGrid(t *testing.T) {
	th := theme.Default().Light
	tree := wtest.TextTree(t, basic.Theme{Data: th, Child: basic.Column{Spacing: theme.Space24, Cross: layout.CrossStretch, Children: []core.Widget{
		basic.Text{Content: "x", Style: th.Text.LargeTitle},
		Group{Title: "t", Children: []core.Widget{
			GroupRow{Label: "a", Detail: "b", Child: box(10, 10, wtest.Red)},
			GroupRow{Label: "c", Child: field.Input{}},
		}},
		control.Button{Label: "x", OnPress: func() {}},
	}}})
	for _, op := range wtest.Ops(tree, geom.Size{W: 400, H: 400}) {
		r := op.RRect.Rect
		if op.Kind == paint.OpRRect && (r.Y != geom.Floor32(r.Y) || r.H != geom.Floor32(r.H)) {
			t.Errorf("off the grid: %+v", r)
		}
	}
}

// fruits is the option list of the dropdown tests in field.
func fruits() []field.DropdownOption[string] {
	return []field.DropdownOption[string]{
		{Value: "apple", Label: "Apple"},
		{Value: "apricot", Label: "Apricot"},
		{Value: "banana", Label: "Banana"},
		{Value: "blueberry", Label: "Blueberry", Disabled: true},
		{Value: "cherry", Label: "Cherry"},
		{Value: "avocado", Label: "Avocado"},
	}
}

// box is a w×h Container of color c.
func box(w, h float32, c geom.Color) core.Widget {
	return basic.Container{Width: w, Height: h, Color: c}
}
