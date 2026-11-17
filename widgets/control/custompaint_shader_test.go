package control

import (
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/basic"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
)

func TestCustomPaint_ShaderFillsTheBoxBeforeThePainter(t *testing.T) {
	s := plainShader()
	painter := layout.PainterFunc(func(cv *paint.Canvas, sz geom.Size) {
		cv.DrawRRect(geom.RRect{Rect: geom.RectFromSize(sz)}, paint.Paint{Color: geom.Hex(0xff0000)})
	})
	dl := paintTree(t, basic.Align{Child: CustomPaint{Shader: s, Params: paint.ShaderParams{3}, Time: time.Second, Width: 40, Height: 30, Painter: painter}})
	if got := kinds(dl); len(got) != 2 || got[0] != paint.OpShader || got[1] != paint.OpRRect {
		t.Fatalf("ops %v, want the shader, then the painter", got)
	}
	call := dl.Shaders[0]
	if call.Shader != s || call.Params[0] != 3 || call.Time != time.Second {
		t.Errorf("call = %+v", call)
	}
	if r := dl.Ops[0].RRect.Rect; r.W != 40 || r.H != 30 {
		t.Errorf("the shader fills %v, want the 40x30 box", r)
	}
}

// An animated shader repaints its own boundary each frame, without a build or
// a layout, and without allocating.
func BenchmarkCustomPaint_AnimatedShaderFrame(b *testing.B) {
	tree := core.NewTree(basic.Align{Child: CustomPaint{Shader: plainShader(), Width: 40, Height: 40, Animate: true}})
	b.Cleanup(tree.Dispose)
	size := geom.Size{W: 100, H: 100}
	cv := &paint.Canvas{}
	tree.Layout(geom.Tight(size))
	cv.Reset()
	tree.Paint(cv)
	now := time.Duration(0)
	frame := func() {
		now += 16 * time.Millisecond
		tree.Tick(now)
		tree.Build()
		tree.Layout(geom.Tight(size))
		tree.Repaint(cv)
	}
	frame()
	b.ReportAllocs()
	for b.Loop() {
		frame()
	}
}

func TestCustomPaint_AnimatedShaderFrameDoesNotAllocate(t *testing.T) {
	if n := testing.Benchmark(BenchmarkCustomPaint_AnimatedShaderFrame).AllocsPerOp(); n != 0 {
		t.Errorf("%d allocations per frame, want 0", n)
	}
}

func plainShader() *paint.Shader { return paint.NewShader(paint.Shader{Name: "plain"}) }

// paintTree lays out w at 100x100 and returns what it records.
func paintTree(t testing.TB, w core.Widget) *paint.DisplayList {
	t.Helper()
	tree := core.NewTree(w)
	t.Cleanup(tree.Dispose)
	tree.Layout(geom.Tight(geom.Size{W: 100, H: 100}))
	cv := &paint.Canvas{}
	cv.Reset()
	tree.Paint(cv)
	return cv.DisplayList()
}

func kinds(dl *paint.DisplayList) []paint.OpKind {
	out := make([]paint.OpKind, len(dl.Ops))
	for i := range dl.Ops {
		out[i] = dl.Ops[i].Kind
	}
	return out
}
