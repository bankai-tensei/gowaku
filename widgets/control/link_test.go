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
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/kit"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

// linkHost records the cursor the tree asks for.
type linkHost struct {
	wtest.FakeHost
	cursor input.Cursor
}

// SetCursor implements core.Host.
func (h *linkHost) SetCursor(c input.Cursor) { h.cursor = c }

// linkNode returns the link of the tree, failing the test without one.
func linkNode(t *testing.T, tree *core.Tree) *semantics.Node {
	t.Helper()
	root, _ := tree.SemanticsTree()
	links := wtest.RolesOf(root, semantics.RoleLink)
	if len(links) != 1 {
		t.Fatalf("got %d links", len(links))
	}
	return links[0]
}

// underlines returns the thin lines that the tree paints in color.
func underlines(tree *core.Tree, color geom.Color) []geom.Rect {
	var out []geom.Rect
	ops := wtest.Ops(tree, wtest.Screen)
	for i := range ops {
		if ops[i].Kind == paint.OpRRect && ops[i].RRect.Rect.H <= 2 && ops[i].Color == color {
			out = append(out, ops[i].RRect.Rect)
		}
	}
	return out
}

func TestLink_PressesWithThePointerAndEnter(t *testing.T) {
	presses := 0
	tree := wtest.TextTree(t, basic.Align{Child: Link{Label: "詳しくは", OnPress: func() { presses++ }}})
	wtest.Ops(tree, wtest.Screen)
	n := linkNode(t, tree)
	if n.Label != "詳しくは" || n.Actions&semantics.ActionTap == 0 || n.State&semantics.Disabled != 0 {
		t.Fatalf("a screen reader finds a link: %+v", n)
	}
	wtest.Tap(tree, wtest.Center(n.Rect))
	if presses != 1 {
		t.Fatalf("a click presses: %d", presses)
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	wtest.Ops(tree, wtest.Screen)
	if got := wtest.FocusedLabel(tree); got != "詳しくは" {
		t.Fatalf("Tab focuses the link: %q", got)
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyEnter})
	if presses != 2 {
		t.Errorf("Enter presses: %d", presses)
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyEnter, Repeat: true})
	tree.DispatchKey(core.KeyEvent{Key: input.KeySpace})
	if presses != 2 {
		t.Errorf("neither a repeat nor Space presses a link: %d", presses)
	}
	if !tree.PerformSemanticsAction(n.ID, semantics.ActionTap, core.SemanticsArg{}) || presses != 3 {
		t.Errorf("a screen reader presses it: %d", presses)
	}
}

func TestLink_SpacePassesOn(t *testing.T) {
	outer := 0
	tree := wtest.TextTree(t, keyHandler{onKey: func(*core.KeyEvent) bool { outer++; return true }, child: basic.Align{
		Child: Link{Label: "開く", OnPress: func() {}},
	}})
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	tree.DispatchKey(core.KeyEvent{Key: input.KeySpace})
	if outer != 1 {
		t.Errorf("Space must pass on, to scroll the page: %d", outer)
	}
}

func TestLink_UnderlineAndHandUnderThePointer(t *testing.T) {
	host := &linkHost{}
	tree := wtest.TextTree(t, basic.Align{Child: Link{Label: "開く", OnPress: func() {}}})
	tree.SetHost(host)
	wtest.Ops(tree, wtest.Screen)
	ink := theme.Default().Light.Color.AccentInk
	if got := underlines(tree, ink); len(got) != 0 {
		t.Fatalf("no underline at rest: %v", got)
	}
	n := linkNode(t, tree)
	now := time.Second
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: wtest.Center(n.Rect)})
	wtest.Settled(t, tree, wtest.Screen, &now)
	got := underlines(tree, ink)
	if len(got) != 1 {
		t.Fatalf("the pointer underlines the link: %v", got)
	}
	if u := got[0]; u.W <= 0 || u.W > n.Rect.W+0.5 || u.Y < n.Rect.Y+n.Rect.H/2 || u.Y > n.Rect.Y+n.Rect.H {
		t.Errorf("the line runs under the text: %+v in %+v", u, n.Rect)
	}
	if host.cursor != input.CursorHand {
		t.Errorf("the pointer shows the hand: %v", host.cursor)
	}
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: 390, Y: 290}})
	wtest.Settled(t, tree, wtest.Screen, &now)
	if got := underlines(tree, ink); len(got) != 0 {
		t.Errorf("the underline leaves with the pointer: %v", got)
	}
}

func TestLink_LongLabelUnderlinesEveryLine(t *testing.T) {
	label := "とても長いリンクの文字列は幅に収まらずに折り返されるので、下線もすべての行に引かれる"
	tree := wtest.TextTree(t, basic.Align{Child: basic.SizedBox{Width: 160, Child: Link{Label: label, OnPress: func() {}}}})
	wtest.Ops(tree, wtest.Screen)
	n := linkNode(t, tree)
	now := time.Second
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: geom.Offset{X: n.Rect.X + 4, Y: n.Rect.Y + 4}})
	wtest.Settled(t, tree, wtest.Screen, &now)
	got := underlines(tree, theme.Default().Light.Color.AccentInk)
	if len(got) < 2 {
		t.Fatalf("every line is underlined: %v", got)
	}
	for i := 1; i < len(got); i++ {
		if got[i].Y <= got[i-1].Y {
			t.Errorf("the lines go down: %v", got)
		}
	}
}

func TestLink_RightToLeftUnderline(t *testing.T) {
	tree := wtest.TextTree(t, basic.Align{Child: Link{Label: "اقرأ المزيد", OnPress: func() {}}})
	wtest.Ops(tree, wtest.Screen)
	n := linkNode(t, tree)
	now := time.Second
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: wtest.Center(n.Rect)})
	wtest.Settled(t, tree, wtest.Screen, &now)
	got := underlines(tree, theme.Default().Light.Color.AccentInk)
	if len(got) == 0 {
		t.Fatal("right-to-left text is underlined too")
	}
	for _, r := range got {
		if r.W <= 0 || r.X < n.Rect.X-0.5 || r.X+r.W > n.Rect.X+n.Rect.W+0.5 {
			t.Errorf("the line stays under the text: %+v in %+v", r, n.Rect)
		}
	}
}

func TestLink_Disabled(t *testing.T) {
	host := &linkHost{}
	tree := wtest.TextTree(t, basic.Column{Size: layout.MainMin, Children: []core.Widget{
		Link{Label: "開けない"},
		Button{Label: "次", OnPress: func() {}},
	}})
	tree.SetHost(host)
	wtest.Ops(tree, wtest.Screen)
	n := linkNode(t, tree)
	if n.State&semantics.Disabled == 0 || n.Actions&semantics.ActionTap != 0 {
		t.Fatalf("a link without OnPress is disabled: %+v", n)
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	wtest.Ops(tree, wtest.Screen)
	if got := wtest.FocusedLabel(tree); got != "次" {
		t.Errorf("Tab passes a disabled link by: %q", got)
	}
	now := time.Second
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: wtest.Center(n.Rect)})
	wtest.Settled(t, tree, wtest.Screen, &now)
	if host.cursor == input.CursorHand {
		t.Error("a disabled link shows no hand")
	}
	ink3 := theme.Default().Light.Color.Ink3
	glyphs := 0
	for _, op := range wtest.Ops(tree, wtest.Screen) {
		if op.Kind == paint.OpGlyphs && op.Color == ink3 {
			glyphs++
		}
		if op.Kind == paint.OpRRect && op.RRect.Rect.H <= 2 {
			t.Errorf("a disabled link is never underlined: %+v", op.RRect)
		}
	}
	if glyphs == 0 {
		t.Error("a disabled link is set in Ink3")
	}
}

func TestLink_StyleAndColor(t *testing.T) {
	th := theme.Default().Light
	for _, tt := range []struct {
		name  string
		link  Link
		color geom.Color
		size  float32
	}{
		{name: "既定", link: Link{Label: "a", OnPress: func() {}}, color: th.Color.AccentInk, size: th.Text.Body.Size},
		{name: "小さな字", link: Link{Label: "a", Style: th.Text.Caption, OnPress: func() {}}, color: th.Color.AccentInk, size: th.Text.Caption.Size},
		{name: "色付き", link: Link{Label: "a", Style: kit.Colored(th.Text.Body, th.Color.Danger), OnPress: func() {}}, color: th.Color.Danger, size: th.Text.Body.Size},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tree := wtest.TextTree(t, basic.Align{Child: tt.link})
			found := false
			for _, op := range wtest.Ops(tree, wtest.Screen) {
				if op.Kind == paint.OpGlyphs {
					found = true
					if op.Color != tt.color || op.Glyphs.Size != tt.size {
						t.Errorf("got %v at %v, want %v at %v", op.Color, op.Glyphs.Size, tt.color, tt.size)
					}
				}
			}
			if !found {
				t.Error("no text")
			}
		})
	}
}

func TestLink_RebuildMidPress(t *testing.T) {
	var got []string
	label := "前"
	var state *wtest.SwapState
	tree := wtest.TextTree(t, wtest.Swap{State: &state, Build: func() core.Widget {
		l := label
		return basic.Align{Child: Link{Label: l, OnPress: func() { got = append(got, l) }}}
	}})
	wtest.Ops(tree, wtest.Screen)
	at := wtest.Center(linkNode(t, tree).Rect)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: at})
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: at})
	state.SetState(func() { label = "後" })
	wtest.Ops(tree, wtest.Screen)
	tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: at})
	if len(got) != 1 || got[0] != "後" {
		t.Errorf("the press ends on the link of the moment: %v", got)
	}
}

func TestLink_ZeroValueAndFocusRing(t *testing.T) {
	tree := wtest.TextTree(t, basic.Align{Child: Link{}})
	wtest.Ops(tree, wtest.Screen)
	if n := linkNode(t, tree); n.State&semantics.Disabled == 0 {
		t.Errorf("the zero value is a disabled link: %+v", n)
	}
	tree = wtest.TextTree(t, basic.Align{Child: Link{Label: "開く", OnPress: func() {}}})
	if hasRing(tree) {
		t.Fatal("no ring before the focus")
	}
	tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
	now := time.Second
	wtest.Settled(t, tree, wtest.Screen, &now)
	if !hasRing(tree) {
		t.Error("the keyboard shows the focus ring")
	}
}

func TestLink_UnderlineDoesNotAllocate(t *testing.T) {
	tree := wtest.TextTree(t, basic.Align{Child: Link{Label: "詳しくは", OnPress: func() {}}})
	wtest.Ops(tree, wtest.Screen)
	var u *renderLinkUnderline
	var find func(ro core.RenderObject)
	find = func(ro core.RenderObject) {
		if r, ok := ro.(*renderLinkUnderline); ok {
			u = r
		}
		for i := range ro.ChildCount() {
			find(ro.ChildAt(i))
		}
	}
	find(tree.RootRenderObject())
	if u == nil {
		t.Fatal("no underline")
	}
	u.color = wtest.Blue
	var cv paint.Canvas
	cv.Reset()
	u.Paint(&cv, geom.Offset{})
	allocs := testing.AllocsPerRun(100, func() {
		cv.Reset()
		u.Paint(&cv, geom.Offset{})
	})
	if allocs != 0 {
		t.Errorf("painting the underline allocates %v times", allocs)
	}
}
