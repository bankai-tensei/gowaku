package wtest

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// Colors and page sizes the tests draw with.
var (
	Red    = geom.Hex(0xff0000)
	Green  = geom.Hex(0x00ff00)
	Blue   = geom.Hex(0x0000ff)
	Page   = geom.Size{W: 200, H: 100}
	Screen = geom.Size{W: 400, H: 300}
)

// Rects returns the rectangle of every command, in paint order.
func Rects(t *testing.T, w core.Widget, size geom.Size) []geom.Rect {
	t.Helper()
	tree := core.NewTree(w)
	defer tree.Dispose()
	tree.Build()
	tree.Layout(geom.Tight(size))
	var cv paint.Canvas
	cv.Reset()
	tree.Paint(&cv)
	ops := cv.DisplayList().Ops
	out := make([]geom.Rect, 0, len(ops))
	for i := range ops {
		out = append(out, ops[i].RRect.Rect)
	}
	return out
}

// NearRect reports whether a and b agree to a thousandth.
func NearRect(a, b geom.Rect) bool {
	return geom.Abs32(a.X-b.X) < 1e-3 && geom.Abs32(a.Y-b.Y) < 1e-3 && geom.Abs32(a.W-b.W) < 1e-3 && geom.Abs32(a.H-b.H) < 1e-3
}

// Ops renders tree and returns its commands, with layers flattened into
// window coordinates.
func Ops(tree *core.Tree, size geom.Size) []paint.Op {
	tree.Build()
	tree.Layout(geom.Tight(size))
	for tree.NeedsBuild() {
		tree.Build()
		tree.Layout(geom.Tight(size))
	}
	var cv paint.Canvas
	cv.Reset()
	tree.Paint(&cv)
	return Flatten(nil, cv.DisplayList(), geom.Offset{})
}

// Flatten appends the commands of dl to out, moving those of nested layers
// by their origins.
func Flatten(out []paint.Op, dl *paint.DisplayList, off geom.Offset) []paint.Op {
	for i := range dl.Ops {
		op := dl.Ops[i]
		if op.Kind == paint.OpLayer {
			out = Flatten(out, op.Layer.DisplayList(), off.Add(op.Origin))
			continue
		}
		op.RRect.Rect = op.RRect.Rect.Translate(off)
		op.Origin = op.Origin.Add(off)
		out = append(out, op)
	}
	return out
}

// Rrects renders tree and returns its rounded rectangle commands.
func Rrects(tree *core.Tree, size geom.Size) []paint.Op {
	var out []paint.Op
	ops := Ops(tree, size)
	for i := range ops {
		if ops[i].Kind == paint.OpRRect {
			out = append(out, ops[i])
		}
	}
	return out
}
