package soft

import (
	"image"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

// shaderBatch fills r with a shader instance of s.
func shaderBatch(s *gpu.Shader, r geom.Rect, params [16]float32) *gpu.Batch {
	in := gpu.Instance{Rect: r, Color: geom.Color{A: 1}, Kind: gpu.InstanceShader, BorderWidth: 2.5}
	in.SetShaderParams(&params)
	return &gpu.Batch{
		Instances: []gpu.Instance{in},
		Draws:     []gpu.DrawCall{{Kind: gpu.DrawQuads, Count: 1, Shader: s}},
	}
}

func TestCustom_RunsTheFallback(t *testing.T) {
	var seen gpu.ShaderInput
	s := &gpu.Shader{Name: "uv", Fallback: func(in *gpu.ShaderInput) geom.Color {
		if in.Pos.X < 1 && in.Pos.Y < 1 {
			seen = *in
		}
		// Red along x, premultiplied.
		return geom.Color{R: in.UV.X, A: 1}
	}}
	img := render(t, frame{width: 20, height: 10, plain: true}, shaderBatch(s, geom.Rect{W: 20, H: 10}, [16]float32{7}))
	if seen.Pos != (geom.Offset{X: 0.5, Y: 0.5}) || seen.Size != (geom.Size{W: 20, H: 10}) || seen.Time != 2.5 ||
		seen.Scale != 1 || seen.Params[0] != 7 {
		t.Errorf("the fallback saw %+v", seen)
	}
	if got := img.NRGBAAt(0, 5).R; got != 6 { // 0.025 of 255, rounded
		t.Errorf("left pixel red = %d, want 6", got)
	}
	if got := img.NRGBAAt(19, 5).R; got != 249 {
		t.Errorf("right pixel red = %d, want 249", got)
	}
}

func TestCustom_WithoutFallbackDrawsNothing(t *testing.T) {
	s := &gpu.Shader{Name: "gpu only"}
	img := render(t, frame{width: 8, height: 8, plain: true}, shaderBatch(s, geom.Rect{W: 8, H: 8}, [16]float32{}))
	if img.NRGBAAt(4, 4) != white {
		t.Errorf("pixel = %v, want the background", img.NRGBAAt(4, 4))
	}
}

// The sampler reads a texel exactly where a pixel lies on it, blends between
// texels, and reads transparent outside the texels of the picture.
func TestShaderSampler(t *testing.T) {
	d := New()
	tex, err := d.CreateTexture(4, 1, gpu.TextureRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	pix := []byte{0, 0, 0, 255, 100, 0, 0, 255, 200, 0, 0, 255, 255, 0, 0, 255}
	if err := d.UpdateTexture(tex, image.Rect(0, 0, 4, 1), pix, 16); err != nil {
		t.Fatal(err)
	}
	// Texels 1 and 2 hold the picture, the others are its neighbors.
	s := shaderSampler{t: tex.(*Texture), at: gpu.ShaderTexture{Place: [4]float32{1, 0, 3, 1}, Texels: [4]float32{1, 0, 3, 1}}}
	near := func(got, want float32) bool { return geom.Abs32(got-want) < 1e-5 }
	if c := s.Sample(geom.Offset{X: 0.25, Y: 0.5}); !near(c.R, 100.0/255) || c.A != 1 {
		t.Errorf("texel 1 = %+v", c)
	}
	if c := s.Sample(geom.Offset{X: 0.5, Y: 0.5}); !near(c.R, 150.0/255) {
		t.Errorf("between texels 1 and 2 = %+v", c)
	}
	if c := s.Sample(geom.Offset{X: 0, Y: 0.5}); !near(c.R, 50.0/255) || !near(c.A, 0.5) {
		t.Errorf("the left edge = %+v, want texel 1 fading into transparent", c)
	}
	if got := s.Size(); got != (geom.Size{W: 2, H: 1}) {
		t.Errorf("Size() = %v", got)
	}
	var none shaderSampler
	if c := none.Sample(geom.Offset{X: 0.5, Y: 0.5}); c != (geom.Color{}) {
		t.Errorf("no texture reads %+v", c)
	}
}

// Turned, the shader sees the point under the pixel center, clamped to the
// rectangle, and its edge is smooth.
func TestCustom_Turned(t *testing.T) {
	s := &gpu.Shader{Name: "solid", Fallback: func(in *gpu.ShaderInput) geom.Color {
		if in.Pos.X < 0 || in.Pos.Y < 0 || in.Pos.X > in.Size.W || in.Pos.Y > in.Size.H {
			return geom.Color{G: 1, A: 1}
		}
		return geom.Color{B: 1, A: 1}
	}}
	b := shaderBatch(s, geom.Rect{X: 5, Y: 5, W: 20, H: 20}, [16]float32{})
	m := geom.Translation(geom.Offset{X: 15, Y: 15}).Mul(geom.Rotation(0.5)).Mul(geom.Translation(geom.Offset{X: -15, Y: -15}))
	b.Instances[0].SetTransform(m)
	img := render(t, frame{width: 30, height: 30, plain: true}, b)
	if img.NRGBAAt(15, 15) != blue {
		t.Errorf("center = %v, want blue", img.NRGBAAt(15, 15))
	}
	partial := 0
	for y := range 30 {
		for x := range 30 {
			c := img.NRGBAAt(x, y)
			if c.G == 255 && c.R != 255 {
				t.Fatalf("pixel (%d, %d) saw a point outside the rectangle", x, y)
			}
			if c.R > 0 && c.R < 255 {
				partial++
			}
		}
	}
	if partial == 0 {
		t.Error("the turned edge is not smoothed")
	}
}
