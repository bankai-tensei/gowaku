package render

import (
	"image"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
)

func TestEncode_ImagesCarryTheirFilter(t *testing.T) {
	f := newFixture(t, "")
	img := testImage()
	f.canvas.Reset()
	qualities := []paint.FilterQuality{paint.FilterDefault, paint.FilterNearest, paint.FilterLinear, paint.FilterMipmap}
	for _, q := range qualities {
		f.canvas.DrawImageQuality(img, geom.Rect{W: 10, H: 10}, 1, q)
	}
	Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
	if err := f.glyphs.Finish(&f.batch); err != nil {
		t.Fatal(err)
	}
	want := []gpu.ImageFilter{gpu.FilterMipmap, gpu.FilterNearest, gpu.FilterLinear, gpu.FilterMipmap}
	for i, w := range want {
		if got := f.batch.Instances[i].ImageFilter(); got != w {
			t.Errorf("%v: got filter %v, want %v", qualities[i], got, w)
		}
	}
	if len(f.batch.Draws) != 1 {
		t.Errorf("the filters of one image must share a draw, got %d", len(f.batch.Draws))
	}
	if len(f.store.formats) != 1 || f.store.formats[0] != gpu.TextureRGBA8Mipmapped {
		t.Errorf("got textures %v, want one with mip levels", f.store.formats)
	}
	if want := 4 * 4 * 4 * 4 / 3; f.glyphs.imageBytes != want {
		t.Errorf("got %d bytes, want %d with the levels", f.glyphs.imageBytes, want)
	}
}

// BenchmarkEncode_1kImages draws a thousand mipmapped images a frame, some
// turned, once they are on the GPU.
func BenchmarkEncode_1kImages(b *testing.B) {
	f := newFixture(b, "")
	images := make([]*paint.Image, 16)
	for i := range images {
		images[i] = paint.NewImage(image.NewRGBA(image.Rect(0, 0, 64, 64)))
	}
	f.canvas.Reset()
	for i := range 1000 {
		r := geom.Rect{X: float32(i%40) * 20, Y: float32(i/40) * 20, W: 16, H: 16}
		if i%5 == 0 {
			f.canvas.Save()
			f.canvas.Transform(geom.Translation(r.Center()).Mul(geom.Rotation(0.3)))
			f.canvas.DrawImage(images[i%len(images)], geom.Rect{X: -8, Y: -8, W: 16, H: 16}, 1)
			f.canvas.Restore()
			continue
		}
		f.canvas.DrawImage(images[i%len(images)], r, 1)
	}
	dl := f.canvas.DisplayList()
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

// TestEncode_LoopingAnimationKeepsItsFrames draws the frames of a looping
// 60-frame animation one per frame, as AnimatedImage does. Each frame is
// uploaded once and stays: the later loops upload nothing and allocate
// nothing.
func TestEncode_LoopingAnimationKeepsItsFrames(t *testing.T) {
	f := newFixture(t, "")
	frames := make([]*paint.Image, 60)
	for i := range frames {
		frames[i] = paint.NewImage(image.NewRGBA(image.Rect(0, 0, 120, 90)))
	}
	n := 0
	step := func() {
		f.canvas.Reset()
		f.canvas.DrawImage(frames[n%len(frames)], geom.Rect{W: 120, H: 90}, 1)
		n++
		Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
		if err := f.glyphs.Finish(&f.batch); err != nil {
			t.Fatal(err)
		}
	}
	for range len(frames) {
		step()
	}
	if len(f.store.formats) != len(frames) {
		t.Fatalf("the first loop made %d textures, want %d", len(f.store.formats), len(frames))
	}
	loop := func() {
		for range len(frames) {
			step()
		}
	}
	if allocs := testing.AllocsPerRun(3, loop); allocs != 0 {
		t.Errorf("a loop allocates %v times", allocs)
	}
	if len(f.store.formats) != len(frames) {
		t.Errorf("later loops made textures: %d in all", len(f.store.formats))
	}
}
