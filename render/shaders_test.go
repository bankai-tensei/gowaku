package render

import (
	"image"
	"strings"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
)

func newShader(name string) *paint.Shader {
	return paint.NewShader(paint.Shader{Name: name, DXBC: []byte(name), SPIRV: []byte("spv" + name)})
}

func TestEncode_ShaderInstance(t *testing.T) {
	f := newFixture(t, "")
	s := newShader("a")
	img := paint.NewImage(image.NewRGBA(image.Rect(0, 0, 8, 4)))
	f.canvas.Reset()
	f.canvas.Translate(10, 20)
	f.canvas.Scale(2)
	f.canvas.PushOpacity(0.5)
	params := paint.ShaderParams{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	f.canvas.DrawShader(geom.Rect{W: 30, H: 40}, paint.ShaderCall{
		Shader: s, Params: params, Time: 1500 * time.Millisecond, Tex0: paint.ShaderTexture{Image: img},
	})
	Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
	if err := f.glyphs.Finish(&f.batch); err != nil {
		t.Fatal(err)
	}
	if len(f.batch.Draws) != 1 || len(f.batch.Instances) != 1 {
		t.Fatalf("%d draws and %d instances, want 1 and 1", len(f.batch.Draws), len(f.batch.Instances))
	}
	d := f.batch.Draws[0]
	if d.Shader == nil || d.Shader.Name != "a" || string(d.Shader.DXBC) != "a" || d.Image == nil || d.Texture != nil {
		t.Errorf("draw = %+v", d)
	}
	in := f.batch.Instances[0]
	if in.Kind != gpu.InstanceShader || in.Rect != (geom.Rect{X: 10, Y: 20, W: 60, H: 80}) || in.Color.A != 0.5 {
		t.Errorf("kind %v, rect %v, alpha %v", in.Kind, in.Rect, in.Color.A)
	}
	if in.BorderWidth != 1.5 || in.Blur != 2 {
		t.Errorf("time %v and zoom %v, want 1.5 and 2", in.BorderWidth, in.Blur)
	}
	if got := in.ShaderParams(); got != [16]float32(params) {
		t.Errorf("params = %v", got)
	}
	want := gpu.ShaderTexture{Place: [4]float32{0, 0, 8, 4}, Texels: [4]float32{0, 0, 8, 4}}
	if got := in.ShaderTexture(0); got != want {
		t.Errorf("tex0 = %+v, want %+v", got, want)
	}
	if got := in.ShaderTexture(1); got != (gpu.ShaderTexture{}) {
		t.Errorf("tex1 = %+v, want none", got)
	}
	if f.store.prepared != 1 {
		t.Errorf("prepared %d times, want 1", f.store.prepared)
	}
}

// A call holds one shader with its textures: another one, or anything else,
// starts a new call, and the same one joins.
func TestEncode_ShadersSplitCalls(t *testing.T) {
	f := newFixture(t, "")
	a, b := newShader("a"), newShader("b")
	f.canvas.Reset()
	r := geom.Rect{W: 10, H: 10}
	f.canvas.DrawShader(r, paint.ShaderCall{Shader: a})
	f.canvas.DrawShader(r.Translate(geom.Offset{X: 20}), paint.ShaderCall{Shader: a})
	f.canvas.DrawShader(r, paint.ShaderCall{Shader: b})
	f.canvas.DrawRRect(geom.RRect{Rect: r}, paint.Paint{Color: geom.Hex(0xff0000)})
	f.canvas.DrawShader(r, paint.ShaderCall{Shader: b})
	Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
	if err := f.glyphs.Finish(&f.batch); err != nil {
		t.Fatal(err)
	}
	var shape []string
	for _, d := range f.batch.Draws {
		name := "sdf"
		if d.Shader != nil {
			name = d.Shader.Name
		}
		shape = append(shape, name+strings.Repeat("+", d.Count-1))
	}
	if got := strings.Join(shape, " "); got != "a+ b sdf b" {
		t.Errorf("calls = %q, want %q", got, "a+ b sdf b")
	}
	if f.store.prepared != 2 {
		t.Errorf("prepared %d times, want once per shader", f.store.prepared)
	}
	if f.batch.Draws[1].Shader != f.batch.Draws[3].Shader {
		t.Error("the calls of one shader do not share its gpu.Shader")
	}
}

// A shader that reads the backdrop gets a call of its own, as beginBackdrop
// makes, which it fills.
func TestEncode_ShaderBlendsReadTheBackdrop(t *testing.T) {
	f := newFixture(t, "")
	f.canvas.Reset()
	f.canvas.DrawRRect(geom.RRect{Rect: geom.Rect{W: 10, H: 10}}, paint.Paint{Color: geom.Hex(0xff0000), Blend: paint.BlendMultiply})
	f.canvas.PushBlend(paint.BlendMultiply)
	f.canvas.DrawShader(geom.Rect{X: 50, W: 10, H: 10}, paint.ShaderCall{Shader: newShader("a")})
	Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
	if len(f.batch.Draws) != 2 {
		t.Fatalf("%d draws, want 2", len(f.batch.Draws))
	}
	d := f.batch.Draws[1]
	if d.Shader == nil || d.Count != 1 || d.Blend != gpu.BlendMultiply || d.Bounds.Size().Empty() {
		t.Errorf("draw = %+v", d)
	}
	if got := f.batch.Instances[1].Kind.Blend(); got != gpu.BlendMultiply {
		t.Errorf("the instance carries %v, want multiply", got)
	}
}

// A layer as a texture goes on a page of shaderLayers, and the shader reads it
// at its place: here the physical pixel x of the surface is texel x - 4 + 1 of
// the page, past the padding.
func TestEncode_ShaderReadsALayer(t *testing.T) {
	f := newFixture(t, "")
	var l paint.Layer
	l.Begin().DrawRRect(geom.RRect{Rect: geom.Rect{X: 2, Y: 3, W: 20, H: 10}}, paint.Paint{Color: geom.Hex(0x00ff00)})
	l.End()
	f.canvas.Reset()
	f.canvas.DrawShader(geom.Rect{X: 10, Y: 10, W: 40, H: 30}, paint.ShaderCall{
		Shader: newShader("a"), Tex0: paint.ShaderTexture{Layer: &l, Origin: geom.Offset{X: 10, Y: 10}},
	})
	Encode(f.canvas.DisplayList(), f.glyphs, 2, &f.batch)
	if err := f.glyphs.Finish(&f.batch); err != nil {
		t.Fatal(err)
	}
	e := f.glyphs.shaderLayers.entries[&l]
	if e == nil || e.page == nil || len(f.glyphs.layers.pages) != 0 {
		t.Fatal("the layer is not on a page of shaderLayers")
	}
	if f.batch.Draws[0].Image != e.page.target || f.store.targetDraws != 1 {
		t.Error("the shader does not read the page, or the page was not drawn")
	}
	x0 := float32(e.slot.Min.X + layerPadding)
	y0 := float32(e.slot.Min.Y + layerPadding)
	got := f.batch.Instances[0].ShaderTexture(0)
	// The layer starts at its origin, 20 physical pixels in, plus e.left.
	px, py := x0+20-(20+float32(e.left)), y0+20-(20+float32(e.top))
	want := gpu.ShaderTexture{
		Place:  [4]float32{px, py, px + 80, py + 60},
		Texels: [4]float32{x0, y0, x0 + float32(e.width), y0 + float32(e.height)},
	}
	if got != want {
		t.Errorf("tex0 = %+v, want %+v", got, want)
	}
}

// A layer that needs multisampling cannot be a texture: it is drawn as it is,
// without the shader.
func TestEncode_ShaderDrawsALayerPlainWhenItMust(t *testing.T) {
	f := newFixture(t, "")
	var l paint.Layer
	lc := l.Begin()
	lc.ClipRRect(geom.RRectAll(geom.Rect{W: 20, H: 20}, 5))
	lc.DrawRRect(geom.RRect{Rect: geom.Rect{W: 20, H: 20}}, paint.Paint{Color: geom.Hex(0x00ff00)})
	l.End()
	f.canvas.Reset()
	f.canvas.DrawShader(geom.Rect{W: 40, H: 30}, paint.ShaderCall{Shader: newShader("a"), Tex0: paint.ShaderTexture{Layer: &l}})
	Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
	for _, d := range f.batch.Draws {
		if d.Shader != nil {
			t.Fatal("the shader is drawn")
		}
	}
	if !f.batch.Multisampled() {
		t.Error("the layer is not drawn in place")
	}
}

// A shader that the backend refuses leaves the frame, and the next ones, and
// DrawError reports it once.
func TestFinish_RefusedShaderLeaves(t *testing.T) {
	f := newFixture(t, "")
	f.store.refuse = true
	s := newShader("broken")
	draw := func() {
		f.canvas.Reset()
		f.canvas.DrawShader(geom.Rect{W: 10, H: 10}, paint.ShaderCall{Shader: s})
		f.canvas.DrawRRect(geom.RRect{Rect: geom.Rect{W: 10, H: 10}}, paint.Paint{Color: geom.Hex(0xff0000)})
		Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
		if err := f.glyphs.Finish(&f.batch); err != nil {
			t.Fatal(err)
		}
	}
	draw()
	if f.batch.Draws[0].Count != 0 || f.batch.Draws[1].Count != 1 {
		t.Errorf("counts %d and %d, want the shader gone and the box kept", f.batch.Draws[0].Count, f.batch.Draws[1].Count)
	}
	err := f.glyphs.DrawError()
	if err == nil || !strings.Contains(err.Error(), `"broken"`) {
		t.Errorf("DrawError() = %v", err)
	}
	draw()
	if len(f.batch.Draws) != 1 || f.batch.Draws[0].Shader != nil {
		t.Error("the refused shader came back")
	}
	if err := f.glyphs.DrawError(); err != nil || f.store.prepared != 1 {
		t.Errorf("DrawError() = %v after %d tries, want nil after 1", err, f.store.prepared)
	}
}

// Without Resources nothing knows the shader, which draws nothing.
func TestEncode_ShaderWithoutResources(t *testing.T) {
	var c paint.Canvas
	c.Reset()
	c.DrawShader(geom.Rect{W: 10, H: 10}, paint.ShaderCall{Shader: newShader("a")})
	var b gpu.Batch
	Encode(c.DisplayList(), nil, 1, &b)
	if len(b.Instances) != 0 {
		t.Error("a shader drew without Resources")
	}
}

// Inline, as a layer that keeps changing is drawn, the instance moves with the
// layer and keeps its data.
func TestAppendMoved_KeepsShaderData(t *testing.T) {
	src := gpu.Batch{Instances: []gpu.Instance{{Rect: geom.Rect{W: 10, H: 10}, Kind: gpu.InstanceShader, Color: geom.Color{A: 1}}}}
	p := [16]float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	src.Instances[0].SetShaderParams(&p)
	src.Instances[0].SetShaderTexture(0, gpu.ShaderTexture{Place: [4]float32{1, 2, 3, 4}, Texels: [4]float32{5, 6, 7, 8}})
	src.Draws = []gpu.DrawCall{{Count: 1}}
	var dst gpu.Batch
	appendMoved(&dst, &src, geom.Offset{X: 5, Y: 7}, geom.Affine{}, geom.Rect{}, false, 0)
	in := dst.Instances[0]
	if in.Rect.X != 5 || in.Rect.Y != 7 || in.ShaderParams() != p || in.Color.A != 1 {
		t.Errorf("rect %v, params %v, alpha %v", in.Rect, in.ShaderParams(), in.Color.A)
	}
	if got := in.ShaderTexture(0); got.Texels[3] != 8 || got.Place[3] != 4 {
		t.Errorf("tex0 = %+v, faded", got)
	}
}

func TestEncode_ShadersDoNotAllocate(t *testing.T) {
	f := newFixture(t, "")
	a, b := newShader("a"), newShader("b")
	img := paint.NewImage(image.NewRGBA(image.Rect(0, 0, 8, 8)))
	var l paint.Layer
	frame := func() {
		// As a ShaderMask does, the layer is recorded anew each frame.
		l.Begin().DrawRRect(geom.RRect{Rect: geom.Rect{W: 20, H: 10}}, paint.Paint{Color: geom.Hex(0x00ff00)})
		l.End()
		f.canvas.Reset()
		for i := range 50 {
			r := geom.Rect{X: float32(i), W: 10, H: 10}
			f.canvas.DrawShader(r, paint.ShaderCall{Shader: a, Tex0: paint.ShaderTexture{Image: img}})
			f.canvas.DrawShader(r, paint.ShaderCall{Shader: b, Tex1: paint.ShaderTexture{Layer: &l}})
		}
		Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
		if err := f.glyphs.Finish(&f.batch); err != nil {
			t.Fatal(err)
		}
	}
	frame()
	if n := testing.AllocsPerRun(20, frame); n != 0 {
		t.Errorf("%v allocations per frame, want 0", n)
	}
}

// BenchmarkEncode_1kShaders encodes 1000 instances of two shaders that take
// turns in pairs, half of them reading an image, and Finish.
func BenchmarkEncode_1kShaders(b *testing.B) {
	f := newFixture(b, "")
	shaders := [2]*paint.Shader{newShader("a"), newShader("b")}
	img := paint.NewImage(image.NewRGBA(image.Rect(0, 0, 8, 8)))
	f.canvas.Reset()
	for i := range 1000 {
		call := paint.ShaderCall{Shader: shaders[i/2%2], Params: paint.ShaderParams{float32(i)}}
		if i%2 == 0 {
			call.Tex0 = paint.ShaderTexture{Image: img}
		}
		f.canvas.DrawShader(geom.Rect{X: float32(i % 40 * 20), Y: float32(i / 40 * 20), W: 18, H: 18}, call)
	}
	dl := f.canvas.DisplayList()
	// The first frame grows the batch and uploads the image.
	Encode(dl, f.glyphs, 1, &f.batch)
	if err := f.glyphs.Finish(&f.batch); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		Encode(dl, f.glyphs, 1, &f.batch)
		if err := f.glyphs.Finish(&f.batch); err != nil {
			b.Fatal(err)
		}
	}
}
