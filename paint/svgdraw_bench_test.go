package paint

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
)

const benchCompositeSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">
  <defs>
    <linearGradient id="g" x2="1"><stop offset="0" stop-color="#ff375f"/><stop offset="1" stop-color="#5e5ce6"/></linearGradient>
    <filter id="b"><feGaussianBlur stdDeviation="4"/></filter>
    <mask id="m"><rect width="200" height="200" fill="url(#g)"/></mask>
  </defs>
  <g opacity="0.8" mask="url(#m)">
    <circle cx="70" cy="80" r="50" fill="url(#g)" filter="url(#b)"/>
    <rect x="80" y="60" width="100" height="100" rx="16" fill="#30d158" stroke="#1d1d1f" stroke-width="4"/>
  </g>
</svg>`

// BenchmarkSVG_DrawComposited records a drawing with groups at a size it keeps:
// the groups of the canvas that the GPU composites. It must not allocate.
func BenchmarkSVG_DrawComposited(b *testing.B) {
	s := mustParseSVG(b, benchCompositeSVG)
	c := newCanvas()
	b.ReportAllocs()
	for b.Loop() {
		c.Reset()
		s.Draw(c, geom.Rect{W: 200, H: 200}, geom.Color{A: 1})
	}
}
