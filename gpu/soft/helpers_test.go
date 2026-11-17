package soft

import (
	"image"
	"image/color"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

var (
	white = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	black = color.NRGBA{A: 255}
	red   = color.NRGBA{R: 255, A: 255}
	green = color.NRGBA{G: 255, A: 255}
	blue  = color.NRGBA{B: 255, A: 255}
)

// frame says how render draws.
type frame struct {
	width, height int
	// scale defaults to 1.
	scale float32
	// plain turns multisampling off.
	plain bool
}

// render draws batch on a white background and returns the presented frame.
func render(t *testing.T, f frame, batch *gpu.Batch) *image.NRGBA {
	t.Helper()
	d := New()
	defer d.Destroy()
	return renderOn(t, d, f, batch)
}

func renderOn(t *testing.T, d *Device, f frame, batch *gpu.Batch) *image.NRGBA {
	t.Helper()
	if f.scale == 0 {
		f.scale = 1
	}
	s, err := d.CreateSurface(nil, f.width, f.height, f.scale)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Destroy()
	s.SetMultisample(!f.plain)
	if err := d.BeginFrame(s, geom.Hex(0xffffff)); err != nil {
		t.Fatal(err)
	}
	if err := d.Draw(s, batch); err != nil {
		t.Fatal(err)
	}
	if err := d.EndFrame(s); err != nil {
		t.Fatal(err)
	}
	return d.Frame(s)
}

// solid returns a rounded rectangle instance without a border.
func solid(x, y, w, h float32, c geom.Color) gpu.Instance {
	return gpu.Instance{Rect: geom.Rect{X: x, Y: y, W: w, H: h}, Color: c}
}

// cover returns an InstanceCover.
func cover(x, y, w, h float32, c geom.Color) gpu.Instance {
	in := solid(x, y, w, h, c)
	in.Kind = gpu.InstanceCover
	return in
}

func quads(instances ...gpu.Instance) *gpu.Batch {
	b := &gpu.Batch{}
	for i := range instances {
		addQuad(b, instances[i], 0)
	}
	return b
}

func addCall(b *gpu.Batch, in gpu.Instance, call gpu.DrawCall) {
	call.First, call.Count = len(b.Instances), 1
	b.Instances = append(b.Instances, in)
	b.Draws = append(b.Draws, call)
}

func addShape(b *gpu.Batch, in gpu.Instance, vertices []geom.Offset, call gpu.DrawCall) {
	call.VertexFirst, call.VertexCount = len(b.Vertices), len(vertices)
	b.Vertices = append(b.Vertices, vertices...)
	addCall(b, in, call)
}

// addQuad draws in at a clip level.
func addQuad(b *gpu.Batch, in gpu.Instance, level uint8) {
	addCall(b, in, gpu.DrawCall{Kind: gpu.DrawQuads, ClipLevel: level})
}

// addFill fills triangles with the color of in, which must be an InstanceCover.
func addFill(b *gpu.Batch, in gpu.Instance, triangles []geom.Offset, evenOdd bool, level uint8) {
	addShape(b, in, triangles, gpu.DrawCall{Kind: gpu.DrawFill, EvenOdd: evenOdd, ClipLevel: level})
}

// addPush narrows the clip from level to triangles, inside bounds.
func addPush(b *gpu.Batch, bounds geom.Rect, triangles []geom.Offset, level uint8) {
	in := gpu.Instance{Rect: bounds, Kind: gpu.InstanceCover}
	addShape(b, in, triangles, gpu.DrawCall{Kind: gpu.DrawClipPush, ClipLevel: level})
}

// addPop returns the samples inside bounds to level.
func addPop(b *gpu.Batch, bounds geom.Rect, level uint8) {
	addCall(b, gpu.Instance{Rect: bounds, Kind: gpu.InstanceCover}, gpu.DrawCall{Kind: gpu.DrawClipPop, ClipLevel: level})
}

// fanOf splits a polygon into a triangle fan around its first point, as a stencil
// fill may: winding numbers come out right even if the polygon is not convex.
func fanOf(points ...geom.Offset) []geom.Offset {
	var fan []geom.Offset
	for i := 1; i+1 < len(points); i++ {
		fan = append(fan, points[0], points[i], points[i+1])
	}
	return fan
}

func checkPixel(t *testing.T, img *image.NRGBA, x, y int, want color.NRGBA) {
	t.Helper()
	if got := img.NRGBAAt(x, y); got != want {
		t.Errorf("pixel (%d, %d): got %v, want %v", x, y, got, want)
	}
}

// checkNear allows each channel to be off by tolerance.
func checkNear(t *testing.T, img *image.NRGBA, x, y int, want color.NRGBA, tolerance int) {
	t.Helper()
	got := img.NRGBAAt(x, y)
	if distance(got.R, want.R) > tolerance || distance(got.G, want.G) > tolerance ||
		distance(got.B, want.B) > tolerance || got.A != want.A {
		t.Errorf("pixel (%d, %d): got %v, want %v within %d", x, y, got, want, tolerance)
	}
}

func distance(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }

// mix is what a resolve gives if n samples of 4 have fg and the others bg.
func mix(bg, fg color.NRGBA, n int) color.NRGBA {
	channel := func(b, f uint8) uint8 { return uint8((int(b)*(4-n) + int(f)*n + 1) / 4) }
	return color.NRGBA{R: channel(bg.R, fg.R), G: channel(bg.G, fg.G), B: channel(bg.B, fg.B), A: 255}
}

// over mixes fg over bg with an alpha, in float64 and rounded once.
func over(bg, fg color.NRGBA, alpha float64) color.NRGBA {
	channel := func(b, f uint8) uint8 { return uint8(float64(f)*alpha + float64(b)*(1-alpha) + 0.5) }
	return color.NRGBA{R: channel(bg.R, fg.R), G: channel(bg.G, fg.G), B: channel(bg.B, fg.B), A: 255}
}
