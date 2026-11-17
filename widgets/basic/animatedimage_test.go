package basic

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/bankai-tensei/gowaku/widgets/internal/wtest"

	"github.com/bankai-tensei/gowaku/core"
	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/layout"
	"github.com/bankai-tensei/gowaku/paint"
	"github.com/bankai-tensei/gowaku/semantics"
)

// testAnimation returns n frames of 8x8, each a solid color, each shown for
// delay.
func testAnimation(n int, delay time.Duration, loops int) *paint.Animation {
	a := &paint.Animation{Loops: loops}
	for i := range n {
		m := image.NewRGBA(image.Rect(0, 0, 8, 8))
		c := color.RGBA{R: uint8(40 * i), G: 100, B: 200, A: 255}
		for p := 0; p < len(m.Pix); p += 4 {
			m.Pix[p], m.Pix[p+1], m.Pix[p+2], m.Pix[p+3] = c.R, c.G, c.B, c.A
		}
		a.Frames = append(a.Frames, paint.NewImage(m))
		a.Delays = append(a.Delays, delay)
	}
	return a
}

// animatedTree returns a tree that has drawn its first frame, the render
// object of its animated image, and a function that draws the frame at a
// time, reporting whether the whole window was recorded again.
func animatedTree(t testing.TB, w core.Widget) (*core.Tree, *layout.RenderAnimatedImage, func(now time.Duration) bool) {
	t.Helper()
	tree := core.NewTree(w)
	t.Cleanup(tree.Dispose)
	size := geom.Size{W: 100, H: 100}
	cv := &paint.Canvas{}
	frame := func(now time.Duration) bool {
		tree.Tick(now)
		tree.Build()
		tree.Layout(geom.Tight(size))
		return tree.Repaint(cv)
	}
	tree.Layout(geom.Tight(size))
	cv.Reset()
	tree.Paint(cv)
	var found *layout.RenderAnimatedImage
	var walk func(ro core.RenderObject)
	walk = func(ro core.RenderObject) {
		if r, ok := ro.(*layout.RenderAnimatedImage); ok {
			found = r
		}
		for i := range ro.ChildCount() {
			walk(ro.ChildAt(i))
		}
	}
	walk(tree.RootRenderObject())
	if found == nil {
		t.Fatal("no animated image")
	}
	return tree, found, frame
}

func TestAnimatedImage_FramesRepaintWithoutBuildOrLayout(t *testing.T) {
	var layouts int
	anim := testAnimation(3, 100*time.Millisecond, paint.LoopForever)
	tree, r, frame := animatedTree(t, Align{Child: wtest.LayoutCount{N: &layouts, Child: AnimatedImage{Source: anim, Width: 40}}})
	if r.Size() != (geom.Size{W: 40, H: 40}) {
		t.Errorf("size %v: an animated image lays out as an image", r.Size())
	}
	layouts = 0
	frame(0) // the first tick starts the clock
	if tree.Ticking() {
		t.Error("the ticker runs every frame instead of sleeping until the next frame")
	}
	if wake, ok := tree.NextWake(); !ok || wake != 100*time.Millisecond {
		t.Errorf("wakes at %v, %v; want 100ms", wake, ok)
	}
	frame(100 * time.Millisecond)
	if r.Frame() != 1 || r.Source != anim.Frames[1] {
		t.Fatalf("frame %d", r.Frame())
	}
	if tree.NeedsFrame() {
		t.Error("a frame is still due after the paint")
	}
	// The next change, checked before the frame draws.
	tree.Tick(200 * time.Millisecond)
	if tree.NeedsBuild() {
		t.Error("a frame change asks for a build")
	}
	tree.Build()
	tree.Layout(geom.Tight(geom.Size{W: 100, H: 100}))
	if layouts != 0 {
		t.Errorf("%d layouts", layouts)
	}
	if frame(200*time.Millisecond) || r.Frame() != 2 {
		t.Errorf("frame %d; the whole window was recorded again, not only the boundary", r.Frame())
	}
}

func TestAnimatedImage_SteadyStateDoesNotAllocate(t *testing.T) {
	anim := testAnimation(60, 16*time.Millisecond, paint.LoopForever)
	_, r, frame := animatedTree(t, Align{Child: AnimatedImage{Source: anim}})
	now := time.Duration(0)
	loop := func() {
		for range 60 {
			frame(now)
			now += 16 * time.Millisecond
		}
	}
	loop()
	if allocs := testing.AllocsPerRun(5, loop); allocs != 0 {
		t.Errorf("a loop of 60 frames allocates %v times", allocs)
	}
	if r.Frame() == 0 && now == 0 {
		t.Error("never moved")
	}
}

func TestAnimatedImage_SourceChangeStartsOver(t *testing.T) {
	var h *wtest.HolderState
	a := testAnimation(3, 100*time.Millisecond, paint.LoopForever)
	b := testAnimation(2, 50*time.Millisecond, 1)
	_, r, frame := animatedTree(t, wtest.Holder{First: AnimatedImage{Source: a}, State: &h})
	frame(0)
	frame(150 * time.Millisecond)
	if r.Frame() != 1 {
		t.Fatalf("frame %d", r.Frame())
	}
	h.SetState(func() { h.Child = AnimatedImage{Source: b} })
	frame(160 * time.Millisecond)
	if r.Animation() != b || r.Frame() != 0 || r.Source != b.Frames[0] {
		t.Fatalf("frame %d of the new animation", r.Frame())
	}
	frame(170 * time.Millisecond) // starts the clock of b
	frame(220 * time.Millisecond)
	if r.Frame() != 1 {
		t.Errorf("frame %d of b", r.Frame())
	}
}

func TestAnimatedImage_ZeroValueAndLabel(t *testing.T) {
	tree := wtest.A11yTree(t, Align{Child: AnimatedImage{Label: "spinner", Source: testAnimation(2, time.Second, 0)}})
	root, _ := tree.SemanticsTree()
	if n := wtest.ByLabel(t, root, "spinner"); n.Role != semantics.RoleImage {
		t.Error("a labeled animated image is an image to assistive technology")
	}
	zero := core.NewTree(Align{Child: AnimatedImage{}})
	t.Cleanup(zero.Dispose)
	zero.Layout(geom.Tight(geom.Size{W: 10, H: 10}))
	var cv paint.Canvas
	cv.Reset()
	zero.Paint(&cv)
	if zero.Ticking() {
		t.Error("an empty animated image ticks")
	}
}

// BenchmarkAnimatedImage_Frame runs a frame of a window whose only change is
// the frame of an animation: the tick, and the record of its boundary.
func BenchmarkAnimatedImage_Frame(b *testing.B) {
	anim := testAnimation(60, 16*time.Millisecond, paint.LoopForever)
	_, _, frame := animatedTree(b, Align{Child: AnimatedImage{Source: anim}})
	now := time.Duration(0)
	b.ReportAllocs()
	for b.Loop() {
		frame(now)
		now += 16 * time.Millisecond
	}
}
