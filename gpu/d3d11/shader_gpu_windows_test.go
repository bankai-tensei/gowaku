//go:build gpu

package d3d11

import (
	"image"
	"image/color"
	"testing"

	"github.com/bankai-tensei/gowaku/examples/shader/shaders"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

// shaderInstance fills r with the color c through aurora, whose two colors are
// then the same, or reads tex0 through crossfade.
func shaderInstance(r geom.Rect, params [16]float32, tex0 gpu.ShaderTexture) gpu.Instance {
	in := gpu.Instance{Rect: r, Color: geom.Color{A: 1}, Kind: gpu.InstanceShader}
	in.SetShaderParams(&params)
	in.SetShaderTexture(0, tex0)
	return in
}

func TestShader_DrawsAndKeepsItsPipeline(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	tg := f.open()
	if err := tg.surface.Resize(320, 240, 1); err != nil {
		t.Fatal(err)
	}
	aurora := &gpu.Shader{Name: "aurora", DXBC: shaders.Aurora.DXBC}
	crossfade := &gpu.Shader{Name: "crossfade", DXBC: shaders.Crossfade.DXBC}
	for _, s := range []*gpu.Shader{aurora, crossfade, aurora} {
		if err := f.device.PrepareShader(s); err != nil {
			t.Fatal(err)
		}
	}
	green, err := f.device.CreateTexture(2, 2, gpu.TextureRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	defer green.Destroy()
	pix := []byte{0, 255, 0, 255, 0, 255, 0, 255, 0, 255, 0, 255, 0, 255, 0, 255}
	if err := f.device.UpdateTexture(green, image.Rect(0, 0, 2, 2), pix, 8); err != nil {
		t.Fatal(err)
	}
	red := [16]float32{1, 0, 0, 1, 1, 0, 0, 1}
	batch := &gpu.Batch{
		Instances: []gpu.Instance{
			shaderInstance(geom.Rect{X: 10, Y: 10, W: 50, H: 50}, red, gpu.ShaderTexture{}),
			shaderInstance(geom.Rect{X: 70, Y: 10, W: 50, H: 50}, [16]float32{}, gpu.ShaderTexture{Place: [4]float32{0, 0, 2, 2}, Texels: [4]float32{0, 0, 2, 2}}),
			{Rect: geom.Rect{X: 130, Y: 10, W: 50, H: 50}, Color: geom.Color{B: 1, A: 1}},
			shaderInstance(geom.Rect{X: 190, Y: 10, W: 50, H: 50}, red, gpu.ShaderTexture{}),
		},
		Draws: []gpu.DrawCall{
			{Count: 1, Shader: aurora},
			{First: 1, Count: 1, Shader: crossfade, Image: green},
			{First: 2, Count: 1},
			{First: 3, Count: 1, Shader: aurora, Blend: gpu.BlendMultiply, Bounds: geom.Rect{X: 189, Y: 9, W: 52, H: 52}},
		},
	}
	batch.Instances[3].Kind = batch.Instances[3].Kind.WithBlend(gpu.BlendMultiply)
	for _, multisample := range []bool{false, true} {
		tg.surface.SetMultisample(multisample)
		if err := f.device.BeginFrame(tg.surface, geom.Hex(0xffff00)); err != nil {
			t.Fatal(err)
		}
		if err := f.device.Draw(tg.surface, batch); err != nil {
			t.Fatal(err)
		}
		img, err := f.device.ReadPixels(tg.surface)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.device.EndFrame(tg.surface); err != nil {
			t.Fatal(err)
		}
		expectNear(t, img, "aurora", 35, 35, color.NRGBA{R: 255, A: 255}, 1)
		expectNear(t, img, "crossfade of tex0", 95, 35, color.NRGBA{G: 255, A: 255}, 1)
		expectNear(t, img, "the SDF shader after them", 155, 35, color.NRGBA{B: 255, A: 255}, 0)
		// Red multiplied with yellow.
		expectNear(t, img, "aurora multiplied", 215, 35, color.NRGBA{R: 255, A: 255}, 1)
		expectNear(t, img, "outside", 300, 200, color.NRGBA{R: 255, G: 255, A: 255}, 0)
	}
	ps := f.device.shaders[aurora]
	allocs := testing.AllocsPerRun(20, func() {
		if err := f.device.BeginFrame(tg.surface, geom.Hex(0xffff00)); err != nil {
			t.Fatal(err)
		}
		if err := f.device.Draw(tg.surface, batch); err != nil {
			t.Fatal(err)
		}
		if err := f.device.EndFrame(tg.surface); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Errorf("a frame with custom shaders allocates %v times", allocs)
	}
	if f.device.shaders[aurora] != ps || len(f.device.shaders) != 2 {
		t.Error("a pixel shader was made again")
	}
}

// What the GPU refuses is an error of PrepareShader, and a call of a shader that
// was not prepared draws nothing, without a panic.
func TestShader_RefusedIsAnError(t *testing.T) {
	f := newFixture(t, gpu.LowPower)
	tg := f.open()
	for _, s := range []*gpu.Shader{{Name: "empty"}, {Name: "garbage", DXBC: []byte("DXBC but not really")}} {
		if err := f.device.PrepareShader(s); err == nil {
			t.Errorf("%s: no error", s.Name)
		}
	}
	unprepared := &gpu.Shader{Name: "unprepared", DXBC: shaders.Aurora.DXBC}
	batch := &gpu.Batch{
		Instances: []gpu.Instance{shaderInstance(geom.Rect{W: 50, H: 50}, [16]float32{1, 0, 0, 1, 1, 0, 0, 1}, gpu.ShaderTexture{})},
		Draws:     []gpu.DrawCall{{Count: 1, Shader: unprepared}},
	}
	f.renderCheck(tg, geom.Hex(0xffffff), batch, 25, 25, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
}
