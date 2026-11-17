package paint

import (
	"fmt"
	"math"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

func TestParseDashes(t *testing.T) {
	cases := map[string][]float32{
		"":          nil,
		"none":      nil,
		"4 2":       {4, 2},
		"4,2, 1":    {4, 2, 1, 4, 2, 1},
		"0 0":       nil,
		"4 -1":      nil,
		"4 x":       nil,
		"10%":       {2, 2},
		"1px 2pt":   {1, 8.0 / 3},
		"0, 3":      {0, 3},
		"  5\n 5  ": {5, 5},
	}
	for in, want := range cases {
		if got := parseDashes(in, 20); fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%q: got %v, want %v", in, got, want)
		}
	}
}

// subpaths returns the subpaths of p as lists of points.
func subpaths(p *Path) [][]geom.Offset {
	var out [][]geom.Offset
	for i, v := range p.verbs {
		if v == verbMove {
			out = append(out, nil)
		}
		out[len(out)-1] = append(out[len(out)-1], p.points[i])
	}
	return out
}

func TestDash_Line(t *testing.T) {
	line := &Path{}
	line.MoveTo(0, 0)
	line.LineTo(10, 0)
	cases := []struct {
		dashes []float32
		offset float32
		want   [][2]float32 // x of each dash's ends
	}{
		{[]float32{3, 1}, 0, [][2]float32{{0, 3}, {4, 7}, {8, 10}}},
		{[]float32{3, 1}, 2, [][2]float32{{0, 1}, {2, 5}, {6, 9}}},
		{[]float32{3, 1}, -1, [][2]float32{{1, 4}, {5, 8}, {9, 10}}},
		{[]float32{0, 5}, 0, [][2]float32{{0, 0}, {5, 5}}},
	}
	for _, c := range cases {
		got := subpaths(dash(line, c.dashes, c.offset, 1, 0.01))
		if len(got) != len(c.want) {
			t.Errorf("%v offset %v: got %v", c.dashes, c.offset, got)
			continue
		}
		for i, w := range c.want {
			sp := got[i]
			if geom.Abs32(sp[0].X-w[0]) > 1e-4 || geom.Abs32(sp[len(sp)-1].X-w[1]) > 1e-4 {
				t.Errorf("%v offset %v: dash %d is %v, want %v", c.dashes, c.offset, i, sp, w)
			}
		}
	}
	// pathLength scales the pattern.
	if got := subpaths(dash(line, []float32{1, 1}, 0, 2, 0.01)); len(got) != 3 {
		t.Errorf("scaled: got %v", got)
	}
}

func TestDash_ClosedContourJoinsAtTheStart(t *testing.T) {
	square := &Path{}
	square.MoveTo(0, 0)
	square.LineTo(10, 0)
	square.LineTo(10, 10)
	square.LineTo(0, 10)
	square.Close()
	// 40 long: dashes at 0-6, 10-16, 20-26, 30-36, and the one from 38 runs
	// over the start into the first.
	got := subpaths(dash(square, []float32{6, 4}, 2, 1, 0.01))
	if len(got) != 4 {
		t.Fatalf("got %d dashes: %v", len(got), got)
	}
	first := got[0]
	// The ends are interpolated, and a multiply-add that arm64 fuses rounds
	// them a bit away from the whole numbers.
	if !nearOffset(first[0], geom.Offset{Y: 2}) || !nearOffset(first[len(first)-1], geom.Offset{X: 4}) {
		t.Errorf("the wrapped dash runs %v to %v", first[0], first[len(first)-1])
	}
}

func TestParseSVG_DashedStroke(t *testing.T) {
	s := mustParseSVG(t, svgDoc(`<line x1="0" y1="12" x2="24" y2="12" stroke="black" stroke-width="2" stroke-dasharray="4 4"/>`))
	if len(s.shapes) != 1 || s.Skipped != 0 {
		t.Fatalf("got %d shapes, %d skipped", len(s.shapes), s.Skipped)
	}
	p := s.shapes[0].path
	if !inside(p, 2, 12) || inside(p, 6, 12) || !inside(p, 10, 12) {
		t.Error("the dashes are not where they belong")
	}
}

func TestParseSVG_PaintOrder(t *testing.T) {
	s := mustParseSVG(t, svgDoc(`<rect width="10" height="10" fill="red" stroke="blue" paint-order="stroke"/>
		<g paint-order="markers fill"><rect width="10" height="10" fill="red" stroke="blue"/></g>`))
	got := shapeColors(s)
	want := []geom.Color{svgBlue, svgRed, svgRed, svgBlue}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseSVG_Markers(t *testing.T) {
	defs := `<defs><marker id="m" viewBox="0 0 10 10" refX="10" refY="5" markerWidth="4" markerHeight="4" orient="auto">
		<path d="M0 0L10 5L0 10z" fill="red"/></marker>
		<marker id="r" viewBox="0 0 10 10" refX="10" refY="5" orient="auto-start-reverse"><path d="M0 0L10 5L0 10z"/></marker>
		<marker id="u" markerUnits="userSpaceOnUse" markerWidth="2" markerHeight="2" refX="1" refY="1"><rect width="2" height="2" fill="blue"/></marker></defs>`
	t.Run("end follows the line", func(t *testing.T) {
		s := mustParseSVG(t, svgDoc(defs+`<line x1="12" y1="2" x2="12" y2="20" stroke="black" stroke-width="1" marker-end="url(#m)"/>`))
		if len(s.shapes) != 2 {
			t.Fatalf("got %d shapes", len(s.shapes))
		}
		// Pointing down, its tip on the end, four stroke widths long.
		b := s.shapes[1].path.Bounds()
		if geom.Abs32(b.Y+b.H-20) > 1e-3 || geom.Abs32(b.H-4) > 1e-3 || geom.Abs32(b.X+b.W/2-12) > 1e-3 {
			t.Errorf("got %v", b)
		}
	})
	t.Run("markerUnits scale with the stroke", func(t *testing.T) {
		s := mustParseSVG(t, svgDoc(defs+`<line x1="2" y1="12" x2="20" y2="12" stroke="black" stroke-width="2" marker-end="url(#m)"/>`))
		if b := s.shapes[1].path.Bounds(); geom.Abs32(b.W-8) > 1e-3 || geom.Abs32(b.X+b.W-20) > 1e-3 {
			t.Errorf("got %v", b)
		}
	})
	t.Run("start reversed", func(t *testing.T) {
		s := mustParseSVG(t, svgDoc(defs+`<line x1="2" y1="12" x2="20" y2="12" stroke="black" marker-start="url(#r)"/>`))
		// The tip of the start points away from the line, to the left.
		if b := s.shapes[1].path.Bounds(); geom.Abs32(b.X-2) > 1e-3 || geom.Abs32(b.W-3) > 1e-3 {
			t.Errorf("got %v", b)
		}
	})
	t.Run("mid on every inner vertex", func(t *testing.T) {
		s := mustParseSVG(t, svgDoc(defs+`<polyline points="2,2 10,2 10,10 2,10" fill="none" stroke="black" marker-mid="url(#u)"/>`))
		if len(s.shapes) != 3 {
			t.Fatalf("got %d shapes", len(s.shapes))
		}
		for i, want := range []geom.Offset{{X: 10, Y: 2}, {X: 10, Y: 10}} {
			b := s.shapes[1+i].path.Bounds()
			if c := (geom.Offset{X: b.X + b.W/2, Y: b.Y + b.H/2}); !nearOffset(c, want) {
				t.Errorf("marker %d at %v, want %v", i, c, want)
			}
		}
	})
	t.Run("a marker inherits from where it is defined", func(t *testing.T) {
		s := mustParseSVG(t, svgDoc(`<g fill="blue"><marker id="k" markerUnits="userSpaceOnUse"><rect width="2" height="2"/></marker></g>
			<line x2="10" stroke="black" fill="red" marker-end="url(#k)"/>`))
		if got := shapeColors(s); len(got) != 2 || got[1] != svgBlue {
			t.Errorf("got %v", got)
		}
	})
	t.Run("loops", func(t *testing.T) {
		s := mustParseSVG(t, svgDoc(`<marker id="k"><path d="M0 0L2 2" stroke="black" marker-end="url(#k)"/></marker>
			<line x2="10" stroke="black" marker-end="url(#k)"/>`))
		if s.Skipped == 0 {
			t.Error("want the loop counted")
		}
	})
}

func TestVertexAngle(t *testing.T) {
	deg := func(a float32) float32 { return a * 180 / math.Pi }
	right, down, left := geom.Offset{X: 1}, geom.Offset{Y: 1}, geom.Offset{X: -1}
	cases := []struct {
		v    svgVertex
		kind svgMarkerKind
		want float32
	}{
		{svgVertex{out: down}, svgMarkerStart, 90},
		{svgVertex{in: down}, svgMarkerEnd, 90},
		{svgVertex{in: right, out: down}, svgMarkerMid, 45},
		{svgVertex{in: down, out: left}, svgMarkerMid, 135},
		{svgVertex{in: left, out: geom.Offset{Y: -1}}, svgMarkerMid, -135},
	}
	for _, c := range cases {
		if got := deg(vertexAngle(c.v)); geom.Abs32(got-c.want) > 1e-3 {
			t.Errorf("%+v: got %v, want %v", c.v, got, c.want)
		}
	}
}

func TestPathVertices(t *testing.T) {
	v := svgVertices{start: -1}
	sp := svgParser{in: "M0 0 L10 0 C10 5 15 10 20 10 Z M30 0 h5", path: &Path{}, verts: &v}
	if err := sp.run(); err != nil {
		t.Fatal(err)
	}
	pos := make([]geom.Offset, len(v.list))
	for i := range v.list {
		pos[i] = v.list[i].pos
	}
	want := []geom.Offset{{}, {X: 10}, {X: 20, Y: 10}, {}, {X: 30}, {X: 35}}
	if fmt.Sprint(pos) != fmt.Sprint(want) {
		t.Fatalf("got %v, want %v", pos, want)
	}
	// The curve leaves (10, 0) downwards, toward its first control point.
	if v.list[1].out != (geom.Offset{Y: 5}) {
		t.Errorf("got %v", v.list[1].out)
	}
	// The closed subpath joins its ends.
	if v.list[0].in != v.list[3].in || v.list[3].out != v.list[0].out {
		t.Errorf("got %+v", v.list[:4])
	}
}
