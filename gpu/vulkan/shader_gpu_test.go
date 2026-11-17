//go:build gpu && (linux || windows)

package vulkan

import (
	"image"
	"image/color"
	"testing"

	"github.com/bankai-tensei/gowaku/examples/shader/shaders"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

func shaderInstance(r geom.Rect, params [16]float32, tex0 gpu.ShaderTexture) gpu.Instance {
	in := gpu.Instance{Rect: r, Color: geom.Color{A: 1}, Kind: gpu.InstanceShader}
	in.SetShaderParams(&params)
	in.SetShaderTexture(0, tex0)
	return in
}

// readFrame draws batch on yellow and reads it.
func (f *fixture) readFrame(tg *testWindow, batch *gpu.Batch) *image.NRGBA {
	f.t.Helper()
	if err := f.device.BeginFrame(tg.surface, geom.Hex(0xffff00)); err != nil {
		f.t.Fatal(err)
	}
	if err := f.device.Draw(tg.surface, batch); err != nil {
		f.t.Fatal(err)
	}
	img, err := f.device.ReadPixels(tg.surface)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.device.EndFrame(tg.surface); err != nil {
		f.t.Fatal(err)
	}
	return img
}

func TestShader_DrawsAndKeepsItsPipelines(t *testing.T) {
	f := newFastFixture(t)
	tg := f.open()
	aurora := &gpu.Shader{Name: "aurora", SPIRV: shaders.Aurora.SPIRV}
	crossfade := &gpu.Shader{Name: "crossfade", SPIRV: shaders.Crossfade.SPIRV}
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
		img := f.readFrame(tg, batch)
		near(t, img, "aurora", 35, 35, color.NRGBA{R: 255, A: 255}, 1)
		near(t, img, "crossfade of tex0", 95, 35, color.NRGBA{G: 255, A: 255}, 1)
		near(t, img, "the SDF shader after them", 155, 35, color.NRGBA{B: 255, A: 255}, 0)
		near(t, img, "aurora multiplied", 215, 35, color.NRGBA{R: 255, A: 255}, 1)
		near(t, img, "outside", 300, 200, color.NRGBA{R: 255, G: 255, A: 255}, 0)
	}
	sh := f.device.shaders[aurora]
	if sh.pipelines[passPlain][gpu.BlendSrcOver] == 0 || sh.pipelines[passMulti][gpu.BlendSrcOver] == 0 {
		t.Error("the pipelines of both passes were not made")
	}
	allocs := testing.AllocsPerRun(20, func() { f.frame(tg, batch) })
	if allocs != 0 {
		t.Errorf("a frame with custom shaders allocates %v times", allocs)
	}
	if f.device.shaders[aurora] != sh || len(f.device.shaders) != 2 {
		t.Error("a shader was made again")
	}
}

func TestShader_RefusedIsAnError(t *testing.T) {
	f := newFixture(t)
	tg := f.open()
	for _, s := range []*gpu.Shader{{Name: "empty"}, {Name: "odd", SPIRV: []byte{1, 2, 3}}, {Name: "garbage", SPIRV: make([]byte, 64)}} {
		if err := f.device.PrepareShader(s); err == nil {
			t.Errorf("%s: no error", s.Name)
		}
	}
	unprepared := &gpu.Shader{Name: "unprepared", SPIRV: shaders.Aurora.SPIRV}
	batch := &gpu.Batch{
		Instances: []gpu.Instance{shaderInstance(geom.Rect{W: 50, H: 50}, [16]float32{1, 0, 0, 1, 1, 0, 0, 1}, gpu.ShaderTexture{})},
		Draws:     []gpu.DrawCall{{Count: 1, Shader: unprepared}},
	}
	img := f.readFrame(tg, batch)
	near(t, img, "unprepared", 25, 25, color.NRGBA{R: 255, G: 255, A: 255}, 0)
}

// A target draws with the pipelines of the plain pass, which its pass is
// compatible with.
func TestShader_DrawsToATarget(t *testing.T) {
	f := newFastFixture(t)
	tg := f.open()
	aurora := &gpu.Shader{Name: "aurora", SPIRV: shaders.Aurora.SPIRV}
	if err := f.device.PrepareShader(aurora); err != nil {
		t.Fatal(err)
	}
	target, err := f.device.CreateTarget(64, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Destroy()
	blue := [16]float32{0, 0, 1, 1, 0, 0, 1, 1}
	layer := &gpu.Batch{
		Instances: []gpu.Instance{shaderInstance(geom.Rect{X: 16, Y: 16, W: 32, H: 32}, blue, gpu.ShaderTexture{})},
		Draws:     []gpu.DrawCall{{Count: 1, Shader: aurora}},
	}
	if err := f.device.DrawToTarget(target, layer, 1); err != nil {
		t.Fatal(err)
	}
	frame := &gpu.Batch{
		Instances: []gpu.Instance{{Rect: geom.Rect{W: 64, H: 64}, Color: geom.Color{A: 1}, GradientEnd: geom.Offset{X: 1, Y: 1}, Kind: gpu.InstanceImage}},
		Draws:     []gpu.DrawCall{{Count: 1, Image: target}},
	}
	tg.surface.SetMultisample(false)
	img := f.readFrame(tg, frame)
	near(t, img, "the shader in the target", 32, 32, color.NRGBA{B: 255, A: 255}, 1)
	near(t, img, "around it", 5, 5, color.NRGBA{R: 255, G: 255, A: 255}, 0)
}
