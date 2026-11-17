package wayland

import (
	"testing"

	"github.com/bankai-tensei/gowaku/input"
)

func TestButtonOf(t *testing.T) {
	for _, tt := range []struct {
		code uint32
		want input.Button
		ok   bool
	}{
		{btnLeft, input.ButtonPrimary, true},
		{btnRight, input.ButtonSecondary, true},
		{btnMiddle, input.ButtonMiddle, true},
		{btnSide, input.ButtonBack, true},
		{btnExtra, input.ButtonForward, true},
		{btnBack, input.ButtonBack, true},
		{btnForward, input.ButtonForward, true},
		{0x117, 0, false}, // BTN_TASK
	} {
		got, ok := buttonOf(tt.code)
		if got != tt.want || ok != tt.ok {
			t.Errorf("buttonOf(%#x) = %v, %v; want %v, %v", tt.code, got, ok, tt.want, tt.ok)
		}
	}
}

func TestScrollAccumulator(t *testing.T) {
	tests := []struct {
		name   string
		feed   func(s *scrollAccumulator)
		x, y   float32
		moving bool
	}{
		{"何も無い", func(*scrollAccumulator) {}, 0, 0, false},
		{"ホイールを手前に 1 段", func(s *scrollAccumulator) {
			s.axis(axisVertical, 15)
			s.value120(axisVertical, 120)
		}, 0, -1, true},
		{"高分解能ホイールの半段", func(s *scrollAccumulator) { s.value120(axisVertical, -60) }, 0, 0.5, true},
		{"v8 より前の discrete", func(s *scrollAccumulator) { s.discrete(axisHorizontal, 2) }, 2, 0, true},
		{"タッチパッド", func(s *scrollAccumulator) { s.axis(axisVertical, -7.5) }, 0, 0.5, true},
		{"斜め", func(s *scrollAccumulator) {
			s.axis(axisVertical, 15)
			s.axis(axisHorizontal, -15)
		}, -1, -1, true},
		{"知らない軸は無視", func(s *scrollAccumulator) { s.axis(2, 15) }, 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s scrollAccumulator
			tt.feed(&s)
			x, y, ok := s.take()
			if x != tt.x || y != tt.y || ok != tt.moving {
				t.Errorf("take() = %v, %v, %v; want %v, %v, %v", x, y, ok, tt.x, tt.y, tt.moving)
			}
			if _, _, ok := s.take(); ok {
				t.Error("take does not reset")
			}
		})
	}
}

func TestPhysicalSize(t *testing.T) {
	for _, tt := range []struct {
		logical int32
		scale   float64
		want    int
	}{
		{800, 1, 800},
		{800, 2, 1600},
		{801, 1.25, 1001}, // 1001.25
		{799, 1.5, 1199},  // 1198.5 rounds away from zero
		{760, 1.5, 1140},
	} {
		if got := physicalSize(tt.logical, tt.scale); got != tt.want {
			t.Errorf("physicalSize(%d, %v) = %d, want %d", tt.logical, tt.scale, got, tt.want)
		}
	}
	if got := fractionalScale(180); got != 1.5 {
		t.Errorf("fractionalScale(180) = %v, want 1.5", got)
	}
	if got := fractionalScale(0); got != 1 {
		t.Errorf("fractionalScale(0) = %v, want 1", got)
	}
}

func TestEdgeAt(t *testing.T) {
	const w, h = 400, 300
	for _, tt := range []struct {
		name string
		x, y float64
		want edge
	}{
		{"中", 200, 150, edgeNone},
		{"上", 200, 2, edgeTop},
		{"下", 200, 298, edgeBottom},
		{"左", 1, 150, edgeLeft},
		{"右", 398, 150, edgeRight},
		{"左上の角", 2, 2, edgeTopLeft},
		{"上の辺でも角の近く", 8, 1, edgeTopLeft},
		{"右の辺でも角の近く", 399, 295, edgeBottomRight},
		{"右上", 396, 3, edgeTopRight},
		{"左下", 3, 297, edgeBottomLeft},
		{"窓の外", -1, 150, edgeNone},
		{"右端の外", 400, 150, edgeNone},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := edgeAt(tt.x, tt.y, w, h); got != tt.want {
				t.Errorf("edgeAt(%v, %v) = %v, want %v", tt.x, tt.y, got, tt.want)
			}
		})
	}
}

func TestCursorNames(t *testing.T) {
	for _, e := range []edge{edgeTop, edgeBottom, edgeLeft, edgeRight, edgeTopLeft, edgeTopRight, edgeBottomLeft, edgeBottomRight} {
		s := edgeShape(e)
		if s == shapeDefault {
			t.Errorf("edge %d has no shape", e)
		}
		if cursorName(s) == "default" || legacyCursorName(s) == "left_ptr" {
			t.Errorf("shape %d of edge %d has no name", s, e)
		}
	}
	for c, want := range map[input.Cursor]string{input.CursorArrow: "default", input.CursorIBeam: "text", input.CursorHand: "pointer",
		input.CursorColResize: "col-resize", input.CursorRowResize: "row-resize"} {
		if got := cursorName(shapeOf(c)); got != want {
			t.Errorf("cursor %v is named %q, want %q", c, got, want)
		}
	}
}
