package render

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

func benchIcons(n int, size float32) []*paint.Path {
	shapes := make([]*paint.Path, n)
	for i := range shapes {
		p := &paint.Path{}
		p.AddRRect(geom.RRectAll(geom.Rect{X: 1, Y: 1, W: size, H: size}, float32(i%8)))
		shapes[i] = p
	}
	return shapes
}

func benchEncodePaths(b *testing.B, size float32) {
	f := newFixture(b, "")
	shapes := benchIcons(1000, size)
	frame := func() {
		f.canvas.Reset()
		for i, p := range shapes {
			f.canvas.Save()
			f.canvas.Translate(float32(i%40)*(size+4), float32(i/40)*(size+4))
			f.canvas.DrawPath(p, paint.Paint{Color: geom.Hex(0x202020)})
			f.canvas.Restore()
		}
		Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
	}
	frame()
	b.ReportAllocs()
	for b.Loop() {
		frame()
	}
}

// BenchmarkEncode_1kSmallPaths records and encodes icons that go through the
// atlas.
func BenchmarkEncode_1kSmallPaths(b *testing.B) { benchEncodePaths(b, 20) }

// BenchmarkEncode_1kLargePaths records and encodes paths drawn with stencil and
// cover.
func BenchmarkEncode_1kLargePaths(b *testing.B) { benchEncodePaths(b, 100) }
