package soft

import (
	"image"
	"image/color"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

func rectOf(width, height int) image.Rectangle { return image.Rect(0, 0, width, height) }

func pt(x, y float32) geom.Offset { return geom.Offset{X: x, Y: y} }

// square returns two triangles, clockwise on screen.
func square(x, y, w, h float32) []geom.Offset {
	return fanOf(pt(x, y), pt(x+w, y), pt(x+w, y+h), pt(x, y+h))
}

func TestScissor_LimitsDrawing(t *testing.T) {
	b := quads(solid(0, 0, 10, 10, geom.Hex(0x000000)))
	b.Draws[0].Clipped, b.Draws[0].Clip = true, geom.Rect{X: 2, Y: 3, W: 4, H: 2}
	img := render(t, frame{width: 10, height: 10}, b)
	checkPixel(t, img, 1, 3, white)
	checkPixel(t, img, 2, 3, black)
	checkPixel(t, img, 5, 4, black)
	checkPixel(t, img, 6, 4, white)
	checkPixel(t, img, 2, 2, white)
	checkPixel(t, img, 2, 5, white)
}

func TestScissor_EdgesRoundToPhysicalPixels(t *testing.T) {
	b := quads(solid(0, 0, 10, 10, geom.Hex(0x000000)))
	// At scale 1.5 the edges 1 and 3 are 1.5 and 4.5, which round up to 2 and 5.
	b.Draws[0].Clipped, b.Draws[0].Clip = true, geom.Rect{X: 1, Y: 0, W: 2, H: 10}
	img := render(t, frame{width: 15, height: 15, scale: 1.5}, b)
	checkPixel(t, img, 1, 3, white)
	checkPixel(t, img, 2, 3, black)
	checkPixel(t, img, 4, 3, black)
	checkPixel(t, img, 5, 3, white)
}

func TestScissor_OutsideTheSurfaceIsClamped(t *testing.T) {
	b := quads(solid(0, 0, 10, 10, geom.Hex(0x000000)))
	b.Draws[0].Clipped, b.Draws[0].Clip = true, geom.Rect{X: -50, Y: -50, W: 500, H: 500}
	img := render(t, frame{width: 4, height: 4}, b)
	checkPixel(t, img, 3, 3, black)
}

func TestScissor_AppliesToStencilSteps(t *testing.T) {
	b := &gpu.Batch{}
	addFill(b, cover(0, 0, 8, 8, geom.Hex(0x000000)), square(0, 0, 8, 8), false, 0)
	b.Draws[0].Clipped, b.Draws[0].Clip = true, geom.Rect{W: 4, H: 8}
	img := render(t, frame{width: 8, height: 8}, b)
	checkPixel(t, img, 3, 3, black)
	checkPixel(t, img, 4, 3, white)
}

func TestClip_LevelHidesWhatIsOutside(t *testing.T) {
	bounds := geom.Rect{W: 20, H: 20}
	b := &gpu.Batch{}
	addPush(b, bounds, square(4, 4, 8, 8), 0)
	addQuad(b, solid(0, 0, 20, 20, geom.Hex(0x000000)), 1)
	img := render(t, frame{width: 20, height: 20}, b)
	checkPixel(t, img, 3, 8, white)
	checkPixel(t, img, 4, 8, black)
	checkPixel(t, img, 11, 11, black)
	checkPixel(t, img, 12, 8, white)
}

func TestClip_DrawsNothingItself(t *testing.T) {
	b := &gpu.Batch{}
	addPush(b, geom.Rect{W: 8, H: 8}, square(0, 0, 8, 8), 0)
	img := render(t, frame{width: 8, height: 8}, b)
	checkPixel(t, img, 4, 4, white)
}

func TestClip_ShapedEdgeIsPerSample(t *testing.T) {
	// The boundary x = 4.5 lies inside the column of pixel 4.
	b := &gpu.Batch{}
	addPush(b, geom.Rect{W: 8, H: 8}, square(0, 0, 4.5, 8), 0)
	addQuad(b, solid(0, 0, 8, 8, geom.Hex(0x000000)), 1)
	img := render(t, frame{width: 8, height: 8}, b)
	checkPixel(t, img, 3, 4, black)
	checkPixel(t, img, 4, 4, mix(white, black, 2))
	checkPixel(t, img, 5, 4, white)
}

func TestClip_NestedNarrowsAndPopsBack(t *testing.T) {
	bounds := geom.Rect{W: 30, H: 10}
	full := func(c uint32) gpu.Instance { return solid(0, 0, 30, 10, geom.Hex(c)) }
	b := &gpu.Batch{}
	addPush(b, bounds, square(5, 0, 15, 10), 0) // x 5 to 20
	addQuad(b, full(0xff0000), 1)
	addPush(b, bounds, square(10, 0, 15, 10), 1) // x 10 to 25, narrowed to 10 to 20
	addQuad(b, full(0x00ff00), 2)
	addPop(b, bounds, 1)
	// Back at level 1, which paints the whole of the outer clip again.
	addQuad(b, solid(0, 0, 30, 5, geom.Hex(0x0000ff)), 1)
	addPop(b, bounds, 0)
	addQuad(b, solid(0, 0, 30, 2, geom.Hex(0x000000)), 0)
	img := render(t, frame{width: 30, height: 10}, b)

	// Rows 5 to 9 show the clips.
	checkPixel(t, img, 4, 7, white)
	checkPixel(t, img, 5, 7, red)
	checkPixel(t, img, 9, 7, red)
	checkPixel(t, img, 10, 7, green)
	checkPixel(t, img, 19, 7, green)
	checkPixel(t, img, 20, 7, white) // inside the inner clip, outside the outer
	checkPixel(t, img, 24, 7, white)
	// Rows 2 to 4 went through level 1 after the first pop.
	checkPixel(t, img, 4, 3, white)
	checkPixel(t, img, 5, 3, blue)
	checkPixel(t, img, 19, 3, blue)
	checkPixel(t, img, 20, 3, white)
	// Rows 0 and 1 went through level 0, with all clips gone.
	checkPixel(t, img, 0, 1, black)
	checkPixel(t, img, 29, 1, black)
}

func TestClip_PushOutsideTheParentDoesNotGrowIt(t *testing.T) {
	bounds := geom.Rect{W: 20, H: 4}
	b := &gpu.Batch{}
	addPush(b, bounds, square(0, 0, 5, 4), 0)
	addPush(b, bounds, square(10, 0, 5, 4), 1) // misses the parent altogether
	addQuad(b, solid(0, 0, 20, 4, geom.Hex(0x000000)), 2)
	img := render(t, frame{width: 20, height: 4}, b)
	for x := range 20 {
		checkPixel(t, img, x, 2, white)
	}
}

func TestClip_PopOnlyTouchesItsBounds(t *testing.T) {
	b := &gpu.Batch{}
	addPush(b, geom.Rect{W: 8, H: 4}, square(0, 0, 8, 4), 0)
	addPop(b, geom.Rect{W: 4, H: 4}, 0)
	addQuad(b, solid(0, 0, 8, 4, geom.Hex(0x000000)), 0)
	img := render(t, frame{width: 8, height: 4}, b)
	checkPixel(t, img, 3, 2, black)
	checkPixel(t, img, 4, 2, white) // still at level 1
}

func TestClip_FillInsideAClip(t *testing.T) {
	bounds := geom.Rect{W: 20, H: 4}
	b := &gpu.Batch{}
	addPush(b, bounds, square(0, 0, 10, 4), 0)
	addFill(b, cover(0, 0, 20, 4, geom.Hex(0x000000)), square(5, 0, 15, 4), false, 1)
	// A second fill with no triangles would show marks left by the first.
	addFill(b, cover(0, 0, 20, 4, geom.Hex(0xff0000)), nil, false, 0)
	img := render(t, frame{width: 20, height: 4}, b)
	checkPixel(t, img, 4, 2, white)
	checkPixel(t, img, 5, 2, black)
	checkPixel(t, img, 9, 2, black)
	checkPixel(t, img, 10, 2, white)
}

// sampleWinding returns the winding number of a closed polygon at a point.
func sampleWinding(polygon []geom.Offset, x, y float64) int {
	winding := 0
	for i := range polygon {
		a, b := polygon[i], polygon[(i+1)%len(polygon)]
		ax, ay, bx, by := float64(a.X), float64(a.Y), float64(b.X), float64(b.Y)
		side := (bx-ax)*(y-ay) - (by-ay)*(x-ax)
		switch {
		case ay <= y && by > y && side < 0:
			winding--
		case ay > y && by <= y && side > 0:
			winding++
		}
	}
	return winding
}

// star returns a pentagram on the grid of the rasterizer, off the sample positions.
func star() []geom.Offset {
	const g = 1.0 / 256
	return []geom.Offset{
		pt(30+27*g, 2+13*g), pt(48+5*g, 56+7*g), pt(2+3*g, 21+11*g),
		pt(58+9*g, 21+17*g), pt(12+19*g, 56+23*g),
	}
}

func TestFill_MatchesWindingRulePerSample(t *testing.T) {
	polygon := star()
	const size = 64
	tests := []struct {
		name    string
		evenOdd bool
		plain   bool
		filled  func(winding int) bool
	}{
		{"non-zero, 4 samples", false, false, func(w int) bool { return w != 0 }},
		{"non-zero, 1 sample", false, true, func(w int) bool { return w != 0 }},
		{"even-odd, 4 samples", true, false, func(w int) bool { return w%2 != 0 }},
		{"even-odd, 1 sample", true, true, func(w int) bool { return w%2 != 0 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &gpu.Batch{}
			addFill(b, cover(0, 0, size, size, geom.Hex(0x000000)), fanOf(polygon...), tt.evenOdd, 0)
			img := render(t, frame{width: size, height: size, plain: tt.plain}, b)
			samples := 4
			if tt.plain {
				samples = 1
			}
			positions := samplePositions(samples)
			for y := range size {
				for x := range size {
					n := 0
					for _, p := range positions {
						sx := float64(x) + float64(p.x)/subpixel
						sy := float64(y) + float64(p.y)/subpixel
						if tt.filled(sampleWinding(polygon, sx, sy)) {
							n++
						}
					}
					checkPixel(t, img, x, y, mix(white, black, n*4/samples))
				}
			}
		})
	}
}

func TestFill_StarCenterDependsOnRule(t *testing.T) {
	const size = 64
	center := func(evenOdd bool) color.NRGBA {
		b := &gpu.Batch{}
		addFill(b, cover(0, 0, size, size, geom.Hex(0x000000)), fanOf(star()...), evenOdd, 0)
		return render(t, frame{width: size, height: size}, b).NRGBAAt(30, 30)
	}
	if got := center(false); got != black {
		t.Errorf("non-zero: got %v", got)
	}
	if got := center(true); got != white {
		t.Errorf("even-odd: got %v", got)
	}
}

func TestFill_OppositeWindingsCancel(t *testing.T) {
	cw := square(0, 0, 8, 8)
	ccw := fanOf(pt(0, 0), pt(0, 8), pt(8, 8), pt(8, 0))
	b := &gpu.Batch{}
	addFill(b, cover(0, 0, 8, 8, geom.Hex(0x000000)), append(cw, ccw...), false, 0)
	checkPixel(t, render(t, frame{width: 8, height: 8}, b), 4, 2, white)
}

func TestFill_SameWindingsDoNotCancel(t *testing.T) {
	b := &gpu.Batch{}
	addFill(b, cover(0, 0, 8, 8, geom.Hex(0x000000)), append(square(0, 0, 8, 8), square(0, 0, 8, 8)...), false, 0)
	checkPixel(t, render(t, frame{width: 8, height: 8}, b), 4, 2, black)
}

func TestFill_CoverResetsTheStencil(t *testing.T) {
	b := &gpu.Batch{}
	addFill(b, cover(0, 0, 8, 8, geom.Hex(0x000000)), square(0, 0, 8, 8), false, 0)
	addFill(b, cover(0, 0, 8, 8, geom.Hex(0xff0000)), nil, false, 0)
	checkPixel(t, render(t, frame{width: 8, height: 8}, b), 4, 4, black)
}

func TestFill_CoverRectangleLimitsPaint(t *testing.T) {
	b := &gpu.Batch{}
	addFill(b, cover(0, 0, 4, 8, geom.Hex(0x000000)), square(0, 0, 8, 8), false, 0)
	// Marks outside the cover stay, so that the next cover still finds them.
	addFill(b, cover(4, 0, 4, 8, geom.Hex(0xff0000)), nil, false, 0)
	img := render(t, frame{width: 8, height: 8}, b)
	checkPixel(t, img, 3, 4, black)
	checkPixel(t, img, 4, 4, red)
}

// With even-odd a sample that two triangles both took would come out empty, and one
// that neither took as well. Pixel centers lie on this diagonal exactly.
func TestFill_SharedEdgeOwnsEachSampleOnce(t *testing.T) {
	for _, plain := range []bool{false, true} {
		b := &gpu.Batch{}
		triangles := []geom.Offset{pt(0, 0), pt(8, 0), pt(8, 8), pt(0, 0), pt(8, 8), pt(0, 8)}
		addFill(b, cover(0, 0, 8, 8, geom.Hex(0x000000)), triangles, true, 0)
		img := render(t, frame{width: 8, height: 8, plain: plain}, b)
		for y := range 8 {
			for x := range 8 {
				checkPixel(t, img, x, y, black)
			}
		}
	}
}

func TestFill_TopLeftRuleOnTriangles(t *testing.T) {
	// A right triangle whose top and left edges pass through pixel centers, and
	// the same area flipped so that those are its bottom and right edges.
	plain := frame{width: 8, height: 8, plain: true}
	in := cover(0, 0, 8, 8, geom.Hex(0x000000))

	b := &gpu.Batch{}
	addFill(b, in, []geom.Offset{pt(2.5, 2.5), pt(6.5, 2.5), pt(2.5, 6.5)}, false, 0)
	img := render(t, plain, b)
	checkPixel(t, img, 2, 2, black)
	checkPixel(t, img, 2, 5, black)
	checkPixel(t, img, 1, 2, white)
	checkPixel(t, img, 2, 1, white)

	b = &gpu.Batch{}
	addFill(b, in, []geom.Offset{pt(6.5, 6.5), pt(2.5, 6.5), pt(6.5, 2.5)}, false, 0)
	img = render(t, plain, b)
	checkPixel(t, img, 6, 4, white)
	checkPixel(t, img, 5, 4, black)
	checkPixel(t, img, 5, 6, white)
	checkPixel(t, img, 5, 5, black)
}

func TestFill_DegenerateTriangleMarksNothing(t *testing.T) {
	b := &gpu.Batch{}
	addFill(b, cover(0, 0, 8, 8, geom.Hex(0x000000)), []geom.Offset{pt(0, 0), pt(8, 8), pt(4, 4)}, false, 0)
	checkPixel(t, render(t, frame{width: 8, height: 8}, b), 4, 4, white)
}

func TestSnap_VerticesLandOnTheGrid(t *testing.T) {
	// 1/1024 of a pixel away from a sample: without snapping the sample would be
	// outside.
	b := &gpu.Batch{}
	addFill(b, cover(0, 0, 8, 8, geom.Hex(0x000000)), square(2.5+1.0/1024, 0, 4, 8), false, 0)
	checkPixel(t, render(t, frame{width: 8, height: 8, plain: true}, b), 2, 4, black)
}

func TestMultisample_EdgeCoverageCountsSamples(t *testing.T) {
	// The samples lie at x = 1/8, 3/8, 5/8 and 7/8 of the pixel, each once.
	tests := []struct {
		edge float32
		n    int
	}{
		{4, 4}, {4.1, 4}, {4.2, 3}, {4.3, 3}, {4.4, 2}, {4.6, 2}, {4.7, 1}, {4.8, 1}, {4.9, 0},
	}
	for _, tt := range tests {
		b := &gpu.Batch{}
		addFill(b, cover(0, 0, 8, 2, geom.Hex(0x000000)), square(tt.edge, 0, 4, 2), false, 0)
		checkPixel(t, render(t, frame{width: 8, height: 2}, b), 4, 1, mix(white, black, tt.n))
	}
}

func TestMultisample_HorizontalEdgeUsesTheOtherAxis(t *testing.T) {
	b := &gpu.Batch{}
	addFill(b, cover(0, 0, 2, 8, geom.Hex(0x000000)), square(0, 4.3, 2, 4), false, 0)
	checkPixel(t, render(t, frame{width: 2, height: 8}, b), 1, 4, mix(white, black, 3))
}

func TestMultisample_BlendsEachSampleOnItsOwn(t *testing.T) {
	// Half the samples of pixel 4 are black. A translucent layer over the whole
	// pixel then blends black and white apart, and the average comes last.
	b := &gpu.Batch{}
	addFill(b, cover(0, 0, 8, 2, geom.Hex(0x000000)), square(4.5, 0, 4, 2), false, 0)
	addQuad(b, solid(0, 0, 8, 2, geom.Color{R: 1, A: 0.5}), 0)
	img := render(t, frame{width: 8, height: 2}, b)
	// Over black red is 128 and the others 0, over white red is 255 and the others
	// 127. With two samples of each, red is (766 + 1) / 4 and the others (254 + 1) / 4.
	checkPixel(t, img, 4, 1, color.NRGBA{R: 191, G: 63, B: 63, A: 255})
}

func TestMultisample_OffNeedsTheCenter(t *testing.T) {
	b := &gpu.Batch{}
	addFill(b, cover(0, 0, 8, 2, geom.Hex(0x000000)), square(4.4, 0, 4, 2), false, 0)
	checkPixel(t, render(t, frame{width: 8, height: 2, plain: true}, b), 4, 1, black)

	b = &gpu.Batch{}
	addFill(b, cover(0, 0, 8, 2, geom.Hex(0x000000)), square(4.6, 0, 4, 2), false, 0)
	checkPixel(t, render(t, frame{width: 8, height: 2, plain: true}, b), 4, 1, white)
}

func TestMultisample_DoesNotShowOnQuads(t *testing.T) {
	in := solid(3.3, 2.7, 20.1, 9.2, geom.Color{R: 0.2, G: 0.4, B: 0.6, A: 0.8})
	in.Radii = [4]float32{5, 5, 5, 5}
	in.BorderWidth, in.BorderColor = 1.5, geom.Hex(0x102030)
	on := render(t, frame{width: 30, height: 16}, quads(in))
	off := render(t, frame{width: 30, height: 16, plain: true}, quads(in))
	for y := range 16 {
		for x := range 30 {
			checkPixel(t, on, x, y, off.NRGBAAt(x, y))
		}
	}
}
