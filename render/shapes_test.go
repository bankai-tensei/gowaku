package render

import (
	"image"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
)

func triangle(s float32) *paint.Path {
	p := &paint.Path{}
	p.MoveTo(0, 0)
	p.LineTo(s, 0)
	p.LineTo(s, s)
	p.Close()
	return p
}

func TestEncode_LargePathIsFanAndCover(t *testing.T) {
	f := newFixture(t, "")
	f.canvas.Reset()
	f.canvas.Translate(10, 20)
	f.canvas.DrawPath(triangle(100), paint.Paint{Color: geom.Hex(0xff0000)})
	Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
	if len(f.batch.Draws) != 1 || f.batch.Draws[0].Kind != gpu.DrawFill {
		t.Fatalf("got %+v", f.batch.Draws)
	}
	d := f.batch.Draws[0]
	if d.VertexCount != 3 || f.batch.Vertices[0] != (geom.Offset{X: 10, Y: 20}) {
		t.Errorf("got %+v, %v", d, f.batch.Vertices)
	}
	in := f.batch.Instances[d.First]
	if in.Kind != gpu.InstanceCover || in.Rect.X > 10 || in.Rect.W < 100 {
		t.Errorf("got %+v", in)
	}
}

func TestEncode_SmallPathGoesThroughTheAtlas(t *testing.T) {
	f := newFixture(t, "")
	f.canvas.Reset()
	f.canvas.DrawPath(triangle(20), paint.Paint{Color: geom.Hex(0xff0000)})
	Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
	if err := f.glyphs.Finish(&f.batch); err != nil {
		t.Fatal(err)
	}
	if len(f.batch.Vertices) != 0 || f.batch.Instances[0].Kind != gpu.InstanceGlyph {
		t.Errorf("got %+v", f.batch)
	}
}

func TestEncode_ReusedPathDrawsEveryShape(t *testing.T) {
	f := newFixture(t, "")
	f.canvas.Reset()
	var p paint.Path
	for _, x := range []float32{0, 200} {
		p.Reset()
		p.MoveTo(x, 0)
		p.LineTo(x+100, 0)
		p.LineTo(x+100, 100)
		p.Close()
		f.canvas.DrawPath(&p, paint.Paint{Color: geom.Hex(0xff0000)})
	}
	Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
	if len(f.batch.Vertices) != 6 {
		t.Fatalf("got %d vertices, want two triangles", len(f.batch.Vertices))
	}
	if first, second := f.batch.Vertices[0].X, f.batch.Vertices[3].X; first != 0 || second != 200 {
		t.Errorf("the triangles start at %v and %v, want 0 and 200", first, second)
	}
}

func TestEncode_ReusedSmallPathDrawsEveryShape(t *testing.T) {
	f := newFixture(t, "")
	f.canvas.Reset()
	var p paint.Path
	for _, s := range []float32{10, 20} {
		p.Reset()
		p.AddRRect(geom.RRect{Rect: geom.Rect{W: s, H: s}})
		f.canvas.DrawPath(&p, paint.Paint{Color: geom.Hex(0xff0000)})
	}
	Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
	if err := f.glyphs.Finish(&f.batch); err != nil {
		t.Fatal(err)
	}
	in := f.batch.Instances
	if len(in) != 2 || in[0].Kind != gpu.InstanceGlyph || in[1].Kind != gpu.InstanceGlyph {
		t.Fatalf("got %+v", in)
	}
	if in[0].Rect.W >= in[1].Rect.W {
		t.Errorf("the shapes are %v and %v wide, want the first narrower", in[0].Rect.W, in[1].Rect.W)
	}
	// The next frame changes the path behind the same pointer.
	f.canvas.Reset()
	p.Reset()
	p.AddRRect(geom.RRect{Rect: geom.Rect{W: 30, H: 30}})
	f.canvas.DrawPath(&p, paint.Paint{Color: geom.Hex(0xff0000)})
	Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
	if w := f.batch.Instances[0].Rect.W; w < 30 {
		t.Errorf("the changed path is drawn %v wide, want 30 or more", w)
	}
}

func TestEncode_AtlasLimitIsInPhysicalPixels(t *testing.T) {
	f := newFixture(t, "")
	f.canvas.Reset()
	f.canvas.DrawPath(triangle(40), paint.Paint{Color: geom.Hex(0xff0000)})
	Encode(f.canvas.DisplayList(), f.glyphs, 2, &f.batch)
	if f.batch.Draws[0].Kind != gpu.DrawFill {
		t.Error("80 physical pixels are too large for the atlas")
	}
}

func TestEncode_PathsTheAtlasCannotDraw(t *testing.T) {
	odd := triangle(20)
	odd.Rule = paint.FillEvenOdd
	gradient := &paint.Gradient{EndColor: geom.Hex(0)}
	tests := []struct {
		name string
		path *paint.Path
		pt   paint.Paint
		want int
	}{
		{"even-odd", odd, paint.Paint{Color: geom.Hex(0)}, 1},
		{"gradient", triangle(20), paint.Paint{Gradient: gradient}, 1},
		{"stroke only", triangle(20), paint.Paint{Stroke: &paint.Stroke{Width: 2, Color: geom.Hex(0)}}, 1},
		{"fill and stroke", triangle(20), paint.Paint{Color: geom.Hex(0), Stroke: &paint.Stroke{Width: 2, Color: geom.Hex(0)}}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, "")
			f.canvas.Reset()
			f.canvas.DrawPath(tt.path, tt.pt)
			Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
			if len(f.batch.Draws) != tt.want {
				t.Fatalf("got %d draws", len(f.batch.Draws))
			}
			for _, d := range f.batch.Draws {
				if d.Kind != gpu.DrawFill {
					t.Errorf("got %+v", d)
				}
			}
			if odd := tt.path.Rule == paint.FillEvenOdd; f.batch.Draws[0].EvenOdd != odd {
				t.Error("the fill rule must reach the GPU")
			}
		})
	}
}

// signedArea tells which way a triangle faces.
func signedArea(a, b, c geom.Offset) float32 {
	u, v := b.Sub(a), c.Sub(a)
	return u.X*v.Y - u.Y*v.X
}

func TestStroke_TrianglesFaceOneWay(t *testing.T) {
	f := newFixture(t, "")
	f.canvas.Reset()
	f.canvas.DrawPath(triangle(100), paint.Paint{Stroke: &paint.Stroke{Width: 6, Color: geom.Hex(0)}})
	Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
	v := f.batch.Vertices
	if len(v) == 0 || len(v)%3 != 0 {
		t.Fatalf("got %d vertices", len(v))
	}
	for i := 0; i < len(v); i += 3 {
		if signedArea(v[i], v[i+1], v[i+2]) < 0 {
			t.Fatalf("triangle %d faces the other way", i/3)
		}
	}
}

func TestEncode_ClipOps(t *testing.T) {
	f := newFixture(t, "")
	f.canvas.Reset()
	f.canvas.Save()
	f.canvas.ClipRRect(geom.RRectAll(geom.Rect{W: 100, H: 100}, 10))
	f.canvas.DrawRRect(geom.RRect{Rect: geom.Rect{W: 10, H: 10}}, paint.Paint{Color: geom.Hex(0)})
	f.canvas.Restore()
	f.canvas.DrawRRect(geom.RRect{Rect: geom.Rect{W: 10, H: 10}}, paint.Paint{Color: geom.Hex(0)})
	Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
	want := []struct {
		kind  gpu.DrawKind
		level uint8
	}{{gpu.DrawClipPush, 0}, {gpu.DrawQuads, 1}, {gpu.DrawClipPop, 0}, {gpu.DrawQuads, 0}}
	if len(f.batch.Draws) != len(want) {
		t.Fatalf("got %+v", f.batch.Draws)
	}
	for i, w := range want {
		if d := f.batch.Draws[i]; d.Kind != w.kind || d.ClipLevel != w.level {
			t.Errorf("draw %d: got %+v", i, d)
		}
	}
}

func testImage() *paint.Image { return paint.NewImage(image.NewRGBA(image.Rect(0, 0, 4, 4))) }

func TestEncode_ImageUploadsOnce(t *testing.T) {
	f := newFixture(t, "")
	img := testImage()
	frame := func() {
		f.canvas.Reset()
		f.canvas.DrawImage(img, geom.Rect{W: 10, H: 10}, 1)
		Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
		if err := f.glyphs.Finish(&f.batch); err != nil {
			t.Fatal(err)
		}
	}
	frame()
	frame()
	if f.store.created != 1 || len(f.store.updates) != 1 {
		t.Errorf("created %d, updated %d", f.store.created, len(f.store.updates))
	}
	in := f.batch.Instances[0]
	if in.Kind != gpu.InstanceImage || in.GradientEnd != (geom.Offset{X: 1, Y: 1}) {
		t.Errorf("got %+v", in)
	}
	if f.batch.Draws[0].Image == nil {
		t.Error("the draw must carry the image")
	}
}

func TestEncode_DifferentImagesSplitDraws(t *testing.T) {
	f := newFixture(t, "")
	f.canvas.Reset()
	f.canvas.DrawImage(testImage(), geom.Rect{W: 10, H: 10}, 1)
	f.canvas.DrawImage(testImage(), geom.Rect{W: 10, H: 10}, 1)
	Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
	if len(f.batch.Draws) != 2 {
		t.Errorf("got %d draws", len(f.batch.Draws))
	}
}

func TestImages_OverBudgetDropsTheOldest(t *testing.T) {
	f := newFixture(t, "")
	old, kept := testImage(), testImage()
	frame := func(img *paint.Image) {
		f.canvas.Reset()
		f.canvas.DrawImage(img, geom.Rect{W: 10, H: 10}, 1)
		Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
		if err := f.glyphs.Finish(&f.batch); err != nil {
			t.Fatal(err)
		}
	}
	frame(old)
	f.glyphs.imageBytes = imageBudget + 1
	frame(kept)
	if _, ok := f.glyphs.images[old]; ok {
		t.Error("the image of an earlier frame must go")
	}
	if _, ok := f.glyphs.images[kept]; !ok {
		t.Error("the image of this frame must stay")
	}
}

func TestEncode_WarmShapesDoNotAllocate(t *testing.T) {
	f := newFixture(t, "")
	big, small := triangle(100), triangle(20)
	img := testImage()
	f.canvas.Reset()
	f.canvas.DrawPath(big, paint.Paint{Color: geom.Hex(0), Stroke: &paint.Stroke{Width: 2, Color: geom.Hex(0)}})
	f.canvas.DrawPath(small, paint.Paint{Color: geom.Hex(0)})
	f.canvas.DrawImage(img, geom.Rect{W: 10, H: 10}, 1)
	f.canvas.Save()
	f.canvas.ClipRRect(geom.RRectAll(geom.Rect{W: 100, H: 100}, 10))
	f.canvas.Restore()
	dl := f.canvas.DisplayList()
	Encode(dl, f.glyphs, 1, &f.batch)
	allocs := testing.AllocsPerRun(10, func() { Encode(dl, f.glyphs, 1, &f.batch) })
	if allocs != 0 {
		t.Errorf("got %v allocs", allocs)
	}
}
