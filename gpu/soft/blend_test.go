package soft

import (
	"image"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
)

// blendTarget draws a 4x4 backdrop, then a source over its middle 2x2 with mode,
// on a transparent target, and returns the premultiplied target.
func blendTarget(t *testing.T, backdrop, src geom.Color, mode gpu.BlendMode) *Texture {
	t.Helper()
	d := New()
	tx, err := d.CreateTarget(6, 6)
	if err != nil {
		t.Fatal(err)
	}
	b := &gpu.Batch{}
	if backdrop.A > 0 {
		b.Instances = append(b.Instances, solid(0, 0, 4, 4, backdrop))
		b.Draws = append(b.Draws, gpu.DrawCall{Count: 1})
	}
	in := solid(1, 1, 2, 2, src)
	in.Kind = in.Kind.WithBlend(mode)
	b.Draws = append(b.Draws, gpu.DrawCall{First: len(b.Instances), Count: 1, Blend: mode, Bounds: geom.Rect{X: 0, Y: 0, W: 4, H: 4}})
	b.Instances = append(b.Instances, in)
	if err := d.DrawToTarget(tx, b, 1); err != nil {
		t.Fatal(err)
	}
	return tx.(*Texture)
}

func texel(tx *Texture, x, y int) [4]int {
	p := tx.pix[(y*tx.width+x)*4:]
	return [4]int{int(p[0]), int(p[1]), int(p[2]), int(p[3])}
}

func near(got, want [4]int, tolerance int) bool {
	for k := range got {
		if d := got[k] - want[k]; d < -tolerance || d > tolerance {
			return false
		}
	}
	return true
}

// The backdrop is premultiplied (60, 120, 30, 150) and the source (102, 51, 0,
// 102): the colors (0.4, 0.8, 0.2) and (1, 0.5, 0) at alphas 150/255 and 0.4.
// The expected values are the Porter-Duff formulas worked out by hand on those 8
// bit values.
var (
	translucentBackdrop = geom.Color{R: 0.4, G: 0.8, B: 0.2, A: 150.0 / 255}
	translucentSource   = geom.Color{R: 1, G: 0.5, A: 0.4}
)

func TestBlend_PorterDuffOnTranslucentBackdrop(t *testing.T) {
	tests := []struct {
		mode gpu.BlendMode
		want [4]int
	}{
		{gpu.BlendSrcOver, [4]int{138, 123, 18, 192}},
		{gpu.BlendClear, [4]int{0, 0, 0, 0}},
		{gpu.BlendSrc, [4]int{102, 51, 0, 102}},
		{gpu.BlendDst, [4]int{60, 120, 30, 150}},
		{gpu.BlendDstOver, [4]int{102, 141, 30, 192}},
		{gpu.BlendSrcIn, [4]int{60, 30, 0, 60}},
		{gpu.BlendDstIn, [4]int{24, 48, 12, 60}},
		{gpu.BlendSrcOut, [4]int{42, 21, 0, 42}},
		{gpu.BlendDstOut, [4]int{36, 72, 18, 90}},
		{gpu.BlendSrcAtop, [4]int{96, 102, 18, 150}},
		{gpu.BlendDstAtop, [4]int{66, 69, 12, 102}},
		{gpu.BlendXor, [4]int{78, 93, 18, 132}},
		{gpu.BlendPlus, [4]int{162, 171, 30, 252}},
		{gpu.BlendScreen, [4]int{138, 147, 30, 192}},
		{gpu.BlendModulate, [4]int{24, 24, 0, 60}},
		// The W3C formula: the mixed color (0.647, 0.441, 0) stands in for
		// the source, which then goes over the backdrop.
		{gpu.BlendMultiply, [4]int{102, 117, 18, 192}},
	}
	for _, tt := range tests {
		tx := blendTarget(t, translucentBackdrop, translucentSource, tt.mode)
		if got := texel(tx, 1, 1); !near(got, tt.want, 1) {
			t.Errorf("%d inside: got %v, want %v", tt.mode, got, tt.want)
		}
		// Every mode is bounded by the shape.
		if got := texel(tx, 0, 0); got != [4]int{60, 120, 30, 150} {
			t.Errorf("%d outside: got %v, want the backdrop", tt.mode, got)
		}
	}
}

// The opaque backdrop (0.2, 0.6, 0.8) and the opaque source (0.9, 0.3, 0.5) are
// whole multiples of 1/255 apart from the source, so that the formulas worked
// out by hand hold to the rounding of the last step.
func TestBlend_SeparableModes(t *testing.T) {
	backdrop := geom.Color{R: 0.2, G: 0.6, B: 0.8, A: 1}
	src := geom.Color{R: 0.9, G: 0.3, B: 0.5, A: 1}
	tests := []struct {
		mode gpu.BlendMode
		want [3]int
	}{
		{gpu.BlendMultiply, [3]int{46, 46, 102}},
		{gpu.BlendScreen, [3]int{235, 184, 230}},
		{gpu.BlendOverlay, [3]int{92, 112, 204}},
		{gpu.BlendDarken, [3]int{51, 77, 128}},
		{gpu.BlendLighten, [3]int{230, 153, 204}},
		{gpu.BlendColorDodge, [3]int{255, 219, 255}},
		{gpu.BlendColorBurn, [3]int{28, 0, 153}},
		{gpu.BlendHardLight, [3]int{214, 92, 204}},
		{gpu.BlendSoftLight, [3]int{102, 129, 204}},
		{gpu.BlendDifference, [3]int{179, 77, 77}},
		{gpu.BlendExclusion, [3]int{189, 138, 128}},
	}
	for _, tt := range tests {
		tx := blendTarget(t, backdrop, src, tt.mode)
		got := texel(tx, 1, 1)
		want := [4]int{tt.want[0], tt.want[1], tt.want[2], 255}
		if !near(got, want, 1) {
			t.Errorf("%d: got %v, want %v", tt.mode, got, want)
		}
	}
}

// The source (1, 0.2, 0) has the luminosity 0.418 and the backdrop 0.502.
func TestBlend_NonSeparableModes(t *testing.T) {
	backdrop := geom.Color{R: 0.2, G: 0.6, B: 0.8, A: 1}
	src := geom.Color{R: 1, G: 0.2, B: 0, A: 1}
	tests := []struct {
		mode gpu.BlendMode
		want [3]int
	}{
		// SetLum(SetSat(Cs, 0.6), 0.502): (0.6, 0.12, 0) moves up by 0.2512.
		{gpu.BlendHue, [3]int{217, 95, 64}},
		// SetLum(SetSat(Cb, 1), 0.502): (0, 0.667, 1), clipped at the dark end.
		{gpu.BlendSaturation, [3]int{0, 170, 254}},
		// SetLum(Cs, 0.502): (1.084, 0.284, 0.084), clipped at the light end.
		{gpu.BlendColor, [3]int{255, 80, 37}},
		// SetLum(Cb, 0.418): the backdrop moves down by 0.084.
		{gpu.BlendLuminosity, [3]int{30, 132, 183}},
	}
	for _, tt := range tests {
		tx := blendTarget(t, backdrop, src, tt.mode)
		got := texel(tx, 1, 1)
		want := [4]int{tt.want[0], tt.want[1], tt.want[2], 255}
		if !near(got, want, 1) {
			t.Errorf("%d: got %v, want %v", tt.mode, got, want)
		}
	}
}

// Where there is no backdrop, the W3C modes draw the source as source-over does.
func TestBlend_TransparentBackdropShowsTheSource(t *testing.T) {
	src := geom.Color{R: 0.9, G: 0.3, B: 0.5, A: 1}
	for m := gpu.BlendMultiply; int(m) < gpu.BlendModeCount; m++ {
		tx := blendTarget(t, geom.Color{}, src, m)
		if got, want := texel(tx, 1, 1), [4]int{230, 77, 128, 255}; !near(got, want, 1) {
			t.Errorf("%d: got %v, want %v", m, got, want)
		}
	}
}

// Instances of one call read the backdrop of before the call, and those of the
// next call see what the first drew.
func TestBlend_BackdropIsCopiedPerCall(t *testing.T) {
	d := New()
	tx, err := d.CreateTarget(4, 2)
	if err != nil {
		t.Fatal(err)
	}
	gray := geom.Color{R: 0.5, G: 0.5, B: 0.5, A: 1}
	half := geom.Color{R: 0.5, G: 0.5, B: 0.5, A: 1}
	b := &gpu.Batch{}
	b.Instances = append(b.Instances, solid(0, 0, 4, 2, gray))
	b.Draws = append(b.Draws, gpu.DrawCall{Count: 1})
	for range 2 {
		in := solid(0, 0, 2, 2, half)
		in.Kind = in.Kind.WithBlend(gpu.BlendMultiply)
		b.Draws = append(b.Draws, gpu.DrawCall{First: len(b.Instances), Count: 1, Blend: gpu.BlendMultiply, Bounds: geom.Rect{W: 2, H: 2}})
		b.Instances = append(b.Instances, in)
	}
	if err := d.DrawToTarget(tx, b, 1); err != nil {
		t.Fatal(err)
	}
	// 0.5 multiplied twice is 0.125, about 32.
	if got := texel(tx.(*Texture), 0, 0); !near(got, [4]int{32, 32, 32, 255}, 1) {
		t.Errorf("got %v, want 0.125 gray", got)
	}
}

// A multisampled frame resolves the backdrop, and blends by coverage along the
// antialiased edge.
func TestBlend_MultisampledFrameEdge(t *testing.T) {
	b := &gpu.Batch{}
	b.Instances = append(b.Instances, solid(0, 0, 8, 8, geom.Color{R: 0.2, G: 0.6, B: 0.8, A: 1}))
	b.Draws = append(b.Draws, gpu.DrawCall{Count: 1})
	in := solid(2, 2, 3.5, 3.5, geom.Color{R: 0.9, G: 0.3, B: 0.5, A: 1})
	in.Kind = in.Kind.WithBlend(gpu.BlendDifference)
	b.Draws = append(b.Draws, gpu.DrawCall{First: 1, Count: 1, Blend: gpu.BlendDifference, Bounds: geom.Rect{W: 8, H: 8}})
	b.Instances = append(b.Instances, in)
	img := render(t, frame{width: 8, height: 8}, b)
	inside := img.NRGBAAt(3, 3)
	if !near([4]int{int(inside.R), int(inside.G), int(inside.B), 255}, [4]int{179, 77, 77, 255}, 1) {
		t.Errorf("inside: got %v", inside)
	}
	// The pixel from 5 to 6 is half covered: halfway between the backdrop
	// and the difference.
	edge := img.NRGBAAt(5, 3)
	if !near([4]int{int(edge.R), int(edge.G), int(edge.B), 255}, [4]int{115, 115, 141, 255}, 2) {
		t.Errorf("edge: got %v", edge)
	}
	if got := img.NRGBAAt(7, 7); got.R != 51 || got.G != 153 || got.B != 204 {
		t.Errorf("outside: got %v", got)
	}
}

func TestBackdropRect(t *testing.T) {
	c := gpu.DrawCall{Bounds: geom.Rect{X: -2, Y: 1.2, W: 10, H: 3}, Clip: geom.Rect{X: 0, Y: 0, W: 5, H: 50}, Clipped: true}
	if got, want := gpu.BackdropRect(&c, 2, 8, 100), image.Rect(0, 2, 8, 9); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	c.Bounds = geom.Rect{X: 20, Y: 20, W: 1, H: 1}
	if got := gpu.BackdropRect(&c, 1, 8, 100); !got.Empty() {
		t.Errorf("outside the clip: got %v, want empty", got)
	}
}
