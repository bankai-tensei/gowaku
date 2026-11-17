package x11

import (
	"testing"

	"github.com/bankai-tensei/gowaku/input"
)

func TestButtonOf(t *testing.T) {
	for _, tt := range []struct {
		detail uint32
		want   input.Button
		ok     bool
	}{
		{1, input.ButtonPrimary, true},
		{2, input.ButtonMiddle, true},
		{3, input.ButtonSecondary, true},
		{wheelUp, 0, false},
		{wheelRight, 0, false},
		{8, input.ButtonBack, true},
		{9, input.ButtonForward, true},
		{10, 0, false},
	} {
		got, ok := buttonOf(tt.detail)
		if got != tt.want || ok != tt.ok {
			t.Errorf("buttonOf(%d) = %v, %v; want %v, %v", tt.detail, got, ok, tt.want, tt.ok)
		}
	}
	for detail, want := range map[uint32]bool{3: false, wheelUp: true, wheelDown: true, wheelLeft: true, wheelRight: true, 8: false} {
		if got := isWheel(detail); got != want {
			t.Errorf("isWheel(%d) = %v, want %v", detail, got, want)
		}
	}
}

func TestModifiersOf(t *testing.T) {
	got := modifiersOf(ShiftMask | ControlMask | Mod1Mask | 1<<4)
	if want := input.ModShift | input.ModControl | input.ModAlt; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := modifiersOf(1 << 4); got != 0 {
		t.Errorf("NumLock は修飾キーではない: got %v", got)
	}
}

func TestScaleOf(t *testing.T) {
	tests := []struct {
		name      string
		resources string
		want      float32
	}{
		{"無し", "", 1},
		{"等倍", "Xft.dpi:\t96\n", 1},
		{"2 倍", "Xcursor.size:\t24\nXft.dpi:\t192\nXft.hinting:\t1\n", 2},
		{"150%", "Xft.dpi: 144", 1.5},
		{"壊れた値", "Xft.dpi:\tlarge\n", 1},
		{"他の名前", "Foo.Xft.dpi:\t192\n", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scaleOf(tt.resources); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
