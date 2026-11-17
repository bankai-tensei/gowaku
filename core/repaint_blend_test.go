package core

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

// blendWidget is the widget of a blended RenderRepaintBoundary.
type blendWidget struct {
	mode  paint.BlendMode
	child Widget
}

func (blendWidget) Key() Key { return nil }
func (w blendWidget) CreateRenderObject() RenderObject {
	r := NewRenderRepaintBoundary()
	r.SetBlend(w.mode)
	return r
}
func (w blendWidget) UpdateRenderObject(ro RenderObject) {
	ro.(*RenderRepaintBoundary).SetBlend(w.mode)
}
func (w blendWidget) ChildWidget() Widget { return w.child }

// kinds returns the kinds of ops.
func kinds(ops []paint.Op) []paint.OpKind {
	out := make([]paint.OpKind, len(ops))
	for i := range ops {
		out[i] = ops[i].Kind
	}
	return out
}

// isGroup reports whether ops are a group around one command of kind.
func isGroup(ops []paint.Op, kind paint.OpKind) bool {
	return len(ops) == 3 && ops[0].Kind == paint.OpGroupPush && ops[1].Kind == kind && ops[2].Kind == paint.OpGroupPop
}

func TestRepaintBoundary_BlendDrawsTheLayerAsOnePicture(t *testing.T) {
	leaf := &paintCounter{}
	fade := &transformed{opacity: 0.5}
	tree, cv := settled(t, transformedWidget{fade, blendWidget{paint.BlendMultiply, counterWidget{leaf}}})
	ops := cv.DisplayList().Ops
	if !isGroup(ops, paint.OpLayer) {
		t.Fatalf("got %v, want the layer in a group: a blended group fades as a whole", kinds(ops))
	}
	if ops[0].Blend != paint.BlendMultiply || ops[0].Color.A != 0.5 {
		t.Errorf("group blend %v at opacity %v, want multiply at 0.5", ops[0].Blend, ops[0].Color.A)
	}
	if k := ops[1].Layer.DisplayList().Ops[0]; k.Blend != paint.BlendSrcOver || k.Color.A != 1 {
		t.Errorf("inside the layer: got %v at alpha %v, want the child as it is", k.Blend, k.Color.A)
	}
	leaf.MarkNeedsPaint()
	if tree.Repaint(cv) {
		t.Error("a change below stays in the layer")
	}
}

func TestRepaintBoundary_BlendInsideALayerIsStillOnePicture(t *testing.T) {
	leaf := &paintCounter{}
	_, cv := settled(t, boundaryWidget{blendWidget{paint.BlendScreen, counterWidget{leaf}}})
	inner := cv.DisplayList().Ops[0].Layer.DisplayList().Ops
	if !isGroup(inner, paint.OpRRect) || inner[0].Blend != paint.BlendScreen || inner[1].Blend != paint.BlendSrcOver {
		t.Fatalf("got %v, want the command of the child in a screened group", kinds(inner))
	}
}

func TestRepaintBoundary_SetBlendRepaintsTheParent(t *testing.T) {
	leaf := &paintCounter{}
	tree, cv := settled(t, blendWidget{paint.BlendMultiply, counterWidget{leaf}})
	r := tree.rootRO.(*RenderRepaintBoundary)
	r.SetBlend(paint.BlendScreen)
	if !tree.Repaint(cv) {
		t.Fatal("the mode belongs to the picture that draws the layer")
	}
	if got := cv.DisplayList().Ops[0].Blend; got != paint.BlendScreen {
		t.Errorf("blend %v, want screen", got)
	}
	if leaf.paints != 1 {
		t.Errorf("the child painted %d times, want its layer kept", leaf.paints)
	}
}

func TestRepaintBoundary_OpacityFadesTheLayerAsOnePicture(t *testing.T) {
	leaf := &paintCounter{}
	tree, cv := settled(t, boundaryWidget{counterWidget{leaf}})
	r := tree.rootRO.(*RenderRepaintBoundary)
	for _, a := range []float32{0.5, 0.25} {
		r.SetOpacity(a)
		if !tree.Repaint(cv) {
			t.Fatal("the opacity belongs to the picture that draws the layer")
		}
		ops := cv.DisplayList().Ops
		if !isGroup(ops, paint.OpLayer) || ops[0].Color.A != a {
			t.Fatalf("got %v, want the layer in a group at opacity %v", kinds(ops), a)
		}
	}
	if leaf.paints != 1 {
		t.Errorf("the child painted %d times, want its layer kept through the fade", leaf.paints)
	}
	r.SetOpacity(0)
	tree.Repaint(cv)
	if ops := cv.DisplayList().Ops; len(ops) != 0 {
		t.Errorf("at 0 nothing paints: %v", kinds(ops))
	}
	r.SetOpacity(1)
	tree.Repaint(cv)
	if ops := cv.DisplayList().Ops; len(ops) != 1 || ops[0].Kind != paint.OpLayer {
		t.Errorf("at 1 the layer draws as usual: %v", kinds(ops))
	}
}

func TestRepaintBoundary_SetBlurDrawsABlurredGroup(t *testing.T) {
	leaf := &paintCounter{}
	tree, cv := settled(t, blendWidget{paint.BlendSrcOver, counterWidget{leaf}})
	r := tree.rootRO.(*RenderRepaintBoundary)
	r.SetBlur(4)
	if r.Blur() != 4 || !tree.Repaint(cv) {
		t.Fatal("the blur belongs to the picture that draws the layer")
	}
	dl := cv.DisplayList()
	if !isGroup(dl.Ops, paint.OpLayer) {
		t.Fatalf("got %v, want the layer in a group", kinds(dl.Ops))
	}
	f := dl.Groups[dl.Ops[0].ShaderCall].Style.Filter
	if f == nil || len(f.Primitives) != 1 || f.Primitives[0].Kind != paint.FilterBlur || f.Primitives[0].StdDev.X != 4 {
		t.Fatalf("filter %+v, want a blur of 4", f)
	}
	// The box of 100 by 100 and the layer, which reaches 4 past its shape,
	// grown by three standard deviations and a pixel.
	if got, want := f.Region, (geom.Rect{X: -17, Y: -17, W: 130, H: 130}); got != want {
		t.Errorf("region %v, want %v", got, want)
	}
	if leaf.paints != 1 {
		t.Errorf("the child painted %d times, want its layer kept", leaf.paints)
	}
	r.SetBlur(4)
	if tree.Repaint(cv) {
		t.Error("the same blur repaints")
	}
}

// The group of a faded boundary is cached: its version stays through the fade,
// and changes with the layer.
func TestRepaintBoundary_GroupKeepsItsPictureThroughAFade(t *testing.T) {
	leaf := &paintCounter{}
	tree, cv := settled(t, boundaryWidget{counterWidget{leaf}})
	r := tree.rootRO.(*RenderRepaintBoundary)
	version := func() uint64 {
		dl := cv.DisplayList()
		call := dl.Groups[dl.Ops[0].ShaderCall]
		if call.Style.Cache == nil {
			t.Fatal("the group is not cached")
		}
		return call.Style.Cache.Version()
	}
	r.SetOpacity(0.5)
	tree.Repaint(cv)
	v := version()
	r.SetOpacity(0.25)
	tree.Repaint(cv)
	if got := version(); got != v {
		t.Errorf("fading changed the version from %d to %d", v, got)
	}
	// Only the layer is recorded again; the list that holds the group stays.
	leaf.MarkNeedsPaint()
	if tree.Repaint(cv) {
		t.Fatal("a change below stays in the layer")
	}
	if got := version(); got == v {
		t.Error("a new layer kept the version")
	}
}
