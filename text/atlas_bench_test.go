package text

import (
	"testing"

	"github.com/bankai-tensei/gowaku/paint"
)

// churnFrame draws one frame of a long Japanese document scrolling by: the
// glyphs of the interface on screen every frame, at body, ruby and heading sizes
// and sideways, and a window of the document that moves through a stream of
// 20 000 keys, a few new glyphs a frame.
type churnFrame struct {
	face   paint.FaceID
	stream int
}

const (
	churnUI       = 500
	churnRuby     = 300
	churnHeading  = 200
	churnWindow   = 2000
	churnPerFrame = 40
	churnStream   = 20000
	churnFirstID  = 1000
)

func (c *churnFrame) draw(a *Atlas) {
	c.drawAt(a, c.stream)
	c.stream += churnPerFrame
}

// drawAt draws a frame with the window at key at.
func (c *churnFrame) drawAt(a *Atlas, at int) {
	a.BeginFrame()
	for i := range churnUI {
		a.Glyph(GlyphKey{Face: c.face, ID: uint32(churnFirstID + i), Size: 16})
	}
	for i := range churnRuby {
		a.Glyph(GlyphKey{Face: c.face, ID: uint32(churnFirstID + i), Size: 8})
	}
	for i := range churnHeading {
		a.Glyph(GlyphKey{Face: c.face, ID: uint32(churnFirstID + i), Size: 24, Sideways: true})
	}
	for i := range churnWindow {
		n := (at + i) % churnStream
		a.Glyph(GlyphKey{Face: c.face, ID: uint32(churnFirstID + churnUI + n/SubpixelSteps), Size: 16, Subpixel: uint8(n % SubpixelSteps)})
	}
}

// BenchmarkAtlas_ChurnCJK is one frame of churnFrame on an atlas of two pages of
// 1024 texels, which hold what is on screen with room to spare but not the
// stream.
// rasters/frame is the glyphs drawn again per frame; the 40 new ones are the
// least it can be.
func BenchmarkAtlas_ChurnCJK(b *testing.B) {
	s := systemShaper(b)
	a := newAtlas(s, 1024, 2)
	c := &churnFrame{face: s.Shape("語", Style{Size: 16}).clusters[0].face}
	// Fill the atlas and run the stream once around before measuring.
	for range churnStream / churnPerFrame {
		c.draw(a)
	}
	before := a.rasters
	frames := 0
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.draw(a)
		frames++
	}
	b.ReportMetric(float64(a.rasters-before)/float64(frames), "rasters/frame")
	b.ReportMetric(float64(a.PageCount()), "pages")
}

// BenchmarkAtlas_ScrollBack scrolls the window of churnFrame down 6000 keys and
// back up, over and over, on an atlas of two pages of 1024 texels that holds
// most of what it passes. A frame on the way back finds the glyphs it drew a
// moment ago only if the atlas kept the recent ones.
func BenchmarkAtlas_ScrollBack(b *testing.B) {
	s := systemShaper(b)
	a := newAtlas(s, 1024, 2)
	c := &churnFrame{face: s.Shape("語", Style{Size: 16}).clusters[0].face}
	const span = 6000 / churnPerFrame
	frame := 0
	at := func() int {
		k := frame % (2 * span)
		frame++
		if k >= span {
			k = 2*span - k
		}
		return 3*churnStream/4*(frame/(2*span)%2) + k*churnPerFrame
	}
	for range 4 * span {
		c.drawAt(a, at())
	}
	before := a.rasters
	frames := 0
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.drawAt(a, at())
		frames++
	}
	b.ReportMetric(float64(a.rasters-before)/float64(frames), "rasters/frame")
	b.ReportMetric(float64(a.PageCount()), "pages")
}
