package render

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
)

// layerFixture records layers and encodes frames with them.
type layerFixture struct {
	*fixture
}

func newLayerFixture(t testing.TB) *layerFixture {
	return &layerFixture{newFixture(t, "gowaku")}
}

// filled records a colored box into l.
func filled(l *paint.Layer, w, h float32) {
	cv := l.Begin()
	cv.DrawRRect(geom.RRect{Rect: geom.Rect{W: w, H: h}}, paint.Paint{Color: red})
	l.End()
}

func (f *layerFixture) frame(t testing.TB, scale float32, draw func(cv *paint.Canvas)) {
	t.Helper()
	f.canvas.Reset()
	draw(&f.canvas)
	Encode(f.canvas.DisplayList(), f.glyphs, scale, &f.batch)
	if err := f.glyphs.Finish(&f.batch); err != nil {
		t.Fatal(err)
	}
}

func TestLayer_DrawsOncePerVersion(t *testing.T) {
	f := newLayerFixture(t)
	var l paint.Layer
	filled(&l, 100, 20)
	draw := func(cv *paint.Canvas) { cv.DrawLayer(&l, geom.Offset{X: 10, Y: 30}) }

	for range layerCalmFrames + 1 {
		f.frame(t, 1, draw)
	}
	if f.store.targetDraws != 1 {
		t.Fatalf("drew the layer %d times for one version", f.store.targetDraws)
	}
	filled(&l, 100, 20)
	f.frame(t, 1, draw)
	if f.store.targetDraws != 2 {
		t.Fatalf("a new version drew %d times in all, want 2", f.store.targetDraws)
	}
	if f.store.targets != 1 {
		t.Errorf("created %d pages", f.store.targets)
	}
}

func TestLayer_ChangingEveryFrameStaysOffThePage(t *testing.T) {
	f := newLayerFixture(t)
	var l paint.Layer
	draw := func(cv *paint.Canvas) { cv.DrawLayer(&l, geom.Offset{}) }
	for range 10 {
		filled(&l, 100, 20)
		f.frame(t, 1, draw)
	}
	if f.store.targetDraws != 1 {
		t.Errorf("drew to a page %d times, want only the first", f.store.targetDraws)
	}
	if k := f.batch.Instances[0].Kind; k != gpu.InstanceRRect {
		t.Errorf("kind %d, want the commands of the layer", k)
	}
	for range layerCalmFrames + 1 {
		f.frame(t, 1, draw)
	}
	if f.store.targetDraws != 2 {
		t.Errorf("drew to a page %d times, want once more after calm", f.store.targetDraws)
	}
	if k := f.batch.Instances[0].Kind; k != gpu.InstanceImage {
		t.Errorf("kind %d, want the pixels back", k)
	}
}

func TestLayer_ScaleDrawsAgain(t *testing.T) {
	f := newLayerFixture(t)
	var l paint.Layer
	filled(&l, 100, 20)
	draw := func(cv *paint.Canvas) { cv.DrawLayer(&l, geom.Offset{}) }
	for range layerCalmFrames + 1 {
		f.frame(t, 1, draw)
	}
	f.frame(t, 2, draw)
	if f.store.targetDraws != 2 {
		t.Errorf("drew %d times over two scales", f.store.targetDraws)
	}
}

func TestLayer_CompositeLandsOnWholePixels(t *testing.T) {
	f := newLayerFixture(t)
	var l paint.Layer
	filled(&l, 100, 20)
	f.frame(t, 1, func(cv *paint.Canvas) { cv.DrawLayer(&l, geom.Offset{X: 10.3, Y: 30.6}) })
	if len(f.batch.Instances) != 1 {
		t.Fatalf("got %d instances", len(f.batch.Instances))
	}
	in := f.batch.Instances[0]
	if in.Kind != gpu.InstanceImage {
		t.Fatalf("kind %d", in.Kind)
	}
	if in.Rect.X != geom.Floor32(in.Rect.X) || in.Rect.Y != geom.Floor32(in.Rect.Y) {
		t.Errorf("composite at %v", in.Rect)
	}
	// The box starts one pixel inside what the shader reaches.
	if in.Rect.X != 9 || in.Rect.Y != 30 {
		t.Errorf("composite at %v, want the rounded origin less the edge", in.Rect)
	}
}

func TestLayer_ShapesGoInline(t *testing.T) {
	f := newLayerFixture(t)
	var l paint.Layer
	cv := l.Begin()
	var p paint.Path
	p.AddRRect(geom.RRect{Rect: geom.Rect{W: 300, H: 300}, TopLeft: 20, TopRight: 20, BottomRight: 20, BottomLeft: 20})
	cv.DrawPath(&p, paint.Paint{Color: red})
	l.End()

	draw := func(cv *paint.Canvas) {
		cv.ClipRect(geom.Rect{X: 50, Y: 50, W: 100, H: 100})
		cv.DrawLayer(&l, geom.Offset{X: 40, Y: 40})
	}
	f.frame(t, 1, draw)
	if f.store.targetDraws != 0 {
		t.Error("a path needs multisampling, which pages do not have")
	}
	if !f.batch.Multisampled() {
		t.Fatal("the path must join the frame")
	}
	d := f.batch.Draws[0]
	if !d.Clipped || d.Clip != (geom.Rect{X: 50, Y: 50, W: 100, H: 100}) {
		t.Errorf("clip %v, want that of the frame", d.Clip)
	}
	if v := f.batch.Vertices[0]; v.X < 40 || v.Y < 40 {
		t.Errorf("vertex %v did not move to the origin of the layer", v)
	}
}

func TestLayer_TooLargeGoesInline(t *testing.T) {
	f := newLayerFixture(t)
	var l paint.Layer
	filled(&l, layerPageSize+10, 20)
	f.frame(t, 1, func(cv *paint.Canvas) { cv.DrawLayer(&l, geom.Offset{}) })
	if f.store.targetDraws != 0 || f.store.targets != 0 {
		t.Error("a layer larger than a page cannot go on one")
	}
	if len(f.batch.Instances) != 1 || f.batch.Instances[0].Kind != gpu.InstanceRRect {
		t.Errorf("got %+v", f.batch.Instances)
	}
}

func TestLayer_ScrollingReusesIdlePages(t *testing.T) {
	f := newLayerFixture(t)
	// A layer of this size fills a page, so that each one takes a page of its own.
	const size = layerPageSize - 8
	layers := make([]paint.Layer, 40)
	for i := range layers {
		filled(&layers[i], size, size)
		f.frame(t, 1, func(cv *paint.Canvas) { cv.DrawLayer(&layers[i], geom.Offset{}) })
	}
	if f.store.targets > layerPageIdle+1 {
		t.Errorf("created %d pages for one layer at a time", f.store.targets)
	}
	if _, ok := f.glyphs.layers.entries[&layers[0]]; ok {
		t.Error("the oldest layer must leave")
	}
	if _, ok := f.glyphs.layers.entries[&layers[39]]; !ok {
		t.Error("the newest layer must stay")
	}
}

func TestLayer_AllPagesInUseGoesInline(t *testing.T) {
	f := newLayerFixture(t)
	const size = layerPageSize - 8
	layers := make([]paint.Layer, layerPageLimit+1)
	for i := range layers {
		filled(&layers[i], size, size)
	}
	f.frame(t, 1, func(cv *paint.Canvas) {
		for i := range layers {
			cv.DrawLayer(&layers[i], geom.Offset{})
		}
	})
	if f.store.targets != layerPageLimit {
		t.Errorf("created %d pages", f.store.targets)
	}
	last := f.batch.Instances[len(f.batch.Instances)-1]
	if last.Kind != gpu.InstanceRRect {
		t.Errorf("the layer without room has kind %d, want its own commands", last.Kind)
	}
}

func TestLayer_RowsShareAShelf(t *testing.T) {
	var p layerPage
	a, _ := p.allocate(100, 30)
	b, _ := p.allocate(100, 30)
	if a.Min.Y != b.Min.Y || b.Min.X != a.Max.X {
		t.Errorf("slots %v and %v", a, b)
	}
	c, _ := p.allocate(100, 200)
	if c.Min.Y < a.Max.Y {
		t.Errorf("a taller slot %v overlaps %v", c, a)
	}
	if _, ok := p.allocate(layerPageSize+1, 10); ok {
		t.Error("a slot wider than the page cannot exist")
	}
}

func TestLayer_DestroyReleasesPages(t *testing.T) {
	f := newLayerFixture(t)
	var l paint.Layer
	filled(&l, 10, 10)
	f.frame(t, 1, func(cv *paint.Canvas) { cv.DrawLayer(&l, geom.Offset{}) })
	before := f.store.destroyed
	f.glyphs.Destroy()
	if f.store.destroyed != before+1 {
		t.Errorf("destroyed %d textures", f.store.destroyed-before)
	}
}

func BenchmarkEncode_CleanLayers(b *testing.B) {
	f := newLayerFixture(b)
	layers := make([]paint.Layer, 40)
	for i := range layers {
		filled(&layers[i], 400, 24)
	}
	draw := func(cv *paint.Canvas) {
		for i := range layers {
			cv.DrawLayer(&layers[i], geom.Offset{Y: float32(i) * 24})
		}
	}
	f.frame(b, 1, draw)
	b.ReportAllocs()
	for b.Loop() {
		f.frame(b, 1, draw)
	}
}
