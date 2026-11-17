package render

import (
	"image"
	"image/color"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/gpu/soft"
	"github.com/bankai-tensei/gowaku/internal/testutil"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/text"
)

// softFrame draws what draw records on the CPU backend, on white, and returns
// the frame.
func softFrame(t testing.TB, width, height int, scale float32, draw func(cv *paint.Canvas)) *image.NRGBA {
	t.Helper()
	device := soft.New()
	t.Cleanup(device.Destroy)
	res := NewResources(text.NewAtlas(testutil.NewShaper(t)), device)
	t.Cleanup(res.Destroy)
	surface, err := device.CreateSurface(nil, width, height, scale)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(surface.Destroy)
	var cv paint.Canvas
	var b gpu.Batch
	cv.Reset()
	draw(&cv)
	Encode(cv.DisplayList(), res, scale, &b)
	if err := res.Finish(&b); err != nil {
		t.Fatal(err)
	}
	if err := res.DrawError(); err != nil {
		t.Fatal(err)
	}
	if err := device.BeginFrame(surface, geom.Color{R: 1, G: 1, B: 1, A: 1}); err != nil {
		t.Fatal(err)
	}
	if err := device.Draw(surface, &b); err != nil {
		t.Fatal(err)
	}
	img, err := device.ReadPixels(surface)
	if err != nil {
		t.Fatal(err)
	}
	if err := device.EndFrame(surface); err != nil {
		t.Fatal(err)
	}
	return img
}

func near(c color.NRGBA, r, g, b uint8, tolerance int) bool {
	d := func(x, y uint8) bool { return int(x)-int(y) <= tolerance && int(y)-int(x) <= tolerance }
	return d(c.R, r) && d(c.G, g) && d(c.B, b)
}

func rect(x, y, w, h float32) geom.RRect { return geom.RRect{Rect: geom.Rect{X: x, Y: y, W: w, H: h}} }

// An opacity over a group fades the group as one picture: where its shapes
// overlap, the one below does not show through.
func TestGroup_OpacityIsolates(t *testing.T) {
	img := softFrame(t, 100, 50, 1, func(cv *paint.Canvas) {
		cv.Save()
		cv.PushOpacity(0.5)
		cv.PushGroup(paint.GroupStyle{})
		cv.DrawRRect(rect(10, 10, 40, 30), paint.Paint{Color: geom.Color{R: 1, A: 1}})
		cv.DrawRRect(rect(30, 10, 40, 30), paint.Paint{Color: geom.Color{B: 1, A: 1}})
		cv.PopGroup()
		cv.Restore()
	})
	// Blue at half opacity over white, in the overlap as alone.
	for _, x := range []int{40, 60} {
		if c := img.NRGBAAt(x, 25); !near(c, 128, 128, 255, 1) {
			t.Errorf("at %d: %v, want half blue", x, c)
		}
	}
	if c := img.NRGBAAt(20, 25); !near(c, 255, 128, 128, 1) {
		t.Errorf("red part: %v", c)
	}
}

// A mask multiplies the group by its alpha or by its luminance.
func TestGroup_Masks(t *testing.T) {
	img := softFrame(t, 120, 40, 1, func(cv *paint.Canvas) {
		cv.PushGroup(paint.GroupStyle{Mask: paint.MaskAlpha})
		cv.DrawRRect(rect(0, 0, 60, 40), paint.Paint{Color: geom.Color{A: 1}})
		cv.BeginMask()
		cv.DrawRRect(rect(10, 10, 20, 20), paint.Paint{Color: geom.Color{R: 1, A: 0.5}})
		cv.PopGroup()
		cv.PushGroup(paint.GroupStyle{Mask: paint.MaskLuminance})
		cv.DrawRRect(rect(60, 0, 60, 40), paint.Paint{Color: geom.Color{A: 1}})
		cv.BeginMask()
		cv.DrawRRect(rect(70, 10, 20, 20), paint.Paint{Color: geom.Color{R: 1, G: 1, B: 1, A: 1}})
		cv.DrawRRect(rect(90, 10, 20, 20), paint.Paint{Color: geom.Color{G: 1, A: 1}})
		cv.PopGroup()
	})
	checks := []struct {
		x, y    int
		r, g, b uint8
	}{
		{5, 5, 255, 255, 255}, // outside the alpha mask
		{20, 20, 128, 128, 128},
		{80, 20, 0, 0, 0},     // white shows all
		{100, 20, 73, 73, 73}, // green shows 0.7154
		{65, 5, 255, 255, 255},
	}
	for _, c := range checks {
		if got := img.NRGBAAt(c.x, c.y); !near(got, c.r, c.g, c.b, 2) {
			t.Errorf("at %d,%d: %v, want %d %d %d", c.x, c.y, got, c.r, c.g, c.b)
		}
	}
}

// Groups nest, and a filter runs over what its group draws.
func TestGroup_NestedFilter(t *testing.T) {
	filter := &paint.Filter{
		Region: geom.Rect{X: 0, Y: 0, W: 100, H: 60},
		Primitives: []paint.FilterPrimitive{
			{Kind: paint.FilterOffset, Offset: geom.Offset{X: 20}},
			{Kind: paint.FilterFlood, Color: geom.Color{G: 1, A: 1}, Subregion: geom.Rect{X: 0, Y: 0, W: 10, H: 10}},
			{Kind: paint.FilterMerge, Inputs: []paint.FilterInput{paint.FilterResult(1), paint.FilterResult(0)}},
		},
	}
	img := softFrame(t, 100, 60, 1, func(cv *paint.Canvas) {
		cv.PushGroup(paint.GroupStyle{Filter: filter})
		cv.Save()
		cv.PushOpacity(0.5)
		cv.PushGroup(paint.GroupStyle{})
		cv.DrawRRect(rect(10, 20, 30, 20), paint.Paint{Color: geom.Color{R: 1, A: 1}})
		cv.DrawRRect(rect(10, 20, 30, 20), paint.Paint{Color: geom.Color{R: 1, A: 1}})
		cv.PopGroup()
		cv.Restore()
		cv.PopGroup()
	})
	if c := img.NRGBAAt(15, 30); !near(c, 255, 255, 255, 1) {
		t.Errorf("the offset left %v behind", c)
	}
	if c := img.NRGBAAt(45, 30); !near(c, 255, 128, 128, 2) {
		t.Errorf("moved: %v, want half red once", c)
	}
	if c := img.NRGBAAt(5, 5); !near(c, 0, 255, 0, 1) {
		t.Errorf("flood: %v", c)
	}
}

// A group inside a layer reaches the page, and one inside a group still
// composites as one picture.
func TestGroup_InsideLayer(t *testing.T) {
	var l paint.Layer
	cv := l.Begin()
	cv.Save()
	cv.PushOpacity(0.5)
	cv.PushGroup(paint.GroupStyle{})
	cv.DrawRRect(rect(0, 0, 40, 30), paint.Paint{Color: geom.Color{R: 1, A: 1}})
	cv.DrawRRect(rect(20, 0, 40, 30), paint.Paint{Color: geom.Color{B: 1, A: 1}})
	cv.PopGroup()
	cv.Restore()
	l.End()
	img := softFrame(t, 100, 50, 1, func(cv *paint.Canvas) {
		cv.DrawLayer(&l, geom.Offset{X: 10, Y: 10})
	})
	if c := img.NRGBAAt(40, 25); !near(c, 128, 128, 255, 1) {
		t.Errorf("overlap: %v, want half blue", c)
	}
}

// Encoding a frame of groups again allocates nothing.
func TestGroup_EncodeDoesNotAllocate(t *testing.T) {
	f := newFixture(t, "a")
	filter := &paint.Filter{
		Region: geom.Rect{W: 100, H: 100},
		Primitives: []paint.FilterPrimitive{
			{Kind: paint.FilterBlur, StdDev: geom.Offset{X: 3, Y: 3}},
			{Kind: paint.FilterDropShadow, StdDev: geom.Offset{X: 1, Y: 1}, Offset: geom.Offset{X: 2, Y: 2}, Color: geom.Color{A: 1}},
			{Kind: paint.FilterComponentTransfer, Transfer: [4]paint.TransferFunc{{Kind: paint.TransferGamma, Amplitude: 1, Exponent: 2}}},
			{Kind: paint.FilterMerge, Inputs: []paint.FilterInput{paint.FilterSourceGraphic, paint.FilterResult(1), paint.FilterResult(2)}},
		},
	}
	var path paint.Path
	path.MoveTo(10, 10)
	path.LineTo(90, 20)
	path.LineTo(40, 90)
	path.Close()
	frame := func() {
		f.canvas.Reset()
		cv := &f.canvas
		cv.Save()
		cv.PushOpacity(0.8)
		cv.PushGroup(paint.GroupStyle{Mask: paint.MaskLuminance, Filter: filter})
		cv.DrawPath(&path, paint.Paint{Color: geom.Color{R: 1, A: 1}})
		cv.PushGroup(paint.GroupStyle{Blend: paint.BlendMultiply})
		cv.DrawRRect(rect(20, 20, 50, 50), paint.Paint{Color: geom.Color{G: 1, A: 1}})
		cv.PopGroup()
		cv.BeginMask()
		cv.DrawRRect(rect(0, 0, 60, 60), paint.Paint{Color: geom.Color{R: 1, G: 1, B: 1, A: 1}})
		cv.PopGroup()
		cv.Restore()
		Encode(f.canvas.DisplayList(), f.glyphs, 1.5, &f.batch)
		if err := f.glyphs.Finish(&f.batch); err != nil {
			t.Fatal(err)
		}
	}
	for range 3 {
		frame()
	}
	if n := testing.AllocsPerRun(20, frame); n != 0 {
		t.Errorf("%v allocations per frame", n)
	}
}

// A group larger than maxGroupSize draws at a lower resolution, whose content
// is encoded again at that scale; the first encoding leaves no jobs behind.
func TestGroup_LargerThanATarget(t *testing.T) {
	f := newFixture(t, "a")
	f.canvas.Reset()
	f.canvas.PushGroup(paint.GroupStyle{})
	f.canvas.DrawRRect(rect(0, 0, 3000, 100), paint.Paint{Color: geom.Color{R: 1, A: 1}})
	f.canvas.PushGroup(paint.GroupStyle{})
	// Two shapes, which a group of one would draw without a picture.
	f.canvas.DrawRRect(rect(10, 10, 20, 20), paint.Paint{Color: geom.Color{B: 1, A: 1}})
	f.canvas.DrawRRect(rect(20, 20, 20, 20), paint.Paint{Color: geom.Color{G: 1, A: 1}})
	f.canvas.PopGroup()
	f.canvas.PopGroup()
	Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
	if n := len(f.glyphs.jobs); n != 2 {
		t.Errorf("%d jobs, want one per group", n)
	}
	for _, j := range f.glyphs.jobs {
		if j.scale >= 1 {
			t.Errorf("job at scale %v, want below 1", j.scale)
		}
	}
	in := f.batch.Instances[len(f.batch.Instances)-1]
	if in.Kind != gpu.InstanceImage || geom.Abs32(in.Rect.W-3000) > 4 {
		t.Errorf("picture %+v, want 3000 wide", in.Rect)
	}
}

// A filter can move what lies outside the clip of its group inside it, so the
// content of a filtered group is not cut to that clip.
func TestGroup_FilterReadsPastTheClip(t *testing.T) {
	filter := &paint.Filter{
		Region:     geom.Rect{X: -100, Y: 0, W: 200, H: 50},
		Primitives: []paint.FilterPrimitive{{Kind: paint.FilterOffset, Offset: geom.Offset{X: 40}}},
	}
	img := softFrame(t, 100, 50, 1, func(cv *paint.Canvas) {
		cv.ClipRect(geom.Rect{X: 0, Y: 0, W: 100, H: 50})
		cv.PushGroup(paint.GroupStyle{Filter: filter})
		cv.DrawRRect(rect(-30, 10, 50, 30), paint.Paint{Color: geom.Color{R: 1, A: 1}})
		cv.PopGroup()
	})
	if c := img.NRGBAAt(15, 25); !near(c, 255, 0, 0, 1) {
		t.Errorf("moved in from outside the clip: %v", c)
	}
	if c := img.NRGBAAt(5, 25); !near(c, 255, 255, 255, 1) {
		t.Errorf("left behind: %v", c)
	}
}

// BenchmarkEncode_Groups encodes a frame of a group with a mask and a filter of
// blurs, a drop shadow and a merge, which become jobs for the GPU. It must not
// allocate.
func BenchmarkEncode_Groups(b *testing.B) {
	f := newFixture(b, "a")
	filter := &paint.Filter{
		Region: geom.Rect{W: 200, H: 200},
		Primitives: []paint.FilterPrimitive{
			{Kind: paint.FilterBlur, StdDev: geom.Offset{X: 4, Y: 4}},
			{Kind: paint.FilterDropShadow, StdDev: geom.Offset{X: 2, Y: 2}, Offset: geom.Offset{X: 3, Y: 3}, Color: geom.Color{A: 1}},
			{Kind: paint.FilterMerge, Inputs: []paint.FilterInput{paint.FilterResult(0), paint.FilterResult(1)}},
		},
	}
	b.ReportAllocs()
	for b.Loop() {
		f.canvas.Reset()
		cv := &f.canvas
		cv.PushGroup(paint.GroupStyle{Mask: paint.MaskLuminance, Filter: filter})
		cv.DrawRRect(rect(20, 20, 150, 150), paint.Paint{Color: geom.Color{R: 1, A: 1}})
		cv.BeginMask()
		cv.DrawRRect(rect(0, 0, 200, 200), paint.Paint{Color: geom.Color{R: 1, G: 1, B: 1, A: 1}})
		cv.PopGroup()
		Encode(f.canvas.DisplayList(), f.glyphs, 1.5, &f.batch)
		if err := f.glyphs.Finish(&f.batch); err != nil {
			b.Fatal(err)
		}
	}
}

// A group of one shape draws the shape faded, without a picture; two shapes,
// which overlap, need one.
func TestGroup_OfOneShapeNeedsNoPicture(t *testing.T) {
	f := newFixture(t, "a")
	f.canvas.Reset()
	f.canvas.Save()
	f.canvas.PushOpacity(0.5)
	f.canvas.PushGroup(paint.GroupStyle{})
	f.canvas.DrawRRect(rect(10, 10, 20, 20), paint.Paint{Color: geom.Color{B: 1, A: 1}, Stroke: &paint.Stroke{Width: 2, Color: geom.Color{A: 0.8}}})
	f.canvas.PopGroup()
	f.canvas.Restore()
	Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
	if n := len(f.glyphs.jobs); n != 0 {
		t.Errorf("%d jobs, want none", n)
	}
	if len(f.batch.Instances) != 1 {
		t.Fatalf("%d instances, want the shape", len(f.batch.Instances))
	}
	if in := f.batch.Instances[0]; in.Kind != gpu.InstanceRRect || in.Color.A != 0.5 || in.BorderColor.A != 0.4 {
		t.Errorf("instance %+v, want the shape at half opacity", in)
	}
	f.canvas.Reset()
	f.canvas.Save()
	f.canvas.PushOpacity(0.5)
	f.canvas.PushGroup(paint.GroupStyle{})
	f.canvas.DrawRRect(rect(10, 10, 20, 20), paint.Paint{Color: geom.Color{B: 1, A: 1}})
	f.canvas.DrawRRect(rect(20, 20, 20, 20), paint.Paint{Color: geom.Color{G: 1, A: 1}})
	f.canvas.PopGroup()
	f.canvas.Restore()
	Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
	if n := len(f.glyphs.jobs); n != 1 {
		t.Errorf("%d jobs, want the picture of the group", n)
	}
}
