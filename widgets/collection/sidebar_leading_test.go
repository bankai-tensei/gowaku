package collection

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/icons"
	"github.com/bankai-tensei/gowaku/theme"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"
)

// themeInk records the Ink2 of the theme it is built under, which OnTint
// changes.
type themeInk struct {
	core.StatelessBase
	ink *geom.Color
}

func (w themeInk) Build(ctx core.BuildContext) core.Widget {
	*w.ink = basic.ThemeOf(ctx).Color.Ink2
	return basic.SizedBox{Width: sidebarIcon, Height: sidebarIcon}
}

func TestSidebarItem_LeadingDotKeepsItsColorOnTheTint(t *testing.T) {
	red := geom.Color{R: 0.9, G: 0.2, B: 0.2, A: 1}
	dot := func() core.Widget { return basic.Container{Width: 10, Height: 10, Radius: 5, Color: red} }
	var plainInk, chosenInk geom.Color
	tree := wtest.A11yTree(t, sidebarPage(Sidebar{Title: "Labels", Children: []core.Widget{
		SidebarItem{Label: "Work", Leading: dot(), OnPress: func() {}},
		SidebarItem{Label: "Home", Leading: dot(), Selected: true, OnPress: func() {}},
		SidebarItem{Label: "Plain", Leading: themeInk{ink: &plainInk}, OnPress: func() {}},
		SidebarItem{Label: "Chosen", Leading: themeInk{ink: &chosenInk}, Selected: true, OnPress: func() {}},
		SidebarItem{Label: "Iconed", Icon: icons.Star(), OnPress: func() {}},
		SidebarItem{Label: "Both", Icon: icons.Star(), Leading: dot(), OnPress: func() {}},
	}}))
	dots := 0
	for _, op := range wtest.Rrects(tree, wtest.Screen) {
		if op.Color == red && op.RRect.Rect.W == 10 && op.RRect.Rect.H == 10 {
			dots++
		}
	}
	if dots != 3 {
		t.Errorf("got %d red dots, want one on each row with a dot, chosen or not", dots)
	}
	th := theme.Default().Light
	if th.Color.Ink2 == th.OnTint().Color.Ink2 {
		t.Fatal("OnTint keeps Ink2; the probe proves nothing")
	}
	if plainInk != th.Color.Ink2 || chosenInk != th.OnTint().Color.Ink2 {
		t.Errorf("leading built under ink %v and %v, want those of the theme and of its OnTint", plainInk, chosenInk)
	}
	root, _ := tree.SemanticsTree()
	work, iconed := wtest.ByLabel(t, root, "Work"), wtest.ByLabel(t, root, "Iconed")
	if work.Rect.H != theme.ControlHeight || work.Rect.H != iconed.Rect.H {
		t.Errorf("a row with a dot is %g high, one with an icon %g", work.Rect.H, iconed.Rect.H)
	}
	if len(work.Children) != 0 {
		t.Errorf("the dot adds %d nodes to the row", len(work.Children))
	}
}
