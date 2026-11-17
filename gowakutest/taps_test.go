package gowakutest_test

import (
	"slices"
	"testing"

	"github.com/bankai-tensei/gowaku"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gowakutest"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/collection"
	"github.com/bankai-tensei/gowaku/widgets/interact"
	"github.com/bankai-tensei/gowaku/widgets/overlay"
)

func TestSecondaryTap_OpensAContextMenu(t *testing.T) {
	deleted := 0
	tt := gowakutest.New(t, basic.Center{Child: overlay.ContextMenu{
		Items: []overlay.MenuItem{{Action: interact.Action{Label: "Delete", OnInvoke: func() { deleted++ }}}},
		Child: interact.Semantics{Role: semantics.RoleGroup, Label: "Photo", Child: basic.SizedBox{Width: 120, Height: 80}},
	}}, goFonts)
	tt.Tap("Photo")
	if _, ok := tt.Find("Delete"); ok {
		t.Fatal("a primary tap opened the context menu")
	}
	tt.SecondaryTap("Photo")
	if _, ok := tt.Find("Delete"); !ok {
		t.Fatalf("no context menu after a secondary tap:\n%s", tt.Dump())
	}
	tt.Tap("Delete")
	if deleted != 1 {
		t.Errorf("Delete ran %d times, want once", deleted)
	}
}

// folders is a Tree of three rows that records what it is asked.
type folders struct {
	gowaku.StatelessBase
	opened, menus *[]string
}

func (f folders) Build(gowaku.BuildContext) gowaku.Widget {
	return basic.SizedBox{Width: 300, Height: 200, Child: collection.Tree[string]{
		Roots:         []string{"Music", "Pictures", "Videos"},
		ChildrenOf:    func(string) []string { return nil },
		Label:         "Files",
		OnActivate:    func(k string) { *f.opened = append(*f.opened, k) },
		OnContextMenu: func(k string, _ geom.Offset) { *f.menus = append(*f.menus, k) },
	}}
}

func TestSecondaryTapAndDoubleTap_OnATree(t *testing.T) {
	var opened, menus []string
	tt := gowakutest.New(t, folders{opened: &opened, menus: &menus}, goFonts)

	tt.SecondaryTap("Pictures")
	if !slices.Equal(menus, []string{"Pictures"}) {
		t.Errorf("OnContextMenu got %v, want Pictures", menus)
	}
	if n, ok := tt.Find("Pictures"); !ok || n.State&semantics.Selected == 0 {
		t.Errorf("a secondary tap must select the row:\n%s", tt.Dump())
	}

	// Two taps in a row are two clicks, not a double click.
	tt.Tap("Music")
	tt.Tap("Music")
	if len(opened) != 0 {
		t.Errorf("two taps activated %v, want nothing", opened)
	}
	tt.DoubleTap("Videos")
	if !slices.Equal(opened, []string{"Videos"}) {
		t.Errorf("OnActivate got %v, want Videos", opened)
	}
	n, _ := tt.Find("Music")
	tt.DoubleTapAt(geom.Offset{X: n.Rect.X + n.Rect.W/2, Y: n.Rect.Y + n.Rect.H/2})
	tt.SecondaryTapAt(geom.Offset{X: n.Rect.X + n.Rect.W/2, Y: n.Rect.Y + n.Rect.H/2})
	if !slices.Equal(opened, []string{"Videos", "Music"}) || !slices.Equal(menus, []string{"Pictures", "Music"}) {
		t.Errorf("at a point: activated %v and asked menus for %v", opened, menus)
	}
}
