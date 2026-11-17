package catalog

import (
	"image"
	"image/color"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/input"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
	"github.com/bankai-tensei/gowaku/text"
	"github.com/bankai-tensei/gowaku/theme"
)

// The shapes of the scenes that stand for no icon of package icons, on the 24
// unit grid of Icon.

func squareIcon() *paint.Path {
	var p paint.Path
	p.AddRRect(geom.RRectAll(geom.Rect{X: 5, Y: 5, W: 14, H: 14}, 3))
	return &p
}

func circleIcon() *paint.Path {
	var p paint.Path
	p.AddRRect(geom.RRectAll(geom.Rect{X: 4, Y: 4, W: 16, H: 16}, 8))
	return &p
}

func triangleIcon() *paint.Path {
	var p paint.Path
	p.MoveTo(12, 4)
	p.LineTo(20, 19)
	p.LineTo(4, 19)
	p.Close()
	return &p
}

// photo returns a small picture for Image: a sky over hills, drawn in code.
func photo() *paint.Image {
	const w, h = 160, 100
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			t := float32(y) / h
			c := color.NRGBA{R: uint8(120 + 80*t), G: uint8(170 + 50*t), B: 235, A: 255}
			hill := 62 + 10*geom.Sin32(float32(x)/18)
			far := 50 + 8*geom.Sin32(float32(x)/30+1)
			switch {
			case float32(y) > hill:
				c = color.NRGBA{R: 64, G: 132, B: 90, A: 255}
			case float32(y) > far:
				c = color.NRGBA{R: 112, G: 160, B: 128, A: 255}
			}
			dx, dy := float32(x-118), float32(y-26)
			if dx*dx+dy*dy < 120 {
				c = color.NRGBA{R: 255, G: 236, B: 170, A: 255}
			}
			m.SetNRGBA(x, y, c)
		}
	}
	return paint.NewImage(m)
}

// themed builds content with the theme of the window, as an application reads
// it with ThemeOf.
func themed(content func(th *theme.Theme) core.Widget) core.Widget {
	return basic.Builder{Content: func(ctx core.BuildContext) core.Widget { return content(basic.ThemeOf(ctx)) }}
}

// colored returns s in color c.
func colored(s text.Style, c geom.Color) text.Style {
	s.Color = c
	return s
}

// tile is a quiet box with a caption, for the layout scenes.
func tile(label string, w, h float32) core.Widget {
	return themed(func(th *theme.Theme) core.Widget {
		return basic.Container{
			Width: w, Height: h, Radius: th.Radius.Control, Color: th.Color.AccentTint,
			Child: basic.Center{Child: basic.Text{Content: label, Style: colored(th.Text.Caption, th.Color.AccentInk)}},
		}
	})
}

// nodeWhere returns the first node of the accessibility tree that ok accepts.
func nodeWhere(tree *core.Tree, ok func(n *semantics.Node) bool) *semantics.Node {
	var at *semantics.Node
	root, _ := tree.SemanticsTree()
	root.Walk(func(n *semantics.Node) bool {
		if ok(n) {
			at = n
			return false
		}
		return true
	})
	return at
}

// click presses and releases button at the middle of the first node of role r
// labeled label, or of any label when it is empty, below pixels lower.
func click(role semantics.Role, label string, below float32, button input.Button) func(*core.Tree, core.BuildContext) {
	return func(tree *core.Tree, _ core.BuildContext) {
		n := nodeWhere(tree, func(n *semantics.Node) bool {
			return (role == 0 || n.Role == role) && (label == "" || n.Label == label)
		})
		if n == nil {
			panic("catalog: no node " + label)
		}
		pos := geom.Offset{X: n.Rect.X + n.Rect.W/2, Y: n.Rect.Y + n.Rect.H/2 + below}
		tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: pos})
		tree.DispatchPointer(core.PointerEvent{Kind: core.PointerDown, Pos: pos, Button: button})
		tree.DispatchPointer(core.PointerEvent{Kind: core.PointerUp, Pos: pos, Button: button})
	}
}

// steps runs acts in turn, building and laying out the tree in a window of
// size between them, so that each finds what the one before it opened.
func steps(size geom.Size, acts ...func(*core.Tree, core.BuildContext)) func(*core.Tree, core.BuildContext) {
	return func(tree *core.Tree, ctx core.BuildContext) {
		for i, act := range acts {
			if i > 0 {
				tree.Build()
				tree.Layout(geom.Tight(size))
				for tree.NeedsBuild() {
					tree.Build()
					tree.Layout(geom.Tight(size))
				}
			}
			act(tree, ctx)
		}
	}
}

// hover rests the pointer on the middle of the first node labeled label.
func hover(label string) func(*core.Tree, core.BuildContext) {
	return func(tree *core.Tree, _ core.BuildContext) {
		n := nodeWhere(tree, func(n *semantics.Node) bool { return n.Label == label })
		if n == nil {
			panic("catalog: no node " + label)
		}
		pos := geom.Offset{X: n.Rect.X + n.Rect.W/2, Y: n.Rect.Y + n.Rect.H/2}
		tree.DispatchPointer(core.PointerEvent{Kind: core.PointerMove, Pos: pos})
	}
}

// tabs presses Tab n times, to show the focus ring.
func tabs(n int) func(*core.Tree, core.BuildContext) {
	return func(tree *core.Tree, _ core.BuildContext) {
		for range n {
			tree.DispatchKey(core.KeyEvent{Key: input.KeyTab})
		}
	}
}

// focus gives the keyboard to the node of role r labeled label, and types s.
func focus(role semantics.Role, label, s string) func(*core.Tree, core.BuildContext) {
	return func(tree *core.Tree, _ core.BuildContext) {
		n := nodeWhere(tree, func(n *semantics.Node) bool { return n.Role == role && n.Label == label })
		if n == nil {
			panic("catalog: no node " + label)
		}
		tree.PerformSemanticsAction(n.ID, semantics.ActionFocus, core.SemanticsArg{})
		if s != "" {
			tree.DispatchText(s)
		}
	}
}

// expand opens the node of role r labeled label, as a screen reader does.
func expand(role semantics.Role, label string) func(*core.Tree, core.BuildContext) {
	return func(tree *core.Tree, _ core.BuildContext) {
		n := nodeWhere(tree, func(n *semantics.Node) bool { return n.Role == role && n.Label == label })
		if n == nil {
			panic("catalog: no node " + label)
		}
		tree.PerformSemanticsAction(n.ID, semantics.ActionExpand, core.SemanticsArg{})
	}
}
