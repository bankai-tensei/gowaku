//go:build gpu && darwin && arm64

package metal

import (
	"image"
	"image/color"
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/platform"
)

// newTestSurface makes a device and a surface on a CAMetalLayer of its own,
// which vends drawables without a window.
func newTestSurface(t *testing.T, width, height int) (*Device, *Surface) {
	t.Helper()
	d, err := New(Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	layer := objc.ID(objc.GetClass("CAMetalLayer")).Send(objc.RegisterName("new"))
	s, err := d.CreateSurface(platform.CocoaHandle{Layer: uintptr(layer)}, width, height, 1)
	layer.Send(objc.RegisterName("release"))
	if err != nil {
		d.Destroy()
		t.Fatalf("CreateSurface: %v", err)
	}
	t.Cleanup(func() {
		s.Destroy()
		d.Destroy()
	})
	return d, s.(*Surface)
}

func pixel(img *image.NRGBA, x, y int) color.NRGBA {
	return img.NRGBAAt(x, y)
}

// drawFrame draws batch on s with the given multisampling and returns the pixels.
func drawFrame(t *testing.T, d *Device, s *Surface, batch *gpu.Batch, multisample bool) *image.NRGBA {
	t.Helper()
	s.SetMultisample(multisample)
	if err := d.BeginFrame(s, geom.Color{R: 1, G: 1, B: 1, A: 1}); err != nil {
		t.Fatalf("BeginFrame: %v", err)
	}
	if err := d.Draw(s, batch); err != nil {
		t.Fatalf("Draw: %v", err)
	}
	img, err := d.ReadPixels(s)
	if err != nil {
		t.Fatalf("ReadPixels: %v", err)
	}
	if err := d.EndFrame(s); err != nil {
		t.Fatalf("EndFrame: %v", err)
	}
	return img
}

func TestDevice_ClearAndRect(t *testing.T) {
	d, s := newTestSurface(t, 64, 64)
	red := geom.Color{R: 1, A: 1}
	batch := &gpu.Batch{
		Instances: []gpu.Instance{{Rect: geom.Rect{X: 8, Y: 8, W: 16, H: 16}, Color: red}},
		Draws:     []gpu.DrawCall{{Kind: gpu.DrawQuads, Count: 1}},
	}
	for _, multisample := range []bool{false, true} {
		img := drawFrame(t, d, s, batch, multisample)
		if got := pixel(img, 2, 2); got != (color.NRGBA{255, 255, 255, 255}) {
			t.Errorf("multisample %v: background got %v, want white", multisample, got)
		}
		if got := pixel(img, 16, 16); got != (color.NRGBA{255, 0, 0, 255}) {
			t.Errorf("multisample %v: rect got %v, want red", multisample, got)
		}
		if got := pixel(img, 30, 30); got != (color.NRGBA{255, 255, 255, 255}) {
			t.Errorf("multisample %v: outside got %v, want white", multisample, got)
		}
	}
}

func TestDevice_PathFillAndClip(t *testing.T) {
	d, s := newTestSurface(t, 64, 64)
	blue := geom.Color{B: 1, A: 1}
	// A triangle filled through the stencil, then a clip to the left half
	// around a rect that covers everything.
	tri := []geom.Offset{{X: 0, Y: 0}, {X: 64, Y: 0}, {X: 0, Y: 64}}
	half := []geom.Offset{{X: 0, Y: 0}, {X: 32, Y: 0}, {X: 0, Y: 64}, {X: 32, Y: 0}, {X: 32, Y: 64}, {X: 0, Y: 64}}
	all := geom.Rect{W: 64, H: 64}
	batch := &gpu.Batch{
		Instances: []gpu.Instance{
			{Rect: all, Color: blue, Kind: gpu.InstanceCover},
		},
		Vertices: tri,
		Draws:    []gpu.DrawCall{{Kind: gpu.DrawFill, Count: 1, VertexCount: 3}},
	}
	img := drawFrame(t, d, s, batch, true)
	if got := pixel(img, 8, 8); got != (color.NRGBA{0, 0, 255, 255}) {
		t.Errorf("inside the triangle got %v, want blue", got)
	}
	if got := pixel(img, 56, 56); got != (color.NRGBA{255, 255, 255, 255}) {
		t.Errorf("outside the triangle got %v, want white", got)
	}

	green := geom.Color{G: 1, A: 1}
	batch = &gpu.Batch{
		Instances: []gpu.Instance{
			{Rect: all, Kind: gpu.InstanceCover},
			{Rect: all, Color: green},
			{Rect: all, Kind: gpu.InstanceCover},
		},
		Vertices: half,
		Draws: []gpu.DrawCall{
			{Kind: gpu.DrawClipPush, First: 0, Count: 1, VertexCount: 6},
			{Kind: gpu.DrawQuads, First: 1, Count: 1, ClipLevel: 1},
			{Kind: gpu.DrawClipPop, First: 2, Count: 1, ClipLevel: 1},
		},
	}
	img = drawFrame(t, d, s, batch, true)
	if got := pixel(img, 8, 32); got != (color.NRGBA{0, 255, 0, 255}) {
		t.Errorf("inside the clip got %v, want green", got)
	}
	if got := pixel(img, 48, 32); got != (color.NRGBA{255, 255, 255, 255}) {
		t.Errorf("outside the clip got %v, want white", got)
	}
}

func TestDevice_TextureAndTarget(t *testing.T) {
	d, s := newTestSurface(t, 32, 32)
	tex, err := d.CreateTexture(2, 2, gpu.TextureRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	defer tex.Destroy()
	pix := []byte{255, 0, 0, 255, 255, 0, 0, 255, 255, 0, 0, 255, 255, 0, 0, 255}
	if err := d.UpdateTexture(tex, image.Rect(0, 0, 2, 2), pix, 8); err != nil {
		t.Fatal(err)
	}
	target, err := d.CreateTarget(32, 32)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Destroy()
	img := gpu.Instance{Rect: geom.Rect{W: 16, H: 16}, Color: geom.Color{A: 1}, Kind: gpu.InstanceImage,
		GradientEnd: geom.Offset{X: 1, Y: 1}}
	img.SetImageFilter(gpu.FilterNearest)
	layer := &gpu.Batch{
		Instances: []gpu.Instance{img},
		Draws:     []gpu.DrawCall{{Kind: gpu.DrawQuads, Count: 1, Image: tex}},
	}
	if err := d.DrawToTarget(target, layer, 1); err != nil {
		t.Fatal(err)
	}
	show := gpu.Instance{Rect: geom.Rect{W: 32, H: 32}, Color: geom.Color{A: 1}, Kind: gpu.InstanceImage,
		GradientEnd: geom.Offset{X: 1, Y: 1}}
	show.SetImageFilter(gpu.FilterNearest)
	got := drawFrame(t, d, s, &gpu.Batch{
		Instances: []gpu.Instance{show},
		Draws:     []gpu.DrawCall{{Kind: gpu.DrawQuads, Count: 1, Image: target}},
	}, false)
	if c := pixel(got, 4, 4); c != (color.NRGBA{255, 0, 0, 255}) {
		t.Errorf("layer got %v, want red", c)
	}
	if c := pixel(got, 24, 24); c != (color.NRGBA{255, 255, 255, 255}) {
		t.Errorf("transparent part of the layer got %v, want white", c)
	}
}
