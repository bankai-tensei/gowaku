package paint

import (
	"image"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/geom"
)

func testShader() *Shader {
	return NewShader(Shader{Name: "test", DXBC: []byte{1}, SPIRV: []byte{2, 0, 0, 0}, MSL: []byte("m")})
}

func TestNewShader_HashesTheCode(t *testing.T) {
	a, b := testShader(), testShader()
	if a == b || a.ID != b.ID || a.ID == 0 {
		t.Errorf("two shaders of the same code: IDs %x and %x", a.ID, b.ID)
	}
	c := NewShader(Shader{Name: "other", DXBC: []byte{1}, SPIRV: []byte{2, 0, 0, 1}})
	if c.ID == a.ID {
		t.Error("different code has the same ID")
	}
}

func TestCanvas_DrawShaderRecordsTheCall(t *testing.T) {
	c := newCanvas()
	s := testShader()
	img := NewImage(image.NewRGBA(image.Rect(0, 0, 4, 4)))
	c.Translate(10, 20)
	c.PushOpacity(0.5)
	c.PushBlend(BlendMultiply)
	params := ShaderParams{1, 2, 3}
	c.DrawShader(geom.Rect{X: 1, Y: 2, W: 30, H: 40}, ShaderCall{Shader: s, Params: params, Time: time.Second, Tex1: ShaderTexture{Image: img}})
	dl := c.DisplayList()
	if len(dl.Ops) != 1 || len(dl.Shaders) != 1 {
		t.Fatalf("%d ops and %d shader calls, want 1 and 1", len(dl.Ops), len(dl.Shaders))
	}
	op := dl.Ops[0]
	if op.Kind != OpShader || op.Blend != BlendMultiply || op.Color.A != 0.5 || op.ShaderCall != 0 {
		t.Errorf("op = %v, blend %v, alpha %v, call %d", op.Kind, op.Blend, op.Color.A, op.ShaderCall)
	}
	if want := (geom.Rect{X: 11, Y: 22, W: 30, H: 40}); op.RRect.Rect != want {
		t.Errorf("rect = %v, want %v", op.RRect.Rect, want)
	}
	call := dl.Shaders[0]
	if call.Shader != s || call.Params != params || call.Time != time.Second || call.Tex1.Image != img {
		t.Errorf("call = %+v", call)
	}
	c.Reset()
	if len(c.DisplayList().Shaders) != 0 {
		t.Error("Reset kept the shader calls")
	}
}

func TestCanvas_DrawShaderPlacesTheLayer(t *testing.T) {
	c := newCanvas()
	var l Layer
	l.Begin().DrawRRect(unitRect, Paint{Color: opaqueRed})
	l.End()
	c.Translate(5, 6)
	c.DrawShader(geom.Rect{W: 10, H: 10}, ShaderCall{Shader: testShader(), Tex0: ShaderTexture{Layer: &l, Origin: geom.Offset{X: 1, Y: 2}}})
	if got := c.DisplayList().Shaders[0].Tex0.Origin; got != (geom.Offset{X: 6, Y: 8}) {
		t.Errorf("origin = %v, want the place on the surface", got)
	}
}

func TestCanvas_DrawShaderSkipsWhatShowsNothing(t *testing.T) {
	c := newCanvas()
	c.DrawShader(geom.Rect{W: 10, H: 10}, ShaderCall{})
	c.DrawShader(geom.Rect{W: 0, H: 10}, ShaderCall{Shader: testShader()})
	c.Save()
	c.PushOpacity(0)
	c.DrawShader(geom.Rect{W: 10, H: 10}, ShaderCall{Shader: testShader()})
	c.Restore()
	c.ClipRect(geom.Rect{X: 100, Y: 100, W: 10, H: 10})
	c.DrawShader(geom.Rect{W: 10, H: 10}, ShaderCall{Shader: testShader()})
	if n := len(c.DisplayList().Ops); n != 0 {
		t.Errorf("%d ops, want none", n)
	}
}

func TestCanvas_DrawShaderContracts(t *testing.T) {
	var l Layer
	img := NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	for name, draw := range map[string]func(c *Canvas){
		"image and layer": func(c *Canvas) {
			c.DrawShader(geom.Rect{W: 1, H: 1}, ShaderCall{Shader: testShader(), Tex0: ShaderTexture{Image: img, Layer: &l}})
		},
		"layer under a scale": func(c *Canvas) {
			c.Scale(2)
			c.DrawShader(geom.Rect{W: 1, H: 1}, ShaderCall{Shader: testShader(), Tex1: ShaderTexture{Layer: &l}})
		},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: no panic", name)
				}
			}()
			draw(newCanvas())
		}()
	}
}

func TestCanvas_CanShadeLayerInsideALayer(t *testing.T) {
	var outer Layer
	c := outer.Begin()
	if !c.CanShadeLayer() || c.CanDrawLayer() {
		t.Error("a layer can read another layer, though it cannot draw one")
	}
	c.Scale(0.9)
	if c.CanShadeLayer() {
		t.Error("CanShadeLayer under a scale")
	}
}

func BenchmarkCanvas_DrawShader(b *testing.B) {
	c := newCanvas()
	s := testShader()
	call := ShaderCall{Shader: s, Params: ShaderParams{1, 2, 3, 4}}
	b.ReportAllocs()
	for b.Loop() {
		c.Reset()
		for i := range 100 {
			c.DrawShader(geom.Rect{X: float32(i), W: 10, H: 10}, call)
		}
	}
}
