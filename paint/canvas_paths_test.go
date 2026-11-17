package paint

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

// chartSeries draws series areas of points each, all with one path, the way a
// painter that keeps a single Path does.
func chartSeries(c *Canvas, p *Path, series, points int) {
	for s := range series {
		p.Reset()
		base := float32(s * 40)
		p.MoveTo(0, base+40)
		for i := range points {
			x := float32(i) * 4
			p.LineTo(x, base+float32(i%7)*5)
		}
		p.LineTo(float32(points-1)*4, base+40)
		p.Close()
		c.DrawPath(p, Paint{Color: geom.Hex(0x3070f0)})
	}
}

// icons draws unchanging paths, the way widgets draw their glyphs.
func icons(c *Canvas, shapes []*Path) {
	for i, p := range shapes {
		c.Save()
		c.Translate(float32(i%40)*24, float32(i/40)*24)
		c.DrawPath(p, Paint{Color: geom.Hex(0x202020)})
		c.Restore()
	}
}

func iconShapes(n int) []*Path {
	shapes := make([]*Path, n)
	for i := range shapes {
		p := &Path{}
		p.AddRRect(geom.RRectAll(geom.Rect{X: 2, Y: 2, W: 20, H: 20}, float32(i%8)))
		shapes[i] = p
	}
	return shapes
}

func TestCanvas_ReusedPathKeepsEachShape(t *testing.T) {
	var c Canvas
	c.Reset()
	var p Path
	p.AddRRect(geom.RRect{Rect: geom.Rect{X: 0, Y: 0, W: 10, H: 10}})
	c.DrawPath(&p, Paint{Color: geom.Hex(0xff0000)})
	p.Reset()
	p.AddRRect(geom.RRect{Rect: geom.Rect{X: 50, Y: 0, W: 20, H: 20}})
	c.DrawPath(&p, Paint{Color: geom.Hex(0x00ff00)})
	ops := c.DisplayList().Ops
	if len(ops) != 2 {
		t.Fatalf("got %d ops, want 2", len(ops))
	}
	if got, want := ops[0].Path.Bounds(), (geom.Rect{W: 10, H: 10}); got != want {
		t.Errorf("the first shape became %v, want %v", got, want)
	}
	if got, want := ops[1].Path.Bounds(), (geom.Rect{X: 50, W: 20, H: 20}); got != want {
		t.Errorf("the second shape is %v, want %v", got, want)
	}
	if ops[0].Path == &p || ops[1].Path == &p || ops[0].Path == ops[1].Path {
		t.Error("each op must point to a copy of its own")
	}
}

func TestCanvas_ReusedClipPathKeepsEachShape(t *testing.T) {
	var c Canvas
	c.Reset()
	var p Path
	p.AddRRect(geom.RRect{Rect: geom.Rect{W: 10, H: 10}})
	c.Save()
	c.ClipPath(&p)
	p.Reset()
	p.AddRRect(geom.RRect{Rect: geom.Rect{X: 2, Y: 2, W: 4, H: 4}})
	c.ClipPath(&p)
	c.Restore()
	ops := c.DisplayList().Ops
	if ops[0].Kind != OpClipPush || ops[1].Kind != OpClipPush {
		t.Fatalf("got %v and %v", ops[0].Kind, ops[1].Kind)
	}
	if got, want := ops[0].Path.Bounds(), (geom.Rect{W: 10, H: 10}); got != want {
		t.Errorf("the outer clip became %v, want %v", got, want)
	}
}

func TestCanvas_CopiesSurviveTheNextFrame(t *testing.T) {
	var c Canvas
	var p Path
	c.Reset()
	chartSeries(&c, &p, 3, 10)
	c.Reset()
	chartSeries(&c, &p, 2, 10)
	ops := c.DisplayList().Ops
	if len(ops) != 2 || ops[0].Path.Bounds().Y == ops[1].Path.Bounds().Y {
		t.Fatalf("got %d ops, the series must stay apart", len(ops))
	}
	if !ops[1].Path.Equal(&p) {
		t.Error("the last copy must equal the path")
	}
}

func TestCanvas_CopyFollowsThePathAcrossFrames(t *testing.T) {
	square := func(p *Path, size float32) {
		p.Reset()
		p.AddRRect(geom.RRect{Rect: geom.Rect{W: size, H: size}})
	}
	var c Canvas
	p := &Path{}
	square(p, 10)
	paint := func() { c.DrawPath(p, Paint{Color: geom.Hex(0)}) }
	frame := func(draw func()) geom.Rect {
		c.Reset()
		draw()
		ops := c.DisplayList().Ops
		return ops[len(ops)-1].Path.Bounds()
	}
	if got := frame(paint); got.W != 10 {
		t.Fatalf("first frame: got %v", got)
	}
	if got := frame(paint); got.W != 10 {
		t.Errorf("unchanged: got %v", got)
	}
	square(p, 20)
	if got := frame(paint); got.W != 20 {
		t.Errorf("changed: got %v, want 20 wide", got)
	}
	// Assigning another path over p can bring back the same count of changes
	// with other contents.
	other := &Path{}
	other.AddRRect(geom.RRect{Rect: geom.Rect{W: 30, H: 30}})
	other.gen = p.gen
	*p = *other
	if got := frame(paint); got.W != 30 {
		t.Errorf("assigned over: got %v, want 30 wide", got)
	}
	p = &Path{}
	square(p, 40)
	if got := frame(paint); got.W != 40 {
		t.Errorf("another path: got %v, want 40 wide", got)
	}
	// A rounded clip takes the place of the copy, and the next frame draws p
	// there again.
	frame(func() {
		c.Save()
		c.ClipRRect(geom.RRectAll(geom.Rect{W: 5, H: 5}, 1))
		c.Restore()
		paint()
	})
	if got := frame(paint); got.W != 40 {
		t.Errorf("after a clip in its place: got %v, want 40 wide", got)
	}
	p.Rule = FillEvenOdd
	frame(paint)
	if got := c.DisplayList().Ops[0].Path.Rule; got != FillEvenOdd {
		t.Errorf("got rule %v, want the new one", got)
	}
}

func TestCanvas_ReusedPathDoesNotAllocate(t *testing.T) {
	var c Canvas
	var p Path
	c.Reset()
	chartSeries(&c, &p, 8, 200)
	allocs := testing.AllocsPerRun(10, func() {
		c.Reset()
		chartSeries(&c, &p, 8, 200)
		c.Save()
		c.ClipPath(&p)
		c.ClipRRect(geom.RRectAll(geom.Rect{W: 10, H: 10}, 2))
		c.Restore()
	})
	if allocs != 0 {
		t.Errorf("got %v allocs, want 0", allocs)
	}
}

func BenchmarkCanvas_ReusedPath(b *testing.B) {
	c := newCanvas()
	var p Path
	c.Reset()
	chartSeries(c, &p, 8, 200)
	b.ReportAllocs()
	for b.Loop() {
		c.Reset()
		chartSeries(c, &p, 8, 200)
	}
}

func BenchmarkCanvas_1kIcons(b *testing.B) {
	c := newCanvas()
	shapes := iconShapes(1000)
	c.Reset()
	icons(c, shapes)
	b.ReportAllocs()
	for b.Loop() {
		c.Reset()
		icons(c, shapes)
	}
}
