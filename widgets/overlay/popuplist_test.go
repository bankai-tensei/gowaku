package overlay

import (
	"strconv"
	"testing"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
)

func TestPopupList_RevealsTheHighlightedRow(t *testing.T) {
	labels := make([]string, 30)
	for i := range labels {
		labels[i] = "row " + strconv.Itoa(i)
	}
	list := func(hot int) core.Widget {
		return PopupList{Labels: labels, Hot: hot, Checked: -1, Width: 200, OnHover: func(int) {}, OnTap: func(int) {}}
	}
	tree, swap := wtest.SwapTree(t, basic.Align{Alignment: layout.AlignTopLeft, Child: list(25)})
	size := geom.Size{W: 400, H: 600}
	testutil.RenderTree(tree, size)
	visible := func(label string) bool {
		root, _ := tree.SemanticsTree()
		var view, row geom.Rect
		root.Walk(func(n *semantics.Node) bool {
			switch {
			case n.Role == semantics.RoleList:
				view = n.Rect
			case n.Label == label:
				row = n.Rect
			}
			return true
		})
		return row.Y >= view.Y && row.Y+row.H <= view.Y+view.H
	}
	if !visible("row 25") {
		t.Error("the highlighted row must be in view when the list opens")
	}
	if visible("row 0") {
		t.Error("the list must have scrolled")
	}
	swap(basic.Align{Alignment: layout.AlignTopLeft, Child: list(2)})
	testutil.RenderTree(tree, size)
	if !visible("row 2") {
		t.Error("the list must follow the highlight up")
	}
}
