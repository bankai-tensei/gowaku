package render

import (
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/gpu"
	"github.com/bankai-tensei/gowaku/paint"
)

// cachedFrame records two overlapping boxes in a group cached by cache, at off
// and at opacity a, under a clip, and encodes and finishes the frame.
func cachedFrame(t testing.TB, f *fixture, cache *paint.GroupCache, off geom.Offset, a float32) {
	t.Helper()
	cv := &f.canvas
	cv.Reset()
	cv.Save()
	cv.ClipRect(geom.Rect{W: 200, H: 200})
	cv.Translate(off.X, off.Y)
	cv.PushOpacity(a)
	cv.PushGroup(paint.GroupStyle{Cache: cache})
	cv.DrawRRect(rect(0, 0, 40, 40), paint.Paint{Color: geom.Color{R: 1, A: 1}})
	cv.DrawRRect(rect(20, 20, 40, 40), paint.Paint{Color: geom.Color{B: 1, A: 1}})
	cv.PopGroup()
	cv.Restore()
	Encode(cv.DisplayList(), f.glyphs, 1, &f.batch)
	if err := f.glyphs.Finish(&f.batch); err != nil {
		t.Fatal(err)
	}
}

// lastImage returns the last instance of the batch, which must be a picture.
func lastImage(t *testing.T, b *gpu.Batch) gpu.Instance {
	t.Helper()
	in := b.Instances[len(b.Instances)-1]
	if in.Kind != gpu.InstanceImage {
		t.Fatalf("last instance %+v, want a picture", in)
	}
	return in
}

// A plain cached group joins the frame the first time, keeps its picture the
// second time, and draws only the picture after that, also when it moves.
func TestCachedGroup_PlainKeepsItsPictureOnTheSecondDraw(t *testing.T) {
	f := newFixture(t, "a")
	var cache paint.GroupCache
	cachedFrame(t, f, &cache, geom.Offset{X: 10, Y: 10}, 1)
	if n := len(f.glyphs.jobs); n != 0 || len(f.batch.Instances) != 2 {
		t.Fatalf("first draw: %d jobs, %d instances, want the boxes in the frame", n, len(f.batch.Instances))
	}
	cachedFrame(t, f, &cache, geom.Offset{X: 10, Y: 10}, 1)
	if n := len(f.glyphs.jobs); n != 1 {
		t.Fatalf("second draw: %d jobs, want the picture drawn once", n)
	}
	first := lastImage(t, &f.batch).Rect
	draws := f.store.targetDraws
	cachedFrame(t, f, &cache, geom.Offset{X: 10, Y: 10}, 1)
	if n := len(f.glyphs.jobs); n != 0 || f.store.targetDraws != draws {
		t.Errorf("third draw: %d jobs, %d draws to targets, want the kept picture", n, f.store.targetDraws-draws)
	}
	if got := lastImage(t, &f.batch).Rect; got != first {
		t.Errorf("picture at %v, want %v", got, first)
	}
	// Moving by a fraction of a pixel lands on the nearest whole one.
	cachedFrame(t, f, &cache, geom.Offset{X: 30.4, Y: 9.6}, 1)
	if n := len(f.glyphs.jobs); n != 0 {
		t.Errorf("moved: %d jobs, want the kept picture", n)
	}
	if got, want := lastImage(t, &f.batch).Rect, first.Translate(geom.Offset{X: 20}); got != want {
		t.Errorf("moved picture at %v, want %v", got, want)
	}
	// Changing what the group draws joins the frame again.
	cache.Invalidate()
	cachedFrame(t, f, &cache, geom.Offset{X: 10, Y: 10}, 1)
	if n := len(f.glyphs.jobs); n != 0 || len(f.batch.Instances) != 2 {
		t.Errorf("changed: %d jobs, %d instances, want the boxes in the frame", n, len(f.batch.Instances))
	}
}

// A group that needs a picture anyway keeps it the first time, and its opacity
// changes nothing in the picture.
func TestCachedGroup_FadedKeepsItsPictureAtOnce(t *testing.T) {
	f := newFixture(t, "a")
	var cache paint.GroupCache
	cachedFrame(t, f, &cache, geom.Offset{}, 0.5)
	if n := len(f.glyphs.jobs); n != 1 {
		t.Fatalf("first draw: %d jobs, want the picture", n)
	}
	cachedFrame(t, f, &cache, geom.Offset{}, 0.25)
	if n := len(f.glyphs.jobs); n != 0 {
		t.Errorf("faded further: %d jobs, want the kept picture", n)
	}
	if a := lastImage(t, &f.batch).Color.A; a != 0.25 {
		t.Errorf("opacity %v, want 0.25", a)
	}
}

// The content of a cached group is recorded without the clip, which the picture
// gets when it is drawn.
func TestCachedGroup_PictureHoldsWhatTheClipCuts(t *testing.T) {
	f := newFixture(t, "a")
	var cache paint.GroupCache
	cv := &f.canvas
	for range 2 {
		cv.Reset()
		cv.Save()
		cv.ClipRect(geom.Rect{W: 30, H: 30})
		cv.PushGroup(paint.GroupStyle{Cache: &cache})
		cv.DrawRRect(rect(0, 0, 100, 100), paint.Paint{Color: geom.Color{R: 1, A: 1}})
		cv.DrawRRect(rect(50, 50, 40, 40), paint.Paint{Color: geom.Color{B: 1, A: 1}})
		cv.PopGroup()
		cv.Restore()
		Encode(cv.DisplayList(), f.glyphs, 1, &f.batch)
	}
	if got := f.canvas.DisplayList().Ops[1]; got.Clipped {
		t.Errorf("content clipped to %v", got.Clip)
	}
	in := lastImage(t, &f.batch)
	if in.Rect.W < 100 || in.Rect.H < 100 {
		t.Errorf("picture %v, want all of the content", in.Rect)
	}
	if d := f.batch.Draws[len(f.batch.Draws)-1]; !d.Clipped || d.Clip != (geom.Rect{W: 30, H: 30}) {
		t.Errorf("picture drawn under %v, want the clip", d.Clip)
	}
}

// A group not drawn for groupIdle frames lets go of its picture.
func TestCachedGroup_IdleDropsThePicture(t *testing.T) {
	f := newFixture(t, "a")
	var cache paint.GroupCache
	cachedFrame(t, f, &cache, geom.Offset{}, 0.5)
	destroyed := f.store.destroyed
	for range groupIdle + 2 {
		f.canvas.Reset()
		Encode(f.canvas.DisplayList(), f.glyphs, 1, &f.batch)
	}
	if len(f.glyphs.cachedList) != 0 || f.glyphs.groupCacheBytes != 0 || f.store.destroyed == destroyed {
		t.Errorf("%d cached, %d bytes, %d destroyed", len(f.glyphs.cachedList), f.glyphs.groupCacheBytes, f.store.destroyed-destroyed)
	}
}

// Past the budget, the pictures not drawn this frame make way, the oldest first.
func TestCachedGroup_BudgetEvictsTheOldest(t *testing.T) {
	f := newFixture(t, "a")
	caches := make([]paint.GroupCache, 16)
	cv := &f.canvas
	for i := range caches {
		cv.Reset()
		cv.Save()
		cv.PushOpacity(0.5)
		cv.PushGroup(paint.GroupStyle{Cache: &caches[i]})
		cv.DrawRRect(rect(0, 0, 1500, 1500), paint.Paint{Color: geom.Color{R: 1, A: 1}})
		cv.DrawRRect(rect(10, 10, 20, 20), paint.Paint{Color: geom.Color{B: 1, A: 1}})
		cv.PopGroup()
		cv.Restore()
		Encode(cv.DisplayList(), f.glyphs, 1, &f.batch)
	}
	if b := f.glyphs.groupCacheBytes; b > groupCacheBudget || b == 0 {
		t.Errorf("%d bytes kept, want at most %d", b, groupCacheBudget)
	}
	if e := f.glyphs.cachedGroups[&caches[0]]; e == nil || e.tex != nil {
		t.Errorf("the first picture is still kept: %+v", e)
	}
	if e := f.glyphs.cachedGroups[&caches[15]]; e == nil || e.tex == nil {
		t.Error("the last picture is not kept")
	}
}

// Drawing kept pictures, and content that changes every frame, allocates
// nothing.
func TestCachedGroup_EncodeDoesNotAllocate(t *testing.T) {
	f := newFixture(t, "a")
	var still, moving paint.GroupCache
	frame := func() {
		moving.Invalidate()
		cv := &f.canvas
		cv.Reset()
		for _, c := range []*paint.GroupCache{&still, &moving} {
			cv.PushGroup(paint.GroupStyle{Cache: c})
			cv.DrawRRect(rect(0, 0, 40, 40), paint.Paint{Color: geom.Color{R: 1, A: 1}})
			cv.DrawRRect(rect(20, 20, 40, 40), paint.Paint{Color: geom.Color{B: 1, A: 1}})
			cv.PopGroup()
		}
		Encode(cv.DisplayList(), f.glyphs, 1, &f.batch)
		if err := f.glyphs.Finish(&f.batch); err != nil {
			t.Fatal(err)
		}
	}
	for range 3 {
		frame()
	}
	if n := testing.AllocsPerRun(20, frame); n != 0 {
		t.Errorf("%v allocations per frame", n)
	}
}
