package catalog

import (
	"github.com/bankai-tensei/gowaku/widgets/basic"
	"github.com/bankai-tensei/gowaku/widgets/field"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/theme"
)

func sceneFieldSelectionArea() core.Widget {
	// catalog:begin
	return themed(func(th *theme.Theme) core.Widget {
		return basic.SizedBox{Width: 380, Child: field.SelectionArea{Child: basic.Column{
			Size: layout.MainMin, Cross: layout.CrossStart, Spacing: theme.Space8, Children: []core.Widget{
				basic.Text{Content: "Upload failed", Style: th.Text.Title},
				basic.Text{Content: "dial tcp 192.0.2.10:443: i/o timeout"},
				basic.Text{Content: "Drag, double click or Ctrl+A, then Ctrl+C.", Style: colored(th.Text.Caption, th.Color.Ink2)},
			},
		}}}
	})
	// catalog:end
}

// dragText drags from near the start of the text labeled from to the middle
// of the text labeled to.
func dragText(from, to string) func(*core.Tree, core.BuildContext) {
	return func(tree *core.Tree, _ core.BuildContext) {
		node := func(label string) *semantics.Node {
			n := nodeWhere(tree, func(n *semantics.Node) bool { return n.Label == label })
			if n == nil {
				panic("catalog: no node " + label)
			}
			return n
		}
		a, b := node(from), node(to)
		start := geom.Offset{X: a.Rect.X + a.Rect.W*0.3, Y: a.Rect.Y + a.Rect.H/2}
		end := geom.Offset{X: b.Rect.X + b.Rect.W*0.55, Y: b.Rect.Y + b.Rect.H/2}
		tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: start})
		tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: start})
		tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: end})
		tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: end})
	}
}
