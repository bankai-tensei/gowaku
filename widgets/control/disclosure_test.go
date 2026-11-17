package control

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

// disclosureContent is the height of the content the tests open.
const disclosureContent = 120

// discBelow marks the place after a disclosure, which moves as it opens.
func discBelow() core.Widget {
	return interact.Semantics{Role: semantics.RoleText, Label: "below", Child: basic.SizedBox{Height: 10}}
}

// discStacked lays out ws at the top of the page, one under the other, and the
// mark after them.
func discStacked(ws ...core.Widget) core.Widget {
	return basic.Column{Size: layout.MainMin, Cross: layout.CrossStretch, Children: append(ws, discBelow())}
}

// discContent is a body of disclosureContent with a button in it.
func discContent(label string, presses *int) core.Widget {
	return basic.SizedBox{Height: disclosureContent, Child: basic.Align{Child: Button{Label: label, OnPress: func() {
		if presses != nil {
			*presses++
		}
	}}}}
}

// discHeader returns the header of the disclosure titled title.
func discHeader(t *testing.T, tree *core.Tree, title string) *semantics.Node {
	t.Helper()
	root, _ := tree.SemanticsTree()
	for _, n := range wtest.RolesOf(root, semantics.RoleButton) {
		if n.Label == title {
			return n
		}
	}
	t.Fatalf("no header %q", title)
	return nil
}

// discBelowY returns where the mark after the disclosures is.
func discBelowY(t *testing.T, tree *core.Tree) float32 {
	t.Helper()
	root, _ := tree.SemanticsTree()
	return wtest.ByLabel(t, root, "below").Rect.Y
}

// hasLabel reports whether a node of the tree has label.
func discHasLabel(tree *core.Tree, label string) bool {
	root, _ := tree.SemanticsTree()
	found := false
	root.Walk(func(n *semantics.Node) bool {
		found = found || n.Label == label
		return !found
	})
	return found
}

func TestDisclosure_OpensAndClosesWithThePointer(t *testing.T) {
	presses := 0
	tree := wtest.TextTree(t, discStacked(Disclosure{Title: "詳細", Child: discContent("適用", &presses)}))
	wtest.Ops(tree, wtest.Screen)
	h := discHeader(t, tree, "詳細")
	if h.State&semantics.Collapsed == 0 || h.State&semantics.Expanded != 0 {
		t.Fatalf("a disclosure starts closed: %+v", h)
	}
	if h.Rect.H != 30 || h.Rect.W != wtest.Screen.W {
		t.Errorf("the discHeader is a control high and takes the width: %+v", h.Rect)
	}
	if y := discBelowY(t, tree); y != 30 {
		t.Errorf("closed, only the header takes room: %v", y)
	}
	if discHasLabel(tree, "適用") {
		t.Error("closed content must be out of the semantics tree")
	}

	now := time.Second
	wtest.Tap(tree, wtest.Center(h.Rect))
	wtest.Settled(t, tree, wtest.Screen, &now)
	h = discHeader(t, tree, "詳細")
	if h.State&semantics.Expanded == 0 {
		t.Fatalf("a click opens: %+v", h)
	}
	want := float32(30 + 4 + disclosureContent + 8)
	if y := discBelowY(t, tree); y != want {
		t.Errorf("open, the content takes its height: got %v, want %v", y, want)
	}
	root, _ := tree.SemanticsTree()
	wtest.Tap(tree, wtest.Center(wtest.ByLabel(t, root, "適用").Rect))
	if presses != 1 {
		t.Errorf("the open content takes the pointer: %d", presses)
	}

	wtest.Tap(tree, wtest.Center(h.Rect))
	wtest.Settled(t, tree, wtest.Screen, &now)
	if h := discHeader(t, tree, "詳細"); h.State&semantics.Collapsed == 0 {
		t.Fatalf("a second click closes: %+v", h)
	}
	if y := discBelowY(t, tree); y != 30 {
		t.Errorf("closed again: %v", y)
	}
}

func TestDisclosure_SlidesOpenOverTime(t *testing.T) {
	tree := wtest.TextTree(t, discStacked(Disclosure{Title: "詳細", Child: discContent("適用", nil)}))
	wtest.Ops(tree, wtest.Screen)
	now := time.Second
	wtest.Tap(tree, wtest.Center(discHeader(t, tree, "詳細").Rect))
	wtest.Advance(tree, wtest.Screen, &now, 80*time.Millisecond)
	mid := discBelowY(t, tree)
	full := float32(30 + 4 + disclosureContent + 8)
	if mid <= 30 || mid >= full {
		t.Fatalf("part way through, the discContent is part open: %v", mid)
	}
	if mid != geom.Floor32(mid) {
		t.Errorf("the height moves by whole pixels: %v", mid)
	}
	// The part still closed is clipped away.
	ops := wtest.Ops(tree, wtest.Screen)
	clipped := false
	for i := range ops {
		clipped = clipped || ops[i].Kind == paint.OpClipPush || ops[i].Clipped
	}
	if !clipped {
		t.Error("an opening body clips its content")
	}
	wtest.Settled(t, tree, wtest.Screen, &now)
	if y := discBelowY(t, tree); y != full {
		t.Errorf("the motion ends open: %v", y)
	}
}

func TestDisclosure_ReduceMotionJumps(t *testing.T) {
	tree := wtest.TextTree(t, basic.MediaQuery{Data: basic.MediaQueryData{Scale: 1, ReduceMotion: true},
		Child: discStacked(Disclosure{Title: "詳細", Child: discContent("適用", nil)})})
	wtest.Ops(tree, wtest.Screen)
	wtest.Tap(tree, wtest.Center(discHeader(t, tree, "詳細").Rect))
	wtest.Ops(tree, wtest.Screen)
	if y := discBelowY(t, tree); y != 30+4+disclosureContent+8 {
		t.Errorf("under ReduceMotion the content opens at once: %v", y)
	}
}

func TestDisclosure_Keys(t *testing.T) {
	tree := wtest.TextTree(t, discStacked(Disclosure{Title: "詳細", Child: discContent("適用", nil)}))
	wtest.Ops(tree, wtest.Screen)
	now := time.Second
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	wtest.Ops(tree, wtest.Screen)
	if got := wtest.FocusedLabel(tree); got != "詳細" {
		t.Fatalf("Tab focuses the header: %q", got)
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyEnter})
	wtest.Settled(t, tree, wtest.Screen, &now)
	if discHeader(t, tree, "詳細").State&semantics.Expanded == 0 {
		t.Fatal("Enter opens")
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeySpace, Repeat: true})
	wtest.Settled(t, tree, wtest.Screen, &now)
	if discHeader(t, tree, "詳細").State&semantics.Expanded == 0 {
		t.Fatal("a repeat does nothing")
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeySpace})
	wtest.Settled(t, tree, wtest.Screen, &now)
	if discHeader(t, tree, "詳細").State&semantics.Collapsed == 0 {
		t.Fatal("Space closes")
	}
}

func TestDisclosure_ShowsTheFocusRingFromTheKeyboard(t *testing.T) {
	tree := wtest.TextTree(t, discStacked(Disclosure{Title: "詳細"}))
	wtest.Ops(tree, wtest.Screen)
	ring := func() bool { return hasRing(tree) }
	if ring() {
		t.Fatal("no ring without the focus")
	}
	now := time.Second
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	wtest.Settled(t, tree, wtest.Screen, &now)
	if !ring() {
		t.Error("the keyboard focus shows a ring")
	}
}

func TestDisclosure_ClosedContentIsSkipped(t *testing.T) {
	presses := 0
	tree := wtest.TextTree(t, discStacked(
		Disclosure{Title: "詳細", Child: discContent("中のボタン", &presses)},
		Button{Label: "次", OnPress: func() {}},
	))
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	wtest.Ops(tree, wtest.Screen)
	if got := wtest.FocusedLabel(tree); got != "次" {
		t.Errorf("Tab passes the closed content by: %q", got)
	}
	// Where the content would be, the pointer reaches the next button.
	wtest.Tap(tree, geom.Offset{X: 200, Y: 40})
	if presses != 0 {
		t.Error("the closed content must not take the pointer")
	}
}

func TestDisclosure_SemanticsActions(t *testing.T) {
	tree := wtest.TextTree(t, discStacked(Disclosure{Title: "詳細", Child: discContent("適用", nil)}))
	wtest.Ops(tree, wtest.Screen)
	h := discHeader(t, tree, "詳細")
	if h.Actions&semantics.ActionExpand == 0 || h.Actions&semantics.ActionCollapse == 0 || h.Actions&semantics.ActionTap == 0 {
		t.Fatalf("the header offers expand, collapse and tap: %+v", h)
	}
	now := time.Second
	if !tree.PerformSemanticsAction(h.ID, semantics.ActionExpand, core.SemanticsArg{}) {
		t.Fatal("expand must be done")
	}
	wtest.Settled(t, tree, wtest.Screen, &now)
	if h := discHeader(t, tree, "詳細"); h.State&semantics.Expanded == 0 {
		t.Fatal("expand opens")
	}
	// Expanding what is open changes nothing.
	tree.PerformSemanticsAction(h.ID, semantics.ActionExpand, core.SemanticsArg{})
	wtest.Settled(t, tree, wtest.Screen, &now)
	if !discHasLabel(tree, "適用") {
		t.Fatal("open discContent is in the semantics tree")
	}
	tree.PerformSemanticsAction(h.ID, semantics.ActionCollapse, core.SemanticsArg{})
	wtest.Settled(t, tree, wtest.Screen, &now)
	if h := discHeader(t, tree, "詳細"); h.State&semantics.Collapsed == 0 {
		t.Fatal("collapse closes")
	}
}

// discAccordion is a row of controlled disclosures of which one shows at a time.
type discAccordion struct {
	tree    *core.Tree
	open    int
	asks    []bool
	reject  bool
	state   *wtest.SwapState
	titles  []string
	presses int
}

func newDiscAccordion(t *testing.T, open int) *discAccordion {
	t.Helper()
	a := &discAccordion{open: open, titles: []string{"一般", "表示", "詳細"}}
	a.tree = wtest.TextTree(t, wtest.Swap{State: &a.state, Build: func() core.Widget {
		ws := make([]core.Widget, len(a.titles))
		for i, title := range a.titles {
			ws[i] = Disclosure{Title: title, Child: discContent(title+"の中", &a.presses), Open: a.open == i,
				OnChange: func(open bool) {
					a.asks = append(a.asks, open)
					if a.reject {
						return
					}
					a.state.SetState(func() {
						a.open = -1
						if open {
							a.open = i
						}
					})
				}}
		}
		return discStacked(ws...)
	}})
	wtest.Ops(a.tree, wtest.Screen)
	return a
}

func (a *discAccordion) expanded(t *testing.T) []bool {
	t.Helper()
	out := make([]bool, len(a.titles))
	for i, title := range a.titles {
		out[i] = discHeader(t, a.tree, title).State&semantics.Expanded != 0
	}
	return out
}

func TestDisclosure_AccordionShowsOneAtATime(t *testing.T) {
	a := newDiscAccordion(t, 0)
	now := time.Second
	if got := a.expanded(t); !got[0] || got[1] || got[2] {
		t.Fatalf("the parent opens the first: %v", got)
	}
	wtest.Tap(a.tree, wtest.Center(discHeader(t, a.tree, "表示").Rect))
	wtest.Settled(t, a.tree, wtest.Screen, &now)
	if got := a.expanded(t); got[0] || !got[1] || got[2] {
		t.Fatalf("opening the second closes the first: %v", got)
	}
	if len(a.asks) != 1 || !a.asks[0] {
		t.Errorf("OnChange asks to open: %v", a.asks)
	}
	// Only the second section takes room.
	want := float32(3*30 + 4 + disclosureContent + 8)
	if y := discBelowY(t, a.tree); y != want {
		t.Errorf("got %v, want %v", y, want)
	}
	wtest.Tap(a.tree, wtest.Center(discHeader(t, a.tree, "表示").Rect))
	wtest.Settled(t, a.tree, wtest.Screen, &now)
	if got := a.expanded(t); got[0] || got[1] || got[2] {
		t.Fatalf("closing the open one leaves all closed: %v", got)
	}
	if y := discBelowY(t, a.tree); y != 3*30 {
		t.Errorf("all closed: %v", y)
	}
}

func TestDisclosure_ControlledStaysUntilTheParentAgrees(t *testing.T) {
	a := newDiscAccordion(t, -1)
	a.reject = true
	now := time.Second
	wtest.Tap(a.tree, wtest.Center(discHeader(t, a.tree, "一般").Rect))
	wtest.Settled(t, a.tree, wtest.Screen, &now)
	if len(a.asks) != 1 || !a.asks[0] {
		t.Fatalf("the press asks: %v", a.asks)
	}
	if got := a.expanded(t); got[0] {
		t.Error("a controlled disclosure shows Open, not the press")
	}
	if y := discBelowY(t, a.tree); y != 3*30 {
		t.Errorf("nothing opened: %v", y)
	}
}

func TestDisclosure_SwitchingMidMotion(t *testing.T) {
	a := newDiscAccordion(t, 0)
	now := time.Second
	wtest.Tap(a.tree, wtest.Center(discHeader(t, a.tree, "表示").Rect))
	wtest.Advance(a.tree, wtest.Screen, &now, 60*time.Millisecond)
	// A third section opens before the second has finished.
	wtest.Tap(a.tree, wtest.Center(discHeader(t, a.tree, "詳細").Rect))
	wtest.Advance(a.tree, wtest.Screen, &now, 16*time.Millisecond)
	if got := a.expanded(t); got[0] || got[1] || !got[2] {
		t.Fatalf("the state follows at once: %v", got)
	}
	wtest.Settled(t, a.tree, wtest.Screen, &now)
	want := float32(3*30 + 4 + disclosureContent + 8)
	if y := discBelowY(t, a.tree); y != want {
		t.Errorf("one section open at the end: got %v, want %v", y, want)
	}
}

func TestDisclosure_UncontrolledFollowsTheParent(t *testing.T) {
	open := false
	title := "詳細"
	var state *wtest.SwapState
	tree := wtest.TextTree(t, wtest.Swap{State: &state, Build: func() core.Widget {
		return discStacked(Disclosure{Title: title, Open: open, Child: discContent("適用", nil)})
	}})
	wtest.Ops(tree, wtest.Screen)
	now := time.Second
	// The user opens it, then the parent rebuilds mid-motion with a new title.
	wtest.Tap(tree, wtest.Center(discHeader(t, tree, "詳細").Rect))
	wtest.Advance(tree, wtest.Screen, &now, 40*time.Millisecond)
	state.SetState(func() { title = "詳細設定" })
	wtest.Settled(t, tree, wtest.Screen, &now)
	if h := discHeader(t, tree, "詳細設定"); h.State&semantics.Expanded == 0 {
		t.Fatal("a rebuild keeps the state the user chose")
	}
	// A parent that changes Open, as an "open all" button does, wins.
	state.SetState(func() { open = true })
	wtest.Ops(tree, wtest.Screen)
	wtest.Settled(t, tree, wtest.Screen, &now)
	state.SetState(func() { open = false })
	wtest.Ops(tree, wtest.Screen)
	wtest.Settled(t, tree, wtest.Screen, &now)
	if h := discHeader(t, tree, "詳細設定"); h.State&semantics.Collapsed == 0 {
		t.Fatal("the parent closes it by changing Open")
	}
}

func TestDisclosure_StartsOpen(t *testing.T) {
	tree := wtest.TextTree(t, discStacked(Disclosure{Title: "詳細", Open: true, Child: discContent("適用", nil)}))
	wtest.Ops(tree, wtest.Screen)
	if tree.Ticking() {
		t.Error("the first frame does not animate")
	}
	if y := discBelowY(t, tree); y != 30+4+disclosureContent+8 {
		t.Errorf("Open starts open: %v", y)
	}
}

func TestDisclosure_ZeroValueAndEmptyContent(t *testing.T) {
	tree := wtest.TextTree(t, discStacked(Disclosure{}))
	wtest.Ops(tree, wtest.Screen)
	root, _ := tree.SemanticsTree()
	buttons := wtest.RolesOf(root, semantics.RoleButton)
	if len(buttons) != 1 || buttons[0].State&semantics.Collapsed == 0 {
		t.Fatalf("the zero value is a closed header: %+v", buttons)
	}
	now := time.Second
	wtest.Tap(tree, wtest.Center(buttons[0].Rect))
	wtest.Settled(t, tree, wtest.Screen, &now)
	// With nothing inside, opening shows only the padding.
	if y := discBelowY(t, tree); y != 30+4+8 {
		t.Errorf("got %v", y)
	}
}

func TestDisclosure_HeaderWidgetKeepsTheTitleForScreenReaders(t *testing.T) {
	tree := wtest.TextTree(t, discStacked(Disclosure{Title: "通知", Header: basic.Row{Size: layout.MainMin, Spacing: 6, Children: []core.Widget{
		basic.Text{Content: "通知"}, Badge{Count: 3},
	}}}))
	wtest.Ops(tree, wtest.Screen)
	if h := discHeader(t, tree, "通知"); h.State&semantics.Collapsed == 0 {
		t.Fatalf("got %+v", h)
	}
}

func TestDisclosure_RightToLeftAndLongTitles(t *testing.T) {
	for _, title := range []string{
		"الإعدادات المتقدمة",
		"とても長い見出しが一行に収まらないときでも見出しの高さは変わらずに省略されるか折り返されるかを確かめる",
	} {
		t.Run(title, func(t *testing.T) {
			tree := wtest.TextTree(t, discStacked(Disclosure{Title: title, Open: true, Child: discContent("適用", nil)}))
			wtest.Ops(tree, wtest.Screen)
			if h := discHeader(t, tree, title); h.Rect.H != 30 || h.State&semantics.Expanded == 0 {
				t.Errorf("got %+v", h)
			}
		})
	}
}

func TestDisclosure_BodyDoesNotAllocate(t *testing.T) {
	tree := wtest.TextTree(t, discStacked(Disclosure{Title: "詳細", Open: true, Child: discContent("適用", nil)}))
	wtest.Ops(tree, wtest.Screen)
	var body *renderDisclosureBody
	var find func(ro core.RenderObject)
	find = func(ro core.RenderObject) {
		if b, ok := ro.(*renderDisclosureBody); ok {
			body = b
		}
		for i := range ro.ChildCount() {
			find(ro.ChildAt(i))
		}
	}
	find(tree.RootRenderObject())
	if body == nil {
		t.Fatal("no body")
	}
	var cv paint.Canvas
	c := geom.Constraints{MinW: 400, MaxW: 400, MaxH: geom.Inf}
	factors := [...]float32{0, 0.5, 1}
	i := 0
	cv.Reset()
	body.Paint(&cv, geom.Offset{})
	allocs := testing.AllocsPerRun(100, func() {
		body.factor = factors[i%len(factors)]
		i++
		body.Layout(c)
		cv.Reset()
		body.Paint(&cv, geom.Offset{})
	})
	if allocs != 0 {
		t.Errorf("a frame of the motion allocates %v times", allocs)
	}
}
