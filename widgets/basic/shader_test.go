package basic

import (
	"testing"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

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

func TestShaderMask_ReadsItsChildFromALayer(t *testing.T) {
	s := plainShader()
	child := Container{Width: 30, Height: 20, Color: geom.Hex(0x00ff00)}
	dl := paintTree(t, Align{Child: ShaderMask{Shader: s, Child: child}})
	if got := kinds(dl); len(got) != 1 || got[0] != paint.OpShader {
		t.Fatalf("ops %v, want the shader alone", got)
	}
	tex := dl.Shaders[0].Tex0
	if tex.Layer == nil || tex.Layer.Empty() || tex.Origin != (geom.Offset{X: dl.Ops[0].RRect.Rect.X, Y: dl.Ops[0].RRect.Rect.Y}) {
		t.Errorf("tex0 = %+v, want the layer of the child at the place of the mask", tex)
	}
}

// Under a scale transition a layer cannot follow, so the child shows as it is.
func TestShaderMask_UnderAScaleDrawsTheChild(t *testing.T) {
	child := Container{Width: 30, Height: 20, Color: geom.Hex(0x00ff00)}
	dl := paintTree(t, Scale{Scale: 0.9, Child: Align{Child: ShaderMask{Shader: plainShader(), Child: child}}})
	if got := kinds(dl); len(got) != 1 || got[0] != paint.OpRRect {
		t.Errorf("ops %v, want the child alone", got)
	}
}

func TestShaderMask_WithoutShaderDrawsTheChild(t *testing.T) {
	dl := paintTree(t, Align{Child: ShaderMask{Child: Container{Width: 30, Height: 20, Color: geom.Hex(0x00ff00)}}})
	if got := kinds(dl); len(got) != 1 || got[0] != paint.OpRRect {
		t.Errorf("ops %v, want the child alone", got)
	}
}
