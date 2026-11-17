package text

import (
	"strconv"
	"testing"

	"github.com/bankai-tensei/gowaku/geom"
	"github.com/bankai-tensei/gowaku/paint"
)

func TestLayout_BaselineIsWhereTheRunsSit(t *testing.T) {
	s := goShaper(t)
	l := s.Shape("Label", Style{Size: 20}).Layout(geom.Inf)
	b := l.Baseline()
	if b <= 0 || b >= l.Size().H {
		t.Fatalf("baseline %g outside the box of height %g", b, l.Size().H)
	}
	for _, r := range l.Runs() {
		if geom.Abs32(r.Origin.Y-b) > 1e-3 {
			t.Errorf("run at y %g, baseline %g", r.Origin.Y, b)
		}
	}
	if v := s.Shape("縦", Style{Orientation: Vertical}).Layout(geom.Inf).Baseline(); v != 0 {
		t.Errorf("vertical baseline %g, want 0", v)
	}
	if e := s.Shape("", Style{Size: 20}).Layout(geom.Inf).Baseline(); geom.Abs32(e-b) > 1e-3 {
		t.Errorf("empty text baseline %g, want %g", e, b)
	}
}

func TestCanvas_DrawTextAnchors(t *testing.T) {
	s := goShaper(t)
	l := s.Shape("Tick 10", Style{Size: 12}).Layout(geom.Inf)
	sz, base := l.Size(), l.Baseline()
	origin := l.Runs()[0].Origin
	pos := geom.Offset{X: 100, Y: 50}
	for _, c := range []struct {
		name   string
		anchor paint.TextAnchor
		// want is the top left corner of the box.
		want geom.Offset
	}{
		{"top left", paint.TextAnchor{}, pos},
		{"top center", paint.TextAnchor{X: 0.5}, geom.Offset{X: pos.X - sz.W/2, Y: pos.Y}},
		{"bottom right", paint.TextAnchor{X: 1, Y: 1}, geom.Offset{X: pos.X - sz.W, Y: pos.Y - sz.H}},
		{"middle", paint.TextAnchor{X: 0.5, Y: 0.5}, geom.Offset{X: pos.X - sz.W/2, Y: pos.Y - sz.H/2}},
		{"baseline left", paint.TextAnchor{Baseline: true}, geom.Offset{X: pos.X, Y: pos.Y - base}},
		{"baseline right ignores Y", paint.TextAnchor{X: 1, Y: 1, Baseline: true}, geom.Offset{X: pos.X - sz.W, Y: pos.Y - base}},
	} {
		var cv paint.Canvas
		cv.Reset()
		cv.DrawText(l, pos, c.anchor, geom.Hex(0x202020))
		ops := cv.DisplayList().Ops
		if len(ops) != len(l.Runs()) {
			t.Fatalf("%s: %d ops, want %d", c.name, len(ops), len(l.Runs()))
		}
		got := ops[0].Origin
		want := c.want.Add(origin)
		if geom.Abs32(got.X-want.X) > 1e-3 || geom.Abs32(got.Y-want.Y) > 1e-3 {
			t.Errorf("%s: glyphs at %v, want %v", c.name, got, want)
		}
	}
}

func TestCanvas_DrawTextBaselineSitsOnTheLine(t *testing.T) {
	s := goShaper(t)
	small := s.Shape("a", Style{Size: 10}).Layout(geom.Inf)
	large := s.Shape("a", Style{Size: 30}).Layout(geom.Inf)
	var cv paint.Canvas
	cv.Reset()
	cv.DrawText(small, geom.Offset{Y: 40}, paint.TextAnchor{Baseline: true}, geom.Color{A: 1})
	cv.DrawText(large, geom.Offset{X: 50, Y: 40}, paint.TextAnchor{Baseline: true}, geom.Color{A: 1})
	ops := cv.DisplayList().Ops
	if ops[0].Origin.Y != 40 || ops[1].Origin.Y != 40 {
		t.Errorf("baselines at %g and %g, want both at 40", ops[0].Origin.Y, ops[1].Origin.Y)
	}
}

func TestLabels_KeepsLayouts(t *testing.T) {
	s := goShaper(t)
	ls := Labels{Shaper: s, Style: Style{Size: 12}}
	a := ls.Get("10")
	if ls.Get("10") != a || ls.GetBytes([]byte("10")) != a {
		t.Error("the same text must return the same layout")
	}
	ls.Style.Color = geom.Hex(0xff0000)
	if ls.Get("10") != a {
		t.Error("a change of color alone must keep the layouts")
	}
	ls.Style.Size = 14
	if ls.Get("10") == a {
		t.Error("a change of size must lay the text out again")
	}
	for i := range maxLabels + 10 {
		ls.Get(strconv.Itoa(i))
	}
	if len(ls.kept) > maxLabels {
		t.Errorf("kept %d labels, want at most %d", len(ls.kept), maxLabels)
	}
}

func TestLabels_ZeroValuePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Get without a Shaper must panic")
		}
	}()
	var ls Labels
	ls.Get("x")
}

func TestLabels_PaintingKeptLabelsDoesNotAllocate(t *testing.T) {
	s := goShaper(t)
	ls := Labels{Shaper: s, Style: Style{Size: 11}}
	var buf []byte
	var cv paint.Canvas
	paintRuler := func() {
		cv.Reset()
		for i := range 10 {
			buf = strconv.AppendInt(buf[:0], int64(i*10), 10)
			cv.DrawText(ls.GetBytes(buf), geom.Offset{X: float32(i) * 40, Y: 20}, paint.TextAnchor{X: 0.5, Baseline: true}, geom.Color{A: 1})
		}
	}
	paintRuler()
	if allocs := testing.AllocsPerRun(20, paintRuler); allocs != 0 {
		t.Errorf("%g allocations per frame, want 0", allocs)
	}
}
